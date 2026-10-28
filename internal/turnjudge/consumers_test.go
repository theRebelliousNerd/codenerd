package turnjudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every judgment Jev is asked about a turn is read by a rule: a question no
// rule reads costs tokens and decides nothing. The rules read a judgment by
// its quoted name (jev_yes("x"), jev_no("x"), side_turn_judgment("x", P)),
// and answered_N through jev_judgment_about.
func TestEveryTurnJudgmentHasAConsumingRule(t *testing.T) {
	var policy strings.Builder
	for _, glob := range []string{"../core/defaults/*.mg", "../core/defaults/policy/*.mg"} {
		files, err := filepath.Glob(glob)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(string(data), "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "#") {
					policy.WriteString(line + "\n")
				}
			}
		}
	}
	rules := policy.String()
	if rules == "" {
		t.Fatal("no policy found")
	}
	names := []string{ApprovesHeldWork, AddsToHeldWork}
	for name := range fixedQuestions() {
		names = append(names, name)
	}
	for _, name := range names {
		if !strings.Contains(rules, `"`+name+`"`) {
			t.Errorf("judgment %s is asked but no rule reads it", name)
		}
	}
	if !strings.Contains(rules, "jev_judgment_about(Name, Question)") {
		t.Error("answered_N judgments have no consuming rule")
	}
}
