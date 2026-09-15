package campaign

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Wave planning for /recurse: deterministic, no LLM. The creativity lives in
// the shards that execute the tasks; the sweep order, angle rotation, and
// retargeting are fixed rules so a wave plan is reviewable and reproducible.

// RecurseAngle is one direction of attack. Each wave hits every subsystem
// from two angles, and the pair rotates every wave, so consecutive waves come
// at the same code from different directions instead of re-plowing one furrow.
type RecurseAngle string

const (
	RecurseAngleHarden RecurseAngle = "harden" // robustness: error paths, edge cases, fail-closed behavior
	RecurseAngleWire   RecurseAngle = "wire"   // integration: wiring, contracts, dead code between units
	RecurseAngleReview RecurseAngle = "review" // architecture: read-back against the corpus and the north star
	RecurseAngleTest   RecurseAngle = "test"   // coverage: regression and table tests for untested branches
	RecurseAngleBench  RecurseAngle = "bench"  // performance: benchmarks, then the wins they prove
	RecurseAngleSecure RecurseAngle = "secure" // boundaries: containment, permissions, trust of inputs
)

// RecurseAngles lists all angles in rotation order.
func RecurseAngles() []RecurseAngle {
	return []RecurseAngle{
		RecurseAngleHarden,
		RecurseAngleWire,
		RecurseAngleReview,
		RecurseAngleTest,
		RecurseAngleBench,
		RecurseAngleSecure,
	}
}

// ParseRecurseAngle validates a CLI/chat angle name. Fail closed: a wave with
// a misspelled angle would silently sweep nothing.
func ParseRecurseAngle(name string) (RecurseAngle, error) {
	for _, a := range RecurseAngles() {
		if string(a) == strings.ToLower(strings.TrimSpace(name)) {
			return a, nil
		}
	}
	return "", fmt.Errorf("unknown recurse angle %q (want one of: harden, wire, review, test, bench, secure)", name)
}

// waveAnglePairs is the six-wave rotation. Every primary and every secondary
// appears once per cycle, all six pairs are distinct, and consecutive waves
// (including the wrap from wave 5 to wave 6) share no angle — so each wave
// comes at the same code from genuinely different directions. Tabular rather
// than arithmetic because no single offset gives all three properties;
// TestWaveAngles_RotateWithoutOverlap pins them.
var waveAnglePairs = [][2]RecurseAngle{
	{RecurseAngleHarden, RecurseAngleTest},
	{RecurseAngleWire, RecurseAngleBench},
	{RecurseAngleReview, RecurseAngleSecure},
	{RecurseAngleTest, RecurseAngleWire},
	{RecurseAngleBench, RecurseAngleHarden},
	{RecurseAngleSecure, RecurseAngleReview},
}

// WaveAngles returns the two angles for a zero-based wave.
func WaveAngles(wave int) (primary, secondary RecurseAngle) {
	pair := waveAnglePairs[wave%len(waveAnglePairs)]
	return pair[0], pair[1]
}

// RecurseConfig is the operator-facing shape of a /recurse run. Every field
// is settable from flags; the CLI coverage test enforces that.
type RecurseConfig struct {
	// MaxWaves bounds the run. Zero means unbounded, which the CLIs only
	// allow with yolo mode (the runner also requires AllowUnbounded).
	MaxWaves int `json:"max_waves"`
	// Angles overrides the rotation with a fixed pair. Empty means rotate.
	Angles []RecurseAngle `json:"angles,omitzero"`
	// Subsystems focuses the sweep; dependencies are pulled in automatically.
	Subsystems []string `json:"subsystems,omitzero"`
	// StallWaveLimit stops the run after this many consecutive waves with
	// zero completed tasks. Zero means the default (2). This is the fuse
	// that keeps "forever" honest: looping without progress is a stall.
	StallWaveLimit int `json:"stall_wave_limit"`
	// ContextBudget overrides the campaign context budget. Zero keeps the default.
	ContextBudget int `json:"context_budget"`
}

// Normalize fills defaults and validates. Angles beyond two are rejected: a
// wave is two angles, and a third would either starve or serialize the sweep.
func (c RecurseConfig) Normalize() (RecurseConfig, error) {
	if c.MaxWaves < 0 {
		return c, fmt.Errorf("recurse: max waves must be >= 0 (0 = unbounded with yolo)")
	}
	if len(c.Angles) > 2 {
		return c, fmt.Errorf("recurse: at most 2 angles per wave, got %d", len(c.Angles))
	}
	for _, a := range c.Angles {
		if _, err := ParseRecurseAngle(string(a)); err != nil {
			return c, err
		}
	}
	if c.StallWaveLimit < 0 {
		return c, fmt.Errorf("recurse: stall wave limit must be >= 0")
	}
	if c.StallWaveLimit == 0 {
		c.StallWaveLimit = DefaultRecurseStallWaves
	}
	return c, nil
}

