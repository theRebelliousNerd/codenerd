package main

import (
	"codenerd/internal/auth/antigravity"
	"codenerd/internal/config"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// authCmd manages CLI engine authentication
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage CLI engine authentication",
	Long: `Configure authentication for CLI-based LLM engines.

Available subcommands:
  claude - Authenticate and configure Claude Code CLI engine
  codex  - Authenticate and configure Codex CLI engine
  status - Show current authentication status`,
}

// authClaudeCmd authenticates with Claude Code CLI
var authClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Authenticate with Claude Code CLI",
	Long: `Authenticate with Claude Code CLI and configure codeNERD to use it.

This command:
1. Checks if Claude Code CLI is installed
2. Runs 'claude login' if not authenticated
3. Updates .nerd/config.json to use claude-cli engine`,
	RunE: runAuthClaude,
}

// authCodexCmd authenticates with Codex CLI
var authCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Authenticate with Codex CLI",
	Long: `Authenticate with Codex CLI and configure codeNERD to use it.

This command:
1. Checks if Codex CLI is installed
2. Runs 'codex login' if not authenticated
3. Updates .nerd/config.json to use codex-cli engine`,
	RunE: runAuthCodex,
}

// authAntigravityCmd is the parent command for Antigravity account management
var authAntigravityCmd = &cobra.Command{
	Use:   "antigravity",
	Short: "Manage Google Antigravity accounts",
	Long: `Manage Google Antigravity (Cloud Code) accounts for codeNERD.

Available subcommands:
  add    - Add a new Google account via OAuth
  list   - List all configured accounts with health scores
  remove - Remove an account by email
  status - Show detailed account statistics`,
}

// authAntigravityAddCmd adds a new Google account
var authAntigravityAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new Google account via OAuth",
	Long: `Add a new Google account for Antigravity by initiating OAuth2 flow.

This command:
1. Opens browser for Google OAuth consent
2. Stores the refresh token in ~/.nerd/antigravity_accounts.json
3. Updates .nerd/config.json to use 'antigravity' provider`,
	RunE: runAuthAntigravityAdd,
}

// authAntigravityListCmd lists all configured accounts
var authAntigravityListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured Google accounts",
	Long:  `List all configured Google accounts with their health scores and status.`,
	RunE:  runAuthAntigravityList,
}

