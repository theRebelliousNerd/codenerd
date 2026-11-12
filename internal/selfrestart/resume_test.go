package selfrestart

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// The work a restart cut crosses it whole, and a restore file an older build
// wrote (no resume list) still reads: the old process writes, the new reads.
func TestWriteState_ReadState_CarriesResumedWork(t *testing.T) {
	t.Parallel()
	in := State{SessionID: "s1", Resume: []ResumeWork{
		{Kind: ResumeBackground, ID: "self-repair-1", Shard: "reviewer", Task: "diagnose", Target: "t", Constraint: "c", Request: "r", FollowUp: "fix it", SelfRepair: "repair"},
		{Kind: ResumeTurn, Input: "/help", Command: true},
	}}
	path, err := WriteState(t.TempDir(), in)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadState(path)
	if err != nil {
		t.Fatal(err)
	}
	in.Version = stateVersion
	if diff := cmp.Diff(in, got); diff != "" {
		t.Fatalf("resumed work did not round-trip (-want +got):\n%s", diff)
	}

	old := filepath.Join(t.TempDir(), "restore-1.json")
	if err := os.WriteFile(old, []byte(`{"version":1,"session_id":"s0","interrupted":["background job x"],"old_build":"a","new_build":"b"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := ReadState(old)
	if err != nil || st.SessionID != "s0" || len(st.Resume) != 0 {
		t.Fatalf("an older build's restore file: %+v, %v", st, err)
	}
}
