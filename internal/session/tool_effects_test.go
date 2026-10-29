package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// Tool effect receipts (tool_effects.go, policy/tool_effects.mg): a validator
// can reject a result after its bytes landed, and a failed postcondition is
// not proof of no mutation. These fixtures are the audit's
// (Docs/audits/tool-effect-receipts-and-validation-completeness.md, section 8)
// through the production accounting path, executeToolBatch.

// scriptedGate is an executive gate whose post-action validation answers
// from a script, one state per call, and counts what it was asked.
type scriptedGate struct {
	testExecutiveStore
	answers []types.ToolValidation
	asked   []string
}

func (g *scriptedGate) ValidateInteractiveToolResult(_ context.Context, actionID, _ string, _ map[string]any, _ string, _ bool) types.ToolValidation {
	g.asked = append(g.asked, actionID)
	if len(g.answers) == 0 {
		return types.ToolValidation{State: types.ToolValidationVerified}
	}
	v := g.answers[0]
	if len(g.answers) > 1 {
		g.answers = g.answers[1:]
	}
	return v
}

// effectFixture registers fs_write (a write-mutation tool by name) whose
// handler writes args.content to args.path and then returns fail, and an
// executor on workspace dir with gate and kernel k. The kernel is required:
// without one nerd.md write protection fails closed before any handler runs.
func effectFixture(t *testing.T, k types.Kernel, gate *scriptedGate, fail error) (*Executor, *config.EffectiveAgentRuntimeConfig, string) {
	t.Helper()
	dir := t.TempDir()
	registerTestTool(t, &tools.Tool{
		Name:     "fs_write",
		Effect:   tools.EffectWrite,
		Category: tools.CategoryGeneral,
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			path, _ := args["path"].(string)
			content, _ := args["content"].(string)
			if content != "" {
				if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
					return "", err
				}
			}
			return "wrote " + path, fail
		},
	})
	e := NewExecutor(k, gate, nil, nil, nil, nil)
	e.config = ExecutorConfig{WorkspaceRoot: dir}
	t.Cleanup(e.cleanupTurnFacts)
	return e, &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"fs_write"}}, dir
}

func writeCall(id, path, content string) types.ToolCall {
	return types.ToolCall{ID: id, Name: "fs_write", Input: map[string]any{"path": path, "content": content}}
}

func turnHolds(t *testing.T, k types.Kernel, predicate string, turn types.MangleAtom) bool {
	t.Helper()
	rows, err := k.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	for _, f := range rows {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == string(turn) {
			return true
		}
	}
	return false
}

func missingEvidence(t *testing.T, k types.Kernel, turn types.MangleAtom, atom string) bool {
	t.Helper()
	rows, err := k.Query("turn_missing_evidence")
	if err != nil {
		t.Fatalf("query turn_missing_evidence: %v", err)
	}
	for _, f := range rows {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == string(turn) && types.ExtractString(f.Args[1]) == atom {
			return true
		}
	}
	return false
}

// TOOL-EFFECT-01: an authorized write whose validator rejects the bytes it
// left. The bytes stay attributed to the turn, the handler's output reaches
// the model with the rejection, and no successful validation is claimed.
func TestToolEffects_RejectedWriteKeepsItsChange(t *testing.T) {
	gate := &scriptedGate{answers: []types.ToolValidation{{State: types.ToolValidationRejected, Reason: "invalid JSON: unexpected end"}}}
	e, cfg, dir := effectFixture(t, realKernel(t), gate, nil)
	result := &ExecutionResult{TestRunSinceLastWrite: &tools.TestRun{}}

	results, errs := e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "a.json", `{"a":`)}, cfg, result)

	if got, _ := os.ReadFile(filepath.Join(dir, "a.json")); string(got) != `{"a":` {
		t.Fatalf("bytes on disk = %q, want the rejected write's (results %+v errs %v)", got, results, errs)
	}
	if len(errs) != 1 || !results[0].IsError {
		t.Fatalf("errs=%v results=%+v, want the rejection surfaced", errs, results)
	}
	if !strings.Contains(results[0].Content, "post-action validation failed") || !strings.Contains(results[0].Content, "wrote a.json") {
		t.Fatalf("tool result %q, want the rejection and the handler's output", results[0].Content)
	}
	if result.WriteToolEffects != 1 || result.SuccessfulToolCalls != 0 {
		t.Fatalf("WriteToolEffects=%d SuccessfulToolCalls=%d, want 1 and 0", result.WriteToolEffects, result.SuccessfulToolCalls)
	}
	if len(result.WrittenPaths) != 1 || result.WrittenPaths[0] != "a.json" {
		t.Fatalf("WrittenPaths = %v, want the rejected write's path", result.WrittenPaths)
	}
	if result.TestRunSinceLastWrite != nil {
		t.Fatal("a test run before the rejected write still witnesses the bytes it left")
	}
	if len(result.ToolReceipts) != 1 {
		t.Fatalf("receipts = %+v, want one", result.ToolReceipts)
	}
	r := result.ToolReceipts[0]
	if r.Validation.State != types.ToolValidationRejected || !r.Changed() || r.HandlerError != "" {
		t.Fatalf("receipt = %+v, want a rejected change with no handler error", r)
	}
	if r.Effects[0].Before != identityAbsent || !strings.HasPrefix(r.Effects[0].After, "sha256:") {
		t.Fatalf("effect = %+v, want absent -> sha256", r.Effects[0])
	}
}

