package config

import "testing"

func TestValidateIgnorePattern(t *testing.T) {
	cases := []struct {
		p   string
		bad bool
	}{
		{p: "node_modules"},
		{p: "build/"},
		{p: "/src/gen/**"},
		{p: "!keep.go"},
		{p: `src\gen\**`},
		{p: "foo/[abc]"},
		{p: "", bad: true},
		{p: "   ", bad: true},
		{p: "!", bad: true},
		{p: "! ", bad: true},
		{p: "foo//bar", bad: true},
		{p: "foo/[abc", bad: true},
		{p: "/", bad: true},
	}
	for _, tc := range cases {
		err := ValidateIgnorePattern(tc.p)
		if tc.bad && err == nil {
			t.Errorf("ValidateIgnorePattern(%q) = nil, want error", tc.p)
		}
		if !tc.bad && err != nil {
			t.Errorf("ValidateIgnorePattern(%q) = %v", tc.p, err)
		}
	}
}

func TestWorldConfigCheck_IgnorePatterns(t *testing.T) {
	c := WorldConfig{IgnorePatterns: []string{"vendor", "!", "build/"}}
	probs := c.Check("")
	if len(probs) != 1 {
		t.Fatalf("Check = %+v, want one problem", probs)
	}
	if probs[0].Severity != SeverityError {
		t.Errorf("severity = %q", probs[0].Severity)
	}
	if probs[0].Path != "world.ignore_patterns[1]" {
		t.Errorf("path = %q", probs[0].Path)
	}
	if probs[0].Message == "" || probs[0].Fix == "" {
		t.Errorf("problem missing message or fix: %+v", probs[0])
	}
	prefixed := c.Check("profiles.x")
	if len(prefixed) != 1 || prefixed[0].Path != "profiles.x.ignore_patterns[1]" {
		t.Fatalf("prefixed Check = %+v", prefixed)
	}
	if probs := (WorldConfig{}).Check("world"); probs != nil {
		t.Fatalf("empty patterns = %+v", probs)
	}
}
