package session

import (
	"strings"
	"sync"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/projectdoc"
	"codenerd/internal/types"
)

func TestExecutorExtractTargetSharesProjectDocPathContract(t *testing.T) {
	executor := &Executor{}
	for _, key := range projectdoc.PathArgs {
		t.Run(key, func(t *testing.T) {
			const want = "protected/config.json"
			if got := executor.extractTarget(map[string]any{key: want}); got != want {
				t.Fatalf("extractTarget(%s) = %q, want %q", key, got, want)
			}
		})
	}
}

func TestValidMangleVerbRejectsQueryFragments(t *testing.T) {
	for _, valid := range []string{"/review", "/generate_tool", "/v2"} {
		if !validMangleVerb(valid) {
			t.Errorf("validMangleVerb(%q) = false", valid)
		}
	}
	for _, invalid := range []string{"", "review", "/Review", "/review)", "/review, /other", "/consult/name", "/with-dash"} {
		if validMangleVerb(invalid) {
			t.Errorf("validMangleVerb(%q) = true", invalid)
		}
	}
}

type exactPermissionKernel struct {
	*MockKernel
	bareQueries int
}

func (k *exactPermissionKernel) Query(query string) ([]types.Fact, error) {
	if query == "permitted" {
		k.bareQueries++
		return nil, nil
	}
	if strings.HasPrefix(query, "permitted(") {
		return []types.Fact{{
			Predicate: "permitted",
			Args:      []any{types.MangleAtom("/read_file"), "x.go", `{"path":"x.go"}`},
		}}, nil
	}
	return k.MockKernel.Query(query)
}

func TestCheckSafetyUsesGroundedPermissionQueryFastPath(t *testing.T) {
	kernel := &exactPermissionKernel{MockKernel: &MockKernel{}}
	executor := &Executor{kernel: kernel, config: DefaultExecutorConfig()}
	allowed := executor.checkSafety(ToolCall{
		ID: "exact-permission-1", Name: "read_file", Args: map[string]any{"path": "x.go"},
	})
	if !allowed {
		t.Fatal("grounded permitted fact was denied")
	}
	if kernel.bareQueries != 0 {
		t.Fatalf("bare permitted scans = %d, want 0 on an exact-match fast path", kernel.bareQueries)
	}
}

func TestCheckSafetyRejectsMalformedActionBeforeQuery(t *testing.T) {
	kernel := &MockKernel{}
	executor := &Executor{kernel: kernel, config: DefaultExecutorConfig()}
	if executor.checkSafety(ToolCall{ID: "malformed-1", Name: "read file", Args: map[string]any{"path": "x.go"}}) {
		t.Fatal("malformed action name was allowed")
	}
	for _, fact := range kernel.asserts {
		if fact.Predicate == "pending_action" {
			t.Fatalf("malformed action reached pending_action: %#v", fact)
		}
	}
}

// TestExecutor_CheckSafety_NilAgentConfigGracefulRejection verifies that when
// AgentConfig is nil, buildToolDefinitions returns nil (gracefully) and
// isToolAllowed fails closed. The pipeline should not panic.
//
// QA boundary item: "Add test for nil AgentConfig — graceful rejection"
func TestExecutor_CheckSafety_NilAgentConfigGracefulRejection(t *testing.T) {
	executor := &Executor{
		kernel: &MockKernel{},
		config: DefaultExecutorConfig(),
	}

	// buildToolDefinitions must handle nil cfg without panic
	defs := executor.buildToolDefinitions(nil)
	if defs != nil {
		t.Errorf("expected nil tool defs for nil cfg, got %d", len(defs))
	}

	// isToolAllowed must handle nil cfg without panic and deny the capability.
	if executor.isToolAllowed("any_tool", nil) {
		t.Error("expected isToolAllowed to fail closed for nil cfg")
	}

	// And with an empty (but non-nil) config
	emptyCfg := &config.EffectiveAgentRuntimeConfig{}
	defs = executor.buildToolDefinitions(emptyCfg)
	if defs != nil {
		t.Errorf("expected nil tool defs for empty cfg, got %d", len(defs))
	}
	if executor.isToolAllowed("any_tool", emptyCfg) {
		t.Error("expected isToolAllowed to fail closed for empty cfg")
	}
}

