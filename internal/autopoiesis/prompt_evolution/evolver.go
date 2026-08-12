package prompt_evolution

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"

	"gopkg.in/yaml.v3"
)

// EvolverConfig configures the prompt evolution system.
type EvolverConfig struct {
	// MinFailuresForEvolution is the minimum failures needed to trigger evolution
	MinFailuresForEvolution int `json:"min_failures_for_evolution"`

	// EvolutionInterval is the minimum time between evolution cycles
	EvolutionInterval time.Duration `json:"evolution_interval"`

	// MaxAtomsPerEvolution is the maximum atoms to generate per cycle
	MaxAtomsPerEvolution int `json:"max_atoms_per_evolution"`

	// ConfidenceThreshold is the threshold for auto-promoting atoms
	ConfidenceThreshold float64 `json:"confidence_threshold"`

	// AutoPromote enables automatic promotion of successful atoms
	AutoPromote bool `json:"auto_promote"`

	// EnableStrategies enables the SPL strategy database
	EnableStrategies bool `json:"enable_strategies"`

	// StrategyRefineThreshold is the number of uses before refinement consideration
	StrategyRefineThreshold int `json:"strategy_refine_threshold"`

	// JudgeModel is the model name used for the judge (default: gemini-3-pro)
	JudgeModel string `json:"judge_model"`
}

