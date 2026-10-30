package prompt_evolution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// EvolverConfig configures the prompt evolution system.
type EvolverConfig struct {
	// MinFailuresForEvolution is the minimum unconsumed failures a group
	// needs before generation may be bought for it.
	MinFailuresForEvolution int `json:"min_failures_for_evolution"`

	// EvolutionInterval is the minimum time between evolution cycles
	EvolutionInterval time.Duration `json:"evolution_interval"`

	// MaxAtomsPerEvolution is the maximum atoms stored from one failure
	// group. MaxCycleAtoms bounds the whole cycle.
	MaxAtomsPerEvolution int `json:"max_atoms_per_evolution"`

	// ConfidenceThreshold is the threshold for auto-promoting atoms
	ConfidenceThreshold float64 `json:"confidence_threshold"`

	// AutoPromote enables automatic promotion of successful atoms.
	//
	// Default false: a promoted atom is spliced into the system prompt of
	// every subsequent shard invocation, so promotion is an edit to the
	// agent's own instructions made by the agent, on the evidence of a
	// ConfidenceThreshold that a handful of samples can satisfy. The
	// quarantine plus PromoteAtom/RejectAtom/ApproveTrial is the review
	// surface, and the sibling learning-candidate pipeline already defaults
	// to LearningCandidateAutoPromote=false for the same reason
	// (internal/shards/system/perception.go). Operators who want the closed
	// loop opt in; even then a quarantined candidate only accumulates
	// evidence through an approved trial.
	AutoPromote bool `json:"auto_promote"`

	// EnableStrategies enables the SPL strategy database
	EnableStrategies bool `json:"enable_strategies"`

	// StrategyRefineThreshold is the number of uses before refinement consideration
	StrategyRefineThreshold int `json:"strategy_refine_threshold"`

	// JudgeModel is the model name used for the judge (default: gemini-3-pro)
	JudgeModel string `json:"judge_model"`

	// AtomPinScope controls how tightly evolved atoms are bound to the
	// provider/model whose failures produced them. It governs both the pins
	// stamped onto generated atoms and the granularity at which failures are
	// grouped, which must agree -- see GetFailuresByProblemType.
	//
	// Default PinScopeModel (the exact model). Family and provider scope are
	// explicit opt-ins. Every scope also pins the source role, and evidence
	// lacking a provider or model yields no atom.
	AtomPinScope PinScope `json:"atom_pin_scope"`

	// The bounds below come from the prompt_evolution section of
	// .nerd/config.json (nerdconfig.PromptEvolutionConfig); see there for
	// their meaning. EvolverConfigFrom maps the section onto this struct.
	RecordQueueCapacity     int           `json:"record_queue_capacity"`
	RecordDrainTimeout      time.Duration `json:"record_drain_timeout"`
	MaxCycleScreeningCalls  int           `json:"max_cycle_screening_calls"`
	EvidenceCohortSize      int           `json:"evidence_cohort_size"`
	MaxCycleJudgeCalls      int           `json:"max_cycle_judge_calls"`
	MaxCycleGenerationCalls int           `json:"max_cycle_generation_calls"`
	MaxCycleAtoms           int           `json:"max_cycle_atoms"`
	ClaimLease              time.Duration `json:"claim_lease"`
	TrialMaxExposures       int           `json:"trial_max_exposures"`
	TrialTTL                time.Duration `json:"trial_ttl"`
}

// DefaultEvolverConfig returns the default configuration, with the
// prompt_evolution bounds at their config defaults.
func DefaultEvolverConfig() *EvolverConfig {
	cfg := &EvolverConfig{
		MinFailuresForEvolution: 3,
		EvolutionInterval:       10 * time.Minute,
		MaxAtomsPerEvolution:    3,
		ConfidenceThreshold:     0.7,
		AutoPromote:             false, // human-in-the-loop by default; see AutoPromote doc
		EnableStrategies:        true,

		StrategyRefineThreshold: 10,
		// JudgeModel is a label for the verdict record. Empty means "the model
		// of the client that judged", read from the client at construction.
		AtomPinScope: DefaultPinScope,
	}
	cfg.applyBounds(nerdconfig.DefaultPromptEvolutionConfig())
	return cfg
}

