// Package main implements the codeNERD CLI commands.
// This file contains init, scan, and workspace setup commands.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
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
	return runInitWithLLMConfigurer(cmd, args, configureInitLLM)
}

// runInitWithLLMConfigurer keeps command tests hermetic while preserving the
// complete production initializer. The command entry point always uses
// configureInitLLM; tests may provide nil to prove the filesystem workflow
// without contacting the user's configured provider.
func runInitWithLLMConfigurer(cmd *cobra.Command, args []string, configureLLM func(*nerdinit.InitConfig, *nerdconfig.UserConfig)) error {
	ctx, stop := commandOperationContext(cmd)
	defer stop()

	// Resolve workspace
	cwd, err := resolveCommandWorkspace()
	if err != nil {
		return err
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
	appCfg := loadCampaignConfig(filepath.Join(cwd, ".nerd"))
	config.Context7APIKey = appCfg.GetContext7APIKey()

	// Set up the LLM client from .nerd/config.json (wrapped with the scheduler
	// for concurrency control).
	//
	// This used to be hardcoded to Z.AI: it read only --api-key or ZAI_API_KEY
	// and called perception.NewZAIClient, ignoring the configured provider,
	// model and keys entirely. With a stale ZAI_API_KEY in the ambient
	// environment, a cold start ran all ~200 of its knowledge-base and doc
	// analysis calls against an unconfigured provider — and since init reports
	// per-KB "quality" without checking whether the calls succeeded, a run
	// where nearly every call 429'd still printed a clean bill of health.
	//
	// Init prefers the WORKER client. Its LLM work is ~200 documentation-relevance
	// classifications ("is this doc strategically relevant?") issued one at a
	// time, which is bulk labelling, not reasoning. Measured on a cold start:
	// ~25s per call on a high-reasoning model, so the serialized loop needs ~82
	// minutes and cannot finish inside the 25-minute operation timeout. On the
	// cheap tier the same loop is the work the tier exists for.
	if configureLLM != nil {
		configureLLM(&config, appCfg)
	}

	// Run initialization
	initializer, err := nerdinit.NewInitializer(config)
	if err != nil {
		return fmt.Errorf("failed to create initializer: %w", err)
	}
	defer initializer.Close()

	result, err := initializer.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	if !result.Success {
		if len(result.Failures) > 0 {
			return fmt.Errorf("initialization completed with errors: %s", result.Failures[0])
		}
		return fmt.Errorf("initialization completed with errors")
	}

	return nil
}

func configureInitLLM(config *nerdinit.InitConfig, appCfg *nerdconfig.UserConfig) {
	if err := applyInitAPIKeyOverride(appCfg, apiKey); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: --api-key override ignored: %v\n", err)
	}
	if worker, werr := perception.NewWorkerClientFromUserConfig(appCfg); werr != nil {
		fmt.Fprintf(os.Stderr, "Warning: worker LLM init failed: %v (init uses the main client)\n", werr)
	} else if worker != nil {
		if warning := initAPIKeyWorkerWarning(appCfg, apiKey); warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
		config.LLMClient = core.NewScheduledLLMCall("init", worker)
		if appCfg.Worker != nil {
			config.LLMProvider = appCfg.Worker.Provider
			config.LLMModel = appCfg.Worker.Model
		}
	}
	if config.LLMClient == nil {
		if llmClient, err := newConfiguredLLMClient(appCfg, "init"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: no LLM client for init: %v\n", err)
			fmt.Fprintln(os.Stderr, "         Knowledge-base and documentation phases will be skipped.")
		} else {
			config.LLMClient = llmClient
			config.LLMProvider = appCfg.Provider
			if appCfg.Engine != "" && !strings.EqualFold(appCfg.Engine, "api") {
				config.LLMProvider = appCfg.Engine
			}
			config.LLMModel = appCfg.Model
		}
	}
}

