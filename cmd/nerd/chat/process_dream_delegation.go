package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

// shouldAutoClarify heuristically decides when to trigger the clarifier shard without a command.
func (m Model) shouldAutoClarify(intent *perception.Intent, input string) bool {
	// Avoid loops on the same input
	if strings.TrimSpace(input) != "" && strings.EqualFold(strings.TrimSpace(input), strings.TrimSpace(m.lastClarifyInput)) {
		return false
	}

	lower := strings.ToLower(input)

	looksLikeCampaign := strings.Contains(lower, "campaign") ||
		strings.Contains(lower, "plan") ||
		strings.Contains(lower, "roadmap") ||
		strings.Contains(lower, "project") ||
		strings.Contains(lower, "initiative") ||
		strings.Contains(lower, "blueprint") ||
		strings.Contains(lower, "feature")

	needsDetails := intent != nil && (intent.Target == "" || intent.Constraint == "" || intent.Verb == "/generate" || intent.Verb == "/scaffold")

	isBuildish := intent != nil && (intent.Category == "/mutation" || intent.Category == "/instruction")

	return isBuildish && (looksLikeCampaign || needsDetails)
}

func (m Model) shouldClarifyFromKernel(intent *perception.Intent, input string) (string, []string, bool) {
	if m.kernel == nil || intent == nil {
		return "", nil, false
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return "", nil, false
	}
	if strings.EqualFold(trimmed, strings.TrimSpace(m.lastClarifyInput)) {
		return "", nil, false
	}

	intentID := "/current_intent"
	question := ""
	if facts, err := m.kernel.Query("clarification_question"); err == nil {
		for _, f := range facts {
			if len(f.Args) < 2 {
				continue
			}
			if types.ExtractString(f.Args[0]) != intentID {
				continue
			}
			if q, ok := f.Args[1].(string); ok {
				question = q
			} else {
				question = types.ExtractString(f.Args[1])
			}
			break
		}
	}
	if question == "" {
		if facts, err := m.kernel.Query("awaiting_clarification"); err == nil && len(facts) > 0 {
			if len(facts[0].Args) > 0 {
				if q, ok := facts[0].Args[0].(string); ok {
					question = q
				} else {
					question = types.ExtractString(facts[0].Args[0])
				}
			}
		}
	}
	if question == "" {
		return "", nil, false
	}

	options := make([]string, 0)
	seen := make(map[string]struct{})
	if facts, err := m.kernel.Query("clarification_option"); err == nil {
		for _, f := range facts {
			if len(f.Args) < 3 {
				continue
			}
			if types.ExtractString(f.Args[0]) != intentID {
				continue
			}
			verb := types.ExtractString(f.Args[1])
			label := types.ExtractString(f.Args[2])
			option := verb
			if label != "" && label != "<nil>" {
				option = fmt.Sprintf("%s (%s)", label, verb)
			}
			if _, ok := seen[option]; ok {
				continue
			}
			seen[option] = struct{}{}
			options = append(options, option)
		}
	}

	return question, options, true
}

func (m Model) shouldClarifyIntent(intent *perception.Intent, input string) bool {
	if intent == nil {
		return false
	}

	trimmed := strings.TrimSpace(input)
	if trimmed == "" || strings.HasPrefix(trimmed, "/") {
		return false
	}

	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "clarification:") {
		return false
	}

	if strings.EqualFold(trimmed, strings.TrimSpace(m.lastClarifyInput)) {
		return false
	}

	if isConversationalIntent(*intent) {
		return false
	}

	shardType := perception.GetShardTypeForVerb(intent.Verb)
	actionable := shardType != "" || intent.Verb == "/read" || intent.Verb == "/search" || intent.Verb == "/run" || intent.Verb == "/test" || intent.Verb == "/diff" || intent.Verb == "/git" || intent.Verb == "/build" || intent.Verb == "/fix" || intent.Verb == "/refactor" || intent.Verb == "/review" || intent.Verb == "/generate" || intent.Verb == "/create"

	if !actionable {
		return false
	}

	// NOTE: intent.Ambiguity is NOT checked here because the Understanding adapter
	// always populates it with debug metadata (semantic_type, action_type, domain).
	// The real ambiguity signal is the confidence score below.

	if intent.Confidence < 0.45 {
		return true
	}

	target := strings.TrimSpace(intent.Target)
	if target == "" || target == "none" {
		return true
	}

	return false
}

func (m Model) needsWorkspaceScanForDelegation(intent perception.Intent) bool {
	if intent.Category != "/query" && intent.Category != "/mutation" {
		return false
	}
	return perception.GetShardTypeForVerb(intent.Verb) != ""
}

