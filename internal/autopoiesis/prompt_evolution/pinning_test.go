package prompt_evolution

import (
	"testing"
)

func TestPinScopeValid(t *testing.T) {
	for _, s := range AllPinScopes() {
		if !s.Valid() {
			t.Errorf("AllPinScopes returned invalid scope %q", s)
		}
	}
	for _, s := range []PinScope{"", "exact", "MODEL", "family"} {
		if s.Valid() {
			t.Errorf("scope %q should be invalid", s)
		}
	}
}

func TestPinSelectorsByScope(t *testing.T) {
	pin := ServingPin{Provider: "Anthropic", Model: "anthropic/claude-opus-4-20260501"}

	tests := []struct {
		scope         PinScope
		wantProviders []string
		wantModels    []string
	}{
		{PinScopeModelFamily, []string{"anthropic"}, []string{"claude_opus_4"}},
		{PinScopeModel, []string{"anthropic"}, []string{"claude_opus_4_20260501"}},
		{PinScopeProvider, []string{"anthropic"}, nil},
		{PinScopeNone, nil, nil},
	}

	for _, tc := range tests {
		t.Run(string(tc.scope), func(t *testing.T) {
			ag := &AtomGenerator{pinScope: tc.scope}
			providers, models := ag.pinSelectors(pin)

			if !equalStrings(providers, tc.wantProviders) {
				t.Errorf("providers = %v, want %v", providers, tc.wantProviders)
			}
			if !equalStrings(models, tc.wantModels) {
				t.Errorf("models = %v, want %v", models, tc.wantModels)
			}
		})
	}
}

// A scope asking for more specificity than the pin carries must degrade to what
// is known rather than fabricate a model selector.
func TestPinSelectorsDegradeOnMissingProvenance(t *testing.T) {
	ag := &AtomGenerator{pinScope: PinScopeModelFamily}

	providers, models := ag.pinSelectors(ServingPin{Provider: "anthropic"})
	if !equalStrings(providers, []string{"anthropic"}) || models != nil {
		t.Errorf("provider-only pin: providers=%v models=%v, want [anthropic] and nil", providers, models)
	}

	providers, models = ag.pinSelectors(ServingPin{})
	if providers != nil || models != nil {
		t.Errorf("empty pin: providers=%v models=%v, want nil and nil", providers, models)
	}
}

// The generated atom must carry selectors in the canonical vocabulary, since an
// atom pinned in raw vendor spelling would look pinned and never match.
func TestConvertToPromptAtomAppliesPin(t *testing.T) {
	ag := &AtomGenerator{pinScope: PinScopeModelFamily}
	def := atomDefinition{
		ID:       "evolved/test",
		Category: "methodology",
		Content:  "Always check the error.",
	}

	atom := ag.convertToPromptAtom(def, "/coder", ServingPin{
		Provider: "anthropic",
		Model:    "claude-opus-4-20260501",
	})
	if atom == nil {
		t.Fatal("convertToPromptAtom returned nil")
	}

	if !equalStrings(atom.Providers, []string{"anthropic"}) {
		t.Errorf("atom.Providers = %v, want [anthropic]", atom.Providers)
	}
	if !equalStrings(atom.Models, []string{"claude_opus_4"}) {
		t.Errorf("atom.Models = %v, want [claude_opus_4]", atom.Models)
	}
}

func TestConvertToPromptAtomUnpinnedUnderScopeNone(t *testing.T) {
	ag := &AtomGenerator{pinScope: PinScopeNone}
	atom := ag.convertToPromptAtom(
		atomDefinition{Category: "methodology", Content: "x"},
		"/coder",
		ServingPin{Provider: "anthropic", Model: "claude-opus-4"},
	)

	if len(atom.Providers) != 0 || len(atom.Models) != 0 {
		t.Errorf("scope none produced pins: providers=%v models=%v", atom.Providers, atom.Models)
	}
}