// With the policy present the rejection derives /repair, the model is told
// the change is on disk, and the turn cannot verify until a later write to
// the same path is measured.
func TestToolEffects_PolicyDerivesRepairAndWithholdsTheTurn(t *testing.T) {
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{
		{State: types.ToolValidationVerified},
		{State: types.ToolValidationRejected, Reason: "invalid JSON"},
		{State: types.ToolValidationVerified},
	}}
	e, cfg, _ := effectFixture(t, k, gate, nil)
	result := &ExecutionResult{}
	turn := result.turnAtom()

	// A verified write first: the turn did act, so its verdict is about
	// what it left, not a hollow turn.
	results, _ := e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c0", "ok.json", `{}`), writeCall("c1", "a.json", `{"a":`)}, cfg, result)
	results = results[1:]
	if got := result.ToolReceipts[1].Recovery; got != "/repair" {
		t.Fatalf("recovery = %q, want /repair", got)
	}
	if !strings.Contains(results[0].Content, "Host receipt") || !strings.Contains(results[0].Content, "Repeating the same call will not undo it") {
		t.Fatalf("tool result %q, want the receipt's repair note", results[0].Content)
	}
	if !turnHolds(t, k, "turn_has_rejected_effect", turn) {
		t.Fatal("turn_has_rejected_effect not derived for the rejected current change")
	}
	// The executive contract: the turn's verdict names the rejected change.
	e.assertTurnEvidence(turn, "/fix", result)
	if !missingEvidence(t, k, turn, "/effect_rejected") {
		t.Fatal("turn_missing_evidence(/effect_rejected) not derived for the turn")
	}

	results, _ = e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c2", "a.json", `{"a":1}`)}, cfg, result)
	if results[0].IsError || strings.Contains(results[0].Content, "Host receipt") {
		t.Fatalf("repairing write result = %+v, want a clean success", results[0])
	}
	if turnHolds(t, k, "turn_has_rejected_effect", turn) {
		t.Fatal("the repair's verified change did not supersede the rejected one")
	}
	if missingEvidence(t, k, turn, "/effect_rejected") {
		t.Fatal("the verdict still names a rejected change the repair superseded")
	}
	if result.WriteToolEffects != 3 || result.SuccessfulToolCalls != 2 || len(result.ToolReceipts) != 3 {
		t.Fatalf("WriteToolEffects=%d SuccessfulToolCalls=%d receipts=%d, want 3, 2, 3", result.WriteToolEffects, result.SuccessfulToolCalls, len(result.ToolReceipts))
	}
}

// A handler that fails after changing the file had no validator measure the
// change. The policy derives /revalidate, and the host measures it once.
func TestToolEffects_HandlerFailureAfterChangeIsRevalidated(t *testing.T) {
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{{State: types.ToolValidationVerified}}}
	e, cfg, _ := effectFixture(t, k, gate, errors.New("index update failed"))
	result := &ExecutionResult{}
	turn := result.turnAtom()

	results, _ := e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "b.txt", "hello")}, cfg, result)
	if !results[0].IsError || !strings.Contains(results[0].Content, "index update failed") {
		t.Fatalf("result = %+v, want the handler's failure", results[0])
	}
	r := result.ToolReceipts[0]
	if !r.Revalidated || r.Validation.State != types.ToolValidationVerified || r.HandlerError == "" {
		t.Fatalf("receipt = %+v, want a handler failure the host revalidated as verified", r)
	}
	if len(gate.asked) != 1 || !strings.HasSuffix(gate.asked[0], ":revalidate") {
		t.Fatalf("gate asked %v, want one revalidation", gate.asked)
	}
	if r.Recovery != "" || turnHolds(t, k, "turn_has_unmeasured_effect", turn) {
		t.Fatalf("recovery=%q after a verified revalidation, want none and no unmeasured effect", r.Recovery)
	}
	if result.WriteToolEffects != 1 || len(result.WrittenPaths) != 1 {
		t.Fatalf("WriteToolEffects=%d WrittenPaths=%v, want the failed call's change in the write set", result.WriteToolEffects, result.WrittenPaths)
	}
}

