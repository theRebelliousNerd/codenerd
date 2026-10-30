package prompt_evolution

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// atomGeneratorVersion names the generation procedure (meta-prompt, parser,
// pinning) that consumes failure evidence. It is part of every generation
// claim key: bump it when the procedure changes and the same evidence becomes
// eligible for one deliberate, separately charged reanalysis.
const atomGeneratorVersion = "atomgen-cohort-v2"

// cycleBudget is the global spend bound of one cycle, across every group.
type cycleBudget struct {
	generationCalls int
	screeningCalls  int
	atoms           int
}

// failureGroup is one qualifying group with its unconsumed evidence.
type failureGroup struct {
	key      FailureGroupKey
	failures []*ExecutionRecord
	oldest   time.Time
}

// RunEvolutionCycle executes one bounded evolution cycle.
//
// No evolver lock is held across a model call: evidence is snapshotted, the
// judge and generator run unlocked, and results are published under the lock
// behind a generation-claim fence. Recording therefore never waits behind a
// slow model, and a worker whose claim was taken over cannot publish.
//
// Spend is bounded per cycle (MaxCycleJudgeCalls, MaxCycleScreeningCalls, MaxCycleGenerationCalls,
// MaxCycleAtoms) across all groups, and groups are served oldest unconsumed
// evidence first, so a busy group cannot starve an old one. Evidence that has
// bought a generation is consumed and never buys another under the same
// generator version.
func (pe *PromptEvolver) RunEvolutionCycle(ctx context.Context) (*EvolutionResult, error) {
	ctx, release, err := pe.beginCycle(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	pe.cycleMu.Lock()
	defer pe.cycleMu.Unlock()

	timer := logging.StartTimer(logging.CategoryAutopoiesis, "PromptEvolver.RunEvolutionCycle")
	defer timer.Stop()

	start := time.Now()
	cycleID := newOwnerToken()
	result := &EvolutionResult{Timestamp: start}
	logging.Autopoiesis("Starting evolution cycle %s", cycleID)

	pe.mu.RLock()
	cfg := *pe.config
	pe.mu.RUnlock()
	budget := cycleBudget{generationCalls: cfg.MaxCycleGenerationCalls, screeningCalls: cfg.MaxCycleScreeningCalls, atoms: cfg.MaxCycleAtoms}

	// 1. Judge what is unjudged, bounded, unlocked.
	pe.judgeUnevaluated(ctx, cycleID, cfg.MaxCycleJudgeCalls, result)

	// 2. Unconsumed failure groups, oldest evidence first.
	grouped, err := pe.feedbackCollector.GetFailuresByProblemType(cfg.MinFailuresForEvolution, cfg.AtomPinScope)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		pe.finishCycle(result, start, false)
		return result, err
	}
	groups := orderGroupsOldestFirst(grouped)
	result.GroupsProcessed = len(groups)

	// 3. Buy generation for groups while the budget lasts.
	for i, group := range groups {
		if ctx.Err() != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("cycle canceled: %v", ctx.Err()))
			result.GroupsDeferred += len(groups) - i
			break
		}
		if budget.generationCalls <= 0 || budget.atoms <= 0 {
			result.GroupsDeferred += len(groups) - i
			break
		}
		pe.generateForGroup(ctx, cycleID, cfg, group, &budget, result)
	}

	// 4. Opt-in auto-promotion over ledger-derived counts.
	if cfg.AutoPromote {
		held := pe.promotionHolds(ctx)
		pe.mu.Lock()
		pending, promoted := pe.autoPromoteAtoms(held)
		pe.mu.Unlock()
		result.AtomsPromoted = promoted
		firePromotions(pending)
		if promoted > 0 {
			pe.publishTrials()
		}
	}

	pe.finishCycle(result, start, len(result.Errors) == 0)
	logging.Autopoiesis("Evolution cycle %s complete: failures=%d atoms=%d discarded=%d deferred=%d promoted=%d duration=%v",
		cycleID, result.FailuresAnalyzed, result.AtomsGenerated, result.AtomsDiscarded, result.GroupsDeferred,
		result.AtomsPromoted, result.Duration)
	return result, nil
}

