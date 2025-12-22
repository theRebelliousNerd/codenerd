package context

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
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
func (c *Compressor) refreshActivationContextsLocked() {
	if c.kernel == nil || c.activation == nil {
		return
	}

	// -------------------------------------------------------------------------
	// Campaign context (phase-aware activation)
	// -------------------------------------------------------------------------
	campaignFacts, _ := c.kernel.Query("current_campaign")
	if len(campaignFacts) == 0 {
		c.activation.ClearCampaignContext()
	} else {
		campaignID, _ := campaignFacts[len(campaignFacts)-1].Args[0].(string)

		phaseID := ""
		phaseName := ""
		if phases, _ := c.kernel.Query("current_phase"); len(phases) > 0 {
			phaseID, _ = phases[len(phases)-1].Args[0].(string)
			// Find phase name from campaign_phase facts.
			if allPhases, _ := c.kernel.Query("campaign_phase"); len(allPhases) > 0 {
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
		if tasks, _ := c.kernel.Query("next_campaign_task"); len(tasks) > 0 {
			taskID, _ = tasks[len(tasks)-1].Args[0].(string)
			if allTasks, _ := c.kernel.Query("campaign_task"); len(allTasks) > 0 {
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
			if objectives, _ := c.kernel.Query("phase_objective"); len(objectives) > 0 {
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
			if artifacts, _ := c.kernel.Query("task_artifact"); len(artifacts) > 0 {
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

	// -------------------------------------------------------------------------
	// Issue context (issue-driven activation)
	// -------------------------------------------------------------------------
	issueID := ""
	source := ""
	if swe, _ := c.kernel.Query("swebench_instance"); len(swe) > 0 {
		issueID, _ = swe[len(swe)-1].Args[0].(string)
		source = "swebench"
	} else if issues, _ := c.kernel.Query("issue_context"); len(issues) > 0 {
		issueID, _ = issues[len(issues)-1].Args[0].(string)
		source = "issue_tracker"
	} else if kws, _ := c.kernel.Query("issue_keyword"); len(kws) > 0 {
		issueID, _ = kws[len(kws)-1].Args[0].(string)
		source = "issue_tracker"
	}

	issueText := ""
	if texts, _ := c.kernel.Query("issue_text"); len(texts) > 0 {
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
	if kws, _ := c.kernel.Query("issue_keyword"); len(kws) > 0 {
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
				var weight float64
				switch v := f.Args[2].(type) {
				case float64:
					weight = v
				case int64:
					weight = float64(v)
				case int:
					weight = float64(v)
				default:
					weight = 0.5
				}
				keywords[kw] = weight
			}
		}
	}

	var mentionedFiles []string
	if mentions, _ := c.kernel.Query("file_mentioned"); len(mentions) > 0 {
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
	if tiers, _ := c.kernel.Query("tiered_context_file"); len(tiers) > 0 {
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
		if exp, _ := c.kernel.Query("swebench_expected_fail_to_pass"); len(exp) > 0 {
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
		if exp, _ := c.kernel.Query("swebench_expected_pass_to_pass"); len(exp) > 0 {
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
// Turn Processing (The Core Compression Loop)
// =============================================================================

// ProcessTurn handles a completed conversation turn.
// This is the main entry point for the compression loop.
//
// The loop:
// 1. User says "Fix server" -> Agent replies "Fixing..."
// 2. Extract control_packet atoms from the response
// 3. Commit atoms to kernel (the Logical Twin updates)
// 4. Delete the surface text "Fixing..." from history
// 5. Next turn sees only the atoms task_status(/server, /fixing)
func (c *Compressor) ProcessTurn(ctx context.Context, turn Turn) (*TurnResult, error) {
	timer := logging.StartTimer(logging.CategoryContext, fmt.Sprintf("ProcessTurn[%d]", turn.Number))
	defer timer.Stop()

	c.mu.Lock()
	defer c.mu.Unlock()

	logging.Context("Processing turn %d (role=%s)", turn.Number, turn.Role)

	result := &TurnResult{}

	// 1. Extract atoms from control packet
	var atoms []core.Fact
	if turn.ControlPacket != nil {
		extracted, err := ExtractAtomsFromControlPacket(turn.ControlPacket)
		if err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to extract atoms from control packet: %v", err)
		}
		atoms = extracted
		logging.ContextDebug("Extracted %d atoms from control packet", len(atoms))
	}

	// Add any pre-extracted atoms
	atoms = append(atoms, turn.ExtractedAtoms...)
	if len(turn.ExtractedAtoms) > 0 {
		logging.ContextDebug("Added %d pre-extracted atoms (total: %d)", len(turn.ExtractedAtoms), len(atoms))
	}

	// 2. Commit atoms to kernel
	committedCount := 0
	if len(atoms) > 0 && c.kernel != nil {
		if err := c.kernel.AssertBatch(atoms); err != nil {
			// Fallback: per-atom assert so one bad fact doesn't block the rest.
			for _, atom := range atoms {
				if err := c.kernel.Assert(atom); err != nil {
					logging.Get(logging.CategoryContext).Warn("Failed to assert atom %s: %v", atom.Predicate, err)
				} else {
					committedCount++
				}
			}
		} else {
			committedCount = len(atoms)
		}
	}
	result.CommittedAtoms = atoms
	logging.ContextDebug("Committed %d/%d atoms to kernel", committedCount, len(atoms))

	// Mark atoms as new for recency scoring
	c.activation.MarkNewFacts(atoms)

	// Refresh campaign/issue activation contexts from latest kernel state.
	c.refreshActivationContextsLocked()

	// 3. Process memory operations
	if turn.ControlPacket != nil && len(turn.ControlPacket.MemoryOperations) > 0 {
		logging.ContextDebug("Processing %d memory operations", len(turn.ControlPacket.MemoryOperations))
		for _, op := range turn.ControlPacket.MemoryOperations {
			c.processMemoryOperation(op)
		}
		result.MemoryOps = turn.ControlPacket.MemoryOperations
	}

	// 4. Create compressed turn (NO SURFACE TEXT)
	originalTokens := c.counter.CountString(turn.SurfaceResponse) + c.counter.CountString(turn.UserInput)
	compressed := CompressedTurn{
		TurnNumber:     turn.Number,
		Role:           turn.Role,
		Timestamp:      turn.Timestamp,
		OriginalTokens: originalTokens,
	}

	// Extract intent atom
	for _, atom := range atoms {
		if atom.Predicate == "user_intent" {
			compressed.IntentAtom = &atom
		} else if atom.Predicate == "focus_resolution" {
			compressed.FocusAtoms = append(compressed.FocusAtoms, atom)
		} else {
			compressed.ResultAtoms = append(compressed.ResultAtoms, atom)
		}
	}

	// Store mangle updates (not surface text!)
	if turn.ControlPacket != nil {
		compressed.MangleUpdates = turn.ControlPacket.MangleUpdates
		compressed.MemoryOperations = turn.ControlPacket.MemoryOperations
	}

	// 5. Add to recent turns (sliding window)
	c.recentTurns = append(c.recentTurns, compressed)

	// 6. Check if compression is needed
	compressedTokens := c.counter.CountTurn(compressed)
	c.totalOriginalTokens += originalTokens
	c.totalCompressedTokens += compressedTokens

	logging.ContextDebug("Turn %d tokens: original=%d, compressed=%d (total: orig=%d, comp=%d)",
		turn.Number, originalTokens, compressedTokens, c.totalOriginalTokens, c.totalCompressedTokens)

	// Recalculate token budget so compression decisions reflect current usage.
	c.recalcBudget(turn.Number, originalTokens)

	utilization := c.budget.Utilization()
	logging.ContextDebug("Token budget utilization: %.1f%% (threshold: %.1f%%)",
		utilization*100, c.config.CompressionThreshold*100)

	if c.shouldCompress() {
		logging.Context("COMPRESSION TRIGGERED: utilization %.1f%% exceeds threshold %.1f%%",
			utilization*100, c.config.CompressionThreshold*100)
		compressTimer := logging.StartTimer(logging.CategoryContext, "Compression")
		if err := c.compress(ctx); err != nil {
			logging.Get(logging.CategoryContext).Error("Compression failed: %v", err)
		}
		compressTimer.Stop()
		result.CompressionTriggered = true
	}

	// 7. Prune old turns from sliding window
	beforePrune := len(c.recentTurns)
	c.pruneRecentTurns()
	if len(c.recentTurns) < beforePrune {
		logging.ContextDebug("Pruned %d turns from sliding window (now: %d)", beforePrune-len(c.recentTurns), len(c.recentTurns))
	}

	// 8. Update turn number
	c.turnNumber = turn.Number

	// 9. Calculate final token usage
	result.TokenUsage = c.budget.GetUsage()
	logging.Context("Turn %d complete: %d atoms, %d recent turns, usage=%d/%d tokens (%.1f%%)",
		turn.Number, len(atoms), len(c.recentTurns),
		result.TokenUsage.Total, c.config.TotalBudget, utilization*100)

	// 10. Persist compressed state + activation analytics (best-effort)
	if c.store != nil {
		state := c.buildStateLocked()
		if data, err := MarshalCompressedState(state); err == nil {
			_ = c.store.StoreCompressedState(c.sessionID, c.turnNumber, string(data), state.CompressionRatio)
		}
		// Log top hot facts for long-term activation analytics.
		maxLogs := 50
		for i, sf := range state.HotFacts {
			if i >= maxLogs {
				break
			}
			_ = c.store.LogActivation(sf.Fact.String(), sf.Score)
		}
	}

	return result, nil
}

// processMemoryOperation handles a memory operation from the control packet.
func (c *Compressor) processMemoryOperation(op perception.MemoryOperation) {
	switch op.Op {
	case "promote_to_long_term":
		logging.ContextDebug("Memory op: promote_to_long_term key=%s", op.Key)
		// Store in cold storage
		if c.store != nil {
			c.store.StoreFact(op.Key, []interface{}{op.Value}, "preference", 10)
		}
	case "forget":
		logging.ContextDebug("Memory op: forget key=%s", op.Key)
		// Remove from kernel
		c.kernel.Retract(op.Key)
	case "store_vector":
		logging.ContextDebug("Memory op: store_vector key=%s", op.Key)
		// Store in vector memory
		if c.store != nil {
			c.store.StoreVector(op.Value, map[string]interface{}{"key": op.Key})
		}
	}
}

// shouldCompress returns true if compression should be triggered.
// Compression is purely token-budget driven - we only compress when
// approaching the context window limit, not based on arbitrary turn counts.
func (c *Compressor) shouldCompress() bool {
	return c.budget.ShouldCompress()
}

// recalcBudget recomputes token usage across core facts, context atoms,
// history, and recent turns so compression thresholds work correctly.
func (c *Compressor) recalcBudget(turnNumber int, workingTokens int) {
	timer := logging.StartTimer(logging.CategoryContext, "RecalcBudget")
	defer timer.Stop()

	// Gather context components
	coreFacts := c.getCoreFacts()
	allFacts := c.kernel.GetAllFacts()

	var currentIntent *core.Fact
	if intents, _ := c.kernel.Query("user_intent"); len(intents) > 0 {
		currentIntent = &intents[len(intents)-1]
	}

	scoredFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)

	start := len(c.recentTurns) - c.config.RecentTurnWindow
	if start < 0 {
		start = 0
	}
	recent := c.recentTurns[start:]

	builder := NewContextBlockBuilder()
	compressedCtx := builder.Build(
		coreFacts,
		scoredFacts,
		c.rollingSummary.Text,
		recent,
		turnNumber,
	)

	usage := compressedCtx.TokenUsage

	// Reset and set budget usage
	c.budget.Reset()
	c.budget.used.core = usage.Core
	c.budget.used.atoms = usage.Atoms
	c.budget.used.history = usage.History
	c.budget.used.recent = usage.Recent
	c.budget.used.working = workingTokens

	logging.ContextDebug("Budget recalculated: core=%d, atoms=%d, history=%d, recent=%d, working=%d (total=%d)",
		usage.Core, usage.Atoms, usage.History, usage.Recent, workingTokens, c.budget.TotalUsed())
}

// compress performs the actual compression.
func (c *Compressor) compress(ctx context.Context) error {
	if len(c.recentTurns) <= c.config.RecentTurnWindow {
		logging.ContextDebug("Compression skipped: only %d turns (need > %d)", len(c.recentTurns), c.config.RecentTurnWindow)
		return nil // Nothing to compress
	}

	// Determine turns to compress (everything except recent window)
	cutoff := len(c.recentTurns) - c.config.RecentTurnWindow
	turnsToCompress := c.recentTurns[:cutoff]
	logging.Context("Compressing %d turns (keeping %d recent)", cutoff, c.config.RecentTurnWindow)

	keyAtoms := c.collectKeyAtoms(turnsToCompress, 64)
	logging.ContextDebug("Collected %d key atoms for compression segment", len(keyAtoms))

	// Create summary using LLM
	summaryTimer := logging.StartTimer(logging.CategoryContext, "GenerateSummary")
	summary, err := c.generateSummary(ctx, turnsToCompress)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("LLM summary failed, using simple summary: %v", err)
		// Fallback to simple summary
		summary = c.generateSimpleSummary(turnsToCompress)
	}
	summaryTimer.Stop()

	// Calculate metrics with original token estimates preserved per turn
	originalTokens := c.countOriginalTokens(turnsToCompress)
	if originalTokens == 0 {
		originalTokens = c.counter.CountTurns(turnsToCompress)
	}
	compressedTokens := c.counter.CountString(summary)

	logging.ContextDebug("Initial compression: %d -> %d tokens (ratio: %.1f:1)",
		originalTokens, compressedTokens, float64(originalTokens)/float64(max(compressedTokens, 1)))

	// Enforce target compression ratio by preferring structured atoms or trimming
	maxSummaryTokens := 0
	if c.config.TargetCompressionRatio > 0 {
		maxSummaryTokens = max(1, int(float64(originalTokens)/c.config.TargetCompressionRatio))
	}
	if maxSummaryTokens > 0 && compressedTokens > maxSummaryTokens {
		logging.ContextDebug("Summary exceeds budget (%d > %d), attempting atom serialization", compressedTokens, maxSummaryTokens)
		serializedAtoms := c.serializer.SerializeFacts(keyAtoms)
		atomTokens := c.counter.CountString(serializedAtoms)

		if atomTokens > 0 && atomTokens <= maxSummaryTokens {
			logging.ContextDebug("Using atom serialization (%d tokens)", atomTokens)
			summary = serializedAtoms
			compressedTokens = atomTokens
		} else {
			logging.ContextDebug("Trimming summary to %d tokens", maxSummaryTokens)
			summary = c.trimToTokens(summary, maxSummaryTokens)
			compressedTokens = c.counter.CountString(summary)
		}
	}

	ratio := float64(originalTokens) / float64(max(compressedTokens, 1))

	// Create segment
	segment := HistorySegment{
		ID:               fmt.Sprintf("seg_%d_%d", turnsToCompress[0].TurnNumber, turnsToCompress[len(turnsToCompress)-1].TurnNumber),
		StartTurn:        turnsToCompress[0].TurnNumber,
		EndTurn:          turnsToCompress[len(turnsToCompress)-1].TurnNumber,
		Summary:          summary,
		KeyAtoms:         keyAtoms,
		OriginalTokens:   originalTokens,
		CompressedTokens: compressedTokens,
		CompressionRatio: ratio,
		CompressedAt:     time.Now(),
	}

	// Update rolling summary
	c.rollingSummary.Segments = append(c.rollingSummary.Segments, segment)
	c.rollingSummary.TotalTurns += len(turnsToCompress)
	c.rollingSummary.TotalOriginalTokens += originalTokens
	c.rollingSummary.TotalCompressedTokens += compressedTokens
	c.rollingSummary.OverallRatio = float64(c.rollingSummary.TotalOriginalTokens) / float64(max(c.rollingSummary.TotalCompressedTokens, 1))
	c.rollingSummary.LastUpdate = time.Now()

	logging.Context("COMPRESSION COMPLETE: turns %d-%d, %d->%d tokens (%.1f:1 ratio), %d segments total",
		segment.StartTurn, segment.EndTurn, originalTokens, compressedTokens, ratio, len(c.rollingSummary.Segments))
	logging.Context("Rolling summary: %d turns compressed, overall ratio %.1f:1",
		c.rollingSummary.TotalTurns, c.rollingSummary.OverallRatio)

	// Rebuild rolling summary text
	c.rebuildRollingSummaryText()

	// Remove compressed turns from recent
	c.recentTurns = c.recentTurns[cutoff:]
	logging.ContextDebug("Removed %d compressed turns, %d remaining", cutoff, len(c.recentTurns))

	// Decay recency scores for old facts
	c.activation.DecayRecency(30 * time.Minute)

	return nil
}

// generateSummary uses LLM to create a compressed summary.
func (c *Compressor) generateSummary(ctx context.Context, turns []CompressedTurn) (string, error) {
	if c.llmClient == nil {
		logging.ContextDebug("No LLM client, using simple summary")
		return c.generateSimpleSummary(turns), nil
	}

	// Build prompt
	var sb strings.Builder
	sb.WriteString("Summarize these conversation turns concisely (max 100 words). Focus on:\n")
	sb.WriteString("1. User intents and goals\n")
	sb.WriteString("2. Actions taken\n")
	sb.WriteString("3. Results and state changes\n\n")

	for _, turn := range turns {
		sb.WriteString(fmt.Sprintf("Turn %d (%s):\n", turn.TurnNumber, turn.Role))
		if turn.IntentAtom != nil {
			sb.WriteString(fmt.Sprintf("  Intent: %s\n", turn.IntentAtom.String()))
		}
		for _, atom := range turn.ResultAtoms {
			sb.WriteString(fmt.Sprintf("  Result: %s\n", atom.String()))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nSummary:")

	logging.ContextDebug("Generating LLM summary for %d turns", len(turns))

	// Set system context for trace attribution (routes through shard infrastructure)
	sysCtx := perception.NewSystemLLMContext(c.llmClient, "compressor", "context-compression")
	defer sysCtx.Clear()

	resp, err := sysCtx.Complete(ctx, sb.String())
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp), nil
}

// generateSimpleSummary creates a basic summary without LLM.
func (c *Compressor) generateSimpleSummary(turns []CompressedTurn) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Compressed History (Turns %d-%d)\n", turns[0].TurnNumber, turns[len(turns)-1].TurnNumber))

	for _, turn := range turns {
		if turn.IntentAtom != nil {
			sb.WriteString(turn.IntentAtom.String())
			sb.WriteString("\n")
		}
		for _, atom := range turn.ResultAtoms[:min(3, len(turn.ResultAtoms))] {
			sb.WriteString(atom.String())
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// rebuildRollingSummaryText rebuilds the combined summary text.
func (c *Compressor) rebuildRollingSummaryText() {
	var sb strings.Builder
	sb.WriteString("# Conversation History (Compressed)\n")
	sb.WriteString(fmt.Sprintf("# Total turns: %d | Compression ratio: %.1f:1\n\n", c.rollingSummary.TotalTurns, c.rollingSummary.OverallRatio))

	for _, seg := range c.rollingSummary.Segments {
		sb.WriteString(fmt.Sprintf("## Turns %d-%d\n", seg.StartTurn, seg.EndTurn))
		sb.WriteString(seg.Summary)
		sb.WriteString("\n")
		if len(seg.KeyAtoms) > 0 {
			sb.WriteString("# Key Atoms\n")
			sb.WriteString(c.serializer.SerializeFacts(seg.KeyAtoms))
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	c.rollingSummary.Text = sb.String()
}

// pruneRecentTurns keeps only the most recent turns within the window.
func (c *Compressor) pruneRecentTurns() {
	maxTurns := c.config.RecentTurnWindow * 2 // Keep 2x window before compression
	if len(c.recentTurns) > maxTurns {
		c.recentTurns = c.recentTurns[len(c.recentTurns)-maxTurns:]
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
	allFacts := c.kernel.GetAllFacts()
	logging.ContextDebug("Building context: %d total facts in kernel", len(allFacts))

	// 2. Find current intent (for activation scoring)
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
		logging.ContextDebug("Current intent: %s", currentIntent.String())
	}

	// 3. Score and filter facts using activation engine
	activationTimer := logging.StartTimer(logging.CategoryContext, "ActivationScoring")
	scoredFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)
	activationTimer.Stop()
	logging.ContextDebug("Activation scoring: %d facts selected (budget: %d tokens)", len(scoredFacts), c.config.AtomReserve)

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

	// Always include permission-related facts
	predicates := []string{"permitted", "dangerous_action", "admin_override", "security_violation", "block_commit"}

	for _, pred := range predicates {
		facts, err := c.kernel.Query(pred)
		if err == nil {
			coreFacts = append(coreFacts, facts...)
		}
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

// =============================================================================
// Metrics & State
// =============================================================================

// GetMetrics returns compression metrics.
func (c *Compressor) GetMetrics() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ratio := 1.0
	if c.totalCompressedTokens > 0 {
		ratio = float64(c.totalOriginalTokens) / float64(c.totalCompressedTokens)
	}

	return map[string]interface{}{
		"turn_number":             c.turnNumber,
		"recent_turns":            len(c.recentTurns),
		"compressed_segments":     len(c.rollingSummary.Segments),
		"total_compressed_turns":  c.rollingSummary.TotalTurns,
		"total_original_tokens":   c.totalOriginalTokens,
		"total_compressed_tokens": c.totalCompressedTokens,
		"compression_ratio":       ratio,
		"target_ratio":            c.config.TargetCompressionRatio,
	}
}

// GetCompressionRatio returns the current compression ratio.
func (c *Compressor) GetCompressionRatio() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.totalCompressedTokens == 0 {
		return 1.0
	}
	return float64(c.totalOriginalTokens) / float64(c.totalCompressedTokens)
}

// GetBudgetUtilization returns the fraction of the configured context budget used.
// Safe for UI display; returns 0 when budget is unavailable.
func (c *Compressor) GetBudgetUtilization() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.budget == nil || c.config.TotalBudget == 0 {
		return 0
	}
	return c.budget.Utilization()
}

// GetBudgetUsage returns (used, total) token counts for the context window.
// This is approximate and based on the internal token counter heuristic.
func (c *Compressor) GetBudgetUsage() (int, int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.budget == nil {
		return 0, c.config.TotalBudget
	}
	return c.budget.TotalUsed(), c.config.TotalBudget
}

// IsCompressionActive returns true if callers should use compressed context
// instead of raw conversation history. This is token-budget driven:
// - Returns false when we have room for raw history (use full context)
// - Returns true when approaching token limit (switch to compressed)
func (c *Compressor) IsCompressionActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// If we have compressed segments, always use compressed context
	if len(c.rollingSummary.Segments) > 0 {
		logging.ContextDebug("Compression active: %d segments exist", len(c.rollingSummary.Segments))
		return true
	}

	// If approaching token limit, signal that we should use compressed context
	// This prevents the "dump 50 messages" problem on rehydrated sessions
	shouldCompress := c.budget.ShouldCompress()
	if shouldCompress {
		logging.ContextDebug("Compression active: budget threshold reached (%.1f%%)", c.budget.Utilization()*100)
	}
	return shouldCompress
}

// GetRecentTurnWindow returns the configured recent turn window size.
func (c *Compressor) GetRecentTurnWindow() int {
	return c.config.RecentTurnWindow
}

// buildStateLocked constructs a CompressedState assuming c.mu is already held.
func (c *Compressor) buildStateLocked() *CompressedState {
	// Get hot facts
	allFacts := c.kernel.GetAllFacts()
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
	}
	hotFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)

	ratio := 1.0
	if c.totalCompressedTokens > 0 {
		ratio = float64(c.totalOriginalTokens) / float64(c.totalCompressedTokens)
	}

	return &CompressedState{
		SessionID:            c.sessionID,
		Version:              "1.0.0",
		TurnNumber:           c.turnNumber,
		Timestamp:            time.Now(),
		RollingSummary:       c.rollingSummary,
		RecentTurns:          c.recentTurns,
		HotFacts:             hotFacts,
		TotalCompressedTurns: c.rollingSummary.TotalTurns,
		CompressionRatio:     ratio,
	}
}

// GetState returns the full compressed state for persistence.
func (c *Compressor) GetState() *CompressedState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	logging.ContextDebug("Getting compressed state for persistence")
	state := c.buildStateLocked()
	logging.ContextDebug("State: turn=%d, recent=%d, segments=%d, hot_facts=%d",
		state.TurnNumber, len(state.RecentTurns), len(state.RollingSummary.Segments), len(state.HotFacts))
	return state
}

// LoadState restores state from a persisted CompressedState.
func (c *Compressor) LoadState(state *CompressedState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	logging.Context("Loading compressed state: session=%s, turn=%d, segments=%d",
		state.SessionID, state.TurnNumber, len(state.RollingSummary.Segments))

	c.sessionID = state.SessionID
	c.turnNumber = state.TurnNumber
	c.rollingSummary = state.RollingSummary
	c.recentTurns = state.RecentTurns

	// Restore hot facts to kernel
	restoredCount := 0
	if c.kernel != nil {
		existing := make(map[string]struct{})
		for _, f := range c.kernel.GetAllFacts() {
			existing[f.String()] = struct{}{}
		}
		missing := make([]core.Fact, 0, len(state.HotFacts))
		for _, sf := range state.HotFacts {
			key := sf.Fact.String()
			if _, ok := existing[key]; ok {
				continue
			}
			existing[key] = struct{}{}
			missing = append(missing, sf.Fact)
		}
		if len(missing) > 0 {
			// Best-effort batch restore; fallback to per-assert.
			if err := c.kernel.AssertBatch(missing); err != nil {
				for _, f := range missing {
					_ = c.kernel.Assert(f)
				}
			}
			for _, f := range missing {
				c.activation.RecordFactTimestamp(f)
			}
			restoredCount = len(missing)
		}
	}

	logging.Context("State loaded: restored %d hot facts, %d recent turns", restoredCount, len(c.recentTurns))
	return nil
}

// Reset clears all compression state.
func (c *Compressor) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	logging.Context("Resetting compressor state")

	c.turnNumber = 0
	c.recentTurns = nil
	c.rollingSummary = RollingSummary{}
	c.totalOriginalTokens = 0
	c.totalCompressedTokens = 0
	c.activation.ClearState()
	c.budget.Reset()
	c.sessionID = fmt.Sprintf("session_%d", time.Now().Unix())

	logging.Context("Compressor reset complete, new session: %s", c.sessionID)
}

// countOriginalTokens sums the pre-compression token estimates for turns.
func (c *Compressor) countOriginalTokens(turns []CompressedTurn) int {
	total := 0
	for _, t := range turns {
		if t.OriginalTokens > 0 {
			total += t.OriginalTokens
			continue
		}
		total += c.counter.CountTurn(t)
	}
	return total
}

// collectKeyAtoms extracts a bounded set of high-signal atoms to persist with the summary.
func (c *Compressor) collectKeyAtoms(turns []CompressedTurn, limit int) []core.Fact {
	seen := make(map[string]bool)
	var atoms []core.Fact

	add := func(f core.Fact) {
		if len(atoms) >= limit {
			return
		}
		key := f.String()
		if seen[key] {
			return
		}
		seen[key] = true
		atoms = append(atoms, f)
	}

	for _, turn := range turns {
		if turn.IntentAtom != nil {
			add(*turn.IntentAtom)
		}
		for _, f := range turn.FocusAtoms {
			add(f)
		}
		for i, f := range turn.ResultAtoms {
			if i >= 5 { // keep summaries small; first few results capture state
				break
			}
			add(f)
		}
		for _, f := range turn.ActionAtoms {
			add(f)
		}
	}

	return atoms
}

// trimToTokens truncates a string to fit within the approximate token budget.
func (c *Compressor) trimToTokens(s string, maxTokens int) string {
	if maxTokens <= 0 || c.counter.CountString(s) <= maxTokens {
		return strings.TrimSpace(s)
	}

	runes := []rune(s)
	low, high := 0, len(runes)
	for low < high {
		mid := (low + high) / 2
		if c.counter.CountString(string(runes[:mid])) > maxTokens {
			high = mid - 1
		} else {
			low = mid + 1
		}
	}

	cut := max(1, high)
	return strings.TrimSpace(string(runes[:cut]))
}

// min returns the minimum of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LoadPrioritiesFromCorpus loads predicate priorities from the kernel's corpus.
// GAP-003 FIX: This enables activation engine to use corpus-defined priorities.
func (c *Compressor) LoadPrioritiesFromCorpus(corpus *core.PredicateCorpus) error {
	if c.activation == nil {
		return nil
	}
	return c.activation.LoadPrioritiesFromCorpus(corpus)
}

// GetActivationScores returns current activation scores for all facts.
// Used by JIT Prompt Compiler to boost atoms related to highly-activated facts.
// Returns a map of fact string representation → activation score (0.0-1.0).
func (c *Compressor) GetActivationScores() map[string]float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scores := make(map[string]float64)

	if c.kernel == nil {
		return scores
	}

	// Get all facts and their activation scores
	allFacts := c.kernel.GetAllFacts()
	if len(allFacts) == 0 {
		return scores
	}

	// Get current intent for context-aware scoring
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
	}

	// Score all facts using the activation engine
	scoredFacts := c.activation.ScoreFacts(allFacts, currentIntent)
	for _, sf := range scoredFacts {
		// Normalize score to 0.0-1.0 range (scores are typically 0-100)
		normalizedScore := sf.Score / 100.0
		if normalizedScore > 1.0 {
			normalizedScore = 1.0
		}
		scores[sf.Fact.String()] = normalizedScore
	}

	return scores
}

// GetHighActivationFactKeys returns fact keys with activation above threshold.
// Used by JIT compiler to find atoms related to "hot" facts.
func (c *Compressor) GetHighActivationFactKeys(threshold float64) []string {
	var keys []string
	scores := c.GetActivationScores()

	for key, score := range scores {
		if score >= threshold {
			keys = append(keys, key)
		}
	}

	return keys
}
