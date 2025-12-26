package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/usage"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var assaultPathTokenRE = regexp.MustCompile(`(?i)([a-z0-9_.-]+[\\/][a-z0-9_.\\/-]+)`)
var assaultWindowsAbsPathRE = regexp.MustCompile(`(?i)([a-z]:\\[^\\s"']+)`)

func (m Model) startAssaultCampaign(args []string) tea.Cmd {
	return func() tea.Msg {
		if m.kernel == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: kernel not initialized")}
		}
		if m.client == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: LLM client not initialized")}
		}
		if m.shardMgr == nil {
			return campaignErrorMsg{err: fmt.Errorf("system not ready: shard manager not initialized")}
		}

		cfg, err := parseAssaultArgs(args)
		if err != nil {
			return campaignErrorMsg{err: err}
		}

		// Use shutdown context if available
		var ctx context.Context
		var cancel context.CancelFunc
		if m.shutdownCtx != nil {
			ctx, cancel = context.WithTimeout(m.shutdownCtx, config.GetLLMTimeouts().ShardExecutionTimeout)
		} else {
			ctx, cancel = context.WithTimeout(context.Background(), config.GetLLMTimeouts().ShardExecutionTimeout)
		}
		if m.usageTracker != nil {
			ctx = usage.NewContext(ctx, m.usageTracker)
		}
		defer cancel()

		m.ReportStatus("Creating adversarial assault campaign...")

		// JIT prompt provider for campaign roles (if compiler available)
		var promptProvider campaign.PromptProvider
		if m.jitCompiler != nil {
			if pa, err := articulation.NewPromptAssemblerWithJIT(m.kernel, m.jitCompiler); err == nil {
				jitCfg := config.DefaultJITConfig()
				if m.Config != nil {
					jitCfg = m.Config.GetEffectiveJITConfig()
				}
				pa.SetJITBudgets(jitCfg.TokenBudget, jitCfg.ReservedTokens, jitCfg.SemanticTopK)
				pa.EnableJIT(jitCfg.Enabled)
				promptProvider = &campaignJITProvider{assembler: pa}
			}
		}

		camp := campaign.NewAdversarialAssaultCampaign(m.workspace, cfg)

		// Larger buffers prevent backpressure during adversarial campaigns
		progressChan := make(chan campaign.Progress, 100)
		eventChan := make(chan campaign.OrchestratorEvent, 200)

		orch := campaign.NewOrchestrator(campaign.OrchestratorConfig{
			Workspace:        m.workspace,
			Kernel:           m.kernel,
			LLMClient:        m.client,
			ShardManager:     m.shardMgr,
			Executor:         m.executor,
			VirtualStore:     m.virtualStore,
			ProgressChan:     progressChan,
			EventChan:        eventChan,
			AutoReplan:       true,
			CheckpointOnFail: true,
			DisableTimeouts:  true,
			MaxParallelTasks: 1,
		})
		if promptProvider != nil {
			orch.SetPromptProvider(promptProvider)
		}

		if err := orch.SetCampaign(camp); err != nil {
			return campaignErrorMsg{err: fmt.Errorf("failed to set assault campaign: %w", err)}
		}

		m.ReportStatus("Assault campaign started")
		return campaignStartedMsg{
			campaign:     camp,
			orch:         orch,
			progressChan: progressChan,
			eventChan:    eventChan,
		}
	}
}

