package campaign

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

// Metrics hooks, deliberately backend-agnostic.
//
// The orchestrator already produces every number an operator wants — task
// durations, checkpoint outcomes, phase wall time — and then throws them away
// into a log line. Wiring Prometheus (or OTel, or statsd) in here would put a
// metrics dependency in the campaign engine and force every consumer to accept
// that choice.
//
// Instead the engine calls a narrow sink. The whole surface is four methods
// with primitive arguments, so an adapter for any backend is a dozen lines and
// lives in the consumer. Nil sink = no observation, no allocation, no cost.

// MetricsSink receives campaign execution observations. Implementations MUST be
// safe for concurrent use: tasks run in parallel goroutines.
type MetricsSink interface {
	// ObserveTaskDuration records how long one task took. outcome is one of
	// "completed", "failed". taskType is the /atom task type.
	ObserveTaskDuration(campaignID, phaseID, taskType, outcome string, d time.Duration)

	// ObservePhaseDuration records wall time from phase start to completion.
	ObservePhaseDuration(campaignID, phaseID string, d time.Duration)

	// ObserveCheckpoint records one verification result. method is the
	// VerificationMethod atom.
	ObserveCheckpoint(campaignID, phaseID, method string, passed bool, d time.Duration)

	// ObserveRiskPreflight records the preflight outcome and its score.
	ObserveRiskPreflight(campaignID string, score int, allowed bool, hardFindings, softFindings int)
}

// SetMetricsSink installs a metrics sink. Passing nil disables observation.
func (o *Orchestrator) SetMetricsSink(sink MetricsSink) {
	o.metricsMu.Lock()
	defer o.metricsMu.Unlock()
	o.metrics = sink
}

// metricsSink deliberately uses its own mutex rather than o.mu. Risk preflight
// observation happens while Run holds o.mu, and sync.RWMutex is not reentrant:
// taking o.mu.RLock there would deadlock the campaign at start.
func (o *Orchestrator) metricsSink() MetricsSink {
	o.metricsMu.RLock()
	defer o.metricsMu.RUnlock()
	return o.metrics
}

func (o *Orchestrator) observeTaskDuration(phaseID string, task *Task, outcome string, d time.Duration) {
	sink := o.metricsSink()
	if sink == nil || task == nil {
		return
	}
	sink.ObserveTaskDuration(o.campaignIDForMetrics(), phaseID, string(task.Type), outcome, d)
}

// markPhaseStart records when a phase entered /in_progress. Phase carries no
// StartedAt field on disk, and adding one would change the durable snapshot
// schema for an observability nicety, so the clock stays in memory.
func (o *Orchestrator) markPhaseStart(phaseID string) {
	if o.metricsSink() == nil || phaseID == "" {
		return
	}
	o.phaseStarts.Store(phaseID, time.Now())
}

func (o *Orchestrator) observePhaseDuration(phaseID string) {
	sink := o.metricsSink()
	if sink == nil || phaseID == "" {
		return
	}
	started, ok := o.phaseStarts.LoadAndDelete(phaseID)
	if !ok {
		return
	}
	start, ok := started.(time.Time)
	if !ok {
		return
	}
	sink.ObservePhaseDuration(o.campaignIDForMetrics(), phaseID, time.Since(start))
}

func (o *Orchestrator) observeCheckpoint(phaseID, method string, passed bool, d time.Duration) {
	if sink := o.metricsSink(); sink != nil {
		sink.ObserveCheckpoint(o.campaignIDForMetrics(), phaseID, method, passed, d)
	}
}

// observeRiskPreflight takes the campaign ID as an argument because it runs
// while Run holds o.mu; it must not reach back through campaignIDForMetrics.
func (o *Orchestrator) observeRiskPreflight(campaignID string, eval *RiskGateEvaluation) {
	sink := o.metricsSink()
	if sink == nil || eval == nil {
		return
	}
	score := 0
	if eval.Decision != nil {
		score = eval.Decision.Score
	}
	sink.ObserveRiskPreflight(campaignID, score, eval.Allowed,
		len(eval.HardFindings()), len(eval.SoftFindings()))
}

