package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"strings"
	"time"
)

// ContextPager manages context window for campaign execution.
// It implements phase-aware context paging with compression and activation.
type ContextPager struct {
	kernel    *core.RealKernel
	llmClient perception.LLMClient

	// Context window budget (approximate token counts)
	totalBudget     int // Total tokens available
	coreReserve     int // Always-present facts (campaign identity, rules)
	phaseReserve    int // Current phase context
	historyReserve  int // Compressed phase summaries
	workingReserve  int // Current task execution
	prefetchReserve int // Upcoming task hints

	// Tracking
	usedTokens int
}

// NewContextPager creates a new context pager.
func NewContextPager(kernel *core.RealKernel, llmClient perception.LLMClient) *ContextPager {
	return &ContextPager{
		kernel:          kernel,
		llmClient:       llmClient,
		totalBudget:     100000, // 100k tokens default
		coreReserve:     5000,   // 5% for core facts
		phaseReserve:    30000,  // 30% for current phase
		historyReserve:  15000,  // 15% for compressed history
		workingReserve:  40000,  // 40% for working memory
		prefetchReserve: 10000,  // 10% for prefetch
	}
}

// SetBudget updates the total token budget.
func (cp *ContextPager) SetBudget(tokens int) {
	cp.totalBudget = tokens
	// Recalculate reserves
	cp.coreReserve = tokens * 5 / 100
	cp.phaseReserve = tokens * 30 / 100
	cp.historyReserve = tokens * 15 / 100
	cp.workingReserve = tokens * 40 / 100
	cp.prefetchReserve = tokens * 10 / 100
}

// GetUsage returns current context usage stats.
func (cp *ContextPager) GetUsage() (used, total int, utilization float64) {
	return cp.usedTokens, cp.totalBudget, float64(cp.usedTokens) / float64(cp.totalBudget)
}

// ActivatePhase loads context for a new phase.
func (cp *ContextPager) ActivatePhase(ctx context.Context, phase *Phase) error {
	if phase == nil {
		return nil
	}

	// 1. Get context profile for this phase
	profile, err := cp.getContextProfile(phase.ContextProfile)
	if err != nil {
		// Use default profile if not found
		profile = &ContextProfile{
			ID:              phase.ContextProfile,
			RequiredSchemas: []string{"file_topology", "symbol_graph", "diagnostic"},
			RequiredTools:   []string{"fs_read", "fs_write", "exec_cmd"},
			FocusPatterns:   []string{"**/*"},
		}
	}

	// 2. Boost activation for phase-specific facts
	for _, pattern := range profile.FocusPatterns {
		cp.boostPattern(pattern, 120)
	}

	// 3. Load phase context atoms
	for _, task := range phase.Tasks {
		// Boost task artifacts
		for _, artifact := range task.Artifacts {
			cp.kernel.Assert(core.Fact{
				Predicate: "phase_context_atom",
				Args:      []interface{}{phase.ID, fmt.Sprintf("file_topology(%q, _, _, _, _)", artifact.Path), 100},
			})
		}
	}

	// 4. Suppress irrelevant schemas (negative activation)
	allSchemas := []string{
		"dom_node", "geometry", "interactable", "computed_style", // Browser
		"vector_recall", // Memory (if not research phase)
	}
	for _, schema := range allSchemas {
		if !contains(profile.RequiredSchemas, schema) {
			cp.suppressSchema(schema)
		}
	}

	// 5. Update usage estimate
	cp.usedTokens = cp.estimatePhaseTokens(phase)

	return nil
}

// CompressPhase summarizes and stores a completed phase's context.
// Returns the summary, original atom count, and timestamp for persistence.
func (cp *ContextPager) CompressPhase(ctx context.Context, phase *Phase) (string, int, time.Time, error) {
	if phase == nil {
		return "", 0, time.Time{}, nil
	}

	// 1. Gather facts from this phase
	phaseAtoms, err := cp.kernel.Query("phase_context_atom")
	if err != nil {
		return "", 0, time.Time{}, err
	}

	// Filter to this phase
	var phaseFacts []core.Fact
	for _, atom := range phaseAtoms {
		if len(atom.Args) >= 1 && fmt.Sprintf("%v", atom.Args[0]) == phase.ID {
			phaseFacts = append(phaseFacts, atom)
		}
	}

	// 2. Build summary of what was accomplished
	var accomplishments []string
	for _, task := range phase.Tasks {
		if task.Status == TaskCompleted {
			accomplishments = append(accomplishments, fmt.Sprintf("- %s", task.Description))
			for _, artifact := range task.Artifacts {
				accomplishments = append(accomplishments, fmt.Sprintf("  → Created: %s", artifact.Path))
			}
		}
	}

	// 3. Use LLM to create concise summary if we have accomplishments
	var summary string
	if len(accomplishments) > 0 {
		prompt := fmt.Sprintf(`Summarize what was accomplished in this phase (max 100 words):

Phase: %s
Completed Tasks:
%s

Summary:`, phase.Name, strings.Join(accomplishments, "\n"))

		resp, err := cp.llmClient.Complete(ctx, prompt)
		if err != nil {
			// Fallback to simple summary
			summary = fmt.Sprintf("Phase '%s' completed. %d tasks done: %s", phase.Name, len(accomplishments), strings.Join(accomplishments[:min(3, len(accomplishments))], "; "))
		} else {
			summary = strings.TrimSpace(resp)
		}
	} else {
		summary = fmt.Sprintf("Phase '%s' completed with no recorded accomplishments.", phase.Name)
	}

	// 4. Store compression
	now := time.Now()
	cp.kernel.Assert(core.Fact{
		Predicate: "context_compression",
		Args:      []interface{}{phase.ID, summary, len(phaseFacts), now.Unix()},
	})

	// 4b. Retract phase-specific context atoms now that they've been compressed
	_ = cp.kernel.RetractFact(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []interface{}{phase.ID},
	})

	// 5. Reduce activation of phase-specific facts
	for _, fact := range phaseFacts {
		if len(fact.Args) >= 2 {
			factPredicate := fmt.Sprintf("%v", fact.Args[1])
			cp.kernel.Assert(core.Fact{
				Predicate: "activation",
				Args:      []interface{}{factPredicate, -100},
			})
		}
	}

	// 6. Update compressed summary in the phase struct
	// (This should be done by the orchestrator, not here)

	return summary, len(phaseFacts), now, nil
}

