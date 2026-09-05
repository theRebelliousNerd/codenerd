package chat

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/transparency"

	textarea "github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// Shutdown gracefully stops all background goroutines and releases resources.
// Safe to call multiple times - only executes once.
// MUST be called before tea.Quit to prevent goroutine leaks.
func (m *Model) Shutdown() {
	m.shutdownOnce.Do(func() {
		// Cancel all background operations via root context
		if m.shutdownCancel != nil {
			m.shutdownCancel()
		}

		// Stop browser manager goroutine (the Cortex shuts the manager itself down)
		if m.browserCtxCancel != nil {
			m.browserCtxCancel()
		}

		// Stop campaign orchestrator if running
		if m.campaignOrch != nil {
			m.campaignOrch.Stop()
		}

		// Wait for all background goroutines with timeout
		if m.goroutineWg != nil {
			done := make(chan struct{})
			go func() {
				m.goroutineWg.Wait()
				close(done)
			}()

			select {
			case <-done:
				// All goroutines finished cleanly
			case <-time.After(5 * time.Second):
				// Timeout waiting for goroutines - some may be stuck
				// Proceeding anyway to prevent hanging shutdown
				fmt.Println("[Shutdown] Warning: Some background goroutines did not finish within timeout")
			}
		}

		// NOTE: statusChan is intentionally NOT closed here. Bubble Tea models
		// are passed by value, so copies of this Model may still hold a
		// reference to statusChan and call ReportStatus(); closing the channel
		// would panic those sends. waitForStatus and ReportStatus both observe
		// shutdownCtx (cancelled above) to exit cleanly without a close.

		// Stop Background Observer Manager
		if m.observerMgr != nil {
			m.observerMgr.Stop()
		}

		// Stop Mangle file watcher
		if m.mangleWatcher != nil {
			m.mangleWatcher.Stop()
		}

		// The Cortex owns the usage tracker (its Close flushes .nerd/usage.json),
		// the shard manager, the ToolStore, the Ouroboros listener, the
		// browser manager and the local DB; one Close covers all of them.
		// Only the legacy boot path leaves it nil, in which case the shard
		// manager is the one thing left to stop here.
		if m.cortex != nil {
			if err := m.cortex.Close(); err != nil {
				fmt.Printf("[Shutdown] Warning: cortex close: %v\n", err)
			}
		} else if m.shardMgr != nil {
			m.shardMgr.StopAll()
		}

		// Bug #17: stop the shared taxonomy consolidation worker.
		// SharedTaxonomy is a package-level singleton started in init();
		// without this call the worker goroutine leaks past chat shutdown.
		// StopWorker is nil-guarded and idempotent (sync.Once), so multiple
		// callers (e.g. chat + tests) can invoke it safely.
		perception.ShutdownSharedTaxonomy()
	})
}

// IsKernelReady returns true if the kernel is initialized and ready for queries.
// Use this guard before any kernel operations in commands.
func (m *Model) IsKernelReady() bool {
	return m.kernel != nil && !m.isBooting
}

// performShutdown is a value-receiver wrapper for Shutdown() that can be called
// from Update(). It uses a local copy to call the pointer method.
func (m Model) performShutdown() {
	// Create a temporary pointer to call Shutdown
	// This is safe because Shutdown uses sync.Once internally
	modelPtr := &m
	modelPtr.Shutdown()
}

// waitForStatus listens for status updates.
// Returns nil (terminating the cmd) when shutdown is signaled, so the goroutine
// does not leak after the program exits. The statusChan is never closed —
// model copies may still hold references to it.
func (m Model) waitForStatus() tea.Cmd {
	return func() tea.Msg {
		if m.statusChan == nil {
			return nil
		}
		select {
		case s, ok := <-m.statusChan:
			if !ok {
				return nil
			}
			return statusMsg(s)
		case <-m.shutdownCtx.Done():
			return nil
		}
	}
}

// ReportStatus sends a non-blocking status update.
// Safe to call on stale Model copies after Shutdown: shutdownCtx cancellation
// short-circuits the send, and the select is non-blocking via default.
// When Glass Box debug mode is on, the same status also streams into chat
// via the event bus so phase pings ("Perception: parsing...", "Spawning...")
// are visible in scrollback, not only the footer status line.
func (m Model) ReportStatus(msg string) {
	if m.statusChan == nil && (m.glassBoxEventBus == nil || !m.glassBoxEnabled) {
		return
	}
	// Fast-path: if shutdown has been signaled, drop the update.
	if m.shutdownCtx != nil {
		select {
		case <-m.shutdownCtx.Done():
			return
		default:
		}
	}
	defer func() {
		// Defensive: if a future change ever closes statusChan, swallow the
		// panic rather than crash the caller's goroutine.
		_ = recover()
	}()
	if m.statusChan != nil {
		select {
		case m.statusChan <- msg:
		default:
			// Channel full, drop update to prevent blocking
		}
	}
	// Dual-emit into Glass Box full stream so phase progress is chat-visible.
	if m.glassBoxEnabled && m.glassBoxEventBus != nil && strings.TrimSpace(msg) != "" {
		m.glassBoxEventBus.EmitImmediate(transparency.GlassBoxEvent{
			Timestamp: time.Now(),
			Category:  transparency.CategoryControl,
			Summary:   msg,
			Source:    "status",
			TurnID:    m.turnCount,
		})
	}
}

