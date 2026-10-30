package session

import (
	"fmt"
	"os"
	"path/filepath"
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
	// Handoffs are the answer's HANDOFF lines: edge-bound advice between
	// two files (canonical), dealt to the steps on either end.
	Handoffs []cardHandoff
}

// cardHandoff is one HANDOFF line of a card: a change in From obliges
// Contract of To.
type cardHandoff struct {
	From, To, Contract string
}

// handoffPrefix opens a hand-off line in a consultation's answer.
const handoffPrefix = "HANDOFF "

// parseHandoffs reads "HANDOFF <from> -> <to>: <contract>" lines out of an
// answer, canonicalizing both files against workspace. A line missing a file
// or the contract is not a hand-off.
func parseHandoffs(workspace, answer string) []cardHandoff {
	var out []cardHandoff
	for _, line := range strings.Split(answer, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), handoffPrefix)
		if !ok {
			continue
		}
		ends, contract, ok := strings.Cut(rest, ": ")
		if !ok {
			continue
		}
		from, to, ok := strings.Cut(ends, "->")
		from = types.CanonicalPath(workspace, strings.Trim(strings.TrimSpace(from), "`'\""))
		to = types.CanonicalPath(workspace, strings.Trim(strings.TrimSpace(to), "`'\""))
		contract = strings.TrimSpace(contract)
		if !ok || from == "" || to == "" || from == to || contract == "" {
			continue
		}
		out = append(out, cardHandoff{From: from, To: to, Contract: contract})
	}
	return out
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
		for i, h := range c.Handoffs {
			e.assertTurnFact(types.Fact{Predicate: "turn_card_handoff", Args: []any{turn, agent, int64(i), h.From, h.To}})
		}
	}
}

// cardHandoffAt is the hand-off a turn_handoff_* row names.
func (r *ExecutionResult) cardHandoffAt(agent string, idx int64) (cardHandoff, bool) {
	for _, c := range r.expertCards {
		if experts.AgentKey(c.Agent) != agent {
			continue
		}
		if idx >= 0 && int(idx) < len(c.Handoffs) {
			return c.Handoffs[idx], true
		}
	}
	return cardHandoff{}, false
}

// handoffRow reads a turn_handoff_{step,unresolved} row: agent key, index and
// consumer file.
func handoffRow(row types.Fact) (string, int64, string, bool) {
	agent, okA := nthArg(row.Args, 1)
	idx, okI := nthArg(row.Args, 2)
	to, okT := nthArg(row.Args, 3)
	n, okN := idx.(int64)
	if !okA || !okI || !okT || !okN {
		return "", 0, "", false
	}
	return experts.AgentKey(types.ExtractString(agent)), n, types.ExtractString(to), true
}

// reconcileHandoffs checks the plan against the hand-offs the turn's cards
// named (turn_handoff_step / turn_handoff_unresolved, policy/shard_experts.mg):
// a hand-off out of a planned file into an existing file the plan does not
// edit becomes a step on that file, placed after the planned steps and bound
// by maxSteps like them (past it, the step is Beyond: owed, reported); one
// into a file that is not on disk is returned as an unresolved note for the
// step report.
func (e *Executor) reconcileHandoffs(result *ExecutionResult, steps []workStep, maxSteps int) ([]workStep, string) {
	if e.kernel == nil || result == nil || len(result.expertCards) == 0 {
		return steps, ""
	}
	turn := result.turnAtom()
	workspace := e.workspaceForVerification()
	for _, s := range steps {
		e.assertTurnFact(types.Fact{Predicate: "turn_plan_file", Args: []any{turn, types.CanonicalPath(workspace, s.File)}})
	}
	for _, c := range result.expertCards {
		for _, h := range c.Handoffs {
			if _, err := os.Stat(filepath.Join(workspace, filepath.FromSlash(h.To))); err == nil {
				e.assertTurnFact(types.Fact{Predicate: "turn_handoff_target_exists", Args: []any{turn, h.To}})
			}
		}
	}
	stepRows, err := e.turnRows("turn_handoff_step", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("reconciling the plan with the experts' hand-offs failed (%v); the plan runs as planned", err)
		return steps, ""
	}
	run, beyond := splitPlan(steps)
	added := map[string]bool{}
	for _, row := range stepRows {
		agent, idx, to, ok := handoffRow(row)
		h, found := result.cardHandoffAt(agent, idx)
		if !ok || !found || added[to] {
			continue
		}
		added[to] = true
		step := workStep{File: to, Change: fmt.Sprintf("carry the hand-off from %s (%s): %s", h.From, agent, h.Contract)}
		if len(run) < maxSteps {
			run = append(run, step)
		} else {
			step.Beyond = true
			beyond = append(beyond, step)
		}
		logging.Session("Plan gains a step on %s: hand-off from %s named by %s", to, h.From, agent)
	}
	var note strings.Builder
	if rows, err := e.turnRows("turn_handoff_unresolved", turn); err == nil {
		for _, row := range rows {
			agent, idx, to, ok := handoffRow(row)
			h, found := result.cardHandoffAt(agent, idx)
			if !ok || !found {
				continue
			}
			fmt.Fprintf(&note, "\nUnresolved hand-off: %s (%s) says %s obliges %s, which is not on disk: %s", agent, consultationRole(result.cardMode(agent)), h.From, to, h.Contract)
		}
	}
	return append(run, beyond...), note.String()
}

// cardMode is the spawn mode of the agent's card.
func (r *ExecutionResult) cardMode(agent string) string {
	for _, c := range r.expertCards {
		if experts.AgentKey(c.Agent) == agent {
			return c.Mode
		}
	}
	return ""
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
	var handoffs strings.Builder
	if rows, err := e.turnRows("turn_handoff_dealt", turn); err == nil {
		seen := map[string]bool{}
		for _, row := range rows {
			stepFile, okF := nthArg(row.Args, 1)
			agent, okA := nthArg(row.Args, 2)
			idx, okI := nthArg(row.Args, 3)
			n, okN := idx.(int64)
			if !okF || !okA || !okI || !okN || types.ExtractString(stepFile) != canonical {
				continue
			}
			key := experts.AgentKey(types.ExtractString(agent))
			h, found := result.cardHandoffAt(key, n)
			id := fmt.Sprintf("%s#%d", key, n)
			if !found || seen[id] {
				continue
			}
			seen[id] = true
			fmt.Fprintf(&handoffs, "  - %s -> %s (%s): %s\n", h.From, h.To, key, h.Contract)
		}
	}
	logging.Session("Step on %s: dealt %v, not dealt %v", canonical, delivered, withheld)
	var b strings.Builder
	b.WriteString(consultationHeader)
	b.WriteString(hand.String())
	if handoffs.Len() > 0 {
		b.WriteString("\nHand-offs this step is an end of (what a change on one side obliges of the other):\n")
		b.WriteString(handoffs.String())
	}
	if left.Len() > 0 {
		b.WriteString("\nConsulted for this turn but not dealt to this step:\n")
		b.WriteString(left.String())
	}
	return b.String()
}
