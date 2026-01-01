package perception

import (
	"math"
	"testing"
)

func TestExtractFactFromResponse_JSONNoise(t *testing.T) {
	response := `Some analysis {"mangle_updates":["learned_exemplar(\"Add a feature\", /create, \"feature\", \"ensure: wired to the TUI, log a note\", 0.90)."]} trailing text`
	want := `learned_exemplar(\"Add a feature\", /create, \"feature\", \"ensure: wired to the TUI, log a note\", 0.90).`

	got := ExtractFactFromResponse(response)
	if got != want {
		t.Fatalf("ExtractFactFromResponse() = %q, want %q", got, want)
	}
}

func TestExtractFactFromResponse_ParensInQuotes(t *testing.T) {
	response := `prefix learned_exemplar("Add a feature", /create, "feature", "ensure: wired (TUI)", 0.90). suffix`
	want := `learned_exemplar("Add a feature", /create, "feature", "ensure: wired (TUI)", 0.90).`

	got := ExtractFactFromResponse(response)
	if got != want {
		t.Fatalf("ExtractFactFromResponse() = %q, want %q", got, want)
	}
}

func TestParseLearnedFact_AllowsCommaAndQuotes(t *testing.T) {
	tests := []struct {
		name       string
		fact       string
		pattern    string
		verb       string
		target     string
		constraint string
		confidence float64
	}{
		{
			name:       "comma in constraint",
			fact:       `learned_exemplar("Add a feature", /create, "feature", "ensure: wired to the TUI, log a note", 0.90).`,
			pattern:    "Add a feature",
			verb:       "/create",
			target:     "feature",
			constraint: "ensure: wired to the TUI, log a note",
			confidence: 0.90,
		},
		{
			name:       "escaped quotes",
			fact:       `learned_exemplar("Add \"quoted\" feature", /create, "feature", "ensure: \"wired\" to the TUI", 0.75).`,
			pattern:    `Add "quoted" feature`,
			verb:       "/create",
			target:     "feature",
			constraint: `ensure: "wired" to the TUI`,
			confidence: 0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, verb, target, constraint, confidence, err := ParseLearnedFact(tt.fact)
			if err != nil {
				t.Fatalf("ParseLearnedFact() error = %v", err)
			}
			if pattern != tt.pattern {
				t.Fatalf("pattern = %q, want %q", pattern, tt.pattern)
			}
			if verb != tt.verb {
				t.Fatalf("verb = %q, want %q", verb, tt.verb)
			}
			if target != tt.target {
				t.Fatalf("target = %q, want %q", target, tt.target)
			}
			if constraint != tt.constraint {
				t.Fatalf("constraint = %q, want %q", constraint, tt.constraint)
			}
			if math.Abs(confidence-tt.confidence) > 0.0001 {
				t.Fatalf("confidence = %f, want %f", confidence, tt.confidence)
			}
		})
	}
}