// tickMemory samples Go runtime memory usage for UI display.
// Runs periodically regardless of loading state so the footer stays fresh.
func (m Model) tickMemory() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return memUsageMsg{Alloc: ms.Alloc, Sys: ms.Sys}
	})
}

// Init initializes the interactive chat model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		// m.checkWorkspaceSync(), // DEFERRED until boot complete
		tea.EnableMouseCellMotion,
		tea.EnableBracketedPaste, // Allow multi-line paste without sending early
		m.waitForStatus(),        // Start status listener
		m.tickMemory(),           // Start memory sampler
		performSystemBoot(m.Config, m.DisableSystemShards, m.workspace), // Start heavy system initialization
	)
}

// fetchTrace queries the kernel for a Mangle derivation trace and returns a command
// that sends it to the logic pane.
func (m Model) fetchTrace(query string) tea.Cmd {
	return m.fetchTraceWithOptions(query, false)
}

// fetchTraceForWhy queries the kernel and returns results for display in chat.
// Used by the /why command to show explanations directly to the user.
func (m Model) fetchTraceForWhy(query string) tea.Cmd {
	return m.fetchTraceWithOptions(query, true)
}

// fetchTraceWithOptions is the internal implementation for trace fetching.
func (m Model) fetchTraceWithOptions(query string, showInChat bool) tea.Cmd {
	return func() tea.Msg {
		if m.kernel == nil {
			return nil
		}

		// Build list of queries to try
		var queries []string
		if query != "" {
			queries = []string{query}
		} else {
			// Fallback cascade - try predicates in order of usefulness
			queries = []string{
				"user_intent(?a, ?b, ?c, ?d, ?e)",
				"next_action(?a)",
				"file_topology(?a, ?b, ?c, ?d)",
				"context_atom(?a)",
				"activation(?a, ?b)",
			}
		}

		// Try each query until one returns results
		for _, q := range queries {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			trace, err := m.kernel.TraceQuery(ctx, q)
			cancel()

			if err == nil && trace != nil && len(trace.RootNodes) > 0 {
				return traceUpdateMsg{Trace: trace, ShowInChat: showInChat, QuerySource: query}
			}
		}

		// If nothing found, return trace for first query (shows "0 facts")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		trace, err := m.kernel.TraceQuery(ctx, queries[0])
		if err != nil {
			return nil
		}

		return traceUpdateMsg{Trace: trace, ShowInChat: showInChat, QuerySource: query}
	}
}

// storeShardResult saves shard execution results for follow-up queries.
// This enables conversational follow-ups like "show me more" or "what are the warnings?".
// Also maintains a sliding window history for cross-shard context (blackboard pattern).
func (m *Model) storeShardResult(shardType, task, result string, facts []core.Fact) {
	sr := &ShardResult{
		ShardType:  shardType,
		Task:       task,
		RawOutput:  result,
		Timestamp:  time.Now(),
		TurnNumber: m.turnCount,
		Findings:   extractFindings(result),
		Metrics:    extractMetrics(result),
		ExtraData:  make(map[string]any),
	}

	// Store facts for later reference
	if len(facts) > 0 {
		factStrings := make([]string, len(facts))
		for i, f := range facts {
			factStrings[i] = f.String()
		}
		sr.ExtraData["facts"] = factStrings
	}

	// Set as most recent result
	m.lastShardResult = sr

	// Add to history (sliding window of last 10 results)
	m.shardResultHistory = append(m.shardResultHistory, sr)
	const maxHistorySize = 10
	if len(m.shardResultHistory) > maxHistorySize {
		m.shardResultHistory = m.shardResultHistory[len(m.shardResultHistory)-maxHistorySize:]
	}
}

// storeAggregatedReviewResult persists a multi-shard review as a reviewer shard result.
// This keeps follow-up questions ("show more") wired to the aggregated findings.
func (m *Model) storeAggregatedReviewResult(review *AggregatedReview, rendered string) {
	if review == nil {
		return
	}

	findings := make([]map[string]any, 0, len(review.DeduplicatedList))
	for _, f := range review.DeduplicatedList {
		findings = append(findings, map[string]any{
			"file":           f.File,
			"line":           float64(f.Line),
			"severity":       f.Severity,
			"category":       f.Category,
			"message":        f.Message,
			"recommendation": f.Recommendation,
			"shard":          f.ShardSource,
		})
	}

	metrics := map[string]any{
		"total_findings": review.TotalFindings,
		"files_reviewed": len(review.Files),
		"participants":   strings.Join(review.Participants, ", "),
		"duration":       review.Duration.String(),
	}

	sr := &ShardResult{
		ShardType:  "reviewer",
		Task:       fmt.Sprintf("multi-shard review: %s", review.Target),
		RawOutput:  rendered,
		Timestamp:  time.Now(),
		TurnNumber: m.turnCount,
		Findings:   findings,
		Metrics:    metrics,
		ExtraData: map[string]any{
			"review_id":         review.ID,
			"holistic_insights": review.HolisticInsights,
			"incomplete_reason": review.IncompleteReason,
			"files":             review.Files,
		},
	}

	m.lastShardResult = sr
	m.shardResultHistory = append(m.shardResultHistory, sr)
	const maxHistorySize = 10
	if len(m.shardResultHistory) > maxHistorySize {
		m.shardResultHistory = m.shardResultHistory[len(m.shardResultHistory)-maxHistorySize:]
	}
}

// RunInteractiveChat starts the interactive chat session
func RunInteractiveChat(cfg Config) error {
	model := InitChat(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
