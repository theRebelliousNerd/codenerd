package chat

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

// goldenArchitectPersona pins the architect's words. *.golden is eol=lf in
// .gitattributes, so this comparison is genuinely byte-for-byte on every
// platform and does not need newline normalisation.
const goldenArchitectPersona = "testdata/architect_persona.golden"

// TestPersonaWordingUnchanged is the "nobody edits his words" gate.
//
// The persona is the architect's own voice, written by him. A refactor, a
// linter, a well-meaning tidy-up of a markdown table, or a model asked to
// "improve the prompt" must not be able to change a single byte of it without a
// test going red and a human deciding that was intended.
//
// If this fails and the change WAS intended, the fix is to update the golden
// deliberately in the same commit — never to loosen the comparison.
func TestPersonaWordingUnchanged(t *testing.T) {
	want, err := os.ReadFile(goldenArchitectPersona)
	if err != nil {
		t.Fatalf("read golden persona: %v", err)
	}

	got := architectPersona
	if string(want) == got {
		return
	}

	wantLines := strings.Split(string(want), "\n")
	gotLines := strings.Split(got, "\n")
	for i := 0; i < len(wantLines) && i < len(gotLines); i++ {
		if wantLines[i] != gotLines[i] {
			t.Fatalf("architect persona wording changed at line %d\n golden: %q\n   code: %q\n"+
				"The persona text is the architect's own words. If you meant to change them, "+
				"update %s in the same commit.",
				i+1, wantLines[i], gotLines[i], goldenArchitectPersona)
		}
	}
	t.Fatalf("architect persona length changed: golden has %d lines / %d bytes, code has %d lines / %d bytes",
		len(wantLines), len(want), len(gotLines), len(got))
}

// TestMainChatSystemPrompt_AlwaysCarriesArchitectPersona is the invariant this
// whole seam exists for: the main TUI chat turn cannot run without the
// architect's persona, and the persona cannot drift away from the front of the
// window.
//
// It compiles the system prompt through the production path —
// Model.articulationSystemPrompt, which cmd/nerd/chat/process.go calls verbatim
// — with a real kernel, under two budgets:
//
//	(a) a normal budget, and
//	(b) the tightest budget the config schema permits, tighter than the persona
//	    itself costs.
//
// Case (b) is the one that matters. Every other budget-sensitive prompt in this
// repository either sheds content or refuses to compile when the budget cannot
// hold the mandatory skeleton (internal/prompt/compiler.go:1753). The chat
// persona must do neither: it is delivered whole, at offset 0, no matter what
// the numbers say.
//
// PROVEN TO FAIL: deleting the withArchitectPersona call from
// articulationSystemPrompt (i.e. returning the kernel's base prompt alone, which
// is what the code did before this seam existed minus the append) turns both
// subtests red on "does not carry the architect persona at all".
func TestMainChatSystemPrompt_AlwaysCarriesArchitectPersona(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.UserConfig
	}{
		{
			name: "normal budget",
			cfg: &config.UserConfig{
				ContextWindow: &config.ContextWindowConfig{MaxTokens: 1048576},
				JIT:           &config.JITConfig{TokenBudget: 200000, ReservedTokens: 8000},
			},
		},
		{
			// The tightest budget the config allows: one token of context window
			// and one token of JIT budget. GetEffectiveJITConfig clamps the JIT
			// budget to the context window and then zeroes the reserve, so this
			// is the floor, and it is orders of magnitude below what the persona
			// costs.
			name: "tightest budget the config allows",
			cfg: &config.UserConfig{
				ContextWindow: &config.ContextWindowConfig{MaxTokens: 1},
				JIT:           &config.JITConfig{TokenBudget: 1, ReservedTokens: 1},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, err := core.NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel: %v", err)
			}
			m := NewTestModel()
			m.kernel = k
			m.Config = tc.cfg

			systemPrompt := m.articulationSystemPrompt()

			idx := strings.Index(systemPrompt, architectPersona)
			switch {
			case idx < 0:
				t.Fatalf("main-chat system prompt does not carry the architect persona at all "+
					"(prompt is %d bytes). This is the failure this test exists to prevent: "+
					"the persona IS the chat identity, and a turn without it is a different agent.",
					len(systemPrompt))
			case idx != architectPersonaOffset:
				t.Fatalf("architect persona is at byte offset %d, want %d (the top). "+
					"Where a thing sits in the window is a decision; this one is 'first'. "+
					"Prefix found before it: %q",
					idx, architectPersonaOffset, systemPrompt[:idx])
			}

			// Byte-identical: Index already proves the exact bytes are present,
			// so this pins the stronger property that nothing was interpolated
			// into or trimmed out of the copy that shipped.
			if got := systemPrompt[:len(architectPersona)]; got != architectPersona {
				t.Fatalf("architect persona was altered in delivery (%d bytes delivered vs %d bytes of source)",
					len(got), len(architectPersona))
			}

			// The cost is counted, with the same estimator the JIT budget is
			// fitted with. A budget of 1 token does not shrink it.
			tokens := architectPersonaTokens()
			if tokens <= 0 {
				t.Fatalf("architect persona token cost not counted: got %d", tokens)
			}
			effective := tc.cfg.GetEffectiveJITConfig()
			t.Logf("persona: %d tokens at offset %d; effective JIT budget: %d tokens (reserve %d)",
				tokens, architectPersonaOffset, effective.TokenBudget, effective.ReservedTokens)
		})
	}
}

