package context

import (
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

func bigFacts(n int, argLen int) []core.Fact {
	facts := make([]core.Fact, n)
	for i := range facts {
		facts[i] = core.Fact{
			Predicate: "permitted",
			Args:      []any{"actor", strings.Repeat("z", argLen)},
		}
	}
	return facts
}

// The maxLineLength cut lived only in serializeGrouped, so calling
// WithGrouping(false) silently removed the only per-fact length bound in the
// injected context block. A fact's arguments are arbitrary strings written by
// whatever asserted them — a tool result, a path, an error message.
func TestFactSerializer_PerFactBoundOnBothPaths(t *testing.T) {
	facts := bigFacts(3, 50_000)

	tests := []struct {
		name    string
		grouped bool
	}{
		{name: "grouped", grouped: true},
		{name: "flat", grouped: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFactSerializer().WithGrouping(tt.grouped).SerializeFacts(facts)
			for _, line := range strings.Split(got, "\n") {
				if strings.HasPrefix(line, "#") || line == "" {
					continue
				}
				if len(line) > 500 {
					t.Fatalf("%s path emitted a %d-char fact line; the per-fact bound must apply to both paths",
						tt.name, len(line))
				}
			}
		})
	}
}

func TestFactSerializer_SerializeScoredFactsIsBounded(t *testing.T) {
	scored := make([]ScoredFact, 5)
	for i, f := range bigFacts(5, 50_000) {
		scored[i] = ScoredFact{Fact: f, Score: 10}
	}
	got := NewFactSerializer().SerializeScoredFacts(scored, false)
	for _, line := range strings.Split(got, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 500 {
			t.Fatalf("scored fact line is %d chars; the per-fact bound is not applied", len(line))
		}
	}
}

// Nothing downstream re-checks the size of this block. ContextBlockBuilder.Build
// MEASURES it (TokenUsage) but enforces nothing, and TokenBudget.Allocate — the
// API that would have rejected an over-budget category — has no production
// caller. CheckTotalBudget then runs at the START of the next BuildContext,
// which means an over-budget block is always shipped once before anyone notices.
func TestSerializeCompressedContext_Bounds(t *testing.T) {
	tests := []struct {
		name       string
		ctx        *CompressedContext
		wantMarker bool
		mustKeep   []string
	}{
		{
			name: "a normal block is untouched",
			ctx: &CompressedContext{
				CoreFacts:      "permitted(/read, \"main.go\").",
				ContextAtoms:   "modified(\"main.go\").",
				HistorySummary: "user asked to fix the router",
			},
			mustKeep: []string{"permitted(/read", "modified(", "fix the router"},
		},
		{
			name: "a session that accumulated thousands of permitted rows is capped",
			ctx: &CompressedContext{
				CoreFacts:    NewFactSerializer().SerializeFacts(bigFacts(20_000, 60)),
				ContextAtoms: "modified(\"main.go\").",
			},
			wantMarker: true,
			mustKeep:   []string{"MANGLE CONTEXT BLOCK"},
		},
		{
			name: "an oversized rolling summary is capped",
			ctx: &CompressedContext{
				HistorySummary: strings.Repeat("H", 5_000_000),
			},
			wantMarker: true,
			mustKeep:   []string{"MANGLE CONTEXT BLOCK"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFactSerializer().SerializeCompressedContext(tt.ctx)

			if len(got) > maxContextBlockChars+512 {
				t.Errorf("context block is %d chars, cap is %d", len(got), maxContextBlockChars)
			}
			if prompt.IsClamped(got) != tt.wantMarker {
				t.Errorf("IsClamped = %v, want %v", prompt.IsClamped(got), tt.wantMarker)
			}
			for _, want := range tt.mustKeep {
				if !strings.Contains(got, want) {
					t.Errorf("block lost %q", want)
				}
			}
		})
	}
}
