package core

import (
	"context"
	"strings"
	"testing"
)

func TestDreamerNeverReusesAnAuthorization(t *testing.T) {
	d, k := setupTestDreamer(t)
	req := ActionRequest{Type: ActionWriteFile, Target: "scratch.txt", Payload: map[string]any{"content": "first"}}
	if got := d.SimulateAction(context.Background(), req); got.Unsafe {
		t.Fatal(got.Reason)
	}
	// Negative control: the exact same action is now forbidden by new policy.
	k.AppendPolicy(`panic_state(ID, "policy changed") :- projected_action(ID, /write_file, "scratch.txt").`)
	if err := k.Evaluate(); err != nil {
		t.Fatal(err)
	}
	req.Payload["content"] = "second"
	if got := d.SimulateAction(context.Background(), req); !got.Unsafe {
		t.Fatal("reused verdict across policy/payload change")
	}
	d.SetKernel(nil)
	if got := d.SimulateAction(context.Background(), req); !got.Unsafe || !strings.Contains(got.Reason, "unavailable") {
		t.Fatalf("missing kernel: %+v", got)
	}
}

func TestDreamerCancellationCannotHitWarmVerdict(t *testing.T) {
	d, _ := setupTestDreamer(t)
	req := ActionRequest{Type: ActionReadFile, Target: "scratch.txt"}
	if got := d.SimulateAction(context.Background(), req); got.Unsafe {
		t.Fatal(got.Reason)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := d.SimulateAction(ctx, req); !got.Unsafe || !strings.Contains(got.Reason, "canceled") {
		t.Fatalf("canceled: %+v", got)
	}
	var absent *Dreamer
	if got := absent.SimulateAction(context.Background(), req); !got.Unsafe {
		t.Fatal("nil dreamer allowed")
	}
}

func TestInteractiveEffectsFailClosed(t *testing.T) {
	var vs *VirtualStore
	for _, name := range []string{"write_file", "run_tests", "git_operation", "browser_act", "unknown_tool"} {
		if err := vs.PreflightDestructiveToolCall(context.Background(), "probe", name, nil); err == nil {
			t.Errorf("%s allowed without gate", name)
		}
	}
	if err := vs.PreflightDestructiveToolCall(context.Background(), "read", "read_file", nil); err != nil {
		t.Fatal(err)
	}
}