func (m Model) loadWorkspaceFacts(ctx context.Context, intent perception.Intent, warnings *[]string) bool {
	if m.scanner == nil || m.kernel == nil {
		return false
	}
	if intent.Category != "/query" && intent.Category != "/mutation" {
		return false
	}

	res, err := m.scanner.ScanWorkspaceIncremental(ctx, m.workspace, m.localDB, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("Workspace scan skipped: %v", err))
		}
		return false
	}
	if res == nil || res.Unchanged || len(res.NewFacts) == 0 {
		return true
	}

	if applyErr := world.ApplyIncrementalResult(m.kernel, res); applyErr != nil {
		if warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("Workspace apply skipped: %v", applyErr))
		}
		return true
	}

	if m.virtualStore != nil {
		if err := m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 5); err != nil && warnings != nil {
			*warnings = append(*warnings, fmt.Sprintf("Knowledge persistence warning: %v", err))
		}
		for _, f := range res.NewFacts {
			switch f.Predicate {
			case "dependency_link":
				if len(f.Args) >= 2 {
					a := types.ExtractString(f.Args[0])
					b := types.ExtractString(f.Args[1])
					rel := "depends_on"
					if len(f.Args) >= 3 {
						rel = "depends_on:" + types.ExtractString(f.Args[2])
					}
					_ = m.virtualStore.PersistLink(a, rel, b, 1.0, map[string]any{"source": "scan"})
				}
			case "symbol_graph":
				if len(f.Args) >= 4 {
					sid := types.ExtractString(f.Args[0])
					file := types.ExtractString(f.Args[3])
					_ = m.virtualStore.PersistLink(sid, "defined_in", file, 1.0, map[string]any{"source": "scan"})
				}
			}
		}
	}

	return true
}

// systemExecutionResult holds the result of a system action execution.
type systemExecutionResult struct {
	ActionID   string
	ActionType string
	Target     string
	Success    bool
	Output     string
	Timestamp  int64
}

func (m Model) handleSystemDelegations(ctx context.Context, input string, intent perception.Intent, baseRouting, baseExec int) tea.Msg {
	if m.kernel == nil || m.shardMgr == nil {
		return nil
	}

	delegateFacts, _ := m.kernel.Query("delegate_task")
	execFacts := m.diffFacts("execution_result", baseExec)
	if len(execFacts) == 0 && shouldWaitForSystemResults(intent, len(delegateFacts) > 0) {
		_, execFacts = m.waitForSystemResults(ctx, baseRouting, baseExec, 1200*time.Millisecond)
	}

	executions := parseExecutionResults(execFacts)
	if msg := m.buildResponseFromExecutions(ctx, input, intent, delegateFacts, executions, baseRouting, baseExec); msg != nil {
		return msg
	}

	if len(delegateFacts) == 0 {
		return nil
	}

	return m.executeDelegateTaskFallback(ctx, input, intent, delegateFacts, baseRouting, baseExec)
}

func shouldWaitForSystemResults(intent perception.Intent, hasDelegations bool) bool {
	if hasDelegations {
		return true
	}
	if perception.GetShardTypeForVerb(intent.Verb) != "" {
		return true
	}
	switch intent.Verb {
	case "/read", "/search", "/run", "/test", "/diff", "/git", "/build":
		return true
	default:
		return false
	}
}

func (m Model) buildResponseFromExecutions(ctx context.Context, input string, intent perception.Intent, delegateFacts []core.Fact, executions []systemExecutionResult, baseRouting, baseExec int) tea.Msg {
	if len(executions) == 0 {
		return nil
	}

	sort.Slice(executions, func(i, j int) bool {
		return executions[i].Timestamp > executions[j].Timestamp
	})

	for _, exec := range executions {
		actionType := normalizeActionType(exec.ActionType)
		if actionType == "" {
			continue
		}

		if actionType == "run_tests" {
			surface := m.formatInterpretedResult(ctx, input, "tester", "run_tests", exec.Output, "")
			return assistantMsg{
				Surface: m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			}
		}

		shardType := actionTypeToShardType(actionType, exec.Target)
		if shardType == "" {
			task := strings.TrimSpace(strings.Join([]string{actionType, exec.Target}, " "))
			if task == "" {
				task = "system_action"
			}
			surface := m.formatInterpretedResult(ctx, input, "system", task, exec.Output, "")
			return assistantMsg{
				Surface: m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			}
		}

		task := resolveDelegateTask(shardType, delegateFacts, intent, m.workspace, m.lastShardResult)
		if task == "" {
			task = exec.Target
		}

		surface := m.formatDelegationOutput(ctx, input, shardType, task, exec.Output)
		payload := m.buildShardResultPayload(shardType, task, exec.Output, nil)
		if payload != nil && m.kernel != nil && len(payload.Facts) > 0 {
			_ = m.kernel.LoadFacts(payload.Facts)
		}
		return assistantMsg{
			Surface:     m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			ShardResult: payload,
		}
	}

	return nil
}

