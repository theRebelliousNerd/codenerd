package transpiler

import (
	"strings"
	"testing"
)

func TestSanitizeAtoms(t *testing.T) {
	s := NewSanitizer()

	input := `research_topic("gemini", "testing", "pending").`
	expected := `research_topic(/gemini, "testing", /pending).`

	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize failed: %v", err)
	}

	t.Logf("SanitizeAtoms Output: %q", output)
	t.Logf("SanitizeAtoms Expected: %q", expected)

	output = strings.TrimSpace(output)
	if normalize(output) != normalize(expected) {
		t.Errorf("Mismatch")
	}
}

func normalize(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

func TestInjectSafety(t *testing.T) {
	s := NewSanitizer()

	input := `unsafe(X) :- !safe(X).`

	output, err := s.Sanitize(input)
	if err != nil {
		t.Logf("Retrying with 'not'...")
		input = `unsafe(X) :- not safe(X).`
		output, err = s.Sanitize(input)
		if err != nil {
			t.Fatalf("Sanitize failed: %v", err)
		}
	}

	t.Logf("Safety Output: %q", output)

	if !strings.Contains(output, "candidate_node(X)") {
		t.Errorf("Safety injection missing")
	}
}

func TestAggregationRepair(t *testing.T) {
	s := NewSanitizer()

	input := `count_topics(Agent, C) :- research_topic(Agent, _, _), C = count(Agent).`
	expectedPipeFragment := `|> do fn:group_by(Agent), let C = fn:count(Agent)`

	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize failed: %v", err)
	}

	t.Logf("Aggregation Output: %q", output)

	if !strings.Contains(normalize(output), normalize(expectedPipeFragment)) {
		t.Errorf("Pipe syntax mismatch")
	}
}

func TestMixedSanitization(t *testing.T) {
	s := NewSanitizer()

	input := `agent_stats(Name, Total) :- research_topic(Name, "foo", "active"), Total = count(Name).`

	output, err := s.Sanitize(input)
	if err != nil {
		t.Fatalf("Sanitize failed: %v", err)
	}

	t.Logf("Mixed Output: %q", output)

	if !strings.Contains(output, `research_topic(Name, "foo", /active)`) {
		t.Errorf("Atom interning failed")
	}

	if !strings.Contains(output, "|>") {
		t.Errorf("Pipe missing")
	}
}
