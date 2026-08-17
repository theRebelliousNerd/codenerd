package context

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"context"
	"fmt"

	// NERD-EVOLVE-START: context_compilation_pipeline

	// NERD-EVOLVE-END: context_compilation_pipeline
	"strings"
	"sync"
	"time"
	"slices"
)

// =============================================================================
// Context Compressor
// =============================================================================
// Implements §8.2: Infinite Context via Semantic Compression
// The system achieves "Infinite Context" by continuously discarding surface text
// and retaining only logical state. Target compression ratio: >100:1

// Compressor manages semantic compression of conversation history.
type Compressor struct {
	mu sync.RWMutex

	// Dependencies
	kernel    *core.RealKernel
	store     *store.LocalStore
	llmClient perception.LLMClient

	// Configuration
	config CompressorConfig

	// Engines
	activation *ActivationEngine
	budget     *TokenBudget
	serializer *FactSerializer
	counter    *TokenCounter

	// State
	turnNumber     int
	recentTurns    []CompressedTurn
	rollingSummary RollingSummary
	sessionID      string

	// Metrics
	totalOriginalTokens   int
	totalCompressedTokens int

	// selection tracks how often the kernel gate vs the Go activation engine
	// decided the ACTIVE CONTEXT block. Guarded by mu.
	selection SelectionStats

	// feedbackStore is retained (not just handed to the activation engine) so
	// the third learning loop's state is inspectable from the compressor —
	// glass-box surfaces have no other handle on it.
	feedbackStore *ContextFeedbackStore
}

// NewCompressor creates a new context compressor.
func NewCompressor(kernel *core.RealKernel, localStorage *store.LocalStore, llmClient perception.LLMClient) *Compressor {
	cfg := DefaultConfig()
	logging.Context("Compressor initialized with default config: budget=%d tokens, threshold=%.0f%%, window=%d turns",
		cfg.TotalBudget, cfg.CompressionThreshold*100, cfg.RecentTurnWindow)

	// GAP-016 FIX: Load corpus-based serialization order for deterministic fact ordering
	serializer := NewFactSerializer()
	if kernel != nil {
		if corpus := kernel.GetPredicateCorpus(); corpus != nil {
			serializer.LoadSerializationOrderFromCorpus(corpus)
			logging.Context("Loaded corpus-based serialization order for context facts")
		}
	}

	return &Compressor{
		kernel:     kernel,
		store:      localStorage,
		llmClient:  llmClient,
		config:     cfg,
		activation: NewActivationEngine(cfg),
		budget:     NewTokenBudget(cfg),
		serializer: serializer,
		counter:    NewTokenCounter(),
		sessionID:  fmt.Sprintf("session_%d", time.Now().Unix()),
	}
}

// refreshActivationContextsLocked derives campaign/issue activation contexts from kernel facts.
// Call only when c.mu is held.
// OPTIMIZATION: Uses single QueryAll() instead of N separate Query() calls (10-50x faster).
func (c *Compressor) refreshActivationContextsLocked() {
	if c.kernel == nil || c.activation == nil {
		return
	}

	// OPTIMIZATION: Fetch ALL facts once, then filter in memory
	allFactsByPredicate, err := c.kernel.QueryAll()
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("refreshActivationContextsLocked: QueryAll failed: %v", err)
		return
	}

	// Helper to get facts by predicate from the map
	getFacts := func(pred string) []core.Fact {
		if facts, ok := allFactsByPredicate[pred]; ok {
			return facts
		}
		return nil
	}

	c.refreshCampaignContextLocked(getFacts)
	c.refreshIssueContextLocked(getFacts)
	c.refreshBackReferenceContextLocked(getFacts)
}