// DefaultRecurseStallWaves stops an unbounded run after two consecutive waves
// with nothing completed.
const DefaultRecurseStallWaves = 2

// WaveFindings is what the planner reads back from a finished wave: counts
// plus the per-subsystem failures the next wave retargets. Deterministic —
// read from task statuses, never synthesized prose.
type WaveFindings struct {
	Completed int
	Failed    int
	// FailedNodes lists subsystem node IDs with at least one failed task,
	// most failures first. The next wave sweeps these first.
	FailedNodes []string
}

// SummarizeWave reads a finished wave campaign. Tasks are matched to DAG
// nodes through their phase names ("recurse:<node-id>:...").
func SummarizeWave(c *Campaign) WaveFindings {
	var f WaveFindings
	failures := map[string]int{}
	for i := range c.Phases {
		node := recurseNodeOfPhase(c.Phases[i].Name)
		for j := range c.Phases[i].Tasks {
			switch c.Phases[i].Tasks[j].Status {
			case TaskCompleted:
				f.Completed++
			case TaskFailed:
				f.Failed++
				if node != "" {
					failures[node]++
				}
			}
		}
	}
	for node := range failures {
		f.FailedNodes = append(f.FailedNodes, node)
	}
	sort.Slice(f.FailedNodes, func(i, j int) bool {
		if failures[f.FailedNodes[i]] != failures[f.FailedNodes[j]] {
			return failures[f.FailedNodes[i]] > failures[f.FailedNodes[j]]
		}
		return f.FailedNodes[i] < f.FailedNodes[j]
	})
	return f
}

func recurseNodeOfPhase(name string) string {
	rest, ok := strings.CutPrefix(name, "recurse:")
	if !ok {
		return ""
	}
	node, _, _ := strings.Cut(rest, ":")
	return node
}

// NewRecurseCampaign builds wave zero: every DAG node in topo order, two
// angle tasks per node, hard phase dependencies along the DAG edges, and
// context chaining so each task sees what the sweep already learned.
func NewRecurseCampaign(workspace string, cfg RecurseConfig) (*Campaign, error) {
	cfg, err := cfg.Normalize()
	if err != nil {
		return nil, err
	}
	nodes, err := TopoOrder(RecurseDAG())
	if err != nil {
		return nil, err
	}
	if len(cfg.Subsystems) > 0 {
		nodes, err = FilterDAG(nodes, cfg.Subsystems)
		if err != nil {
			return nil, err
		}
		nodes, err = TopoOrder(nodes)
		if err != nil {
			return nil, err
		}
	}
	recurseID := fmt.Sprintf("/recurse_%s", uuid.New().String()[:8])
	return buildRecurseWave(workspace, recurseID, 0, cfg, nodes, nil), nil
}

// PlanNextWave builds wave prev.RecurseWave+1 from the previous wave's
// findings: failed nodes sweep first (in topo order among themselves), the
// angle pair rotates, and the goal records what the wave is answering.
func PlanNextWave(workspace string, cfg RecurseConfig, prev *Campaign) (*Campaign, error) {
	cfg, err := cfg.Normalize()
	if err != nil {
		return nil, err
	}
	if prev == nil || prev.RecurseID == "" {
		return nil, fmt.Errorf("recurse: cannot plan a next wave without a previous recurse wave")
	}
	nodes, err := TopoOrder(RecurseDAG())
	if err != nil {
		return nil, err
	}
	if len(cfg.Subsystems) > 0 {
		nodes, err = FilterDAG(nodes, cfg.Subsystems)
		if err != nil {
			return nil, err
		}
		nodes, err = TopoOrder(nodes)
		if err != nil {
			return nil, err
		}
	}
	findings := SummarizeWave(prev)
	nodes = retargetNodes(nodes, findings.FailedNodes)
	return buildRecurseWave(workspace, prev.RecurseID, prev.RecurseWave+1, cfg, nodes, &findings), nil
}

