package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/session"
	"codenerd/internal/world"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// NewOrchestrator creates a new campaign orchestrator.
func NewOrchestrator(cfg OrchestratorConfig) (*Orchestrator, error) {
	timer := logging.StartTimer(logging.CategoryCampaign, "NewOrchestrator")
	defer timer.Stop()

	if err := validateOrchestratorConfig(cfg); err != nil {
		logging.Get(logging.CategoryCampaign).Error("Invalid campaign orchestrator config: %v", err)
		_ = cfg.NorthstarObserver.Close()
		return nil, err
	}

	nerdDir := filepath.Join(cfg.Workspace, ".nerd")

	policy, err := cfg.Campaign.Resolve()
	if err != nil {
		logging.Get(logging.CategoryCampaign).Error("Invalid campaign policy: %v", err)
		_ = cfg.NorthstarObserver.Close()
		return nil, errors.Join(ErrInvalidConfig, err)
	}
	applyOrchestratorDefaults(&cfg)

	logging.Campaign("Initializing campaign orchestrator for workspace: %s", cfg.Workspace)
	logging.CampaignDebug("Orchestrator policy: %+v", policy.Section)

	// pauseCh starts CLOSED (state = running/resumed) so the runPhase loop's
	// `select { case <-o.pauseCh: ... }` returns immediately when not paused.
	initialPauseCh := make(chan struct{})
	close(initialPauseCh)

	o := &Orchestrator{
		kernel:          cfg.Kernel,
		llmClient:       cfg.LLMClient,
		planningClient:  cfg.PlanningLLMClient,
		shardMgr:        cfg.ShardManager,
		taskExecutor:    cfg.TaskExecutor,
		executor:        cfg.Executor,
		virtualStore:    cfg.VirtualStore,
		workspace:       cfg.Workspace,
		nerdDir:         nerdDir,
		progressChan:    cfg.ProgressChan,
		eventChan:       cfg.EventChan,
		taskResults:     make(map[string]string),
		taskResultOrder: make([]string, 0),
		config:          cfg,
		policy:          policy,
		// Every door's tasks get their specialist's knowledge: the provider
		// needs only the workspace, so no caller has to remember it.
		specialistKnowledgeProvider: NewLocalSpecialistKnowledgeProvider(cfg.Workspace),
		writeSetLocks:               newWriteSetLockManager(cfg.Workspace),
		pauseCh:                     initialPauseCh,
	}

	// Initialize sub-components. Planning (decomposition and every replan) runs
	// on the planning client; paging and the document fallback on LLMClient.
	o.contextPager = NewContextPager(cfg.Kernel, cfg.LLMClient, cfg.ContextBudget)
	o.checkpoint = NewCheckpointRunner(cfg.Executor, cfg.TaskExecutor, cfg.Workspace, cfg.Kernel)
	o.replanner = NewReplanner(cfg.Kernel, cfg.PlanningLLMClient, cfg.Workspace)
	o.decomposer = NewDecomposer(cfg.Kernel, cfg.PlanningLLMClient, cfg.Workspace)
	if cfg.ShardManager != nil {
		o.decomposer.SetShardLister(cfg.ShardManager) // Enable shard-aware planning
	}
	wireIntelligenceComponents(o, cfg)
	defaultWireIntelligence(o, cfg)
	// The campaign section is the gatherer's policy, whether the caller
	// built the gatherer or defaultWireIntelligence did. A gatherer left on
	// the package defaults would ignore campaign.intelligence.
	if o.intelligenceGatherer != nil {
		o.intelligenceGatherer.WithConfig(IntelligenceConfigFromPolicy(policy))
	}

	o.replanner.SetContextLimits(policy)
	o.checkpoint.SetCommandTimeout(policy.CheckpointCommandTimeout)
	o.refreshRiskGateState()
	o.publishPolicyParams()

	logging.Campaign("Orchestrator initialized with maxParallelTasks=%d", o.policy.MaxParallelTasks)

	return o, nil
}

// isNilTaskExecutor reports whether te is unusable. It catches the typed-nil
// case (`var x *session.JITExecutor; cfg.TaskExecutor = x`) as well as a plain
// nil interface: a typed nil is non-nil as an interface but panics or misbehaves
// on first use, and it is exactly what an accessor like
// CampaignRunnerShard.TaskExecutor() returns when a concrete pointer field was
// never assigned.
func isNilTaskExecutor(te session.TaskExecutor) bool {
	return northstar.IsNil(te)
}

