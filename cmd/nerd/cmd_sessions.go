// Package main implements session management CLI commands for codeNERD.
// This file handles session listing, loading, and management.
package main

import (
	"fmt"
	"os"
	"strings"

	nerdinit "codenerd/internal/init"

	"github.com/spf13/cobra"
)

// =============================================================================
// SESSION MANAGEMENT COMMANDS
// =============================================================================

// sessionsCmd manages CLI sessions
var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage codeNERD sessions",
	Long: `List and manage codeNERD sessions.

Subcommands:
  list   - List all saved sessions
  load   - Load a specific session`,
	RunE: runSessionsList,
}

// sessionsListCmd lists available sessions
var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved sessions",
	RunE:  runSessionsList,
}

// sessionsLoadCmd loads a specific session
var sessionsLoadCmd = &cobra.Command{
	Use:   "load <session-id>",
	Short: "Load a specific session",
	Args:  cobra.ExactArgs(1),
	RunE:  runSessionsLoad,
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}

	sessions, err := nerdinit.ListSessionHistories(ws)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions found.")
		return nil
	}

	fmt.Println("📁 Saved Sessions")
	fmt.Println(strings.Repeat("─", 50))
	for i, s := range sessions {
		fmt.Printf("  %d. %s\n", i+1, s)
	}
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total: %d sessions\n", len(sessions))
	fmt.Println("\nUse: nerd sessions load <session-id>")

	return nil
}

func runSessionsLoad(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}

	sessions, err := nerdinit.ListSessionHistories(ws)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	found := false
	for _, s := range sessions {
		if s == sessionID {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("session '%s' not found. Use 'nerd sessions list' to see available sessions", sessionID)
	}

	fmt.Printf("✅ Session '%s' exists.\n", sessionID)
	fmt.Println("To load this session, start the TUI with:")
	fmt.Printf("  nerd --load-session %s\n", sessionID)

	return nil
}

func init() {
	sessionsCmd.AddCommand(sessionsListCmd, sessionsLoadCmd)
}
