package session

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/experts"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The /expert_critique round: the shard agents a /fix or /refactor spawned
// review the change the turn made, before its verdict.
//
// It has the build critic's shape (verifyAndUpliftWithCritic), because it is
// the same kind of evidence: a model's reading of another model's change.
// The policy owes the round (turn_round_owed, policy/turn_rounds.mg, from
// turn_expert_critic), each agent reviews through the consultation path the
// lead consultation used (ExpertConsultant), its on-change findings are
// asserted (turn_expert_finding) and triaged by policy
// (turn_expert_actionable, the critic's severity bar), and the actionable ones
// go back to the model as one repair round. Like the critic it never fails a
// turn: a repair that breaks the build or the tests is undone
// (recheckUplift), and the rounds after it measure what it wrote.

// ExpertFinding is one finding of a shard agent's review of the turn.
type ExpertFinding struct {
	Agent string
	CriticFinding
}

// recordExpertCritics records, for the turn, every shard agent its selection
// owes a critique (expert_critique): turn_expert_critic, which the round's
// owing reads. A turn that writes nothing owes no round whatever is recorded.
func (e *Executor) recordExpertCritics(result *ExecutionResult, sel experts.Selection) {
	if e.kernel == nil || result == nil {
		return
	}
	turn := result.turnAtom()
	for _, need := range sel.Needs {
		if need.Critique {
			e.assertTurnFact(types.Fact{Predicate: "turn_expert_critic", Args: []any{turn, experts.AgentAtom(need.Agent)}})
			// The agent's domain files ride with the obligation: its review
			// is shown them first (readWrittenFilesForReview).
			if result.expertDomainFiles == nil {
				result.expertDomainFiles = map[string][]string{}
			}
			key := experts.AgentKey(need.Agent)
			for _, f := range need.Files {
				if !slices.Contains(result.expertDomainFiles[key], f) {
					result.expertDomainFiles[key] = append(result.expertDomainFiles[key], f)
				}
			}
		}
	}
}

// expertCritiqueTask is what a reviewing shard agent is asked: the change,
// the files of its domain it covers, what it was not shown, and the build
// critic's reply contract, which parseCriticFindings reads.
func expertCritiqueTask(agent string, views, removals map[string]string, request string, domain []string, omitted []reviewOmission) string {
	var b strings.Builder
	fmt.Fprintf(&b, "As the %s shard agent, review this change for defects in your domain: a misuse of the technology, a broken contract, a wrong idiom with a real consequence. Do not edit files.\n\n", agent)
	if len(domain) > 0 {
		fmt.Fprintf(&b, "Your domain covers these files of the task: %s.\n", strings.Join(domain, ", "))
	}
	if section := renderReviewOmissions(omitted, true); section != "" {
		b.WriteString(section)
	}
	if len(domain) > 0 || len(omitted) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("Reply with one line per real defect, exactly:\nFINDING <file>:<line> <high|medium|low>: <what is wrong and why>\n")
	b.WriteString("Cite a line inside the change. If the change is sound, reply with exactly: NO FINDINGS\n\n")
	b.WriteString(buildCriticPrompt(views, removals, "", request))
	return b.String()
}

// expertRepairPrompt hands the actionable findings back to the model.
func expertRepairPrompt(findings []ExpertFinding) string {
	var b strings.Builder
	b.WriteString("The shard agents whose domain this change is in reviewed the code you just wrote and reported:\n\n")
	for _, f := range findings {
		fmt.Fprintf(&b, "  - [%s] %s:%d [%s] %s\n", f.Agent, f.File, f.Line, f.Severity, f.Claim)
	}
	b.WriteString("\nFor each item: either fix it with the edit tools, or state plainly that the finding is wrong and why. " +
		"Do not make a change you believe is incorrect just to clear an item -- the reviewer is fallible too.\n\n" +
		"Do not restate the findings and do not report success. The build and tests are checked again after you finish, " +
		"and edits that break them are undone.")
	return b.String()
}