// DefaultEvolverConfig returns the default configuration.
func DefaultEvolverConfig() *EvolverConfig {
	return &EvolverConfig{
		MinFailuresForEvolution: 3,
		EvolutionInterval:       10 * time.Minute,
		MaxAtomsPerEvolution:    3,
		ConfidenceThreshold:     0.7,
		AutoPromote:             true,
		EnableStrategies:        true,

		StrategyRefineThreshold: 10,
		JudgeModel:              "gemini-3-pro",
	}
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
	if strings.TrimSpace(normalized.JudgeModel) == "" {
		normalized.JudgeModel = "gemini-3-pro"
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

// promotionEvent holds information needed to invoke the promotion callback
// outside the evolver lock.
type promotionEvent struct {
	atomID     string
	promotedAt time.Time
	cb         AtomPromotedCallback
}

// PromptEvolver orchestrates the prompt evolution system.
// It ties together the judge, feedback collector, strategy store,
// and atom generator into a cohesive learning loop.
type PromptEvolver struct {
	mu sync.RWMutex

	// Configuration
	config  *EvolverConfig
	nerdDir string

	// Core components
	judge             *TaskJudge
	strategyStore     *StrategyStore
	atomGenerator     *AtomGenerator
	feedbackCollector *FeedbackCollector
	classifier        *ProblemClassifier

	// State
	lastEvolution  time.Time
	evolutionCount int
	evolvedAtoms   map[string]*GeneratedAtom // id -> atom

	// Storage paths
	evolvedDir  string
	pendingDir  string
	promotedDir string
	rejectedDir string

	// Callback for notifying parent when an atom is promoted
	onAtomPromoted AtomPromotedCallback
}

// NewPromptEvolver creates a new prompt evolver.
func NewPromptEvolver(
	nerdDir string,
	llmClient LLMClient,
	config *EvolverConfig,
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

	for _, dir := range []string{pendingDir, promotedDir, rejectedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create components
	feedbackCollector, err := NewFeedbackCollector(nerdDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create feedback collector: %w", err)
	}

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

	judgeModel := config.JudgeModel
	judge := NewTaskJudge(llmClient, judgeModel)
	atomGenerator := NewAtomGenerator(llmClient, strategyStore)
	classifier := NewProblemClassifier()

	pe := &PromptEvolver{
		config:            config,
		nerdDir:           nerdDir,
		judge:             judge,
		strategyStore:     strategyStore,
		atomGenerator:     atomGenerator,
		feedbackCollector: feedbackCollector,
		classifier:        classifier,
		evolvedAtoms:      make(map[string]*GeneratedAtom),
		evolvedDir:        evolvedDir,
		pendingDir:        pendingDir,
		promotedDir:       promotedDir,
		rejectedDir:       rejectedDir,
	}

	// Load existing evolved atoms
	pe.loadEvolvedAtoms()

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

// RecordExecution records a task execution for later analysis.
func (pe *PromptEvolver) RecordExecution(exec *ExecutionRecord) error {
	pe.mu.Lock()

	// Classify the problem type if not set
	if exec.ProblemType == "" {
		problemType, _ := pe.classifier.ClassifyWithContext(
			context.Background(),
			exec.TaskRequest,
			exec.ShardType,
		)
		exec.ProblemType = string(problemType)
	}

	pending := pe.applyExecutionOutcomeLocked(exec)
	err := pe.feedbackCollector.Record(exec)
	pe.mu.Unlock()
	// Invoke promotion callbacks without holding lock to avoid deadlock.
	for _, ev := range pending {
		if ev.cb != nil {
			ev.cb(ev.atomID, ev.promotedAt)
		}
	}
	return err
}

// TriggerEvolution manually triggers an evolution cycle.
func (pe *PromptEvolver) TriggerEvolution(ctx context.Context) (*EvolutionResult, error) {
	return pe.RunEvolutionCycle(ctx)
}

// RunEvolutionCycle executes a full evolution cycle.
func (pe *PromptEvolver) RunEvolutionCycle(ctx context.Context) (*EvolutionResult, error) {
	pe.mu.Lock()

	timer := logging.StartTimer(logging.CategoryAutopoiesis, "PromptEvolver.RunEvolutionCycle")
	defer timer.Stop()

	start := time.Now()
	result := &EvolutionResult{
		Timestamp: start,
	}

	logging.Autopoiesis("Starting evolution cycle")

	var pendingPromotions []promotionEvent

	// 1. Get recent failures that need evaluation
	unevaluated, err := pe.feedbackCollector.GetUnevaluated(20)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to get unevaluated: %v", err)
	}

	// 2. Evaluate unevaluated executions
	if len(unevaluated) > 0 {
		verdicts, err := pe.judge.EvaluateBatch(ctx, unevaluated)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
		for i, v := range verdicts {
			if v != nil && i < len(unevaluated) {
				pe.feedbackCollector.UpdateVerdict(unevaluated[i].TaskID, v)
				unevaluated[i].Verdict = v
				if evs := pe.applyExecutionOutcomeLocked(unevaluated[i]); len(evs) > 0 {
					pendingPromotions = append(pendingPromotions, evs...)
				}
			}
		}
	}

	// 3. Get failures grouped by problem type and shard
	grouped, err := pe.feedbackCollector.GetFailuresByProblemType(pe.config.MinFailuresForEvolution)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		pe.mu.Unlock()
		// Invoke any pending promotions without holding lock.
		for _, ev := range pendingPromotions {
			if ev.cb != nil {
				ev.cb(ev.atomID, ev.promotedAt)
			}
		}
		return result, err
	}

	result.FailuresAnalyzed = 0
	result.GroupsProcessed = len(grouped)

	// 4. Process each group
	for groupKey, failures := range grouped {
		// Parse group key (format: "problem_type:shard_type")
		parts := strings.SplitN(groupKey, ":", 2)
		if len(parts) != 2 {
			continue
		}
		problemType, shardType := parts[0], parts[1]

		// Extract verdicts from failures
		verdicts := make([]*JudgeVerdict, 0, len(failures))
		for _, f := range failures {
			if f.Verdict != nil {
				verdicts = append(verdicts, f.Verdict)
				result.FailuresAnalyzed++
			}
		}

		if len(verdicts) < pe.config.MinFailuresForEvolution {
			continue
		}

		// 5. Generate new atoms from failures
		atoms, err := pe.atomGenerator.GenerateFromFailures(ctx, verdicts, shardType, problemType)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("atom generation failed for %s: %v", groupKey, err))
			continue
		}

		// Limit atoms per evolution
		if len(atoms) > pe.config.MaxAtomsPerEvolution {
			atoms = atoms[:pe.config.MaxAtomsPerEvolution]
		}

		// 6. Store evolved atoms
		for _, atom := range atoms {
			if err := pe.storeEvolvedAtom(atom); err != nil {
				result.Errors = append(result.Errors, err.Error())
				continue
			}
			result.AtomsGenerated++
			result.AtomIDs = append(result.AtomIDs, atom.Atom.ID)
		}

		// 7. Update strategies if enabled
		if pe.config.EnableStrategies && pe.strategyStore != nil {
			pe.updateStrategies(ctx, ProblemType(problemType), shardType, verdicts)
		}
	}

	// 8. Auto-promote eligible atoms
	if pe.config.AutoPromote {
		autoPending, promoted := pe.autoPromoteAtoms()
		pendingPromotions = append(pendingPromotions, autoPending...)
		result.AtomsPromoted = promoted
	}

	// Update state
	pe.lastEvolution = time.Now()
	pe.evolutionCount++
	result.Duration = time.Since(start)

	logging.Autopoiesis("Evolution cycle complete: failures=%d, atoms=%d, promoted=%d, duration=%v",
		result.FailuresAnalyzed, result.AtomsGenerated, result.AtomsPromoted, result.Duration)

	pe.mu.Unlock()
	// Invoke promotion callbacks without holding lock to avoid deadlock
	// if the callback re-enters the evolver (e.g., GetPromotedAtoms).
	for _, ev := range pendingPromotions {
		if ev.cb != nil {
			ev.cb(ev.atomID, ev.promotedAt)
		}
	}

	return result, nil
}

