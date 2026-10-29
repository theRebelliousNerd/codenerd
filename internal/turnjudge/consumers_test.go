package turnjudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/typesafe"
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
	names := []string{ApprovesHeldWork, AddsToHeldWork, WantsToStop}
	for name, q := range fixedQuestions() {
		if _, choice := q.(typesafe.Choice); choice {
			continue // a choice is read through jev_chosen; checked below
		}
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
	if !strings.Contains(rules, `jev_chosen("`+ConversationAct+`", Option)`) {
		t.Errorf("the %s choice has no consuming rule", ConversationAct)
	}
	// Every option the act question offers has its act in the kernel's
	// vocabulary, and the vocabulary offers nothing the question does not ask.
	q := conversationActQuestion().(typesafe.Choice)
	if len(q.Criteria) != len(ConversationActs) {
		t.Errorf("the question offers %d options, ConversationActs lists %d", len(q.Criteria), len(ConversationActs))
	}
	for _, act := range ConversationActs {
		if _, ok := q.Criteria[act]; !ok {
			t.Errorf("act %s is listed but not offered", act)
		}
		if !strings.Contains(rules, `conversation_act_option("`+act+`", /`+act+`)`) {
			t.Errorf("act %s has no conversation_act_option row", act)
		}
	}
	if got, want := strings.Count(rules, "conversation_act_option(\""), len(ConversationActs); got != want {
		t.Errorf("the kernel maps %d options, the question offers %d", got, want)
	}
}