func (c *Compressor) refreshCampaignContextLocked(getFacts func(pred string) []core.Fact) {
	// -------------------------------------------------------------------------
	// Campaign context (phase-aware activation)
	// -------------------------------------------------------------------------
	campaignFacts := getFacts("current_campaign")
	if len(campaignFacts) == 0 {
		c.activation.ClearCampaignContext()
	} else {
		campaignID, _ := campaignFacts[len(campaignFacts)-1].Args[0].(string)

		phaseID := ""
		phaseName := ""
		if phases := getFacts("current_phase"); len(phases) > 0 {
			phaseID, _ = phases[len(phases)-1].Args[0].(string)
			// Find phase name from campaign_phase facts.
			if allPhases := getFacts("campaign_phase"); len(allPhases) > 0 {
				for _, f := range allPhases {
					if len(f.Args) >= 3 {
						id, _ := f.Args[0].(string)
						if id == phaseID {
							phaseName, _ = f.Args[2].(string)
							break
						}
					}
				}
			}
		}

		taskID := ""
		taskDesc := ""
		if tasks := getFacts("next_campaign_task"); len(tasks) > 0 {
			taskID, _ = tasks[len(tasks)-1].Args[0].(string)
			if allTasks := getFacts("campaign_task"); len(allTasks) > 0 {
				for _, f := range allTasks {
					if len(f.Args) >= 3 {
						id, _ := f.Args[0].(string)
						if id == taskID {
							taskDesc, _ = f.Args[2].(string)
							break
						}
					}
				}
			}
		}

		// Phase objectives as goals.
		var phaseGoals []string
		if phaseID != "" {
			if objectives := getFacts("phase_objective"); len(objectives) > 0 {
				for _, f := range objectives {
					if len(f.Args) >= 3 {
						pid, _ := f.Args[0].(string)
						if pid == phaseID {
							if desc, ok := f.Args[2].(string); ok && desc != "" {
								phaseGoals = append(phaseGoals, desc)
							}
						}
					}
				}
			}
		}

		// Task artifacts as relevant files/symbols.
		filesSet := make(map[string]struct{})
		symbolsSet := make(map[string]struct{})
		if taskID != "" {
			if artifacts := getFacts("task_artifact"); len(artifacts) > 0 {
				for _, f := range artifacts {
					if len(f.Args) >= 3 {
						tid, _ := f.Args[0].(string)
						if tid != taskID {
							continue
						}
						atype, _ := f.Args[1].(string)
						path, _ := f.Args[2].(string)
						if path == "" {
							continue
						}
						lowType := strings.ToLower(atype)
						if strings.Contains(lowType, "symbol") {
							symbolsSet[path] = struct{}{}
						} else {
							filesSet[path] = struct{}{}
						}
					}
				}
			}
		}

		var relevantFiles []string
		for p := range filesSet {
			relevantFiles = append(relevantFiles, p)
		}
		var relevantSymbols []string
		for s := range symbolsSet {
			relevantSymbols = append(relevantSymbols, s)
		}

		currentPhase := phaseName
		if currentPhase == "" {
			currentPhase = phaseID
		}
		currentTask := taskDesc
		if currentTask == "" {
			currentTask = taskID
		}

		c.activation.SetCampaignContext(&CampaignActivationContext{
			CampaignID:      campaignID,
			CurrentPhase:    currentPhase,
			CurrentTask:     currentTask,
			PhaseGoals:      phaseGoals,
			RelevantFiles:   relevantFiles,
			RelevantSymbols: relevantSymbols,
		})
	}

}