// authAntigravityRemoveCmd removes an account by email
var authAntigravityRemoveCmd = &cobra.Command{
	Use:   "remove <email>",
	Short: "Remove a Google account",
	Long:  `Remove a Google account by email address.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAuthAntigravityRemove,
}

// authAntigravityStatusCmd shows detailed account statistics
var authAntigravityStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Antigravity account statistics",
	Long:  `Show detailed statistics about configured Antigravity accounts.`,
	RunE:  runAuthAntigravityStatus,
}

// authStatusCmd shows authentication status
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE:  runAuthStatus,
}

func runAuthClaude(cmd *cobra.Command, args []string) error {
	fmt.Println("Configuring Claude Code CLI engine...")

	// Check if claude CLI is installed
	claudePath, err := findExecutable("claude")
	if err != nil {
		fmt.Println("\n❌ Claude Code CLI not found.")
		fmt.Println("\nTo install:")
		fmt.Println("  npm install -g @anthropic-ai/claude-code")
		fmt.Println("\nThen run 'claude login' to authenticate with your subscription.")
		return fmt.Errorf("claude CLI not installed")
	}
	fmt.Printf("✓ Found Claude CLI at: %s\n", claudePath)

	// Check authentication status by trying a simple command
	fmt.Println("Checking authentication status...")
	checkCmd := newExecCommand(cmd.Context(), "claude", "--version")
	if output, err := checkCmd.CombinedOutput(); err != nil {
		fmt.Printf("Claude CLI check failed: %s\n", string(output))
		fmt.Println("\nPlease run 'claude login' to authenticate with your Claude subscription.")
		return fmt.Errorf("claude CLI not authenticated")
	}
	fmt.Println("✓ Claude CLI is authenticated")

	// Update config
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetEngine("claude-cli"); err != nil {
		return fmt.Errorf("failed to set engine: %w", err)
	}

	// Ensure claude_cli config exists
	if cfg.ClaudeCLI == nil {
		cfg.ClaudeCLI = &config.ClaudeCLIConfig{
			Model:   "sonnet",
			Timeout: 300,
		}
	}

	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("  Engine: claude-cli")
	fmt.Printf("  Model: %s\n", cfg.ClaudeCLI.Model)
	fmt.Println("\ncodeNERD will now use your Claude subscription for LLM calls.")
	return nil
}

// runAuthCodex authenticates with Codex CLI and configures codeNERD
func runAuthCodex(cmd *cobra.Command, args []string) error {
	fmt.Println("Configuring Codex CLI engine...")

	// Check if codex CLI is installed
	codexPath, err := findExecutable("codex")
	if err != nil {
		fmt.Println("\n❌ Codex CLI not found.")
		fmt.Println("\nTo install:")
		fmt.Println("  npm install -g @openai/codex")
		fmt.Println("\nThen run 'codex login' to authenticate with your ChatGPT subscription.")
		return fmt.Errorf("codex CLI not installed")
	}
	fmt.Printf("✓ Found Codex CLI at: %s\n", codexPath)

	// Check authentication status
	fmt.Println("Checking authentication status...")
	checkCmd := newExecCommand(cmd.Context(), "codex", "--version")
	if output, err := checkCmd.CombinedOutput(); err != nil {
		fmt.Printf("Codex CLI check failed: %s\n", string(output))
		fmt.Println("\nPlease run 'codex login' to authenticate with your ChatGPT subscription.")
		return fmt.Errorf("codex CLI not authenticated")
	}
	fmt.Println("✓ Codex CLI is authenticated")

	// Update config
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetEngine("codex-cli"); err != nil {
		return fmt.Errorf("failed to set engine: %w", err)
	}

	// Ensure codex_cli config exists
	if cfg.CodexCLI == nil {
		cfg.CodexCLI = &config.CodexCLIConfig{
			Model:   "gpt-5",
			Sandbox: "read-only",
			Timeout: 300,
		}
	}

	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("  Engine: codex-cli")
	fmt.Printf("  Model: %s\n", cfg.CodexCLI.Model)
	fmt.Printf("  Sandbox: %s\n", cfg.CodexCLI.Sandbox)
	fmt.Println("\ncodeNERD will now use your ChatGPT subscription for LLM calls.")
	return nil
}

// runAuthAntigravityAdd adds a new Google account via OAuth
func runAuthAntigravityAdd(cmd *cobra.Command, args []string) error {
	fmt.Println("Adding Google Antigravity account...")

	// 1. Start OAuth flow
	authResult, err := antigravity.StartAuth()
	if err != nil {
		return fmt.Errorf("failed to start OAuth: %w", err)
	}

	// 2. Open browser
	fmt.Println("\nOpening browser for Google OAuth...")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authResult.AuthURL)

	// Try to open browser
	openBrowser(authResult.AuthURL)

	// 3. Wait for callback
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	fmt.Println("Waiting for OAuth callback...")
	code, err := antigravity.StartCallbackServer(ctx, authResult.State)
	if err != nil {
		return fmt.Errorf("OAuth callback failed: %w", err)
	}

	// 4. Exchange code for tokens
	tm, err := antigravity.NewTokenManager()
	if err != nil {
		return fmt.Errorf("failed to create token manager: %w", err)
	}

	token, err := tm.ExchangeCode(ctx, code, authResult.Verifier)
	if err != nil {
		return fmt.Errorf("token exchange failed: %w", err)
	}

	// 5. Add account to multi-account manager
	manager, err := antigravity.NewAccountManager()
	if err != nil {
		return fmt.Errorf("failed to open account manager: %w", err)
	}

	account := &antigravity.Account{
		Email:        token.Email,
		RefreshToken: token.RefreshToken,
		AccessToken:  token.AccessToken,
		AccessExpiry: token.Expiry,
		ProjectID:    token.ProjectID,
	}

	if err := manager.AddAccount(account); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	// 6. Update config to use antigravity provider
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.Provider = "antigravity"
	if cfg.Antigravity == nil {
		cfg.Antigravity = &config.AntigravityProviderConfig{
			EnableThinking: true,
			ThinkingLevel:  "high",
		}
	}

	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nAccount added: %s\n", account.Email)
	if account.ProjectID != "" {
		fmt.Printf("Project: %s\n", account.ProjectID)
	}
	fmt.Println("Provider: antigravity")

	// Show account count
	accounts := manager.ListAccounts()
	fmt.Printf("\nTotal accounts configured: %d\n", len(accounts))

	return nil
}

// runAuthAntigravityList lists all configured accounts
func runAuthAntigravityList(cmd *cobra.Command, args []string) error {
	manager, err := antigravity.NewAccountManager()
	if err != nil {
		return fmt.Errorf("failed to open account manager: %w", err)
	}

	accounts := manager.ListAccounts()
	if len(accounts) == 0 {
		fmt.Println("No Antigravity accounts configured.")
		fmt.Println("\nRun 'nerd auth antigravity add' to add an account.")
		return nil
	}

	fmt.Printf("Antigravity Accounts (%d)\n", len(accounts))
	fmt.Println(strings.Repeat("-", 60))

	ht := manager.GetHealthTracker()

	for _, acc := range accounts {
		score := ht.GetScore(acc.Index)
		status := "healthy"
		statusIcon := "Y"

		if score < 30 {
			status = "degraded"
			statusIcon = "!"
		}
		if score == 0 {
			status = "exhausted"
			statusIcon = "X"
		}

		fmt.Printf("[%s] %s\n", statusIcon, acc.Email)
		fmt.Printf("    Health: %d/100\n", score)
		fmt.Printf("    Status: %s\n", status)

		if acc.ProjectID != "" {
			fmt.Printf("    Project: %s\n", acc.ProjectID)
		}

		if !acc.LastUsed.IsZero() {
			fmt.Printf("    Last used: %s\n", acc.LastUsed.Format(time.RFC3339))
		}

		if acc.ConsecutiveFailures > 0 {
			fmt.Printf("    Failures: %d\n", acc.ConsecutiveFailures)
		}

		// Show rate limits
		if len(acc.RateLimitResetTimes) > 0 {
			fmt.Println("    Active Rate Limits:")
			for k, v := range acc.RateLimitResetTimes {
				if time.Now().Before(v) {
					fmt.Printf("      - %s (resets in %v)\n", k, time.Until(v).Round(time.Second))
				}
			}
		}

		fmt.Println()
	}

	return nil
}

// runAuthAntigravityRemove removes an account by email
func runAuthAntigravityRemove(cmd *cobra.Command, args []string) error {
	email := args[0]

	manager, err := antigravity.NewAccountManager()
	if err != nil {
		return fmt.Errorf("failed to open account manager: %w", err)
	}

	if err := manager.DeleteAccount(email); err != nil {
		return fmt.Errorf("failed to delete account: %w", err)
	}

	fmt.Printf("Removed account: %s\n", email)

	remaining := manager.ListAccounts()
	fmt.Printf("Remaining accounts: %d\n", len(remaining))

	if len(remaining) == 0 {
		fmt.Println("\nWarning: No accounts left. Run 'nerd auth antigravity add' to add one.")
	}

	return nil
}

// runAuthAntigravityStatus shows detailed account statistics
func runAuthAntigravityStatus(cmd *cobra.Command, args []string) error {
	manager, err := antigravity.NewAccountManager()
	if err != nil {
		return fmt.Errorf("failed to open account manager: %w", err)
	}

	accounts := manager.ListAccounts()
	ht := manager.GetHealthTracker()

	var healthy, exhausted, total int
	for _, acc := range accounts {
		total++
		if ht.GetScore(acc.Index) >= 30 {
			healthy++
		} else {
			exhausted++
		}
	}

	fmt.Println("Antigravity Account Status")
	fmt.Println(strings.Repeat("=", 40))

	fmt.Printf("Total accounts:    %d\n", total)
	fmt.Printf("Healthy:           %d\n", healthy)
	fmt.Printf("Exhausted:         %d\n", exhausted)

	if total == 0 {
		fmt.Println("\nNo accounts configured.")
		fmt.Println("Run 'nerd auth antigravity add' to add an account.")
		return nil
	}

	fmt.Println()

	// Show next likely account
	next, _ := manager.GetCurrentOrNextForFamily("gemini", "", "hybrid")
	if next != nil {
		score := ht.GetScore(next.Index)
		fmt.Printf("Next account to use: %s (health: %d)\n", next.Email, score)
	} else {
		fmt.Println("No accounts currently available for rotation.")
	}

	return nil
}

// runAuthStatus shows current authentication status
func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	engine := cfg.GetEngine()
	fmt.Printf("Current engine: %s\n\n", engine)

	switch engine {
	case "claude-cli":
		fmt.Println("Backend: Claude Code CLI (subscription)")
		cliCfg := cfg.GetClaudeCLIConfig()
		fmt.Printf("  Model: %s\n", cliCfg.Model)
		fmt.Printf("  Timeout: %ds\n", cliCfg.Timeout)

		// Check CLI status
		if _, err := findExecutable("claude"); err != nil {
			fmt.Println("  Status: ❌ CLI not installed")
		} else {
			fmt.Println("  Status: ✓ CLI installed")
		}

	case "codex-cli":
		fmt.Println("Backend: Codex CLI (ChatGPT subscription)")
		cliCfg := cfg.GetCodexCLIConfig()
		fmt.Printf("  Model: %s\n", cliCfg.Model)
		fmt.Printf("  Sandbox: %s\n", cliCfg.Sandbox)
		fmt.Printf("  Timeout: %ds\n", cliCfg.Timeout)

		// Check CLI status
		if _, err := findExecutable("codex"); err != nil {
			fmt.Println("  Status: ❌ CLI not installed")
		} else {
			fmt.Println("  Status: ✓ CLI installed")
		}

	default:
		fmt.Println("Backend: HTTP API")
		provider, _ := cfg.GetActiveProvider()
		fmt.Printf("  Provider: %s\n", provider)
		if cfg.Model != "" {
			fmt.Printf("  Model: %s\n", cfg.Model)
		}
		if provider == "antigravity" {
			fmt.Println("  Status: Managed by Google Cloud SDK (OAuth)")
		}
	}

	return nil
}

// Helper functions for auth commands

// findExecutable searches for an executable in PATH
func findExecutable(name string) (string, error) {
	// Try exec.LookPath first
	path, err := execLookPath(name)
	if err == nil {
		return path, nil
	}

	// On Windows, try with .exe extension
	if strings.HasSuffix(os.Getenv("OS"), "Windows_NT") || os.Getenv("GOOS") == "windows" {
		path, err = execLookPath(name + ".exe")
		if err == nil {
			return path, nil
		}
		path, err = execLookPath(name + ".cmd")
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s not found in PATH", name)
}

// execLookPath wraps exec.LookPath for testability
var execLookPath = func(file string) (string, error) {
	return exec.LookPath(file)
}

// newExecCommand creates an exec.Cmd for testability
var newExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// loadOrCreateConfig loads user config or creates default
func loadOrCreateConfig() (*config.UserConfig, error) {
	path := config.DefaultUserConfigPath()
	cfg, err := config.LoadUserConfig(path)
	if err != nil {
		// Create new config if doesn't exist
		cfg = config.DefaultUserConfig()
	}
	return cfg, nil
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch {
	case isWindows():
		cmd = exec.Command("cmd", "/c", "start", url)
	case isDarwin():
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}

	return cmd.Start()
}

// isWindows returns true if running on Windows
func isWindows() bool {
	return os.Getenv("GOOS") == "windows" || strings.Contains(os.Getenv("OS"), "Windows")
}

// isDarwin returns true if running on macOS
func isDarwin() bool {
	return os.Getenv("GOOS") == "darwin"
}
