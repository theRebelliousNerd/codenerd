package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntaxValidator_FileParseErrorSurfacesLocation(t *testing.T) {
	v := NewSyntaxValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.go")
	content := "placeholder\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{Type: ActionWriteFile, Target: path}
	result := ActionResult{Success: true}

	vr := v.Validate(context.Background(), req, result)
	if vr.Verified {
		t.Fatal("Expected broken Go file to fail validation")
	}
	if !strings.HasPrefix(vr.Error, "syntax validation failed: ") {
		t.Errorf("Expected error with prefix %q, got %q", "syntax validation failed: ", vr.Error)
	}
	// Prefix must stay intact so the existing prefix match in
	// self_healing.go and the action_validator.go classification table
	// keep classifying this result as a syntax failure.
	if !strings.HasPrefix(vr.Error, "syntax validation failed") {
		t.Errorf("Expected error to keep syntax prefix for self-healing classification, got %q", vr.Error)
	}
	if !strings.Contains(vr.Error, ":1:") {
		t.Errorf("Expected error to contain %q, got %q", ":1:", vr.Error)
	}
	if !strings.Contains(vr.Error, "expected 'package'") {
		t.Errorf("Expected error to contain %q, got %q", "expected 'package'", vr.Error)
	}
}
