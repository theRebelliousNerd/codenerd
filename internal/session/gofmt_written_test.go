package session

import (
	"bytes"
	"context"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

func TestFormatWrittenGoFiles(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	uglyOriginal := "package a\nfunc F() {}\nfunc G() {}\n\n\n"
	fineSrc := "package a\n\nvar X = 1\n"
	fineFormatted, err := format.Source([]byte(fineSrc))
	if err != nil {
		t.Fatalf("failed to format fine source: %v", err)
	}
	brokenContent := "package a\nfunc (\n"
	notesContent := "func (\n"

	initial := map[string][]byte{
		"a/ugly.go":   []byte(uglyOriginal),
		"a/fine.go":   fineFormatted,
		"a/broken.go": []byte(brokenContent),
		"a/notes.txt": []byte(notesContent),
	}
	for rel, data := range initial {
		if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rel)), data, 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	got := formatWrittenGoFiles(ws, []string{"a/ugly.go", "a/fine.go", "a/broken.go", "a/notes.txt", "a/missing.go"})

	if len(got) != 1 || got[0] != "a/ugly.go" {
		t.Fatalf("formatWrittenGoFiles() = %q, want %q", got, []string{"a/ugly.go"})
	}

	expectedUgly, err := format.Source([]byte(uglyOriginal))
	if err != nil {
		t.Fatalf("failed to format ugly source: %v", err)
	}
	uglyOnDisk, err := os.ReadFile(filepath.Join(ws, "a", "ugly.go"))
	if err != nil {
		t.Fatalf("failed to read ugly.go: %v", err)
	}
	if !bytes.Equal(uglyOnDisk, expectedUgly) {
		t.Errorf("ugly.go on disk = %q, want %q", uglyOnDisk, expectedUgly)
	}

	for _, rel := range []string{"a/fine.go", "a/broken.go", "a/notes.txt"} {
		onDisk, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("failed to read %s: %v", rel, err)
		}
		if !bytes.Equal(onDisk, initial[rel]) {
			t.Errorf("%s changed: got %q, want %q", rel, onDisk, initial[rel])
		}
	}
}

func TestFormatWrittenGoFiles_KeepsCRLF(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a"), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	fineCRLF := "package a\r\n\r\nvar X = 1\r\n"
	uglyCRLF := "package a\r\nfunc F() {}\r\nfunc G() {}\r\n"
	initial := map[string][]byte{
		"a/crlf_fine.go": []byte(fineCRLF),
		"a/crlf_ugly.go": []byte(uglyCRLF),
	}
	for rel, data := range initial {
		if err := os.WriteFile(filepath.Join(ws, filepath.FromSlash(rel)), data, 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	got := formatWrittenGoFiles(ws, []string{"a/crlf_fine.go", "a/crlf_ugly.go"})
	if len(got) != 1 || got[0] != "a/crlf_ugly.go" {
		t.Fatalf("formatWrittenGoFiles() = %q, want %q", got, []string{"a/crlf_ugly.go"})
	}

	fineOnDisk, err := os.ReadFile(filepath.Join(ws, "a", "crlf_fine.go"))
	if err != nil {
		t.Fatalf("failed to read crlf_fine.go: %v", err)
	}
	if !bytes.Equal(fineOnDisk, initial["a/crlf_fine.go"]) {
		t.Errorf("crlf_fine.go changed: got %q, want %q", fineOnDisk, initial["a/crlf_fine.go"])
	}

	uglyOnDisk, err := os.ReadFile(filepath.Join(ws, "a", "crlf_ugly.go"))
	if err != nil {
		t.Fatalf("failed to read crlf_ugly.go: %v", err)
	}
	if !strings.Contains(string(uglyOnDisk), "\r\n") {
		t.Errorf("crlf_ugly.go has no CRLF: %q", uglyOnDisk)
	}
	for i, b := range uglyOnDisk {
		if b == '\n' && (i == 0 || uglyOnDisk[i-1] != '\r') {
			t.Fatalf("crlf_ugly.go has lone LF at byte %d: %q", i, uglyOnDisk)
		}
	}
	uglyLF := "package a\nfunc F() {}\nfunc G() {}\n"
	formattedLF, err := format.Source([]byte(uglyLF))
	if err != nil {
		t.Fatalf("failed to format LF ugly source: %v", err)
	}
	expected := tactile.NormalizeLineEnding(string(formattedLF), "\r\n")
	if string(uglyOnDisk) != expected {
		t.Errorf("crlf_ugly.go = %q, want %q", uglyOnDisk, expected)
	}
}

func TestVerifyCompletedToolTurn_LateRoundWriteEndsFormatted(t *testing.T) {
	const code = "package main\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n\nfunc main() {}\n"
	const emptyTest = "package main\n\nimport \"testing\"\n\nfunc TestProbe(t *testing.T) {}\n"
	// Doubled blank line between the two test functions: valid Go that
	// passes, but not gofmt-clean. Mirrors the 2026-09-19 session where the
	// coverage round's insert left exactly this and the turn still ended
	// checks_passed with no second "gofmt: formatted" line.
	const unformattedTest = "package main\n\nimport \"testing\"\n\nfunc TestProbe(t *testing.T) {}\n\n\nfunc TestDouble(t *testing.T) {\n\tif Double(2) != 4 {\n\t\tt.Fatal(\"Double(2) != 4\")\n\t}\n}\n"

	h := newRepairHarness(t, nil)
	initial := func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), code),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), emptyTest),
		}}
	}
	h.executor.llmClient = &MockToolResultsLLM{
		MockLLMClient: &MockLLMClient{
			CompleteWithToolsFunc: func(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
				return initial(), nil
			},
		},
		CompleteWithToolResultsFunc: func(_ context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			if last := history[len(history)-1]; strings.Contains(last.Text, "no test executes these lines") {
				return &types.LLMToolResponse{Text: "testing Double", ToolCalls: []types.ToolCall{
					h.writeCall("t1", "write_file", filepath.Join(h.ws, "main_test.go"), unformattedTest),
				}}, nil
			}
			for _, m := range history {
				if len(m.ToolResults) > 0 {
					return &types.LLMToolResponse{Text: "done"}, nil
				}
			}
			return initial(), nil
		},
	}
	result, err := h.drive(t, "fix add a Double function")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if result.TestCheck.Verdict() != VerifyPassed {
		t.Fatalf("TestCheck = %+v, want passed", result.TestCheck)
	}
	data, readErr := os.ReadFile(filepath.Join(h.ws, "main_test.go"))
	if readErr != nil {
		t.Fatalf("reading main_test.go: %v", readErr)
	}
	if !strings.Contains(string(data), "TestDouble") {
		t.Fatalf("the coverage round's test did not land on disk:\n%s", data)
	}
	formatted, fmtErr := format.Source(data)
	if fmtErr != nil {
		t.Fatalf("main_test.go does not parse after the turn: %v\n%s", fmtErr, data)
	}
	if !bytes.Equal(data, formatted) {
		t.Fatalf("main_test.go is not gofmt-clean when the turn ends:\ngot:\n%s\nwant:\n%s", data, formatted)
	}
}
