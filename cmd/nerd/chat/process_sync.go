// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains background workspace sync functionality.
package chat

import (
	"context"
	"time"

	"codenerd/internal/world"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// BACKGROUND SYNC
// =============================================================================

// checkWorkspaceSync performs a background scan of the workspace on startup.
// It ensures that even with a "Warm Start" from cache, the system eventually
// synchronizes with the actual file system state.
func (m Model) checkWorkspaceSync() tea.Cmd {
	return func() tea.Msg {
		if m.scanner == nil {
			return nil
		}

		start := time.Now()
		res, err := m.scanner.ScanWorkspaceIncremental(context.Background(), m.workspace, m.localDB, world.IncrementalOptions{SkipWhenUnchanged: true})
		if err != nil {
			return scanCompleteMsg{err: err}
		}
		if res == nil || res.Unchanged {
			return nil
		}

		if m.kernel != nil {
			_ = world.ApplyIncrementalResult(m.kernel, res)
		}
		if m.virtualStore != nil && len(res.NewFacts) > 0 {
			_ = m.virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 5)
		}

		return scanCompleteMsg{
			fileCount:      res.FileCount,
			directoryCount: res.DirectoryCount,
			factCount:      len(res.NewFacts),
			duration:       time.Since(start),
			err:            nil,
		}
	}
}
