package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Dogfood run 5 (2026-09-29): a repair loop hid another agent's compile
// failure by adding //go:build ignore to files outside its write set. A
// write that hides its own file from the build is refused before it lands;
// these tests pin that refusal on the VirtualStore file path, with real
// files in a temp workspace. GOFLAGS and CGO_ENABLED are pinned so the
// verdicts hold whatever the ambient toolchain environment carries.
func buildExclusionTestVS(t *testing.T) (*VirtualStore, string) {
	t.Helper()
	t.Setenv("GOFLAGS", "")
	t.Setenv("CGO_ENABLED", "1")
	return createActionsTestVS(t)
}

func TestRejectBuildExclusion_RefusesIgnoreInANewFile(t *testing.T) {
	vs, _ := buildExclusionTestVS(t)
	err := vs.rejectBuildExclusion("hidden.go", nil, []byte("//go:build ignore\n\npackage hidden\n"))
	if err == nil {
		t.Fatal("a write adding //go:build ignore was accepted")
	}
	if !strings.Contains(err.Error(), "hidden.go") || !strings.Contains(err.Error(), "excludes") {
		t.Fatalf("error %q names neither the path nor the exclusion", err.Error())
	}
}

// extractCodeBlockForFile slices a Go payload from its package clause, which
// drops a leading //go:build line. The write is still a request to hide the
// file, so it is refused and nothing is created. Judging only the landed
// bytes would accept it.
func TestHandleWriteFile_RefusesAskedHeaderConstraints(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	res, err := vs.handleWriteFile(context.Background(), ActionRequest{
		ActionID: "ignore-write",
		Target:   "hidden.go",
		Payload:  map[string]any{"content": "//go:build ignore\n\npackage hidden\n"},
	})
	if err != nil {
		t.Fatalf("handleWriteFile: %v", err)
	}
	if res.Success {
		t.Fatal("a write that asked for //go:build ignore was accepted")
	}
	if !strings.Contains(res.Error, "ignore") || !strings.Contains(res.Error, "excludes") {
		t.Fatalf("error %q does not say the file would be excluded", res.Error)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "hidden.go")); !os.IsNotExist(statErr) {
		t.Fatalf("the refused write landed (stat %v)", statErr)
	}
}

func TestHandleWriteFile_WritesCleanGo(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	res, err := vs.handleWriteFile(context.Background(), ActionRequest{
		ActionID: "clean-write",
		Target:   "shown.go",
		Payload:  map[string]any{"content": "package shown\n\nfunc A() {}\n"},
	})
	if err != nil || !res.Success {
		t.Fatalf("clean write: err=%v res=%+v", err, res)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, "shown.go"))
	if readErr != nil || !strings.Contains(string(data), "package shown") {
		t.Fatalf("clean write did not land: %v %q", readErr, data)
	}
}

func TestHandleEditFile_RefusesAddedGoBuildIgnore(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	before := "package p\n\nfunc A() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "ignore-edit",
		Target:   "a.go",
		Payload:  map[string]any{"old": "package p\n", "new": "//go:build ignore\n\npackage p\n"},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if res.Success {
		t.Fatal("an edit adding //go:build ignore was accepted")
	}
	if !strings.Contains(res.Error, "a.go") || !strings.Contains(res.Error, "ignore") {
		t.Fatalf("error %q names neither the path nor the constraint", res.Error)
	}
	if len(res.FactsToAdd) != 1 || res.FactsToAdd[0].Predicate != "edit_failed" {
		t.Fatalf("FactsToAdd = %+v, want one edit_failed fact", res.FactsToAdd)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.go")); string(data) != before {
		t.Fatalf("the refused edit still landed: %q", data)
	}
}

func TestHandleEditFile_RefusesAddedPlusBuildIgnore(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	before := "package p\n\nfunc A() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "plus-ignore-edit",
		Target:   "a.go",
		Payload:  map[string]any{"old": "package p\n", "new": "// +build ignore\n\npackage p\n"},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if res.Success {
		t.Fatal("an edit adding // +build ignore was accepted")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.go")); string(data) != before {
		t.Fatalf("the refused edit still landed: %q", data)
	}
}

// Any excluding constraint is refused, not just ignore: a file gated to a
// platform this build is not running is hidden from this build.
func TestHandleEditFile_RefusesExcludingPlatformConstraint(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	before := "package p\n\nfunc A() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "platform-edit",
		Target:   "a.go",
		Payload: map[string]any{
			"old": "package p\n",
			"new": "//go:build " + other + "\n\npackage p\n",
		},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if res.Success {
		t.Fatalf("an edit adding //go:build %s on %s was accepted", other, runtime.GOOS)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.go")); string(data) != before {
		t.Fatalf("the refused edit still landed: %q", data)
	}
}

// A constraint the current build satisfies is not an exclusion: gating a
// file to the platform it builds on stays writable.
func TestHandleEditFile_AllowsIncludedPlatformConstraint(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	before := "package p\n\nfunc A() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "platform-ok-edit",
		Target:   "a.go",
		Payload: map[string]any{
			"old": "package p\n",
			"new": "//go:build " + runtime.GOOS + "\n\npackage p\n",
		},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if !res.Success {
		t.Fatalf("an edit adding //go:build %s was refused: %s", runtime.GOOS, res.Error)
	}
}

// A file that already had the constraint is not affected: only added lines
// are judged, so an excluded file stays editable.
func TestHandleEditFile_AllowsEditToFileThatAlreadyHadIgnore(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	before := "//go:build ignore\n\npackage p\n\nfunc A() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "already-ignored-edit",
		Target:   "a.go",
		Payload:  map[string]any{"old": "func A() {}", "new": "func B() {}"},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if !res.Success {
		t.Fatalf("an edit to a file that already had //go:build ignore was refused: %s", res.Error)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a.go")); !strings.Contains(string(data), "func B() {}") {
		t.Fatalf("the allowed edit did not land: %q", data)
	}
}

// A bare mention is not a constraint: prose after the package clause that
// names //go:build ignore must not trip the guard.
func TestHandleEditFile_AllowsIgnoreMentionOutsideTheHeader(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	before := "package p\n\nfunc A() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "mention-edit",
		Target:   "a.go",
		Payload: map[string]any{
			"old": "func A() {}",
			"new": "func A() {}\n\n// Never add //go:build ignore here.",
		},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if !res.Success {
		t.Fatalf("an edit merely mentioning //go:build ignore was refused: %s", res.Error)
	}
}

func TestHandleWriteFile_NonGoFileUnaffected(t *testing.T) {
	vs, dir := buildExclusionTestVS(t)
	res, err := vs.handleWriteFile(context.Background(), ActionRequest{
		ActionID: "txt-write",
		Target:   "notes.txt",
		Payload:  map[string]any{"content": "//go:build ignore\n\nnot go\n"},
	})
	if err != nil {
		t.Fatalf("handleWriteFile: %v", err)
	}
	if !res.Success {
		t.Fatalf("a non-Go write was refused: %s", res.Error)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "notes.txt")); statErr != nil {
		t.Fatalf("the allowed write did not land: %v", statErr)
	}
}
