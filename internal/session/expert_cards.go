package session

import (
	"fmt"
	"strings"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// An expert card is one consultation the turn bought, kept as a typed result
// rather than prose appended to one prompt: the agent, its spawn mode, how the
// consultation ended, the answer, and the task files the agent was selected
// for. A planned turn compiles a fresh prompt per step, and the kernel deals
// each step its hand (turn_card_dealt, policy/shard_experts.mg): a lead's card
// to every step, an adviser's card to the steps on the files its domain
// covers. The step's prompt names every card it was not dealt, and why, so
// what was left out is visible rather than silently missing.

// Card dispositions, the value set of turn_expert_card/4's Disposition.
const (
	cardAnswered = "/answered"
	cardFailed   = "/failed"
	cardEmpty    = "/empty"
)

// expertCard is one consultation's typed outcome.
type expertCard struct {
	Agent        string
	Mode         string // experts.SpawnLead or experts.SpawnAdvise
	Technologies []string
	Files        []string // canonical task files the agent was selected for
	Disposition  string
	Body         string // the answer, or why there is none
}

// newExpertCard types one consultation's outcome.
func newExpertCard(r expertConsultation) expertCard {
	card := expertCard{
		Agent: r.need.Agent, Mode: r.need.Spawn,
		Technologies: r.need.Technologies, Files: r.need.Files,
	}
	switch {
	case r.err != nil:
		card.Disposition, card.Body = cardFailed, fmt.Sprintf("The consultation did not complete: %v", r.err)
	case strings.TrimSpace(r.answer) == "":
		card.Disposition, card.Body = cardEmpty, "The consultation returned no answer."
	default:
		card.Disposition, card.Body = cardAnswered, strings.TrimSpace(r.answer)
	}
	return card
}

// render is the card as the model reads it.
func (c expertCard) render() string {
	return fmt.Sprintf("\n### %s (%s; %s)\n%s\n", c.Agent, consultationRole(c.Mode), strings.Join(c.Technologies, ", "), c.Body)
}

// consultationHeader opens a consultation section.
const consultationHeader = "\n\n## Shard agent consultation\n" +
	"The harness consulted these shard agents before this turn because the task's files are in their domain (expert_spawn). Weigh their reading; you still own the result and its verification.\n"

// recordCards publishes the turn's cards to the kernel, which deals them.
func (e *Executor) recordCards(result *ExecutionResult, cards []expertCard) {
	if result == nil {
		return
	}
	result.expertCards = append(result.expertCards, cards...)
	if e.kernel == nil {
		return
	}
	turn := result.turnAtom()
	for _, c := range cards {
		agent := experts.AgentAtom(c.Agent)
		e.assertTurnFact(types.Fact{Predicate: "turn_expert_card", Args: []any{
			turn, agent, types.MangleAtom("/" + c.Mode), types.MangleAtom(c.Disposition)}})
		for _, f := range c.Files {
			e.assertTurnFact(types.Fact{Predicate: "turn_expert_card_file", Args: []any{turn, agent, f}})
		}
	}
}

// dealtCards is the consultation section of one planned step's prompt: the
// cards the kernel dealt the step's file, then a line for each card it was
// not dealt. It is empty when the turn bought no consultation.
func (e *Executor) dealtCards(result *ExecutionResult, file string) string {
	if result == nil || len(result.expertCards) == 0 || e.kernel == nil {
		return ""
	}
	turn := result.turnAtom()
	canonical := types.CanonicalPath(e.workspaceForVerification(), file)
	e.assertTurnFact(types.Fact{Predicate: "turn_step_file", Args: []any{turn, canonical}})
	rows, err := e.turnRows("turn_card_dealt", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("dealing the turn's consultations to the step on %s failed (%v); the step gets none", file, err)
		return ""
	}
	dealt := map[string]bool{}
	for _, row := range rows {
		dealtFile, okFile := nthArg(row.Args, 1)
		agent, okAgent := nthArg(row.Args, 2)
		if okFile && okAgent && types.ExtractString(dealtFile) == canonical {
			dealt[experts.AgentKey(types.ExtractString(agent))] = true
		}
	}
	var hand, left strings.Builder
	var delivered, withheld []string
	for _, c := range result.expertCards {
		if dealt[experts.AgentKey(c.Agent)] {
			hand.WriteString(c.render())
			delivered = append(delivered, c.Agent)
			continue
		}
		withheld = append(withheld, c.Agent)
		switch c.Disposition {
		case cardAnswered:
			fmt.Fprintf(&left, "  - %s %s %s, not this step's file\n", c.Agent, consultationRole(c.Mode), strings.Join(c.Files, ", "))
		default:
			fmt.Fprintf(&left, "  - %s: %s\n", c.Agent, c.Body)
		}
	}
	logging.Session("Step on %s: dealt %v, not dealt %v", canonical, delivered, withheld)
	var b strings.Builder
	b.WriteString(consultationHeader)
	b.WriteString(hand.String())
	if left.Len() > 0 {
		b.WriteString("\nConsulted for this turn but not dealt to this step:\n")
		b.WriteString(left.String())
	}
	return b.String()
}
