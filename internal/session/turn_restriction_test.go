package session

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/approval"
	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// These drive the registered executor path with the production constitution,
// a real kernel and counting tools under the reviewed names. A tool's handler
// count is the evidence of dispatch: a catalog entry that is absent, or a
// refusal that reads well, proves nothing.

type restrictionRig struct {
	exec      *Executor
	kernel    *core.RealKernel
	writes    *atomic.Int64
	reads     *atomic.Int64
	workspace string
}

func newRestrictionRig(t *testing.T) *restrictionRig {
	t.Helper()
	rig := &restrictionRig{writes: &atomic.Int64{}, reads: &atomic.Int64{}, workspace: t.TempDir()}
	registerTestTool(t, &tools.Tool{Name: "write_file", Effect: tools.EffectWrite, Description: "counting write",
		Execute: func(context.Context, map[string]any) (string, error) { rig.writes.Add(1); return "wrote", nil }})
	registerTestTool(t, &tools.Tool{Name: "read_file", Effect: tools.EffectRead, Description: "counting read",
		Execute: func(context.Context, map[string]any) (string, error) { rig.reads.Add(1); return "read", nil }})
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	rig.kernel = k
	rig.exec = NewExecutor(k, &testExecutiveStore{}, nil, nil, nil, nil)
	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = rig.workspace
	rig.exec.SetConfig(cfg)
	rig.exec.SetSessionID("s-restriction")
	return rig
}

// turnCtx is the context a turn runs under: its restriction and its turn
// identity, with the restriction's facts told to the kernel exactly as
// ProcessWithIntent does.
func (r *restrictionRig) turnCtx(t *testing.T, u perception.Understanding) context.Context {
	t.Helper()
	restriction := perception.RestrictionFrom(&u)
	turn := newTurnAtom()
	ctx := withCatalogTurn(perception.WithRestriction(context.Background(), restriction), turn)
	if err := r.exec.assertTurnRestriction(turn, restriction); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.exec.cleanupTurnFacts)
	return ctx
}

func restrictedWriteCall(id string) ToolCall {
	return ToolCall{ID: id, Name: "write_file", Args: map[string]any{"path": "out.txt", "content": "hello"}}
}
func restrictedReadCall(id string) ToolCall {
	return ToolCall{ID: id, Name: "read_file", Args: map[string]any{"path": "out.txt"}}
}

