package main

import (
	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/perception/xaioauth"
	"context"
	"fmt"
	"net/http"
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
	Long: `Configure authentication for CLI-based and SuperGrok OAuth LLM engines.

Available subcommands:
  claude - Authenticate and configure Claude Code CLI engine
  codex  - Authenticate and configure Codex CLI engine
  grok   - Authenticate SuperGrok / X Premium+ OAuth (xai-oauth engine)
  status - Show current authentication status`,
}

// authGrokCmd authenticates SuperGrok OAuth
var authGrokCmd = &cobra.Command{
	Use:   "grok",
	Short: "Authenticate with SuperGrok OAuth",
	Long: `Authenticate with SuperGrok / X Premium+ via OAuth device code and configure codeNERD.

This command:
1. Optionally imports credentials from ~/.grok/auth.json (Grok CLI login)
2. Otherwise runs OAuth device-code login against auth.x.ai
3. Saves tokens to ~/.nerd/xai_oauth.json
4. Updates .nerd/config.json to use engine=xai-oauth

No XAI_API_KEY is required — usage draws SuperGrok subscription limits.`,
	RunE: runAuthGrok,
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

// runAuthGrok authenticates SuperGrok OAuth and configures codeNERD.
func runAuthGrok(cmd *cobra.Command, args []string) error {
	fmt.Println("Configuring SuperGrok OAuth engine (xai-oauth)...")

	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	oauthCfg := cfg.GetXAIOAuthConfig()

	client := xaioauth.NewClientFromUserConfig(oauthCfg)
	ts := client.TokenSource()

	// Prefer existing / importable credentials when they already work.
	probeCtx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()
	if err := ts.Load(); err == nil {
		fmt.Println("Found existing SuperGrok credentials; running health probe...")
		probe := client.RunHealthProbe(probeCtx)
		if probe.Classification == xaioauth.ProbeReady {
			if err := cfg.SetEngine("xai-oauth"); err != nil {
				return err
			}
			cfg.XAIOAuth = oauthCfg
			if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
			fmt.Println("\n✓ Existing SuperGrok OAuth credentials are ready")
			fmt.Println("  Engine: xai-oauth")
			fmt.Printf("  Model: %s\n", oauthCfg.Model)
			fmt.Printf("  Source: %s\n", probe.Source)
			return nil
		}
		fmt.Printf("Existing credentials not ready (%s): %s\n", probe.Classification, probe.Message)
		fmt.Println("Starting device-code login...")
	} else {
		fmt.Println("No local credentials; starting device-code login...")
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	pkgCfg := client.Config()
	loginCtx, loginCancel := context.WithTimeout(cmd.Context(), 15*time.Minute)
	defer loginCancel()

	creds, err := xaioauth.LoginDeviceCode(loginCtx, httpClient, pkgCfg, func(dc xaioauth.DeviceCodeResponse) {
		verify := dc.VerificationURIComplete
		if verify == "" {
			verify = dc.VerificationURI
		}
		fmt.Println()
		fmt.Println("Open this URL in a browser and approve access:")
		fmt.Printf("  %s\n", verify)
		fmt.Printf("  User code: %s\n", dc.UserCode)
		fmt.Println()
		fmt.Println("Waiting for approval...")
	})
	if err != nil {
		return fmt.Errorf("device login failed: %w", err)
	}
	if err := ts.SetCredentials(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	fmt.Println("✓ Tokens saved")

	probe := client.RunHealthProbe(probeCtx)
	if probe.Classification == xaioauth.ProbeTierForbidden {
		fmt.Println("\n⚠️  OAuth login succeeded but inference is tier-gated (HTTP 403).")
		fmt.Println("   Fall back to metered API: set engine=api, provider=xai, and xai_api_key.")
	} else if probe.Classification != xaioauth.ProbeReady {
		fmt.Printf("\n⚠️  Login saved but probe status=%s: %s\n", probe.Classification, probe.Message)
	} else {
		fmt.Println("✓ Health probe passed")
	}

	if err := cfg.SetEngine("xai-oauth"); err != nil {
		return err
	}
	cfg.XAIOAuth = oauthCfg
	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("  Engine: xai-oauth")
	fmt.Printf("  Model: %s\n", oauthCfg.Model)
	fmt.Printf("  Credential path: %s\n", client.Config().CredentialPath)
	fmt.Println("\ncodeNERD will use your SuperGrok subscription for LLM calls.")
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

	case "xai-oauth":
		fmt.Println("Backend: SuperGrok OAuth (subscription)")
		oauthCfg := cfg.GetXAIOAuthConfig()
		fmt.Printf("  Model: %s\n", oauthCfg.Model)
		fmt.Printf("  Timeout: %ds\n", oauthCfg.Timeout)
		fmt.Printf("  Max concurrent calls: %d\n", oauthCfg.MaxConcurrentCalls)
		fmt.Printf("  Effective scheduler ceiling: %d\n", cfg.GetEffectiveMaxConcurrentAPICalls())
		client := xaioauth.NewClientFromUserConfig(oauthCfg)
		fmt.Printf("  Credential path: %s\n", client.Config().CredentialPath)
		probeCtx, cancel := context.WithTimeout(cmd.Context(), 45*time.Second)
		defer cancel()
		probe := client.RunHealthProbe(probeCtx)
		fmt.Printf("  Probe: %s\n", probe.Classification)
		fmt.Printf("  Message: %s\n", probe.Message)
		if probe.Source != "" {
			fmt.Printf("  Source: %s\n", probe.Source)
		}
		if !probe.ExpiresAt.IsZero() {
			fmt.Printf("  Token expires: %s\n", probe.ExpiresAt.Format(time.RFC3339))
		}
		if probe.RawError != "" && probe.Classification != xaioauth.ProbeReady {
			fmt.Printf("  Detail: %s\n", probe.RawError)
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
