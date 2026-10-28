package autopoiesis

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// ToolValueClass is the policy's verdict on one tool.
type ToolValueClass string

const (
	ToolEarning  ToolValueClass = "earning"
	ToolMarginal ToolValueClass = "marginal"
	ToolDead     ToolValueClass = "dead"
	ToolUnknown  ToolValueClass = "unknown"
)

// ToolRetirementProposal is a derived proposal about one tool. Kind is "retire"
// for a generated tool (removal may be proposed through the work-mode
// approval) and "flag" for a built-in (only ever flagged). It removes nothing.
type ToolRetirementProposal struct {
	Tool   string `json:"tool"`
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

// ToolValueRow is one tool in the report: the policy's class and reasons, and
// the measurements they were derived from.
type ToolValueRow struct {
	Tool       string         `json:"tool"`
	Class      ToolValueClass `json:"class"`
	Origin     string         `json:"origin,omitempty"`
	Capability string         `json:"capability,omitempty"`

	Calls         int64 `json:"calls"`
	CallSessions  int64 `json:"call_sessions"`
	Offers        int64 `json:"offers"`
	OfferSessions int64 `json:"offer_sessions"`

	PromptTokens int64 `json:"prompt_tokens"`
	CallTokens   int64 `json:"call_tokens"`
	ResultTokens int64 `json:"result_tokens"`
	VerifyTokens int64 `json:"verify_tokens"`

	// Condensation: only calls whose tool reported a scan. KnownCalls == 0
	// means the tool's condensation is unknown, not small; the percentages are
	// then null.
	KnownCalls          int64  `json:"known_calls"`
	ConsumedBytes       int64  `json:"consumed_bytes"`
	ReturnedBytes       int64  `json:"returned_bytes"`
	CondensationPercent *int64 `json:"condensation_percent"`
	LeveragePercent     *int64 `json:"leverage_percent"`

	Channels    map[string]int64 `json:"channels,omitempty"`
	DeadReasons []string         `json:"dead_reasons,omitempty"`
	// Evidence is Jev's reading of the tool, when the kernel holds it
	// (tool_value_evidence): duplicate, harness_covers, hits_mark, misses_mark.
	Evidence []string `json:"jev_evidence,omitempty"`
	// JevLanded and JevMissed are the persisted tallies behind the hits and
	// misses evidence (calls Jev read that settled either way).
	JevLanded int64 `json:"jev_landed,omitempty"`
	JevMissed int64 `json:"jev_missed,omitempty"`
}

// CostTokens is everything the tool cost: prompt share, calls, results and the
// verification attributed to it.
func (r ToolValueRow) CostTokens() int64 {
	return r.PromptTokens + r.CallTokens + r.ResultTokens + r.VerifyTokens
}

// ToolValueReport is the policy's answer for every tool the snapshot names.
type ToolValueReport struct {
	Rows      []ToolValueRow           `json:"tools"`
	Proposals []ToolRetirementProposal `json:"proposals"`
}

// Counts tallies rows by class.
func (r ToolValueReport) Counts() map[ToolValueClass]int {
	out := map[ToolValueClass]int{}
	for _, row := range r.Rows {
		out[row.Class]++
	}
	return out
}

// EvaluateToolValue asserts the snapshot into the kernel and reads back what
// policy/tool_value.mg derives. The classes, reasons, percentages and
// proposals are the kernel's; Go adds nothing to them.
func EvaluateToolValue(k types.Kernel, s ToolLedgerSnapshot, cfg config.ToolValueConfig) (ToolValueReport, error) {
	if err := AssertToolLedger(k, s, cfg); err != nil {
		return ToolValueReport{}, err
	}
	classesRows, err := k.Query("tool_value_class")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_value_class: %w", err)
	}
	classes := pairsOf(classesRows)
	reasonsRows, err := k.Query("tool_dead_reason")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_dead_reason: %w", err)
	}
	reasons := pairsOf(reasonsRows)
	evidenceRows, err := k.Query("tool_value_evidence")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_value_evidence: %w", err)
	}
	evidence := pairsOf(evidenceRows)
	condensationRows, err := k.Query("tool_condensation_percent")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_condensation_percent: %w", err)
	}
	condensation := numbersOf(condensationRows)
	leverageRows, err := k.Query("tool_leverage_percent")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_leverage_percent: %w", err)
	}
	leverage := numbersOf(leverageRows)
	originsRows, err := k.Query("tool_origin")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_origin: %w", err)
	}
	origins := pairsOf(originsRows)
	capabilitiesRows, err := k.Query("tool_capability_declared")
	if err != nil {
		return ToolValueReport{}, fmt.Errorf("tool value: query tool_capability_declared: %w", err)
	}
	capabilities := pairsOf(capabilitiesRows)
	proposalRows, err := k.Query("tool_retirement_proposal")
	if err != nil {
		return ToolValueReport{}, err
	}

	var report ToolValueReport
	for tool, cls := range classes {
		l := s.Ledgers[tool]
		row := ToolValueRow{
			Tool:  tool,
			Class: ToolValueClass(strings.TrimPrefix(cls[0], "/")),

			Calls: l.Calls, CallSessions: l.CallSessions, Offers: l.Offers, OfferSessions: l.OfferSessions,
			PromptTokens: l.CatalogTokens + l.AtomTokens, CallTokens: l.CallTokens,
			ResultTokens: l.ResultTokens, VerifyTokens: l.VerifyTokens,
			KnownCalls: l.KnownCalls, ConsumedBytes: l.ConsumedBytes, ReturnedBytes: l.ReturnedBytes,
			Channels:  copyChannels(l.Channels),
			JevLanded: l.Jev.Landed, JevMissed: l.Jev.Missed,
		}
		row.Origin = first(origins[tool])
		row.Capability = first(capabilities[tool])
		row.DeadReasons = trimNames(reasons[tool])
		row.Evidence = trimNames(evidence[tool])
		if v, ok := condensation[tool]; ok {
			row.CondensationPercent = &v
		}
		if v, ok := leverage[tool]; ok {
			row.LeveragePercent = &v
		}
		report.Rows = append(report.Rows, row)
	}
	sort.Slice(report.Rows, func(a, b int) bool {
		ra, rb := report.Rows[a], report.Rows[b]
		if classRank(ra.Class) != classRank(rb.Class) {
			return classRank(ra.Class) < classRank(rb.Class)
		}
		return ra.Tool < rb.Tool
	})
	for _, f := range proposalRows {
		if len(f.Args) != 3 {
			continue
		}
		report.Proposals = append(report.Proposals, ToolRetirementProposal{
			Tool:   types.ExtractString(f.Args[0]),
			Kind:   strings.TrimPrefix(types.ExtractString(f.Args[1]), "/"),
			Reason: strings.TrimPrefix(types.ExtractString(f.Args[2]), "/"),
		})
	}
	sort.Slice(report.Proposals, func(a, b int) bool {
		pa, pb := report.Proposals[a], report.Proposals[b]
		if pa.Tool != pb.Tool {
			return pa.Tool < pb.Tool
		}
		return pa.Reason < pb.Reason
	})
	return report, nil
}

