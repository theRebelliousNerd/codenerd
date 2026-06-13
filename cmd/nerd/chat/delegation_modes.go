package chat

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/shards"

	tea "github.com/charmbracelet/bubbletea"
)

// executeParallelMode runs all shards in parallel (for /review, /security, /test)
func (m Model) executeParallelMode(ctx context.Context, verb, shardType, task, target string, specialists []shards.SpecialistMatch, startTime time.Time) tea.Msg {
	m.ReportStatus(fmt.Sprintf("Spawning %s + %d specialists in parallel...", shardType, len(specialists)))

	resultsChan := make(chan spawnResult, len(specialists)+1)
	var wg sync.WaitGroup

	// Spawn generic shard
	if shards.ShouldIncludeGenericShard(verb) {
		wg.Go(func() {
			shardStart := time.Now()
			result, err := m.spawnTask(ctx, shardType, task)
			duration := time.Since(shardStart)
			resultsChan <- spawnResult{Name: shardType, Result: result, Err: err, Task: task, Duration: duration}
		})
	}

	// Spawn specialists in parallel
	for _, spec := range specialists {
		s := spec
		wg.Go(func() {
			shardStart := time.Now()
			specTask := fmt.Sprintf("%s files:%s context:[matched for %s]",
				strings.TrimPrefix(verb, "/"), strings.Join(s.Files, ","), s.Reason)
			result, err := m.spawnTask(ctx, s.AgentName, specTask)
			duration := time.Since(shardStart)
			resultsChan <- spawnResult{Name: s.AgentName, Result: result, Err: err, Task: specTask, Duration: duration}
		})
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []spawnResult
	for r := range resultsChan {
		results = append(results, r)
	}

	// Record each execution for prompt evolution learning
	for _, r := range results {
		m.recordShardExecution(r.Name, r.Task, r.Result, r.Err, r.Duration)
	}

	return m.formatParallelResults(verb, shardType, task, target, results, startTime)
}

// executeAdvisoryMode: Phase 1 (advice) → Phase 2 (execute with advice)
func (m Model) executeAdvisoryMode(ctx context.Context, verb, shardType, task, target string, files []string, specialists []shards.SpecialistMatch, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s with Specialist Advisory\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n\n", target))

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 1: Gather specialist advice in parallel
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 1: Gathering advice from %d specialists...", len(specialists)))
	sb.WriteString("### Phase 1: Specialist Advisory\n\n")

	adviceResults := m.gatherSpecialistAdvice(ctx, verb, files, specialists)
	var combinedAdvice strings.Builder
	for _, adv := range adviceResults {
		if adv.Err != nil {
			sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to provide advice\n\n", adv.Name))
		} else {
			sb.WriteString(fmt.Sprintf("**%s** (%s):\n%s\n\n", adv.Name, adv.Reason, adv.Result))
			combinedAdvice.WriteString(fmt.Sprintf("[%s advice]: %s\n", adv.Name, adv.Result))
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 2: Execute with specialist advice as context
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 2: Executing %s with specialist context...", shardType))
	sb.WriteString("---\n\n### Phase 2: Execution\n\n")

	// Inject advice into the task
	enhancedTask := task
	if combinedAdvice.Len() > 0 {
		enhancedTask = fmt.Sprintf("%s\n\n[SPECIALIST ADVICE - Consider these domain-specific recommendations]:\n%s",
			task, combinedAdvice.String())
	}

	execStart := time.Now()
	result, err := m.spawnTask(ctx, shardType, enhancedTask)
	execDuration := time.Since(execStart)
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)

	if err != nil {
		sb.WriteString(fmt.Sprintf("**%s**: ❌ Failed: %v\n\n", shardType, err))
	} else {
		sb.WriteString(fmt.Sprintf("**%s** output:\n\n%s\n\n", shardType, result))
	}

	// Inject facts
	if m.kernel != nil && len(facts) > 0 {
		if err := m.kernel.LoadFacts(facts); err != nil {
			logging.Routing("[delegation] failed to load advisory facts: %v", err)
		}
	}

	// Record execution for prompt evolution learning
	m.recordShardExecution(shardType, enhancedTask, result, err, execDuration)

	sb.WriteString(fmt.Sprintf("---\n\n**Duration**: %s\n", time.Since(startTime).Round(time.Second)))
	sb.WriteString(fmt.Sprintf("**Advisors**: %s\n", formatAdvisorNames(adviceResults)))

	m.ReportStatus(fmt.Sprintf("%s complete", shardType))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    result,
			Facts:     facts,
		},
	}
}

