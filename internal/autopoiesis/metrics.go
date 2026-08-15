package autopoiesis

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// METRICS EXPORT
// =============================================================================
// TODO P2: "Export optional metrics (generation latency, reject rates)."
//
// OuroborosStats is a pile of counters: useful to a debugger, useless to an
// operator deciding whether tool generation is working. The rates are what
// carry the signal — a rejection rate climbing past a third of attempts means
// the model has lost the plot on the safety policy, and a Thunderdome kill
// rate climbing means it is producing code that crashes on hostile input. Both
// are invisible in raw counts because the denominators differ.

// AutopoiesisMetrics is a derived, ratio-oriented snapshot of tool generation
// health. All rates are 0..1 and are 0 when the denominator is zero.
type AutopoiesisMetrics struct {
	// Volume
	GenerationRuns int `json:"generation_runs"`
	ToolsGenerated int `json:"tools_generated"`
	ToolsCompiled  int `json:"tools_compiled"`
	ToolsRejected  int `json:"tools_rejected"`
	ExecutionCount int `json:"execution_count"`

	// Latency
	MeanGenerationLatency time.Duration `json:"mean_generation_latency"`
	MaxGenerationLatency  time.Duration `json:"max_generation_latency"`

	// Rates
	RejectRate           float64 `json:"reject_rate"`            // rejected / (generated + rejected)
	SafetyViolationRate  float64 `json:"safety_violation_rate"`  // safety-exhausted / (generated + rejected)
	PanicRate            float64 `json:"panic_rate"`             // panics / runs
	ThunderdomeKillRate  float64 `json:"thunderdome_kill_rate"`  // kills / battles
	ThunderdomeEntryRate float64 `json:"thunderdome_entry_rate"` // battles / runs

	// Registry
	RegisteredTools int       `json:"registered_tools"`
	LastGeneration  time.Time `json:"last_generation"`
}

// ExportMetrics derives the current metrics snapshot.
//
// Safe on a nil Orchestrator and on one with no Ouroboros attached; callers
// are usually optional exporters (status commands, dashboards) that should not
// have to nil-check the whole chain.
func (o *Orchestrator) ExportMetrics() AutopoiesisMetrics {
	if o == nil {
		return AutopoiesisMetrics{}
	}
	o.mu.RLock()
	synth := o.ouroboros
	o.mu.RUnlock()
	if synth == nil {
		return AutopoiesisMetrics{}
	}

	stats := synth.GetStats()
	m := AutopoiesisMetrics{
		GenerationRuns:  stats.GenerationRuns,
		ToolsGenerated:  stats.ToolsGenerated,
		ToolsCompiled:   stats.ToolsCompiled,
		ToolsRejected:   stats.ToolsRejected,
		ExecutionCount:  stats.ExecutionCount,
		RegisteredTools: len(synth.ListRuntimeTools()),
		LastGeneration:  stats.LastGeneration,

		MaxGenerationLatency: stats.LongestGeneration,
	}

	if stats.GenerationRuns > 0 {
		m.MeanGenerationLatency = stats.TotalGenerationTime / time.Duration(stats.GenerationRuns)
		m.PanicRate = ratio(stats.Panics, stats.GenerationRuns)
		m.ThunderdomeEntryRate = ratio(stats.ThunderdomeRuns, stats.GenerationRuns)
	}

	// Verdicts, not runs: a run that retried three times and then succeeded is
	// one generated tool, not four.
	verdicts := stats.ToolsGenerated + stats.ToolsRejected
	m.RejectRate = ratio(stats.ToolsRejected, verdicts)
	m.SafetyViolationRate = ratio(stats.SafetyViolations, verdicts)
	m.ThunderdomeKillRate = ratio(stats.ThunderdomeKills, stats.ThunderdomeRuns)

	return m
}

func ratio(num, den int) float64 {
	if den <= 0 {
		return 0
	}
	return float64(num) / float64(den)
}

// Fields renders the metrics as sorted key/value pairs for a status line or a
// log record, so exporters do not each invent their own field names.
func (m AutopoiesisMetrics) Fields() map[string]string {
	return map[string]string{
		"generation_runs":        fmt.Sprintf("%d", m.GenerationRuns),
		"tools_generated":        fmt.Sprintf("%d", m.ToolsGenerated),
		"tools_compiled":         fmt.Sprintf("%d", m.ToolsCompiled),
		"tools_rejected":         fmt.Sprintf("%d", m.ToolsRejected),
		"registered_tools":       fmt.Sprintf("%d", m.RegisteredTools),
		"execution_count":        fmt.Sprintf("%d", m.ExecutionCount),
		"mean_generation_ms":     fmt.Sprintf("%d", m.MeanGenerationLatency.Milliseconds()),
		"max_generation_ms":      fmt.Sprintf("%d", m.MaxGenerationLatency.Milliseconds()),
		"reject_rate":            fmt.Sprintf("%.3f", m.RejectRate),
		"safety_violation_rate":  fmt.Sprintf("%.3f", m.SafetyViolationRate),
		"panic_rate":             fmt.Sprintf("%.3f", m.PanicRate),
		"thunderdome_kill_rate":  fmt.Sprintf("%.3f", m.ThunderdomeKillRate),
		"thunderdome_entry_rate": fmt.Sprintf("%.3f", m.ThunderdomeEntryRate),
	}
}

// String renders a single-line, deterministic summary.
func (m AutopoiesisMetrics) String() string {
	fields := m.Fields()
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(fields[k])
	}
	return sb.String()
}
