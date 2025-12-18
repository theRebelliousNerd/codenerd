package chat

import (
	"context"
	"runtime"
	"time"

	"codenerd/internal/core"

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

		// Cancel autopoiesis listener goroutine
		if m.autopoiesisCancel != nil {
			m.autopoiesisCancel()
			// Wait for listener to stop (with timeout)
			if m.autopoiesisListenerCh != nil {
				select {
				case <-m.autopoiesisListenerCh:
					// Listener stopped cleanly
				case <-time.After(2 * time.Second):
					// Timeout - listener may be stuck, proceed anyway
				}
			}
		}

		// Stop browser manager goroutine
		if m.browserCtxCancel != nil {
			m.browserCtxCancel()
		}
		if m.browserMgr != nil {
			// Give it a moment to stop gracefully
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = m.browserMgr.Shutdown(ctx)
		}

		// Stop campaign orchestrator if running
		if m.campaignOrch != nil {
			m.campaignOrch.Stop()
		}

		// Close status channel to unblock waitForStatus
		// Set to nil after close to prevent sends on closed channel
		if m.statusChan != nil {
			close(m.statusChan)
			m.statusChan = nil
		}

		// Close local database connection
		if m.localDB != nil {
			m.localDB.Close()
		}

		// Stop Mangle file watcher
		if m.mangleWatcher != nil {
			m.mangleWatcher.Stop()
		}

		// Stop all active shards
		if m.shardMgr != nil {
			m.shardMgr.StopAll()
		}
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

// waitForStatus listens for status updates
func (m Model) waitForStatus() tea.Cmd {
	return func() tea.Msg {
		return statusMsg(<-m.statusChan)
	}
}

// ReportStatus sends a non-blocking status update
func (m Model) ReportStatus(msg string) {
	if m.statusChan != nil {
		select {
		case m.statusChan <- msg:
		default:
			// Channel full, drop update to prevent blocking
		}
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

// RunInteractiveChat starts the interactive chat session
func RunInteractiveChat(cfg Config) error {
	model := InitChat(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