// retargetNodes moves failed nodes (and only failed nodes) earlier while
// keeping a valid topo order: stable partition by membership, since the input
// is already topo-sorted and both halves keep their relative order. A failed
// node still sweeps after its dependencies — retargeting reorders the wave,
// it does not break the DAG.
func retargetNodes(nodes []SubsystemNode, failed []string) []SubsystemNode {
	if len(failed) == 0 {
		return nodes
	}
	isFailed := make(map[string]bool, len(failed))
	for _, id := range failed {
		isFailed[id] = true
	}
	var first, rest []SubsystemNode
	for _, n := range nodes {
		if isFailed[n.ID] {
			first = append(first, n)
		} else {
			rest = append(rest, n)
		}
	}
	// Repair: a failed node whose dependency is not failed must stay after
	// it. Walk the joined order and bubble such nodes down past their deps.
	joined := append(first, rest...)
	position := make(map[string]int, len(joined))
	for i, n := range joined {
		position[n.ID] = i
	}
	for {
		moved := false
		for i, n := range joined {
			for _, dep := range n.DependsOn {
				if position[dep] > i {
					// Move n just after dep, keeping everything else stable.
					joined = append(joined[:i], joined[i+1:]...)
					at := position[dep]
					joined = append(joined[:at+1], append([]SubsystemNode{n}, joined[at+1:]...)...)
					for j, m := range joined {
						position[m.ID] = j
					}
					moved = true
					break
				}
			}
			if moved {
				break
			}
		}
		if !moved {
			return joined
		}
	}
}

func buildRecurseWave(workspace, recurseID string, wave int, cfg RecurseConfig, nodes []SubsystemNode, prev *WaveFindings) *Campaign {
	now := time.Now()
	campaignID := fmt.Sprintf("/campaign_%s", uuid.New().String()[:8])
	slug := sanitizeCampaignID(campaignID)

	primary, secondary := WaveAngles(wave)
	if len(cfg.Angles) == 2 {
		primary, secondary = cfg.Angles[0], cfg.Angles[1]
	} else if len(cfg.Angles) == 1 {
		primary, secondary = cfg.Angles[0], cfg.Angles[0]
	}

	title := fmt.Sprintf("Recurse wave %d (%s + %s)", wave, primary, secondary)
	goal := fmt.Sprintf("Self-improvement sweep wave %d over %d subsystems from the %s and %s angles: optimize, stabilize, and harden each node in DAG order, then wiring, review, and benchmarks across the tree.",
		wave, len(nodes), primary, secondary)
	if prev != nil {
		goal += fmt.Sprintf(" Previous wave: %d tasks completed, %d failed.", prev.Completed, prev.Failed)
		if len(prev.FailedNodes) > 0 {
			goal += fmt.Sprintf(" Retargeting failed nodes first: %s.", strings.Join(prev.FailedNodes, ", "))
		}
	}

	c := &Campaign{
		ID:              campaignID,
		Type:            CampaignTypeRecurse,
		Title:           title,
		Goal:            goal,
		SourceMaterial:  []string{},
		KnowledgeBase:   filepath.Join(workspace, ".nerd", "campaigns", slug, "knowledge.db"),
		Status:          StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
		Confidence:      1.0,
		ContextBudget:   cfg.ContextBudget,
		Phases:          make([]Phase, 0, len(nodes)),
		ContextProfiles: buildContextProfiles(campaignID),
		RecurseID:       recurseID,
		RecurseWave:     wave,
	}

	phaseIDs := make(map[string]string, len(nodes))
	for i, n := range nodes {
		phaseID := fmt.Sprintf("/phase_%s_%d", campaignID[10:], i)
		phaseIDs[n.ID] = phaseID
	}
	var prevLastTask string
	for i, n := range nodes {
		phaseID := phaseIDs[n.ID]
		profile := c.ContextProfiles[i%len(c.ContextProfiles)].ID
		tasks := []Task{
			recurseTask(campaignID, phaseID, i, 0, n, primary, prevLastTask),
			recurseTask(campaignID, phaseID, i, 1, n, secondary, ""),
		}
		// The secondary task reads the primary's result; the next phase's
		// primary reads this phase's secondary. Context flows down the DAG
		// instead of each task re-discovering the sweep.
		tasks[1].ContextFrom = []string{tasks[0].ID}
		prevLastTask = tasks[1].ID

		var deps []PhaseDependency
		for _, dep := range n.DependsOn {
			if depID, ok := phaseIDs[dep]; ok {
				deps = append(deps, PhaseDependency{DependsOnPhaseID: depID, Type: DepHard})
			}
		}
		c.Phases = append(c.Phases, Phase{
			ID:             phaseID,
			CampaignID:     campaignID,
			Name:           fmt.Sprintf("recurse:%s:%s", n.ID, n.Title),
			Order:          i,
			Category:       "/recurse",
			Status:         PhasePending,
			ContextProfile: profile,
			Objectives: []PhaseObjective{{
				Type:               ObjectiveModify,
				Description:        fmt.Sprintf("%s the %s subsystem (%s)", angleVerb(primary), n.Title, strings.Join(n.Paths, ", ")),
				VerificationMethod: angleVerification(primary, secondary),
			}},
			EstimatedTasks:      2,
			EstimatedComplexity: "/high",
			Tasks:               tasks,
			Dependencies:        deps,
			Checkpoints: []Checkpoint{{
				Type:      string(angleVerification(primary, secondary)),
				Passed:    false,
				Timestamp: time.Time{},
			}},
		})
	}
	c.TotalPhases = len(c.Phases)
	c.TotalTasks = len(c.Phases) * 2
	return c
}