// Grouping granularity must equal pin granularity, or one atom is generated
// from failures spanning two models and pinned to whichever came first.
func TestGroupKeyMatchesPinGranularity(t *testing.T) {
	may := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "anthropic", Model: "claude-opus-4-20260501",
	}
	sept := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "anthropic", Model: "claude-opus-4-20260901",
	}
	other := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "openai", Model: "gpt-4o",
	}

	// Family scope: two snapshots of one model share a group.
	if GroupKeyFor(may, PinScopeModelFamily) != GroupKeyFor(sept, PinScopeModelFamily) {
		t.Error("family scope split two snapshots of one model into different groups")
	}
	// Exact scope: they must not.
	if GroupKeyFor(may, PinScopeModel) == GroupKeyFor(sept, PinScopeModel) {
		t.Error("model scope merged two distinct snapshots")
	}
	// Different vendors never share a group unless pinning is off.
	for _, scope := range []PinScope{PinScopeModelFamily, PinScopeModel, PinScopeProvider} {
		if GroupKeyFor(may, scope) == GroupKeyFor(other, scope) {
			t.Errorf("scope %q merged two providers", scope)
		}
	}
	if GroupKeyFor(may, PinScopeNone) != GroupKeyFor(other, PinScopeNone) {
		t.Error("scope none should ignore serving identity entirely")
	}
}

// The group's token and the resulting atom's selector must be equal by
// construction; this pins that invariant down rather than trusting it.
func TestGroupTokenEqualsGeneratedSelector(t *testing.T) {
	rec := &ExecutionRecord{
		ProblemType: "debugging", ShardType: "/coder",
		Provider: "Anthropic", Model: "anthropic/claude-opus-4-20260501",
	}

	for _, scope := range []PinScope{PinScopeModelFamily, PinScopeModel, PinScopeProvider} {
		key := GroupKeyFor(rec, scope)
		ag := &AtomGenerator{pinScope: scope}
		providers, models := ag.pinSelectors(servingPinFor([]*ExecutionRecord{rec}))

		gotProvider := ""
		if len(providers) > 0 {
			gotProvider = providers[0]
		}
		if gotProvider != key.Provider {
			t.Errorf("scope %q: atom provider %q != group provider %q", scope, gotProvider, key.Provider)
		}

		gotModel := ""
		if len(models) > 0 {
			gotModel = models[0]
		}
		if gotModel != key.Model {
			t.Errorf("scope %q: atom model %q != group model %q", scope, gotModel, key.Model)
		}
	}
}

func TestServingPinForSkipsRecordsWithoutProvenance(t *testing.T) {
	pin := servingPinFor([]*ExecutionRecord{
		nil,
		{ProblemType: "debugging"},
		{Provider: "anthropic", Model: "claude-opus-4"},
	})

	if pin.Provider != "anthropic" || pin.Model != "claude-opus-4" {
		t.Errorf("servingPinFor = %+v, want anthropic/claude-opus-4", pin)
	}

	if got := servingPinFor(nil); !got.IsZero() {
		t.Errorf("servingPinFor(nil) = %+v, want zero", got)
	}
}

// An omitted scope must default rather than silently disabling pinning.
func TestEvolverConfigPinScopeDefaults(t *testing.T) {
	cfg := DefaultEvolverConfig()
	cfg.AtomPinScope = ""

	validated, err := validateEvolverConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validated.AtomPinScope != PinScopeModelFamily {
		t.Errorf("AtomPinScope = %q, want %q", validated.AtomPinScope, PinScopeModelFamily)
	}
}

func TestEvolverConfigRejectsUnknownPinScope(t *testing.T) {
	cfg := DefaultEvolverConfig()
	cfg.AtomPinScope = "sometimes"

	if _, err := validateEvolverConfig(cfg); err == nil {
		t.Fatal("expected an error for an unknown pin scope")
	}
}

func TestNewAtomGeneratorDefaultsToModelFamily(t *testing.T) {
	if ag := NewAtomGenerator(nil, nil); ag.pinScope != PinScopeModelFamily {
		t.Errorf("pinScope = %q, want %q", ag.pinScope, PinScopeModelFamily)
	}
	// An unknown scope must not silently become "none".
	if ag := NewAtomGeneratorWithPinScope(nil, nil, "bogus"); ag.pinScope != PinScopeModelFamily {
		t.Errorf("pinScope = %q, want fallback %q", ag.pinScope, PinScopeModelFamily)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
