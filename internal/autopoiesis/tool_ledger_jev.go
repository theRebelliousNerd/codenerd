package autopoiesis

import (
	"sort"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// TOOL LEDGER - JEV'S READINGS OF EACH TOOL
// =============================================================================
// Jev's observations about a tool (tool_value.go) are kernel facts, and a
// kernel dies with its process. This keeps what the policy rules need of them
// in the same ledger section as the measurements, so the evidence merges across
// processes the way the measurements do and a restart re-derives the same
// classes:
//
//   - per call, how it was judged once, as a tally (landed, missed, unsettled)
//     with the time of the latest of each: the rules weigh hits against misses
//     over the tool's whole history, so a count is what they need, not the
//     call. Each call is counted once, as it is judged;
//   - per tool, Jev's latest verdict on whether another tool or the harness
//     already does what it does (duplicate, harness_covers), with the
//     probability, the tool Jev named, and when. A later reading replaces an
//     earlier one: it is a state of the tool, not a sum.
//
// The policy reads the tallies as tool_jev_tally and the verdicts as
// tool_jev_verdict (policy/jev_learning.mg).

// JevCallOutcome is how Jev's reading settled one call.
type JevCallOutcome string

const (
	JevCallLanded    JevCallOutcome = "landed"
	JevCallMissed    JevCallOutcome = "missed"
	JevCallUnsettled JevCallOutcome = "unsettled"
)

// Verdict kinds, as the policy names them.
const (
	JevVerdictDuplicate     = "duplicate"
	JevVerdictHarnessCovers = "harness_covers"
)

// ToolJevEvidence is one tool's persisted Jev evidence.
type ToolJevEvidence struct {
	Landed     int64     `json:"landed,omitempty"`
	Missed     int64     `json:"missed,omitempty"`
	Unsettled  int64     `json:"unsettled,omitempty"`
	LastLanded time.Time `json:"last_landed,omitzero"`
	LastMissed time.Time `json:"last_missed,omitzero"`

	Duplicate ToolJevVerdict `json:"duplicate,omitzero"`
	Harness   ToolJevVerdict `json:"harness_covers,omitzero"`
}

// ToolJevVerdict is Jev's latest answer to one tool-level question. A zero At
// is no answer yet.
type ToolJevVerdict struct {
	// Yes is the kernel's reading of Percent against the thresholds in effect
	// at At.
	Yes bool `json:"yes"`
	// Percent is the probability Jev gave.
	Percent int64 `json:"percent"`
	// Other is the tool Jev named as the duplicate (duplicate only).
	Other string    `json:"other,omitempty"`
	At    time.Time `json:"at,omitzero"`
}

// Answered reports whether Jev has ever answered the question.
func (v ToolJevVerdict) Answered() bool { return !v.At.IsZero() }

// Judged is the calls Jev read that settled either way.
func (e ToolJevEvidence) Judged() int64 { return e.Landed + e.Missed }

// NoteJevCall counts one judged call for the tool.
func (ps *ProfileStore) NoteJevCall(tool string, outcome JevCallOutcome) {
	if tool == "" {
		return
	}
	ps.mutateLedgers(ledgerWrite{tool: tool, fn: func(l *ToolLedger) {
		now := time.Now()
		switch outcome {
		case JevCallLanded:
			l.Jev.Landed++
			l.Jev.LastLanded = now
		case JevCallMissed:
			l.Jev.Missed++
			l.Jev.LastMissed = now
		default:
			l.Jev.Unsettled++
		}
	}})
}

// JevVerdictWrite is one tool-level verdict to record.
type JevVerdictWrite struct {
	Tool string
	Kind string // JevVerdictDuplicate or JevVerdictHarnessCovers
	ToolJevVerdict
}

// NoteJevVerdicts records the latest tool-level verdicts, replacing the ones
// they answer. A write with a zero At is stamped now.
func (ps *ProfileStore) NoteJevVerdicts(verdicts []JevVerdictWrite) {
	writes := make([]ledgerWrite, 0, len(verdicts))
	for _, v := range verdicts {
		if v.Tool == "" {
			continue
		}
		if v.At.IsZero() {
			v.At = time.Now()
		}
		writes = append(writes, ledgerWrite{tool: v.Tool, fn: func(l *ToolLedger) {
			switch v.Kind {
			case JevVerdictDuplicate:
				l.Jev.Duplicate = v.ToolJevVerdict
			case JevVerdictHarnessCovers:
				l.Jev.Harness = v.ToolJevVerdict
			}
		}})
	}
	if len(writes) > 0 {
		ps.mutateLedgers(writes...)
	}
}

// Predicates the Jev replay owns.
const (
	jevTallyPred   = "tool_jev_tally"
	jevVerdictPred = "tool_jev_verdict"
)

// jevPredicates are retracted and re-asserted together on a refresh.
var jevPredicates = []string{jevTallyPred, jevVerdictPred}

// jevFacts replays one tool's persisted evidence as the EDB facts the policy
// reads. A tool Jev never read has none.
func jevFacts(tool string, e ToolJevEvidence) []types.Fact {
	var facts []types.Fact
	if e.Judged() > 0 {
		facts = append(facts, types.Fact{Predicate: jevTallyPred, Args: []any{tool, e.Landed, e.Missed}})
	}
	for _, v := range []struct {
		kind    string
		verdict ToolJevVerdict
	}{{JevVerdictDuplicate, e.Duplicate}, {JevVerdictHarnessCovers, e.Harness}} {
		if v.verdict.Answered() && v.verdict.Yes {
			facts = append(facts, types.Fact{Predicate: jevVerdictPred, Args: []any{tool, nameAtom(v.kind), v.verdict.Percent}})
		}
	}
	return facts
}

// JevFacts replays every live tool's persisted evidence. A retired tool has
// none: its record started over.
func JevFacts(ledgers map[string]ToolLedger) []types.Fact {
	names := make([]string, 0, len(ledgers))
	for tool := range ledgers {
		names = append(names, tool)
	}
	sort.Strings(names)
	var facts []types.Fact
	for _, tool := range names {
		l := ledgers[tool]
		if !l.RetiredAt.IsZero() {
			continue
		}
		facts = append(facts, jevFacts(tool, l.Jev)...)
	}
	return facts
}

// ReplaceJevFacts swaps the kernel's Jev replay for the ledgers'.
func ReplaceJevFacts(k types.Kernel, ledgers map[string]ToolLedger) error {
	for _, pred := range jevPredicates {
		if err := k.Retract(pred); err != nil {
			return err
		}
	}
	if facts := JevFacts(ledgers); len(facts) > 0 {
		return k.AssertBatch(facts)
	}
	return nil
}
