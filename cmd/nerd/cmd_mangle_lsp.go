// Package main implements the codeNERD CLI commands.
// This file contains the mangle-lsp command for running the Mangle Language Server.
//
// Symbiogen Product Requirements Document (PRD)
//
// File: cmd/nerd/cmd_mangle_lsp.go
// Author: Claude (via claude/check-lsp-support-VqIOO branch)
// Date: 2025-12-25
//
// Recommended Model: Sonnet 4.5
//
// Overview:
// This file implements the `mangle-lsp` command for the codeNERD CLI.
// Its primary responsibility is to start the Language Server Protocol (LSP) server
// for Mangle (.mg) files, enabling IDE integration (VSCode, Neovim, etc.).
//
// Key Features & Business Value:
// - IDE Integration: Provides autocomplete, go-to-definition, find-references, diagnostics
// - Multi-Stakeholder: Serves both external editors AND internal shards
// - World Model Integration: LSP data flows into Mangle facts for spreading activation
//
// Architectural Context:
// - Component Type: CLI Command + World Model Integration
// - Deployment: Built into `nerd` binary, invoked by editors via LSP protocol
// - Dependencies: internal/world/lsp/manager.go, internal/mangle/lsp.go
//
// Dependencies & Dependents:
// - Dependencies: github.com/spf13/cobra, internal/world/lsp, internal/mangle
// - Is a Dependency for: External editors (VSCode, Neovim, etc.)
//
// Deployment & Operations:
// - Editor Configuration: Add to editor's LSP client config:
//   {
//     "command": "nerd",
//     "args": ["mangle-lsp"],
//     "filetypes": ["mangle"]
//   }
//
// Code Quality Mandate:
// All code in this file must be production-ready. This includes complete error
// handling, graceful shutdown, and stdio protocol compliance.
//
// Functions:
// - runMangleLSP(): Initializes LSP manager and starts stdio server
//
// Usage:
// nerd mangle-lsp
//
// The command reads JSON-RPC messages from stdin and writes responses to stdout.
// This is the standard LSP protocol for editor integration.
//
// References:
// - internal/world/lsp/manager.go - Multi-language LSP manager
// - internal/mangle/lsp.go - Mangle LSP server implementation
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"codenerd/internal/logging"
	"codenerd/internal/world/lsp"

	"github.com/spf13/cobra"
)

// =============================================================================
// MANGLE-LSP COMMAND - Language Server Protocol for Mangle Files
// =============================================================================

var mangleLSPCmd = &cobra.Command{
	Use:   "mangle-lsp",
	Short: "Start Mangle Language Server (for IDE integration)",
	Long: `Starts the Language Server Protocol (LSP) server for Mangle (.mg) files.

This command is meant to be invoked by IDEs/editors (VSCode, Neovim, etc.)
for features like:
- Autocomplete
- Go to definition
- Find references
- Hover documentation
- Real-time diagnostics

Editor Configuration Example (VSCode settings.json):
{
  "mangle": {
    "server": {
      "command": "nerd",
      "args": ["mangle-lsp"]
    }
  }
}

The server communicates via JSON-RPC over stdin/stdout following the LSP specification.
`,
	RunE: runMangleLSP,
}

func init() {
	// Add workspace flag
	mangleLSPCmd.Flags().StringP("workspace", "w", ".", "Workspace root directory to index")
}

func runMangleLSP(cmd *cobra.Command, args []string) error {
	// Get workspace root
	workspace, err := cmd.Flags().GetString("workspace")
	if err != nil {
		return fmt.Errorf("failed to get workspace flag: %w", err)
	}

	// Resolve to absolute path
	if workspace != "." {
		absPath, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		workspace = absPath
	}

	logging.World("Starting Mangle LSP server for workspace: %s", workspace)

	// Create LSP manager
	manager := lsp.NewManager(workspace)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		logging.World("Received shutdown signal, stopping LSP server")
		cancel()
	}()

	// Initialize LSP manager (indexes workspace)
	if err := manager.Initialize(ctx); err != nil {
		logging.Get(logging.CategoryWorld).Error("Failed to initialize LSP manager: %v", err)
		return fmt.Errorf("LSP initialization failed: %w", err)
	}

	// Start stdio server for editor communication
	logging.World("LSP server ready, listening on stdin/stdout")
	if err := manager.ServeStdio(ctx); err != nil {
		if err == context.Canceled {
			logging.World("LSP server stopped gracefully")
			return nil
		}
		logging.Get(logging.CategoryWorld).Error("LSP server error: %v", err)
		return fmt.Errorf("LSP server error: %w", err)
	}

	return nil
}