// EvolverConfigFrom is the default configuration with the operator's
// prompt_evolution section applied.
func EvolverConfigFrom(section nerdconfig.PromptEvolutionConfig) *EvolverConfig {
	cfg := DefaultEvolverConfig()
	cfg.applyBounds(section.WithDefaults())
	return cfg
}

func (c *EvolverConfig) applyBounds(s nerdconfig.PromptEvolutionConfig) {
	c.RecordQueueCapacity = s.RecordQueueCapacity
	c.RecordDrainTimeout = time.Duration(s.RecordDrainTimeoutSeconds) * time.Second
	c.MaxCycleJudgeCalls = s.MaxCycleJudgeCalls
	c.MaxCycleScreeningCalls = s.MaxCycleScreeningCalls
	c.EvidenceCohortSize = s.EvidenceCohortSize
	c.MaxCycleGenerationCalls = s.MaxCycleGenerationCalls
	c.MaxCycleAtoms = s.MaxCycleAtoms
	c.ClaimLease = time.Duration(s.ClaimLeaseSeconds) * time.Second
	c.TrialMaxExposures = s.TrialMaxExposures
	c.TrialTTL = time.Duration(s.TrialTTLHours) * time.Hour
}

func validateEvolverConfig(config *EvolverConfig) (*EvolverConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("evolver config is nil")
	}

	normalized := *config
	var errs []string

	if normalized.MinFailuresForEvolution < 1 {
		errs = append(errs, "min_failures_for_evolution must be >= 1")
	}
	if normalized.EvolutionInterval <= 0 {
		errs = append(errs, "evolution_interval must be > 0")
	}
	if normalized.MaxAtomsPerEvolution < 1 {
		errs = append(errs, "max_atoms_per_evolution must be >= 1")
	}
	if normalized.ConfidenceThreshold < 0 || normalized.ConfidenceThreshold > 1 {
		errs = append(errs, "confidence_threshold must be between 0 and 1")
	}
	if normalized.StrategyRefineThreshold < 1 {
		errs = append(errs, "strategy_refine_threshold must be >= 1")
	}
	positive := func(name string, v int) {
		if v < 1 {
			errs = append(errs, name+" must be >= 1")
		}
	}
	positive("record_queue_capacity", normalized.RecordQueueCapacity)
	positive("max_cycle_judge_calls", normalized.MaxCycleJudgeCalls)
	positive("max_cycle_screening_calls", normalized.MaxCycleScreeningCalls)
	positive("evidence_cohort_size", normalized.EvidenceCohortSize)
	if normalized.EvidenceCohortSize < normalized.MinFailuresForEvolution || normalized.EvidenceCohortSize > learnjudge.SharedCauseMaxFailures {
		errs = append(errs, "evidence_cohort_size must cover min_failures_for_evolution and fit the shared-cause protocol")
	}
	positive("max_cycle_generation_calls", normalized.MaxCycleGenerationCalls)
	positive("max_cycle_atoms", normalized.MaxCycleAtoms)
	positive("trial_max_exposures", normalized.TrialMaxExposures)
	if normalized.RecordDrainTimeout <= 0 {
		errs = append(errs, "record_drain_timeout must be > 0")
	}
	if normalized.ClaimLease <= 0 {
		errs = append(errs, "claim_lease must be > 0")
	}
	if normalized.TrialTTL <= 0 {
		errs = append(errs, "trial_ttl must be > 0")
	}

	// An omitted scope means "unset", and takes the default; there is no scope
	// that opts out of pinning. An explicitly wrong
	// value is a config error, because the alternative is generating a corpus
	// pinned differently than the operator asked for.
	switch {
	case strings.TrimSpace(string(normalized.AtomPinScope)) == "":
		normalized.AtomPinScope = DefaultPinScope
	case !normalized.AtomPinScope.Valid():
		errs = append(errs, fmt.Sprintf("atom_pin_scope %q is not one of %v",
			normalized.AtomPinScope, AllPinScopes()))
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid evolver config: %s", strings.Join(errs, "; "))
	}

	return &normalized, nil
}

// AtomPromotedCallback is called when an evolved atom is promoted.
// This allows the Orchestrator to propagate facts to the parent kernel,
// mirroring ToolRegisteredCallback in internal/autopoiesis/ouroboros.go.
type AtomPromotedCallback func(atomID string, promotedAt time.Time)

// TrialFactSink receives the complete set of live trial envelopes whenever it
// may have changed. Production materializes it as kernel facts
// (prompt_trial_envelope / prompt_trial_exposures); the kernel decides
// admission.
type TrialFactSink func(live []TrialEnvelope)