func parseAssaultArgs(args []string) (campaign.AssaultConfig, error) {
	cfg := campaign.DefaultAssaultConfig()

	// Positional scope (optional) then positional include paths.
	positionalIncludes := make([]string, 0)

	for i := 0; i < len(args); i++ {
		a := strings.TrimSpace(args[i])
		if a == "" {
			continue
		}

		switch {
		case a == "repo":
			cfg.Scope = campaign.AssaultScopeRepo
		case a == "module":
			cfg.Scope = campaign.AssaultScopeModule
		case a == "subsystem":
			cfg.Scope = campaign.AssaultScopeSubsystem
		case a == "package":
			cfg.Scope = campaign.AssaultScopePackage

		case strings.HasPrefix(a, "--batch="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--batch="))
			if err != nil {
				return cfg, fmt.Errorf("invalid --batch: %w", err)
			}
			cfg.BatchSize = n
		case a == "--batch" && i+1 < len(args):
			i++
			n, err := strconv.Atoi(strings.TrimSpace(args[i]))
			if err != nil {
				return cfg, fmt.Errorf("invalid --batch: %w", err)
			}
			cfg.BatchSize = n

		case strings.HasPrefix(a, "--cycles="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--cycles="))
			if err != nil {
				return cfg, fmt.Errorf("invalid --cycles: %w", err)
			}
			cfg.Cycles = n

		case strings.HasPrefix(a, "--timeout="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--timeout="))
			if err != nil {
				return cfg, fmt.Errorf("invalid --timeout: %w", err)
			}
			cfg.DefaultTimeoutSeconds = n

		case a == "--race":
			cfg.Stages = append(cfg.Stages, campaign.AssaultStage{Kind: campaign.AssaultStageGoTestRace, Name: "go test -race", Repeat: 1})

		case a == "--vet":
			cfg.Stages = append(cfg.Stages, campaign.AssaultStage{Kind: campaign.AssaultStageGoVet, Name: "go vet", Repeat: 1})

		case a == "--no-nemesis":
			cfg.EnableNemesis = false
			// Filter out nemesis stage if it exists in defaults.
			filtered := make([]campaign.AssaultStage, 0, len(cfg.Stages))
			for _, st := range cfg.Stages {
				if st.Kind == campaign.AssaultStageNemesisReview {
					continue
				}
				filtered = append(filtered, st)
			}
			cfg.Stages = filtered

		case strings.HasPrefix(a, "--include="):
			val := strings.TrimPrefix(a, "--include=")
			positionalIncludes = append(positionalIncludes, splitCSV(val)...)
		case a == "--include" && i+1 < len(args):
			i++
			positionalIncludes = append(positionalIncludes, splitCSV(args[i])...)

		case strings.HasPrefix(a, "--exclude="):
			val := strings.TrimPrefix(a, "--exclude=")
			cfg.Exclude = append(cfg.Exclude, splitCSV(val)...)
		case a == "--exclude" && i+1 < len(args):
			i++
			cfg.Exclude = append(cfg.Exclude, splitCSV(args[i])...)

		case strings.HasPrefix(a, "--max-remediation="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--max-remediation="))
			if err != nil {
				return cfg, fmt.Errorf("invalid --max-remediation: %w", err)
			}
			cfg.MaxRemediationTasks = n

		default:
			positionalIncludes = append(positionalIncludes, a)
		}
	}

	if len(positionalIncludes) > 0 {
		cfg.Include = append(cfg.Include, positionalIncludes...)
	}

	cfg = cfg.Normalize()
	return cfg, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isAssaultRequest(input string, intent perception.Intent) bool {
	if intent.Verb == "/assault" {
		return true
	}
	if intent.Verb == "/campaign" {
		lower := strings.ToLower(input)
		return strings.Contains(lower, "assault") ||
			strings.Contains(lower, "soak test") ||
			strings.Contains(lower, "stress test") ||
			strings.Contains(lower, "torture test") ||
			strings.Contains(lower, "gauntlet") ||
			strings.Contains(lower, "adversarial")
	}
	return false
}

func assaultArgsFromNaturalLanguage(workspace, input string, intent perception.Intent) ([]string, bool) {
	if !isAssaultRequest(input, intent) {
		return nil, false
	}

	lower := strings.ToLower(input)
	args := make([]string, 0, 8)

	switch {
	case strings.Contains(lower, "package"):
		args = append(args, "package")
	case strings.Contains(lower, "module"):
		args = append(args, "module")
	case strings.Contains(lower, "subsystem") || strings.Contains(lower, "folder") || strings.Contains(lower, "directory"):
		args = append(args, "subsystem")
	case strings.Contains(lower, "scope repo") || strings.Contains(lower, "repo scope") || strings.Contains(lower, "single target") || strings.Contains(lower, "one target"):
		args = append(args, "repo")
	}

	includes := assaultIncludesFromText(workspace, input, intent)
	args = append(args, includes...)

	// Optional flags inferred from natural language.
	if strings.Contains(lower, "-race") || strings.Contains(lower, "race detector") || strings.Contains(lower, "data race") {
		args = append(args, "--race")
	}
	if strings.Contains(lower, "go vet") || strings.Contains(lower, " vet ") || strings.HasSuffix(lower, " vet") {
		args = append(args, "--vet")
	}
	if strings.Contains(lower, "no nemesis") || strings.Contains(lower, "without nemesis") || strings.Contains(lower, "skip nemesis") {
		args = append(args, "--no-nemesis")
	}

	return args, true
}

func assaultIncludesFromText(workspace, input string, intent perception.Intent) []string {
	candidates := []string{
		strings.TrimSpace(intent.Target),
		strings.TrimSpace(intent.Constraint),
		strings.TrimSpace(input),
	}

	raw := make([]string, 0, 8)
	for _, c := range candidates {
		if c == "" || strings.EqualFold(c, "none") {
			continue
		}
		raw = append(raw, assaultWindowsAbsPathRE.FindAllString(c, -1)...)
		raw = append(raw, assaultPathTokenRE.FindAllString(c, -1)...)
	}

	// If the LLM gave a clean single-token target (e.g., "internal"), accept it too.
	// Avoid ingesting descriptive multi-word targets like "soak test internal/core".
	if t := strings.TrimSpace(intent.Target); t != "" && !strings.EqualFold(t, "none") {
		if !strings.ContainsAny(t, " \t\r\n") && (strings.ContainsAny(t, `/\`) || t == "internal" || t == "cmd" || t == "pkg") {
			raw = append(raw, t)
		}
	}

	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		inc := normalizeAssaultInclude(workspace, r)
		if inc == "" {
			continue
		}
		if isGenericAssaultTarget(inc) {
			continue
		}
		if _, ok := seen[inc]; ok {
			continue
		}
		seen[inc] = struct{}{}
		out = append(out, inc)
	}
	return out
}

func normalizeAssaultInclude(workspace, raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, "\"'`")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Strip common go list globs.
	s = strings.TrimSuffix(s, "/...")
	s = strings.TrimSuffix(s, "\\...")

	// Normalize to OS path for Rel/Dir, then back to slash form.
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, ".\\")

	osPath := filepath.Clean(strings.ReplaceAll(s, "/", string(filepath.Separator)))
	if filepath.IsAbs(osPath) {
		rel, err := filepath.Rel(workspace, osPath)
		if err != nil {
			return ""
		}
		osPath = rel
	}

	// Prefer directory prefixes for includes.
	if ext := filepath.Ext(osPath); ext != "" {
		osPath = filepath.Dir(osPath)
	}
	osPath = filepath.Clean(osPath)
	if osPath == "." {
		return ""
	}

	return filepath.ToSlash(osPath)
}

func isGenericAssaultTarget(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	switch t {
	case "", "none", ".", "./", "repo", "repository", "codebase", "project", "everything", "all":
		return true
	}
	return strings.Contains(t, "whole repo") ||
		strings.Contains(t, "entire repo") ||
		strings.Contains(t, "entire codebase") ||
		strings.Contains(t, "whole codebase")
}