func validateOrchestratorConfig(cfg OrchestratorConfig) error {
	var missing []string
	if northstar.IsNil(cfg.Kernel) {
		missing = append(missing, "kernel")
	}
	if northstar.IsNil(cfg.LLMClient) {
		missing = append(missing, "llm_client")
	}
	if northstar.IsNil(cfg.PlanningLLMClient) {
		missing = append(missing, "planning_llm_client")
	}
	if northstar.IsNil(cfg.Executor) {
		missing = append(missing, "executor")
	}
	if cfg.VirtualStore == nil {
		missing = append(missing, "virtual_store")
	}
	// TaskExecutor is required, not "TaskExecutor OR ShardManager".
	//
	// The OR was a lie about what the orchestrator can do. ShardManager is only
	// used for monitoring (GetActiveShards, GetBackpressureStatus); every path
	// that actually runs work goes through spawnTask -> session.TaskExecutor,
	// which returns "taskExecutor not initialized" when it is nil. A campaign
	// built shard-only therefore cannot execute a single task, and — worse —
	// runShardValidationCheckpoint and runNemesisGauntletCheckpoint have no
	// executor either, so before commit 20a90c79 they returned PASS for
	// verifications that never ran.
	//
	// Source-level auditing was not enough on its own. All five production call
	// sites DO write the field, but three of them write a value that can be nil
	// at runtime: cmd/nerd/chat/campaign.go and campaign_assault.go pass
	// m.taskExecutor, and internal/shards/system/campaign_runner.go passes
	// s.TaskExecutor(), which is nil until the boot closure calls
	// SetTaskExecutor. A field that is present but nil looked identical to a
	// correct wiring. Refusing construction makes it loud at the call site.
	if isNilTaskExecutor(cfg.TaskExecutor) {
		missing = append(missing, "task_executor")
	}
	if len(missing) > 0 {
		return errors.Join(
			ErrInvalidConfig,
			ErrNilDependency,
			fmt.Errorf("missing required dependencies: %s", strings.Join(missing, ", ")),
		)
	}

	var invalid []string
	// A campaign reads each task's kernel verdict (spawnTask): an executor
	// that returns only prose would report an /unverified turn as success
	// (external audit F2).
	if _, ok := cfg.TaskExecutor.(session.ObservedTaskExecutor); !ok {
		invalid = append(invalid, fmt.Sprintf("task_executor %T must return an observed result (session.ObservedTaskExecutor)", cfg.TaskExecutor))
	}
	if strings.TrimSpace(cfg.Workspace) == "" {
		invalid = append(invalid, "workspace must be non-empty")
	}
	if cfg.ContextBudget < 0 {
		invalid = append(invalid, "context_budget must be >= 0")
	}
	if cfg.RiskGateThreshold < 0 {
		invalid = append(invalid, "risk_gate_threshold must be >= 0")
	}
	if cfg.RiskIntelligenceTimeout < 0 {
		invalid = append(invalid, "risk_intelligence_timeout must be >= 0")
	}

	if len(invalid) > 0 {
		return errors.Join(
			ErrInvalidConfig,
			fmt.Errorf("invalid configuration values: %s", strings.Join(invalid, "; ")),
		)
	}
	return nil
}

// SetPromptProvider wires the provider that compiles every campaign role's
// system prompt into the orchestrator and its components. Until it is set,
// each campaign model call reports types.ErrRolePromptUnavailable.
func (o *Orchestrator) SetPromptProvider(provider PromptProvider) {
	o.promptProvider = provider
	if o.decomposer != nil {
		o.decomposer.SetPromptProvider(provider)
	}
	if o.replanner != nil {
		o.replanner.SetPromptProvider(provider)
	}
	if o.contextPager != nil {
		o.contextPager.SetPromptProvider(provider)
	}
}