// promotionEvent holds information needed to invoke the promotion callback
// outside the evolver lock.
type promotionEvent struct {
	atomID     string
	promotedAt time.Time
	cb         AtomPromotedCallback
}

// ErrEvolverClosed is returned by operations attempted after Close began.
var ErrEvolverClosed = errors.New("prompt evolver is closed")

// PromptEvolver orchestrates the prompt evolution system.
// It ties together the judge, feedback collector, strategy store,
// and atom generator into a cohesive learning loop.
//
// Locking: mu guards the in-memory candidate set, callbacks and cycle
// counters, and is never held across a model call or the recording queue.
// cycleMu serializes cycles within the process; generation claims fence
// cycles across processes.
type PromptEvolver struct {
	mu      sync.RWMutex
	cycleMu sync.Mutex

	// Configuration
	config  *EvolverConfig
	nerdDir string

	// Core components
	judge             *TaskJudge
	strategyStore     *StrategyStore
	atomGenerator     *AtomGenerator
	feedbackCollector *FeedbackCollector
	ledger            *EvidenceLedger
	classifier        *ProblemClassifier

	// State
	lastEvolution  time.Time
	evolutionCount int
	evolvedAtoms   map[string]*GeneratedAtom // id -> atom
	rejectedCount  int
	// unpinned lists stored atoms moved aside for lacking a recorded model or
	// role to re-pin from (pin_migration.go).
	unpinned []UnpinnedAtom

	successfulCycles   int
	totalCycleDuration time.Duration

	// Storage paths
	evolvedDir  string
	pendingDir  string
	promotedDir string
	rejectedDir string
	unpinnedDir string

	// Callback for notifying parent when an atom is promoted
	onAtomPromoted AtomPromotedCallback
	// trialSink receives live trial envelopes (see TrialFactSink).
	trialSink TrialFactSink
	// jev is Jev for the task verdict, problem type and shared-cause check
	// (jev.go); nil when it is not configured.
	jev *jevUse
	// admission is the check a generated atom passes before it is stored
	// (admission.go); nil until boot installs it.
	admission *atomAdmission

	// Lifecycle: recording queue, cycle cancellation, ordered close.
	recorder  *recordQueue
	lifeCtx   context.Context
	lifeStop  context.CancelFunc
	cycles    sync.WaitGroup
	lifeMu    sync.Mutex
	closing   bool
	closeOnce sync.Once
	closeErr  error
}

