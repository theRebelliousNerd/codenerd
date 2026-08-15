// Package main implements the codeNERD CLI commands.
// This file contains browser automation commands.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"codenerd/internal/browser"
	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// =============================================================================
// BROWSER COMMANDS - Browser automation (§9.0 Browser Physics)
// =============================================================================

// browserOperatorNotice is printed by every browser command that produces
// facts.
//
// Each CLI invocation builds its own mangle.Engine and hands it to a
// SessionManager. That engine dies with the process: nothing it derives is
// visible to the chat/agent Cortex, whose browser manager writes into the live
// kernel instead. Two operators looking at the same Chrome window therefore see
// two disjoint fact worlds, and an operator who "checked for honeypots" from
// the CLI has told the agent nothing. The risk is architectural, not a bug, so
// the UX states it rather than hiding it.
const browserOperatorNotice = "note: facts from this command live in the CLI's own kernel and are NOT visible to the chat/agent Cortex (use the browser tools inside chat for agent-visible facts)"

// browserCmd manages browser sessions (§9.0 Browser Physics)
var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Browser automation commands (DOM snapshotting, React reification)",
	Long: `Browser automation commands (DOM snapshotting, React reification).

Operator note: every browser subcommand runs its own logic kernel. Facts
captured here (DOM, network, honeypot verdicts) stay in that process and never
reach the chat/agent Cortex, which drives its own shared browser manager. Use
the in-chat browser tools when the agent needs to see what you see.`,
	RunE: parentGroupRunE,
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

var browserListCmd = &cobra.Command{
	Use:   "list",
	Short: "List known browser sessions",
	Args:  cobra.NoArgs,
	RunE:  browserList,
}

var browserScreenshotCmd = &cobra.Command{
	Use:   "screenshot [session-id]",
	Short: "Capture a screenshot of a session",
	Args:  cobra.ExactArgs(1),
	RunE:  browserScreenshot,
}

var browserClickCmd = &cobra.Command{
	Use:   "click [session-id] [selector]",
	Short: "Click an element (refused when the kernel derives is_honeypot)",
	Args:  cobra.ExactArgs(2),
	RunE:  browserClick,
}

var browserTypeCmd = &cobra.Command{
	Use:   "type [session-id] [selector] [text]",
	Short: "Type into an element (refused when the kernel derives is_honeypot)",
	Args:  cobra.ExactArgs(3),
	RunE:  browserType,
}

var browserForkCmd = &cobra.Command{
	Use:   "fork [session-id] [url]",
	Short: "Fork a session's cookies and storage into an isolated tab",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  browserFork,
}

var browserHoneypotCmd = &cobra.Command{
	Use:   "honeypot [session-id]",
	Short: "Report honeypot verdicts for the session's links or one selector",
	Args:  cobra.ExactArgs(1),
	RunE:  browserHoneypot,
}

var (
	browserScreenshotOut  string
	browserScreenshotFull bool
	browserAllowHoneypot  bool
	browserTypeSubmit     bool
	browserHoneypotTarget string
	browserHoneypotAll    bool
	browserIngestHeaders  bool
)

func init() {
	browserCmd.PersistentFlags().BoolVar(&browserIngestHeaders, "ingest-headers", false,
		"Ingest redacted request/response headers as facts (off by default for operator sessions)")
	browserScreenshotCmd.Flags().StringVarP(&browserScreenshotOut, "out", "o", "", "Output path (defaults to .nerd/browser/screenshots)")
	browserScreenshotCmd.Flags().BoolVar(&browserScreenshotFull, "full-page", false, "Capture the full scrollable page")
	browserClickCmd.Flags().BoolVar(&browserAllowHoneypot, "allow-honeypot", false, "Interact even when the kernel flags the element as a honeypot")
	browserTypeCmd.Flags().BoolVar(&browserAllowHoneypot, "allow-honeypot", false, "Interact even when the kernel flags the element as a honeypot")
	browserTypeCmd.Flags().BoolVar(&browserTypeSubmit, "submit", false, "Press Enter after typing")
	browserHoneypotCmd.Flags().StringVar(&browserHoneypotTarget, "selector", "", "Check a single element instead of every link")
	browserHoneypotCmd.Flags().BoolVar(&browserHoneypotAll, "all", false, "Report every link, not only the flagged ones")

	// browserCmd itself is registered in main.go; its subcommands are owned by
	// this file so the two do not have to be edited together.
	browserCmd.AddCommand(
		browserListCmd,
		browserScreenshotCmd,
		browserClickCmd,
		browserTypeCmd,
		browserForkCmd,
		browserHoneypotCmd,
	)
}

