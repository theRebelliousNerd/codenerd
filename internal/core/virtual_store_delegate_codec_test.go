package core

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/observation"
)

// TestHandleDelegateMintsSubagentHandle proves the delegate action is a live
// mint site for the subagent-return codec: a transcript above the retention
// threshold comes back as a projection carrying a redeemable handle, and the
// handle reopens the exact transcript through the shared codec — the same
// store the subagent_expand verb reads.
func TestHandleDelegateMintsSubagentHandle(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript := "finding alpha in main.go:10\n" + strings.Repeat("audit line with no structure\n", 60)
	vs.taskDelegator = &mockActionsTaskDelegator{
		executeFunc: func(context.Context, string, string) (string, error) { return transcript, nil },
	}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		ActionID: "del_codec",
		Target:   "reviewer",
		Payload:  map[string]any{"task": "review the diff"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	if !res.Success {
		t.Fatalf("expected success, got %+v", res)
	}
	handle, _ := res.Metadata["subagent_handle"].(string)
	if handle == "" {
		t.Fatal("large delegation must mint a subagent handle in Metadata")
	}
	if !strings.Contains(res.Output, handle) {
		t.Fatal("projection must name its own handle so the parent can redeem it")
	}
	if !strings.Contains(res.Output, "subagent_expand") {
		t.Fatal("projection must name the redemption verb")
	}
	if strings.Contains(res.Output, transcript) {
		t.Fatal("projection must elide the retained transcript, not paste it whole")
	}

	hydrated, err := observation.SharedSubagents().HydrateReturn(handle, observation.ReturnWindow{})
	if err != nil {
		t.Fatalf("minted handle must redeem: %v", err)
	}
	joined := strings.Join(hydrated.Lines, "\n")
	if !strings.Contains(joined, "finding alpha in main.go:10") {
		t.Fatalf("redeemed transcript lost its first line: %q", joined[:100])
	}
}

// TestHandleDelegateShortResultStaysVerbatim pins the historical contract the
// codec wiring must not break: small returns come back byte-identical with no
// handle, because there is nothing to elide.
func TestHandleDelegateShortResultStaysVerbatim(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	vs.taskDelegator = &mockActionsTaskDelegator{
		executeFunc: func(context.Context, string, string) (string, error) { return "all good", nil },
	}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		ActionID: "del_short",
		Target:   "coder",
		Payload:  map[string]any{"task": "check"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	if !res.Success || res.Output != "all good" {
		t.Fatalf("short result must pass through byte-identical, got %+v", res)
	}
	if len(res.Metadata) != 0 {
		t.Fatalf("verbatim result must carry no handle metadata, got %v", res.Metadata)
	}
}

// TestHandleDelegateKeepsRawResultInFact proves the fact channel is untouched
// by the projection: delegation_result still carries the raw transcript for
// kernel consumers even when the parent's Output is the elided projection.
func TestHandleDelegateKeepsRawResultInFact(t *testing.T) {
	vs, _ := createActionsTestVS(t)
	transcript := strings.Repeat("0123456789abcdef\n", 60)
	vs.taskDelegator = &mockActionsTaskDelegator{
		executeFunc: func(context.Context, string, string) (string, error) { return transcript, nil },
	}

	res, err := vs.handleDelegate(context.Background(), ActionRequest{
		ActionID: "del_fact",
		Target:   "tester",
		Payload:  map[string]any{"task": "run tests"},
	})
	if err != nil {
		t.Fatalf("handleDelegate: %v", err)
	}
	found := false
	for _, f := range res.FactsToAdd {
		if f.Predicate == "delegation_result" && len(f.Args) == 2 && f.Args[1] == transcript {
			found = true
		}
	}
	if !found {
		t.Fatalf("delegation_result fact must carry the raw transcript, got %+v", res.FactsToAdd)
	}
}