// NewPromptEvolver creates a new prompt evolver.
func NewPromptEvolver(
	nerdDir string,
	llmClient LLMClient,
	config *EvolverConfig,
	judgeCompiler PromptCompiler,
	jit nerdconfig.JITConfig,
) (*PromptEvolver, error) {
	if config == nil {
		config = DefaultEvolverConfig()
	}
	validatedConfig, err := validateEvolverConfig(config)
	if err != nil {
		return nil, err
	}
	config = validatedConfig

	// Create directories
	evolvedDir := filepath.Join(nerdDir, "prompts", "evolved")
	pendingDir := filepath.Join(evolvedDir, "pending")
	promotedDir := filepath.Join(evolvedDir, "promoted")
	rejectedDir := filepath.Join(evolvedDir, "rejected")
	unpinnedDir := filepath.Join(evolvedDir, "unpinned")

	for _, dir := range []string{pendingDir, promotedDir, rejectedDir, unpinnedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create components
	feedbackCollector, err := NewFeedbackCollector(nerdDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create feedback collector: %w", err)
	}
	ledger := feedbackCollector.ledger

	var strategyStore *StrategyStore
	if config.EnableStrategies {
		strategyStore, err = NewStrategyStore(nerdDir)
		if err != nil {
			feedbackCollector.Close()
			return nil, fmt.Errorf("failed to create strategy store: %w", err)
		}
		// Seed default strategies
		strategyStore.GenerateDefaultStrategies()
	}

	// The verdict records who judged. Until 2026-09-21 that was the literal
	// "gemini-3-pro" whatever client ran, so every verdict named a model that
	// had never seen the task. The label is the judging client's own model.
	judgeModel := strings.TrimSpace(config.JudgeModel)
	if judgeModel == "" {
		if named, ok := llmClient.(interface{ GetModel() string }); ok {
			judgeModel = strings.TrimSpace(named.GetModel())
		}
	}
	// Every adaptation model call is metered into the spend ledger, by
	// category, so the cost of learning is visible apart from task cost.
	judge := NewTaskJudge(&meteredLLM{inner: llmClient, ledger: ledger, category: SpendJudge},
		judgeModel, judgeCompiler, jit)
	// The atom generator's two calls compile their role prompts with the
	// judge's compiler and budget. Without a compiler they are errors at
	// call time (types.ErrRolePromptUnavailable), never other text.
	var generatorRoles types.RolePromptCompiler
	if roles, rolesErr := prompt.NewRoleCompiler(judgeCompiler, jit); rolesErr == nil {
		generatorRoles = roles
	}
	atomGenerator := NewAtomGeneratorWithPinScope(
		&meteredLLM{inner: llmClient, ledger: ledger, category: SpendGenerate},
		generatorRoles, strategyStore, config.AtomPinScope)
	classifier := NewProblemClassifier()

	pe := &PromptEvolver{
		config:            config,
		nerdDir:           nerdDir,
		judge:             judge,
		strategyStore:     strategyStore,
		atomGenerator:     atomGenerator,
		feedbackCollector: feedbackCollector,
		ledger:            ledger,
		classifier:        classifier,
		evolvedAtoms:      make(map[string]*GeneratedAtom),
		evolvedDir:        evolvedDir,
		pendingDir:        pendingDir,
		promotedDir:       promotedDir,
		rejectedDir:       rejectedDir,
		unpinnedDir:       unpinnedDir,
	}
	pe.lifeCtx, pe.lifeStop = context.WithCancel(context.Background())

	// Load existing evolved atoms, then reproduce their counts from the
	// ledger: the counts in the files are a display cache, not evidence.
	pe.loadEvolvedAtoms()
	pe.mu.Lock()
	for _, ga := range pe.evolvedAtoms {
		pe.refreshCountsLocked(ga)
	}
	pe.mu.Unlock()

	pe.recorder = newRecordQueue(pe, config.RecordQueueCapacity)

	logging.Autopoiesis("PromptEvolver initialized: dir=%s, config=%+v", nerdDir, config)
	return pe, nil
}

// SetOnAtomPromoted sets the callback for when an atom is promoted.
// This allows the Orchestrator to propagate facts to the parent kernel.
func (pe *PromptEvolver) SetOnAtomPromoted(cb AtomPromotedCallback) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.onAtomPromoted = cb
}

// ShouldRunEvolution checks if enough time has passed for a new cycle.
func (pe *PromptEvolver) ShouldRunEvolution() bool {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	if pe.lastEvolution.IsZero() {
		return true
	}

	return time.Since(pe.lastEvolution) >= pe.config.EvolutionInterval
}

// GetStats returns evolution statistics.
func (pe *PromptEvolver) GetStats() *EvolutionStats {
	pe.mu.RLock()
	stats := &EvolutionStats{
		TotalCycles:         pe.evolutionCount,
		TotalAtomsGenerated: len(pe.evolvedAtoms),
		AtomsRejected:       pe.rejectedCount,
		AtomsUnpinned:       len(pe.unpinned),
		SuccessfulCycles:    pe.successfulCycles,
		LastEvolutionAt:     pe.lastEvolution,
	}
	if pe.evolutionCount > 0 {
		stats.AvgCycleDuration = pe.totalCycleDuration / time.Duration(pe.evolutionCount)
	}
	for _, ga := range pe.evolvedAtoms {
		if ga.PromotedAt.IsZero() {
			stats.AtomsPending++
		} else {
			stats.AtomsPromoted++
		}
	}
	pe.mu.RUnlock()

	if pe.feedbackCollector != nil {
		stats.TotalExecutionsRecorded, stats.TotalFailuresAnalyzed = pe.feedbackCollector.GetStats()
		stats.OverallSuccessRate = pe.feedbackCollector.GetSuccessRate()
	}
	if pe.recorder != nil {
		stats.Recording = pe.recorder.stats()
	}
	if pe.ledger != nil {
		if trials, err := pe.ledger.Trials(); err == nil {
			now := time.Now()
			for _, trial := range trials {
				if trial.Live(now) {
					stats.ActiveTrials++
				}
			}
		}
		if claims, err := pe.ledger.ClaimCounts(); err == nil {
			stats.GenerationClaimsCompleted = claims[claimStatusCompleted]
			stats.GenerationClaimsOpen = claims[claimStatusClaimed]
			stats.GenerationClaimsFailed = claims[claimStatusPublishFailed]
		}
		if spend, err := pe.ledger.SpendSummaries(); err == nil {
			stats.Spend = spend
		}
	}
	if pe.strategyStore != nil {
		stats.TotalStrategies, stats.AvgStrategySuccessRate, _ = pe.strategyStore.GetStats()
	}
	return stats
}