// getBrowserConfig returns browser config with persistent session store
func getBrowserConfig() browser.Config {
	cwd, _ := os.Getwd()
	cfg := browser.DefaultConfig()
	if userCfg, err := config.LoadUserConfig(filepath.Join(cwd, ".nerd", "config.json")); err == nil {
		configured := userCfg.GetBrowserConfig()
		cfg.DebuggerURL = configured.DebuggerURL
		cfg.Launch = append([]string(nil), configured.Launch...)
		cfg.Headless = configured.Headless
		cfg.ViewportWidth = configured.ViewportWidth
		cfg.ViewportHeight = configured.ViewportHeight
		cfg.NavigationTimeoutMs = configured.NavigationTimeoutMs
		cfg.MultiTabDefault = configured.MultiTabDefault
		cfg.MaxTabs = configured.MaxTabs
		cfg.MaxBrowsers = configured.MaxBrowsers
		cfg.IdleTabTimeoutMs = configured.IdleTabTimeoutMs
		cfg.ExtraSensitiveKeys = append([]string(nil), configured.ExtraSensitiveKeys...)
		cfg.WritableRoots = append([]string(nil), configured.WritableRoots...)
		cfg.EvidenceEnabled = configured.EvidenceEnabled
		cfg.EvidenceDir = configured.EvidenceDir
		cfg.MaxEvidenceFiles = configured.MaxEvidenceFiles
		cfg.MaxEvidenceFileBytes = configured.MaxEvidenceFileBytes
		cfg.Specs = configured.Specs
	}
	cfg.WorkspaceRoot = cwd
	cfg.SessionStore = filepath.Join(cwd, ".nerd", "browser", "sessions.json")
	// Operator mode. CLI sessions drive the human's own Chrome profile, where
	// request headers carry live session cookies and bearer tokens; the
	// research default of redacted header ingestion is not worth that exposure
	// on a surface nobody is asking questions of. --ingest-headers opts back in
	// per invocation (config.BrowserAutomationConfig has no field for it yet).
	cfg.HeaderIngestionMode = browser.HeaderIngestionOff
	if browserIngestHeaders {
		cfg.HeaderIngestionMode = browser.HeaderIngestionRedacted
	}
	return cfg
}

// browserControlFile returns the path of the file `nerd browser launch` writes.
func browserControlFile() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".nerd", "browser", "control.txt")
}

// connectBrowserManager attaches to the browser started by `nerd browser
// launch`. It never launches one: a command that silently spawned a second
// Chrome would operate on an empty session list and confuse the operator far
// more than an error does.
func connectBrowserManager(ctx context.Context) (*browser.SessionManager, *mangle.Engine, error) {
	cfg := getBrowserConfig()
	controlFile := browserControlFile()
	controlURL, err := os.ReadFile(controlFile)
	if err != nil || len(strings.TrimSpace(string(controlURL))) == 0 {
		return nil, nil, fmt.Errorf("no browser running - use 'nerd browser launch' first")
	}
	cfg.DebuggerURL = strings.TrimSpace(string(controlURL))
	if browserAllowHoneypot {
		cfg.HoneypotGuard = browser.HoneypotGuardOff
	}

	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create mangle engine: %w", err)
	}
	if err := loadBrowserLogic(engine); err != nil {
		return nil, nil, err
	}

	mgr, _, err := startBrowserManagerWithFallback(ctx, cfg, engine, controlFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to browser: %w", err)
	}
	return mgr, engine, nil
}

// resolveBrowserSession looks a session up, reattaching to its target when the
// persisted metadata says the page handle was lost.
func resolveBrowserSession(ctx context.Context, mgr *browser.SessionManager, sessionID string) (string, error) {
	session, found := mgr.GetSession(sessionID)
	if !found {
		sessions := mgr.List()
		if len(sessions) == 0 {
			return "", fmt.Errorf("session %q not found - no active sessions", sessionID)
		}
		fmt.Printf("Session %q not found. Available sessions:\n", sessionID)
		for _, s := range sessions {
			fmt.Printf("  %s  [%s] %s\n", s.ID, s.Status, s.URL)
		}
		return "", fmt.Errorf("session not found")
	}
	if session.Status == "detached" && session.TargetID != "" {
		reattached, err := mgr.Attach(ctx, session.TargetID)
		if err != nil {
			return "", fmt.Errorf("failed to reattach to session: %w", err)
		}
		return reattached.ID, nil
	}
	return sessionID, nil
}

