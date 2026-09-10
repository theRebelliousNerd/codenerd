package system

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// After Close, nothing this process holds may still point inside the workspace.
//
// This is the general form of a defect that has now cost this repo twice, and
// it is a defect Linux hides. Closing is what releases a file, but on Linux an
// unlinked-but-open file simply disappears at exit, so a handle left behind by
// boot has no symptom at all. On Windows an open handle means the file cannot
// be deleted, so the same bug shows up as t.TempDir cleanup failing — which is
// how the meter logs were caught, sixteen internal/system tests failing at once
// on "the process cannot access the file because it is being used by another
// process", none of them about anything those tests were testing.
//
// Waiting for Windows CI to find these is a slow loop and an obscure symptom.
// Reading /proc/self/fd gives the same answer directly, on the platform this is
// developed on, naming the file rather than the cleanup that tripped over it.
//
// It is deliberately a check on the WHOLE workspace rather than on any
// particular file. Every future thing boot opens is covered without anyone
// remembering to add it here — which is exactly what went wrong: the meter logs
// were added to boot and never to Cortex.Close, whose own doc comment says it
// exists for this.
func TestCloseReleasesEveryWorkspaceHandle(t *testing.T) {
	if runtime.GOOS != "linux" {
		// /proc/self/fd is the mechanism. On Windows the same defect surfaces
		// as the TempDir cleanup failure this test exists to pre-empt, so
		// coverage there is real, just indirect.
		t.Skip("needs /proc/self/fd")
	}
	if testing.Short() {
		t.Skip("boots a full Cortex")
	}

	cortex := bootSessionWiringCortex(t, "handle-release")
	workspace := cortex.Workspace
	if workspace == "" {
		t.Fatal("booted Cortex has no workspace, so this test cannot scope its check")
	}

	// Prove the check can see something before trusting it to see nothing:
	// a file we hold open under the workspace must be reported.
	canary := filepath.Join(workspace, "canary.open")
	f, err := os.Create(canary)
	if err != nil {
		t.Fatalf("create canary: %v", err)
	}
	if open := openUnder(t, workspace); len(open) == 0 {
		_ = f.Close()
		t.Fatal("the open-handle scan found nothing while a file was deliberately held open; " +
			"it cannot detect a leak and would pass no matter what Close did")
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close canary: %v", err)
	}
	if err := os.Remove(canary); err != nil {
		t.Fatalf("remove canary: %v", err)
	}

	if err := cortex.Close(); err != nil {
		t.Fatalf("Cortex.Close: %v", err)
	}

	if leaked := openUnder(t, workspace); len(leaked) > 0 {
		t.Errorf("Cortex.Close left %d handle(s) open under the workspace:\n  %s\n\n"+
			"On Windows each of these makes its file undeletable, which is how this "+
			"shows up in CI: TempDir cleanup failing on a test that was passing.",
			len(leaked), strings.Join(leaked, "\n  "))
	}
}

// openUnder lists the paths this process currently has open beneath root.
func openUnder(t *testing.T, root string) []string {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatalf("read /proc/self/fd: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}

	var out []string
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", e.Name()))
		if err != nil {
			// The descriptor closed while we were walking; that is the outcome
			// this test wants anyway.
			continue
		}
		// A deleted-but-open file is exactly the leak on Windows, so the
		// " (deleted)" suffix Linux appends must not exempt it.
		clean := strings.TrimSuffix(target, " (deleted)")
		if strings.HasPrefix(clean, resolvedRoot+string(os.PathSeparator)) || strings.HasPrefix(clean, root+string(os.PathSeparator)) {
			out = append(out, clean)
		}
	}
	return out
}
