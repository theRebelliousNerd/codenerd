package session

import (
	"testing"

	"codenerd/internal/articulation"
	"codenerd/internal/types"
)

// The campaign checkpoint reads the reviewer's structured verdict from the
// kernel (internal/campaign/checkpoint.go lookupKernelVerdict). The reviewer
// runs on this executor, so the verdict only reaches the kernel if this
// allowlist lets it through. Audit campaign 5a2f4c8d (2026-09-04) failed every
// checkpoint closed because it did not.
func TestProcessMangleUpdates_CheckpointVerdictReachesKernel(t *testing.T) {
	kernel := &MockKernel{}
	e := &Executor{kernel: kernel, config: DefaultExecutorConfig()}

	env := &articulation.PiggybackEnvelope{
		Surface: "PASS: objectives met",
		Control: articulation.ControlPacket{
			MangleUpdates: []string{
				`checkpoint_verdict("Retrieval Scaffold Inventory", /pass, "inventory complete", 92).`,
				`permitted(/delete_file, "x", /allow).`, // must stay blocked
			},
		},
	}
	e.processMangleUpdatesFromEnvelope(env)

	verdicts, err := kernel.Query("checkpoint_verdict")
	if err != nil {
		t.Fatal(err)
	}
	if len(verdicts) != 1 {
		t.Fatalf("expected one checkpoint_verdict fact in the kernel, got %d", len(verdicts))
	}
	if got := types.ExtractString(verdicts[0].Args[0]); got != "Retrieval Scaffold Inventory" {
		t.Fatalf("phase arg = %q", got)
	}
	if len(verdicts[0].Args) != 4 {
		t.Fatalf("expected arity 4, got %d", len(verdicts[0].Args))
	}

	blocked, _ := kernel.Query("permitted")
	if len(blocked) != 0 {
		t.Fatalf("permitted must never be assertable from mangle_updates, got %d facts", len(blocked))
	}
}