func (r *restrictionRig) requireNoAuthorityLeft(t *testing.T) {
	t.Helper()
	for _, p := range []string{"pending_action", "human_approval_granted", "pending_action_turn", "pending_action_effect"} {
		rows, err := r.kernel.Query(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 0 {
			t.Fatalf("%s survived the admission: %v", p, rows)
		}
	}
}

// Modify with no changes allowed: the forbidden write never reaches its
// handler, and permissible reading still works under the same restriction.
func TestRestriction_NoChangesNeverDispatchesAWrite(t *testing.T) {
	rig := newRestrictionRig(t)
	ctx := rig.turnCtx(t, perception.Understanding{UserConstraints: []string{"no_changes"}})
	// The catalog is deliberately permissive: admission is the line under test.
	cfg := validCapabilityTestConfig("write_file", "read_file")

	_, err := rig.exec.executeToolCall(ctx, restrictedWriteCall("w1"), cfg)
	if err == nil || !strings.Contains(err.Error(), "restricts this turn") {
		t.Fatalf("a write under no_changes = %v, want the restriction named", err)
	}
	if got := rig.writes.Load(); got != 0 {
		t.Fatalf("the forbidden write reached its handler %d time(s)", got)
	}
	if _, err := rig.exec.executeToolCall(ctx, restrictedReadCall("r1"), cfg); err != nil {
		t.Fatalf("a read under no_changes was refused: %v", err)
	}
	if got := rig.reads.Load(); got != 1 {
		t.Fatalf("the permissible read ran %d time(s), want 1", got)
	}
	rig.requireNoAuthorityLeft(t)
}

// Every barred kind bars the same effects; the receipt keeps the user's word.
func TestRestriction_EveryBarringKindRefusesTheWrite(t *testing.T) {
	for _, u := range []perception.Understanding{
		{UserConstraints: []string{"read_only"}},
		{UserConstraints: []string{"dry_run"}},
		{Signals: perception.Signals{IsHypothetical: true}},
	} {
		rig := newRestrictionRig(t)
		ctx := rig.turnCtx(t, u)
		if _, err := rig.exec.executeToolCall(ctx, restrictedWriteCall("w"), validCapabilityTestConfig("write_file")); err == nil {
			t.Fatalf("%+v: the write was admitted", u)
		}
		if rig.writes.Load() != 0 {
			t.Fatalf("%+v: the write dispatched", u)
		}
	}
}

// Nothing a retry, an alias of the same tuple, or a later call does lifts it.
func TestRestriction_RetryDoesNotLiftIt(t *testing.T) {
	rig := newRestrictionRig(t)
	ctx := rig.turnCtx(t, perception.Understanding{UserConstraints: []string{"no_changes"}})
	for i := 0; i < 3; i++ {
		if _, err := rig.exec.executeToolCall(ctx, restrictedWriteCall("same-call"), validCapabilityTestConfig("write_file")); err == nil {
			t.Fatalf("attempt %d was admitted", i)
		}
	}
	if rig.writes.Load() != 0 {
		t.Fatal("a retry dispatched the write")
	}
}

// A restricted turn followed by an authorized edit: no stale over-blocking,
// and a sibling turn is not held to this turn's limit.
func TestRestriction_OtherTurnsAreNotHeldToIt(t *testing.T) {
	rig := newRestrictionRig(t)
	restricted := rig.turnCtx(t, perception.Understanding{UserConstraints: []string{"no_changes"}})
	cfg := validCapabilityTestConfig("write_file")
	if _, err := rig.exec.executeToolCall(restricted, restrictedWriteCall("a"), cfg); err == nil {
		t.Fatal("the restricted write was admitted")
	}
	// The next request is unrestricted: same executor, same kernel.
	free := withCatalogTurn(context.Background(), newTurnAtom())
	if _, err := rig.exec.executeToolCall(free, restrictedWriteCall("b"), cfg); err != nil {
		t.Fatalf("an unrestricted turn after a restricted one was blocked: %v", err)
	}
	if rig.writes.Load() != 1 {
		t.Fatalf("writes = %d, want exactly the unrestricted one", rig.writes.Load())
	}
	// Concurrent differently restricted turns: each action uses its owner's.
	other := rig.turnCtx(t, perception.Understanding{})
	if _, err := rig.exec.executeToolCall(other, restrictedWriteCall("c"), cfg); err != nil {
		t.Fatalf("a turn with no restriction borrowed a sibling's: %v", err)
	}
}

// A restricted context with no turn identity cannot be enforced against, so
// the call is refused rather than run unrestricted.
func TestRestriction_NoTurnIdentityFailsClosed(t *testing.T) {
	rig := newRestrictionRig(t)
	ctx := perception.WithRestriction(context.Background(), perception.RestrictionFrom(&perception.Understanding{UserConstraints: []string{"no_changes"}}))
	if _, err := rig.exec.executeToolCall(ctx, restrictedWriteCall("w"), validCapabilityTestConfig("write_file")); err == nil {
		t.Fatal("a restricted context with no turn ran a write")
	}
	if rig.writes.Load() != 0 {
		t.Fatal("the write dispatched")
	}
}

// The catalog is the first line: a barred tool is not offered, a read is.
func TestRestriction_CatalogWithholdsBarredEffects(t *testing.T) {
	rig := newRestrictionRig(t)
	turn := newTurnAtom()
	t.Cleanup(rig.exec.cleanupTurnFacts)
	r := perception.RestrictionFrom(&perception.Understanding{UserConstraints: []string{"read_only"}})
	if err := rig.exec.assertTurnRestriction(turn, r); err != nil {
		t.Fatal(err)
	}
	candidates := []string{"read_file", "write_file", "edit_file", "run_command", "run_tests", "search_code", "web_fetch", "mystery_tool_without_effect"}
	withheld := rig.exec.restrictionWithheldTools(turn, r, candidates)
	for _, barred := range []string{"write_file", "edit_file", "run_command", "run_tests", "mystery_tool_without_effect"} {
		if !slices.Contains(withheld, barred) {
			t.Errorf("%s was left in a read-only catalog (withheld %v)", barred, withheld)
		}
	}
	for _, kept := range []string{"read_file", "search_code", "web_fetch"} {
		if slices.Contains(withheld, kept) {
			t.Errorf("%s was withheld although its effect is not barred", kept)
		}
	}
	if got := rig.exec.restrictionWithheldTools(turn, perception.Restriction{}, candidates); len(got) != 0 {
		t.Fatalf("an unrestricted turn lost tools: %v", got)
	}
	confirm := perception.RestrictionFrom(&perception.Understanding{UserConstraints: []string{"confirm_first"}})
	turn2 := newTurnAtom()
	if err := rig.exec.assertTurnRestriction(turn2, confirm); err != nil {
		t.Fatal(err)
	}
	if got := rig.exec.restrictionWithheldTools(turn2, confirm, candidates); len(got) != 0 {
		t.Fatalf("confirm_first withholds tools (it owes consent, it does not forbid): %v", got)
	}
}

// An effect nobody declared is not read-only.
func TestRestriction_UnknownEffectIsNotAssumedReadOnly(t *testing.T) {
	if got := effectAtom("no_such_tool_declared_anywhere"); got != "/unknown" {
		t.Fatalf("an undeclared tool's effect = %q, want /unknown", got)
	}
	rig := newRestrictionRig(t)
	turn := newTurnAtom()
	t.Cleanup(rig.exec.cleanupTurnFacts)
	if err := rig.exec.assertTurnRestriction(turn, perception.RestrictionFrom(&perception.Understanding{UserConstraints: []string{"no_changes"}})); err != nil {
		t.Fatal(err)
	}
	// A call whose effect was never measured is barred too.
	id := "exec-unmeasured"
	for _, f := range []types.Fact{
		{Predicate: "pending_action", Args: []any{id, types.MangleAtom("/read_file"), "x", "{}", int64(1)}},
		{Predicate: "pending_action_turn", Args: []any{id, turn}},
	} {
		if err := rig.kernel.Assert(f); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := rig.kernel.Query("permitted(/read_file, \"x\", \"{}\")")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatal("an unmeasured call under a barring restriction was permitted")
	}
}

type confirmRig struct {
	*restrictionRig
	broker *approval.Broker
	host   *approval.Host
}

func newConfirmRig(t *testing.T, attachHost bool) *confirmRig {
	t.Helper()
	base := newRestrictionRig(t)
	b, err := approval.NewBroker(approval.Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour,
		JournalPath: filepath.Join(base.workspace, ".nerd", "approvals", "journal.jsonl")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	base.exec.SetApprovalBroker(b)
	rig := &confirmRig{restrictionRig: base, broker: b}
	if attachHost {
		h, err := b.AttachHost("tui:test", "operator")
		if err != nil {
			t.Fatal(err)
		}
		rig.host = h
	}
	return rig
}

func (r *confirmRig) run(ctx context.Context, call ToolCall) <-chan callResult {
	done := make(chan callResult, 1)
	go func() {
		out, err := r.exec.executeToolCall(ctx, call, validCapabilityTestConfig("write_file", "read_file"))
		done <- callResult{out, err}
	}()
	return done
}

func (r *confirmRig) awaitRequest(t *testing.T, id string) approval.Request {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case req := <-r.host.Updates():
			if req.IntentID == "exec-"+id && req.Status == approval.StatusAwaitingHuman {
				return req
			}
		case <-deadline:
			t.Fatal("no approval request was opened")
		}
	}
}

