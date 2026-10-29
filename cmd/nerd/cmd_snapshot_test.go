package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/persist/snapshot"
)

// Snapshot commands report to the operator on stdout, so asserting on that
// text is asserting on the command's actual contract. captureStdout lives in
// cmd_mcp_select_test.go.

// useTempWorkspace points the shared --workspace variable at a temp dir and
// restores the snapshot command flags afterwards, since they are package-level
// state shared with every other test in this binary.
func useTempWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	prevWorkspace := workspace
	prevCodec, prevPreds, prevDerived := snapshotCodec, snapshotPreds, snapshotDerived
	prevOut, prevAssert, prevMangle, prevShow := snapshotOutFile, snapshotAssert, snapshotToMangle, snapshotShowFacts
	workspace = root
	snapshotCodec, snapshotPreds, snapshotDerived = "gzip", nil, false
	snapshotOutFile, snapshotAssert, snapshotToMangle, snapshotShowFacts = "", false, "", 0
	t.Cleanup(func() {
		workspace = prevWorkspace
		snapshotCodec, snapshotPreds, snapshotDerived = prevCodec, prevPreds, prevDerived
		snapshotOutFile, snapshotAssert, snapshotToMangle, snapshotShowFacts = prevOut, prevAssert, prevMangle, prevShow
	})
	return root
}

func TestSnapshotExport_WhenWorkspaceBooted_ShouldWriteReadableSnapshot(t *testing.T) {
	root := useTempWorkspace(t)

	out, err := captureStdout(t, func() error {
		return runSnapshotExport(snapshotExportCmd, []string{"cli-export"})
	})
	if err != nil {
		t.Fatalf("snapshot export: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "Wrote") || !strings.Contains(out, "cli-export") {
		t.Fatalf("export did not report the file it wrote: %s", out)
	}

	entries, err := snapshot.List(root)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "cli-export" {
		t.Fatalf("expected one snapshot named cli-export, got %+v", entries)
	}
	if !entries[0].Verifiable {
		t.Fatal("export should leave an integrity sidecar")
	}

	// The command's own import path must resolve what its export path wrote.
	facts, path, err := snapshot.Import(root, "cli-export")
	if err != nil {
		t.Fatalf("Import after CLI export: %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("exported snapshot at %s is empty", path)
	}
}

func TestSnapshotImport_WhenToMangleRequested_ShouldWriteSortedDatalog(t *testing.T) {
	root := useTempWorkspace(t)

	if _, err := captureStdout(t, func() error {
		return runSnapshotExport(snapshotExportCmd, []string{"for-mangle"})
	}); err != nil {
		t.Fatalf("export: %v", err)
	}

	dest := filepath.Join(root, "out", "imported.mg")
	snapshotToMangle = dest
	out, err := captureStdout(t, func() error {
		return runSnapshotImport(snapshotImportCmd, []string{"for-mangle"})
	})
	if err != nil {
		t.Fatalf("import: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "sha256 verified") {
		t.Fatalf("import should report sidecar verification: %s", out)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read rendered mangle: %v", err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "# Facts imported from ") {
		t.Fatalf("rendered mangle missing provenance header: %.80s", text)
	}
	var prev string
	facts := 0
	for _, line := range strings.Split(text, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasSuffix(line, ".") {
			t.Fatalf("line is not a Datalog fact: %q", line)
		}
		if prev != "" && line < prev {
			// Sorted output is what makes two exports of the same kernel
			// diffable instead of producing noise.
			t.Fatalf("facts are not sorted: %q came after %q", line, prev)
		}
		prev = line
		facts++
	}
	if facts == 0 {
		t.Fatal("rendered mangle contains no facts")
	}
}

func TestSnapshotImport_WhenAssertRequested_ShouldLoadIntoLocalKernelOnly(t *testing.T) {
	root := useTempWorkspace(t)

	if _, err := captureStdout(t, func() error {
		return runSnapshotExport(snapshotExportCmd, []string{"assert-me"})
	}); err != nil {
		t.Fatalf("export: %v", err)
	}

	snapshotAssert = true
	out, err := captureStdout(t, func() error {
		return runSnapshotImport(snapshotImportCmd, []string{"assert-me"})
	})
	if err != nil {
		t.Fatalf("import --assert: %v (output: %s)", err, out)
	}
	if !strings.Contains(out, "Asserted into a local kernel") {
		t.Fatalf("--assert did not report the kernel delta: %s", out)
	}

	// --assert is explicitly in-process: a snapshot must never quietly become
	// part of the workspace's boot state just because someone inspected it.
	if entries, _ := os.ReadDir(filepath.Join(root, ".nerd", "mangle")); len(entries) != 0 {
		t.Fatalf("--assert wrote into .nerd/mangle: %v", entries)
	}
}

func TestSnapshotImport_WhenReferenceMissing_ShouldErrorNotPanic(t *testing.T) {
	useTempWorkspace(t)
	_, err := captureStdout(t, func() error {
		return runSnapshotImport(snapshotImportCmd, []string{"does-not-exist"})
	})
	if err == nil {
		t.Fatal("expected an error for a missing snapshot")
	}
	if !strings.Contains(err.Error(), "snapshots") {
		t.Fatalf("error should point at the snapshot directory: %v", err)
	}
}

func TestSnapshotList_WhenNoSnapshots_ShouldSuggestExport(t *testing.T) {
	useTempWorkspace(t)
	out, err := captureStdout(t, func() error {
		return runSnapshotList(snapshotListCmd, nil)
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "No snapshots") || !strings.Contains(out, "nerd snapshot export") {
		t.Fatalf("empty list should tell the operator what to do next: %s", out)
	}
}

func TestSnapshotExport_WhenPredicateUnknown_ShouldFailLoudly(t *testing.T) {
	useTempWorkspace(t)
	snapshotPreds = []string{"no_such_predicate_anywhere"}
	_, err := captureStdout(t, func() error {
		return runSnapshotExport(snapshotExportCmd, []string{"empty-export"})
	})
	if err == nil {
		// Writing an empty snapshot would look like success and later import
		// as "the kernel knew nothing", which is a different claim entirely.
		t.Fatal("expected an error when the selected predicates match no facts")
	}
}

func TestHumanBytes_WhenScaling_ShouldPickUnit(t *testing.T) {
	cases := map[int64]string{
		0:         "0 B",
		512:       "512 B",
		2048:      "2.0 KiB",
		3 << 20:   "3.0 MiB",
		1<<20 - 1: "1024.0 KiB",
		1<<10 - 1: "1023 B",
		10 << 20:  "10.0 MiB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Fatalf("humanBytes(%d) = %s, want %s", in, got, want)
		}
	}
}
