package marathon

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeClient struct {
	provider string
	model    string

	supports bool

	searchResult *types.GroundedWebSearchResult
	searchErr    error

	completion  string
	completeErr error
}

func (c *fakeClient) Complete(ctx context.Context, p string) (string, error) { return "", nil }
func (c *fakeClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	return c.completion, c.completeErr
}
func (c *fakeClient) CompleteWithStreaming(ctx context.Context, sys, user string, thinking bool) (<-chan string, <-chan error) {
	return nil, nil
}
func (c *fakeClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, nil
}

// identifyingClient is the fake plus ModelIdentity, so a bare fakeClient still
// exercises the "client cannot report its model" path.
type identifyingClient struct{ *fakeClient }

func (c identifyingClient) ModelIdentity() (string, string) { return c.provider, c.model }

type groundingClient struct{ identifyingClient }

func (c groundingClient) SupportsGroundedWebSearch() bool { return c.supports }
func (c groundingClient) GroundedWebSearch(ctx context.Context, q string) (*types.GroundedWebSearchResult, error) {
	return c.searchResult, c.searchErr
}

func groundedResult(text string, urls ...string) *types.GroundedWebSearchResult {
	r := &types.GroundedWebSearchResult{Text: text}
	for _, u := range urls {
		r.Citations = append(r.Citations, types.GroundedCitation{URL: u, Title: "doc"})
	}
	return r
}

// ---------------------------------------------------------------------------
// Hard-fail gates
// ---------------------------------------------------------------------------

func TestServingIdentityIsRequired(t *testing.T) {
	// A client that cannot say which model it serves.
	_, _, err := ServingIdentity(&fakeClient{})
	if !errors.Is(err, ErrNoModelIdentity) {
		t.Errorf("expected ErrNoModelIdentity, got %v", err)
	}

	// One that can, but reports nothing.
	blank := identifyingClient{&fakeClient{}}
	if _, _, err := ServingIdentity(blank); !errors.Is(err, ErrNoModelIdentity) {
		t.Errorf("expected ErrNoModelIdentity for a blank identity, got %v", err)
	}

	ok := identifyingClient{&fakeClient{provider: "anthropic", model: "claude-opus-4"}}
	provider, model, err := ServingIdentity(ok)
	if err != nil || provider != "anthropic" || model != "claude-opus-4" {
		t.Errorf("ServingIdentity = (%q, %q, %v)", provider, model, err)
	}
}

func TestResearcherRequiresGroundedSearch(t *testing.T) {
	// Does not implement the interface at all.
	if _, err := NewResearcher(&fakeClient{}); !errors.Is(err, ErrNoGroundedSearch) {
		t.Errorf("expected ErrNoGroundedSearch, got %v", err)
	}

	// Implements it but reports grounding unavailable. This is the case that
	// would otherwise produce an empty profile much later.
	unavailable := groundingClient{identifyingClient{&fakeClient{supports: false}}}
	if _, err := NewResearcher(unavailable); !errors.Is(err, ErrNoGroundedSearch) {
		t.Errorf("expected ErrNoGroundedSearch for unavailable grounding, got %v", err)
	}
}

// The load-bearing gate: grounded search answers from the model's own priors
// when retrieval finds nothing, so prose without citations must NOT count as
// documentation.
func TestResearchFailsOnProseWithoutCitations(t *testing.T) {
	client := groundingClient{identifyingClient{&fakeClient{
		supports:     true,
		searchResult: groundedResult("Here is some confident advice about prompting."),
	}}}

	r, err := NewResearcher(client)
	if err != nil {
		t.Fatalf("NewResearcher: %v", err)
	}

	_, err = r.Research(context.Background(), "anthropic", "claude-opus-4")
	if !errors.Is(err, ErrNoModelDocs) {
		t.Errorf("uncited prose was accepted as documentation; got %v", err)
	}
}

func TestResearchFailsWhenSearchErrors(t *testing.T) {
	client := groundingClient{identifyingClient{&fakeClient{
		supports:  true,
		searchErr: errors.New("upstream unavailable"),
	}}}

	r, _ := NewResearcher(client)
	if _, err := r.Research(context.Background(), "anthropic", "claude-opus-4"); !errors.Is(err, ErrNoModelDocs) {
		t.Errorf("expected ErrNoModelDocs when every query failed, got %v", err)
	}
}