func (r *confirmRig) decide(t *testing.T, req approval.Request, v approval.Verdict, event string) {
	t.Helper()
	if _, err := r.host.Decide(approval.DecisionInput{RequestID: req.ID, RequestVersion: req.Version, RequestDigest: req.Digest, EventID: event, Verdict: v}); err != nil {
		t.Fatalf("decide: %v", err)
	}
}

// "Modify this file, but confirm with me first": an ordinary write waits for
// the person to approve the frozen request, and does not dispatch before.
func TestRestriction_ConfirmFirstWaitsForThePerson(t *testing.T) {
	rig := newConfirmRig(t, true)
	ctx := rig.turnCtx(t, perception.Understanding{Signals: perception.Signals{RequiresConfirmation: true}})
	done := rig.run(ctx, restrictedWriteCall("c1"))
	req := rig.awaitRequest(t, "c1")
	if req.Subject.Action != "/write_file" || req.Subject.Effect != string(tools.EffectWrite) {
		t.Fatalf("the request does not freeze the call: %+v", req.Subject)
	}
	select {
	case res := <-done:
		t.Fatalf("the call finished before the decision: %+v", res)
	case <-time.After(100 * time.Millisecond):
	}
	if rig.writes.Load() != 0 {
		t.Fatal("the write dispatched before the person decided")
	}
	rig.decide(t, req, approval.VerdictApprove, "ev-1")
	if res := <-done; res.err != nil {
		t.Fatalf("approved write failed: %v", res.err)
	}
	if rig.writes.Load() != 1 {
		t.Fatalf("approved write dispatched %d times, want 1", rig.writes.Load())
	}
	rig.requireNoAuthorityLeft(t)
}

