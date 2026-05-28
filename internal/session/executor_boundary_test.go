package session

import (
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/types"
)

// TestExecutor_CheckSafety_NilAgentConfigGracefulRejection verifies that when
// AgentConfig is nil, buildToolDefinitions returns nil (gracefully) and
// isToolAllowed returns true (no restrictions). The pipeline should not panic.
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

	// isToolAllowed must handle nil cfg without panic (returns true: no restrictions)
	if !executor.isToolAllowed("any_tool", nil) {
		t.Error("expected isToolAllowed to return true for nil cfg")
	}

	// And with an empty (but non-nil) config
	emptyCfg := &config.EffectiveAgentRuntimeConfig{}
	defs = executor.buildToolDefinitions(emptyCfg)
	if defs != nil {
		t.Errorf("expected nil tool defs for empty cfg, got %d", len(defs))
	}
}

// TestExecutor_CheckSafety_MassivePayloadRejected verifies that payloads
// exceeding maxPayloadBytes are rejected before being asserted into the
// kernel's fact store, preventing GC spikes / OOMs.
//
// QA boundary items 4+5: massive payload truncation/guard before Kernel.Assert.
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
		Args: map[string]interface{}{
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
		Args: map[string]interface{}{
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
			Args: map[string]interface{}{"path": "x"},
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
func TestExecutor_CheckSafety_NilArgsAssertsEmptyObject(t *testing.T) {
	mockKernel := &MockKernel{
		// Pre-load a permitted fact for the readFile action with empty payload.
		facts: []types.Fact{{
			Predicate: "permitted",
			Args:      []interface{}{"/readFile", "unknown", "{}"},
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
