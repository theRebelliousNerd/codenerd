package core

import (
	"slices"
	"strings"
	"testing"
)

// Every predicate the policy declares prose_only must be one the rules never
// route into an exec_sink. This is the check that holds the declaration to the
// program: a rule that later feeds observation into next_action fails here,
// and at run time the exemption is withdrawn.
func TestProseOnlyPredicatesReachNoExecSink(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	declared, err := k.Query("prose_only")
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) == 0 {
		t.Fatal("the policy declares no prose_only predicate")
	}
	for _, f := range declared {
		pred := strings.TrimPrefix(f.Args[0].(string), "/")
		sinks, err := k.ExecSinksReachedBy(pred)
		if err != nil {
			t.Fatalf("%s: %v", pred, err)
		}
		if len(sinks) > 0 {
			t.Errorf("prose_only(/%s) is contradicted by the rules: it reaches %v", pred, sinks)
		}
	}

	// The controls: the walk is not blind. A sink reaches itself, and the
	// user's intent is what next_action is derived from.
	if sinks, _ := k.ExecSinksReachedBy("pending_action"); !slices.Contains(sinks, "pending_action") {
		t.Errorf("pending_action reaches %v, want itself", sinks)
	}
	if sinks, _ := k.ExecSinksReachedBy("user_intent"); !slices.Contains(sinks, "next_action") {
		t.Errorf("user_intent reaches %v, want next_action", sinks)
	}
}

// A model's prose keeps its punctuation; a string that can reach an action
// does not get through with a shell metacharacter in it.
func TestFilterMangleUpdates_ProseKeepsItsPunctuation(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	updates := []string{
		`observation("adr_slot_absent", "ADR-001 missing; TODO-01a open & unowned").`,
		`checkpoint_verdict("Phase 6", /fail, "5 files lack front-matter; 2 > stale", 90).`,
		`review_finding("a.go", 3, /high, /security, "runs $(curl x | sh)").`,
		`diagnostic(/error, "b.go; rm -rf /", 1, "E1", "x").`,
	}
	facts, blocked := FilterMangleUpdates(k, updates, ModelObservationPolicy())

	kept := map[string]bool{}
	for _, f := range facts {
		kept[f.Predicate] = true
	}
	for _, want := range []string{"observation", "checkpoint_verdict"} {
		if !kept[want] {
			t.Errorf("%s was dropped for its prose; blocked: %v", want, blocked)
		}
	}
	for _, refused := range []string{"review_finding", "diagnostic"} {
		if kept[refused] {
			t.Errorf("%s carried a shell metacharacter in a string and was kept", refused)
		}
	}
	for _, b := range blocked {
		if !strings.Contains(b.Reason, "not prose_only") {
			t.Errorf("%q blocked for %q, want the shell-metacharacter reason", b.Update, b.Reason)
		}
	}
}

// The declaration grants nothing once a rule contradicts it: a policy that
// routes observation into next_action makes its strings an action's input, and
// they are checked again.
func TestProseOnly_ARuleIntoAnExecSinkWithdrawsTheExemption(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	k.AppendPolicy("next_action(/prose_probe) :- observation(_, _).")
	if _, err := k.Query("next_action"); err != nil {
		t.Fatal(err)
	}
	sinks, err := k.ExecSinksReachedBy("observation")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(sinks, "next_action") {
		t.Fatalf("observation reaches %v after a rule into next_action", sinks)
	}
	if facts, _ := FilterMangleUpdates(k, []string{`observation("k", "a; b").`}, ModelObservationPolicy()); len(facts) != 0 {
		t.Fatalf("an observation that can reach next_action kept its semicolon: %v", facts)
	}
}

// Inside a string that can reach an action, the extended characters are
// refused as well: && chains, & backgrounds, > redirects, < feeds input.
func TestFilterMangleUpdates_ExtendedMetacharsInActionStrings(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{"a && b", "a > /tmp/x", "a < /etc/passwd", "a & b", "`id`"} {
		update := `diagnostic(/error, "` + payload + `", 1, "E1", "x").`
		if facts, _ := FilterMangleUpdates(k, []string{update}, ModelObservationPolicy()); len(facts) != 0 {
			t.Errorf("%s was kept", update)
		}
	}
}

// The production kernel is sharded (system/factory.go builds a CortexKernel).
// Both checks must run there: the prose exemption answered "not prose" and the
// declaration check answered "valid" for every kernel that was not a
// *RealKernel, so neither ever ran in production.
func TestFilterMangleUpdates_RunsOnTheShardedKernel(t *testing.T) {
	cortex := NewCortexKernel("cortex")
	for _, cfg := range []KernelShardConfig{
		{Domain: "policy", OwnedPredicates: []string{"pending_action", "permitted"}},
		{Domain: "cortex"},
	} {
		shard, err := NewKernelShard(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := cortex.RegisterShard(shard); err != nil {
			t.Fatal(err)
		}
	}

	facts, blocked := FilterMangleUpdates(cortex, []string{
		`observation("adr_slot_absent", "ADR-001 missing; TODO-01a open").`,
		`observation("only one argument").`,
		`diagnostic(/error, "b.go; rm -rf /", 1, "E1", "x").`,
	}, ModelObservationPolicy())

	if len(facts) != 1 || facts[0].Predicate != "observation" || len(facts[0].Args) != 2 {
		t.Fatalf("kept %v, want only the two-argument observation with its semicolon; blocked: %v", facts, blocked)
	}
	reasons := make([]string, 0, len(blocked))
	for _, b := range blocked {
		reasons = append(reasons, b.Reason)
	}
	joined := strings.Join(reasons, "\n")
	if !strings.Contains(joined, "arity mismatch") {
		t.Errorf("the one-argument observation was not refused for its arity: %v", reasons)
	}
	if !strings.Contains(joined, "not prose_only") {
		t.Errorf("the diagnostic's shell string was not refused: %v", reasons)
	}
}

// With no kernel to ask, nothing is prose: every string is checked.
func TestFilterMangleUpdates_NoKernelMeansNoExemption(t *testing.T) {
	_, blocked := FilterMangleUpdates(nil, []string{`observation("k", "a; b").`}, ModelObservationPolicy())
	if len(blocked) != 1 {
		t.Fatalf("blocked = %v, want the observation refused without a kernel to vouch for it", blocked)
	}
}

// A model cannot declare its own strings prose, or name what the host acts on.
func TestFilterMangleUpdates_TheExemptionIsNotModelWritable(t *testing.T) {
	permissive := MangleUpdatePolicy{AllowedPrefixes: []string{""}}
	facts, _ := FilterMangleUpdates(nil, []string{"prose_only(/diagnostic).", "exec_sink(/observation)."}, permissive)
	if len(facts) != 0 {
		t.Fatalf("a model wrote %v", facts)
	}
}