// storeEvolvedAtom saves an evolved atom to the pending directory.
func (pe *PromptEvolver) storeEvolvedAtom(ga *GeneratedAtom) error {
	path := pe.atomStoragePathLocked(ga.Atom.ID, !ga.PromotedAt.IsZero())
	data, err := pe.marshalGeneratedAtom(ga)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write atom file: %w", err)
	}

	// Only add to memory after durable write succeeds.
	pe.evolvedAtoms[ga.Atom.ID] = ga

	logging.Autopoiesis("Evolved atom stored: id=%s, path=%s", ga.Atom.ID, path)
	return nil
}

func (pe *PromptEvolver) atomStoragePathLocked(atomID string, promoted bool) string {
	filename := strings.ReplaceAll(atomID, "/", "_") + ".yaml"
	if promoted {
		return filepath.Join(pe.promotedDir, filename)
	}
	return filepath.Join(pe.pendingDir, filename)
}

func (pe *PromptEvolver) marshalGeneratedAtom(ga *GeneratedAtom) ([]byte, error) {
	wrapper := struct {
		Atom         *prompt.PromptAtom `yaml:"atom"`
		Source       string             `yaml:"source"`
		SourceIDs    []string           `yaml:"source_ids"`
		ShardType    string             `yaml:"shard_type"`
		ProblemType  string             `yaml:"problem_type"`
		Confidence   float64            `yaml:"confidence"`
		UsageCount   int                `yaml:"usage_count"`
		SuccessCount int                `yaml:"success_count"`
		CreatedAt    time.Time          `yaml:"created_at"`
		PromotedAt   time.Time          `yaml:"promoted_at,omitempty"`
	}{
		Atom:         ga.Atom,
		Source:       ga.Source,
		SourceIDs:    ga.SourceIDs,
		ShardType:    ga.ShardType,
		ProblemType:  ga.ProblemType,
		Confidence:   ga.Confidence,
		UsageCount:   ga.UsageCount,
		SuccessCount: ga.SuccessCount,
		CreatedAt:    ga.CreatedAt,
		PromotedAt:   ga.PromotedAt,
	}

	data, err := yaml.Marshal(wrapper)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal atom: %w", err)
	}
	return data, nil
}

// updateStrategies updates or creates strategies based on outcomes.
func (pe *PromptEvolver) updateStrategies(
	ctx context.Context,
	problemType ProblemType,
	shardType string,
	verdicts []*JudgeVerdict,
) {
	// Check if any strategies need refinement
	strategies, _ := pe.strategyStore.GetStrategiesNeedingRefinement(
		pe.config.StrategyRefineThreshold,
		3,
	)

	for _, strategy := range strategies {
		if strategy.ProblemType != problemType || strategy.ShardType != shardType {
			continue
		}

		refined, err := pe.atomGenerator.RefineStrategy(ctx, strategy, verdicts)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to refine strategy %s: %v", strategy.ID, err)
			continue
		}

		if err := pe.strategyStore.RefineStrategy(strategy.ID, refined.Content); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to save refined strategy: %v", err)
		}
	}
}

// autoPromoteAtoms promotes atoms that meet the confidence threshold.
func (pe *PromptEvolver) autoPromoteAtoms() ([]promotionEvent, int) {
	promoted := 0
	var pending []promotionEvent

	for id, ga := range pe.evolvedAtoms {
		if ga.ShouldPromote(pe.config.ConfidenceThreshold) && ga.PromotedAt.IsZero() {
			cb, promotedAt, err := pe.promoteAtomLocked(id)
			if err == nil {
				promoted++
				if cb != nil {
					pending = append(pending, promotionEvent{atomID: id, promotedAt: promotedAt, cb: cb})
				}
			}
		}
	}

	return pending, promoted
}