func applyInitAPIKeyOverride(appCfg *nerdconfig.UserConfig, override string) error {
	if override == "" {
		return nil
	}
	provider, _ := appCfg.GetActiveProvider()
	return appCfg.SetAPIKeyForProvider(provider, override)
}

func initAPIKeyWorkerWarning(appCfg *nerdconfig.UserConfig, override string) string {
	if strings.TrimSpace(override) == "" || appCfg == nil || appCfg.Worker == nil {
		return ""
	}
	mainProvider, _ := appCfg.GetActiveProvider()
	workerProvider := strings.TrimSpace(appCfg.Worker.Provider)
	if workerProvider == "" || strings.EqualFold(mainProvider, workerProvider) {
		return ""
	}
	return fmt.Sprintf("Warning: --api-key applies to the main %s client; init is using the configured %s worker", mainProvider, workerProvider)
}

// runScan refreshes the codebase index
func runScan(cmd *cobra.Command, args []string) error {
	return runScanWithKernelFactory(cmd, args, func(workspace string) (scanKernel, error) {
		return core.NewRealKernelWithWorkspace(workspace)
	})
}

type scanKernel interface {
	LoadFacts([]core.Fact) error
	LoadFactsFromFile(string) error
}

func runScanWithKernelFactory(cmd *cobra.Command, args []string, newKernel func(string) (scanKernel, error)) error {
	// Resolve workspace
	cwd, err := resolveCommandWorkspace()
	if err != nil {
		return err
	}

	// Check if initialized
	if !nerdinit.IsInitialized(cwd) {
		return fmt.Errorf("project not initialized; run 'nerd init' first")
	}
	ctx, stop := commandOperationContext(cmd)
	defer stop()

	fmt.Println("🔍 Scanning codebase...")

	// Create scanner
	scanner := world.NewScanner()

	// Scan workspace
	facts, err := scanner.ScanWorkspaceCtx(ctx, cwd)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// This full kernel boot is a deliberate validation gate. The scanner's
	// durable DB cache is the preferred boot source, so it must never be
	// replaced until the exact fact set has evaluated successfully.
	kernel, err := newKernel(cwd)
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

	// Persist the validated snapshot to the preferred incremental boot source.
	dbPath := filepath.Join(cwd, ".nerd", "knowledge.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		return fmt.Errorf("open knowledge DB %s: %w", dbPath, err)
	}
	if err := world.PersistFastSnapshotToDB(db, facts); err != nil {
		_ = db.Close()
		return fmt.Errorf("persist world snapshot to knowledge DB: %w", err)
	}
	if err := db.Close(); err != nil {
		return fmt.Errorf("close knowledge DB: %w", err)
	}

	// Persist scan facts to .nerd/mangle/scan.mg for reloading on boot
	scanPath := filepath.Join(cwd, ".nerd", "mangle", "scan.mg")
	if writeErr := writeScanFacts(scanPath, facts); writeErr != nil {
		return fmt.Errorf("persist scan facts: %w", writeErr)
	}
	fmt.Printf("   Facts persisted: %s\n", scanPath)

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
		for _, lang := range sortedLanguageNames(langStats) {
			count := langStats[lang]
			fmt.Printf("     %-12s: %d\n", lang, count)
		}
	}

	return nil
}

func commandOperationContext(cmd *cobra.Command) (context.Context, func()) {
	parent := context.Background()
	if cmd != nil && cmd.Context() != nil {
		parent = cmd.Context()
	}
	signalCtx, stopSignals := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithTimeout(signalCtx, timeout)
	return ctx, func() {
		cancel()
		stopSignals()
	}
}

func resolveCommandWorkspace() (string, error) {
	resolved := workspace
	if resolved == "" {
		var err error
		resolved, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve workspace: %w", err)
		}
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", resolved, err)
	}
	return filepath.Clean(abs), nil
}

func sortedLanguageNames(stats map[string]int) []string {
	names := make([]string, 0, len(stats))
	for name := range stats {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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