// Close stops admission, cancels any running cycle, drains accepted records,
// and only then closes the stores those records and cycles write.
//
// Draining is bounded by RecordDrainTimeout. Records still queued at the
// deadline are abandoned and counted, and Close returns an error naming
// them: a timeout is an incomplete shutdown, never reported as a clean one.
// The stores are closed only after the recording worker and every cycle have
// returned, so a slow drain can delay Close but cannot race it.
func (pe *PromptEvolver) Close() error {
	pe.closeOnce.Do(func() {
		pe.lifeMu.Lock()
		pe.closing = true
		pe.lifeMu.Unlock()
		pe.lifeStop()

		var errs []error
		if pe.recorder != nil {
			if err := pe.recorder.close(pe.config.RecordDrainTimeout); err != nil {
				errs = append(errs, err)
			}
		}
		pe.cycles.Wait()

		pe.mu.Lock()
		defer pe.mu.Unlock()
		if pe.feedbackCollector != nil {
			if err := pe.feedbackCollector.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if pe.strategyStore != nil {
			if err := pe.strategyStore.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) > 0 {
			pe.closeErr = fmt.Errorf("close errors: %w", errors.Join(errs...))
		}
	})
	return pe.closeErr
}

// DrainTimeout is the bound Close places on draining accepted records.
func (pe *PromptEvolver) DrainTimeout() time.Duration {
	if pe == nil || pe.config == nil {
		return 0
	}
	return pe.config.RecordDrainTimeout
}

// enter registers synchronous work (a cycle, a direct RecordExecution) with
// the lifecycle, or refuses once Close has begun. Close waits for every
// registered unit before closing the stores.
func (pe *PromptEvolver) enter() (func(), error) {
	pe.lifeMu.Lock()
	defer pe.lifeMu.Unlock()
	if pe.closing {
		return nil, ErrEvolverClosed
	}
	pe.cycles.Add(1)
	return pe.cycles.Done, nil
}

// beginCycle registers a cycle and derives a context that Close cancels.
func (pe *PromptEvolver) beginCycle(ctx context.Context) (context.Context, func(), error) {
	release, err := pe.enter()
	if err != nil {
		return nil, nil, err
	}
	cycleCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(pe.lifeCtx, cancel)
	return cycleCtx, func() {
		stop()
		cancel()
		release()
	}, nil
}

// ExportStats exports statistics as JSON.
func (pe *PromptEvolver) ExportStats() (string, error) {
	stats := pe.GetStats()
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetFeedbackCollector returns the feedback collector for external use.
func (pe *PromptEvolver) GetFeedbackCollector() *FeedbackCollector {
	return pe.feedbackCollector
}

// GetStrategyStore returns the strategy store for external use.
func (pe *PromptEvolver) GetStrategyStore() *StrategyStore {
	return pe.strategyStore
}

// GetClassifier returns the problem classifier for external use.
func (pe *PromptEvolver) GetClassifier() *ProblemClassifier {
	return pe.classifier
}

// SelectStrategies selects the best strategies for a task.
func (pe *PromptEvolver) SelectStrategies(taskRequest, shardType string) ([]*Strategy, error) {
	if pe.strategyStore == nil {
		return nil, nil
	}

	problemType, _ := pe.classifier.ClassifyWithContext(context.Background(), taskRequest, shardType)
	return pe.strategyStore.SelectStrategies(problemType, shardType, 3)
}

// GetStrategies returns all strategies from the database.
func (pe *PromptEvolver) GetStrategies() []*Strategy {
	if pe.strategyStore == nil {
		return nil
	}
	strategies, _ := pe.strategyStore.GetAllStrategies("", "") // Empty filters = all
	return strategies
}

// Compatibility with SQLite database driver
func init() {
	// SQLite driver is registered elsewhere in the codebase
	_ = sql.Drivers()
}
