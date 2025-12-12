package chat

import (
	"codenerd/internal/articulation"
	"codenerd/internal/campaign"
	"codenerd/internal/usage"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

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
			ctx, cancel = context.WithTimeout(m.shutdownCtx, 2*time.Minute)
		} else {
			ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
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
				promptProvider = &campaignJITProvider{assembler: pa}
			}
		}

		camp := campaign.NewAdversarialAssaultCampaign(m.workspace, cfg)

		progressChan := make(chan campaign.Progress, 10)
		eventChan := make(chan campaign.OrchestratorEvent, 20)

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

