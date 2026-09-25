package main

import (
	"regexp"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// --yolo's help text promises "never overrides safety denials", and nothing in
// cmd/nerd enforces it: the promise lives in the policy corpus, as the rule
// that yolo_mode is read only by the clarification machinery -- it silences
// questions, it never grants or removes a permission. This test is that rule.
// A clause that reads yolo_mode must be negated and must conclude a question,
// never a verdict; a new consumer of yolo_mode has to be added here, where the
// promise is.
func TestYoloMode_OnlySilencesQuestions(t *testing.T) {
	schemas, policy, err := core.DefaultCorpusText()
	if err != nil {
		t.Fatalf("DefaultCorpusText: %v", err)
	}
	consumers, violations := yoloViolations(schemas + "\n" + policy)
	for _, v := range violations {
		t.Error(v)
	}
	// clarification.mg and learning.mg hold nineteen today; far fewer means
	// the clause splitter stopped seeing them and the test went blind.
	if consumers < 10 {
		t.Fatalf("only %d clauses read yolo_mode: the checker no longer sees the clarification rules", consumers)
	}
}

// The checker itself: a rule that lets yolo remove a block, or grant a
// permission, is caught.
func TestYoloMode_CheckerCatchesAWideningRule(t *testing.T) {
	src := `
# yolo waves a dangerous action through
blocked(A) :- dangerous(A), !yolo_mode().
permitted(A) :- requested(A), yolo_mode().
clarification_question(/current_intent, "Which file?") :- ambiguous(), !yolo_mode().
`
	consumers, violations := yoloViolations(src)
	if consumers != 3 || len(violations) != 3 {
		t.Fatalf("consumers=%d violations=%v; want 3 consumers and 3 violations (two widening heads, one positive read)", consumers, violations)
	}
}

var yoloAllowedHead = regexp.MustCompile(`^(clarification_question|clarification_option)\(|^next_action\(/interrogative_mode\)`)

// yoloViolations lists every clause of src that reads yolo_mode for anything
// but silencing a question.
func yoloViolations(src string) (consumers int, violations []string) {
	for _, clause := range corpusClauses(src) {
		head, body, isRule := strings.Cut(clause, ":-")
		if !isRule || !strings.Contains(body, "yolo_mode") {
			continue
		}
		consumers++
		head = strings.TrimSpace(head)
		if !yoloAllowedHead.MatchString(head) {
			violations = append(violations, "yolo_mode decides "+head+"; it may only silence a question: "+clause)
		}
		if strings.Count(body, "yolo_mode") != strings.Count(body, "!yolo_mode()") {
			violations = append(violations, "yolo_mode is read positively (it would enable, not silence): "+clause)
		}
	}
	return consumers, violations
}

// corpusClauses splits Mangle source into clauses: comment-stripped lines
// accumulated until one ends a statement with '.'.
func corpusClauses(src string) []string {
	var out []string
	var cur strings.Builder
	for line := range strings.Lines(src) {
		if i := strings.Index(line, "#"); i >= 0 && !strings.Contains(line[:i], `"`) {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cur.WriteString(line)
		cur.WriteByte(' ')
		if strings.HasSuffix(line, ".") {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	return out
}
