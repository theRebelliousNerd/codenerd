package session

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// catalogTurnKey carries the turn atom prepareTurnCatalog minted, so the
// catalog compile (resolveAvailableTools, compileConfig) reads
// turn_catalog(Turn, Tool) for this turn. A field on the executor would not
// survive CloneForTask, and the two compilers only have the context.
type catalogTurnKey struct{}

func withCatalogTurn(ctx context.Context, turn types.MangleAtom) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if turn == "" {
		return ctx
	}
	return context.WithValue(ctx, catalogTurnKey{}, turn)
}

func catalogTurnFrom(ctx context.Context) (types.MangleAtom, bool) {
	if ctx == nil {
		return "", false
	}
	turn, ok := ctx.Value(catalogTurnKey{}).(types.MangleAtom)
	if !ok || turn == "" {
		return "", false
	}
	return turn, true
}

// prepareTurnCatalog asserts the facts turn_catalog and the /check gate read
// for this turn: turn_verb, and turn_declared_check when the context carries
// a campaign acceptance command. The policy decides whether that offers
// run_check and whether the write owes /check. A precompiled config is not
// re-derived from these facts; they still stand so the closure can see the
// check on the execution turn, which is a different atom from the one the
// spawner used to freeze the catalog.
func (e *Executor) prepareTurnCatalog(ctx context.Context, result *ExecutionResult, verb string) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	turn := result.turnAtom()
	e.assertTurnVerb(turn, prompt.CanonicalTurnVerb(verb), result)
	if result.checkDeclared {
		return
	}
	if _, ok := tools.CampaignCheckFrom(ctx); !ok {
		return
	}
	if e.assertTurnFact(types.Fact{Predicate: "turn_declared_check", Args: []any{turn}}) {
		result.checkDeclared = true
	}
}

// deriveSpawnTools is the spawner's one catalog read. The subagent's prompt
// is compiled here and the executor later keeps that precompiled allowlist,
// so the check has to be visible to this query or an isolated coder turn
// never sees run_check. The facts are keyed by a temporary turn and retracted
// before return: the execution turn asserts its own, and RetractFact matches
// the predicate plus this turn id, so a concurrent spawn's facts stay.
func (s *Spawner) deriveSpawnTools(ctx context.Context, verb string) ([]string, error) {
	if s == nil || s.kernel == nil {
		return prompt.DeriveTurnTools(nil, verb)
	}
	turn := newTurnAtom()
	facts := []types.Fact{{
		Predicate: "turn_verb",
		Args:      []any{turn, types.MangleAtom(prompt.CanonicalTurnVerb(verb))},
	}}
	if _, ok := tools.CampaignCheckFrom(ctx); ok {
		facts = append(facts, types.Fact{
			Predicate: "turn_declared_check",
			Args:      []any{turn},
		})
	}
	asserted := make([]types.Fact, 0, len(facts))
	for _, fact := range facts {
		if err := s.kernel.Assert(fact); err != nil {
			retractSpawnCatalogFacts(s.kernel, asserted)
			return nil, fmt.Errorf("turn catalog: assert %s: %w", fact.Predicate, err)
		}
		asserted = append(asserted, fact)
	}
	defer retractSpawnCatalogFacts(s.kernel, asserted)
	return prompt.DeriveTurnTools(s.kernel, verb, string(turn))
}

func retractSpawnCatalogFacts(kernel types.Kernel, facts []types.Fact) {
	if kernel == nil {
		return
	}
	for _, fact := range facts {
		if err := kernel.RetractFact(fact); err != nil {
			logging.Get(logging.CategorySession).Warn("turn catalog: failed to retract spawn %s%v: %v", fact.Predicate, fact.Args, err)
		}
	}
}

// assertTurnCheckRun records one run_check receipt. The /check gate does not
// read it; recordBuildState asserts turn_gate from CheckSinceLastWrite, the
// same way /test_run is decided from TestRunSinceLastWrite.
func (e *Executor) assertTurnCheckRun(result *ExecutionResult, exitCode int) {
	if e == nil || e.kernel == nil || result == nil {
		return
	}
	e.assertTurnFact(types.Fact{
		Predicate: "turn_check_run",
		Args:      []any{result.turnAtom(), int64(result.checkSeq), int64(exitCode)},
	})
}

// checkVerdict is the /check gate's verdict: passed when the last run_check
// since the turn's last write exited 0, failed when it exited otherwise,
// skipped -- no verdict -- when none ran since it.
func (r *ExecutionResult) checkVerdict() VerifyOutcome {
	switch {
	case r.CheckSinceLastWrite == nil:
		return VerifySkipped
	case r.CheckSinceLastWrite.ExitCode == 0:
		return VerifyPassed
	default:
		return VerifyFailed
	}
}
