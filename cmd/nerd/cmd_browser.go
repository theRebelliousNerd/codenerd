// Package main implements the codeNERD CLI commands.
// This file contains browser automation commands.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/browser"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// =============================================================================
// BROWSER COMMANDS - Browser automation (§9.0 Browser Physics)
// =============================================================================

// browserCmd manages browser sessions (§9.0 Browser Physics)
var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Browser automation commands (DOM snapshotting, React reification)",
	RunE:  parentGroupRunE,
}

var browserLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch the browser instance",
	RunE:  browserLaunch,
}

var browserSessionCmd = &cobra.Command{
	Use:   "session [url]",
	Short: "Create a new browser session",
	Args:  cobra.ExactArgs(1),
	RunE:  browserSession,
}

var browserSnapshotCmd = &cobra.Command{
	Use:   "snapshot [session-id]",
	Short: "Snapshot DOM as Mangle facts",
	Args:  cobra.ExactArgs(1),
	RunE:  browserSnapshot,
}

// getBrowserConfig returns browser config with persistent session store
func getBrowserConfig() browser.Config {
	cwd, _ := os.Getwd()
	cfg := browser.DefaultConfig()
	cfg.SessionStore = filepath.Join(cwd, ".nerd", "browser", "sessions.json")
	return cfg
}

// loadBrowserSchema loads the embedded browser DOM schema into the engine.
// It must be called after creating the engine and before constructing the
// SessionManager, because DOM reification asserts predicates declared in
// schemas_browser.mg (element/3, position/5, etc.).
func loadBrowserSchema(engine *mangle.Engine) error {
	schema, err := core.GetDefaultContent("schemas_browser.mg")
	if err != nil {
		return fmt.Errorf("failed to read browser schema: %w", err)
	}
	if err := engine.LoadSchemaString(schema); err != nil {
		return fmt.Errorf("failed to load browser schema: %w", err)
	}
	return nil
}

// startBrowserManagerWithFallback tries to start the browser session manager
// treating control.txt as a hint. If cfg.DebuggerURL is set (from controlFile)
// and Start fails, the stale file is removed, a clear log is emitted, and a
// single retry is made with an empty DebuggerURL so it launches a fresh browser
// exactly as if the file had been absent. The updated cfg (DebuggerURL cleared
// on fallback) and the manager to use are returned. On second failure that error
// is returned.
func startBrowserManagerWithFallback(ctx context.Context, cfg browser.Config, engine *mangle.Engine, controlFile string) (*browser.SessionManager, browser.Config, error) {
	mgr := browser.NewSessionManager(cfg, engine)
	if err := mgr.Start(ctx); err == nil {
		return mgr, cfg, nil
	} else if cfg.DebuggerURL == "" {
		// No stored URL to fall back from — return original error.
		return nil, cfg, err
	} else {
		origErr := err
		logger.Warn("Stored browser control URL was stale, removing control file and retrying with fresh browser", zap.String("controlFile", controlFile), zap.String("storedURL", cfg.DebuggerURL), zap.Error(origErr))
		if rmErr := os.Remove(controlFile); rmErr != nil && !os.IsNotExist(rmErr) {
			logger.Warn("Failed to remove stale browser control file", zap.String("controlFile", controlFile), zap.Error(rmErr))
		} else {
			logger.Info("Removed stale browser control file", zap.String("controlFile", controlFile))
		}
		cfg.DebuggerURL = ""
		retryMgr := browser.NewSessionManager(cfg, engine)
		if err2 := retryMgr.Start(ctx); err2 != nil {
			return nil, cfg, err2
		}
		logger.Info("Launched fresh browser after stale control URL fallback", zap.String("controlFile", controlFile))
		return retryMgr, cfg, nil
	}
}

// browserLaunch launches the browser instance
func browserLaunch(cmd *cobra.Command, args []string) error {
	logger.Info("Launching browser")

	// Initialize browser session manager with persistent store
	cfg := getBrowserConfig()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	mgr := browser.NewSessionManager(cfg, engine)

	// Start the session manager (loads persisted sessions)
	if err := mgr.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}

	// Write control URL to file for other commands to use
	cwd, err := os.Getwd()
	if err != nil {
		logging.BootWarn("failed to get working directory: %v", err)
	}
	controlFile := filepath.Join(cwd, ".nerd", "browser", "control.txt")
	if err := os.MkdirAll(filepath.Dir(controlFile), 0o755); err == nil {
		if err := os.WriteFile(controlFile, []byte(mgr.ControlURL()), 0o644); err != nil {
			logging.BootWarn("failed to write browser control file: %v", err)
		}
	}

	fmt.Printf("Browser launched. Control URL: %s\n", mgr.ControlURL())
	fmt.Printf("Session store: %s\n", cfg.SessionStore)
	fmt.Println("Press Ctrl+C to shutdown")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Clean up control file
	if err := os.Remove(controlFile); err != nil && !os.IsNotExist(err) {
		logging.BootWarn("failed to remove browser control file: %v", err)
	}
	if err := mgr.Shutdown(context.Background()); err != nil {
		logging.BootWarn("failed to shutdown browser manager: %v", err)
	}
	return nil
}