func TestResearchSucceedsWithCitations(t *testing.T) {
	client := groundingClient{identifyingClient{&fakeClient{
		supports:     true,
		searchResult: groundedResult("Use XML tags for structure.", "https://docs.example/claude"),
	}}}

	r, _ := NewResearcher(client)
	profile, err := r.Research(context.Background(), "anthropic", "anthropic/claude-opus-4-20260501")
	if err != nil {
		t.Fatalf("Research: %v", err)
	}
	if !profile.HasDocs() {
		t.Error("profile reports no docs despite citations")
	}
	if profile.ProviderPin != "anthropic" {
		t.Errorf("ProviderPin = %q, want anthropic", profile.ProviderPin)
	}
	// Family granularity, so the overlay survives the vendor's next snapshot.
	if profile.ModelPin != "claude_opus_4" {
		t.Errorf("ModelPin = %q, want claude_opus_4 (family, not the dated snapshot)", profile.ModelPin)
	}
}

func TestOptimizerRefusesUnsourcedProfile(t *testing.T) {
	_, err := NewOptimizer(&fakeClient{}, &ModelDocProfile{Guidance: "words, no sources"})
	if !errors.Is(err, ErrNoModelDocs) {
		t.Errorf("optimizer accepted an unsourced profile; got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Variant construction
// ---------------------------------------------------------------------------

func testProfile() *ModelDocProfile {
	return &ModelDocProfile{
		Provider:    "anthropic",
		Model:       "claude-opus-4-20260501",
		ProviderPin: "anthropic",
		ModelPin:    "claude_opus_4",
		Guidance:    "Prefer XML tags.",
		Citations:   []Citation{{URL: "https://docs.example/claude"}},
	}
}

func baseAtom() *prompt.PromptAtom {
	a := &prompt.PromptAtom{
		ID:          "methodology/tdd",
		Category:    prompt.CategoryMethodology,
		Content:     "Write the failing test first.",
		ShardTypes:  []string{"coder"},
		Priority:    70,
		IsMandatory: true,
	}
	a.ContentHash = prompt.HashContent(a.Content)
	return a
}

func TestVariantIsPinnedAndSupersedesBase(t *testing.T) {
	o, err := NewOptimizer(&fakeClient{}, testProfile())
	if err != nil {
		t.Fatalf("NewOptimizer: %v", err)
	}

	base := baseAtom()
	variant := o.buildVariant(base, "<task>Write the failing test first.</task>")

	if variant.ID != "methodology/tdd@claude_opus_4" {
		t.Errorf("variant ID = %q", variant.ID)
	}
	if len(variant.Providers) != 1 || variant.Providers[0] != "anthropic" {
		t.Errorf("Providers = %v, want [anthropic]", variant.Providers)
	}
	if len(variant.Models) != 1 || variant.Models[0] != "claude_opus_4" {
		t.Errorf("Models = %v, want [claude_opus_4]", variant.Models)
	}

	// Supersession, one direction. A mutual pair cancels both atoms in the
	// kernel and deletes the guidance outright.
	if len(variant.ConflictsWith) != 1 || variant.ConflictsWith[0] != base.ID {
		t.Errorf("ConflictsWith = %v, want [%s]", variant.ConflictsWith, base.ID)
	}

	// The variant must be selected wherever the base would have been, or it is
	// not a replacement.
	if variant.Category != base.Category {
		t.Errorf("category drifted: %q vs %q", variant.Category, base.Category)
	}
	if variant.IsMandatory != base.IsMandatory {
		t.Error("mandatory flag drifted; the variant would not stand in for the base")
	}
	if strings.Join(variant.ShardTypes, ",") != strings.Join(base.ShardTypes, ",") {
		t.Errorf("shard selectors drifted: %v vs %v", variant.ShardTypes, base.ShardTypes)
	}

	// The base must be untouched by variant construction.
	if len(base.Providers) != 0 || len(base.Models) != 0 || len(base.ConflictsWith) != 0 {
		t.Error("buildVariant mutated the shipped atom")
	}
	if base.Content != "Write the failing test first." {
		t.Error("buildVariant rewrote the shipped atom's content")
	}

	// Stale concise/min bodies describe the old text and must not survive.
	if variant.ContentConcise != "" || variant.ContentMin != "" {
		t.Error("variant kept the base's concise/min bodies, which describe different text")
	}
	if variant.ContentHash == base.ContentHash {
		t.Error("variant kept the base content hash")
	}
}

// ---------------------------------------------------------------------------
// Optimizer response handling
// ---------------------------------------------------------------------------

func TestOptimizeReturnsNilWhenUnchanged(t *testing.T) {
	client := &fakeClient{completion: "changed: false\nrationale: \"documentation says nothing relevant\"\n"}
	o, _ := NewOptimizer(client, testProfile())

	got, err := o.Optimize(context.Background(), baseAtom())
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if got != nil {
		t.Errorf("expected no variant for an unchanged atom, got %+v", got.Atom.ID)
	}
}

// "changed: true" with no content is a malformed answer, not an instruction to
// emit an empty atom.
func TestOptimizeIgnoresChangedWithEmptyContent(t *testing.T) {
	client := &fakeClient{completion: "changed: true\nrationale: \"x\"\ncontent: \"\"\n"}
	o, _ := NewOptimizer(client, testProfile())

	got, err := o.Optimize(context.Background(), baseAtom())
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if got != nil {
		t.Error("an empty rewrite was emitted as a variant")
	}
}

func TestOptimizeParsesFencedYAML(t *testing.T) {
	client := &fakeClient{completion: "```yaml\nchanged: true\nrationale: \"tags\"\ncontent: |\n  <task>go</task>\n```"}
	o, _ := NewOptimizer(client, testProfile())

	got, err := o.Optimize(context.Background(), baseAtom())
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}
	if got == nil {
		t.Fatal("fenced YAML was not parsed")
	}
	if !strings.Contains(got.Atom.Content, "<task>go</task>") {
		t.Errorf("content = %q", got.Atom.Content)
	}
	if got.BaseID != "methodology/tdd" {
		t.Errorf("BaseID = %q", got.BaseID)
	}
	if len(got.Citations) == 0 {
		t.Error("variant carries no citations; provenance is lost")
	}
}

func TestOptimizeSurfacesClientErrors(t *testing.T) {
	client := &fakeClient{completeErr: errors.New("rate limited")}
	o, _ := NewOptimizer(client, testProfile())

	if _, err := o.Optimize(context.Background(), baseAtom()); err == nil {
		t.Error("client error was swallowed; the atom would be checkpointed as decided")
	}
}

// ---------------------------------------------------------------------------
// Base selection
// ---------------------------------------------------------------------------

// Already-pinned atoms must be skipped: optimizing one produces a variant of a
// variant, pinned to the same model, superseding an atom that already
// supersedes the original.
func TestSelectBasesSkipsPinnedAtoms(t *testing.T) {
	all := []*prompt.PromptAtom{
		{ID: "a", Content: "x", Category: prompt.CategoryMethodology},
		{ID: "b", Content: "x", Category: prompt.CategoryMethodology, Models: []string{"claude_opus_4"}},
		{ID: "c", Content: "x", Category: prompt.CategoryMethodology, Providers: []string{"anthropic"}},
		{ID: "d", Content: "   ", Category: prompt.CategoryMethodology},
	}

	got := selectBases(all, nil)
	if len(got) != 1 || got[0].ID != "a" {
		ids := make([]string, 0, len(got))
		for _, a := range got {
			ids = append(ids, a.ID)
		}
		t.Errorf("selectBases = %v, want [a]", ids)
	}
}

func TestSelectBasesFiltersByCategoryAndSortsStably(t *testing.T) {
	all := []*prompt.PromptAtom{
		{ID: "z", Content: "x", Category: prompt.CategoryMethodology},
		{ID: "a", Content: "x", Category: prompt.CategoryMethodology},
		{ID: "m", Content: "x", Category: prompt.CategorySafety},
	}

	got := selectBases(all, []prompt.AtomCategory{prompt.CategoryMethodology})
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "z" {
		t.Errorf("expected [a z] from the methodology category, got %d atoms", len(got))
	}
}

func TestOverlayAtomID(t *testing.T) {
	if got := OverlayAtomID("methodology/tdd", "claude_opus_4"); got != "methodology/tdd@claude_opus_4" {
		t.Errorf("OverlayAtomID = %q", got)
	}
	// Without a model pin there is nothing to distinguish the variant by, so it
	// must not silently collide with the base ID under a different name.
	if got := OverlayAtomID("methodology/tdd", ""); got != "methodology/tdd" {
		t.Errorf("OverlayAtomID with no pin = %q", got)
	}
}
