// Package main implements the codeNERD CLI commands.
// This file contains init, scan, and workspace setup commands.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/store"
	"codenerd/internal/world"

	"github.com/spf13/cobra"
)

// =============================================================================
// INIT & SCAN COMMANDS - Workspace initialization and indexing
// =============================================================================

// initCmd initializes codeNERD in the current workspace
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize codeNERD in the current workspace",
	Long: `Performs the "Cold Start" initialization for a new project.

This command:
  1. Creates the .nerd/ directory structure
  2. Analyzes the codebase to detect language, framework, and architecture
  3. Builds a project profile for context-aware assistance
  4. Initializes the knowledge database
  5. Sets up user preferences

Run this once when starting to use codeNERD with a new project.`,
	RunE: runInit,
}

// scanCmd refreshes the codebase index without full reinitialization
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Refresh the codebase index",
	Long: `Scans the workspace and refreshes the Mangle kernel with fresh facts.

This is a lighter alternative to 'nerd init --force' that:
  1. Scans the file structure
  2. Extracts AST symbols and dependencies
  3. Updates the kernel with fresh file_topology facts
  4. Reloads profile.mg facts

Use this when files have changed and you want to update the kernel without
recreating agent knowledge bases.`,
	RunE: runScan,
}

// runInit performs the cold-start initialization
func runInit(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInitialization cancelled")
		cancel()
	}()

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Handle backup cleanup (can run standalone without full init)
	if cleanupBackups {
		nerdDir := filepath.Join(cwd, ".nerd")
		deleted, err := nerdinit.CleanupBackups(nerdDir, false)
		if err != nil {
			return fmt.Errorf("failed to cleanup backups: %w", err)
		}
		if deleted == 0 {
			fmt.Println("No backup files found to clean up.")
		}
		return nil
	}

	// Check if already initialized
	if nerdinit.IsInitialized(cwd) && !forceInit {
		fmt.Println("Project already initialized. Use 'nerd status' to view project info.")
		fmt.Println("To reinitialize, use 'nerd init --force' (preserves learned preferences).")
		return nil
	}

	if forceInit {
		fmt.Println("🔄 Force reinitializing workspace...")
	}

	// Configure initializer
	config := nerdinit.DefaultInitConfig(cwd)
	config.Timeout = timeout

	// Set up LLM client if available (wrapped with scheduler for concurrency control)
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}
	if key != "" {
		rawClient := perception.NewZAIClient(key)
		config.LLMClient = core.NewScheduledLLMCall("init", rawClient)
	}

	// Set Context7 API key from environment or config
	context7Key := os.Getenv("CONTEXT7_API_KEY")
	if context7Key == "" {
		// Try loading from config.json
		if providerCfg, err := perception.LoadConfigJSON(perception.DefaultConfigPath()); err == nil && providerCfg.Context7APIKey != "" {
			context7Key = providerCfg.Context7APIKey
		}
	}
	if context7Key != "" {
		config.Context7APIKey = context7Key
	}

	// Run initialization
	initializer, err := nerdinit.NewInitializer(config)
	if err != nil {
		return fmt.Errorf("failed to create initializer: %w", err)
	}
	result, err := initializer.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("initialization completed with errors")
	}

	return nil
}

// runScan refreshes the codebase index
func runScan(cmd *cobra.Command, args []string) error {
	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if initialized
	if !nerdinit.IsInitialized(cwd) {
		fmt.Println("Project not initialized. Run 'nerd init' first.")
		return nil
	}

	fmt.Println("🔍 Scanning codebase...")

	// Create scanner
	scanner := world.NewScanner()

	// Scan workspace
	facts, err := scanner.ScanWorkspace(cwd)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Persist fast world snapshot to knowledge.db for incremental boots.
	dbPath := filepath.Join(cwd, ".nerd", "knowledge.db")
	if db, dbErr := store.NewLocalStore(dbPath); dbErr == nil {
		if err := world.PersistFastSnapshotToDB(db, facts); err != nil {
			logging.WorldWarn("failed to persist world snapshot to DB: %v", err)
		}
		if err := db.Close(); err != nil {
			logging.StoreWarn("failed to close knowledge DB: %v", err)
		}
	}

	// Initialize kernel and load facts
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	if err := kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("failed to load facts: %w", err)
	}

	// Also reload profile.mg if it exists
	factsPath := filepath.Join(cwd, ".nerd", "profile.mg")
	if _, statErr := os.Stat(factsPath); statErr == nil {
		if err := kernel.LoadFactsFromFile(factsPath); err != nil {
			fmt.Printf("⚠️ Warning: failed to load profile.mg: %v\n", err)
		}
	}

	// Persist scan facts to .nerd/mangle/scan.mg for reloading on boot
	scanPath := filepath.Join(cwd, ".nerd", "mangle", "scan.mg")
	if writeErr := writeScanFacts(scanPath, facts); writeErr != nil {
		fmt.Printf("⚠️ Warning: failed to persist scan facts: %v\n", writeErr)
	} else {
		fmt.Printf("   Facts persisted: %s\n", scanPath)
	}

	// Count files and directories
	fileCount := 0
	dirCount := 0
	langStats := make(map[string]int)
	symbolCount := 0

	for _, f := range facts {
		switch f.Predicate {
		case "file_topology":
			fileCount++
			if len(f.Args) > 2 {
				// file_topology(Path, Hash, /Lang, ...)
				if langAtom, ok := f.Args[2].(core.MangleAtom); ok {
					lang := strings.TrimPrefix(string(langAtom), "/")
					langStats[lang]++
				}
			}
		case "directory":
			dirCount++
		case "symbol_graph":
			symbolCount++
		}
	}

	fmt.Println("✅ Scan complete")
	fmt.Printf("   Files indexed:    %d\n", fileCount)
	fmt.Printf("   Directories:      %d\n", dirCount)
	fmt.Printf("   Symbols extracted: %d\n", symbolCount)
	fmt.Printf("   Facts generated:  %d\n", len(facts))

	if len(langStats) > 0 {
		fmt.Println("\n   Language Breakdown:")
		for lang, count := range langStats {
			fmt.Printf("     %-12s: %d\n", lang, count)
		}
	}

	return nil
}

// writeScanFacts persists scan facts to a .mg file for reloading on boot.
func writeScanFacts(path string, facts []core.Fact) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Build content
	var sb strings.Builder
	sb.WriteString("# Auto-generated scan facts - DO NOT EDIT\n")
	sb.WriteString("# Re-run 'nerd scan' to update\n\n")

	for _, fact := range facts {
		// Sanitize fact args to remove characters that Mangle parser can't handle
		sanitizedFact := sanitizeFactForMangle(fact)
		sb.WriteString(sanitizedFact.String())
		sb.WriteString("\n")
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
