package campaign

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	internaltypes "codenerd/internal/types"
)

// Hard vs soft advisory blocking contract.
//
// The decision, recorded here because this is where it is enforced:
//
//   - Go MEASURES the preflight. It runs the strict gates, collects their
//     outcomes and the graded severities behind them, and asserts all of it as
//     ground facts (campaign_risk_gate_outcome, campaign_risk_concern,
//     campaign_protected_surface, campaign_risk_posture, campaign_risk_signal,
//     campaign_risk_override).
//   - The KERNEL DECIDES which of those measurements stops a campaign. The
//     rules live in internal/core/defaults/campaign_rules.mg Section 13 and
//     derive campaign_risk_block (hard) and campaign_risk_warning (soft).
//   - Go then ENFORCES what the kernel derived: a hard block aborts Run with a
//     RiskBlockedError; a soft warning is emitted, recorded, and execution
//     continues.
//
// Why the distinction exists at all: before this, every blocked gate aborted
// the campaign identically, so "the edge detector would prefer some prework on
// a leaf file" and "a critical advisor rejected a rewrite of the logic kernel"
// produced the same refusal. Operators respond to that by disabling gates.
// Grading the outcomes keeps the hard path credible.
//
// The kernel is the authority, not a suggestion. classifyRiskGateResults only
// falls back to the Go mirror below when the readiness canary
// (campaign_risk_classification_ready) does not derive, which means
// campaign_rules.mg is not loaded into this kernel at all. Silently treating
// "no rules" as "no blocks" would turn a missing policy file into a green
// light, so the fallback reproduces the same contract in Go.

// RiskBlockSeverity grades a preflight finding.
type RiskBlockSeverity string

const (
	// RiskSeverityHard stops the campaign before any task runs.
	RiskSeverityHard RiskBlockSeverity = "/hard"
	// RiskSeveritySoft is surfaced to the operator; the campaign proceeds.
	RiskSeveritySoft RiskBlockSeverity = "/soft"
)

// Concern severities asserted for the kernel to grade.
const (
	riskConcernBlocking        = "/blocking"
	riskConcernRequiresChanges = "/requires_changes"
	riskConcernUnapproved      = "/unapproved"
)

// Signals pinned for the kernel's critical-evidence rules.
const (
	riskSignalSafetyWarnings = "/safety_warnings"
	riskSignalBlockedActions = "/blocked_actions"
	riskSignalGatheringErrs  = "/gathering_errors"
	riskSignalToolGaps       = "/tool_gaps"
)

// riskGateOverride is the pseudo-gate name used for operator overrides.
const riskGateOverride RiskGateName = "/override"

// ErrRiskGateBlocked is the sentinel every hard preflight block wraps, so
// callers can branch on it with errors.Is without string matching.
var ErrRiskGateBlocked = errors.New("risk gate blocked campaign start")

// RiskFinding is one graded preflight finding as the kernel classified it.
type RiskFinding struct {
	Gate     RiskGateName      `json:"gate"`
	Reason   string            `json:"reason"`
	Severity RiskBlockSeverity `json:"severity"`
	// Detail carries the human-readable text the gate produced, which the
	// kernel does not see (it grades atoms, not prose).
	Detail string `json:"detail,omitzero"`
}

// RiskBlockedError reports a hard preflight block. It carries the full
// evaluation so CLI and chat can render exactly which gate stopped the run and
// why, instead of only writing it to the campaign log category.
type RiskBlockedError struct {
	CampaignID string
	Evaluation *RiskGateEvaluation
}

func (e *RiskBlockedError) Error() string {
	if e == nil || e.Evaluation == nil {
		return ErrRiskGateBlocked.Error()
	}
	return fmt.Sprintf("%s (%s): %s", ErrRiskGateBlocked.Error(), e.Evaluation.BlockedBy, e.Evaluation.BlockReason)
}

func (e *RiskBlockedError) Unwrap() error { return ErrRiskGateBlocked }

// RiskEvaluation returns the evaluation behind a risk block, or nil when err is
// not a risk block. Operator surfaces use this to render the full gate report.
func RiskEvaluation(err error) *RiskGateEvaluation {
	var blocked *RiskBlockedError
	if errors.As(err, &blocked) && blocked != nil {
		return blocked.Evaluation
	}
	return nil
}

