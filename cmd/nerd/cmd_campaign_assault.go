package main

import (
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/campaign"

	"github.com/spf13/cobra"
)

// `nerd campaign assault` — Cobra parity with the chat `/campaign assault`.
//
// Adversarial assault was reachable only from chat, and the Cobra tree stopped
// at lifecycle verbs. That is the wrong asymmetry for the one campaign type
// designed to be run unattended over hours: chat needs a live terminal session,
// so the long-horizon repo sweep could not be scripted, scheduled or run in CI.
//
// Parity here means every knob of campaign.AssaultConfig is settable from
// flags. TestCampaignAssaultFlags_CoverEveryAssaultConfigField enforces that,
// so a new config field cannot ship without an operator-facing flag.

var (
	assaultScope         string
	assaultInclude       []string
	assaultExclude       []string
	assaultBatchSize     int
	assaultCycles        int
	assaultTimeoutSecs   int
	assaultStages        []string
	assaultCommand       string
	assaultNoNemesis     bool
	assaultMaxRemedy     int
	assaultContextBudget int
	assaultLogMaxBytes   int64
	assaultDryRun        bool
)

// campaignStartOverride hands a pre-built campaign to runCampaignStart in place
// of LLM decomposition. Guarded because Cobra commands are also invoked from
// tests, and a leaked override would silently replace an unrelated campaign.
var (
	campaignStartOverrideMu sync.Mutex
	campaignStartOverride   *campaign.Campaign
)

func setCampaignStartOverride(c *campaign.Campaign) {
	campaignStartOverrideMu.Lock()
	defer campaignStartOverrideMu.Unlock()
	campaignStartOverride = c
}

// takeCampaignStartOverride consumes the override so it can never apply twice.
func takeCampaignStartOverride() *campaign.Campaign {
	campaignStartOverrideMu.Lock()
	defer campaignStartOverrideMu.Unlock()
	c := campaignStartOverride
	campaignStartOverride = nil
	return c
}

var campaignAssaultCmd = &cobra.Command{
	Use:   "assault [scope]",
	Args:  cobra.MaximumNArgs(1),
	Short: "Run an adversarial assault campaign over the repository",
	Long: `Run a durable, batched adversarial assault campaign.

The assault discovers targets, runs the configured stages against each of them
in batches, persists every result under .nerd/campaigns/<id>/assault, triages
the failures and generates remediation tasks. It is deterministic: no LLM
decomposition, so a large repository does not explode into per-file tasks.

Scope controls how targets are grouped:
  repo       the whole repository as a single target
  module     coarse directory grouping (internal, cmd, ...)
  subsystem  medium grouping (internal/core, cmd/nerd, ...)   [default]
  package    individual Go packages

Stages accept: go_test, go_test_race, go_vet, nemesis_review, and command
(paired with --command, where {{target}} is substituted).

Examples:
  nerd campaign assault
  nerd campaign assault package --include internal/core --cycles 3
  nerd campaign assault repo --stages go_test,go_vet --timeout 1800
  nerd campaign assault --stages command --command "golangci-lint run {{target}}"
  nerd campaign assault --dry-run`,
	RunE: runCampaignAssault,
}

func init() {
	f := campaignAssaultCmd.Flags()
	f.StringVar(&assaultScope, "scope", "", "Target grouping: repo|module|subsystem|package (also accepted positionally)")
	f.StringArrayVar(&assaultInclude, "include", nil, "Only assault targets under these workspace-relative prefixes")
	f.StringArrayVar(&assaultExclude, "exclude", nil, "Skip targets under these workspace-relative prefixes")
	f.IntVar(&assaultBatchSize, "batch-size", 0, "Targets per batch task (default 10)")
	f.IntVar(&assaultCycles, "cycles", 0, "Repeat the whole sweep N times, max 10 (default 1)")
	f.IntVar(&assaultTimeoutSecs, "timeout", 0, "Per-stage timeout in seconds (default 900)")
	f.StringSliceVar(&assaultStages, "stages", nil, "Stages to run: go_test,go_test_race,go_vet,nemesis_review,command")
	f.StringVar(&assaultCommand, "command", "", "Command template for the `command` stage; {{target}} is substituted")
	f.BoolVar(&assaultNoNemesis, "no-nemesis", false, "Disable the adversarial nemesis review stage")
	f.IntVar(&assaultMaxRemedy, "max-remediation", 0, "Cap on generated remediation tasks (default 25)")
	f.IntVar(&assaultContextBudget, "context-budget", 0, "Token budget for campaign context (default 200000)")
	f.Int64Var(&assaultLogMaxBytes, "log-max-bytes", 0, "Cap on captured output per stage run (default 2MiB)")
	f.BoolVar(&assaultDryRun, "dry-run", false, "Print the resolved configuration and phase plan without running")

	// runCampaignStart reads these two by name from the invoking command. They
	// are meaningless for a deterministic assault plan, but their absence made
	// the shared start path print an empty campaign type.
	f.StringArray("docs", nil, "unused by assault; present for the shared campaign start path")
	f.String("type", "adversarial_assault", "unused by assault; present for the shared campaign start path")
	_ = f.MarkHidden("docs")
	_ = f.MarkHidden("type")

	campaignCmd.AddCommand(campaignAssaultCmd)
}

