package northstar

import (
	"strings"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// MITIGATION STATE AND EVIDENCE
// =============================================================================
//
// DECISION (northstar blueprint section 9, NS-VISION-03).
//
// A risk's mitigation used to be one string, and has_mitigation/1 fired on the
// string's existence. Writing "we will add tests" therefore silenced the
// unmitigated-high-risk warning exactly as well as shipping and verifying the
// control did. The model now separates three things:
//
//	Mitigation          prose: what the operator intends (unchanged)
//	MitigationStatus    proposed | implemented | verified | waived
//	MitigationEvidence  typed, revision-bound check outcomes
//
// Only a /verified mitigation with /passed evidence bound to the CURRENT git
// HEAD derives has_mitigation (prompt_northstar.mg). A new commit makes that
// evidence stale and the risk re-surfaces until someone re-verifies it; a
// skipped, failed or unknown check never derives it. A waiver is not a
// mitigation: an active, unexpired waiver suppresses the unmitigated warning
// but derives nothing that reads as "mitigated".

// MitigationStatus is the lifecycle state of a risk's mitigation.
type MitigationStatus string

const (
	// MitigationProposed is prose only. It is the effective state of any risk
	// that has mitigation text and no explicit status.
	MitigationProposed MitigationStatus = "proposed"
	// MitigationImplemented claims the control exists but nothing verified it.
	MitigationImplemented MitigationStatus = "implemented"
	// MitigationVerified claims the control was checked; the claim only holds
	// while it has fresh /passed evidence.
	MitigationVerified MitigationStatus = "verified"
	// MitigationWaived records an authorized decision to accept the risk. It is
	// never relabeled mitigated or passed.
	MitigationWaived MitigationStatus = "waived"
)

func (s MitigationStatus) valid() bool {
	switch s {
	case MitigationProposed, MitigationImplemented, MitigationVerified, MitigationWaived:
		return true
	default:
		return false
	}
}

// EvidenceOutcome is the outcome of one check offered as mitigation evidence.
type EvidenceOutcome string

const (
	EvidencePassed    EvidenceOutcome = "passed"
	EvidenceFailed    EvidenceOutcome = "failed"
	EvidenceSkipped   EvidenceOutcome = "skipped"
	EvidenceCancelled EvidenceOutcome = "cancelled"
	EvidenceUnknown   EvidenceOutcome = "unknown"
)

func (o EvidenceOutcome) valid() bool {
	switch o {
	case EvidencePassed, EvidenceFailed, EvidenceSkipped, EvidenceCancelled, EvidenceUnknown:
		return true
	default:
		return false
	}
}

// MitigationEvidence is one typed observation offered in support of a
// mitigation. Revision is the full commit SHA the check ran against; the
// kernel compares it with git_state(/head, Sha) so evidence from an older
// revision cannot keep a mitigation verified.
type MitigationEvidence struct {
	Kind     string          `json:"kind"` // e.g. "test", "review", "witness"
	Ref      string          `json:"ref"`  // what was run or inspected
	Outcome  EvidenceOutcome `json:"outcome"`
	Revision string          `json:"revision"`
}

// RiskWaiver is the separate record a waived risk requires: who accepted the
// risk, for what scope, why, and until when.
type RiskWaiver struct {
	Authority string    `json:"authority"`
	Scope     string    `json:"scope"`
	Rationale string    `json:"rationale"`
	Expires   time.Time `json:"expires"`
}

// EffectiveMitigationStatus is the status the kernel sees: an explicit status,
// else proposed when there is mitigation prose, else "" (nothing to project).
func (r Risk) EffectiveMitigationStatus() MitigationStatus {
	if r.MitigationStatus != "" {
		return r.MitigationStatus
	}
	if strings.TrimSpace(r.Mitigation) != "" {
		return MitigationProposed
	}
	return ""
}

// mitigationFacts projects the risk's mitigation state, evidence and waiver.
func (r Risk) mitigationFacts() []types.Fact {
	status := r.EffectiveMitigationStatus()
	if status == "" {
		return nil
	}
	facts := []types.Fact{{
		Predicate: "northstar_mitigation_state",
		Args:      []any{r.ID, types.MangleAtom("/" + string(status))},
	}}
	for _, ev := range r.MitigationEvidence {
		facts = append(facts, types.Fact{
			Predicate: "northstar_mitigation_evidence",
			Args: []any{
				r.ID,
				enumAtom(ev.Kind),
				types.MangleString(ev.Ref),
				enumAtom(string(ev.Outcome)),
				types.MangleString(strings.TrimSpace(ev.Revision)),
			},
		})
	}
	if r.Waiver != nil {
		facts = append(facts, types.Fact{
			Predicate: "northstar_risk_waiver",
			Args: []any{
				r.ID,
				types.MangleString(r.Waiver.Authority),
				types.MangleString(r.Waiver.Scope),
				types.MangleString(r.Waiver.Rationale),
				r.Waiver.Expires.Unix(),
			},
		})
	}
	return facts
}

func cloneRisk(r Risk) Risk {
	c := r
	c.MitigationEvidence = append([]MitigationEvidence(nil), r.MitigationEvidence...)
	if r.Waiver != nil {
		w := *r.Waiver
		c.Waiver = &w
	}
	return c
}