func (pe *PromptEvolver) finishCycle(result *EvolutionResult, start time.Time, ok bool) {
	result.Duration = time.Since(start)
	pe.mu.Lock()
	pe.lastEvolution = time.Now()
	pe.evolutionCount++
	pe.totalCycleDuration += result.Duration
	if ok {
		pe.successfulCycles++
	}
	pe.mu.Unlock()
}

// judgeUnevaluated sends up to limit unjudged records to the judge and
// applies each verdict: persisted first, then credited.
func (pe *PromptEvolver) judgeUnevaluated(ctx context.Context, cycleID string, limit int, result *EvolutionResult) {
	unevaluated, err := pe.feedbackCollector.GetUnevaluated(limit)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	if len(unevaluated) == 0 {
		return
	}
	result.JudgeCalls = len(unevaluated)
	verdicts, err := pe.evaluateBatch(withSpend(ctx, cycleID, SpendJudge), unevaluated)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	for i, v := range verdicts {
		if v == nil || i >= len(unevaluated) {
			continue
		}
		if err := pe.applyVerdict(unevaluated[i], v); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("apply verdict for %s: %v", unevaluated[i].TaskID, err))
		}
	}
}

// orderGroupsOldestFirst sorts groups by their oldest unconsumed failure,
// then by key for determinism.
func orderGroupsOldestFirst(grouped map[FailureGroupKey][]*ExecutionRecord) []failureGroup {
	groups := make([]failureGroup, 0, len(grouped))
	for key, failures := range grouped {
		g := failureGroup{key: key, failures: failures}
		for _, f := range failures {
			if f != nil && (g.oldest.IsZero() || f.Timestamp.Before(g.oldest)) {
				g.oldest = f.Timestamp
			}
		}
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if !groups[i].oldest.Equal(groups[j].oldest) {
			return groups[i].oldest.Before(groups[j].oldest)
		}
		return groups[i].key.String() < groups[j].key.String()
	})
	return groups
}

