package northstar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// =============================================================================
// LOSSLESS, BASE-BOUND VISION EDITING (NS-EDIT-01)
// =============================================================================
//
// An interactive editor shows a bounded view of the vision (the wizard shows
// text, not mitigation evidence or capability links). Writing that view back
// as the vision loses everything the view does not display, and writing it
// over an authority that moved since it was read loses the other writer's
// work. The contract here has three parts:
//
//	MergeViewEdit        absent view fields mean "not edited"; entities are
//	                     matched by stable ID, never by position, so a view
//	                     written back unchanged is the base, byte for byte.
//	VisionDigest         the generation of the authority is its own content,
//	                     so every writer (wizard, CLI import, accepted file
//	                     reconciliation, derived install) advances it without
//	                     having to remember to.
//	UpdateVisionFromBase the compare of the editor's base against the stored
//	                     content happens inside the authority's write
//	                     transaction; a stale base is refused with a semantic
//	                     comparison and nothing is written.

// NoVisionDigest is the digest of "no vision defined". An editor that started
// from an empty authority passes it, and the save is refused if a vision has
// appeared since.
const NoVisionDigest = ""

// VisionDigest identifies the content of a vision: the SHA-256 of its
// normalized JSON, excluding timestamps (they are bookkeeping that every save
// changes). A nil vision has NoVisionDigest.
func VisionDigest(v *Vision) string {
	if v == nil {
		return NoVisionDigest
	}
	raw, err := json.Marshal(normalizeVision(v))
	if err != nil {
		// Vision holds only strings, numbers, times and nested structs of the
		// same; a marshal failure is a programming error, and an unmatchable
		// digest makes the compare fail closed.
		return "unmarshalable:" + err.Error()
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// BaseStaleError reports that the stored vision no longer has the content an
// edit was based on. It is a conflict, not a validation failure: the candidate
// may be perfectly valid, it is just no longer an edit of the current vision.
type BaseStaleError struct {
	Expected string
	Actual   string
	// Changes lists, in words, what changed in the authority since the editor
	// read it. It is empty when the comparison could not be computed.
	Changes []string
}

// ErrVisionBaseStale is what errors.Is matches a *BaseStaleError against.
var ErrVisionBaseStale = errors.New("vision changed since it was read")

func (e *BaseStaleError) Error() string {
	msg := fmt.Sprintf("%s (base %s, now %s)", ErrVisionBaseStale, shortDigest(e.Expected), shortDigest(e.Actual))
	if len(e.Changes) > 0 {
		msg += ": " + strings.Join(e.Changes, "; ")
	}
	return msg
}

func (e *BaseStaleError) Unwrap() error { return ErrVisionBaseStale }

func shortDigest(d string) string {
	if d == NoVisionDigest {
		return "none"
	}
	if len(d) > 12 {
		return d[:12]
	}
	return d
}

// ErrReassessmentRequired marks an edit that changes the subject of a risk's
// verified or evidenced mitigation without saying what the status now is. The
// evidence was gathered against the old text; carrying a verified label onto
// different text would launder it.
var ErrReassessmentRequired = errors.New("mitigation needs reassessment")

// MergeViewEdit applies an edited view onto base and returns the candidate
// vision. base may be nil (a vision being created); it is never modified.
//
// Rules, all of which follow from "absent means not edited":
//   - Capabilities, risks and requirements are matched to base entities by ID
//     (requirements case-insensitively). A base entity the view does not
//     mention is kept.
//   - A matched entity keeps every field the view leaves zero: capability
//     serves, risk mitigation status/evidence/waiver, requirement supports,
//     addresses and source.
//   - A view entity with no ID is new and is given the lowest free
//     cap_/risk_/req_ number; IDs of existing entities never change.
//   - nil personas or constraints, and empty mission/problem/vision text, are
//     not edits.
//   - Changing a risk's description or mitigation prose while it has verified
//     status or evidence requires the view to state a new status.
//
// The result is not validated; UpdateVisionFromBase does that before writing.
func MergeViewEdit(base *Vision, view *WizardDocument) (*Vision, error) {
	if view == nil {
		return nil, fmt.Errorf("view is nil")
	}
	var out *Vision
	if base != nil {
		out = cloneVision(base)
	} else {
		out = &Vision{}
	}

	if s := strings.TrimSpace(view.Mission); s != "" {
		out.Mission = s
	}
	if s := strings.TrimSpace(view.Problem); s != "" {
		out.Problem = s
	}
	if s := strings.TrimSpace(view.Vision); s != "" {
		out.VisionStmt = s
	}
	if view.Personas != nil {
		out.Personas = make([]Persona, 0, len(view.Personas))
		for _, p := range view.Personas {
			out.Personas = append(out.Personas, Persona{
				Name:       p.Name,
				PainPoints: append([]string(nil), p.PainPoints...),
				Needs:      append([]string(nil), p.Needs...),
			})
		}
	}
	if view.Constraints != nil {
		out.Constraints = append([]string{}, view.Constraints...)
	}

	capIDs, riskIDs, reqIDs := newIDPool(), newIDPool(), newIDPool()
	for _, c := range out.Capabilities {
		capIDs.reserve(c.ID)
	}
	for _, r := range out.Risks {
		riskIDs.reserve(r.ID)
	}
	for _, r := range out.Requirements {
		reqIDs.reserve(r.ID)
	}
	for _, c := range view.Capabilities {
		capIDs.reserve(c.ID)
	}
	for _, r := range view.Risks {
		riskIDs.reserve(r.ID)
	}
	for _, r := range view.Requirements {
		reqIDs.reserve(r.ID)
	}

	if err := mergeCapabilities(out, view.Capabilities, capIDs); err != nil {
		return nil, err
	}
	if err := mergeRisks(out, view.Risks, riskIDs); err != nil {
		return nil, err
	}
	if err := mergeRequirements(out, view.Requirements, reqIDs); err != nil {
		return nil, err
	}
	if out.Personas == nil {
		out.Personas = []Persona{}
	}
	if out.Constraints == nil {
		out.Constraints = []string{}
	}
	return out, nil
}

func mergeCapabilities(out *Vision, view []WizardCapability, pool *idPool) error {
	index := make(map[string]int, len(out.Capabilities))
	for i, c := range out.Capabilities {
		index[c.ID] = i
	}
	seen := make(map[string]struct{}, len(view))
	for _, vc := range view {
		id := strings.TrimSpace(vc.ID)
		if id == "" {
			id = pool.mint("cap_")
		} else if _, dup := seen[id]; dup {
			return fmt.Errorf("view lists capability %q twice", id)
		}
		seen[id] = struct{}{}
		i, known := index[id]
		if !known {
			out.Capabilities = append(out.Capabilities, Capability{
				ID: id, Description: vc.Description, Timeline: vc.Timeline, Priority: vc.Priority,
				Serves: append([]string(nil), vc.Serves...),
			})
			index[id] = len(out.Capabilities) - 1
			continue
		}
		c := &out.Capabilities[i]
		c.Description, c.Timeline, c.Priority = vc.Description, vc.Timeline, vc.Priority
		if vc.Serves != nil {
			c.Serves = append([]string(nil), vc.Serves...)
		}
	}
	return nil
}

func mergeRisks(out *Vision, view []WizardRisk, pool *idPool) error {
	index := make(map[string]int, len(out.Risks))
	for i, r := range out.Risks {
		index[r.ID] = i
	}
	seen := make(map[string]struct{}, len(view))
	for _, vr := range view {
		id := strings.TrimSpace(vr.ID)
		if id == "" {
			id = pool.mint("risk_")
		} else if _, dup := seen[id]; dup {
			return fmt.Errorf("view lists risk %q twice", id)
		}
		seen[id] = struct{}{}
		i, known := index[id]
		if !known {
			risk := cloneRisk(Risk(vr))
			risk.ID = id
			risk.MitigationStatus = MitigationStatus(normalizeEnumWord(string(vr.MitigationStatus)))
			out.Risks = append(out.Risks, risk)
			index[id] = len(out.Risks) - 1
			continue
		}
		r := &out.Risks[i]
		newStatus := MitigationStatus(normalizeEnumWord(string(vr.MitigationStatus)))
		subjectChanged := strings.TrimSpace(vr.Description) != strings.TrimSpace(r.Description) ||
			strings.TrimSpace(vr.Mitigation) != strings.TrimSpace(r.Mitigation)
		evidenced := r.EffectiveMitigationStatus() == MitigationVerified || len(r.MitigationEvidence) > 0
		reassessed := newStatus != "" && newStatus != r.MitigationStatus
		if subjectChanged && evidenced && !reassessed {
			return fmt.Errorf("risk %s: %w: its description or mitigation changed while it is %s with %d evidence record(s); state the new mitigation status in the same edit",
				id, ErrReassessmentRequired, r.EffectiveMitigationStatus(), len(r.MitigationEvidence))
		}
		r.Description, r.Likelihood, r.Impact, r.Mitigation = vr.Description, vr.Likelihood, vr.Impact, vr.Mitigation
		if newStatus != "" {
			r.MitigationStatus = newStatus
		}
		if len(vr.MitigationEvidence) > 0 {
			r.MitigationEvidence = append([]MitigationEvidence(nil), vr.MitigationEvidence...)
		}
		if vr.Waiver != nil {
			w := *vr.Waiver
			r.Waiver = &w
		}
	}
	return nil
}

func mergeRequirements(out *Vision, view []WizardRequirement, pool *idPool) error {
	index := make(map[string]int, len(out.Requirements))
	for i, r := range out.Requirements {
		index[r.ID] = i
	}
	seen := make(map[string]struct{}, len(view))
	for _, vr := range view {
		id := strings.ToLower(strings.TrimSpace(vr.ID))
		if id == "" {
			id = pool.mint("req_")
		} else if _, dup := seen[id]; dup {
			return fmt.Errorf("view lists requirement %q twice", id)
		}
		seen[id] = struct{}{}
		i, known := index[id]
		if !known {
			out.Requirements = append(out.Requirements, Requirement{
				ID: id, Type: vr.Type, Description: vr.Description, Priority: vr.Priority, Source: vr.Source,
				Supports:  append([]string(nil), vr.Supports...),
				Addresses: append([]string(nil), vr.Addresses...),
			})
			index[id] = len(out.Requirements) - 1
			continue
		}
		r := &out.Requirements[i]
		r.Type, r.Description, r.Priority = vr.Type, vr.Description, vr.Priority
		if vr.Source != "" {
			r.Source = vr.Source
		}
		if vr.Supports != nil {
			r.Supports = append([]string(nil), vr.Supports...)
		}
		if vr.Addresses != nil {
			r.Addresses = append([]string(nil), vr.Addresses...)
		}
	}
	return nil
}

// idPool hands out the lowest free <prefix><n> ID. Existing IDs are reserved
// case-insensitively so a minted ID can never collide with, and thereby
// silently overwrite, an existing entity.
type idPool struct{ taken map[string]struct{} }

func newIDPool() *idPool { return &idPool{taken: map[string]struct{}{}} }

func (p *idPool) reserve(id string) {
	if id = strings.ToLower(strings.TrimSpace(id)); id != "" {
		p.taken[id] = struct{}{}
	}
}

func (p *idPool) mint(prefix string) string {
	for n := 1; ; n++ {
		id := fmt.Sprintf("%s%d", prefix, n)
		if _, used := p.taken[id]; !used {
			p.taken[id] = struct{}{}
			return id
		}
	}
}

// MintID returns the lowest <prefix><n> not present in existing (compared
// case-insensitively). Editors use it to give a new entity its ID when it is
// created, so links to it can be written before the vision is saved.
func MintID(prefix string, existing []string) string {
	pool := newIDPool()
	for _, id := range existing {
		pool.reserve(id)
	}
	return pool.mint(prefix)
}

// DiffVisions lists, in words, the semantic differences between two visions:
// scalar fields, and entities by ID. It is for people reading a conflict or a
// commit receipt; nothing branches on its text.
func DiffVisions(from, to *Vision) []string {
	switch {
	case from == nil && to == nil:
		return nil
	case from == nil:
		return []string{"vision created"}
	case to == nil:
		return []string{"vision removed"}
	}
	a, b := normalizeVision(from), normalizeVision(to)
	from, to = &a, &b
	var out []string
	note := func(format string, args ...any) { out = append(out, fmt.Sprintf(format, args...)) }
	if from.Mission != to.Mission {
		note("mission changed")
	}
	if from.Problem != to.Problem {
		note("problem changed")
	}
	if from.VisionStmt != to.VisionStmt {
		note("vision statement changed")
	}
	if !equalJSON(from.Personas, to.Personas) {
		note("personas changed")
	}
	if !equalJSON(from.Constraints, to.Constraints) {
		note("constraints changed")
	}
	diffByID(&out, "capability", from.Capabilities, to.Capabilities, func(c Capability) string { return c.ID })
	diffByID(&out, "risk", from.Risks, to.Risks, func(r Risk) string { return r.ID })
	diffByID(&out, "requirement", from.Requirements, to.Requirements, func(r Requirement) string { return r.ID })
	return out
}

func diffByID[T any](out *[]string, kind string, from, to []T, id func(T) string) {
	fromByID := make(map[string]T, len(from))
	for _, e := range from {
		fromByID[id(e)] = e
	}
	toIDs := make(map[string]struct{}, len(to))
	for _, e := range to {
		toIDs[id(e)] = struct{}{}
		old, existed := fromByID[id(e)]
		switch {
		case !existed:
			*out = append(*out, fmt.Sprintf("%s %s added", kind, id(e)))
		case !equalJSON(old, e):
			*out = append(*out, fmt.Sprintf("%s %s changed", kind, id(e)))
		}
	}
	for _, e := range from {
		if _, kept := toIDs[id(e)]; !kept {
			*out = append(*out, fmt.Sprintf("%s %s removed", kind, id(e)))
		}
	}
}

func equalJSON(a, b any) bool {
	x, errA := json.Marshal(a)
	y, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(x) == string(y)
}

// VisionOutcome is what became of an UpdateVisionFromBase call that was not
// rejected or refused.
type VisionOutcome string

const (
	// VisionUnchanged: the candidate equals the stored vision. Nothing was
	// written: not the store, not the exports, not the kernel.
	VisionUnchanged VisionOutcome = "unchanged"
	// VisionCommitted: the store holds the candidate and every projection
	// (guardian cache, JSON, .mg, kernel facts) was installed.
	VisionCommitted VisionOutcome = "committed"
	// VisionProjectionPending: the store holds the candidate but at least one
	// projection failed. The edit is not rolled back and must not be replayed;
	// the next boot's reconciliation, or a re-save, repairs the projection.
	VisionProjectionPending VisionOutcome = "projection_pending"
)

// VisionUpdate is the receipt of a successful UpdateVisionFromBase.
type VisionUpdate struct {
	Outcome VisionOutcome
	// Vision is the authority after the call, detached from the guardian.
	Vision *Vision
	// Digest is VisionDigest(Vision); an editor that keeps editing uses it as
	// its next base.
	Digest string
	// Changes is the semantic delta between the previous authority and Vision.
	Changes []string
	// ProjectionErrors lists the projections that failed when Outcome is
	// VisionProjectionPending.
	ProjectionErrors []error
}

// UpdateVisionFromBase commits candidate as the vision if the stored vision
// still has the content of base (nil base: no vision may exist yet).
//
// The outcomes are distinct and ordered by when they are decided:
//
//   - rejected: candidate fails ValidateVision. Error is a
//     *VisionValidationError (errors.Is ErrVisionImportRejected); nothing was
//     written.
//   - conflicted: the stored vision is not base. Error is a *BaseStaleError
//     carrying what changed; nothing was written.
//   - unchanged: candidate equals the stored vision, whatever base was.
//   - committed / projection pending: see VisionOutcome.
//
// Validation runs before any effect, and the base comparison runs inside the
// store's write transaction, so a writer that slips in between the read and
// the write cannot be overwritten.
func (g *Guardian) UpdateVisionFromBase(candidate, base *Vision) (VisionUpdate, error) {
	if candidate == nil {
		return VisionUpdate{}, fmt.Errorf("vision is nil")
	}
	if err := ValidateVision(candidate); err != nil {
		return VisionUpdate{}, err
	}
	current, err := g.store.LoadVision()
	if err != nil {
		return VisionUpdate{}, err
	}
	if VisionDigest(current) == VisionDigest(candidate) {
		return VisionUpdate{Outcome: VisionUnchanged, Vision: cloneVision(current), Digest: VisionDigest(current)}, nil
	}

	candidate = cloneVision(candidate)
	if err := g.store.SaveVisionIfBase(candidate, VisionDigest(base)); err != nil {
		var stale *BaseStaleError
		if errors.As(err, &stale) {
			if now, loadErr := g.store.LoadVision(); loadErr == nil {
				stale.Changes = DiffVisions(base, now)
			}
		}
		return VisionUpdate{}, err
	}

	stateErr, projection := g.publishVision(candidate)
	if stateErr != nil {
		projection = append(projection, stateErr)
	}
	update := VisionUpdate{
		Outcome:          VisionCommitted,
		Vision:           cloneVision(candidate),
		Digest:           VisionDigest(candidate),
		Changes:          DiffVisions(current, candidate),
		ProjectionErrors: projection,
	}
	if len(projection) > 0 {
		update.Outcome = VisionProjectionPending
	}
	return update, nil
}

// SnapshotVision reconciles the JSON import surface with the store, then
// returns the authoritative vision, freshly read and so detached from every
// other holder, with its digest. It is where an editor takes its base.
func (g *Guardian) SnapshotVision() (*Vision, string, error) {
	if err := g.Initialize(); err != nil {
		return nil, NoVisionDigest, err
	}
	v, err := g.store.LoadVision()
	if err != nil {
		return nil, NoVisionDigest, err
	}
	return v, VisionDigest(v), nil
}