func TestRestriction_ConfirmFirstRejectionAndNoHostNeverDispatch(t *testing.T) {
	rig := newConfirmRig(t, true)
	ctx := rig.turnCtx(t, perception.Understanding{UserConstraints: []string{"confirm_first"}})
	done := rig.run(ctx, restrictedWriteCall("c1"))
	rig.decide(t, rig.awaitRequest(t, "c1"), approval.VerdictReject, "ev-1")
	if res := <-done; !errors.Is(res.err, approval.ErrRejected) {
		t.Fatalf("rejected call = %v", res.err)
	}
	if rig.writes.Load() != 0 {
		t.Fatal("a rejected write dispatched")
	}

	// No approval host: the effect stays unexecuted with an actionable result.
	headless := newConfirmRig(t, false)
	ctx = headless.turnCtx(t, perception.Understanding{UserConstraints: []string{"confirm_first"}})
	_, err := headless.exec.executeToolCall(ctx, restrictedWriteCall("h1"), validCapabilityTestConfig("write_file"))
	if err == nil || !strings.Contains(err.Error(), "no approval host is attached") {
		t.Fatalf("unattended confirm_first = %v, want an actionable refusal", err)
	}
	if headless.writes.Load() != 0 {
		t.Fatal("an unattended confirm_first write dispatched")
	}
}

// The standing yolo instruction is the operator's general permission to
// proceed; the user's "confirm first" on this request is more specific and
// outranks it. A yolo broker does not decide it.
func TestRestriction_YoloDoesNotDischargeConfirmFirst(t *testing.T) {
	rig := newConfirmRig(t, true)
	rig.broker.SetAutonomous(true)
	ctx := rig.turnCtx(t, perception.Understanding{UserConstraints: []string{"confirm_first"}})
	done := rig.run(ctx, restrictedWriteCall("y1"))
	req := rig.awaitRequest(t, "y1")
	if req.Channel == approval.AutonomyChannel {
		t.Fatal("the standing yolo instruction decided a confirm_first request")
	}
	if rig.writes.Load() != 0 {
		t.Fatal("yolo let a confirm_first write through")
	}
	rig.decide(t, req, approval.VerdictApprove, "ev-1")
	if res := <-done; res.err != nil {
		t.Fatal(res.err)
	}
	if rig.writes.Load() != 1 {
		t.Fatalf("writes = %d, want 1", rig.writes.Load())
	}

	// Control: without the user's request-level limit, yolo still proceeds.
	free := withCatalogTurn(context.Background(), newTurnAtom())
	rig.broker.SetAutonomous(true)
	if _, err := rig.exec.executeToolCall(free, restrictedWriteCall("y2"), validCapabilityTestConfig("write_file")); err != nil {
		t.Fatalf("an unrestricted write was refused: %v", err)
	}
}

// confirm_first must not make a barred operation approvable.
func TestRestriction_ConsentDoesNotLiftABar(t *testing.T) {
	rig := newConfirmRig(t, true)
	ctx := rig.turnCtx(t, perception.Understanding{
		UserConstraints: []string{"no_changes", "confirm_first"},
	})
	_, err := rig.exec.executeToolCall(ctx, restrictedWriteCall("b1"), validCapabilityTestConfig("write_file"))
	if err == nil || !strings.Contains(err.Error(), "restricts this turn") {
		t.Fatalf("barred+confirm write = %v, want the bar, not an approval request", err)
	}
	select {
	case req := <-rig.host.Updates():
		t.Fatalf("a person was asked to approve a barred write: %+v", req)
	case <-time.After(100 * time.Millisecond):
	}
	if rig.writes.Load() != 0 {
		t.Fatal("the barred write dispatched")
	}
}