// loadBrowserLogic loads both the browser schema and the honeypot rule file.
// The schema alone leaves is_honeypot undeclared-but-empty, so every honeypot
// verdict would come back negative without any error to explain why.
func loadBrowserLogic(engine *mangle.Engine) error {
	if err := loadBrowserSchema(engine); err != nil {
		return err
	}
	policy, err := core.GetDefaultContent("policy/browser_honeypot.mg")
	if err != nil {
		return fmt.Errorf("failed to read browser honeypot policy: %w", err)
	}
	if err := engine.LoadSchemaString(policy); err != nil {
		return fmt.Errorf("failed to load browser honeypot policy: %w", err)
	}
	return nil
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
	if err := browsersecurity.EnsurePrivateDir(filepath.Dir(controlFile)); err == nil {
		if err := browsersecurity.WritePrivateFile(controlFile, []byte(mgr.ControlURL())); err != nil {
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
	if err := browsersecurity.EnsurePrivateDir(factsDir); err != nil {
		return fmt.Errorf("failed to create snapshots dir: %w", err)
	}

	snapshotFile, err := mgr.ResolveOutputPath("", factsDir, fmt.Sprintf("%s_%d.mg", sessionID, time.Now().Unix()))
	if err != nil {
		return err
	}

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

	if err := browsersecurity.WritePrivateFile(snapshotFile, []byte(sb.String())); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	fmt.Printf("DOM snapshot complete:\n")
	fmt.Printf("  Facts captured: %d\n", totalFacts)
	fmt.Printf("  Saved to: %s\n", snapshotFile)
	return nil
}

// browserList prints known sessions. It reads the persisted store when no
// browser is running so an operator can see what a previous run left behind
// without paying for a Chrome launch.
func browserList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if _, err := os.Stat(browserControlFile()); err != nil {
		cfg := getBrowserConfig()
		data, readErr := os.ReadFile(cfg.SessionStore)
		if readErr != nil {
			fmt.Println("No browser running and no persisted sessions.")
			fmt.Println("Start one with 'nerd browser launch'.")
			return nil
		}
		var sessions []browser.Session
		if err := json.Unmarshal(data, &sessions); err != nil {
			return fmt.Errorf("failed to read session store %s: %w", cfg.SessionStore, err)
		}
		fmt.Printf("No browser running. %d persisted session(s) in %s:\n", len(sessions), cfg.SessionStore)
		for _, s := range sessions {
			fmt.Printf("  %s  [detached] %s\n", s.ID, s.URL)
		}
		return nil
	}

	mgr, _, err := connectBrowserManager(ctx)
	if err != nil {
		return err
	}

	browsers := mgr.ListBrowsers()
	fmt.Printf("Browsers (%d):\n", len(browsers))
	for _, b := range browsers {
		marker := " "
		if b.Default {
			marker = "*"
		}
		fmt.Printf("  %s %s  tabs=%d\n", marker, b.ID, b.TabCount)
	}

	sessions := mgr.List()
	fmt.Printf("Sessions (%d):\n", len(sessions))
	if len(sessions) == 0 {
		fmt.Println("  (none) - create one with 'nerd browser session <url>'")
		return nil
	}
	for _, s := range sessions {
		title := s.Title
		if title == "" {
			title = "-"
		}
		fmt.Printf("  %s  [%s] %s\n      title: %s  browser: %s  last active: %s\n",
			s.ID, s.Status, s.URL, title, s.BrowserID, s.LastActive.Format(time.RFC3339))
	}
	return nil
}

// browserScreenshot captures a screenshot of a session.
func browserScreenshot(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mgr, _, err := connectBrowserManager(ctx)
	if err != nil {
		return err
	}
	sessionID, err := resolveBrowserSession(ctx, mgr, args[0])
	if err != nil {
		return err
	}

	data, err := mgr.Screenshot(ctx, sessionID, browserScreenshotFull)
	if err != nil {
		return fmt.Errorf("screenshot failed: %w", err)
	}

	cwd, _ := os.Getwd()
	defaultDir := filepath.Join(cwd, ".nerd", "browser", "screenshots")
	if err := browsersecurity.EnsurePrivateDir(defaultDir); err != nil {
		return fmt.Errorf("failed to create screenshots dir: %w", err)
	}
	target, err := mgr.ResolveOutputPath(browserScreenshotOut, defaultDir,
		fmt.Sprintf("%s_%d.png", sessionID, time.Now().Unix()))
	if err != nil {
		return err
	}
	if err := browsersecurity.WritePrivateFile(target, data); err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}

	fmt.Printf("Screenshot saved: %s (%d bytes)\n", target, len(data))
	return nil
}