func applyOrchestratorDefaults(cfg *OrchestratorConfig) {
	// No clock of the campaign's own: a caller's limit is its context's
	// deadline (nerd campaign --timeout sets it). Until 2026-09-19 a zero
	// limit became 4 hours per campaign and 30 minutes per task; after that
	// the two config fields stayed as a caller's optional limits that no
	// caller set, a second spelling of the context deadline, and were
	// removed. A campaign stops when it stops making progress, and each
	// model request is bounded by its client. Every campaign knob is
	// cfg.Campaign's -- the user's config -- resolved in NewOrchestrator.
	if cfg.RiskGateThreshold <= 0 {
		cfg.RiskGateThreshold = defaultRiskGateThreshold
	}
	if cfg.RiskGateThreshold < defaultRiskGateThreshold {
		cfg.RiskGateThreshold = defaultRiskGateThreshold
	}
	if cfg.RiskGateMode == "" {
		cfg.RiskGateMode = RiskGateModeAuto
	}
	if cfg.AdvisoryGateToggle == "" {
		cfg.AdvisoryGateToggle = RiskGateToggleAuto
	}
	if cfg.EdgeGateToggle == "" {
		cfg.EdgeGateToggle = RiskGateToggleAuto
	}
	if cfg.NorthstarGateToggle == "" {
		cfg.NorthstarGateToggle = RiskGateToggleAuto
	}
	if cfg.RiskIntelligenceTimeout <= 0 {
		cfg.RiskIntelligenceTimeout = defaultRiskIntelligenceTimeout
	}
	// Default on for deterministic auto-wiring in the common zero-config path.
	if !cfg.EnableRiskAutoWiring &&
		cfg.RiskGateMode == RiskGateModeAuto &&
		cfg.CampaignRiskOverride == nil &&
		len(cfg.TaskRiskOverrides) == 0 {
		cfg.EnableRiskAutoWiring = true
	}
	// Default on in zero-config path.
	if !cfg.GlobalRiskGate &&
		cfg.RiskGateMode == RiskGateModeAuto &&
		cfg.CampaignRiskOverride == nil {
		cfg.GlobalRiskGate = true
	}
}

// defaultWireIntelligence builds the intelligence components every caller was
// hand-assembling, when the ingredients are already present.
//
// IntelligenceGatherer and EdgeCaseDetector need exactly two things a Cortex
// boot always has: a *core.RealKernel and a workspace path. Everything else
// they consume (world scanner, holographic provider) is constructed from those
// two. Yet every construction site — cmd_campaign.go start, cmd_campaign.go
// resume, chat campaign, chat assault, the campaign runner shard — repeated the
// same twenty lines, and any caller that forgot silently got a campaign with no
// pre-planning intelligence and, because resolveRiskGateEnabled keys off
// availability, no edge risk gate either. A missing sensor is invisible: the
// campaign runs, it is just less informed and less gated.
//
// Explicit config still wins; this only fills a nil. A mock kernel (tests) is
// not a RealKernel, so unit tests keep their zero-intelligence orchestrator.
func defaultWireIntelligence(o *Orchestrator, cfg OrchestratorConfig) {
	realKernel, ok := cfg.Kernel.(*core.RealKernel)
	if !ok || realKernel == nil || strings.TrimSpace(cfg.Workspace) == "" {
		return
	}

	var scanner *world.Scanner
	if o.intelligenceGatherer == nil || o.edgeCaseDetector == nil {
		scanner = world.NewScanner()
	}

	if o.intelligenceGatherer == nil {
		// Consultation stays nil: advisor consultation needs a spawner the
		// orchestrator does not own, and the gatherer treats it as optional.
		gatherer := NewIntelligenceGatherer(
			cfg.Workspace,
			realKernel,
			scanner,
			world.NewHolographicProvider(realKernel, cfg.Workspace),
			nil, nil, nil, nil, nil,
		)
		o.intelligenceGatherer = gatherer
		if o.decomposer != nil {
			o.decomposer.SetIntelligenceGatherer(gatherer)
		}
		logging.Campaign("IntelligenceGatherer default-wired from kernel + workspace")
	}

	if o.edgeCaseDetector == nil {
		o.edgeCaseDetector = NewEdgeCaseDetector(realKernel, scanner)
		logging.Campaign("EdgeCaseDetector default-wired from kernel + workspace")
	}
}

func wireIntelligenceComponents(o *Orchestrator, cfg OrchestratorConfig) {
	// Wire intelligence integration components
	o.intelligenceGatherer = cfg.IntelligenceGatherer
	o.advisoryBoard = cfg.AdvisoryBoard
	o.edgeCaseDetector = cfg.EdgeCaseDetector

	// Take the observer from config when supplied. This line used to read
	// `o.configuredNorthstarObserver = o.northstarObserver`, copying a field
	// nothing ever populated -- so it was always nil, and the protected-surface
	// risk gate refused every campaign touching internal/core and friends. The
	// setter existed; no caller did.
	if cfg.NorthstarObserver != nil {
		o.northstarObserver = cfg.NorthstarObserver
	}
	o.configuredNorthstarObserver = o.northstarObserver

	// Step 0 of decomposition: the gatherer's report goes into the planner's
	// prompt (Decompose). The advisory board and edge-case detector serve the
	// risk preflight, where their verdicts gate the run.
	if cfg.IntelligenceGatherer != nil {
		o.decomposer.SetIntelligenceGatherer(cfg.IntelligenceGatherer)
		logging.CampaignDebug("IntelligenceGatherer wired into decomposer")
	}
}