func (c *Compressor) refreshIssueContextLocked(getFacts func(pred string) []core.Fact) {
	// -------------------------------------------------------------------------
	// Issue context (issue-driven activation)
	// -------------------------------------------------------------------------
	issueID := ""
	source := ""
	if c.kernel.IsPredicateDeclared("swebench_instance") {
		if swe := getFacts("swebench_instance"); len(swe) > 0 {
			issueID, _ = swe[len(swe)-1].Args[0].(string)
			source = "swebench"
		}
	}
	if issueID == "" {
		if issues := getFacts("issue_context"); len(issues) > 0 {
			issueID, _ = issues[len(issues)-1].Args[0].(string)
			source = "issue_tracker"
		} else if kws := getFacts("issue_keyword"); len(kws) > 0 {
			issueID, _ = kws[len(kws)-1].Args[0].(string)
			source = "issue_tracker"
		}
	}

	issueText := ""
	if texts := getFacts("issue_text"); len(texts) > 0 {
		if issueID == "" {
			last := texts[len(texts)-1]
			if len(last.Args) >= 1 {
				issueID, _ = last.Args[0].(string)
			}
			if len(last.Args) >= 2 {
				issueText, _ = last.Args[1].(string)
			}
			if source == "" {
				source = "issue_tracker"
			}
		} else {
			for _, f := range texts {
				if len(f.Args) >= 2 {
					id, _ := f.Args[0].(string)
					if id == issueID {
						issueText, _ = f.Args[1].(string)
						break
					}
				}
			}
		}
	}

	if issueID == "" {
		c.activation.ClearIssueContext()
		return
	}

	keywords := make(map[string]float64)
	if kws := getFacts("issue_keyword"); len(kws) > 0 {
		for _, f := range kws {
			if len(f.Args) >= 3 {
				id, _ := f.Args[0].(string)
				if id != issueID {
					continue
				}
				kw, _ := f.Args[1].(string)
				if kw == "" {
					continue
				}
				// issue_keyword's Weight is declared /number, so a producer must
				// scale its 0..1 ratio to integer percent (types.PercentFromRatio)
				// — a fractional float is rejected by the kernel outright and
				// never arrives. An integer therefore means percent and has to be
				// divided back down; computeIssueScore clamps to 1.0, so reading
				// 90 as a raw weight would silently flatten every keyword to the
				// maximum boost.
				var weight float64
				switch v := f.Args[2].(type) {
				case float64:
					weight = v
				case int64:
					weight = float64(v) / 100.0
				case int:
					weight = float64(v) / 100.0
				default:
					weight = 0.5
				}
				keywords[kw] = weight
			}
		}
	}

	var mentionedFiles []string
	if mentions := getFacts("file_mentioned"); len(mentions) > 0 {
		for _, f := range mentions {
			if len(f.Args) >= 2 {
				file, _ := f.Args[0].(string)
				id, _ := f.Args[1].(string)
				if id == issueID && file != "" {
					mentionedFiles = append(mentionedFiles, file)
				}
			}
		}
	}

	tieredFiles := make(map[string]int)
	if tiers := getFacts("tiered_context_file"); len(tiers) > 0 {
		for _, f := range tiers {
			if len(f.Args) >= 3 {
				id, _ := f.Args[0].(string)
				if id != issueID {
					continue
				}
				file, _ := f.Args[1].(string)
				tierStr, _ := f.Args[2].(string) // /tier1, /tier2...
				if file == "" || tierStr == "" {
					continue
				}
				tierNum := 0
				trim := strings.TrimPrefix(strings.ToLower(tierStr), "/tier")
				if n, err := fmt.Sscanf(trim, "%d", &tierNum); err == nil && n == 1 {
					// ok
				} else {
					tierNum = 0
				}
				if tierNum > 0 {
					tieredFiles[file] = tierNum
				}
			}
		}
	}

	var expectedTests []string
	if source == "swebench" {
		if exp := getFacts("swebench_expected_fail_to_pass"); len(exp) > 0 {
			for _, f := range exp {
				if len(f.Args) >= 2 {
					id, _ := f.Args[0].(string)
					if id != issueID {
						continue
					}
					testName, _ := f.Args[1].(string)
					if testName != "" {
						expectedTests = append(expectedTests, testName)
					}
				}
			}
		}
		if exp := getFacts("swebench_expected_pass_to_pass"); len(exp) > 0 {
			for _, f := range exp {
				if len(f.Args) >= 2 {
					id, _ := f.Args[0].(string)
					if id != issueID {
						continue
					}
					testName, _ := f.Args[1].(string)
					if testName != "" {
						expectedTests = append(expectedTests, testName)
					}
				}
			}
		}
	}

	c.activation.SetIssueContext(&IssueActivationContext{
		IssueID:        issueID,
		IssueText:      issueText,
		Keywords:       keywords,
		MentionedFiles: mentionedFiles,
		TieredFiles:    tieredFiles,
		ErrorTypes:     nil,
		ExpectedTests:  expectedTests,
		Source:         source,
	})

}

