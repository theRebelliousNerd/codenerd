package context

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"strings"
	"time"
	"slices"
)

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
	segmentsBefore := len(c.rollingSummary.Segments)
	c.pruneRecentTurns(ctx)
	if len(c.rollingSummary.Segments) > segmentsBefore {
		result.CompressionTriggered = true
	}
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
			c.store.StoreFact(op.Key, []any{op.Value}, "preference", 10)
		}
	case "forget":
		logging.ContextDebug("Memory op: forget key=%s", op.Key)
		// Remove from kernel
		c.kernel.Retract(op.Key)
	case "store_vector":
		logging.ContextDebug("Memory op: store_vector key=%s", op.Key)
		// Store in vector memory
		if c.store != nil {
			c.store.StoreVector(op.Value, map[string]any{"key": op.Key})
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
	allFacts := slices.Collect(c.kernel.GetAllFactsSeq())

	var currentIntent *core.Fact
	// OPTIMIZATION: Use QueryAll for single predicate lookups too (more consistent)
	if allFacts, err := c.kernel.QueryAll(); err == nil {
		if intents, ok := allFacts["user_intent"]; ok && len(intents) > 0 {
			currentIntent = &intents[len(intents)-1]
		}
	}

	scoredFacts := c.activation.GetHighActivationFacts(allFacts, currentIntent, c.config.AtomReserve)

	start := max(len(c.recentTurns)-c.config.RecentTurnWindow, 0)
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

	// NERD-EVOLVE-START: c3_observation_mask
	// C3: Use observation masking instead of LLM summarization.
	// Assert turn age categories into kernel, then read the kernel's masking
	// decision back out and obey it. The kernel derives
	// should_mask_observation(TurnID) for /old and /ancient turns; masked turns
	// keep their reasoning atoms (intent/focus/action) and drop observations.
	c.assertTurnAgeCategories(turnsToCompress)
	masked := c.maskedObservationTurns()
	summaryTimer := logging.StartTimer(logging.CategoryContext, "GenerateObservationMaskedSummary")
	summary, maskedCount := c.generateObservationMaskedSummary(turnsToCompress, masked)
	summaryTimer.Stop()
	// NERD-EVOLVE-END: c3_observation_mask

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
		MaskedTurns:      maskedCount,
	}

	// Update rolling summary
	c.rollingSummary.Segments = append(c.rollingSummary.Segments, segment)
	c.rollingSummary.TotalTurns += len(turnsToCompress)
	c.rollingSummary.TotalMaskedTurns += maskedCount
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

