package system

import (
	"strings"
	"testing"
)

func TestTruncateRule(t *testing.T) {
	// Newlines are flattened to spaces.
	if got := truncateRule("a\nb"); got != "a b" {
		t.Errorf("truncateRule(a\\nb)=%q, want 'a b'", got)
	}
	// Short rules pass through unchanged.
	short := "foo(X) :- bar(X)."
	if got := truncateRule(short); got != short {
		t.Errorf("truncateRule kept-as-is got %q", got)
	}
	// Long rules are clipped to 80 chars + ellipsis.
	long := strings.Repeat("x", 200)
	got := truncateRule(long)
	if len(got) != 83 || !strings.HasSuffix(got, "...") {
		t.Errorf("truncateRule(long) len=%d suffix-ok=%v, want 83 with ellipsis", len(got), strings.HasSuffix(got, "..."))
	}
}

func TestParseProposedRule(t *testing.T) {
	e := &ExecutivePolicyShard{}
	output := "Here is my proposal:\n" +
		"RULE: next_action(/fix, X) :- failing_test(X).\n" +
		"CONFIDENCE: 0.85\n" +
		"RATIONALE: failing tests should be fixed first\n"
	pr := e.parseProposedRule(output, []UnhandledCase{{Query: "q"}})
	if pr.MangleCode != "next_action(/fix, X) :- failing_test(X)." {
		t.Errorf("MangleCode=%q", pr.MangleCode)
	}
	if pr.Confidence < 0.84 || pr.Confidence > 0.86 {
		t.Errorf("Confidence=%v, want ~0.85", pr.Confidence)
	}
	if pr.Rationale != "failing tests should be fixed first" {
		t.Errorf("Rationale=%q", pr.Rationale)
	}
	if len(pr.BasedOn) != 1 {
		t.Errorf("BasedOn should carry the originating cases, got %d", len(pr.BasedOn))
	}
}

func TestBuildPolicyProposalPrompt(t *testing.T) {
	e := &ExecutivePolicyShard{}
	prompt := e.buildPolicyProposalPrompt([]UnhandledCase{
		{Query: "next_action(?a)", Context: map[string]string{"intent": "/fix"}},
	})
	if !strings.Contains(prompt, "next_action(?a)") {
		t.Error("prompt should include the unhandled query")
	}
	if !strings.Contains(prompt, "RULE:") || !strings.Contains(prompt, "CONFIDENCE:") {
		t.Error("prompt should specify the expected RULE/CONFIDENCE response format")
	}
}

func TestMangleRepair_ExtractRule(t *testing.T) {
	m := &MangleRepairShard{}

	// From a fenced code block (with a language tag line).
	fenced := "Here:\n```mangle\nfoo(X) :- bar(X).\n```\n"
	if got := m.extractRule(fenced); got != "foo(X) :- bar(X)." {
		t.Errorf("extractRule(fenced)=%q", got)
	}

	// From a bare rule-looking line among prose.
	prose := "# comment\nThis is text\nbaz(Y) :- qux(Y).\n"
	if got := m.extractRule(prose); got != "baz(Y) :- qux(Y)." {
		t.Errorf("extractRule(prose)=%q", got)
	}
}

func TestMangleRepair_BuildRepairSemanticQuery(t *testing.T) {
	m := &MangleRepairShard{}
	if got := m.buildRepairSemanticQuery(nil); got != "" {
		t.Errorf("empty errors -> empty query, got %q", got)
	}
	q := m.buildRepairSemanticQuery([]string{"undefined predicate foo", "stratification violation"})
	// Always seeded with the base keywords.
	for _, kw := range []string{"mangle", "rule", "repair", "undefined predicate", "stratification"} {
		if !strings.Contains(q, kw) {
			t.Errorf("query %q missing keyword %q", q, kw)
		}
	}
	// Base keywords are not duplicated.
	if strings.Count(q, "mangle") != 1 {
		t.Errorf("base keyword 'mangle' duplicated in %q", q)
	}
}