// FormatRiskBlock renders a preflight block for a terminal. It returns false
// when err is not a risk block, so callers can fall through to generic error
// handling.
//
// This exists because the block used to be visible only as a CategoryCampaign
// log line plus a wrapped error string. An operator running `nerd campaign
// start` saw "risk gate blocked campaign start (/advisory): ..." with no
// indication of which gates ran, what the score was, or what to do next.
func FormatRiskBlock(err error) (string, bool) {
	eval := RiskEvaluation(err)
	if eval == nil {
		return "", false
	}

	var sb strings.Builder
	sb.WriteString("Campaign refused before any task ran.\n\n")
	sb.WriteString(fmt.Sprintf("  Blocked by : %s\n", eval.BlockedBy))
	sb.WriteString(fmt.Sprintf("  Reason     : %s\n", eval.BlockReason))
	if eval.Decision != nil {
		sb.WriteString(fmt.Sprintf("  Risk score : %d (threshold %d, %s)\n",
			eval.Decision.Score, eval.Decision.Threshold, eval.Decision.TieBreak))
	}
	if len(eval.ProtectedRoots) > 0 {
		sb.WriteString(fmt.Sprintf("  Protected  : %s\n", strings.Join(eval.ProtectedRoots, ", ")))
	}

	if len(eval.Results) > 0 {
		sb.WriteString("\n  Gates:\n")
		for _, r := range eval.Results {
			state := "skipped"
			switch r.Outcome {
			case RiskGateOutcomePassed:
				state = "passed"
			case RiskGateOutcomeBlocked:
				state = "BLOCKED"
			}
			sb.WriteString(fmt.Sprintf("    %-11s %-8s %s\n", r.Name, state, truncateForDisplay(r.Reason, 120)))
		}
	}

	if len(eval.Findings) > 0 {
		sb.WriteString("\n  Kernel classification:\n")
		for _, f := range eval.Findings {
			sb.WriteString(fmt.Sprintf("    [%s] %s %s\n", strings.TrimPrefix(string(f.Severity), "/"), f.Gate, f.Reason))
		}
	}

	sb.WriteString("\n  Soft findings are advisory and do not stop a campaign; hard findings do.\n")
	sb.WriteString("  Override with --risk-gate force_allow only when you understand the finding.\n")
	return sb.String(), true
}

