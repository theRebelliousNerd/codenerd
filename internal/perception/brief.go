package perception

import (
	"strings"

	"codenerd/internal/types"
)

// Brief is what the conversation has settled about the work: the goal, the
// user's constraints, the decisions made, and the questions still open.
// Perception receives the current brief with each turn and returns it whole,
// updated; the chat keeps it in the kernel (policy/brief.mg), where it
// grounds every shard and, in yolo mode, decides whether work may start.
//
// Ready is the main agent's judgment that the brief is enough to start the
// work. The kernel never accepts it beside an open question.
type Brief struct {
	Goal          string   `json:"goal"`
	Constraints   []string `json:"constraints,omitempty"`
	Decisions     []string `json:"decisions,omitempty"`
	OpenQuestions []string `json:"open_questions,omitempty"`
	Ready         bool     `json:"ready"`
}

// BriefPredicates are the base facts Facts writes, for retract-and-replace.
var BriefPredicates = []string{"brief_goal", "brief_constraint", "brief_decision", "brief_open_question", "brief_marked_ready"}

// Empty reports a brief with nothing in it: no goal and nothing settled.
func (b *Brief) Empty() bool {
	return b == nil || (strings.TrimSpace(b.Goal) == "" && len(b.Constraints) == 0 && len(b.Decisions) == 0 && len(b.OpenQuestions) == 0)
}

// Facts projects the brief as brief.mg's base facts. Blank entries are left
// out; they would state nothing.
func (b *Brief) Facts() []types.Fact {
	if b.Empty() {
		return nil
	}
	var out []types.Fact
	add := func(pred, text string) {
		if text = strings.TrimSpace(text); text != "" {
			out = append(out, types.Fact{Predicate: pred, Args: []any{types.MangleString(text)}})
		}
	}
	add("brief_goal", b.Goal)
	for _, c := range b.Constraints {
		add("brief_constraint", c)
	}
	for _, d := range b.Decisions {
		add("brief_decision", d)
	}
	for _, q := range b.OpenQuestions {
		add("brief_open_question", q)
	}
	if b.Ready {
		out = append(out, types.Fact{Predicate: "brief_marked_ready"})
	}
	return out
}

// BriefFromFacts reads the brief back from its facts (any order).
func BriefFromFacts(facts []types.Fact) *Brief {
	b := &Brief{}
	for _, f := range facts {
		if f.Predicate == "brief_marked_ready" {
			b.Ready = true
			continue
		}
		if len(f.Args) != 1 {
			continue
		}
		text := types.ExtractString(f.Args[0])
		switch f.Predicate {
		case "brief_goal":
			b.Goal = text
		case "brief_constraint":
			b.Constraints = append(b.Constraints, text)
		case "brief_decision":
			b.Decisions = append(b.Decisions, text)
		case "brief_open_question":
			b.OpenQuestions = append(b.OpenQuestions, text)
		}
	}
	if b.Empty() {
		return nil
	}
	return b
}

// Render is the brief as the prompt section perception and the main agent
// read. An empty brief renders as "(empty)" so the model knows to start one.
func (b *Brief) Render() string {
	if b.Empty() {
		return "(empty: no work discussed yet)"
	}
	var sb strings.Builder
	sb.WriteString("- Goal: " + strings.TrimSpace(b.Goal) + "\n")
	list := func(label string, items []string) {
		for _, it := range items {
			sb.WriteString("- " + label + ": " + strings.TrimSpace(it) + "\n")
		}
	}
	list("Constraint", b.Constraints)
	list("Decision", b.Decisions)
	list("Open question", b.OpenQuestions)
	if b.Ready {
		sb.WriteString("- Ready: yes\n")
	} else {
		sb.WriteString("- Ready: no\n")
	}
	return sb.String()
}