// browserClick clicks an element, subject to the honeypot guard.
func browserClick(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mgr, _, err := connectBrowserManager(ctx)
	if err != nil {
		return err
	}
	sessionID, err := resolveBrowserSession(ctx, mgr, args[0])
	if err != nil {
		return err
	}

	if err := mgr.Click(ctx, sessionID, args[1]); err != nil {
		if errors.Is(err, browser.ErrHoneypotBlocked) {
			return fmt.Errorf("%w - rerun with --allow-honeypot to override", err)
		}
		return err
	}
	fmt.Printf("Clicked %s in session %s\n", args[1], sessionID)
	fmt.Println(browserOperatorNotice)
	return nil
}

// browserType types into an element, subject to the honeypot guard.
func browserType(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mgr, _, err := connectBrowserManager(ctx)
	if err != nil {
		return err
	}
	sessionID, err := resolveBrowserSession(ctx, mgr, args[0])
	if err != nil {
		return err
	}

	if err := mgr.Type(ctx, sessionID, args[1], args[2]); err != nil {
		if errors.Is(err, browser.ErrHoneypotBlocked) {
			return fmt.Errorf("%w - rerun with --allow-honeypot to override", err)
		}
		return err
	}
	if browserTypeSubmit {
		if err := mgr.PressKey(ctx, sessionID, "Enter"); err != nil {
			return fmt.Errorf("submit failed: %w", err)
		}
	}
	// The typed text is deliberately not echoed: it is frequently a credential.
	fmt.Printf("Typed %d characters into %s (session %s)\n", len([]rune(args[2])), args[1], sessionID)
	fmt.Println(browserOperatorNotice)
	return nil
}

// browserFork clones a session's cookies and storage into an isolated tab.
func browserFork(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mgr, _, err := connectBrowserManager(ctx)
	if err != nil {
		return err
	}
	sessionID, err := resolveBrowserSession(ctx, mgr, args[0])
	if err != nil {
		return err
	}
	url := ""
	if len(args) > 1 {
		url = args[1]
	}

	forked, err := mgr.ForkSession(ctx, sessionID, url)
	if err != nil {
		return fmt.Errorf("fork failed: %w", err)
	}
	fmt.Printf("Forked session %s -> %s\n", sessionID, forked.ID)
	fmt.Printf("  URL: %s\n", forked.URL)
	fmt.Printf("  Isolated: %v\n", forked.Isolated)
	return nil
}

// browserHoneypot reports what the kernel derives about a page's links, or one
// selector when --selector is given.
func browserHoneypot(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	mgr, _, err := connectBrowserManager(ctx)
	if err != nil {
		return err
	}
	sessionID, err := resolveBrowserSession(ctx, mgr, args[0])
	if err != nil {
		return err
	}
	page, ok := mgr.Page(sessionID)
	if !ok {
		return fmt.Errorf("session %s has no live page", sessionID)
	}
	detector := mgr.HoneypotDetector()
	if detector == nil {
		return fmt.Errorf("honeypot detection unavailable: no queryable fact store")
	}

	if browserHoneypotTarget != "" {
		isHoneypot, reasons, checkErr := detector.IsHoneypot(page, browserHoneypotTarget)
		if checkErr != nil {
			return checkErr
		}
		fmt.Printf("%s: honeypot=%v\n", browserHoneypotTarget, isHoneypot)
		for _, reason := range reasons {
			fmt.Printf("  - %s\n", reason)
		}
		fmt.Println(browserOperatorNotice)
		return nil
	}

	links, err := detector.GetAllLinksWithAnalysis(page)
	if err != nil {
		return err
	}
	flagged := 0
	for _, link := range links {
		if link.IsHoneypot {
			flagged++
		}
	}
	fmt.Printf("Analyzed %d links in session %s: %d flagged\n", len(links), sessionID, flagged)
	for _, link := range links {
		if !link.IsHoneypot && !browserHoneypotAll {
			continue
		}
		status := "safe"
		if link.IsHoneypot {
			status = "HONEYPOT"
		}
		fmt.Printf("  [%s] %s\n", status, link.Href)
		for _, reason := range link.HoneypotReasons {
			fmt.Printf("      - %s\n", reason)
		}
	}
	fmt.Println(browserOperatorNotice)
	return nil
}