func classRank(c ToolValueClass) int {
	switch c {
	case ToolDead:
		return 0
	case ToolMarginal:
		return 1
	case ToolUnknown:
		return 2
	}
	return 3
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return strings.TrimPrefix(v[0], "/")
}

func trimNames(v []string) []string {
	out := make([]string, 0, len(v))
	for _, s := range v {
		out = append(out, strings.TrimPrefix(s, "/"))
	}
	sort.Strings(out)
	return out
}

// pairsOf groups a two-argument predicate's second argument by its first.
func pairsOf(rows []types.Fact) map[string][]string {
	out := map[string][]string{}
	for _, f := range rows {
		if len(f.Args) >= 2 {
			tool := types.ExtractString(f.Args[0])
			out[tool] = append(out[tool], types.ExtractString(f.Args[1]))
		}
	}
	return out
}

// numbersOf reads a (Tool, Number) predicate.
func numbersOf(rows []types.Fact) map[string]int64 {
	out := map[string]int64{}
	for _, f := range rows {
		if len(f.Args) == 2 {
			if v, ok := f.Args[1].(int64); ok {
				out[types.ExtractString(f.Args[0])] = v
			}
		}
	}
	return out
}

// Table renders the report for `nerd tools value`: one row per tool with its
// class, cost, condensation and uses; unknown measurements print as "unknown",
// never as a number.
func (r ToolValueReport) Table() string {
	var b strings.Builder
	counts := r.Counts()
	fmt.Fprintf(&b, "%d tools: %d earning, %d marginal, %d dead, %d unknown\n\n",
		len(r.Rows), counts[ToolEarning], counts[ToolMarginal], counts[ToolDead], counts[ToolUnknown])
	fmt.Fprintf(&b, "%-26s %-9s %-10s %-12s %8s %8s %10s %10s %10s\n",
		"TOOL", "CLASS", "ORIGIN", "CAPABILITY", "CALLS", "OFFERS", "COST(tok)", "CONDENSE", "LEVERAGE")
	for _, row := range r.Rows {
		fmt.Fprintf(&b, "%-26s %-9s %-10s %-12s %8d %8d %10d %10s %10s",
			row.Tool, row.Class, orDash(row.Origin), orDash(row.Capability), row.Calls, row.Offers,
			row.CostTokens(), percentOrUnknown(row.CondensationPercent), percentOrUnknown(row.LeveragePercent))
		if len(row.DeadReasons) > 0 {
			fmt.Fprintf(&b, "  dead: %s", strings.Join(row.DeadReasons, ","))
		}
		if len(row.Evidence) > 0 {
			fmt.Fprintf(&b, "  jev: %s", strings.Join(row.Evidence, ","))
			if row.JevLanded+row.JevMissed > 0 {
				fmt.Fprintf(&b, " (%d landed, %d missed)", row.JevLanded, row.JevMissed)
			}
		}
		b.WriteByte('\n')
	}
	if len(r.Proposals) > 0 {
		b.WriteString("\nRetirement proposals (nothing is retired automatically):\n")
		for _, p := range r.Proposals {
			verb := "flagged (built-in: never removed)"
			if p.Kind == "retire" {
				verb = "may be proposed for removal through the work-mode approval"
			}
			fmt.Fprintf(&b, "  %-26s %-20s %s\n", p.Tool, p.Reason, verb)
		}
	}
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func percentOrUnknown(v *int64) string {
	if v == nil {
		return "unknown"
	}
	return fmt.Sprintf("%d%%", *v)
}