// PromoteAtom moves an atom from pending to promoted.
func (pe *PromptEvolver) PromoteAtom(atomID string) error {
	pe.mu.Lock()
	cb, promotedAt, err := pe.promoteAtomLocked(atomID)
	pe.mu.Unlock()
	if err == nil && cb != nil {
		// Callback invoked without pe.mu held to avoid deadlock
		// if the callback re-enters the evolver.
		cb(atomID, promotedAt)
	}
	return err
}

// promoteAtomLocked moves an atom from pending to promoted (must hold lock).
func (pe *PromptEvolver) promoteAtomLocked(atomID string) (AtomPromotedCallback, time.Time, error) {
	ga, exists := pe.evolvedAtoms[atomID]
	if !exists {
		return nil, time.Time{}, fmt.Errorf("atom not found: %s", atomID)
	}

	filename := strings.ReplaceAll(atomID, "/", "_") + ".yaml"
	srcPath := filepath.Join(pe.pendingDir, filename)
	dstPath := filepath.Join(pe.promotedDir, filename)

	// Move file
	if err := os.Rename(srcPath, dstPath); err != nil {
		// Try copy if rename fails (cross-device)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, time.Time{}, err
		}
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			return nil, time.Time{}, err
		}
		os.Remove(srcPath)
	}

	ga.PromotedAt = time.Now()
	if err := pe.storeEvolvedAtom(ga); err != nil {
		return nil, time.Time{}, err
	}
	logging.Autopoiesis("Atom promoted: id=%s", atomID)
	// Snapshot callback while holding lock so caller can invoke it
	// outside the lock. Invoking outside avoids deadlock if the
	// callback re-enters the evolver (e.g., calls GetPromotedAtoms
	// which takes RLock) while the exclusive lock is held.
	cb := pe.onAtomPromoted
	promotedAt := ga.PromotedAt
	return cb, promotedAt, nil
}

// RejectAtom marks an atom as rejected.
func (pe *PromptEvolver) RejectAtom(atomID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	filename := strings.ReplaceAll(atomID, "/", "_") + ".yaml"
	srcPath := filepath.Join(pe.pendingDir, filename)
	dstPath := filepath.Join(pe.rejectedDir, filename)

	if err := os.Rename(srcPath, dstPath); err != nil {
		return err
	}

	delete(pe.evolvedAtoms, atomID)
	logging.Autopoiesis("Atom rejected: id=%s", atomID)
	return nil
}

// RecordAtomUsage records that an atom was used and whether it led to success.
func (pe *PromptEvolver) RecordAtomUsage(atomID string, success bool) {
	pe.mu.Lock()
	pending := pe.recordAtomUsageLocked(atomID, success)
	pe.mu.Unlock()
	// Invoke callbacks without holding lock to avoid deadlock.
	for _, ev := range pending {
		if ev.cb != nil {
			ev.cb(ev.atomID, ev.promotedAt)
		}
	}
}

func (pe *PromptEvolver) recordAtomUsageLocked(atomID string, success bool) []promotionEvent {
	ga, exists := pe.evolvedAtoms[atomID]
	if !exists {
		return nil
	}

	ga.UsageCount++
	if success {
		ga.SuccessCount++
	}

	// Update confidence based on new data
	ga.Confidence = ga.SuccessRate()
	if err := pe.storeEvolvedAtom(ga); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to persist atom usage for %s: %v", atomID, err)
	}
	if pe.config.AutoPromote && ga.ShouldPromote(pe.config.ConfidenceThreshold) && ga.PromotedAt.IsZero() {
		cb, promotedAt, err := pe.promoteAtomLocked(atomID)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to auto-promote atom %s: %v", atomID, err)
			return nil
		}
		if cb != nil {
			return []promotionEvent{{atomID: atomID, promotedAt: promotedAt, cb: cb}}
		}
	}
	return nil
}

// GetEvolvedAtoms returns all evolved atoms.
func (pe *PromptEvolver) GetEvolvedAtoms() []*GeneratedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	result := make([]*GeneratedAtom, 0, len(pe.evolvedAtoms))
	for _, ga := range pe.evolvedAtoms {
		result = append(result, ga)
	}
	return result
}