// executeAdvisoryWithCritiqueMode: Phase 1 (advice) → Phase 2 (execute) → Phase 3 (critique)
func (m Model) executeAdvisoryWithCritiqueMode(ctx context.Context, verb, shardType, task, target string, files []string, specialists []shards.SpecialistMatch, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s with Specialist Advisory & Critique\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n\n", target))

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 1: Gather specialist advice in parallel
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 1: Gathering advice from %d specialists...", len(specialists)))
	sb.WriteString("### Phase 1: Specialist Advisory\n\n")

	adviceResults := m.gatherSpecialistAdvice(ctx, verb, files, specialists)
	var combinedAdvice strings.Builder
	for _, adv := range adviceResults {
		if adv.Err != nil {
			sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to provide advice\n\n", adv.Name))
		} else {
			sb.WriteString(fmt.Sprintf("**%s** (%s):\n%s\n\n", adv.Name, adv.Reason, adv.Result))
			combinedAdvice.WriteString(fmt.Sprintf("[%s advice]: %s\n", adv.Name, adv.Result))
		}
	}

	// ═══════════════════════════════════════════════════════════════════════════
	// PHASE 2: Execute with specialist advice as context
	// ═══════════════════════════════════════════════════════════════════════════
	m.ReportStatus(fmt.Sprintf("Phase 2: Executing %s with specialist context...", shardType))
	sb.WriteString("---\n\n### Phase 2: Execution\n\n")

	enhancedTask := task
	if combinedAdvice.Len() > 0 {
		enhancedTask = fmt.Sprintf("%s\n\n[SPECIALIST ADVICE - Consider these domain-specific recommendations]:\n%s",
			task, combinedAdvice.String())
	}

	execStart := time.Now()
	result, err := m.spawnTask(ctx, shardType, enhancedTask)
	execDuration := time.Since(execStart)
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)

	if err != nil {
		sb.WriteString(fmt.Sprintf("**%s**: ❌ Failed: %v\n\n", shardType, err))
		// Skip critique if execution failed
		sb.WriteString("---\n\n### Phase 3: Critique\n\n⚠️ Skipped due to execution failure.\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("**%s** output:\n\n%s\n\n", shardType, result))

		// ═══════════════════════════════════════════════════════════════════════════
		// PHASE 3: Specialist critique of the result
		// ═══════════════════════════════════════════════════════════════════════════
		m.ReportStatus("Phase 3: Gathering specialist critiques...")
		sb.WriteString("---\n\n### Phase 3: Specialist Critique\n\n")

		critiqueResults := m.gatherSpecialistCritique(ctx, verb, files, specialists, result)
		for _, crit := range critiqueResults {
			if crit.Err != nil {
				sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to critique\n\n", crit.Name))
			} else {
				sb.WriteString(fmt.Sprintf("**%s**:\n%s\n\n", crit.Name, crit.Result))
			}
		}
	}

	// Inject facts
	if m.kernel != nil && len(facts) > 0 {
		if err := m.kernel.LoadFacts(facts); err != nil {
			logging.Routing("[delegation] failed to load critique facts: %v", err)
		}
	}

	// Record execution for prompt evolution learning
	m.recordShardExecution(shardType, enhancedTask, result, err, execDuration)

	sb.WriteString(fmt.Sprintf("---\n\n**Duration**: %s\n", time.Since(startTime).Round(time.Second)))
	sb.WriteString(fmt.Sprintf("**Advisors**: %s\n", formatAdvisorNames(adviceResults)))

	m.ReportStatus(fmt.Sprintf("%s complete", shardType))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    result,
			Facts:     facts,
		},
	}
}

