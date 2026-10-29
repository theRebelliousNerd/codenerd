package autopoiesis

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// Tool generation is admitted against the request it serves. Generating a
// tool writes its source under the workspace and runs the compiler -- write
// and execute effects -- and a request that restricts changes ("read only",
// "dry run", "don't change anything") bars both. The chat's residual
// information-gathering path reached GenerateTool on a restricted /explain
// turn with no admission at all (audit
// residual-tool-generation-restriction-admission); code safety checks and
// simulation judge the generated code, not whether this requester allows it.
//
// The judgment is the kernel's, the same rules the session executor's
// restricted turns obey (policy/constitution.mg: turn_restriction,
// effect_barring_kind, turn_bars_effects). Detection is admitted too: it
// publishes a missing_tool_for observation that the policy reads as a reason
// to generate. A context carrying no restriction -- background maintenance,
// a delegated generation -- is not a user request's, and passes.

var admissionSeq atomic.Uint64

// ErrGenerationRestricted marks a generation the request's restriction bars.
var ErrGenerationRestricted = fmt.Errorf("tool generation refused by the request's restriction")

// admitGeneration refuses when ctx carries a restriction that bars write or
// execute effects.
func (o *Orchestrator) admitGeneration(ctx context.Context) error {
	r := perception.RestrictionFromContext(ctx)
	if !r.Enforced() {
		return nil
	}
	o.mu.RLock()
	k := o.kernel
	o.mu.RUnlock()
	if k == nil {
		return fmt.Errorf("%w: the request is restricted (%v) and no kernel is attached to judge it", ErrGenerationRestricted, r.Kinds())
	}
	turn := types.MangleAtom(fmt.Sprintf("/toolgen_admission_%d", admissionSeq.Add(1)))
	facts := make([]types.Fact, 0, len(r.Kinds()))
	for _, kind := range r.Kinds() {
		facts = append(facts, types.Fact{Predicate: "turn_restriction", Args: []any{turn, types.MangleAtom("/" + string(kind))}})
	}
	if err := k.AssertBatch(facts); err != nil {
		return fmt.Errorf("%w: the restriction could not be recorded: %v", ErrGenerationRestricted, err)
	}
	rows, qerr := k.Query("turn_bars_effects")
	_ = k.RetractExactFactsBatch(facts)
	if qerr != nil {
		return fmt.Errorf("%w: turn_bars_effects could not be read: %v", ErrGenerationRestricted, qerr)
	}
	for _, row := range rows {
		if len(row.Args) > 0 && types.ExtractString(row.Args[0]) == string(turn) {
			kinds := make([]string, 0, len(r.Kinds()))
			for _, kind := range r.Kinds() {
				kinds = append(kinds, string(kind))
			}
			return fmt.Errorf("%w: the request says %s, and generating a tool writes its source and runs the compiler", ErrGenerationRestricted, strings.Join(kinds, ", "))
		}
	}
	return nil
}
