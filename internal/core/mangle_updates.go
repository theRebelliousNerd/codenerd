package core

import (
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
)

// MangleUpdatePolicy constrains which control-packet updates may be asserted.
type MangleUpdatePolicy struct {
	AllowedPredicates map[string]struct{}
	AllowedPrefixes   []string
	MaxUpdates        int
}

// MangleUpdateBlock records a rejected update and the reason.
type MangleUpdateBlock struct {
	Update string
	Reason string
}

// ModelObservationPolicy is the one allowlist for facts a model volunteers
// through control_packet.mangle_updates, shared by every surface that reads
// an envelope (the session executor and the chat turn). A model may report
// what it observed and what it did; it may not write the predicates the
// kernel derives decisions from, and it may not witness its own completion
// (see predicateAllowed).
//
// "test_state" was on this list until 2026-09-18, and the mandatory prompt atom
// protocol/piggyback/mangle_updates taught the model to write it. That was
// harmless while nothing read test_state as turn evidence; it stopped being
// harmless when recordBuildState gave it a real producer and turn_verified
// started reading it, because a model writing test_state(/passing) is a model
// witnessing its own completion. It is now hard-blocked in predicateAllowed,
// and the atom no longer teaches it.
//
// The prompt atom protocol/piggyback/mangle_updates teaches exactly this set
// with its declared arities. Add a predicate here and there together.
func ModelObservationPolicy() MangleUpdatePolicy {
	return MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"missing_tool_for":  {},
			"observation":       {},
			"task_status":       {},
			"task_completed":    {},
			"diagnostic":        {},
			"failing_test":      {},
			"review_finding":    {},
			"modified":          {},
			"modified_function": {},
			// checkpoint_verdict/4 is the campaign checkpoint's structured
			// reviewer verdict. internal/campaign/checkpoint.go queries the
			// kernel for it after the reviewer spawn and retracts it once
			// read, so it decides one checkpoint and nothing else. Without
			// this entry the verdict was blocked and every checkpoint
			// failed closed (audit campaign 5a2f4c8d, 2026-09-04).
			"checkpoint_verdict": {},
		},
		MaxUpdates: 100,
	}
}

// FilterMangleUpdates parses and validates control-packet mangle_updates.
// It rejects rules/decls/imports, enforces allowed predicates/prefixes, and
// validates predicate declarations/arity when possible.
func FilterMangleUpdates(kernel Kernel, updates []string, policy MangleUpdatePolicy) ([]Fact, []MangleUpdateBlock) {
	if len(updates) == 0 {
		return nil, nil
	}

	maxUpdates := policy.MaxUpdates
	if maxUpdates <= 0 {
		maxUpdates = len(updates)
	}

	factCap := min(len(updates), maxUpdates)
	facts := make([]Fact, 0, factCap)
	blocked := make([]MangleUpdateBlock, 0)

	for i, update := range updates {
		if i >= maxUpdates {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "mangle_updates limit exceeded",
			})
			continue
		}

		trimmed := strings.TrimSpace(update)
		if trimmed == "" {
			continue
		}

		if strings.Contains(trimmed, ":-") {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "rules are not allowed in mangle_updates",
			})
			continue
		}

		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "decl ") || strings.HasPrefix(lower, "decl\t") {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "Decl statements are not allowed in mangle_updates",
			})
			continue
		}
		if strings.HasPrefix(lower, "import ") || strings.HasPrefix(lower, "include ") {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: "imports are not allowed in mangle_updates",
			})
			continue
		}

		factText := strings.TrimSuffix(trimmed, ".")
		fact, err := ParseFactString(factText)
		if err != nil {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: fmt.Sprintf("parse error: %v", err),
			})
			continue
		}

		if !predicateAllowed(fact.Predicate, policy) {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: fmt.Sprintf("predicate %q not allowed in mangle_updates", fact.Predicate),
			})
			continue
		}

		if kernel != nil {
			if ok, reason := validatePredicateDeclaration(kernel, fact.Predicate, len(fact.Args)); !ok {
				blocked = append(blocked, MangleUpdateBlock{
					Update: update,
					Reason: reason,
				})
				continue
			}
		}

		if arg, found := shellEscapingArg(fact); found && !proseOnly(kernel, fact.Predicate) {
			blocked = append(blocked, MangleUpdateBlock{
				Update: update,
				Reason: fmt.Sprintf("shell metacharacters in a string argument of %s (%q), which is not prose_only", fact.Predicate, arg),
			})
			continue
		}

		facts = append(facts, fact)
	}

	return facts, blocked
}

