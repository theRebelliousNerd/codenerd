package northstar

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// VISION IMPORT VALIDATION (NS-VISION-01)
// =============================================================================
//
// ToFacts drops a serves/supports/addresses link whose target does not exist,
// because a dangling link would make the coverage rules silently wrong. That
// filter is a projection safeguard, not an import contract: on its own it meant
// a typo in "supports" quietly removed the link and the operator never learned
// their intent had been lost. ValidateVision is the import contract. Every
// write into the authoritative store runs it first (Store.SaveVision,
// Store.installDerivedVisionIfAbsent), so a rejected vision changes nothing:
// not the store, not the JSON/.mg export surfaces, not the kernel.
//
// Ruling: structure is rejected, spelling is not. Duplicate or empty IDs,
// unresolvable references and malformed mitigation records are rejected.
// Free-text enum spellings (priority, timeline, likelihood) stay normalised by
// the projection, because the wizard accepts free text there and rejecting it
// would refuse every vision a human typed.

// ErrVisionImportRejected marks a vision that failed ValidateVision or could not
// be parsed, and was therefore not imported. Callers branch on it with errors.Is.
var ErrVisionImportRejected = errors.New("vision import rejected")

// VisionIssue is one diagnostic: Path locates the offending field in the JSON
// import shape (e.g. "Requirements[req_x].supports[1]").
type VisionIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// VisionValidationError carries every issue found, not just the first, so an
// operator fixing an import sees the whole list in one pass.
type VisionValidationError struct {
	Issues []VisionIssue
}

func (e *VisionValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "invalid vision"
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, issue.Path+": "+issue.Message)
	}
	return fmt.Sprintf("invalid vision (%d issue(s)): %s", len(e.Issues), strings.Join(parts, "; "))
}

func (e *VisionValidationError) Unwrap() error { return ErrVisionImportRejected }

var (
	evidenceKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	commitSHAPattern    = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

// ValidateVision returns a *VisionValidationError listing every structural
// problem in v, or nil when v may become the authoritative vision.
func ValidateVision(v *Vision) error {
	if v == nil {
		return &VisionValidationError{Issues: []VisionIssue{{Path: "Vision", Message: "vision is nil"}}}
	}
	var issues []VisionIssue
	add := func(path, format string, args ...any) {
		issues = append(issues, VisionIssue{Path: path, Message: fmt.Sprintf(format, args...)})
	}

	if strings.TrimSpace(v.Mission) == "" {
		add("Mission", "mission is empty")
	}

	personas := make(map[string]struct{}, len(v.Personas))
	for i, p := range v.Personas {
		path := fmt.Sprintf("Personas[%d]", i)
		id := PersonaFactID(p.Name)
		if id == "" {
			add(path, "persona name is empty")
			continue
		}
		if _, dup := personas[id]; dup {
			add(path, "duplicate persona %q (persona IDs are derived from names)", p.Name)
		}
		personas[id] = struct{}{}
	}

	caps := idSet(len(v.Capabilities))
	for i, c := range v.Capabilities {
		caps.claim(fmt.Sprintf("Capabilities[%d]", i), c.ID, add)
	}
	risks := idSet(len(v.Risks))
	for i, r := range v.Risks {
		risks.claim(fmt.Sprintf("Risks[%d]", i), r.ID, add)
	}
	reqs := idSet(len(v.Requirements))
	for i, r := range v.Requirements {
		reqs.claim(fmt.Sprintf("Requirements[%d]", i), r.ID, add)
	}

	for _, c := range v.Capabilities {
		for j, target := range c.Serves {
			path := fmt.Sprintf("Capabilities[%s].serves[%d]", c.ID, j)
			id := PersonaFactID(target)
			if id == "" {
				add(path, "empty persona reference")
			} else if _, ok := personas[id]; !ok {
				add(path, "references unknown persona %q", target)
			}
		}
	}
	for _, r := range v.Requirements {
		for j, target := range r.Supports {
			checkRef(fmt.Sprintf("Requirements[%s].supports[%d]", r.ID, j), "capability", target, caps.ids, add)
		}
		for j, target := range r.Addresses {
			checkRef(fmt.Sprintf("Requirements[%s].addresses[%d]", r.ID, j), "risk", target, risks.ids, add)
		}
	}
	for _, r := range v.Risks {
		validateMitigation(r, add)
	}

	if len(issues) > 0 {
		return &VisionValidationError{Issues: issues}
	}
	return nil
}

type ids struct{ ids map[string]struct{} }

func idSet(n int) *ids { return &ids{ids: make(map[string]struct{}, n)} }

func (s *ids) claim(path, id string, add func(string, string, ...any)) {
	id = strings.TrimSpace(id)
	if id == "" {
		add(path, "id is empty")
		return
	}
	if _, dup := s.ids[id]; dup {
		add(path, "duplicate id %q", id)
	}
	s.ids[id] = struct{}{}
}

func checkRef(path, kind, target string, known map[string]struct{}, add func(string, string, ...any)) {
	target = strings.TrimSpace(target)
	if target == "" {
		add(path, "empty %s reference", kind)
		return
	}
	if _, ok := known[target]; !ok {
		add(path, "references unknown %s %q", kind, target)
	}
}

func validateMitigation(r Risk, add func(string, string, ...any)) {
	path := fmt.Sprintf("Risks[%s]", r.ID)
	status := r.MitigationStatus
	if status != "" && !status.valid() {
		add(path+".mitigation_status", "unknown status %q (want proposed, implemented, verified or waived)", status)
		return
	}
	effective := r.EffectiveMitigationStatus()
	if (effective == MitigationImplemented || effective == MitigationVerified) && strings.TrimSpace(r.Mitigation) == "" {
		add(path+".mitigation", "status %s requires the mitigation it refers to", effective)
	}
	if effective == MitigationVerified && len(r.MitigationEvidence) == 0 {
		add(path+".mitigation_evidence", "status verified requires at least one evidence record")
	}
	for i, ev := range r.MitigationEvidence {
		evPath := fmt.Sprintf("%s.mitigation_evidence[%d]", path, i)
		if !evidenceKindPattern.MatchString(normalizeEnumWord(ev.Kind)) {
			add(evPath+".kind", "kind %q must be a lowercase word such as test, review or witness", ev.Kind)
		}
		if strings.TrimSpace(ev.Ref) == "" {
			add(evPath+".ref", "ref is empty")
		}
		if !ev.Outcome.valid() {
			add(evPath+".outcome", "unknown outcome %q (want passed, failed, skipped, cancelled or unknown)", ev.Outcome)
		}
		if !commitSHAPattern.MatchString(strings.TrimSpace(ev.Revision)) {
			add(evPath+".revision", "revision %q must be a full lowercase commit SHA", ev.Revision)
		}
	}
	switch {
	case effective == MitigationWaived && r.Waiver == nil:
		add(path+".waiver", "status waived requires a waiver record (authority, scope, rationale, expires)")
	case effective != MitigationWaived && r.Waiver != nil:
		add(path+".waiver", "waiver present but mitigation_status is %q, not waived", effective)
	case r.Waiver != nil:
		w := r.Waiver
		if strings.TrimSpace(w.Authority) == "" {
			add(path+".waiver.authority", "authority is empty")
		}
		if strings.TrimSpace(w.Scope) == "" {
			add(path+".waiver.scope", "scope is empty")
		}
		if strings.TrimSpace(w.Rationale) == "" {
			add(path+".waiver.rationale", "rationale is empty")
		}
		if w.Expires.IsZero() {
			add(path+".waiver.expires", "expires is required; a waiver without expiry never lapses")
		}
	}
}