// GetPendingAtoms returns atoms awaiting promotion.
func (pe *PromptEvolver) GetPendingAtoms() []*GeneratedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var result []*GeneratedAtom
	for _, ga := range pe.evolvedAtoms {
		if ga.PromotedAt.IsZero() {
			result = append(result, ga)
		}
	}
	return result
}

// GetPromotedAtoms returns promoted atoms.
func (pe *PromptEvolver) GetPromotedAtoms() []*GeneratedAtom {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var result []*GeneratedAtom
	for _, ga := range pe.evolvedAtoms {
		if !ga.PromotedAt.IsZero() {
			result = append(result, ga)
		}
	}
	return result
}

// GetStats returns evolution statistics.
func (pe *PromptEvolver) GetStats() *EvolutionStats {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	stats := &EvolutionStats{
		TotalCycles:         pe.evolutionCount,
		TotalAtomsGenerated: len(pe.evolvedAtoms),
		LastEvolutionAt:     pe.lastEvolution,
	}

	// Count pending vs promoted
	for _, ga := range pe.evolvedAtoms {
		if ga.PromotedAt.IsZero() {
			stats.AtomsPending++
		} else {
			stats.AtomsPromoted++
		}
	}

	// Get feedback stats
	if pe.feedbackCollector != nil {
		stats.TotalExecutionsRecorded, stats.TotalFailuresAnalyzed = pe.feedbackCollector.GetStats()
		stats.OverallSuccessRate = pe.feedbackCollector.GetSuccessRate()
	}

	// Get strategy stats
	if pe.strategyStore != nil {
		stats.TotalStrategies, stats.AvgStrategySuccessRate, _ = pe.strategyStore.GetStats()
	}

	return stats
}

// loadEvolvedAtoms loads existing evolved atoms from disk.
func (pe *PromptEvolver) loadEvolvedAtoms() {
	// Load from pending directory
	pe.loadAtomsFromDir(pe.pendingDir, false)

	// Load from promoted directory
	pe.loadAtomsFromDir(pe.promotedDir, true)

	logging.Autopoiesis("Loaded %d evolved atoms from disk", len(pe.evolvedAtoms))
}

func (pe *PromptEvolver) loadAtomsFromDir(dir string, promoted bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var wrapper struct {
			Atom         *prompt.PromptAtom `yaml:"atom"`
			Source       string             `yaml:"source"`
			SourceIDs    []string           `yaml:"source_ids"`
			ShardType    string             `yaml:"shard_type"`
			ProblemType  string             `yaml:"problem_type"`
			Confidence   float64            `yaml:"confidence"`
			UsageCount   int                `yaml:"usage_count"`
			SuccessCount int                `yaml:"success_count"`
			CreatedAt    time.Time          `yaml:"created_at"`
			PromotedAt   time.Time          `yaml:"promoted_at,omitempty"`
		}

		if err := yaml.Unmarshal(data, &wrapper); err != nil {
			continue
		}

		if wrapper.Atom == nil {
			continue
		}

		ga := &GeneratedAtom{
			Atom:         wrapper.Atom,
			Source:       wrapper.Source,
			SourceIDs:    wrapper.SourceIDs,
			ShardType:    wrapper.ShardType,
			ProblemType:  wrapper.ProblemType,
			Confidence:   wrapper.Confidence,
			UsageCount:   wrapper.UsageCount,
			SuccessCount: wrapper.SuccessCount,
			CreatedAt:    wrapper.CreatedAt,
			PromotedAt:   wrapper.PromotedAt,
		}

		if promoted && ga.PromotedAt.IsZero() {
			info, _ := entry.Info()
			if info != nil {
				ga.PromotedAt = info.ModTime()
			} else {
				ga.PromotedAt = time.Now()
			}
		}

		pe.evolvedAtoms[wrapper.Atom.ID] = ga
	}
}

func (pe *PromptEvolver) applyExecutionOutcomeLocked(exec *ExecutionRecord) []promotionEvent {
	if exec == nil || exec.Verdict == nil || len(exec.AtomIDs) == 0 {
		return nil
	}

	success := exec.Verdict.IsPass()
	var pending []promotionEvent
	for _, atomID := range exec.AtomIDs {
		if evs := pe.recordAtomUsageLocked(atomID, success); len(evs) > 0 {
			pending = append(pending, evs...)
		}
	}
	return pending
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

// Close cleans up resources.
func (pe *PromptEvolver) Close() error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	var errs []error

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
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
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
