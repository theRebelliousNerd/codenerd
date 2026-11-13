package session

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// payloadMarker is a line of failing output that names its attempt: long
// enough that the repair summary's excerpt of it (repairRetainedOutputCap)
// cannot hold it whole, so only the attempt's own prompt carries it verbatim.
func payloadMarker(attempt string) string {
	return strings.Repeat(attempt+"-failing-output ", 400)
}

// payloadTest is a test file whose one test, named for the attempt, fails
// printing payloadMarker(attempt): each attempt fails differently, so the
// repair policy never takes two of them for the same failure.
func payloadTest(attempt string) string {
	return fmt.Sprintf("package main\n\nimport (\n\t\"strings\"\n\t\"testing\"\n)\n\nfunc TestProbe_%s(t *testing.T) {\n\tt.Fatal(strings.Repeat(%q, 400))\n}\n", attempt, attempt+"-failing-output ")
}

// recallHandle finds a retired payload's pointer and its handle.
var recallHandle = regexp.MustCompile(regexp.QuoteMeta(retiredPayloadPrefix) + `[^\n]*? recall_context id="([^"]+)"`)

func requestBytes(history []types.Message) int {
	n := 0
	for _, m := range history {
		text, results, reasoning, calls, _ := messageBytes(m)
		n += text + results + reasoning + calls
	}
	return n
}

// A repair episode's attempts each send a payload -- the failing output, the
// tests and the diff -- and a later attempt's supersedes the one before. The
// superseded payload leaves the next request behind its recall handle instead
// of riding along: attempt N's request does not carry attempt N-1's payload
// verbatim, and recall_context brings it back whole. Live 2026-11-12 one fix
// turn sent ~96k input tokens a call over three repair attempts, every
// attempt's prompt resent on every later call.
func TestRepairLoop_ASupersededPayloadIsRetiredAndRecallable(t *testing.T) {
	t.Parallel()
	allowed := []string{"write_file", "read_file", "edit_file", "recall_context"}
	h := newRepairHarness(t, func(e *Executor) {
		e.config.RepairMaxAttempts = 3
		e.EffectiveAgentRuntimeConfig = &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: allowed}
		e.configFactory = &MockConfigFactory{GenerateFunc: func(context.Context, *prompt.CompilationResult, ...string) (*jitconfig.EffectiveAgentRuntimeConfig, error) {
			return &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: allowed}, nil
		}}
	})
	probe := filepath.Join(h.ws, "main_test.go")
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", probe, payloadTest("seed")),
		}}
	}
	type sent struct {
		bytes int
		text  string
	}
	var requests []sent // the first request of attempts 1, 2 and 3
	recalled := ""
	write := func(content string) types.ToolCall {
		return h.writeCall(fmt.Sprintf("r%d", len(requests)), "write_file", probe, content)
	}
	h.onRepair = func(_ int, history []types.Message) *types.LLMToolResponse {
		text := requestText(history)
		// The current prompt is the one carrying the newest failure whole. A
		// call after the episode converged concludes.
		switch {
		case len(requests) >= 3:
			return &types.LLMToolResponse{Text: "done"}
		case strings.Contains(text, payloadMarker("attempt2")):
			requests = append(requests, sent{bytes: requestBytes(history), text: text})
			return &types.LLMToolResponse{Text: "fixed", ToolCalls: []types.ToolCall{write(repairTestFixed)}}
		case strings.Contains(text, payloadMarker("attempt1")):
			// Attempt 2 recalls what attempt 1 was sent, and edits.
			requests = append(requests, sent{bytes: requestBytes(history), text: text})
			calls := []types.ToolCall{write(payloadTest("attempt2"))}
			if m := recallHandle.FindAllStringSubmatch(text, -1); len(m) > 0 {
				recalled = m[len(m)-1][1]
				calls = append([]types.ToolCall{{ID: "k1", Name: "recall_context", Input: map[string]any{"id": recalled}}}, calls...)
			}
			return &types.LLMToolResponse{Text: "trying", ToolCalls: calls}
		case strings.Contains(text, payloadMarker("seed")):
			requests = append(requests, sent{bytes: requestBytes(history), text: text})
			return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{write(payloadTest("attempt1"))}}
		}
		return &types.LLMToolResponse{Text: "done"}
	}
	result, err := h.drive(t, "fix the probe test")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if rec := result.TestCheck.Repair; rec == nil || !rec.Passed || rec.Cost.Attempts != 3 {
		t.Fatalf("repair record = %+v, want convergence on the third attempt", rec)
	}
	if len(requests) != 3 {
		t.Fatalf("%d repair requests, want one per attempt", len(requests))
	}
	for i, r := range requests {
		t.Logf("repair request, attempt %d: %d bytes", i+1, r.bytes)
	}
	seed, first := payloadMarker("seed"), payloadMarker("attempt1")
	if !strings.Contains(requests[0].text, seed) {
		t.Fatal("attempt 1 was not sent the seed's failing output whole")
	}
	if !strings.Contains(requests[1].text, first) {
		t.Fatal("attempt 2 was not sent attempt 1's failing output whole")
	}
	if strings.Contains(requests[1].text, seed) {
		t.Fatal("attempt 2's request still carries attempt 1's payload (the seed's failing output) verbatim")
	}
	if recalled == "" {
		t.Fatal("attempt 2's request names no recall handle for the payload it retired")
	}
	if !strings.Contains(requests[2].text, payloadMarker("attempt2")) {
		t.Fatal("attempt 3 was not sent attempt 2's failing output whole")
	}
	if strings.Contains(requests[2].text, first) {
		t.Fatal("attempt 3's request still carries attempt 2's payload (attempt 1's failing output) verbatim")
	}
	// What attempt 2 recalled is carried whole in attempt 3's request, as the
	// tool result it asked for.
	if !strings.Contains(requests[2].text, seed) {
		t.Fatalf("recall_context %q did not bring attempt 1's payload back whole", recalled)
	}
}