func (c *Compressor) refreshBackReferenceContextLocked(getFacts func(pred string) []core.Fact) {
	// -------------------------------------------------------------------------
	// Back-reference context (follow-up question activation)
	// -------------------------------------------------------------------------
	// When a turn references back to a previous turn, we boost facts from that turn.
	// This enables "What was the original error?" type queries to retrieve old context.
	backRefs := getFacts("turn_references_back")
	if len(backRefs) == 0 {
		c.activation.ClearBackReferenceContext()
		return
	}

	// Collect referenced turn IDs
	referencedTurnsMap := make(map[int]bool)
	var referencedTurnIDs []int
	var referenceStrength float64 = 1.0

	for _, f := range backRefs {
		if len(f.Args) >= 2 {
			var referencedTurn int
			switch v := f.Args[1].(type) {
			case int:
				referencedTurn = v
			case int64:
				referencedTurn = int(v)
			case float64:
				referencedTurn = int(v)
			}
			if referencedTurn >= 0 && !referencedTurnsMap[referencedTurn] {
				referencedTurnsMap[referencedTurn] = true
				referencedTurnIDs = append(referencedTurnIDs, referencedTurn)
			}
		}
	}

	if len(referencedTurnIDs) == 0 {
		c.activation.ClearBackReferenceContext()
		return
	}

	// Collect topics, files, symbols, and errors from referenced turns
	var referencedTopics []string
	var referencedFiles []string
	var referencedSymbols []string
	var referencedErrors []string

	// turn_topic(turnID, topic)
	if topics := getFacts("turn_topic"); len(topics) > 0 {
		for _, f := range topics {
			if len(f.Args) >= 2 {
				var turnID int
				switch v := f.Args[0].(type) {
				case int:
					turnID = v
				case int64:
					turnID = int(v)
				case float64:
					turnID = int(v)
				}
				if referencedTurnsMap[turnID] {
					if topic, ok := f.Args[1].(string); ok && topic != "" {
						referencedTopics = append(referencedTopics, topic)
					}
				}
			}
		}
	}

	// turn_references_file(turnID, filePath)
	if files := getFacts("turn_references_file"); len(files) > 0 {
		for _, f := range files {
			if len(f.Args) >= 2 {
				var turnID int
				switch v := f.Args[0].(type) {
				case int:
					turnID = v
				case int64:
					turnID = int(v)
				case float64:
					turnID = int(v)
				}
				if referencedTurnsMap[turnID] {
					if file, ok := f.Args[1].(string); ok && file != "" {
						referencedFiles = append(referencedFiles, file)
					}
				}
			}
		}
	}

	// turn_references_symbol(turnID, symbol)
	if symbols := getFacts("turn_references_symbol"); len(symbols) > 0 {
		for _, f := range symbols {
			if len(f.Args) >= 2 {
				var turnID int
				switch v := f.Args[0].(type) {
				case int:
					turnID = v
				case int64:
					turnID = int(v)
				case float64:
					turnID = int(v)
				}
				if referencedTurnsMap[turnID] {
					if symbol, ok := f.Args[1].(string); ok && symbol != "" {
						referencedSymbols = append(referencedSymbols, symbol)
					}
				}
			}
		}
	}

	// turn_error_message(turnID, errorMsg)
	if errors := getFacts("turn_error_message"); len(errors) > 0 {
		for _, f := range errors {
			if len(f.Args) >= 2 {
				var turnID int
				switch v := f.Args[0].(type) {
				case int:
					turnID = v
				case int64:
					turnID = int(v)
				case float64:
					turnID = int(v)
				}
				if referencedTurnsMap[turnID] {
					if errMsg, ok := f.Args[1].(string); ok && errMsg != "" {
						referencedErrors = append(referencedErrors, errMsg)
					}
				}
			}
		}
	}

	c.activation.SetBackReferenceContext(&BackReferenceActivationContext{
		ReferencedTurnIDs: referencedTurnIDs,
		ReferenceStrength: referenceStrength,
		ReferencedTopics:  referencedTopics,
		ReferencedFiles:   referencedFiles,
		ReferencedSymbols: referencedSymbols,
		ReferencedErrors:  referencedErrors,
	})
}