// assaultConfigFromFlags builds the config from flags and the optional
// positional scope. It is separate from the RunE so the parity test can call it.
func assaultConfigFromFlags(args []string) (campaign.AssaultConfig, error) {
	cfg := campaign.DefaultAssaultConfig()

	scope := assaultScope
	if scope == "" && len(args) == 1 {
		scope = args[0]
	}
	if scope != "" {
		parsed, err := parseAssaultScope(scope)
		if err != nil {
			return cfg, err
		}
		cfg.Scope = parsed
	}

	cfg.Include = assaultInclude
	cfg.Exclude = assaultExclude
	if assaultBatchSize > 0 {
		cfg.BatchSize = assaultBatchSize
	}
	if assaultCycles > 0 {
		cfg.Cycles = assaultCycles
	}
	if assaultTimeoutSecs > 0 {
		cfg.DefaultTimeoutSeconds = assaultTimeoutSecs
	}
	if assaultMaxRemedy > 0 {
		cfg.MaxRemediationTasks = assaultMaxRemedy
	}
	if assaultContextBudget > 0 {
		cfg.ContextBudget = assaultContextBudget
	}
	if assaultLogMaxBytes > 0 {
		cfg.LogMaxBytes = assaultLogMaxBytes
	}

	if len(assaultStages) > 0 {
		stages, err := parseAssaultStages(assaultStages, assaultCommand, assaultTimeoutSecs)
		if err != nil {
			return cfg, err
		}
		cfg.Stages = stages
	}

	cfg.EnableNemesis = !assaultNoNemesis
	if assaultNoNemesis {
		filtered := make([]campaign.AssaultStage, 0, len(cfg.Stages))
		for _, s := range cfg.Stages {
			if s.Kind != campaign.AssaultStageNemesisReview {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			return cfg, fmt.Errorf("--no-nemesis removed every configured stage; add --stages go_test")
		}
		cfg.Stages = filtered
	}

	return cfg.Normalize(), nil
}

func parseAssaultScope(raw string) (campaign.AssaultScope, error) {
	switch strings.ToLower(strings.TrimPrefix(strings.TrimSpace(raw), "/")) {
	case "repo", "repository":
		return campaign.AssaultScopeRepo, nil
	case "module":
		return campaign.AssaultScopeModule, nil
	case "subsystem":
		return campaign.AssaultScopeSubsystem, nil
	case "package", "pkg":
		return campaign.AssaultScopePackage, nil
	default:
		return "", fmt.Errorf("unknown scope %q (want repo, module, subsystem or package)", raw)
	}
}

func parseAssaultStages(names []string, command string, timeoutSecs int) ([]campaign.AssaultStage, error) {
	stages := make([]campaign.AssaultStage, 0, len(names))
	for _, raw := range names {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		stage := campaign.AssaultStage{Name: name, Repeat: 1, TimeoutSeconds: timeoutSecs}
		switch strings.TrimPrefix(name, "/") {
		case "go_test", "test":
			stage.Kind = campaign.AssaultStageGoTest
		case "go_test_race", "race":
			stage.Kind = campaign.AssaultStageGoTestRace
		case "go_vet", "vet":
			stage.Kind = campaign.AssaultStageGoVet
		case "nemesis_review", "nemesis":
			stage.Kind = campaign.AssaultStageNemesisReview
		case "command", "cmd":
			if strings.TrimSpace(command) == "" {
				return nil, fmt.Errorf("the `command` stage needs --command (use {{target}} for the target)")
			}
			stage.Kind = campaign.AssaultStageCommand
			stage.Command = command
		default:
			return nil, fmt.Errorf("unknown stage %q", raw)
		}
		stages = append(stages, stage)
	}
	if len(stages) == 0 {
		return nil, fmt.Errorf("--stages was given but resolved to no stages")
	}
	return stages, nil
}

func runCampaignAssault(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	cfg, err := assaultConfigFromFlags(args)
	if err != nil {
		return err
	}

	cwd := campaignWorkspace()
	camp := campaign.NewAdversarialAssaultCampaign(cwd, cfg)

	fmt.Printf("⚔️  Adversarial assault: %s\n", camp.Title)
	fmt.Printf("   scope=%s batch=%d cycles=%d timeout=%ds remediation_cap=%d\n",
		strings.TrimPrefix(string(cfg.Scope), "/"), cfg.BatchSize, cfg.Cycles,
		cfg.DefaultTimeoutSeconds, cfg.MaxRemediationTasks)
	stageNames := make([]string, 0, len(cfg.Stages))
	for _, s := range cfg.Stages {
		stageNames = append(stageNames, strings.TrimPrefix(string(s.Kind), "/"))
	}
	fmt.Printf("   stages=%s\n", strings.Join(stageNames, ","))
	if len(cfg.Include) > 0 {
		fmt.Printf("   include=%s\n", strings.Join(cfg.Include, ","))
	}
	if len(cfg.Exclude) > 0 {
		fmt.Printf("   exclude=%s\n", strings.Join(cfg.Exclude, ","))
	}

	if assaultDryRun {
		fmt.Println("\n   Phase plan:")
		for i, phase := range camp.Phases {
			fmt.Printf("     %d. %s (%d seed task(s))\n", i, phase.Name, len(phase.Tasks))
		}
		fmt.Println("\n   --dry-run: nothing was executed.")
		return nil
	}

	// Reuse the campaign start boot verbatim. The goal string is only used for
	// display and kernel facts here, because the plan is already built.
	setCampaignStartOverride(camp)
	defer setCampaignStartOverride(nil)
	return runCampaignStart(cmd, []string{camp.Goal})
}