func recurseTask(campaignID, phaseID string, phaseOrder, taskOrder int, n SubsystemNode, angle RecurseAngle, contextFrom string) Task {
	scope := strings.Join(n.Paths, ", ")
	if scope == "" {
		scope = "the whole tree"
	}
	t := Task{
		ID:          fmt.Sprintf("/task_%s_%d_%d", campaignID[10:], phaseOrder, taskOrder),
		PhaseID:     phaseID,
		Description: angleTask(angle, n.Title, scope),
		Status:      TaskPending,
		Type:        angleTaskType(angle),
		PlannedType: angleTaskType(angle),
		Priority:    PriorityHigh,
		Order:       taskOrder,
		WriteSet:    n.Paths,
	}
	if taskOrder == 1 {
		t.Priority = PriorityNormal
	}
	if contextFrom != "" && taskOrder == 0 {
		t.ContextFrom = []string{contextFrom}
	}
	return t
}

func angleTaskType(a RecurseAngle) TaskType {
	switch a {
	case RecurseAngleHarden:
		return TaskTypeFileModify
	case RecurseAngleWire:
		return TaskTypeIntegrate
	case RecurseAngleReview, RecurseAngleSecure:
		return TaskTypeResearch
	case RecurseAngleTest, RecurseAngleBench:
		return TaskTypeTestWrite
	default:
		return TaskTypeFileModify
	}
}

func angleVerb(a RecurseAngle) string {
	switch a {
	case RecurseAngleHarden:
		return "Harden"
	case RecurseAngleWire:
		return "Wire up"
	case RecurseAngleReview:
		return "Architecturally review"
	case RecurseAngleTest:
		return "Cover with tests"
	case RecurseAngleBench:
		return "Benchmark and speed up"
	case RecurseAngleSecure:
		return "Audit the boundaries of"
	default:
		return "Improve"
	}
}

func angleTask(a RecurseAngle, title, scope string) string {
	target := fmt.Sprintf("%s (%s)", title, scope)
	switch a {
	case RecurseAngleHarden:
		return fmt.Sprintf("HARDEN %s: close error-path gaps, handle edge cases, make failures fail closed with honest errors, and add a regression test for every fix. Optimize for stability.", target)
	case RecurseAngleWire:
		return fmt.Sprintf("WIRE %s: verify every integration point is actually connected (no dormant handlers, no unwired declarations), fix the gaps, and remove or wire the dead code. Optimize for cohesion.", target)
	case RecurseAngleReview:
		return fmt.Sprintf("REVIEW %s against the architecture corpus and the repo north star: find where the code drifted from the design, file the drift as findings, and fix what can be fixed in this task. Optimize for alignment.", target)
	case RecurseAngleTest:
		return fmt.Sprintf("TEST %s: find untested branches and behaviors, write table-driven regression tests that fail before the fix and pass after, and keep the suite green. Optimize for coverage that means something.", target)
	case RecurseAngleBench:
		return fmt.Sprintf("BENCH %s: write Go benchmarks for the hot paths, measure, optimize what the numbers justify, and keep the benchmarks as permanent guards. Report before/after numbers. Optimize for measured speed.", target)
	case RecurseAngleSecure:
		return fmt.Sprintf("SECURE %s: audit trust boundaries — path containment, permission checks, input validation, secret handling — prove each boundary with an adversarial test, and fix what fails. Optimize for defense in depth.", target)
	default:
		return fmt.Sprintf("IMPROVE %s.", target)
	}
}

// angleVerification picks the honest phase gate: test and bench waves prove
// themselves with the suite; everything else gets an automated reviewer
// shard. Manual review is never planned — a sweep that pauses for a human
// every phase is not a sweep.
func angleVerification(primary, secondary RecurseAngle) VerificationMethod {
	for _, a := range []RecurseAngle{primary, secondary} {
		if a == RecurseAngleTest || a == RecurseAngleBench {
			return VerifyTestsPass
		}
	}
	return VerifyShardValidate
}