// executeSpecialistDirectMode handles high-confidence executor specialists directly.
// When a specialist has ShouldExecute=true and is an Executor, they handle the task
// without going through the generic shard. This implements specialist_should_execute.
func (m Model) executeSpecialistDirectMode(ctx context.Context, verb string, specialist shards.SpecialistMatch, task, target string, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s via Specialist Executor\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n", target))
	sb.WriteString(fmt.Sprintf("**Specialist**: %s (confidence: %.0f%%)\n", specialist.AgentName, specialist.Score*100))
	sb.WriteString("**Mode**: Direct Execution\n\n")

	if specialist.Classification != nil {
		sb.WriteString(fmt.Sprintf("**Classification**: %s / %s\n\n",
			specialist.Classification.ExecutionMode,
			specialist.Classification.KnowledgeTier))
	}

	m.ReportStatus(fmt.Sprintf("Specialist %s executing %s directly...", specialist.AgentName, verb))

	// Build specialist-specific task with full context
	specialistTask := fmt.Sprintf(`%s

Target files: %s
Your expertise: %s

You are executing this task DIRECTLY as the domain specialist. Apply your full expertise.
Do NOT just advise - implement the solution.`,
		task,
		strings.Join(specialist.Files, ", "),
		specialist.Reason)

	// Strategic Advisory Delegation check (consultation.go integration)
	taskComplexity := "normal"
	taskLower := strings.ToLower(task)
	if strings.Contains(taskLower, "complex") || strings.Contains(taskLower, "security") ||
		strings.Contains(taskLower, "architecture") || strings.Contains(taskLower, "critical") ||
		strings.Contains(taskLower, "refactor") || len(specialist.Files) > 3 {
		taskComplexity = "high"
	}

	if shards.ShouldConsultBeforeExecution(specialist.AgentName, taskComplexity) {
		advisors := shards.GetStrategicAdvisorsFor(specialist.AgentName)
		if len(advisors) > 0 {
			m.ReportStatus("Consulting strategic advisors for complex task...")
			sb.WriteString("### Phase 1: Strategic Consultation\n\n")
			sb.WriteString(fmt.Sprintf("Before executing, executor **%s** consulted strategic advisors for guidance on this complex task:\n\n", specialist.AgentName))

			advisorMatches := make([]shards.SpecialistMatch, 0, len(advisors))
			for _, adv := range advisors {
				class, _ := shards.GetSpecialistClassification(adv)
				displayName := adv
				if strings.ToLower(adv) == "securityauditor" {
					displayName = "SecurityAuditor"
				} else if strings.ToLower(adv) == "testarchitect" {
					displayName = "TestArchitect"
				}
				advisorMatches = append(advisorMatches, shards.SpecialistMatch{
					AgentName:      displayName,
					Files:          specialist.Files,
					Score:          1.0,
					Reason:         fmt.Sprintf("Strategic advisor classification for %s", specialist.AgentName),
					Classification: &class,
					ShouldExecute:  false,
				})
			}

			adviceResults := m.gatherSpecialistAdvice(ctx, verb, specialist.Files, advisorMatches)
			var combinedAdvice strings.Builder
			for _, adv := range adviceResults {
				if adv.Err != nil {
					sb.WriteString(fmt.Sprintf("**%s**: ⚠️ Failed to provide strategic advice\n\n", adv.Name))
				} else {
					sb.WriteString(fmt.Sprintf("**%s** strategic advice:\n%s\n\n", adv.Name, adv.Result))
					combinedAdvice.WriteString(fmt.Sprintf("[%s strategic advice]: %s\n", adv.Name, adv.Result))
				}
			}

			// Inject strategic advice into the task
			if combinedAdvice.Len() > 0 {
				specialistTask = fmt.Sprintf("%s\n\n[STRATEGIC ADVICE - Apply these high-level architectural recommendations from your advisors]:\n%s",
					specialistTask, combinedAdvice.String())
			}

			sb.WriteString("---\n\n### Phase 2: Execution\n\n")
		}
	}

	execStart := time.Now()
	result, err := m.spawnTask(ctx, specialist.AgentName, specialistTask)
	execDuration := time.Since(execStart)
	shardID := fmt.Sprintf("%s-%d", specialist.AgentName, time.Now().UnixNano())
	facts := m.shardMgr.ResultToFacts(shardID, specialist.AgentName, task, result, err)

	if err != nil {
		sb.WriteString(fmt.Sprintf("### Execution Failed\n\n❌ Error: %v\n\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("### Result\n\n%s\n\n", result))
	}

	// Inject facts
	if m.kernel != nil && len(facts) > 0 {
		if err := m.kernel.LoadFacts(facts); err != nil {
			logging.Routing("[delegation] failed to load specialist facts: %v", err)
		}
	}

	// Record execution for prompt evolution learning
	m.recordShardExecution(specialist.AgentName, specialistTask, result, err, execDuration)

	sb.WriteString(fmt.Sprintf("---\n\n**Duration**: %s\n", time.Since(startTime).Round(time.Second)))
	sb.WriteString(fmt.Sprintf("**Executor**: %s\n", specialist.AgentName))

	m.ReportStatus(fmt.Sprintf("%s complete (specialist direct)", specialist.AgentName))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: specialist.AgentName,
			Task:      task,
			Result:    result,
			Facts:     facts,
		},
	}
}

// adviceResult holds the result from a specialist advice/critique query
type adviceResult struct {
	Name   string
	Reason string
	Result string
	Err    error
}

