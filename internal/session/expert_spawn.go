package session

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/experts"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
)

// ExpertConsultant runs a shard-agent consultation to completion and returns
// the agent's answer. JITExecutor.SpawnConsultation implements it: the
// consultation runs as "/consult/<agent>" in an isolated subagent, the
// machinery chat delegation's consultations use.
type ExpertConsultant interface {
	SpawnConsultation(ctx context.Context, specialistName, task string) (string, error)
}

// SetExpertConsultant installs the consultation spawner the turn's
// expert_spawn decisions run through. Boot wires the JITExecutor
// (internal/system/factory.go); clones and spawned subagents inherit it.
func (e *Executor) SetExpertConsultant(c ExpertConsultant) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.expertConsultant = c
}

// HasExpertConsultant reports whether a consultation spawner is wired, so
// boot can be checked for the spawn-second path the way HasTurnRecorder
// checks learning.
func (e *Executor) HasExpertConsultant() bool {
	return e.consultant() != nil
}

func (e *Executor) consultant() ExpertConsultant {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.expertConsultant
}

// expertConsultation is one consultation's outcome.
type expertConsultation struct {
	need   experts.Need
	answer string
	err    error
}

// withExpertConsultations runs the consultations the kernel derived for this
// turn (expert_spawn, policy/shard_experts.mg) and appends their answers to
// the system prompt. JIT first, spawn second: the experts' knowledge is
// already in the compile; a consultation is the kernel's call that the task
// needs the expert's own reading of it -- an executor expert laying out the
// work (/lead) or an advisor saying what must hold (/advise). The turn keeps
// its verb's persona, tools, safety atoms and gates: handing a mutation to
// the agent's own verb would run it on the read-only /general catalog
// without the coder's shard-gated atoms.
//
// A consultation that fails, or a turn with no consultant wired, is named in
// the section rather than dropped, so the model knows what it is missing.
//
// Each outcome is also kept as a typed card (expert_cards.go): a planned turn
// runs each step on a freshly compiled prompt, and the kernel deals each step
// the cards that apply to its file. An agent already consulted this turn is
// not consulted again.
func (e *Executor) withExpertConsultations(ctx context.Context, systemPrompt string, intent perception.Intent, input string, sel experts.Selection, result *ExecutionResult) string {
	var spawns []experts.Need
	for _, need := range sel.Needs {
		if need.Spawn != "" && (result == nil || !result.consultedExperts[need.Agent]) {
			spawns = append(spawns, need)
		}
	}
	if len(spawns) == 0 {
		return systemPrompt
	}
	consultant := e.consultant()
	results := make([]expertConsultation, len(spawns))
	var wg sync.WaitGroup
	for i, need := range spawns {
		results[i].need = need
		if consultant == nil {
			results[i].err = fmt.Errorf("no consultation spawner is wired to this executor")
			continue
		}
		wg.Go(func() {
			answer, err := consultant.SpawnConsultation(ctx, need.Agent, consultationTask(need, intent, input))
			results[i].answer, results[i].err = answer, err
		})
	}
	wg.Wait()

	cards := make([]expertCard, len(results))
	var b strings.Builder
	b.WriteString(consultationHeader)
	workspace := e.workspaceForVerification()
	for i, r := range results {
		cards[i] = newExpertCard(r)
		if cards[i].Disposition == cardAnswered {
			cards[i].Handoffs = parseHandoffs(workspace, cards[i].Body)
		}
		if r.err != nil {
			logging.Get(logging.CategorySession).Warn("shard agent %s consultation failed: %v", r.need.Agent, r.err)
		}
		b.WriteString(cards[i].render())
	}
	logging.Session("Consulted %d shard agent(s) for %s %s", len(results), intent.Verb, intent.Target)
	if result != nil {
		if result.consultedExperts == nil {
			result.consultedExperts = map[string]bool{}
		}
		for _, need := range spawns {
			result.consultedExperts[need.Agent] = true
		}
	}
	e.recordCards(result, cards)
	return strings.TrimRight(systemPrompt, "\n") + b.String()
}

func consultationRole(mode string) string {
	if mode == experts.SpawnLead {
		return "leads the work"
	}
	return "advises"
}

// consultationTask is what the consulted agent is asked.
func consultationTask(need experts.Need, intent perception.Intent, input string) string {
	var ask string
	if need.Spawn == experts.SpawnLead {
		ask = "Lay out how to carry out this task in your domain: the steps, the exact files and symbols, the idioms to follow and the mistakes to avoid. Do not edit files; the executing agent applies your plan."
	} else {
		ask = "Advise on this task from your domain: what must hold for the change to be right, and what to avoid. Do not edit files."
	}
	var b strings.Builder
	b.WriteString(ask)
	fmt.Fprintf(&b, "\n\nTask (%s", intent.Verb)
	if t := strings.TrimSpace(intent.Target); t != "" {
		fmt.Fprintf(&b, " on %s", t)
	}
	b.WriteString("):\n")
	b.WriteString(strings.TrimSpace(input))
	if len(need.Files) > 0 {
		fmt.Fprintf(&b, "\n\nFiles in your domain: %s", strings.Join(need.Files, ", "))
	}
	b.WriteString("\n\nWhen a change in one file obliges a change or a condition in another, add one line per hand-off, exactly:\n" +
		handoffPrefix + "<from-file> -> <to-file>: <what must hold between them>")
	return b.String()
}