// TOOL-EFFECT-02: a validator that could not complete is incomplete, not a
// pass: the turn cannot claim the change was measured, and the model is told
// to inspect it.
func TestToolEffects_IncompleteValidationIsNotAffirmative(t *testing.T) {
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{{State: types.ToolValidationIncomplete, Reason: "validation cancelled: context canceled"}}}
	e, cfg, _ := effectFixture(t, k, gate, nil)
	result := &ExecutionResult{}
	turn := result.turnAtom()

	results, _ := e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "c.txt", "x")}, cfg, result)
	if results[0].IsError {
		t.Fatalf("an incomplete validation is not a rejection: %+v", results[0])
	}
	if result.ToolReceipts[0].Recovery != "/inspect" || !strings.Contains(results[0].Content, "no completed validation measured the change") {
		t.Fatalf("receipt %+v / result %q, want /inspect said to the model", result.ToolReceipts[0], results[0].Content)
	}
	if !turnHolds(t, k, "turn_has_unmeasured_effect", turn) {
		t.Fatal("turn_has_unmeasured_effect not derived for an incomplete validation")
	}
}

// A refused write never runs and invents no mutation; a failed write that
// changed nothing owes nothing.
func TestToolEffects_NoChangeNoReceipt(t *testing.T) {
	t.Run("denied by the write guard", func(t *testing.T) {
		gate := &scriptedGate{}
		e, cfg, dir := effectFixture(t, realKernel(t), gate, nil)
		result := &ExecutionResult{}
		ctx := WithWriteGuard(context.Background(), func(context.Context, []string) error { return errors.New("outside the declared write set") })
		results, _ := e.executeToolBatch(ctx, []types.ToolCall{writeCall("c1", "d.txt", "x")}, cfg, result)
		if !results[0].IsError {
			t.Fatalf("result = %+v, want the denial", results[0])
		}
		if _, err := os.Stat(filepath.Join(dir, "d.txt")); !os.IsNotExist(err) {
			t.Fatalf("denied write reached disk: %v", err)
		}
		if result.WriteToolEffects != 0 || len(result.ToolReceipts) != 0 || len(result.WrittenPaths) != 0 || len(gate.asked) != 0 {
			t.Fatalf("denied write accounted: effects=%d receipts=%d written=%v asked=%v", result.WriteToolEffects, len(result.ToolReceipts), result.WrittenPaths, gate.asked)
		}
	})
	t.Run("handler failed before writing", func(t *testing.T) {
		gate := &scriptedGate{}
		e, cfg, _ := effectFixture(t, realKernel(t), gate, errors.New("disk full"))
		result := &ExecutionResult{}
		results, _ := e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "e.txt", "")}, cfg, result)
		if !results[0].IsError || strings.Contains(results[0].Content, "Host receipt") {
			t.Fatalf("result = %+v, want the plain failure", results[0])
		}
		if result.WriteToolEffects != 0 || len(result.ToolReceipts) != 0 || len(result.WrittenPaths) != 0 {
			t.Fatalf("a failed write that changed nothing was accounted: %+v", result)
		}
	})
}

// A receipted change that a writer the receipts did not observe replaced no
// longer describes the file: at the closure it is superseded, and its
// rejection stops withholding the turn.
func TestToolEffects_SupersededChangeStopsWithholding(t *testing.T) {
	k := realKernel(t)
	gate := &scriptedGate{answers: []types.ToolValidation{{State: types.ToolValidationRejected, Reason: "invalid JSON"}}}
	e, cfg, dir := effectFixture(t, k, gate, nil)
	result := &ExecutionResult{}
	turn := result.turnAtom()

	e.executeToolBatch(context.Background(), []types.ToolCall{writeCall("c1", "f.json", `{"a":`)}, cfg, result)
	if !turnHolds(t, k, "turn_has_rejected_effect", turn) {
		t.Fatal("turn_has_rejected_effect not derived")
	}
	if err := os.WriteFile(filepath.Join(dir, "f.json"), []byte(`{"a":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	e.assertSupersededEffects(turn, result)
	if turnHolds(t, k, "turn_has_rejected_effect", turn) {
		t.Fatal("a superseded change still withholds the turn")
	}
}

func TestToolValidationMerge_TheWorseStateWins(t *testing.T) {
	var v types.ToolValidation
	v = v.Merge(types.ToolValidation{State: types.ToolValidationVerified})
	v = v.Merge(types.ToolValidation{State: types.ToolValidationRejected, Reason: "b.txt missing"})
	v = v.Merge(types.ToolValidation{State: types.ToolValidationIncomplete})
	if v.State != types.ToolValidationRejected || v.Reason != "b.txt missing" {
		t.Fatalf("merged = %+v, want the rejection to win", v)
	}
}