// SetSessionID sets the logical session ID for persistence/rehydration.
// Call this after the chat layer resolves the real session ID.
func (c *Compressor) SetSessionID(sessionID string) {
	if sessionID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = sessionID
}

// GetSessionID returns the current session ID.
func (c *Compressor) GetSessionID() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure activation engine has up-to-date campaign/issue context.
	c.refreshActivationContextsLocked()
	return c.sessionID
}

// SetFeedbackStore wires the context feedback store to the activation engine.
// This enables the third feedback loop: learning which context facts are useful.
func (c *Compressor) SetFeedbackStore(store *ContextFeedbackStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.feedbackStore = store
	if c.activation != nil {
		c.activation.SetFeedbackStore(store)
	}
}

// GetFeedbackStats reports the state of the context-learning loop so operators
// (and glass-box surfaces) can see which predicates the system learned to
// trust. Returns a zero-valued FeedbackStats when no store is wired.
func (c *Compressor) GetFeedbackStats(topN int) FeedbackStats {
	c.mu.RLock()
	store := c.feedbackStore
	c.mu.RUnlock()
	return CollectFeedbackStats(store, topN)
}

// NewCompressorWithConfig creates a compressor with custom configuration.
func NewCompressorWithConfig(kernel *core.RealKernel, localStorage *store.LocalStore, llmClient perception.LLMClient, cfg config.ContextWindowConfig) *Compressor {
	// Convert config.ContextWindowConfig to context.CompressorConfig
	compCfg := CompressorConfig{
		TotalBudget:            cfg.MaxTokens,
		CoreReserve:            cfg.MaxTokens * cfg.CoreReservePercent / 100,
		AtomReserve:            cfg.MaxTokens * cfg.AtomReservePercent / 100,
		HistoryReserve:         cfg.MaxTokens * cfg.HistoryReservePercent / 100,
		WorkingReserve:         cfg.MaxTokens * cfg.WorkingReservePercent / 100,
		RecentTurnWindow:       cfg.RecentTurnWindow,
		CompressionThreshold:   cfg.CompressionThreshold,
		TargetCompressionRatio: cfg.TargetCompressionRatio,
		ActivationThreshold:    cfg.ActivationThreshold,
		PredicatePriorities:    DefaultConfig().PredicatePriorities,
	}

	logging.Context("Compressor initialized with custom config: budget=%d tokens, threshold=%.0f%%, window=%d turns",
		compCfg.TotalBudget, compCfg.CompressionThreshold*100, compCfg.RecentTurnWindow)
	logging.ContextDebug("Token allocation: core=%d, atoms=%d, history=%d, working=%d",
		compCfg.CoreReserve, compCfg.AtomReserve, compCfg.HistoryReserve, compCfg.WorkingReserve)

	return newCompressorWithCompressorConfig(kernel, localStorage, llmClient, compCfg)
}

