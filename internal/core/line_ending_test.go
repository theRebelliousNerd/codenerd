package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func countLineEndings(data []byte) (lf int, crlf int, lone int) {
	for i := 0; i < len(data); i++ {
		if data[i] != '\n' {
			continue
		}
		lf++
		if i > 0 && data[i-1] == '\r' {
			crlf++
		}
	}
	return lf, crlf, lf - crlf
}

// Counting matters here. `grep -c $'\r$'` reports every line of these files as
// CRLF and gives the wrong answer — it is what led to "all three files are 100%
// CRLF" when subagent.go in fact held 36 lone LFs. Every assertion below counts
// bytes.
func countEndings(t *testing.T, path string) (lf, crlf, lone int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return countLineEndings(data)
}

func TestDetectLineEnding(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"pure CRLF", "a\r\nb\r\nc\r\n", "\r\n"},
		{"pure LF", "a\nb\nc\n", "\n"},
		{"mostly CRLF with one stray LF", "a\r\nb\r\nc\r\nd\n", "\r\n"},
		{"mostly LF with one stray CRLF", "a\nb\nc\nd\r\n", "\n"},
		{"no newline at all", "single line", "\n"},
		{"empty", "", "\n"},
		// A file that is exactly half and half is not CRLF-dominant, so it stays
		// LF. Ties must not silently rewrite a file to CRLF.
		{"tie stays LF", "a\r\nb\n", "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectLineEnding([]byte(tc.in)); got != tc.want {
				t.Errorf("detectLineEnding(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeLineEnding_DoesNotDoubleUp(t *testing.T) {
	// Content already CRLF, normalized to CRLF, must not become \r\r\n.
	got := normalizeLineEnding("a\r\nb\r\n", "\r\n")
	if got != "a\r\nb\r\n" {
		t.Errorf("re-normalizing CRLF produced %q", got)
	}
	_, _, lone := countLineEndings([]byte(got))
	if lone != 0 {
		t.Errorf("expected 0 lone LF, got %d", lone)
	}
}

// Overwriting an existing CRLF file must leave it CRLF-only. This is the
// defect: WriteFile joined with "\n" unconditionally.
func TestWriteFile_PreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.go")
	if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vs := &VirtualStore{}
	if err := vs.WriteFile(path, []string{"package p", "", "func B() {}", ""}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	lf, crlf, lone := countEndings(t, path)
	if lone != 0 {
		t.Errorf("CRLF file gained %d lone LF after write (lf=%d crlf=%d)", lone, lf, crlf)
	}
	if crlf == 0 {
		t.Error("CRLF file was converted to LF")
	}
}

func TestWriteFile_PreservesLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lf.go")
	if err := os.WriteFile(path, []byte("package p\n\nfunc A() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	vs := &VirtualStore{}
	if err := vs.WriteFile(path, []string{"package p", "", "func B() {}", ""}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, crlf, _ := countEndings(t, path)
	if crlf != 0 {
		t.Errorf("LF file gained %d CRLF after write", crlf)
	}
}

// A file that does not exist yet must keep the previous behaviour rather than
// having a convention invented for it.
func TestWriteFile_NewFileKeepsLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.go")

	vs := &VirtualStore{}
	if err := vs.WriteFile(path, []string{"package p", "func A() {}", ""}); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, crlf, _ := countEndings(t, path)
	if crlf != 0 {
		t.Errorf("new file was written with %d CRLF; expected unchanged LF behaviour", crlf)
	}
}

// Splicing an LF replacement into a CRLF file must not leave lone LFs behind.
// This is exactly how internal/session/subagent.go ended up with 36 of them.
func TestNormalizeLineEnding_SplicedEditLeavesNoLoneLF(t *testing.T) {
	original := "package p\r\n\r\nfunc A() {}\r\n"
	// A replacement emitted with LF endings, as a model would produce it.
	spliced := "package p\r\n\r\nfunc A() {\n\treturn\n}\r\n"

	fixed := normalizeLineEnding(spliced, detectLineEnding([]byte(original)))

	_, crlf, lone := countLineEndings([]byte(fixed))
	if lone != 0 {
		t.Errorf("spliced edit left %d lone LF in a CRLF file", lone)
	}
	if crlf == 0 {
		t.Error("spliced edit converted the CRLF file to LF")
	}
}

func TestHandleWriteFile_PreservesCRLF(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	path := filepath.Join(dir, "write-handler.go")
	if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := vs.handleWriteFile(context.Background(), ActionRequest{
		ActionID: "preserve-crlf-write",
		Target:   filepath.Base(path),
		Payload: map[string]any{
			"content": "package p\n\nfunc B() {}\n",
		},
	})
	if err != nil {
		t.Fatalf("handleWriteFile: %v", err)
	}
	if !result.Success {
		t.Fatalf("handleWriteFile result: %+v", result)
	}

	_, crlf, lone := countEndings(t, path)
	if lone != 0 || crlf == 0 {
		t.Fatalf("write handler did not preserve CRLF: crlf=%d lone_lf=%d", crlf, lone)
	}
}

func TestHandleEditFile_PreservesCRLF(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	path := filepath.Join(dir, "edit-handler.go")
	if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := vs.handleEditFile(context.Background(), ActionRequest{
		ActionID: "preserve-crlf-edit",
		Target:   filepath.Base(path),
		Payload: map[string]any{
			"old": "package p\n\nfunc A() {}",
			"new": "package p\n\nfunc A() {\n\treturn\n}",
		},
	})
	if err != nil {
		t.Fatalf("handleEditFile: %v", err)
	}
	if !result.Success {
		t.Fatalf("handleEditFile result: %+v", result)
	}

	_, crlf, lone := countEndings(t, path)
	if lone != 0 || crlf == 0 {
		t.Fatalf("edit handler did not preserve CRLF: crlf=%d lone_lf=%d", crlf, lone)
	}
}

func TestTransactionManagerCommit_PreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transaction.go")
	if err := os.WriteFile(path, []byte("package p\r\n\r\nfunc A() {}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	kernel := &RealKernel{facts: make([]Fact, 0), policyDirty: true}
	tm := NewTransactionManager(kernel, dir)
	if _, err := tm.Begin(context.Background(), "preserve CRLF"); err != nil {
		t.Fatal(err)
	}
	if err := tm.AddEdit(context.Background(), FileEdit{
		FilePath: path,
		Content:  []byte("package p\n\nfunc B() {}\n"),
		EditType: EditTypeModify,
	}); err != nil {
		t.Fatal(err)
	}

	tm.mu.Lock()
	tm.txns[tm.activeTxnID].Status = TxnStatusReady
	tm.mu.Unlock()
	if err := tm.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, crlf, lone := countEndings(t, path)
	if lone != 0 || crlf == 0 {
		t.Fatalf("transaction commit did not preserve CRLF: crlf=%d lone_lf=%d", crlf, lone)
	}
}

type lineEndingRepairInterceptor struct{}

func (lineEndingRepairInterceptor) InterceptLearnedRule(_ context.Context, rule string) (string, error) {
	return rule + " # repaired", nil
}

func TestMangleWatcherRepairPreservesCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repair.mg")
	if err := os.WriteFile(path, []byte("one(/a).\r\n\r\ntwo(/b).\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	kernel := &RealKernel{}
	kernel.SetRepairInterceptor(lineEndingRepairInterceptor{})
	watcher := &MangleWatcher{kernel: kernel}
	watcher.validateAndRepair(context.Background(), path)

	_, crlf, lone := countEndings(t, path)
	if lone != 0 || crlf == 0 {
		t.Fatalf("Mangle watcher repair did not preserve CRLF: crlf=%d lone_lf=%d", crlf, lone)
	}
}