func truncateForDisplay(s string, limit int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

// riskContractFacts is everything Go measured, ready for the kernel to grade.
type riskContractFacts struct {
	campaignID     string
	gateOutcomes   []RiskGateResult
	concerns       []riskConcern
	protectedRoots []string
	decision       *CampaignRiskDecision
	override       string
}

type riskConcern struct {
	gate     RiskGateName
	severity string
	detail   string
}

// riskContractPredicates are retracted before each preflight so a re-run never
// grades against a previous run's measurements.
var riskContractPredicates = []string{
	"campaign_risk_gate_outcome",
	"campaign_risk_concern",
	"campaign_protected_surface",
	"campaign_risk_posture",
	"campaign_risk_signal",
	"campaign_risk_override",
}

// assertRiskContractFacts publishes the measurements to the kernel.
func (o *Orchestrator) assertRiskContractFacts(m riskContractFacts) {
	if o.kernel == nil || m.campaignID == "" {
		return
	}

	for _, pred := range riskContractPredicates {
		if err := o.retractCampaignScopedFacts(pred, m.campaignID); err != nil {
			logging.CampaignWarn("failed to clear %s before risk preflight: %v", pred, err)
		}
	}

	facts := make([]core.Fact, 0, 16)
	for _, r := range m.gateOutcomes {
		facts = append(facts, core.Fact{
			Predicate: "campaign_risk_gate_outcome",
			Args:      []any{m.campaignID, string(r.Name), string(r.Outcome)},
		})
	}
	for _, c := range m.concerns {
		facts = append(facts, core.Fact{
			Predicate: "campaign_risk_concern",
			Args:      []any{m.campaignID, string(c.gate), c.severity},
		})
	}
	for _, root := range m.protectedRoots {
		facts = append(facts, core.Fact{
			Predicate: "campaign_protected_surface",
			Args:      []any{m.campaignID, root},
		})
	}
	if m.override != "" {
		facts = append(facts, core.Fact{
			Predicate: "campaign_risk_override",
			Args:      []any{m.campaignID, m.override},
		})
	}
	if m.decision != nil {
		gated := "/false"
		if m.decision.Gated {
			gated = "/true"
		}
		facts = append(facts,
			core.Fact{
				Predicate: "campaign_risk_posture",
				Args:      []any{m.campaignID, m.decision.Score, m.decision.Threshold, gated},
			},
			core.Fact{
				Predicate: "campaign_risk_signal",
				Args:      []any{m.campaignID, riskSignalSafetyWarnings, m.decision.Inputs.SafetyWarnings},
			},
			core.Fact{
				Predicate: "campaign_risk_signal",
				Args:      []any{m.campaignID, riskSignalBlockedActions, m.decision.Inputs.BlockedActions},
			},
			core.Fact{
				Predicate: "campaign_risk_signal",
				Args:      []any{m.campaignID, riskSignalGatheringErrs, m.decision.Inputs.GatheringErrors},
			},
			core.Fact{
				Predicate: "campaign_risk_signal",
				Args:      []any{m.campaignID, riskSignalToolGaps, m.decision.Inputs.ToolGaps},
			},
		)
	}

	if len(facts) == 0 {
		return
	}
	if err := o.kernel.LoadFacts(facts); err != nil {
		logging.CampaignWarn("failed to load risk preflight facts: %v", err)
	}
}

// retractCampaignScopedFacts removes every fact of pred whose first argument is
// campaignID. The kernel has no pattern retraction, so this reads and removes.
func (o *Orchestrator) retractCampaignScopedFacts(pred, campaignID string) error {
	facts, err := o.kernel.Query(pred)
	if err != nil {
		// An unknown predicate is not an error worth reporting: it simply means
		// nothing was asserted yet.
		return nil
	}
	for _, f := range facts {
		if len(f.Args) == 0 || internaltypes.ExtractString(f.Args[0]) != campaignID {
			continue
		}
		if rerr := o.kernel.RetractFact(f); rerr != nil {
			return rerr
		}
	}
	return nil
}

// classifyRiskGateResults asks the kernel to grade the measurements and returns
// the hard and soft findings. kernelDecided reports whether the classification
// came from the kernel rules or from the Go mirror.
func (o *Orchestrator) classifyRiskGateResults(m riskContractFacts) (findings []RiskFinding, kernelDecided bool) {
	o.assertRiskContractFacts(m)

	if o.kernelRiskRulesLoaded(m.campaignID) {
		hard := o.queryRiskFindings("campaign_risk_block", m.campaignID, RiskSeverityHard)
		soft := o.queryRiskFindings("campaign_risk_warning", m.campaignID, RiskSeveritySoft)
		findings = append(hard, soft...)
		annotateRiskFindings(findings, m)
		sortRiskFindings(findings)
		return findings, true
	}

	logging.CampaignWarn("campaign risk classification rules not loaded in kernel; using Go mirror of the hard/soft contract")
	findings = mirrorRiskClassification(m)
	sortRiskFindings(findings)
	return findings, false
}

func (o *Orchestrator) kernelRiskRulesLoaded(campaignID string) bool {
	if o.kernel == nil {
		return false
	}
	facts, err := o.kernel.Query("campaign_risk_classification_ready")
	if err != nil {
		return false
	}
	for _, f := range facts {
		if len(f.Args) > 0 && internaltypes.ExtractString(f.Args[0]) == campaignID {
			return true
		}
	}
	return false
}

func (o *Orchestrator) queryRiskFindings(pred, campaignID string, severity RiskBlockSeverity) []RiskFinding {
	facts, err := o.kernel.Query(pred)
	if err != nil {
		logging.CampaignWarn("failed to query %s: %v", pred, err)
		return nil
	}
	seen := make(map[string]struct{}, len(facts))
	out := make([]RiskFinding, 0, len(facts))
	for _, f := range facts {
		if len(f.Args) < 3 || internaltypes.ExtractString(f.Args[0]) != campaignID {
			continue
		}
		gate := internaltypes.ExtractString(f.Args[1])
		reason := internaltypes.ExtractString(f.Args[2])
		key := gate + "|" + reason
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, RiskFinding{
			Gate:     RiskGateName(gate),
			Reason:   reason,
			Severity: severity,
		})
	}
	return out
}