// NewCompressorWithParams creates a compressor with explicit parameters.
// This is useful when the caller doesn't have access to internal/config types.
func NewCompressorWithParams(kernel *core.RealKernel, localStorage *store.LocalStore, llmClient perception.LLMClient,
	maxTokens int, corePercent, atomPercent, historyPercent, workingPercent int,
	recentWindow int, compressionThreshold, targetRatio, activationThreshold float64) *Compressor {

	compCfg := CompressorConfig{
		TotalBudget:            maxTokens,
		CoreReserve:            maxTokens * corePercent / 100,
		AtomReserve:            maxTokens * atomPercent / 100,
		HistoryReserve:         maxTokens * historyPercent / 100,
		WorkingReserve:         maxTokens * workingPercent / 100,
		RecentTurnWindow:       recentWindow,
		CompressionThreshold:   compressionThreshold,
		TargetCompressionRatio: targetRatio,
		ActivationThreshold:    activationThreshold,
		PredicatePriorities:    DefaultConfig().PredicatePriorities,
	}

	logging.Context("Compressor initialized with params: budget=%d tokens, threshold=%.0f%%, window=%d turns, target_ratio=%.1f:1",
		maxTokens, compressionThreshold*100, recentWindow, targetRatio)
	logging.ContextDebug("Token allocation: core=%d%% (%d), atoms=%d%% (%d), history=%d%% (%d), working=%d%% (%d)",
		corePercent, compCfg.CoreReserve, atomPercent, compCfg.AtomReserve,
		historyPercent, compCfg.HistoryReserve, workingPercent, compCfg.WorkingReserve)

	return newCompressorWithCompressorConfig(kernel, localStorage, llmClient, compCfg)
}

// newCompressorWithCompressorConfig is the internal constructor.
func newCompressorWithCompressorConfig(kernel *core.RealKernel, localStorage *store.LocalStore, llmClient perception.LLMClient, compCfg CompressorConfig) *Compressor {

	return &Compressor{
		kernel:     kernel,
		store:      localStorage,
		llmClient:  llmClient,
		config:     compCfg,
		activation: NewActivationEngine(compCfg),
		budget:     NewTokenBudget(compCfg),
		serializer: NewFactSerializer(),
		counter:    NewTokenCounter(),
		sessionID:  fmt.Sprintf("session_%d", time.Now().Unix()),
	}
}

// =============================================================================
// Context Building
// =============================================================================