// generateForGroup claims one group's evidence, buys one generation for it
// unlocked, and publishes behind the claim's fence.
func (pe *PromptEvolver) generateForGroup(ctx context.Context, cycleID string, cfg EvolverConfig, group failureGroup, budget *cycleBudget, result *EvolutionResult) {
	// No pin, no atom: checked before anything is bought for this group.
	if err := requirePin(cfg.AtomPinScope, group.key.ShardType, servingPinFor(group.failures)); err != nil {
		pe.refuseUnpinnedGroup(cycleID, cfg, group, err, result)
		return
	}

	cohort, err := selectEvidenceCohort(ctx, group, cfg)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	if len(cohort.verdicts) < cfg.MinFailuresForEvolution {
		return
	}
	if err := pe.ledger.saveCohort(cohort); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	result.EvidenceDeferred += cohort.omitted
	screenID, proceed := pe.screenCohort(ctx, cycleID, cfg, group, cohort, budget, result)
	if !proceed {
		return
	}
	verdicts, evidence := cohort.verdicts, cohort.evidence
	result.FailuresAnalyzed += len(verdicts)

	groupKey := string(cfg.AtomPinScope) + "|" + group.key.String()
	claimKey := generationClaimKey(groupKey, atomGeneratorVersion+"/"+cohort.id+"/"+screenID, evidence)
	owner := cycleID + "/" + newOwnerToken()
	acquired, err := pe.ledger.Claim(claimKey, groupKey, atomGeneratorVersion, len(evidence), owner, cfg.ClaimLease, time.Now())
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	if !acquired {
		result.GroupsSkipped++
		return
	}

	if _, err := pe.ledger.db.Exec(`INSERT OR IGNORE INTO generation_cohorts
	 (claim_key, cohort_id, screen_key) VALUES (?, ?, ?)`, claimKey, cohort.id, screenID); err != nil {
		_ = pe.ledger.Release(claimKey, owner, err.Error())
		result.Errors = append(result.Errors, err.Error())
		return
	}

	// The model call: unlocked, metered, and charged against the cycle budget
	// whether or not it yields anything usable.
	budget.generationCalls--
	result.GenerationCalls++
	problemType, shardType := group.key.ProblemType, group.key.ShardType
	atoms, err := pe.atomGenerator.GenerateFromFailures(withSpend(ctx, cycleID, SpendGenerate),
		cohort, shardType, problemType, servingPinFor(group.failures))
	if err != nil {
		_ = pe.ledger.Release(claimKey, owner, err.Error())
		_ = pe.ledger.RecordSpend(SpendRow{CycleID: cycleID, Category: SpendGenerate, Outcome: SpendUnusable, Units: 1})
		result.Errors = append(result.Errors, fmt.Sprintf("atom generation failed for %s: %v", group.key, err))
		return
	}

	if len(atoms) == 0 {
		// The call returned but yielded no candidate. The evidence is still
		// consumed below: retrying the same evidence with the same generator
		// would buy the same nothing every cycle.
		_ = pe.ledger.RecordSpend(SpendRow{CycleID: cycleID, Category: SpendGenerate, Outcome: SpendUnusable, Units: 1})
	}

	// The admission check, then the per-group cap and the cycle's remaining
	// atom budget.
	atoms, refused := pe.admitGenerated(ctx, atoms)
	if refused > 0 {
		result.AtomsRefused += refused
		_ = pe.ledger.RecordSpend(SpendRow{CycleID: cycleID, Category: SpendGenerate, Outcome: SpendDiscarded, Units: refused})
	}
	keep := min(len(atoms), cfg.MaxAtomsPerEvolution, budget.atoms)
	if discarded := len(atoms) - keep; discarded > 0 {
		result.AtomsDiscarded += discarded
		_ = pe.ledger.RecordSpend(SpendRow{CycleID: cycleID, Category: SpendGenerate, Outcome: SpendDiscarded, Units: discarded})
	}
	atoms = atoms[:keep]

	for _, atom := range atoms {
		if atom != nil {
			atom.EvidenceCohortID = cohort.id
			atom.ScreenObservationID = screenID
		}
	}
	pe.publishGeneration(cycleID, claimKey, owner, evidence, atoms, budget, result)

	// Strategy refinement for this group, against the same generation budget.
	if cfg.EnableStrategies && pe.strategyStore != nil && budget.generationCalls > 0 {
		pe.updateStrategies(withSpend(ctx, cycleID, SpendRefine), ProblemType(problemType), shardType, verdicts, budget, result)
	}
}