func (m Model) executeDelegateTaskFallback(ctx context.Context, input string, intent perception.Intent, delegateFacts []core.Fact, baseRouting, baseExec int) tea.Msg {
	for _, fact := range delegateFacts {
		shardType, taskDesc, pending := parseDelegateFact(fact)
		if !pending || shardType == "" {
			continue
		}

		task := resolveDelegateTask(shardType, delegateFacts, intent, m.workspace, m.lastShardResult)
		if task == "" {
			task = taskDesc
		}
		if task == "" {
			task = "codebase"
		}

		sessionCtx := m.buildSessionContext(ctx)
		result, spawnErr := m.spawnTaskWithContext(ctx, shardType, task, sessionCtx, types.PriorityHigh)
		payload := m.buildShardResultPayload(shardType, task, result, spawnErr)
		if payload != nil && m.kernel != nil && len(payload.Facts) > 0 {
			_ = m.kernel.LoadFacts(payload.Facts)
		}

		if spawnErr != nil {
			return errorMsg(fmt.Errorf("shard delegation failed: %w", spawnErr))
		}

		surface := m.formatDelegationOutput(ctx, input, shardType, task, result)
		return assistantMsg{
			Surface:     m.appendSystemSummary(surface, m.collectSystemSummary(ctx, baseRouting, baseExec)),
			ShardResult: payload,
		}
	}

	return nil
}

func (m Model) buildShardResultPayload(shardType, task, result string, err error) *ShardResultPayload {
	if m.shardMgr == nil {
		return nil
	}

	shardID := fmt.Sprintf("%s-system-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)
	return &ShardResultPayload{
		ShardType: shardType,
		Task:      task,
		Result:    result,
		Facts:     facts,
	}
}

func (m Model) formatDelegationOutput(ctx context.Context, input, shardType, task, result string) string {
	if shardType == "reviewer" || shardType == "tester" {
		return m.formatInterpretedResult(ctx, input, shardType, task, result, "")
	}

	header := fmt.Sprintf("## %s Result", strings.Title(shardType))
	if shardType == "" {
		header = "## Delegated Result"
	}
	return fmt.Sprintf(`%s
**Agent**: %s
**Task**: %s

### Output
%s`, header, shardType, task, result)
}

func (m Model) buildShardInterpretationPrompt(ctx context.Context, input, shardType, task, result string) (string, string) {
	userPrompt := fmt.Sprintf(`USER REQUEST (ANSWER THIS):
%s

You are translating shard output into a clear, user-facing answer.
Requirements:
- Start with a direct answer in 1-3 sentences.
- If the request asks for the biggest/main issue, identify the single highest-impact issue (or say none found).
- Summarize key evidence from the output without dumping raw logs.
- Provide 3-7 concrete next steps or checks.
- Call out uncertainty if the output is incomplete.

SHARD TYPE: %s
TASK: %s
OUTPUT:
%s
`, input, shardType, task, result)

	if m.jitCompiler != nil {
		semanticQuery := fmt.Sprintf("Translate %s shard output into actionable summary", normalizeShardType(shardType))
		// Derive the JIT budget from the user's actual context-window config
		// instead of hardcoding 12000/2000. Earlier the hardcode produced
		// AvailableTokens=10000 — too small to seat all mandatory atoms on
		// a model like Gemini 3.5-flash (1M context, 65K output reserve),
		// so mandatory atoms in late categories were being dropped at the
		// "exceeds total budget" check (.nerd/logs/2026-05-28_context.log:138-186).
		tokenBudget, reservedTokens := translatorJITBudget(m.Config)
		cc := prompt.NewCompilationContext().
			WithOperationalMode("/active").
			WithIntent("/translate", "").
			WithShard("/analysis_translator", "analysis_translator", "Analysis Translator").
			WithTokenBudget(tokenBudget, reservedTokens).
			WithSemanticQuery(semanticQuery, 8)

		if res, err := m.jitCompiler.Compile(ctx, cc); err == nil && res != nil && strings.TrimSpace(res.Prompt) != "" {
			return res.Prompt, userPrompt
		}
	}

	fallbackPrompt := fmt.Sprintf(`%s

%s`, campaign.AnalysisLogic, userPrompt)
	return stevenMoorePersona, fallbackPrompt
}

// translatorJITBudget derives (TokenBudget, ReservedTokens) for the
// analysis-translator JIT compilation from the user's loaded config so
// the prompt can seat its full mandatory-atom skeleton on large-context
// models like Gemini 3.5-flash. Falls back to a 60k/8k pair when no
// config is loaded — same shape as the original 12k/2k hardcode but
// large enough that all ~30 mandatory atoms fit.
func translatorJITBudget(cfg *config.UserConfig) (int, int) {
	const (
		fallbackBudget   = 60000
		fallbackReserved = 8000
		maxBudget        = 200000 // cap so JIT doesn't try to fit 1M tokens
	)
	if cfg == nil {
		return fallbackBudget, fallbackReserved
	}
	ctxCfg := cfg.GetContextWindowConfig()
	if ctxCfg.MaxTokens <= 0 {
		return fallbackBudget, fallbackReserved
	}
	// Use up to one-eighth of the input window for the translator prompt
	// (translation summaries are short; we don't need the full window).
	budget := ctxCfg.MaxTokens / 8
	if budget > maxBudget {
		budget = maxBudget
	}
	if budget < fallbackBudget {
		budget = fallbackBudget
	}
	reserved := ctxCfg.OutputReserve / 8
	if reserved < fallbackReserved {
		reserved = fallbackReserved
	}
	if reserved >= budget {
		reserved = budget / 10
	}
	return budget, reserved
}

func (m Model) interpretShardOutput(ctx context.Context, input, shardType, task, result string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("LLM client not initialized")
	}

	systemPrompt, userPrompt := m.buildShardInterpretationPrompt(ctx, input, shardType, task, result)
	interpResp, err := m.client.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// Extract grounding sources if client supports it (e.g., Gemini with Google Search)
	var groundingSources []string
	if gp, ok := m.client.(types.GroundingProvider); ok {
		groundingSources = gp.GetLastGroundingSources()
	}

	var interpretation string
	processor := articulation.NewResponseProcessor()
	if processed, procErr := processor.Process(interpResp); procErr == nil && strings.TrimSpace(processed.Surface) != "" {
		interpretation = processed.Surface
	} else {
		trimmed := strings.TrimSpace(interpResp)
		if trimmed == "" {
			return "", fmt.Errorf("empty interpretation response")
		}
		interpretation = trimmed
	}

	// Append grounding sources for transparency
	if len(groundingSources) > 0 {
		interpretation += "\n\n**Sources:**\n"
		for _, src := range groundingSources {
			interpretation += fmt.Sprintf("- %s\n", src)
		}
	}

	return interpretation, nil
}

