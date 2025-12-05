package context

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
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
	return &Compressor{
		kernel:     kernel,
		store:      localStorage,
		llmClient:  llmClient,
		config:     cfg,
		activation: NewActivationEngine(cfg),
		budget:     NewTokenBudget(cfg),
		serializer: NewFactSerializer(),
		counter:    NewTokenCounter(),
		sessionID:  fmt.Sprintf("session_%d", time.Now().Unix()),
	}
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
	c.mu.Lock()
	defer c.mu.Unlock()

	result := &TurnResult{}

	// 1. Extract atoms from control packet
	var atoms []core.Fact
	if turn.ControlPacket != nil {
		extracted, err := ExtractAtomsFromControlPacket(turn.ControlPacket)
		if err != nil {
			// Log but continue - we can still compress without atoms
			fmt.Printf("[Compressor] Warning: failed to extract atoms: %v\n", err)
		}
		atoms = extracted
	}

	// Add any pre-extracted atoms
	atoms = append(atoms, turn.ExtractedAtoms...)

	// 2. Commit atoms to kernel
	for _, atom := range atoms {
		if err := c.kernel.Assert(atom); err != nil {
			fmt.Printf("[Compressor] Warning: failed to assert atom %s: %v\n", atom.Predicate, err)
		}
	}
	result.CommittedAtoms = atoms

	// Mark atoms as new for recency scoring
	c.activation.MarkNewFacts(atoms)

	// 3. Process memory operations
	if turn.ControlPacket != nil {
		for _, op := range turn.ControlPacket.MemoryOperations {
			c.processMemoryOperation(op)
		}
		result.MemoryOps = turn.ControlPacket.MemoryOperations
	}

	// 4. Create compressed turn (NO SURFACE TEXT)
	compressed := CompressedTurn{
		TurnNumber: turn.Number,
		Role:       turn.Role,
		Timestamp:  turn.Timestamp,
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
	originalTokens := c.counter.CountString(turn.SurfaceResponse) + c.counter.CountString(turn.UserInput)
	compressedTokens := c.counter.CountTurn(compressed)
	c.totalOriginalTokens += originalTokens
	c.totalCompressedTokens += compressedTokens

	if c.shouldCompress() {
		if err := c.compress(ctx); err != nil {
			fmt.Printf("[Compressor] Warning: compression failed: %v\n", err)
		}
		result.CompressionTriggered = true
	}

	// 7. Prune old turns from sliding window
	c.pruneRecentTurns()

	// 8. Update turn number
	c.turnNumber = turn.Number

	// 9. Calculate final token usage
	result.TokenUsage = c.budget.GetUsage()

	return result, nil
}

// processMemoryOperation handles a memory operation from the control packet.
func (c *Compressor) processMemoryOperation(op perception.MemoryOperation) {
	switch op.Op {
	case "promote_to_long_term":
		// Store in cold storage
		if c.store != nil {
			c.store.StoreFact(op.Key, []interface{}{op.Value}, "preference", 10)
		}
	case "forget":
		// Remove from kernel
		c.kernel.Retract(op.Key)
	case "store_vector":
		// Store in vector memory
		if c.store != nil {
			c.store.StoreVector(op.Value, map[string]interface{}{"key": op.Key})
		}
	}
}

// shouldCompress returns true if compression should be triggered.
func (c *Compressor) shouldCompress() bool {
	// Compress if we have more turns than the window allows
	if len(c.recentTurns) > c.config.RecentTurnWindow*2 {
		return true
	}

	// Compress based on token budget
	return c.budget.ShouldCompress()
}