// What a repair round adds to each later request is its own round and its
// payload, not every payload before it. Measured on this script before
// payloads were retired: 10,630, 25,012 and 41,734 bytes for attempts 1-3,
// each request carrying every earlier prompt whole; after, 10,630, 15,500 and
// 18,887.
func TestRepairLoop_RequestsDoNotGrowByEverySupersededPayload(t *testing.T) {
	t.Parallel()
	h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 3 })
	probe := filepath.Join(h.ws, "main_test.go")
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", probe, payloadTest("seed")),
		}}
	}
	var sizes []int
	h.onRepair = func(_ int, history []types.Message) *types.LLMToolResponse {
		text := requestText(history)
		next := ""
		switch {
		case len(sizes) >= 3:
			return &types.LLMToolResponse{Text: "done"}
		case strings.Contains(text, payloadMarker("attempt2")):
			next = repairTestFixed
		case strings.Contains(text, payloadMarker("attempt1")):
			next = payloadTest("attempt2")
		case strings.Contains(text, payloadMarker("seed")):
			next = payloadTest("attempt1")
		default:
			return &types.LLMToolResponse{Text: "done"}
		}
		sizes = append(sizes, requestBytes(history))
		return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{h.writeCall(fmt.Sprintf("r%d", len(sizes)), "write_file", probe, next)}}
	}
	if _, err := h.drive(t, "fix the probe test"); err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if len(sizes) != 3 {
		t.Fatalf("%d repair requests, want one per attempt", len(sizes))
	}
	t.Logf("repair requests, attempts 1-3: %v bytes", sizes)
	// A superseded payload carries at least its whole failing output; growing
	// by less than that from one attempt to the next is growing without it.
	whole := len(payloadMarker("attempt1"))
	for i := 1; i < len(sizes); i++ {
		if grew := sizes[i] - sizes[i-1]; grew >= whole {
			t.Fatalf("attempt %d's request grew %d bytes over attempt %d's (sizes %v): the superseded payload rode along", i+1, grew, i, sizes)
		}
	}
}