// publishGeneration completes the claim (fenced) and stores the candidates.
// The claim completes first so a worker that lost its lease writes nothing;
// a storage failure afterwards returns the evidence to the pool and is
// recorded as a failed publication rather than left ambiguous.
func (pe *PromptEvolver) publishGeneration(cycleID, claimKey, owner string, evidence []EvidenceRef, atoms []*GeneratedAtom, budget *cycleBudget, result *EvolutionResult) {
	ids := make([]string, 0, len(atoms))
	for _, atom := range atoms {
		if atom != nil && atom.Atom != nil {
			ids = append(ids, atom.Atom.ID)
		}
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()
	if err := pe.ledger.Complete(claimKey, owner, evidence, ids, time.Now()); err != nil {
		if errors.Is(err, ErrClaimNotOwned) {
			result.GroupsSkipped++
			result.AtomsDiscarded += len(ids)
			_ = pe.ledger.RecordSpend(SpendRow{CycleID: cycleID, Category: SpendGenerate, Outcome: SpendDiscarded, Units: len(ids)})
			return
		}
		result.Errors = append(result.Errors, err.Error())
		return
	}
	result.EvidenceConsumed += len(evidence)

	var failed []string
	for _, atom := range atoms {
		if atom == nil || atom.Atom == nil {
			continue
		}
		if err := pe.storeEvolvedAtom(atom); err != nil {
			failed = append(failed, atom.Atom.ID+": "+err.Error())
			continue
		}
		budget.atoms--
		result.AtomsGenerated++
		result.AtomIDs = append(result.AtomIDs, atom.Atom.ID)
	}
	if len(failed) > 0 {
		reason := strings.Join(failed, "; ")
		_ = pe.ledger.PublishFailed(claimKey, owner, reason)
		_ = pe.ledger.RecordSpend(SpendRow{CycleID: cycleID, Category: SpendGenerate, Outcome: SpendPublishFailed, Units: len(failed)})
		result.EvidenceConsumed -= len(evidence)
		result.Errors = append(result.Errors, "publish failed: "+reason)
	}
}

// refuseUnpinnedGroup logs and counts a group whose evidence cannot be pinned,
// and consumes that evidence: it can never yield an atom, so leaving it
// unconsumed would re-read and re-refuse it every cycle. The execution
// records themselves are untouched.
func (pe *PromptEvolver) refuseUnpinnedGroup(cycleID string, cfg EvolverConfig, group failureGroup, cause error, result *EvolutionResult) {
	result.GroupsWithoutPin++
	logging.Get(logging.CategoryAutopoiesis).Warn(
		"Group %s not evolved (%d failures): %v", group.key, len(group.failures), cause)

	evidence := make([]EvidenceRef, 0, len(group.failures))
	for _, f := range group.failures {
		if f != nil && f.VerdictVersion != "" {
			evidence = append(evidence, EvidenceRef{TaskID: f.TaskID, VerdictVersion: f.VerdictVersion})
		}
	}
	if len(evidence) == 0 {
		return
	}
	groupKey := string(cfg.AtomPinScope) + "|" + group.key.String()
	claimKey := generationClaimKey(groupKey, atomGeneratorVersion, evidence)
	owner := cycleID + "/" + newOwnerToken()
	acquired, err := pe.ledger.Claim(claimKey, groupKey, atomGeneratorVersion, len(evidence), owner, cfg.ClaimLease, time.Now())
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	if !acquired {
		return
	}
	if err := pe.ledger.Complete(claimKey, owner, evidence, nil, time.Now()); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return
	}
	result.EvidenceConsumed += len(evidence)
}

// servingPinFor returns the provider/model a failure group was observed on,
// taking the first record that actually carries provenance. Records predating
// the provenance columns have neither, so a group made entirely of them yields
// the zero pin, which requirePin refuses.
func servingPinFor(failures []*ExecutionRecord) ServingPin {
	for _, f := range failures {
		if f == nil {
			continue
		}
		if strings.TrimSpace(f.Provider) != "" || strings.TrimSpace(f.Model) != "" {
			return ServingPin{Provider: f.Provider, Model: f.Model}
		}
	}
	return ServingPin{}
}

// updateStrategies refines strategies for one group, unlocked, each refine
// call charged against the cycle's generation budget.
func (pe *PromptEvolver) updateStrategies(
	ctx context.Context,
	problemType ProblemType,
	shardType string,
	verdicts []*JudgeVerdict,
	budget *cycleBudget,
	result *EvolutionResult,
) {
	strategies, _ := pe.strategyStore.GetStrategiesNeedingRefinement(pe.config.StrategyRefineThreshold, 3)
	for _, strategy := range strategies {
		if strategy.ProblemType != problemType || strategy.ShardType != shardType {
			continue
		}
		if budget.generationCalls <= 0 || ctx.Err() != nil {
			return
		}
		budget.generationCalls--
		result.GenerationCalls++
		refined, err := pe.atomGenerator.RefineStrategy(ctx, strategy, verdicts)
		if err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to refine strategy %s: %v", strategy.ID, err)
			continue
		}
		if err := pe.strategyStore.RefineStrategy(strategy.ID, refined.Content); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to save refined strategy: %v", err)
			continue
		}
		result.StrategiesRefined++
	}
}