func TestExecutorConfigSnapshotConcurrentSet(t *testing.T) {
	executor := &Executor{config: DefaultExecutorConfig()}
	first := DefaultExecutorConfig()
	first.MaxToolCalls = 11
	first.TokenBudget = 111
	second := DefaultExecutorConfig()
	second.MaxToolCalls = 22
	second.TokenBudget = 222

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 2_000; i++ {
			if i%2 == 0 {
				executor.SetConfig(first)
			} else {
				executor.SetConfig(second)
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 2_000; i++ {
			got := executor.configSnapshot()
			if (got.MaxToolCalls == first.MaxToolCalls && got.TokenBudget != first.TokenBudget) ||
				(got.MaxToolCalls == second.MaxToolCalls && got.TokenBudget != second.TokenBudget) {
				t.Errorf("torn config snapshot: MaxToolCalls=%d TokenBudget=%d", got.MaxToolCalls, got.TokenBudget)
				return
			}
		}
	}()
	wg.Wait()
}

// TestExecutor_CheckSafety_MassivePayloadRejected verifies that payloads
// exceeding maxPayloadBytes are rejected before being asserted into the
// kernel's fact store, preventing GC spikes / OOMs.
//
// QA boundary items 4+5: massive payload truncation/guard before Kernel.Assert.
// TODO: TEST_GAP: [User Request Extremes] Add a negative test where the overall payload size is just under `maxPayloadBytes`, but the extracted `target` string alone is massive (e.g., 90KB), to verify Mangle engine resilience against massive atom names.
// TODO: TEST_GAP: [User Request Extremes] Implement a stress test that fires 10,000 rapid concurrent `checkSafety` calls to validate garbage collection pressure and Mangle EDB growth limits (e.g., the '50 million line monorepo' edge case).
func TestExecutor_CheckSafety_MassivePayloadRejected(t *testing.T) {
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	// Build a payload that, once JSON-encoded, exceeds maxPayloadBytes (100KB).
	// 200,000 chars is comfortably over 100 KB (the JSON-encoded string adds
	// quote characters and key overhead, but the raw value alone is enough).
	huge := strings.Repeat("A", 200_000)
	toolCall := ToolCall{
		ID:   "call_huge",
		Name: "writeFile",
		Args: map[string]any{
			"path":    "out.txt",
			"content": huge,
		},
	}

	allowed := executor.checkSafety(toolCall)
	if allowed {
		t.Error("Expected oversized payload to be denied by safety gate")
	}

	// CRITICAL: the assertion must NOT have reached the kernel. The guard
	// runs after json.Marshal but before kernel.Assert(pendingFact).
	for _, f := range mockKernel.asserts {
		if f.Predicate == "pending_action" {
			t.Error("oversized payload should not have asserted pending_action to kernel")
		}
	}
}

// TestExecutor_CheckSafety_PayloadAtBoundary verifies that payloads just under
// the threshold are accepted (asserted) and only payloads over the threshold
// are rejected. This guards against off-by-one errors in the size check.
// TODO: TEST_GAP: [Type Coercion] Verify that `extractTarget` rejects or gracefully handles malformed complex types (e.g., slices, maps) passed to candidate keys instead of coercing them into invalid string atoms.
// TODO: TEST_GAP: [Type Coercion] Add boundary tests verifying numeric target extraction, including float-to-string formatting differences (e.g., 1.0 vs 1) and excessively large integer values exceeding standard bounds.
func TestExecutor_CheckSafety_PayloadAtBoundary(t *testing.T) {
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	// Build a payload that JSON-encodes to just under the limit. Use small
	// enough that even with key + quote overhead we stay under 100KB.
	smallish := strings.Repeat("x", 50_000)
	toolCall := ToolCall{
		ID:   "call_ok",
		Name: "writeFile",
		Args: map[string]any{
			"path":    "out.txt",
			"content": smallish,
		},
	}

	// The action will be denied (no permitted fact) but it must reach the
	// kernel — i.e., a pending_action assertion must occur, proving the size
	// guard did NOT short-circuit at this size.
	_ = executor.checkSafety(toolCall)

	foundPending := false
	for _, f := range mockKernel.asserts {
		if f.Predicate == "pending_action" {
			foundPending = true
			break
		}
	}
	if !foundPending {
		t.Error("Under-threshold payload should still reach kernel as pending_action")
	}
}

// TestExecutor_CheckSafety_EmptyToolNameRejectsCategorically asserts the
// stricter contract: an empty Name returns false, AND no pending_action fact
// is asserted to the kernel. This validates the categorical rejection path.
func TestExecutor_CheckSafety_EmptyToolNameRejectsCategorically(t *testing.T) {
	mockKernel := &MockKernel{}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	for _, name := range []string{"", "   ", "\t\n"} {
		toolCall := ToolCall{
			ID:   "call_empty",
			Name: name,
			Args: map[string]any{"path": "x"},
		}

		allowed := executor.checkSafety(toolCall)
		if allowed {
			t.Errorf("Expected empty/whitespace name %q to be denied categorically", name)
		}
	}

	// No pending_action fact should have been asserted for empty names —
	// rejection must happen before reaching Kernel.Assert.
	for _, f := range mockKernel.asserts {
		if f.Predicate == "pending_action" {
			t.Error("empty tool name should not assert pending_action to kernel")
		}
	}
}

// TestExecutor_CheckSafety_NilArgsAssertsEmptyObject verifies stricter
// behavior than TestExecutor_NilArgsInToolCall: nil Args is normalized to
// "{}" payload so that permitted facts written by policy (which use "{}" for
// no-arg actions) match correctly.
// TODO: TEST_GAP: [Null/Undefined/Empty] Add boundary tests where the candidate key (e.g., 'path') is explicitly present but its value is `nil`, ensuring `extractTarget` falls back to 'unknown' rather than an empty string.
// TODO: TEST_GAP: [Null/Undefined/Empty] Add tests for ToolCalls containing deeply nested empty objects and uninitialized slices to verify JSON marshaling and Mangle evaluation robustness.
func TestExecutor_CheckSafety_NilArgsAssertsEmptyObject(t *testing.T) {
	mockKernel := &MockKernel{
		// Pre-load a permitted fact for the readFile action with empty payload.
		facts: []types.Fact{{
			Predicate: "permitted",
			Args:      []any{"/readFile", "unknown", "{}"},
		}},
	}
	executor := &Executor{
		kernel: mockKernel,
		config: DefaultExecutorConfig(),
	}

	toolCall := ToolCall{
		ID:   "call_nil_args",
		Name: "readFile",
		Args: nil,
	}

	allowed := executor.checkSafety(toolCall)
	if !allowed {
		t.Error("Expected nil Args to normalize to {} payload and match permitted fact")
	}
}
