package session

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

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
	// cardKnowledge: not consulted, the expert's knowledge rows were already
	// in the prompt (routeKnowledge).
	cardKnowledge = "/knowledge"
	// cardUnresolved: not consulted, the question is left open
	// (routeUnresolved).
	cardUnresolved = "/unresolved"
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
	// Route is how the routing ladder settled this card (consult_route).
	Route string
	// Mandates are the answer's REQUIRE lines: tests the expert says must
	// pass. Proposals; the policy admits them only for an agent the
	// registry grants /mandate (turn_mandate_admitted).
	Mandates []cardMandate
}

// cardMandate is one REQUIRE line: a Go test that must pass, and why.
type cardMandate struct {
	Test, Why string
}

// mandatePrefix opens a requirement line in a consultation's answer.
const mandatePrefix = "REQUIRE "

// parseMandates reads "REQUIRE <TestName>: <why>" lines out of an answer. The
// name must be a Go test function's (Test followed by identifier
// characters): it is what turn_test_case names, so anything else could never
// be discharged.
func parseMandates(answer string) []cardMandate {
	var out []cardMandate
	for _, line := range strings.Split(answer, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), mandatePrefix)
		if !ok {
			continue
		}
		name, why, _ := strings.Cut(rest, ":")
		name = strings.Trim(strings.TrimSpace(name), "`'\"")
		if !goTestName(name) {
			continue
		}
		out = append(out, cardMandate{Test: name, Why: strings.TrimSpace(why)})
	}
	return out
}

// goTestName reports whether name is a Go test function's name.
func goTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}
	for _, r := range rest {
		if !(r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)) {
			return false
		}
	}
	return true
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
	note := ""
	switch c.Route {
	case routeReuse:
		note = "; reused: asked before on the same request, files unchanged since"
	case routeEscalate:
		note = "; asked under the stronger consultation profile"
	}
	return fmt.Sprintf("\n### %s (%s; %s%s)\n%s\n", c.Agent, consultationRole(c.Mode), strings.Join(c.Technologies, ", "), note, c.Body)
}

// consultationHeader opens a consultation section.
const consultationHeader = "\n\n## Shard agent consultation\n" +
	"The harness consulted these shard agents before this turn because the task's files are in their domain (expert_spawn). Weigh their reading; you still own the result and its verification.\n"

// recordCards publishes the turn's cards to the kernel, which deals them. A
// card's identity in the facts is its position in the turn's cards.
func (e *Executor) recordCards(result *ExecutionResult, cards []expertCard) {
	if result == nil {
		return
	}
	first := len(result.expertCards)
	result.expertCards = append(result.expertCards, cards...)
	if e.kernel == nil {
		return
	}
	turn := result.turnAtom()
	for i, c := range cards {
		id := int64(first + i)
		e.assertTurnFact(types.Fact{Predicate: "turn_expert_card", Args: []any{
			turn, id, experts.AgentAtom(c.Agent), types.MangleAtom("/" + c.Mode), types.MangleAtom(c.Disposition)}})
		for _, f := range c.Files {
			e.assertTurnFact(types.Fact{Predicate: "turn_expert_card_file", Args: []any{turn, id, f}})
		}
		for j, h := range c.Handoffs {
			e.assertTurnFact(types.Fact{Predicate: "turn_card_handoff", Args: []any{turn, id, int64(j), h.From, h.To}})
		}
		for j, m := range c.Mandates {
			e.assertTurnFact(types.Fact{Predicate: "turn_card_mandate", Args: []any{turn, id, int64(j), m.Test}})
		}
	}
}

// admittedMandates is the section naming the requirements the turn must
// meet (turn_mandate_admitted), empty when there are none.
func (e *Executor) admittedMandates(result *ExecutionResult) string {
	if e.kernel == nil || result == nil || len(result.expertCards) == 0 {
		return ""
	}
	rows, err := e.turnRows("turn_mandate_admitted", result.turnAtom())
	if err != nil || len(rows) == 0 {
		return ""
	}
	var tests []string
	for _, row := range rows {
		if test, ok := stringArg(row, 1); ok && !slices.Contains(tests, test) {
			tests = append(tests, test)
		}
	}
	slices.Sort(tests)
	var b strings.Builder
	b.WriteString("\n\n## Requirements this turn must meet\n")
	b.WriteString("A shard agent the user granted mandate authority requires these tests to pass. The turn is not verified until each passes in its own test run; write or fix the test if it does not exist or fails.\n")
	for _, test := range tests {
		fmt.Fprintf(&b, "  - %s\n", test)
	}
	return b.String()
}