// PrefetchNextTasks loads hints for upcoming tasks.
func (cp *ContextPager) PrefetchNextTasks(ctx context.Context, tasks []Task, limit int) error {
	if limit <= 0 {
		limit = 3
	}

	for i, task := range tasks {
		if i >= limit {
			break
		}

		// Boost artifacts for upcoming tasks
		for _, artifact := range task.Artifacts {
			cp.kernel.Assert(core.Fact{
				Predicate: "activation",
				Args:      []interface{}{fmt.Sprintf("file_topology(%q, _, _, _, _)", artifact.Path), 50},
			})
		}
	}

	return nil
}

// PruneIrrelevant removes facts not relevant to current phase.
func (cp *ContextPager) PruneIrrelevant(profile *ContextProfile) error {
	// Get all facts
	allFacts, err := cp.kernel.QueryAll()
	if err != nil {
		return err
	}

	// Determine which predicates to suppress
	irrelevantPredicates := []string{
		"dom_node", "attr", "geometry", "computed_style", "interactable", "visible_text", // Browser
	}

	// Check if browser is needed
	if contains(profile.RequiredSchemas, "browser") {
		irrelevantPredicates = []string{} // Keep browser facts
	}

	// Suppress irrelevant facts
	for _, pred := range irrelevantPredicates {
		if facts, ok := allFacts[pred]; ok {
			for range facts {
				cp.kernel.Assert(core.Fact{
					Predicate: "activation",
					Args:      []interface{}{pred, -200}, // Heavy suppression
				})
			}
		}
	}

	return nil
}

// getContextProfile retrieves a context profile from the kernel.
func (cp *ContextPager) getContextProfile(profileID string) (*ContextProfile, error) {
	facts, err := cp.kernel.Query("context_profile")
	if err != nil {
		return nil, err
	}

	for _, fact := range facts {
		if len(fact.Args) >= 4 && fmt.Sprintf("%v", fact.Args[0]) == profileID {
			return &ContextProfile{
				ID:              profileID,
				RequiredSchemas: strings.Split(fmt.Sprintf("%v", fact.Args[1]), ","),
				RequiredTools:   strings.Split(fmt.Sprintf("%v", fact.Args[2]), ","),
				FocusPatterns:   strings.Split(fmt.Sprintf("%v", fact.Args[3]), ","),
			}, nil
		}
	}

	return nil, fmt.Errorf("context profile %s not found", profileID)
}

// boostPattern boosts activation for files matching a pattern.
func (cp *ContextPager) boostPattern(pattern string, boost int) {
	// Assert activation boost for the pattern
	// The actual file matching is done by the kernel's spreading activation
	cp.kernel.Assert(core.Fact{
		Predicate: "activation",
		Args:      []interface{}{fmt.Sprintf("file_pattern(%q)", pattern), boost},
	})
}

// suppressSchema reduces activation for an entire schema.
func (cp *ContextPager) suppressSchema(schema string) {
	cp.kernel.Assert(core.Fact{
		Predicate: "activation",
		Args:      []interface{}{schema, -100},
	})
}

// estimatePhaseTokens estimates token usage for a phase.
func (cp *ContextPager) estimatePhaseTokens(phase *Phase) int {
	// Rough estimate:
	// - Each task description ~50 tokens
	// - Each artifact reference ~20 tokens
	// - Phase metadata ~100 tokens
	tokens := 100 // Base

	for _, task := range phase.Tasks {
		tokens += 50 // Description
		tokens += len(task.Artifacts) * 20
	}

	return tokens
}

// contains checks if a slice contains a string.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// min returns the minimum of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