// browserSession creates a new browser session
func browserSession(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := args[0]
	logger.Info("Creating browser session", zap.String("url", url))

	cfg := getBrowserConfig()

	// Try to connect to existing browser first
	cwd, _ := os.Getwd()
	controlFile := filepath.Join(cwd, ".nerd", "browser", "control.txt")
	if controlURL, err := os.ReadFile(controlFile); err == nil && len(controlURL) > 0 {
		cfg.DebuggerURL = string(controlURL)
		logger.Info("Connecting to existing browser", zap.String("url", cfg.DebuggerURL))
	}

	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	if err := loadBrowserSchema(engine); err != nil {
		return err
	}

	mgr, cfg, err := startBrowserManagerWithFallback(ctx, cfg, engine, controlFile)
	if err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}

	session, err := mgr.CreateSession(ctx, url)
	if err != nil {
		// Shutdown only if we launched a new browser (cfg.DebuggerURL empty means
		// either no control file existed or we fell back from a stale one and
		// launched it ourselves).
		if cfg.DebuggerURL == "" {
			_ = mgr.Shutdown(context.Background())
		}
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Session created: %s\n", session.ID)
	fmt.Printf("Target ID: %s\n", session.TargetID)
	fmt.Printf("URL: %s\n", session.URL)
	fmt.Printf("\nUse 'nerd browser snapshot %s' to capture DOM facts\n", session.ID)

	// Note: Don't shutdown - leave browser running for snapshot command
	return nil
}

// browserSnapshot snapshots DOM as Mangle facts
func browserSnapshot(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	logger.Info("Snapshotting DOM", zap.String("session", sessionID))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cfg := getBrowserConfig()

	// Must connect to existing browser
	cwd, _ := os.Getwd()
	controlFile := filepath.Join(cwd, ".nerd", "browser", "control.txt")
	controlURL, err := os.ReadFile(controlFile)
	if err != nil || len(controlURL) == 0 {
		return fmt.Errorf("no browser running - use 'nerd browser launch' first")
	}
	cfg.DebuggerURL = string(controlURL)

	// Create mangle engine to receive facts
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	if err := loadBrowserSchema(engine); err != nil {
		return err
	}

	// Snapshot never shuts the manager down, so it does not need the updated
	// config the way browserSession does for its ownership check.
	mgr, _, err := startBrowserManagerWithFallback(ctx, cfg, engine, controlFile)
	if err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	// Look up session
	session, found := mgr.GetSession(sessionID)
	if !found {
		// List available sessions
		sessions := mgr.List()
		if len(sessions) == 0 {
			return fmt.Errorf("session %q not found - no active sessions", sessionID)
		}
		fmt.Printf("Session %q not found. Available sessions:\n", sessionID)
		for _, s := range sessions {
			fmt.Printf("  %s  [%s] %s\n", s.ID, s.Status, s.URL)
		}
		return fmt.Errorf("session not found")
	}

	// Reattach to the session's target if needed
	if session.Status == "detached" && session.TargetID != "" {
		logger.Info("Reattaching to detached session", zap.String("target", session.TargetID))
		reattached, err := mgr.Attach(ctx, session.TargetID)
		if err != nil {
			return fmt.Errorf("failed to reattach to session: %w", err)
		}
		sessionID = reattached.ID
	}

	// Capture DOM facts
	fmt.Printf("Capturing DOM for session %s...\n", sessionID)
	if err := mgr.SnapshotDOM(ctx, sessionID); err != nil {
		return fmt.Errorf("DOM snapshot failed: %w", err)
	}

	// Also capture React components if available
	reactFacts, err := mgr.ReifyReact(ctx, sessionID)
	if err != nil {
		logger.Info("React reification skipped", zap.Error(err))
	} else {
		fmt.Printf("Captured %d React component facts\n", len(reactFacts))
	}

	// Export facts to file
	factsDir := filepath.Join(cwd, ".nerd", "browser", "snapshots")
	if err := os.MkdirAll(factsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create snapshots dir: %w", err)
	}

	snapshotFile := filepath.Join(factsDir, fmt.Sprintf("%s_%d.mg", sessionID, time.Now().Unix()))

	// Query for all DOM-related predicates
	domPredicates := []string{
		"dom_node", "dom_text", "dom_attr", "dom_layout",
		"react_component", "react_prop", "react_state", "dom_mapping",
		"navigation_event", "current_url", "console_event",
		"net_request", "net_response", "net_header", "request_initiator",
		"click_event", "input_event", "state_change",
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# DOM Snapshot for session %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("# Captured at %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# URL: %s\n\n", session.URL))

	totalFacts := 0
	for _, pred := range domPredicates {
		facts, err := engine.GetFacts(pred)
		if err != nil {
			continue // Predicate not declared, skip
		}
		for _, fact := range facts {
			sb.WriteString(fact.String())
			sb.WriteString("\n")
			totalFacts++
		}
	}

	if totalFacts == 0 {
		fmt.Println("Warning: No DOM facts captured. The page may not have loaded yet.")
		fmt.Println("Try waiting for the page to fully load, then run snapshot again.")
	}

	if err := os.WriteFile(snapshotFile, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	fmt.Printf("DOM snapshot complete:\n")
	fmt.Printf("  Facts captured: %d\n", totalFacts)
	fmt.Printf("  Saved to: %s\n", snapshotFile)
	return nil
}
