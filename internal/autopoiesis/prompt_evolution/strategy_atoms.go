package prompt_evolution

import (
	"math"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
)

// StrategyAtomIDPrefix marks a prompt atom as a rendered learned strategy.
//
// It is the handle that closes the strategy feedback loop. A compiled turn
// reports the atom IDs that were in its prompt; anything under this prefix is
// a strategy whose outcome can be attributed back to the strategy store with
// StrategyIDFromAtomID, which is what makes SelectStrategies' ORDER BY
// success_rate mean something.
const StrategyAtomIDPrefix = "strategy/"

// StrategyIDFromAtomID recovers the strategy behind a rendered atom, or "" if
// the atom is not one.
func StrategyIDFromAtomID(atomID string) string {
	if !strings.HasPrefix(atomID, StrategyAtomIDPrefix) {
		return ""
	}
	return strings.TrimPrefix(atomID, StrategyAtomIDPrefix)
}

// maxStrategyAtomsPerPrompt caps how many strategies one prompt may carry.
//
// Three, matching what SelectStrategies already returns by default. The cap is
// not really about tokens — the compiler's budget fitter would drop the
// overflow anyway — it is about not handing a model five competing
// step-by-step procedures for the same task and expecting it to pick.
const maxStrategyAtomsPerPrompt = 3

// StrategyAtomProvider renders the strategies that apply to a compilation as
// prompt atoms, so the strategy database is read as well as written.
//
// Before it existed, GenerateDefaultStrategies seeded a per-problem-type
// playbook at every boot and the evolution cycle refined those entries from
// real failures — and no strategy ever reached a model. SelectStrategies had
// no production caller at all.
type StrategyAtomProvider struct {
	store      *StrategyStore
	classifier *ProblemClassifier
}

// NewStrategyAtomProvider builds the provider. A nil store yields a provider
// that returns nothing, which is the correct behaviour when strategies are
// disabled (EvolverConfig.EnableStrategies) rather than an error.
func NewStrategyAtomProvider(store *StrategyStore, classifier *ProblemClassifier) *StrategyAtomProvider {
	if classifier == nil {
		classifier = NewProblemClassifier()
	}
	return &StrategyAtomProvider{store: store, classifier: classifier}
}

// StrategyAtoms implements prompt.StrategyProvider.
//
// Classification is the regex classifier, not the LLM one: this runs inside
// prompt compilation on the turn's critical path, and a network call there
// would put the learning loop's latency in front of every user request.
func (p *StrategyAtomProvider) StrategyAtoms(cc *prompt.CompilationContext) []*prompt.PromptAtom {
	if p == nil || p.store == nil || cc == nil {
		return nil
	}

	shardType := strings.TrimSpace(cc.ShardType)
	if shardType == "" {
		// Strategies are stored per shard type; without one there is no row to
		// look up, and guessing would serve a coder playbook to a reviewer.
		return nil
	}

	problemType, _ := p.classifier.Classify(strategyQueryFor(cc))
	strategies, err := p.store.SelectStrategies(problemType, shardType, maxStrategyAtomsPerPrompt)
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn(
			"Failed to select strategies for %s/%s: %v", problemType, shardType, err)
		return nil
	}

	atoms := make([]*prompt.PromptAtom, 0, len(strategies))
	for _, s := range strategies {
		if s == nil || strings.TrimSpace(s.Content) == "" {
			continue
		}
		atoms = append(atoms, strategyAtom(s))
	}
	return atoms
}

// strategyQueryFor assembles the text the problem classifier reads. The
// compiler's SemanticQuery is the task description; the intent verb and target
// are added because a bare verb ("/test") is often the only signal a delegated
// task carries.
func strategyQueryFor(cc *prompt.CompilationContext) string {
	parts := make([]string, 0, 3)
	for _, s := range []string{cc.SemanticQuery, cc.IntentTarget, cc.IntentVerb} {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			parts = append(parts, strings.TrimPrefix(trimmed, "/"))
		}
	}
	return strings.Join(parts, " ")
}

// strategyAtom renders one strategy as a methodology atom scoped to its shard.
func strategyAtom(s *Strategy) *prompt.PromptAtom {
	atom := prompt.NewPromptAtom(
		StrategyAtomIDPrefix+s.ID,
		prompt.CategoryMethodology,
		s.Content,
	)
	atom.Subcategory = "learned_strategy"
	atom.Version = s.Version
	atom.ShardTypes = []string{s.ShardType}
	atom.Priority = strategyPriority(s)
	atom.NormalizeSelectors()
	return atom
}

// strategyPriority ranks a strategy against the rest of the methodology
// corpus, which is hand-written and sits in the 80s.
//
// A learned strategy stays below that band by construction: curated
// methodology should outrank a machine-refined heuristic, and a strategy that
// could climb past it would let one bad refinement cycle displace the
// instructions that keep the agent coherent.
//
// Within the band, rank tracks the measured success rate — except for a
// strategy nobody has used yet, which gets the neutral middle rather than the
// floor. Zero uses is absence of evidence, and starting every seeded strategy
// at "proven useless" would bury the default playbook under the first
// mediocre strategy that happened to succeed once.
func strategyPriority(s *Strategy) int {
	const (
		floor   = 55
		ceiling = 75
		neutral = 62
	)
	if s == nil || s.TotalUses() == 0 {
		return neutral
	}
	rate := math.Max(0, math.Min(1, s.SuccessRate))
	return floor + int(math.Round(rate*float64(ceiling-floor)))
}

// StrategyAtomProvider must satisfy the compiler's seam.
var _ prompt.StrategyProvider = (*StrategyAtomProvider)(nil)

// NewStrategyAtomProviderFor builds a provider backed by this evolver's store
// and classifier. Returns nil when strategies are disabled, so a caller can
// register the result unconditionally.
func (pe *PromptEvolver) NewStrategyAtomProviderFor() *StrategyAtomProvider {
	if pe == nil {
		return nil
	}
	pe.mu.RLock()
	store, classifier := pe.strategyStore, pe.classifier
	pe.mu.RUnlock()
	if store == nil {
		return nil
	}
	return NewStrategyAtomProvider(store, classifier)
}

// RecordStrategyOutcome attributes one finished turn to every strategy that
// was in its prompt.
//
// This is the write-back half of the loop that makes SelectStrategies' ranking
// real: without it every strategy keeps a success rate of zero forever and the
// ordering is arbitrary.
func (pe *PromptEvolver) RecordStrategyOutcome(taskID string, atomIDs []string, success bool) {
	if pe == nil {
		return
	}
	pe.mu.RLock()
	store := pe.strategyStore
	pe.mu.RUnlock()
	if store == nil {
		return
	}

	for _, atomID := range atomIDs {
		strategyID := StrategyIDFromAtomID(atomID)
		if strategyID == "" {
			continue
		}
		if err := store.RecordOutcome(strategyID, taskID, success); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Debug(
				"Failed to record outcome for strategy %s: %v", strategyID, err)
		}
	}
}
