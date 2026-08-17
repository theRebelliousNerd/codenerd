// Package marathon implements the long-running corpus optimization pass behind
// `nerd init --marathon`.
//
// Standard init builds a project profile and generates project-specific atoms.
// The marathon does something different and far more expensive: it researches
// the prompting documentation for the model that will actually serve this
// workspace, then rewrites the whole shipped atom corpus against that
// documentation, emitting the results as model-pinned variants.
//
// Three properties define it:
//
//  1. It is grounded or it does not run. The optimization is only worth
//     anything if it is written against real, current documentation for the
//     specific serving model. If grounded search is unavailable, or the search
//     turns up no documentation for that model, the marathon fails rather than
//     falling back to the optimizer's own opinions about prompting. An
//     "optimization" derived from a model's own priors is indistinguishable
//     from noise and would be written into the corpus with the authority of
//     research.
//
//  2. It never touches the shipped corpus. Every result is a NEW atom, pinned
//     to the serving provider/model and declaring supersession over the atom it
//     was derived from. internal/prompt/atoms/**.yaml is read-only to this
//     package. On the model it was optimized for the variant stands in for the
//     original; on every other model the original is served untouched.
//
//  3. It is resumable. A full pass is hundreds of LLM calls, so progress is
//     checkpointed per atom and a re-run picks up where it stopped.
package marathon

import (
	"errors"
	"time"

	"codenerd/internal/prompt"
)

// Hard-fail sentinels. Each of these ends the run; none of them degrades into a
// partial or unresearched pass.
var (
	// ErrNoModelIdentity means the serving provider/model could not be
	// determined. Everything downstream is keyed on it -- the research query,
	// the pins, the checkpoint -- so there is nothing sensible to do without it.
	ErrNoModelIdentity = errors.New("marathon: serving provider/model is unknown; " +
		"the LLM client does not implement types.ModelIdentifier")

	// ErrNoGroundedSearch means the client cannot perform grounded web search.
	// This is the constitutionally permitted research route (safe_action
	// /grounded_web_search); web_search and web_fetch are routed but carry no
	// safe_action entry, so they cannot be used here.
	ErrNoGroundedSearch = errors.New("marathon: the configured client does not support " +
		"grounded web search, which is the only research route the constitution permits")

	// ErrNoModelDocs means the research phase completed but found no usable
	// documentation for this specific model. Optimizing anyway would produce
	// atoms whose authority is fabricated.
	ErrNoModelDocs = errors.New("marathon: no prompting documentation found for the serving model")
)

// Citation is a source the research phase actually used, carried through to
// every atom derived from it so a reviewer can check the claim.
type Citation struct {
	URL   string `json:"url"`
	Title string `json:"title,omitzero"`
}

// ModelDocProfile is the researched prompting guidance for one model. It is the
// sole input the optimizer is allowed to treat as authoritative.
type ModelDocProfile struct {
	// Provider and Model are the raw serving identity, plus the canonical pin
	// tokens derived from them.
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	ProviderPin string `json:"provider_pin"`
	ModelPin    string `json:"model_pin"`

	// Guidance is the synthesized prompting documentation for this model:
	// formatting preferences, instruction-following characteristics, known
	// failure modes, tool-call conventions.
	Guidance string `json:"guidance"`

	// Citations are the sources behind Guidance. A profile with no citations is
	// rejected -- that is the ErrNoModelDocs case.
	Citations []Citation `json:"citations"`

	ResearchedAt time.Time `json:"researched_at"`
}

// HasDocs reports whether the profile carries usable, sourced guidance.
func (p *ModelDocProfile) HasDocs() bool {
	return p != nil && len(p.Citations) > 0 && len(p.Guidance) > 0
}

// OptimizedAtom is one rewritten atom plus the provenance needed to review or
// revert it.
type OptimizedAtom struct {
	// Atom is the emitted variant: a new ID, pinned, superseding BaseID.
	Atom *prompt.PromptAtom `json:"atom"`

	// BaseID is the shipped atom this was derived from.
	BaseID string `json:"base_id"`

	// BaseContentHash pins which revision of the base was optimized. If the
	// shipped atom later changes, this is how a stale variant is detected.
	BaseContentHash string `json:"base_content_hash"`

	// Rationale is the optimizer's one-line account of what it changed and
	// which part of the documentation motivated it.
	Rationale string `json:"rationale"`

	// Citations are the subset of profile sources this rewrite leaned on.
	Citations []Citation `json:"citations,omitzero"`

	OptimizedAt time.Time `json:"optimized_at"`
}

// Result summarizes one marathon run.
type Result struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`

	AtomsConsidered int `json:"atoms_considered"`
	AtomsOptimized  int `json:"atoms_optimized"`
	AtomsUnchanged  int `json:"atoms_unchanged"`
	AtomsFailed     int `json:"atoms_failed"`
	AtomsResumed    int `json:"atoms_resumed"`

	Citations []Citation    `json:"citations,omitzero"`
	Duration  time.Duration `json:"duration"`
	Errors    []string      `json:"errors,omitzero"`
}

// OverlayAtomID derives the variant's ID from its base and model pin.
//
// The suffix keeps the variant a distinct atom rather than an upsert over the
// shipped one, which is what allows both to coexist and lets supersession pick
// between them per compile. The separator is "@" because atom IDs are
// path-shaped ("methodology/tdd/red_green") and "@" appears in none of them.
func OverlayAtomID(baseID, modelPin string) string {
	if modelPin == "" {
		return baseID
	}
	return baseID + "@" + modelPin
}