// annotateRiskFindings attaches the gate's own prose to each finding. The
// kernel grades atoms; the operator needs the sentence the gate produced.
func annotateRiskFindings(findings []RiskFinding, m riskContractFacts) {
	detail := make(map[RiskGateName]string, len(m.gateOutcomes))
	for _, r := range m.gateOutcomes {
		if r.Outcome == RiskGateOutcomeBlocked {
			detail[r.Name] = r.Reason
		}
	}
	for _, c := range m.concerns {
		if _, ok := detail[c.gate]; !ok && c.detail != "" {
			detail[c.gate] = c.detail
		}
	}
	for i := range findings {
		findings[i].Detail = detail[findings[i].Gate]
	}
}

func sortRiskFindings(findings []RiskFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity == RiskSeverityHard
		}
		if findings[i].Gate != findings[j].Gate {
			return riskGatePrecedence(findings[i].Gate) < riskGatePrecedence(findings[j].Gate)
		}
		return findings[i].Reason < findings[j].Reason
	})
}

func riskGatePrecedence(name RiskGateName) int {
	switch name {
	case riskGateOverride:
		return 0
	case RiskGateNorthstar:
		return 1
	case RiskGateEdge:
		return 2
	case RiskGateAdvisory:
		return 3
	default:
		return 4
	}
}

// mirrorRiskClassification reproduces campaign_rules.mg Section 13 in Go. It is
// only reached when the kernel has no campaign risk rules loaded; keeping the
// two in step is enforced by TestRiskClassification_KernelAndMirror_ShouldAgree.
func mirrorRiskClassification(m riskContractFacts) []RiskFinding {
	hard := make(map[RiskGateName]string)
	blocked := make([]RiskGateName, 0, len(m.gateOutcomes))
	for _, r := range m.gateOutcomes {
		if r.Outcome == RiskGateOutcomeBlocked {
			blocked = append(blocked, r.Name)
		}
	}

	critical := false
	gatedOverThreshold := false
	if m.decision != nil {
		critical = m.decision.Inputs.SafetyWarnings > 0 || m.decision.Inputs.BlockedActions > 0
		gatedOverThreshold = m.decision.Gated
	}

	for _, gate := range blocked {
		switch {
		case len(m.protectedRoots) > 0:
			hard[gate] = "/protected_surface"
		case gate == RiskGateNorthstar:
			hard[gate] = "/vision_alignment"
		case gatedOverThreshold && critical:
			hard[gate] = "/gated_with_critical_signals"
		}
	}
	for _, c := range m.concerns {
		if c.severity == riskConcernBlocking {
			hard[c.gate] = "/critical_advisor_rejection"
		}
	}
	if m.override == string(RiskGateModeForceBlock) {
		hard[riskGateOverride] = "/force_block"
	}

	findings := make([]RiskFinding, 0, len(hard)+len(blocked))
	for gate, reason := range hard {
		findings = append(findings, RiskFinding{Gate: gate, Reason: reason, Severity: RiskSeverityHard})
	}
	for _, gate := range blocked {
		if _, isHard := hard[gate]; !isHard {
			findings = append(findings, RiskFinding{Gate: gate, Reason: "/advisory_only", Severity: RiskSeveritySoft})
		}
	}
	for _, c := range m.concerns {
		if _, isHard := hard[c.gate]; isHard {
			continue
		}
		switch c.severity {
		case riskConcernRequiresChanges:
			findings = append(findings, RiskFinding{Gate: c.gate, Reason: "/requires_changes", Severity: RiskSeveritySoft})
		case riskConcernUnapproved:
			findings = append(findings, RiskFinding{Gate: c.gate, Reason: "/unapproved", Severity: RiskSeveritySoft})
		}
	}
	annotateRiskFindings(findings, m)
	return findings
}

func hardRiskFindings(findings []RiskFinding) []RiskFinding {
	out := make([]RiskFinding, 0, len(findings))
	for _, f := range findings {
		if f.Severity == RiskSeverityHard {
			out = append(out, f)
		}
	}
	return out
}

func softRiskFindings(findings []RiskFinding) []RiskFinding {
	out := make([]RiskFinding, 0, len(findings))
	for _, f := range findings {
		if f.Severity == RiskSeveritySoft {
			out = append(out, f)
		}
	}
	return out
}
