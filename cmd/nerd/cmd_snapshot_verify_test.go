package main

import (
	"strings"
	"testing"

	"codenerd/internal/persist/factsnap"
	"codenerd/internal/persist/snapshot"
	"codenerd/internal/types"
)

// `nerd snapshot verify` checks a snapshot's sidecar without importing it,
// and fails -- rather than printing success -- on a snapshot with nothing to
// verify against.
func TestSnapshotVerify_ShouldReportAMatchAndRefuseAnUnverifiableSnapshot(t *testing.T) {
	root := useTempWorkspace(t)
	facts := []types.Fact{{Predicate: "p", Args: []any{"a"}}}
	if _, err := snapshot.Export(root, "checked", facts, factsnap.CodecGzip); err != nil {
		t.Fatalf("Export: %v", err)
	}
	out, err := captureStdout(t, func() error {
		return runSnapshotVerify(snapshotVerifyCmd, []string{"checked"})
	})
	if err != nil || !strings.Contains(out, "matches its sidecar") {
		t.Fatalf("verify of a good snapshot: out=%q err=%v", out, err)
	}

	if _, err := factsnap.WritePath(snapshot.Dir(root)+"/scratch", facts, factsnap.Options{NoSidecar: true}); err != nil {
		t.Fatalf("WritePath: %v", err)
	}
	if _, err := captureStdout(t, func() error {
		return runSnapshotVerify(snapshotVerifyCmd, []string{"scratch"})
	}); err == nil || !strings.Contains(err.Error(), "sidecar") {
		t.Fatalf("verify of a snapshot with no sidecar should fail naming the sidecar: %v", err)
	}
}
