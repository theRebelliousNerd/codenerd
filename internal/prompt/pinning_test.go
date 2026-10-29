package prompt

import (
	"slices"
	"strings"
	"testing"
)

func TestNormalizeProviderToken(t *testing.T) {
	cases := map[string]string{
		"anthropic":   "anthropic",
		"Anthropic":   "anthropic",
		"/openai":     "openai",
		"  zai  ":     "zai",
		"vertex_ai":   "vertex_ai",
		"vertex-ai":   "vertex_ai",
		"Vertex AI":   "vertex_ai",
		"":            "",
		"   ":         "",
		"///":         "",
		"open.router": "open_router",
	}

	for in, want := range cases {
		if got := NormalizeProviderToken(in); got != want {
			t.Errorf("NormalizeProviderToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeModelToken(t *testing.T) {
	cases := map[string]string{
		"claude-opus-4-20260501":           "claude_opus_4_20260501",
		"anthropic/claude-opus-4-20260501": "claude_opus_4_20260501",
		"/claude-opus-4":                   "claude_opus_4",
		"anthropic.claude-sonnet-4-v1:0":   "claude_sonnet_4_v1",
		"vertex_ai/gemini-2.5-pro":         "gemini_2_5_pro",
		"gpt-4o":                           "gpt_4o",
		"GPT-4O":                           "gpt_4o",
		"gemma4:12b":                       "gemma4",
		"":                                 "",
		"   ":                              "",
	}

	for in, want := range cases {
		if got := NormalizeModelToken(in); got != want {
			t.Errorf("NormalizeModelToken(%q) = %q, want %q", in, got, want)
		}
	}
}

// The whole mechanism rests on both sides of a pin converging on one token, so
// this asserts the convergence directly rather than only the two halves.
func TestPinTokensConvergeAcrossSpellings(t *testing.T) {
	spellings := []string{
		"claude-opus-4-20260501",
		"anthropic/claude-opus-4-20260501",
		"Claude-Opus-4-20260501",
	}

	want := NormalizeModelToken(spellings[0])
	for _, s := range spellings[1:] {
		if got := NormalizeModelToken(s); got != want {
			t.Errorf("spelling %q normalized to %q, want %q", s, got, want)
		}
	}

	// An atom authored at family granularity must match a runtime id that
	// carries a date suffix.
	atomSide := NormalizeModelToken("claude-opus-4")
	contextSide := ModelPinTokens("anthropic/claude-opus-4-20260501")
	if !slices.Contains(contextSide, atomSide) {
		t.Errorf("family pin %q not satisfied by context tokens %v", atomSide, contextSide)
	}
}

func TestModelFamilyToken(t *testing.T) {
	cases := map[string]string{
		"claude_opus_4_20260501": "claude_opus_4",
		"gpt_4o_2024_08_06":      "gpt_4o",
		"gemini_3_pro_latest":    "gemini_3_pro",
		"gemini_3_pro_preview":   "gemini_3_pro",
		"claude_opus_4_2026_05":  "claude_opus_4",

		// Bare numeric tails are part of the name, not a version. Stripping
		// these would collapse distinct models onto one family token and let a
		// pin for one match the other.
		"claude_opus_4": "claude_opus_4",
		"gpt_4o":        "gpt_4o",
		"gemma4":        "gemma4",
		"gpt_5":         "gpt_5",

		// Never strip down to nothing.
		"20260501": "20260501",
		"latest":   "latest",
		"":         "",
	}

	for in, want := range cases {
		if got := ModelFamilyToken(in); got != want {
			t.Errorf("ModelFamilyToken(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestModelPinTokens(t *testing.T) {
	tokens := ModelPinTokens("anthropic/claude-opus-4-20260501")
	want := []string{"claude_opus_4_20260501", "claude_opus_4"}
	if !slices.Equal(tokens, want) {
		t.Errorf("ModelPinTokens = %v, want %v", tokens, want)
	}

	// No family distinct from the exact token means no duplicate entry.
	if got := ModelPinTokens("gpt-4o"); !slices.Equal(got, []string{"gpt_4o"}) {
		t.Errorf("ModelPinTokens(gpt-4o) = %v, want [gpt_4o]", got)
	}

	if got := ModelPinTokens(""); got != nil {
		t.Errorf("ModelPinTokens(\"\") = %v, want nil", got)
	}
}

// Tokens must be fixpoints of the Mangle name-constant writer, or the fact the
// context asserts differs from the tag the atom carries.
func TestPinTokensAreMangleSafe(t *testing.T) {
	inputs := []string{
		"anthropic/claude-opus-4-20260501",
		"anthropic.claude-sonnet-4-v1:0",
		"vertex_ai/gemini-2.5-pro",
		"gemma4:12b",
	}

	for _, in := range inputs {
		for _, token := range ModelPinTokens(in) {
			if got := mangleNormalizeNameConst("/" + token); got != "/"+token {
				t.Errorf("token %q from %q is not a Mangle fixpoint: got %q", token, in, got)
			}
			if strings.ContainsAny(token, "-./:% ~") {
				t.Errorf("token %q from %q contains a character writeAtom preserves but pinning must not emit", token, in)
			}
		}
	}
}

func TestCompilationContextEmitsPinFacts(t *testing.T) {
	cc := NewCompilationContext().WithProviderModel("Anthropic", "claude-opus-4-20260501")

	facts := cc.GenerateFacts(FactStyle{Predicate: "current_context", UseShort: true, ForceAtoms: true})

	joined := make([]string, 0, len(facts))
	for _, f := range facts {
		joined = append(joined, f.(string))
	}
	all := strings.Join(joined, "\n")

	for _, want := range []string{
		"current_context(/provider, /anthropic)",
		"current_context(/model, /claude_opus_4_20260501)",
		"current_context(/model, /claude_opus_4)",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("missing fact %q in:\n%s", want, all)
		}
	}
}

// An unset pin must emit nothing rather than an empty-valued fact, which would
// be a name constant of "/" and either fail to parse or match nothing.
func TestCompilationContextOmitsEmptyPinFacts(t *testing.T) {
	cc := NewCompilationContext()

	for _, style := range []FactStyle{
		{Predicate: "current_context", UseShort: true, ForceAtoms: true},
		{Predicate: "compile_context", AddDot: true},
	} {
		for _, f := range cc.GenerateFacts(style) {
			s := f.(string)
			if strings.Contains(s, "provider") || strings.Contains(s, "model") {
				t.Errorf("unset pin emitted a fact: %q", s)
			}
		}
	}
}

// compile_context is the ForceAtoms:false style; the pin values must still land
// as /name constants, not quoted strings, or they never unify with atom_selector.
func TestCompileContextPinsAreNameConstants(t *testing.T) {
	cc := NewCompilationContext().WithProviderModel("openai", "gpt-4o")

	var found bool
	for _, f := range cc.GenerateFacts(FactStyle{Predicate: "compile_context", AddDot: true}) {
		s := f.(string)
		if !strings.Contains(s, "provider") {
			continue
		}
		found = true
		if strings.Contains(s, `"`) {
			t.Errorf("provider emitted as a quoted string, not a name constant: %q", s)
		}
		if want := "compile_context(/provider, /openai)."; s != want {
			t.Errorf("got %q, want %q", s, want)
		}
	}
	if !found {
		t.Fatal("no provider fact emitted")
	}
}

func TestContextHashSeparatesPins(t *testing.T) {
	base := NewCompilationContext()
	anthropic := NewCompilationContext().WithProviderModel("anthropic", "claude-opus-4")
	openai := NewCompilationContext().WithProviderModel("openai", "gpt-4o")

	if base.Hash() == anthropic.Hash() {
		t.Error("pinned and unpinned contexts share a cache identity")
	}
	if anthropic.Hash() == openai.Hash() {
		t.Error("two providers share a cache identity")
	}

	// Two spellings of one model select identically and so must share one entry.
	a := NewCompilationContext().WithProviderModel("openai", "gpt-4o")
	b := NewCompilationContext().WithProviderModel("openai", "openai/GPT-4O")
	if a.Hash() != b.Hash() {
		t.Error("equivalent model spellings produced different cache identities")
	}
}