// Reads owe no consent under confirm_first.
func TestRestriction_ConfirmFirstDoesNotGateReading(t *testing.T) {
	rig := newConfirmRig(t, true)
	ctx := rig.turnCtx(t, perception.Understanding{UserConstraints: []string{"confirm_first"}})
	if _, err := rig.exec.executeToolCall(ctx, restrictedReadCall("r1"), validCapabilityTestConfig("read_file")); err != nil {
		t.Fatalf("a read under confirm_first was gated: %v", err)
	}
	if rig.reads.Load() != 1 {
		t.Fatal("the read did not run")
	}
}

// A delegated task is not re-perceived, so it must inherit its parent's limit
// from the context it runs under, and the model it consults must be offered
// only the tools that limit leaves.
func TestProcessWithIntent_PresetTaskInheritsTheParentsRestriction(t *testing.T) {
	registerTestTool(t, &tools.Tool{Name: "write_file", Effect: tools.EffectWrite, Description: "w",
		Execute: func(context.Context, map[string]any) (string, error) { return "w", nil }})
	registerTestTool(t, &tools.Tool{Name: "read_file", Effect: tools.EffectRead, Description: "r",
		Execute: func(context.Context, map[string]any) (string, error) { return "r", nil }})

	run := func(t *testing.T, ctx context.Context, preset *perception.Intent) (offered []string, k *core.RealKernel, res *ExecutionResult) {
		t.Helper()
		k, err := core.NewRealKernel()
		if err != nil {
			t.Fatal(err)
		}
		llm := &MockLLMClient{
			CompleteWithToolsFunc: func(_ context.Context, _, _ string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
				for _, d := range defs {
					offered = append(offered, d.Name)
				}
				return &types.LLMToolResponse{Text: "done"}, nil
			},
			CompleteWithSystemFunc: func(context.Context, string, string) (string, error) { return "done", nil },
		}
		cf := &MockConfigFactory{GenerateFunc: func(context.Context, *prompt.CompilationResult, ...string) (*config.EffectiveAgentRuntimeConfig, error) {
			return validCapabilityTestConfig("read_file", "write_file"), nil
		}}
		e := NewExecutor(k, &testExecutiveStore{}, llm, &MockJITCompiler{}, cf, nil)
		cfg := DefaultExecutorConfig()
		cfg.WorkspaceRoot = t.TempDir()
		e.SetConfig(cfg)
		// A subagent runs under the config its spawner compiled, which can
		// name tools the persona's static envelope does not.
		e.SetAgentConfig(validCapabilityTestConfig("read_file", "write_file"))
		res, err = e.ProcessWithIntent(ctx, "fix the retry loop", preset)
		if err != nil {
			t.Fatalf("ProcessWithIntent: %v", err)
		}
		return offered, k, res
	}
	preset := func() *perception.Intent {
		return &perception.Intent{Verb: "/review", Category: "/query", Confidence: 1}
	}

	// Control: an unrestricted delegated task is offered both.
	offered, _, _ := run(t, context.Background(), preset())
	if !slices.Contains(offered, "write_file") || !slices.Contains(offered, "read_file") {
		t.Fatalf("unrestricted task offered %v, want read_file and write_file", offered)
	}

	// The parent's restriction rides the context; the preset carries nothing.
	parent := perception.WithRestriction(context.Background(),
		perception.RestrictionFrom(&perception.Understanding{UserConstraints: []string{"no_changes"}}))
	offered, k, res := run(t, parent, preset())
	if slices.Contains(offered, "write_file") {
		t.Fatalf("a task under no_changes was offered write_file: %v", offered)
	}
	if !slices.Contains(offered, "read_file") {
		t.Fatalf("a task under no_changes lost reading: %v", offered)
	}
	if !res.Intent.Restriction.Has(perception.RestrictNoChanges) {
		t.Fatalf("the result's intent lost the inherited restriction: %v", res.Intent.Restriction.Kinds())
	}
	// A restriction's facts leave with the turn: nothing for a later turn to inherit.
	for _, p := range []string{"turn_restriction", "restriction_tool_effect"} {
		if rows, _ := k.Query(p); len(rows) != 0 {
			t.Fatalf("%s survived the turn: %v", p, rows)
		}
	}

	// A restriction the intent carries itself binds the same way, and a
	// parent context cannot be lifted by an intent that carries none.
	own := preset()
	own.Restriction = perception.RestrictionFrom(&perception.Understanding{UserConstraints: []string{"dry_run"}})
	offered, _, _ = run(t, context.Background(), own)
	if slices.Contains(offered, "write_file") {
		t.Fatalf("an intent carrying dry_run was offered write_file: %v", offered)
	}
}
