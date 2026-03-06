package main

import (
	"codenerd/internal/config"
	"codenerd/internal/perception"
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

	// Load config before probing so custom Codex settings are honored.
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	probeCfg := cfg.GetCodexCLIConfig()

	fmt.Println("Running noninteractive codex exec readiness probe...")
	probeCtx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
	defer cancel()
	probeClient := perception.NewCodexCLIClient(probeCfg)
	probeResult, probeErr := probeClient.RunHealthProbe(probeCtx)
	if probeErr != nil {
		fmt.Printf("Codex exec probe status: %s\n", probeResult.Failure)
		if probeResult.Detail != "" {
			fmt.Printf("Details: %s\n", probeResult.Detail)
		}
		if probeResult.RawError != "" {
			fmt.Printf("Raw error: %s\n", probeResult.RawError)
		}
		switch probeResult.Failure {
		case perception.CodexCLIProbeFailureAuthUnavailable:
			fmt.Println("\nPlease run 'codex login' to authenticate with your ChatGPT subscription.")
			return fmt.Errorf("codex CLI not authenticated")
		case perception.CodexCLIProbeFailureSkillMissing:
			return fmt.Errorf("codex exec repo skill missing")
		case perception.CodexCLIProbeFailureSchemaRejected:
			return fmt.Errorf("codex exec schema probe failed")
		case perception.CodexCLIProbeFailureRateLimited:
			return fmt.Errorf("codex exec rate limited during probe")
		case perception.CodexCLIProbeFailureFallbackModelMissing:
			return fmt.Errorf("codex exec fallback model exhausted during probe")
		default:
			return fmt.Errorf("codex exec readiness probe failed")
		}
	}
	fmt.Println("✓ Codex exec is authenticated and ready")

	if err := cfg.SetEngine("codex-cli"); err != nil {
		return fmt.Errorf("failed to set engine: %w", err)
	}

	cfg.CodexCLI = probeCfg

	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("  Engine: codex-cli")
	fmt.Printf("  Model: %s\n", cfg.CodexCLI.Model)
	fmt.Printf("  Sandbox: %s\n", cfg.CodexCLI.Sandbox)
	if cfg.CodexCLI.SkillEnabled != nil {
		fmt.Printf("  Skill enabled: %t\n", *cfg.CodexCLI.SkillEnabled)
	}
	fmt.Printf("  Skill: %s\n", cfg.CodexCLI.SkillName)
	fmt.Printf("  Max concurrent calls: %d\n", cfg.CodexCLI.MaxConcurrentCalls)
	fmt.Println("\ncodeNERD will now use your ChatGPT subscription for LLM calls.")
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
		fmt.Printf("  Skill enabled: %t\n", cliCfg.SkillEnabled != nil && *cliCfg.SkillEnabled)
		fmt.Printf("  Skill name: %s\n", cliCfg.SkillName)
		fmt.Printf("  Max concurrent calls: %d\n", cliCfg.MaxConcurrentCalls)
		fmt.Printf("  Effective scheduler ceiling: %d\n", cfg.GetEffectiveMaxConcurrentAPICalls())

		// Check CLI status
		if codexPath, err := findExecutable("codex"); err != nil {
			fmt.Println("  Status: ❌ CLI not installed")
		} else {
			fmt.Printf("  Status: ✓ CLI installed (%s)\n", codexPath)
			probeCtx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			probeClient := perception.NewCodexCLIClient(cliCfg)
			probeResult, probeErr := probeClient.RunHealthProbe(probeCtx)
			probeLabel := "success"
			if probeResult.Failure != perception.CodexCLIProbeFailureNone {
				probeLabel = string(probeResult.Failure)
			}
			fmt.Printf("  Probe: %s\n", probeLabel)
			fmt.Printf("  Skill path: %s\n", probeResult.SkillPath)
			fmt.Printf("  Skill available: %t\n", probeResult.SkillAvailable)
			fmt.Printf("  Schema support: %t\n", probeResult.SchemaValidated)
			if probeResult.AuthAvailable {
				fmt.Println("  Auth: ✓ noninteractive codex exec usable")
			} else if probeErr == nil {
				fmt.Println("  Auth: ✓ noninteractive codex exec ready")
			} else {
				fmt.Printf("  Auth: ❌ %s\n", probeResult.Detail)
			}
		}

	default:
		fmt.Println("Backend: HTTP API")
		provider, _ := cfg.GetActiveProvider()
		fmt.Printf("  Provider: %s\n", provider)
		if cfg.Model != "" {
			fmt.Printf("  Model: %s\n", cfg.Model)
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