// reviewWithShardExperts runs the /expert_critique round.
func (e *Executor) reviewWithShardExperts(
	ctx context.Context,
	trp types.ToolResultsProvider,
	systemPrompt string,
	history []types.Message,
	toolDefs []types.ToolDefinition,
	cfg *jitconfig.EffectiveAgentRuntimeConfig,
	result *ExecutionResult,
) ([]string, error) {
	if result == nil || e.kernel == nil {
		return nil, nil
	}
	turn := result.turnAtom()
	rows, err := e.turnRows("turn_expert_critic", turn)
	if err != nil {
		return nil, fmt.Errorf("query turn_expert_critic: %w", err)
	}
	var agents []string
	for _, row := range rows {
		if len(row.Args) > 1 {
			agents = append(agents, experts.AgentKey(types.ExtractString(row.Args[1])))
		}
	}
	consultant := e.consultant()
	if len(agents) == 0 {
		return nil, nil
	}
	if consultant == nil {
		logging.Get(logging.CategorySession).Warn("the turn owes %v a critique, but no consultation spawner is wired", agents)
		return nil, nil
	}
	workspace := e.workspaceForVerification()
	request := ""
	if loop := activeWorkingLoop(ctx); loop != nil {
		request = loop.task
	}

	// Each agent's packet puts its own domain files first, so the review's
	// file bound never drops the subject it was selected for, and names
	// every written file it was not shown.
	var onChange []ExpertFinding
	for _, agent := range agents {
		domain := result.expertDomainFiles[agent]
		files, omitted := readWrittenFilesForReview(workspace, result.WrittenPaths, domain)
		views, removals, windows := turnChangeViews(workspace, files, result)
		if len(views) == 0 {
			continue
		}
		if len(omitted) > 0 {
			logging.Get(logging.CategorySession).Info("shard agent %s review: %d written file(s) not shown, named to it", agent, len(omitted))
		}
		answer, err := consultant.SpawnConsultation(ctx, agent, expertCritiqueTask(agent, views, removals, request, domain, omitted))
		if err != nil {
			// Advisory, like the build critic: a review that did not
			// complete is a missing opinion, not a failed turn.
			logging.Get(logging.CategorySession).Warn("shard agent %s critique failed (%v); turn continues", agent, err)
			continue
		}
		kept, dropped := findingsOnChange(parseCriticFindings(answer), windows)
		if len(dropped) > 0 {
			logging.Get(logging.CategorySession).Info("shard agent %s: %d finding(s) cite code this turn did not change; not charged to it", agent, len(dropped))
		}
		for _, f := range kept {
			onChange = append(onChange, ExpertFinding{Agent: agent, CriticFinding: f})
		}
	}
	result.ExpertFindings = onChange
	if len(onChange) == 0 {
		return nil, nil
	}
	for i, f := range onChange {
		e.assertTurnFact(types.Fact{Predicate: "turn_expert_finding", Args: []any{
			turn, experts.AgentAtom(f.Agent), int64(i), criticSeverityAtom(f.Severity)}})
	}
	actionableRows, err := e.turnRows("turn_expert_actionable", turn)
	if err != nil {
		return nil, fmt.Errorf("query turn_expert_actionable: %w", err)
	}
	var actionable []ExpertFinding
	for _, row := range actionableRows {
		if len(row.Args) <= 1 {
			continue
		}
		if idx, ok := row.Args[1].(int64); ok && idx >= 0 && int(idx) < len(onChange) {
			actionable = append(actionable, onChange[idx])
		}
	}
	logging.Get(logging.CategorySession).Warn("Shard agent review reported %d finding(s), %d worth acting on", len(onChange), len(actionable))
	if len(actionable) == 0 {
		return nil, nil
	}
	if trp == nil {
		logging.Get(logging.CategorySession).Warn("shard agent review found %d actionable item(s), but this provider has no repair channel", len(actionable))
		return nil, nil
	}
	return e.adviseEpisode(broker.WithPhase(ctx, broker.PhaseExpertRepair), trp, systemPrompt, history, toolDefs, cfg, result,
		expertRepairPrompt(actionable), "shard agent repair")
}