// compress performs the actual compression.
func (c *Compressor) compress(ctx context.Context) error {
	if len(c.recentTurns) <= c.config.RecentTurnWindow {
		return nil // Nothing to compress
	}

	// Determine turns to compress (everything except recent window)
	cutoff := len(c.recentTurns) - c.config.RecentTurnWindow
	turnsToCompress := c.recentTurns[:cutoff]

	// Create summary using LLM
	summary, err := c.generateSummary(ctx, turnsToCompress)
	if err != nil {
		// Fallback to simple summary
		summary = c.generateSimpleSummary(turnsToCompress)
	}

	// Calculate metrics
	originalTokens := c.counter.CountTurns(turnsToCompress)
	compressedTokens := c.counter.CountString(summary)
	ratio := float64(originalTokens) / float64(max(compressedTokens, 1))

	// Create segment
	segment := HistorySegment{
		ID:               fmt.Sprintf("seg_%d_%d", turnsToCompress[0].TurnNumber, turnsToCompress[len(turnsToCompress)-1].TurnNumber),
		StartTurn:        turnsToCompress[0].TurnNumber,
		EndTurn:          turnsToCompress[len(turnsToCompress)-1].TurnNumber,
		Summary:          summary,
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

	// Rebuild rolling summary text
	c.rebuildRollingSummaryText()

	// Remove compressed turns from recent
	c.recentTurns = c.recentTurns[cutoff:]

	// Decay recency scores for old facts
	c.activation.DecayRecency(30 * time.Minute)

	return nil
}

// generateSummary uses LLM to create a compressed summary.
func (c *Compressor) generateSummary(ctx context.Context, turns []CompressedTurn) (string, error) {
	if c.llmClient == nil {
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

	resp, err := c.llmClient.Complete(ctx, sb.String())
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
		sb.WriteString("\n\n")
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
func (c *Compressor) BuildContext(ctx context.Context) (*CompressedContext, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 1. Get all facts from kernel
	allFacts := c.kernel.GetAllFacts()

	// 2. Find current intent (for activation scoring)
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
	}

	// 3. Score and filter facts using activation engine
	scoredFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)

	// 4. Get core facts (constitutional, always included)
	coreFacts := c.getCoreFacts()

	// 5. Build context using builder
	builder := NewContextBlockBuilder()
	compressedCtx := builder.Build(
		coreFacts,
		scoredFacts,
		c.rollingSummary.Text,
		c.recentTurns[max(0, len(c.recentTurns)-c.config.RecentTurnWindow):],
		c.turnNumber,
	)

	// 6. Update usage
	compressedCtx.TokenUsage.Available = c.config.TotalBudget - compressedCtx.TokenUsage.Total

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
		return "", err
	}

	return c.serializer.SerializeCompressedContext(compressedCtx), nil
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

// GetState returns the full compressed state for persistence.
func (c *Compressor) GetState() *CompressedState {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Get hot facts
	allFacts := c.kernel.GetAllFacts()
	var currentIntent *core.Fact
	intentFacts, _ := c.kernel.Query("user_intent")
	if len(intentFacts) > 0 {
		currentIntent = &intentFacts[len(intentFacts)-1]
	}
	hotFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)

	return &CompressedState{
		SessionID:            c.sessionID,
		Version:              "1.0.0",
		TurnNumber:           c.turnNumber,
		Timestamp:            time.Now(),
		RollingSummary:       c.rollingSummary,
		RecentTurns:          c.recentTurns,
		HotFacts:             hotFacts,
		TotalCompressedTurns: c.rollingSummary.TotalTurns,
		CompressionRatio:     c.GetCompressionRatio(),
	}
}

// LoadState restores state from a persisted CompressedState.
func (c *Compressor) LoadState(state *CompressedState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sessionID = state.SessionID
	c.turnNumber = state.TurnNumber
	c.rollingSummary = state.RollingSummary
	c.recentTurns = state.RecentTurns

	// Restore hot facts to kernel
	for _, sf := range state.HotFacts {
		c.kernel.Assert(sf.Fact)
		c.activation.RecordFactTimestamp(sf.Fact)
	}

	return nil
}

// Reset clears all compression state.
func (c *Compressor) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.turnNumber = 0
	c.recentTurns = nil
	c.rollingSummary = RollingSummary{}
	c.totalOriginalTokens = 0
	c.totalCompressedTokens = 0
	c.activation.ClearState()
	c.budget.Reset()
	c.sessionID = fmt.Sprintf("session_%d", time.Now().Unix())
}

// min returns the minimum of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
