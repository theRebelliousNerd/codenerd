package lsp

import (
	"context"
	"fmt"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
)

// Manager hosts the in-process Mangle language server behind `nerd mangle-lsp`:
// it indexes the workspace's .mg files and serves an editor over stdio.
//
// It used to also keep external language servers and project their data into
// world facts (symbol_defined, code_diagnostic, ...). Nothing constructed that
// half and nothing read the facts, so it was deleted on 2026-09-25; external
// servers now ground the critic directly (internal/session/lsp_diagnostics.go).

// Manager hosts the Mangle language server.
type Manager struct {
	mu            sync.RWMutex
	mangleServer  *mangle.LSPServer
	mangleEngine  *mangle.Engine
	workspaceRoot string
	indexed       bool
}

// NewManager creates a new LSP manager.
func NewManager(workspaceRoot string) *Manager {
	logging.WorldDebug("Creating LSP Manager for workspace: %s", workspaceRoot)
	return &Manager{
		workspaceRoot: workspaceRoot,
		indexed:       false,
	}
}

// Initialize initializes the Mangle LSP server and indexes the workspace.
func (m *Manager) Initialize(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logging.World("Initializing LSP Manager")

	// Create Mangle engine for LSP server
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to create Mangle engine for LSP: %v", err)
		return fmt.Errorf("failed to create Mangle engine: %w", err)
	}
	m.mangleEngine = engine

	// Create Mangle LSP server
	m.mangleServer = mangle.NewLSPServer(engine)

	// Index workspace
	if err := m.mangleServer.IndexWorkspace(ctx, m.workspaceRoot); err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to index workspace for LSP: %v", err)
		return fmt.Errorf("failed to index workspace: %w", err)
	}

	m.indexed = true
	logging.World("LSP Manager initialized successfully")
	return nil
}

// ============================================================================
// Fact Projection - Convert LSP Data to World Model Facts
// ============================================================================

// ============================================================================
// Batch Query API for Shards
// ============================================================================

// ============================================================================
// Stdio Server for External Editors
// ============================================================================

// ServeStdio starts the LSP server on stdin/stdout for editor integration.
// This is the entry point for `nerd mangle-lsp` CLI command.
func (m *Manager) ServeStdio(ctx context.Context) error {
	m.mu.RLock()
	server := m.mangleServer
	m.mu.RUnlock()

	if server == nil {
		return fmt.Errorf("LSP server not initialized - call Initialize() first")
	}

	logging.World("Starting LSP stdio server for external editors")
	return server.ServeStdio(ctx)
}

// ============================================================================
// Utility Functions
// ============================================================================

// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
	// This should match the implementation in mangle.LSPServer
	// For now, simple implementation
	return "file://" + path
}