// gatherSpecialistAdvice queries specialists for advice in parallel
func (m Model) gatherSpecialistAdvice(ctx context.Context, verb string, files []string, specialists []shards.SpecialistMatch) []adviceResult {
	resultsChan := make(chan adviceResult, len(specialists))
	var wg sync.WaitGroup

	logging.Shards("Gathering advice for %s on %d files", verb, len(files))

	for _, spec := range specialists {
		s := spec
		wg.Go(func() {
			// Advisory task prompt
			adviceTask := fmt.Sprintf(`ADVISORY REQUEST: Provide domain-specific advice for a %s operation.

Target files: %s
Your expertise: %s

Please provide:
1. Key considerations specific to your domain
2. Common pitfalls to avoid
3. Best practices to follow
4. Any specific patterns or approaches to use

Keep your advice concise and actionable. Do NOT make changes yourself - just advise.`,
				strings.TrimPrefix(verb, "/"),
				strings.Join(s.Files, ", "),
				s.Reason)

			result, err := m.spawnTask(ctx, s.AgentName, adviceTask)
			resultsChan <- adviceResult{Name: s.AgentName, Reason: s.Reason, Result: result, Err: err}
		})
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []adviceResult
	for r := range resultsChan {
		results = append(results, r)
	}
	return results
}

// gatherSpecialistCritique queries specialists to critique the execution result
func (m Model) gatherSpecialistCritique(ctx context.Context, verb string, files []string, specialists []shards.SpecialistMatch, executionResult string) []adviceResult {
	resultsChan := make(chan adviceResult, len(specialists))
	var wg sync.WaitGroup

	logging.Shards("Gathering critique for %s on %d files", verb, len(files))

	// Truncate execution result if too long
	truncatedResult := executionResult
	if len(truncatedResult) > 3000 {
		truncatedResult = truncatedResult[:3000] + "\n... [truncated]"
	}

	for _, spec := range specialists {
		s := spec
		wg.Go(func() {
			// Critique task prompt
			critiqueTask := fmt.Sprintf(`CRITIQUE REQUEST: Review the following %s result from your domain expertise perspective.

Target files: %s
Your expertise: %s

=== EXECUTION RESULT ===
%s
=== END RESULT ===

Please provide:
1. ✅ What was done well
2. ⚠️ Potential issues or concerns from your domain perspective
3. 💡 Suggestions for improvement (if any)

Be concise. Focus on domain-specific insights others might miss.`,
				strings.TrimPrefix(verb, "/"),
				strings.Join(s.Files, ", "),
				s.Reason,
				truncatedResult)

			result, err := m.spawnTask(ctx, s.AgentName, critiqueTask)
			resultsChan <- adviceResult{Name: s.AgentName, Reason: s.Reason, Result: result, Err: err}
		})
	}

	go func() { wg.Wait(); close(resultsChan) }()

	var results []adviceResult
	for r := range resultsChan {
		results = append(results, r)
	}
	return results
}

// spawnResult holds the result from a parallel shard spawn
type spawnResult struct {
	Name     string
	Result   string
	Err      error
	Task     string        // Task that was executed
	Duration time.Duration // Execution duration
}

// formatParallelResults formats the output for parallel execution mode
func (m Model) formatParallelResults(verb, shardType, task, target string, results []spawnResult, startTime time.Time) tea.Msg {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Multi-Specialist %s Complete\n\n", strings.Title(strings.TrimPrefix(verb, "/"))))
	sb.WriteString(fmt.Sprintf("**Target**: %s\n", target))
	sb.WriteString(fmt.Sprintf("**Duration**: %s\n\n", time.Since(startTime).Round(time.Second)))

	participants := make([]string, 0, len(results))
	var combinedResult strings.Builder
	allFacts := make([]core.Fact, 0)

	for _, r := range results {
		participants = append(participants, r.Name)
		if r.Err != nil {
			sb.WriteString(fmt.Sprintf("### %s (failed)\n\nError: %v\n\n", r.Name, r.Err))
		} else {
			sb.WriteString(fmt.Sprintf("### %s\n\n%s\n\n", r.Name, r.Result))
			combinedResult.WriteString(r.Result)
			combinedResult.WriteString("\n\n")

			shardID := fmt.Sprintf("%s-%d", r.Name, time.Now().UnixNano())
			facts := m.shardMgr.ResultToFacts(shardID, r.Name, task, r.Result, r.Err)
			allFacts = append(allFacts, facts...)
		}
	}

	sb.WriteString(fmt.Sprintf("**Participants**: %s\n", strings.Join(participants, ", ")))

	if m.kernel != nil && len(allFacts) > 0 {
		if err := m.kernel.LoadFacts(allFacts); err != nil {
			logging.Routing("[delegation] failed to load parallel facts: %v", err)
		}
	}

	m.ReportStatus(fmt.Sprintf("%s + specialists complete", shardType))
	return assistantMsg{
		Surface: sb.String(),
		ShardResult: &ShardResultPayload{
			ShardType: shardType,
			Task:      task,
			Result:    combinedResult.String(),
			Facts:     allFacts,
		},
	}
}

// formatAdvisorNames extracts advisor names for display
func formatAdvisorNames(results []adviceResult) string {
	names := make([]string, 0, len(results))
	for _, r := range results {
		names = append(names, r.Name)
	}
	return strings.Join(names, ", ")
}