// cardAt is the turn's card with identity id.
func (r *ExecutionResult) cardAt(id int64) (expertCard, bool) {
	if id < 0 || int(id) >= len(r.expertCards) {
		return expertCard{}, false
	}
	return r.expertCards[id], true
}

// cardHandoffAt is hand-off idx of card id.
func (r *ExecutionResult) cardHandoffAt(id, idx int64) (expertCard, cardHandoff, bool) {
	c, ok := r.cardAt(id)
	if !ok || idx < 0 || int(idx) >= len(c.Handoffs) {
		return expertCard{}, cardHandoff{}, false
	}
	return c, c.Handoffs[idx], true
}

// numberArg reads argument i of a row as a number.
func numberArg(row types.Fact, i int) (int64, bool) {
	arg, ok := nthArg(row.Args, i)
	if !ok {
		return 0, false
	}
	n, ok := arg.(int64)
	return n, ok
}

// stringArg reads argument i of a row as a string.
func stringArg(row types.Fact, i int) (string, bool) {
	arg, ok := nthArg(row.Args, i)
	if !ok {
		return "", false
	}
	return types.ExtractString(arg), true
}

// handoffRow reads a turn_handoff_{step,unresolved} row: card, index and
// consumer file.
func handoffRow(row types.Fact) (int64, int64, string, bool) {
	card, okC := numberArg(row, 1)
	idx, okI := numberArg(row, 2)
	to, okT := stringArg(row, 3)
	return card, idx, to, okC && okI && okT
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
		id, idx, to, ok := handoffRow(row)
		c, h, found := result.cardHandoffAt(id, idx)
		if !ok || !found || added[to] {
			continue
		}
		added[to] = true
		step := workStep{File: to, Change: fmt.Sprintf("carry the hand-off from %s (%s): %s", h.From, c.Agent, h.Contract)}
		if len(run) < maxSteps {
			run = append(run, step)
		} else {
			step.Beyond = true
			beyond = append(beyond, step)
		}
		logging.Session("Plan gains a step on %s: hand-off from %s named by %s", to, h.From, c.Agent)
	}
	var note strings.Builder
	if rows, err := e.turnRows("turn_handoff_unresolved", turn); err == nil {
		for _, row := range rows {
			id, idx, to, ok := handoffRow(row)
			c, h, found := result.cardHandoffAt(id, idx)
			if !ok || !found {
				continue
			}
			fmt.Fprintf(&note, "\nUnresolved hand-off: %s (%s) says %s obliges %s, which is not on disk: %s", c.Agent, consultationRole(c.Mode), h.From, to, h.Contract)
		}
	}
	return append(run, beyond...), note.String()
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
	dealt := map[int64]bool{}
	for _, row := range rows {
		dealtFile, okFile := stringArg(row, 1)
		id, okID := numberArg(row, 2)
		if okFile && okID && dealtFile == canonical {
			dealt[id] = true
		}
	}
	var hand, left strings.Builder
	var delivered, withheld []string
	for i, c := range result.expertCards {
		if dealt[int64(i)] {
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
		type handoffID struct{ card, idx int64 }
		seen := map[handoffID]bool{}
		for _, row := range rows {
			stepFile, okF := stringArg(row, 1)
			id, okC := numberArg(row, 2)
			idx, okI := numberArg(row, 3)
			if !okF || !okC || !okI || stepFile != canonical || seen[handoffID{id, idx}] {
				continue
			}
			c, h, found := result.cardHandoffAt(id, idx)
			if !found {
				continue
			}
			seen[handoffID{id, idx}] = true
			fmt.Fprintf(&handoffs, "  - %s -> %s (%s): %s\n", h.From, h.To, experts.AgentKey(c.Agent), h.Contract)
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
	b.WriteString(e.admittedMandates(result))
	return b.String()
}

// repairCards is the consultation section one repair's prompt adds to the
// turn's own: the cards planned steps bought after that prompt was built
// which policy deals to this repair (turn_repair_card_dealt), and the turn's
// requirements when a step-born card brought one. subject is what the repair
// works on -- its failure output, or an advisory round's findings -- and its
// scope is the written files it names (repairScope). Empty when no step
// bought a card.
//
// Until 2026-10-30 the repairs after the steps ran on the turn's prompt
// alone, so advice a step bought for the file it edited was absent when a
// repair edited the same file; and the first cut dealt every repair the
// cards of the turn's whole write set, so a test repair on one package was
// handed advice about another.
func (e *Executor) repairCards(result *ExecutionResult, subject string) string {
	if e.kernel == nil || result == nil || result.promptCards >= len(result.expertCards) {
		return ""
	}
	turn := result.turnAtom()
	result.repairs++
	repair := int64(result.repairs)
	e.assertTurnFact(types.Fact{Predicate: "turn_repair", Args: []any{turn, repair}})
	scope := repairScope(e.workspaceForVerification(), result.WrittenPaths, subject)
	for _, f := range scope {
		e.assertTurnFact(types.Fact{Predicate: "turn_repair_file", Args: []any{turn, repair, f}})
	}
	mandated := false
	for id := result.promptCards; id < len(result.expertCards); id++ {
		e.assertTurnFact(types.Fact{Predicate: "turn_card_step_born", Args: []any{turn, int64(id)}})
		mandated = mandated || len(result.expertCards[id].Mandates) > 0
	}
	rows, err := e.turnRows("turn_repair_card_dealt", turn)
	if err != nil {
		logging.Get(logging.CategorySession).Warn("dealing the steps' consultations to repair %d failed (%v); it runs on the turn's prompt", repair, err)
		return ""
	}
	dealt := map[int64]bool{}
	for _, row := range rows {
		r, okR := numberArg(row, 1)
		id, okC := numberArg(row, 2)
		if okR && okC && r == repair {
			dealt[id] = true
		}
	}
	var hand, withheld strings.Builder
	var agents []string
	for id := result.promptCards; id < len(result.expertCards); id++ {
		c := result.expertCards[id]
		switch {
		case dealt[int64(id)]:
			hand.WriteString(c.render())
			agents = append(agents, c.Agent)
		case c.Disposition == cardAnswered && c.Mode == experts.SpawnAdvise:
			fmt.Fprintf(&withheld, "  - %s advises %s, which this repair's subject does not name\n", c.Agent, strings.Join(c.Files, ", "))
		}
	}
	section := ""
	if hand.Len() > 0 || withheld.Len() > 0 {
		logging.Session("Repair %d (scope %v) carries step consultations: %v", repair, scope, agents)
		section = "\n\n## Shard agent consultation bought by the planned steps\n" +
			"These shard agents were consulted for files the steps edited; a repair to those files weighs them as the steps did.\n" + hand.String()
		if withheld.Len() > 0 {
			section += "\nNot dealt to this repair:\n" + withheld.String()
		}
	}
	if mandated {
		section += e.admittedMandates(result)
	}
	return section
}

// repairScope is the written files a repair's subject names: by path, by
// file name followed by a position (a compiler's "b.go:12"), or by the
// directory that holds it (a test run's "FAIL module/internal/b", a test
// file beside it). A measurement of the text; which cards that scope
// admits is policy's.
func repairScope(workspace string, written []string, subject string) []string {
	var scope []string
	for _, p := range written {
		canonical := types.CanonicalPath(workspace, p)
		dir := path.Dir(canonical)
		named := strings.Contains(subject, canonical) ||
			strings.Contains(subject, path.Base(canonical)+":") ||
			(dir != "." && strings.Contains(subject, dir+"/")) ||
			(dir != "." && strings.Contains(subject, dir+"\n")) ||
			(dir != "." && strings.HasSuffix(strings.TrimSpace(subject), dir))
		if named && !slices.Contains(scope, canonical) {
			scope = append(scope, canonical)
		}
	}
	return scope
}