// shellMetacharacters are the characters that let text escape into a shell:
// substitution, chaining, backgrounding, pipes and redirects. (The airtight
// fix lives at the exec site -- never interpolate a fact into a shell -- but a
// hostile string should not reach the kernel unremarked on its way there.)
const shellMetacharacters = "`$;|&<>"

// shellEscapingArg returns the first string argument of fact that carries a
// shell metacharacter. Names cannot carry one; composite constants arrive
// rendered as strings and are checked whole.
func shellEscapingArg(fact Fact) (string, bool) {
	for _, arg := range fact.Args {
		if s, ok := arg.(string); ok && strings.ContainsAny(s, shellMetacharacters) {
			return s, true
		}
	}
	return "", false
}

// proseOnly reports whether a model may put shell metacharacters in the
// string arguments of predicate. It is a property of the predicate, not a
// list here: the policy declares it (prose_only/1, constitution.mg), and the
// program must agree -- no rule may route the predicate into an exec_sink.
// A declaration the rules contradict grants nothing, and anything the kernel
// cannot answer is not prose.
func proseOnly(kernel Kernel, predicate string) bool {
	graph, ok := kernel.(execReachability)
	if !ok || graph == nil {
		return false
	}
	declared, err := kernel.Query(fmt.Sprintf("prose_only(/%s)", predicate))
	if err != nil || len(declared) == 0 {
		return false
	}
	sinks, err := graph.ExecSinksReachedBy(predicate)
	if err != nil {
		logging.Get(logging.CategoryKernel).Warn("prose_only(/%s) not granted: %v", predicate, err)
		return false
	}
	if len(sinks) > 0 {
		logging.Get(logging.CategoryKernel).Error(
			"prose_only(/%s) is contradicted by the rules: its facts reach exec_sink %v; its strings stay checked", predicate, sinks)
		return false
	}
	return true
}

// execReachability is a kernel that can say where a predicate's facts flow:
// the single-store RealKernel and the sharded CortexKernel production runs.
type execReachability interface {
	ExecSinksReachedBy(predicate string) ([]string, error)
}

var (
	_ execReachability = (*RealKernel)(nil)
	_ execReachability = (*CortexKernel)(nil)
)

// ExecSinksReachedBy returns the exec_sink predicates (constitution.mg) that
// facts of predicate can contribute to through the program's rules, the
// predicate itself included when it is one. The walk is over the rule
// dependency graph, negated premises included, so it over-approximates where a
// string can flow; it never under-approximates.
func (k *RealKernel) ExecSinksReachedBy(predicate string) ([]string, error) {
	declared, err := k.Query("exec_sink")
	if err != nil {
		return nil, fmt.Errorf("exec_sink query: %w", err)
	}
	if len(declared) == 0 {
		return nil, fmt.Errorf("the policy declares no exec_sink")
	}
	k.mu.RLock()
	cone := k.cone
	k.mu.RUnlock()
	if cone == nil {
		return nil, fmt.Errorf("no rule-dependency index")
	}
	reached := cone.downstream(predicate)
	reached[predicate] = struct{}{}
	var hit []string
	for _, f := range declared {
		if len(f.Args) == 0 {
			continue
		}
		sink := strings.TrimPrefix(fmt.Sprint(f.Args[0]), "/")
		if _, ok := reached[sink]; ok {
			hit = append(hit, sink)
		}
	}
	sort.Strings(hit)
	return hit, nil
}

