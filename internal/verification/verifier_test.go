package verification

import "testing"

func TestPersonaAtom_NormalizesToLowercaseAtom(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercases", input: "Coder", want: "/coder"},
		{name: "trims spaces and slashes", input: "  /Reviewer  ", want: "/reviewer"},
		{name: "replaces spaces", input: "Coder Shard", want: "/coder_shard"},
		{name: "replaces dashes", input: "my-persona", want: "/my_persona"},
		{name: "already normalized", input: "/coder", want: "/coder"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(personaAtom(tc.input)); got != tc.want {
				t.Fatalf("personaAtom(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPersonaAtom_EmptyFallsBackToNone(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "spaces only", input: "   "},
		{name: "slash only", input: "/"},
		{name: "slash and spaces", input: "  /  "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(personaAtom(tc.input)); got != "/none" {
				t.Fatalf("personaAtom(%q) = %q, want %q", tc.input, got, "/none")
			}
		})
	}
}

func TestPercent_ConvertsValidConfidence(t *testing.T) {
	cases := []struct {
		name  string
		input float64
		want  int64
	}{
		{name: "zero", input: 0, want: 0},
		{name: "half", input: 0.5, want: 50},
		{name: "ninety five", input: 0.95, want: 95},
		{name: "one", input: 1, want: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := percent(tc.input); got != tc.want {
				t.Fatalf("percent(%v) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestOutcomeAtom_NormalizesToLowercaseAtom(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "already normalized", input: "/complete", want: "/complete"},
		{name: "lowercases", input: "/Complete", want: "/complete"},
		{name: "lowercases all caps", input: "/COMPLETE", want: "/complete"},
		{name: "trims spaces", input: "  /complete  ", want: "/complete"},
		{name: "trims and lowercases", input: "  /Complete  ", want: "/complete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(outcomeAtom(tc.input)); got != tc.want {
				t.Fatalf("outcomeAtom(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestOutcomeAtom_FallsBackToNone(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "blank", input: "   "},
		{name: "bare slash", input: "/"},
		{name: "bare slash with spaces", input: "  /  "},
		{name: "missing slash", input: "complete"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(outcomeAtom(tc.input)); got != "/none" {
				t.Fatalf("outcomeAtom(%q) = %q, want %q", tc.input, got, "/none")
			}
		})
	}
}