// TestArchitectPersonaSurvivesEverySeam covers the other two call sites that
// hand the persona to a model, so "never lost" is not a property of the main
// chat turn alone.
func TestArchitectPersonaSurvivesEverySeam(t *testing.T) {
	t.Run("knowledge synthesis", func(t *testing.T) {
		// Same string process_knowledge.go passes.
		got := withArchitectPersona("You are codeNERD answering a user's question after consulting internal specialists.")
		assertPersonaLeads(t, got)
		if !strings.Contains(got, "after consulting internal specialists") {
			t.Fatalf("knowledge-synthesis framing was dropped by the persona seam")
		}
	})

	t.Run("shard-translator fallback", func(t *testing.T) {
		// process_dream_delegation.go's fallback returns the persona alone.
		got := withArchitectPersona("")
		assertPersonaLeads(t, got)
		if got != architectPersona {
			t.Fatalf("translator fallback system prompt changed: want the persona alone (%d bytes), got %d bytes",
				len(architectPersona), len(got))
		}
	})

	t.Run("empty and whitespace base both yield the persona alone", func(t *testing.T) {
		for _, base := range []string{"", "   ", "\n\n", "\t\n "} {
			if got := withArchitectPersona(base); got != architectPersona {
				t.Fatalf("withArchitectPersona(%q) did not yield the persona alone", base)
			}
		}
	})
}

func assertPersonaLeads(t *testing.T, systemPrompt string) {
	t.Helper()
	idx := strings.Index(systemPrompt, architectPersona)
	if idx < 0 {
		t.Fatalf("system prompt does not carry the architect persona (%d bytes)", len(systemPrompt))
	}
	if idx != architectPersonaOffset {
		t.Fatalf("architect persona at offset %d, want %d; prefix was %q",
			idx, architectPersonaOffset, systemPrompt[:idx])
	}
}

// TestArchitectPersonaHasOneDeliverySeam is the structural half of the
// guarantee. The persona can only be lost if there is more than one way to
// deliver it, so there is exactly one: cmd/nerd/chat/persona.go. Every other
// file in this package must go through withArchitectPersona and must never name
// the constant itself.
func TestArchitectPersonaHasOneDeliverySeam(t *testing.T) {
	// \b keeps this from matching withArchitectPersona / architectPersonaTokens
	// / architectPersonaOffset, which are the sanctioned API.
	constRef := regexp.MustCompile(`\barchitectPersona\b`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if name == "persona.go" || name == "persona_test.go" {
			continue
		}
		data, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if constRef.Match(data) {
			t.Errorf("%s names the architectPersona constant directly. "+
				"There is exactly one delivery seam, cmd/nerd/chat/persona.go: call "+
				"withArchitectPersona(base) so the persona always lands at the top and is "+
				"always counted. A second path is how it gets lost.", name)
		}
	}
}