func (m Model) formatInterpretedResult(ctx context.Context, input, shardType, task, result, warning string) string {
	interpretation, err := m.interpretShardOutput(ctx, input, shardType, task, result)
	if err != nil {
		interpretation = fmt.Sprintf("Unable to interpret shard output automatically (%v). Raw output below.", err)
	}

	warning = strings.TrimSpace(warning)
	if warning != "" {
		interpretation = fmt.Sprintf("%s\n\n%s", interpretation, warning)
	}

	return fmt.Sprintf("%s\n\n<details><summary>Raw Output</summary>\n\n%s\n\n</details>", interpretation, result)
}

func (m Model) collectTraceShardTypes() []string {
	candidates := []string{"coder", "tester", "reviewer", "researcher", "planner", "security"}
	if m.lastShardResult != nil && m.lastShardResult.ShardType != "" {
		candidates = append(candidates, m.lastShardResult.ShardType)
	}
	for _, sr := range m.shardResultHistory {
		if sr != nil && sr.ShardType != "" {
			candidates = append(candidates, sr.ShardType)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, shard := range candidates {
		normalized := normalizeShardType(shard)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		unique = append(unique, normalized)
	}

	return unique
}

func resolveDelegateTask(shardType string, delegateFacts []core.Fact, intent perception.Intent, workspace string, priorResult *ShardResult) string {
	task := ""
	for _, fact := range delegateFacts {
		parsedShard, taskDesc, pending := parseDelegateFact(fact)
		if !pending || parsedShard != shardType {
			continue
		}
		task = taskDesc
		break
	}

	task = strings.TrimSpace(task)
	if task == "" {
		task = strings.TrimSpace(intent.Target)
	}

	if task == "" {
		return ""
	}

	if strings.Contains(task, ":") || strings.Contains(task, " ") {
		return task
	}

	verb := defaultVerbForShard(shardType)
	if verb == "" {
		return task
	}

	return formatShardTaskWithContext(verb, task, intent.Constraint, workspace, priorResult)
}

func defaultVerbForShard(shardType string) string {
	switch shardType {
	case "reviewer":
		return "/review"
	case "tester":
		return "/test"
	case "researcher":
		return "/research"
	case "coder":
		return "/fix"
	default:
		return ""
	}
}