// hostWitnessPredicates are host witnesses and conclusions, never model
// observations. Two gates read this one set, so they cannot drift apart:
// predicateAllowed refuses them in a control packet, and the kernel's
// learned-rule validator refuses them as the head of a learned rule
// (installLearnedHeadProtection). A model that may not write
// build_state(/passing) may not learn a rule that derives it either.
//
// build_state and test_state are on this list because they are what the
// session executor's own post-edit gates recorded (recordBuildState, from
// BuildCheck/TestCheck): the compiler and the test runner, measured by the
// host. Since turn_verified reads them (coder_safety.mg), a model able to
// write build_state(/passing) could manufacture its own completion. Two
// routes existed: the SessionPlanner's "build_" prefix allowlist
// (internal/shards/system/planner.go), and ModelObservationPolicy, which
// named "test_state" outright on the two busiest surfaces there are — the
// session executor and the chat turn.
var hostWitnessPredicates = map[string]struct{}{
	"turn_acceptance": {}, "turn_evidence": {}, "turn_executed": {}, "turn_done": {}, "turn_cost": {},
	"turn_verified": {}, "turn_unverified": {}, "turn_wrote": {}, "turn_build_failed": {},
	"turn_missing_evidence": {}, "turn_created_source": {}, "has_turn_acceptance": {},
	"turn_gate": {}, "turn_build_green": {}, "turn_build_red": {}, "turn_tests_green": {}, "turn_tests_red": {},
	"turn_untested": {}, "turn_has_untested": {},
	"turn_uncovered": {}, "turn_has_uncovered": {}, "turn_vet_green": {}, "turn_vet_red": {},
	"turn_created_test": {}, "turn_test_coverage": {}, "turn_missing_test": {},
	// What a write owes (N01): write_class is the table a model could
	// otherwise extend to reclassify its own code as documentation.
	"turn_written": {}, "write_class": {}, "known_write_ext": {}, "has_turn_written": {}, "turn_write_class": {},
	"turn_owes_gate": {}, "turn_unmet_gate": {}, "turn_red_gate": {}, "has_unmet_gate": {}, "has_red_gate": {},
	// What a behaviour change owes (N22): the turn's intent and the table
	// of intents that owe /pinned.
	"turn_verb": {}, "behavior_change_intent": {},
	// The turn's own report admitting unfinished work (Q-14): a model able to
	// retract or never assert it would decide its own verdict; asserting it
	// only withholds, but it is the host's reading, not the model's.
	"turn_self_reported_incomplete": {},
	// The pin gate's survivor count schedules the /survivors round.
	"turn_pin_survivors": {},
	// What the planner is spent on: the sites the host measured in the
	// brief, and the conclusion drawn from them.
	"turn_brief_site": {}, "turn_brief_site_count": {}, "turn_needs_step_plan": {},
	// What a repair episode does next (repair_episode.mg): a model that
	// could write repair_attempt could keep its own episode alive.
	"repair_attempt": {}, "repair_attempt_count": {}, "repair_exhausted": {}, "repair_not_converging": {},
	"repair_gives_up": {}, "repair_move": {}, "repair_closed": {},
	// What a failed checkpoint leads to (campaign_decisions.mg): a model
	// that could write phase_checkpoint_failure could close or keep open
	// its own phase.
	"phase_checkpoint_failure": {}, "phase_ckpt_failures": {}, "phase_ckpt_exhausted": {}, "phase_ckpt_move": {},
	// The user's thresholds (config_params.mg): a model that could write
	// config_param could raise its own attempt cap or lower a gate's bar.
	"config_param": {}, "config_param_required": {},
	"hollow_success": {}, "has_hollow_success": {}, "has_turn_tools": {}, "has_turn_write": {}, "has_turn_test": {},
	"build_state": {}, "test_state": {},
	// What may carry unchecked strings, and what the host acts on: a
	// model that could write either could exempt its own strings.
	"prose_only": {}, "exec_sink": {},
}

func predicateAllowed(predicate string, policy MangleUpdatePolicy) bool {
	// A host witness is refused BEFORE AllowedPredicates and AllowedPrefixes
	// are read, precisely so no caller allowlist can delegate its authority.
	if _, hostOnly := hostWitnessPredicates[predicate]; hostOnly {
		return false
	}
	if len(policy.AllowedPredicates) == 0 && len(policy.AllowedPrefixes) == 0 {
		return true
	}
	if _, ok := policy.AllowedPredicates[predicate]; ok {
		return true
	}
	for _, prefix := range policy.AllowedPrefixes {
		if strings.HasPrefix(predicate, prefix) {
			return true
		}
	}
	return false
}

// validatePredicateDeclaration checks a model-written fact against the
// program's declarations, read through the Kernel interface so it runs on the
// sharded production kernel too. It used to type-assert *RealKernel and answer
// "valid" for anything else -- which on CortexKernel was every model fact, so
// arity and declaration were never checked in production (found 2026-09-23).
func validatePredicateDeclaration(kernel Kernel, predicate string, arity int) (bool, string) {
	programInfo := kernel.GetProgramInfo()
	var schemaValidator *mangle.SchemaValidator
	if rk, ok := kernel.(*RealKernel); ok && rk != nil {
		rk.mu.RLock()
		schemaValidator = rk.schemaValidator
		rk.mu.RUnlock()
	}

	if programInfo != nil && programInfo.Decls != nil {
		for predSym := range programInfo.Decls {
			if predSym.Symbol != predicate {
				continue
			}
			if predSym.Arity != arity {
				return false, fmt.Sprintf("arity mismatch: %s expects %d args (got %d)", predicate, predSym.Arity, arity)
			}
			return true, ""
		}
		return false, fmt.Sprintf("predicate %q not declared", predicate)
	}

	if schemaValidator != nil && !schemaValidator.IsDeclared(predicate) {
		return false, fmt.Sprintf("predicate %q not declared", predicate)
	}

	return true, ""
}