func (o *Orchestrator) campaignIDForMetrics() string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return ""
	}
	return o.campaign.ID
}

// InMemoryMetrics is a dependency-free MetricsSink. `nerd campaign start` and
// `resume` install one and print its Summary when the run ends, so the numbers
// the orchestrator measures reach the operator instead of a log line. It keeps
// counts and totals only — a real histogram belongs in a real backend.
type InMemoryMetrics struct {
	mu sync.Mutex

	TaskCount     map[string]int
	TaskTotal     map[string]time.Duration
	TaskMax       map[string]time.Duration
	PhaseTotal    map[string]time.Duration
	CheckpointOK  map[string]int
	CheckpointBad map[string]int

	RiskPreflights int
	RiskBlocked    int
	RiskSoft       int
	RiskHard       int
	LastRiskScore  int
}

// NewInMemoryMetrics returns an initialized in-memory sink.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{
		TaskCount:     make(map[string]int),
		TaskTotal:     make(map[string]time.Duration),
		TaskMax:       make(map[string]time.Duration),
		PhaseTotal:    make(map[string]time.Duration),
		CheckpointOK:  make(map[string]int),
		CheckpointBad: make(map[string]int),
	}
}

func (m *InMemoryMetrics) ObserveTaskDuration(_, _, taskType, outcome string, d time.Duration) {
	key := taskType + "|" + outcome
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TaskCount[key]++
	m.TaskTotal[key] += d
	if d > m.TaskMax[key] {
		m.TaskMax[key] = d
	}
}

func (m *InMemoryMetrics) ObservePhaseDuration(_, phaseID string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PhaseTotal[phaseID] += d
}

func (m *InMemoryMetrics) ObserveCheckpoint(_, _, method string, passed bool, _ time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if passed {
		m.CheckpointOK[method]++
	} else {
		m.CheckpointBad[method]++
	}
}

func (m *InMemoryMetrics) ObserveRiskPreflight(_ string, score int, allowed bool, hard, soft int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RiskPreflights++
	m.LastRiskScore = score
	if !allowed {
		m.RiskBlocked++
	}
	m.RiskHard += hard
	m.RiskSoft += soft
}

// Summary renders the observations as report lines, sorted so two runs with
// the same numbers print the same text. It is empty when nothing was observed.
func (m *InMemoryMetrics) Summary() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lines []string
	if len(m.TaskCount) > 0 {
		lines = append(lines, "tasks:")
		for _, key := range slices.Sorted(maps.Keys(m.TaskCount)) {
			taskType, outcome, _ := strings.Cut(key, "|")
			count := m.TaskCount[key]
			avg := m.TaskTotal[key] / time.Duration(count)
			lines = append(lines, fmt.Sprintf("  %s %s: %d (avg %s, max %s)",
				taskType, outcome, count, avg.Round(time.Millisecond), m.TaskMax[key].Round(time.Millisecond)))
		}
	}
	methods := make(map[string]bool, len(m.CheckpointOK)+len(m.CheckpointBad))
	for method := range m.CheckpointOK {
		methods[method] = true
	}
	for method := range m.CheckpointBad {
		methods[method] = true
	}
	if len(methods) > 0 {
		lines = append(lines, "checkpoints:")
		for _, method := range slices.Sorted(maps.Keys(methods)) {
			lines = append(lines, fmt.Sprintf("  %s: %d passed, %d failed", method, m.CheckpointOK[method], m.CheckpointBad[method]))
		}
	}
	if len(m.PhaseTotal) > 0 {
		lines = append(lines, "phases:")
		for _, id := range slices.Sorted(maps.Keys(m.PhaseTotal)) {
			lines = append(lines, fmt.Sprintf("  %s: %s", id, m.PhaseTotal[id].Round(time.Second)))
		}
	}
	if m.RiskPreflights > 0 {
		lines = append(lines, fmt.Sprintf("risk preflight: score %d, %d blocked of %d (%d hard, %d soft findings)",
			m.LastRiskScore, m.RiskBlocked, m.RiskPreflights, m.RiskHard, m.RiskSoft))
	}
	return lines
}