// generateObservationMaskedSummary builds the segment summary under the
// kernel's C3 masking decision (masked = should_mask_observation(TurnID)).
//
// The split is: reasoning atoms (intent, focus, action) are ALWAYS emitted —
// that is the should_preserve_reasoning invariant — while observation atoms
// (results/outcomes) are emitted only for turns the kernel did not mask. With
// an empty mask set the output is byte-identical to generateSimpleSummary, so
// a kernel that derives nothing degrades to the old behaviour instead of
// silently losing history.
//
// Returns the summary text and how many turns were actually masked, so callers
// can record the kernel's influence instead of trusting a log line.
func (c *Compressor) generateObservationMaskedSummary(turns []CompressedTurn, masked map[string]bool) (string, int) {
	if len(turns) == 0 {
		return "", 0
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Compressed History (Turns %d-%d)\n", turns[0].TurnNumber, turns[len(turns)-1].TurnNumber))

	maskedCount := 0
	for _, turn := range turns {
		isMasked := masked[turnMaskID(turn.TurnNumber)]
		if isMasked {
			maskedCount++
		}

		// Reasoning chain: never dropped, masked or not.
		if turn.IntentAtom != nil {
			sb.WriteString(turn.IntentAtom.String())
			sb.WriteString("\n")
		}
		if isMasked {
			for _, atom := range turn.FocusAtoms {
				sb.WriteString(atom.String())
				sb.WriteString("\n")
			}
			for _, atom := range turn.ActionAtoms {
				sb.WriteString(atom.String())
				sb.WriteString("\n")
			}
			// Observations are the only thing the mask removes; record that the
			// kernel (not Go) made the call so the block stays auditable.
			sb.WriteString(fmt.Sprintf("# turn %d observations masked by should_mask_observation (%d atoms)\n",
				turn.TurnNumber, len(turn.ResultAtoms)))
			continue
		}
		for _, atom := range turn.ResultAtoms[:min(3, len(turn.ResultAtoms))] {
			sb.WriteString(atom.String())
			sb.WriteString("\n")
		}
	}

	if maskedCount > 0 {
		logging.Context("C3 observation masking applied to %d/%d compressed turns", maskedCount, len(turns))
	}
	return sb.String(), maskedCount
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

// rebuildRollingSummaryText rebuilds the combined summary text and keeps it
// inside HistoryReserve by recursively merging the oldest segments.
//
// The renderer concatenates every segment ever produced. Nothing bounded that:
// a 300-turn session produced 240 segments and a history block large enough to
// push total usage past 100% of the window, at which point BuildContext refuses
// with ErrContextWindowExceeded and the session gets *no* context at all. The
// fix is recursive compression, which is what a "rolling" summary always meant:
// when the block outgrows its reserve, the oldest segments fold into one.
func (c *Compressor) rebuildRollingSummaryText() {
	c.renderRollingSummaryText()

	if c.config.HistoryReserve <= 0 {
		return
	}
	for c.counter.CountString(c.rollingSummary.Text) > c.config.HistoryReserve && len(c.rollingSummary.Segments) > 1 {
		// Merge the oldest half each pass so this converges in O(log n)
		// renders instead of one render per merge.
		c.mergeOldestSegments(max(2, len(c.rollingSummary.Segments)/2))
		c.renderRollingSummaryText()
	}

	// Floor: a single segment cannot be merged with anything, so trim it in
	// place rather than letting one oversized segment defeat the reserve.
	if len(c.rollingSummary.Segments) != 1 {
		return
	}
	seg := &c.rollingSummary.Segments[0]
	for range 4 {
		total := c.counter.CountString(c.rollingSummary.Text)
		if total <= c.config.HistoryReserve {
			break
		}
		// The render frame (headers, serialized key atoms) costs tokens the
		// summary trim cannot see. Budget against the remainder instead of
		// trimming blind, and shed key atoms only when the frame alone
		// already exceeds the reserve.
		overhead := total - c.counter.CountString(seg.Summary)
		if overhead >= c.config.HistoryReserve && len(seg.KeyAtoms) > 0 {
			seg.KeyAtoms = nil
			c.renderRollingSummaryText()
			continue
		}
		// −2 absorbs the rounding slack in the chars-per-token estimate, which
		// is not additive across concatenation.
		seg.Summary = c.trimToTokens(seg.Summary, max(1, c.config.HistoryReserve-overhead-2))
		c.renderRollingSummaryText()
	}

	seg.CompressedTokens = c.counter.CountString(seg.Summary)
	seg.CompressionRatio = float64(seg.OriginalTokens) / float64(max(seg.CompressedTokens, 1))
	c.rollingSummary.TotalCompressedTokens = seg.CompressedTokens
	c.rollingSummary.TotalOriginalTokens = seg.OriginalTokens
	c.rollingSummary.OverallRatio = float64(seg.OriginalTokens) / float64(max(seg.CompressedTokens, 1))
}

// mergeOldestSegments folds the n oldest history segments into a single
// segment, re-trimming their combined summary. Turn coverage, masked-turn
// counts and original-token totals are preserved so the reported ratio stays
// truthful across merges.
func (c *Compressor) mergeOldestSegments(n int) {
	segs := c.rollingSummary.Segments
	if n < 2 || len(segs) < 2 {
		return
	}
	n = min(n, len(segs))
	head := segs[:n]

	var sb strings.Builder
	merged := HistorySegment{
		ID:           fmt.Sprintf("seg_%d_%d", head[0].StartTurn, head[n-1].EndTurn),
		StartTurn:    head[0].StartTurn,
		EndTurn:      head[n-1].EndTurn,
		CompressedAt: time.Now(),
	}
	seenAtoms := make(map[string]bool)
	for _, s := range head {
		sb.WriteString(s.Summary)
		sb.WriteString("\n")
		merged.OriginalTokens += s.OriginalTokens
		merged.MaskedTurns += s.MaskedTurns
		for _, a := range s.KeyAtoms {
			key := a.String()
			if seenAtoms[key] || len(merged.KeyAtoms) >= 64 {
				continue
			}
			seenAtoms[key] = true
			merged.KeyAtoms = append(merged.KeyAtoms, a)
		}
	}

	// Halve the merged summary: merging without re-trimming would not reclaim
	// any tokens and the loop above would never converge.
	budget := max(1, c.counter.CountString(sb.String())/2)
	merged.Summary = c.trimToTokens(sb.String(), budget)
	merged.CompressedTokens = c.counter.CountString(merged.Summary)
	merged.CompressionRatio = float64(merged.OriginalTokens) / float64(max(merged.CompressedTokens, 1))

	c.rollingSummary.Segments = append([]HistorySegment{merged}, segs[n:]...)

	// Recompute totals from the surviving segments; the cumulative counters
	// would otherwise describe segments that no longer exist.
	c.rollingSummary.TotalOriginalTokens = 0
	c.rollingSummary.TotalCompressedTokens = 0
	for _, s := range c.rollingSummary.Segments {
		c.rollingSummary.TotalOriginalTokens += s.OriginalTokens
		c.rollingSummary.TotalCompressedTokens += s.CompressedTokens
	}
	c.rollingSummary.OverallRatio = float64(c.rollingSummary.TotalOriginalTokens) / float64(max(c.rollingSummary.TotalCompressedTokens, 1))

	logging.ContextDebug("Merged %d oldest history segments (turns %d-%d), %d segments remain",
		n, merged.StartTurn, merged.EndTurn, len(c.rollingSummary.Segments))
}

// renderRollingSummaryText renders the current segments into the summary text.
func (c *Compressor) renderRollingSummaryText() {
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

// pruneRecentTurns bounds the sliding window, compressing the overflow instead
// of discarding it.
//
// It used to reslice the window and drop the oldest turns outright. Because
// recalcBudget only counts the last RecentTurnWindow turns, utilization never
// grew with session length, so the budget trigger did not fire on long
// sessions — and every turn beyond 2× the window was deleted without ever
// being folded into a segment. A 300-turn session produced zero segments and a
// 1.0:1 ratio: "infinite context" had quietly become "forget everything older
// than two windows". Budget pressure remains the primary trigger; window
// overflow is the safety net that guarantees turns leave only through
// compression.
func (c *Compressor) pruneRecentTurns(ctx context.Context) {
	maxTurns := c.config.RecentTurnWindow * 2 // Keep 2x window before compression
	if len(c.recentTurns) <= maxTurns {
		return
	}

	logging.ContextDebug("Sliding window overflow: %d turns > %d, compressing before prune", len(c.recentTurns), maxTurns)
	if err := c.compress(ctx); err != nil {
		logging.Get(logging.CategoryContext).Error("Overflow compression failed: %v", err)
	}

	// Last resort: if compression could not reduce the window (no kernel, or a
	// compress() error), bound memory anyway rather than growing without limit.
	if len(c.recentTurns) > maxTurns {
		dropped := len(c.recentTurns) - maxTurns
		logging.Get(logging.CategoryContext).Warn(
			"pruneRecentTurns: dropping %d uncompressed turns after failed compression", dropped)
		c.recentTurns = c.recentTurns[dropped:]
	}
}
