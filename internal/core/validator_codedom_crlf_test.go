package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Working copies on Windows are CRLF and the model inserts LF text. The
// insert_lines content check compared them raw, failed every multi-line
// insert, and the discounted write pushed the coder turn to "hollow" — after
// which the campaign fallback overwrote the coder's correct work
// (campaign 149c512d, schemas_execution.mg, 2026-09-04).
func TestLineEditValidator_InsertLinesAcceptsCRLFFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.mg")
	inserted := "# Section 14\nDecl context_feedback(S, T, U) bound [/string, /number, /number].\nDecl context_missing(S, T, M) bound [/string, /number, /string].\n"
	onDisk := "Decl turn_cost(A).\r\n" + "# Section 14\r\nDecl context_feedback(S, T, U) bound [/string, /number, /number].\r\nDecl context_missing(S, T, M) bound [/string, /number, /string].\r\n"
	if err := os.WriteFile(path, []byte(onDisk), 0o644); err != nil {
		t.Fatal(err)
	}

	v := NewLineEditValidator()
	res := v.Validate(context.Background(), ActionRequest{
		ActionID: "a1",
		Type:     ActionInsertLines,
		Target:   path,
		Payload:  map[string]any{"content": inserted, "after": 1},
	}, ActionResult{Success: true})

	if !res.Verified {
		t.Fatalf("CRLF file with the LF-inserted content must validate, got error %q", res.Error)
	}
}

func TestLineEditValidator_InsertLinesStillRejectsMissingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.mg")
	if err := os.WriteFile(path, []byte("Decl a(X).\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	v := NewLineEditValidator()
	res := v.Validate(context.Background(), ActionRequest{
		ActionID: "a2",
		Type:     ActionInsertLines,
		Target:   path,
		Payload:  map[string]any{"content": "Decl never_written(Y).\n", "after": 1},
	}, ActionResult{Success: true})
	if res.Verified {
		t.Fatal("content absent from the file must still fail validation")
	}
}