// BuildContext creates the compressed context for an LLM call.
// This replaces raw conversation history with semantically compressed state.
// Returns ErrContextWindowExceeded if the context would exceed the hard limit.
func (c *Compressor) BuildContext(ctx context.Context) (*CompressedContext, error) {
	timer := logging.StartTimer(logging.CategoryContext, "BuildContext")
	defer timer.Stop()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure activation engine has up-to-date campaign/issue context.
	c.refreshActivationContextsLocked()

	// ENFORCEMENT: Check if we're already over budget before building
	if err := c.budget.CheckTotalBudget(); err != nil {
		logging.Get(logging.CategoryContext).Error("BuildContext: context window limit already exceeded: %v", err)
		return nil, err
	}

	// 1. Get all facts from kernel
	allFacts := slices.Collect(c.kernel.GetAllFactsSeq())
	logging.ContextDebug("Building context: %d total facts in kernel", len(allFacts))

	// 2. Find current intent (for activation scoring)
	// OPTIMIZATION: Already have allFacts organized by predicate from QueryAll
	var currentIntent *core.Fact
	allFactsByPred, err := c.kernel.QueryAll()
	if err == nil {
		if intentFacts, ok := allFactsByPred["user_intent"]; ok && len(intentFacts) > 0 {
			currentIntent = &intentFacts[len(intentFacts)-1]
			logging.ContextDebug("Current intent: %s", currentIntent.String())
		}
	}

	// 3. Score and filter facts using activation engine
	// NERD-EVOLVE-START: context_compilation_pipeline
	// C1+C4: Try kernel-derived context selection first; fall back to Go activation engine.
	// The kernel derives should_include_context(Fact, Priority) from user_intent,
	// focus_resolution, modified, dependency_link, and test failure predicates.
	activationTimer := logging.StartTimer(logging.CategoryContext, "ActivationScoring")
	var scoredFacts []ScoredFact
	reason := reasonKernelSelected
	kernelFacts, kernelErr := c.kernel.Query("should_include_context")
	switch {
	case kernelErr != nil:
		reason = reasonQueryError
	case len(kernelFacts) == 0:
		reason = reasonNoKernelFacts
	default:
		scoredFacts = c.buildKernelDerivedContext(kernelFacts, allFacts)
		logging.ContextDebug("C1+C4 kernel context: %d facts selected from %d should_include_context results",
			len(scoredFacts), len(kernelFacts))
		if len(scoredFacts) == 0 {
			// The kernel had an opinion but none of the entities it named
			// resolved to a fact we hold. Previously this branch left
			// scoredFacts nil and skipped the fallback entirely, so a live
			// session (which always has user_intent, hence always non-empty
			// should_include_context) shipped an EMPTY active-context block.
			reason = reasonUnresolved
		}
	}
	if len(scoredFacts) == 0 {
		// Fallback: Go-side activation engine (original path)
		scoredFacts = c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)
		logging.ContextDebug("Go activation fallback (%s): %d facts selected (budget: %d tokens)",
			reason, len(scoredFacts), c.config.AtomReserve)
	}
	c.recordSelectionLocked(reason, len(kernelFacts), len(scoredFacts))
	activationTimer.Stop()
	logging.ContextDebug("Activation scoring: %d facts selected (budget: %d tokens)", len(scoredFacts), c.config.AtomReserve)
	// NERD-EVOLVE-END: context_compilation_pipeline

	// 4. Get core facts (constitutional, always included)
	coreFacts := c.getCoreFacts()
	logging.ContextDebug("Core facts (constitutional): %d facts", len(coreFacts))

	// 5. Build context using builder
	builder := NewContextBlockBuilder()
	recentTurns := c.recentTurns[max(0, len(c.recentTurns)-c.config.RecentTurnWindow):]
	compressedCtx := builder.Build(
		coreFacts,
		scoredFacts,
		c.rollingSummary.Text,
		recentTurns,
		c.turnNumber,
	)

	// 6. Update usage
	compressedCtx.TokenUsage.Available = c.config.TotalBudget - compressedCtx.TokenUsage.Total

	logging.Context("Context built: %d tokens used, %d available (core=%d, atoms=%d, history=%d, recent=%d)",
		compressedCtx.TokenUsage.Total, compressedCtx.TokenUsage.Available,
		compressedCtx.TokenUsage.Core, compressedCtx.TokenUsage.Atoms,
		compressedCtx.TokenUsage.History, compressedCtx.TokenUsage.Recent)

	return compressedCtx, nil
}

// getCoreFacts returns constitutional facts that are always included.
func (c *Compressor) getCoreFacts() []core.Fact {
	var coreFacts []core.Fact

	// Always include permission-related facts. Silently swallowing the
	// error here was the same class of bug as the recent prompt_atom
	// silent-drop — if a safety predicate Query failed, the resulting
	// empty coreFacts slice would let the compressor build a context
	// without any safety facts, and policy rules downstream had no way
	// to know whether the kernel had refused the row or simply had
	// nothing to say.
	predicates := []string{"permitted", "dangerous_action", "admin_override", "security_violation", "block_commit"}

	for _, pred := range predicates {
		facts, err := c.kernel.Query(pred)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("getCoreFacts: safety-predicate query failed predicate=%s: %v", pred, err)
			continue
		}
		coreFacts = append(coreFacts, facts...)
	}

	return coreFacts
}

// GetContextString returns the serialized context string for LLM injection.
func (c *Compressor) GetContextString(ctx context.Context) (string, error) {
	compressedCtx, err := c.BuildContext(ctx)
	if err != nil {
		logging.Get(logging.CategoryContext).Error("Failed to build context: %v", err)
		return "", err
	}

	contextStr := c.serializer.SerializeCompressedContext(compressedCtx)
	logging.ContextDebug("Serialized context string: %d characters", len(contextStr))
	return contextStr, nil
}
