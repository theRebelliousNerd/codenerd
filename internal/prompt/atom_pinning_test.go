package prompt

import "testing"

func pinnedAtom(providers, models []string) *PromptAtom {
	a := &PromptAtom{
		ID:        "test/pinned",
		Category:  CategoryMethodology,
		Content:   "content",
		Providers: providers,
		Models:    models,
	}
	a.NormalizeSelectors()
	return a
}

func TestMatchesContextProviderPin(t *testing.T) {
	atom := pinnedAtom([]string{"/anthropic"}, nil)

	tests := []struct {
		name     string
		provider string
		want     bool
	}{
		{"matching provider", "anthropic", true},
		{"matching provider, different case", "Anthropic", true},
		{"different provider", "openai", false},
		// Fail-closed: an unnamed provider cannot demonstrate the pin holds.
		{"unset provider", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cc := NewCompilationContext().WithProviderModel(tc.provider, "")
			if got := atom.MatchesContext(cc); got != tc.want {
				t.Errorf("MatchesContext(provider=%q) = %v, want %v", tc.provider, got, tc.want)
			}
		})
	}
}

func TestMatchesContextModelPin(t *testing.T) {
	exact := pinnedAtom(nil, []string{"claude-opus-4-20260501"})
	family := pinnedAtom(nil, []string{"claude-opus-4"})

	tests := []struct {
		name  string
		atom  *PromptAtom
		model string
		want  bool
	}{
		{"exact pin, exact model", exact, "claude-opus-4-20260501", true},
		{"exact pin, different snapshot", exact, "claude-opus-4-20260901", false},
		{"family pin, dated snapshot", family, "claude-opus-4-20260501", true},
		{"family pin, other snapshot", family, "claude-opus-4-20260901", true},
		{"family pin, different model", family, "claude-sonnet-4-20260501", false},
		{"family pin, routing prefix", family, "anthropic/claude-opus-4-20260501", true},
		{"unset model", family, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cc := NewCompilationContext().WithProviderModel("", tc.model)
			if got := tc.atom.MatchesContext(cc); got != tc.want {
				t.Errorf("MatchesContext(model=%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}

// The overwhelming majority of the corpus declares no pin and must be entirely
// unaffected, including on compiles that name no provider or model.
func TestUnpinnedAtomsIgnorePinDimensions(t *testing.T) {
	atom := pinnedAtom(nil, nil)

	for _, cc := range []*CompilationContext{
		NewCompilationContext(),
		NewCompilationContext().WithProviderModel("anthropic", "claude-opus-4"),
		NewCompilationContext().WithProviderModel("openai", "gpt-4o"),
	} {
		if !atom.MatchesContext(cc) {
			t.Errorf("unpinned atom rejected for provider=%q model=%q", cc.Provider, cc.Model)
		}
	}
}

func TestMatchesContextBothPinsMustHold(t *testing.T) {
	atom := pinnedAtom([]string{"anthropic"}, []string{"claude-opus-4"})

	tests := []struct {
		provider string
		model    string
		want     bool
	}{
		{"anthropic", "claude-opus-4-20260501", true},
		{"anthropic", "gpt-4o", false},
		{"openai", "claude-opus-4-20260501", false},
		{"openai", "gpt-4o", false},
	}

	for _, tc := range tests {
		cc := NewCompilationContext().WithProviderModel(tc.provider, tc.model)
		if got := atom.MatchesContext(cc); got != tc.want {
			t.Errorf("MatchesContext(%q, %q) = %v, want %v", tc.provider, tc.model, got, tc.want)
		}
	}
}

// NormalizeSelectors is what makes an authored pin comparable to a runtime id.
func TestNormalizeSelectorsCanonicalizesPins(t *testing.T) {
	atom := &PromptAtom{
		Providers: []string{"/Anthropic", "anthropic", "  ", "///"},
		Models:    []string{"anthropic/Claude-Opus-4", "claude_opus_4"},
	}
	atom.NormalizeSelectors()

	if len(atom.Providers) != 1 || atom.Providers[0] != "anthropic" {
		t.Errorf("Providers = %v, want [anthropic] (deduped, empties dropped)", atom.Providers)
	}
	if len(atom.Models) != 1 || atom.Models[0] != "claude_opus_4" {
		t.Errorf("Models = %v, want [claude_opus_4] (deduped)", atom.Models)
	}
}

// A selector list that sanitizes away entirely must not become an empty list,
// because an empty list means "match everything" -- the opposite of the intent.
func TestUnusablePinDoesNotBecomeMatchAll(t *testing.T) {
	atom := &PromptAtom{Providers: []string{"///", "   "}}
	atom.NormalizeSelectors()

	if len(atom.Providers) != 0 {
		t.Fatalf("expected all entries dropped, got %v", atom.Providers)
	}
	// Documents the consequence: with nothing left to enforce, the atom is
	// unpinned. Callers authoring pins must supply usable values.
	if !atom.MatchesContext(NewCompilationContext().WithProviderModel("openai", "gpt-4o")) {
		t.Error("atom with no surviving selectors should behave as unpinned")
	}
}

func TestToSelectorFactsEmitsPins(t *testing.T) {
	atom := pinnedAtom([]string{"anthropic"}, []string{"claude-opus-4"})

	var sawProvider, sawModel bool
	for _, f := range atom.ToSelectorFacts() {
		if f.Predicate != "atom_selector" || len(f.Args) != 3 {
			continue
		}
		switch f.Args[1] {
		case "/provider":
			sawProvider = true
			if f.Args[2] != "anthropic" {
				t.Errorf("provider selector value = %v, want anthropic", f.Args[2])
			}
		case "/model":
			sawModel = true
			if f.Args[2] != "claude_opus_4" {
				t.Errorf("model selector value = %v, want claude_opus_4", f.Args[2])
			}
		}
	}

	if !sawProvider {
		t.Error("no /provider selector fact emitted")
	}
	if !sawModel {
		t.Error("no /model selector fact emitted")
	}
}
