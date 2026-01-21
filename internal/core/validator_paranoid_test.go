package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParanoidValidator_New(t *testing.T) {
	v := NewParanoidFileValidator()
	if v == nil {
		t.Fatal("NewParanoidFileValidator returned nil")
	}
	if v.MaxStaleSeconds != 30 {
		t.Errorf("Expected MaxStaleSeconds 30, got %d", v.MaxStaleSeconds)
	}
	if !v.RequireDoubleRead {
		t.Error("Expected RequireDoubleRead to be true by default")
	}
}

func TestParanoidValidator_CanValidate(t *testing.T) {
	v := NewParanoidFileValidator()

	testCases := []struct {
		action ActionType
		want   bool
	}{
		{ActionWriteFile, true},
		{ActionFSWrite, true},
		{ActionEditFile, true},
		{ActionRunTests, false},
		{ActionListFiles, false},
	}

	for _, tc := range testCases {
		got := v.CanValidate(tc.action)
		if got != tc.want {
			t.Errorf("CanValidate(%v) = %v, want %v", tc.action, got, tc.want)
		}
	}
}

func TestParanoidValidator_ValidateSuccess(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false // Speed up test
	v.MaxStaleSeconds = 60

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	content := "hello world"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"content": content,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if !vr.Verified {
		t.Errorf("Expected Verified=true, got false. Error: %s", vr.Error)
	}
	if vr.Confidence != 1.0 {
		t.Errorf("Expected Confidence=1.0, got %f", vr.Confidence)
	}
}

func TestParanoidValidator_ValidateMismatch(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false
	v.MaxStaleSeconds = 60

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	actualContent := "actual content"
	expectedContent := "expected content"

	if err := os.WriteFile(path, []byte(actualContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"content": expectedContent,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Errorf("Expected Verified=false for content mismatch")
	}
}

func TestParanoidValidator_ValidateStale(t *testing.T) {
	v := NewParanoidFileValidator()
	v.MaxStaleSeconds = 1 // 1 second max

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	content := "content"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Wait for file to become stale
	time.Sleep(2 * time.Second)

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"content": content,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Errorf("Expected Verified=false for stale file")
	}
}

// TODO: TEST_GAP: Missing test for null/empty 'Target' path in ActionRequest.

// TODO: TEST_GAP: Missing test for missing "content" key in 'Payload' for ActionWriteFile.

// TODO: TEST_GAP: Missing test for nil 'Payload' in ActionRequest.

// TODO: TEST_GAP: Verify behavior when 'content' in Payload is incorrect type (e.g., int or []byte instead of string).

// TODO: TEST_GAP: Missing test for file size below 'MinFileSizeBytes' (requires configuring MinFileSizeBytes > 0).

// TODO: TEST_GAP: Missing test for file size exceeding 'MaxFileSizeBytes'.

// TODO: TEST_GAP: Verify behavior when the target path points to a directory instead of a regular file.

// TODO: TEST_GAP: Missing test for non-existent file (os.Stat failure).

// TODO: TEST_GAP: Verify behavior when file exists but read permissions are denied (os.ReadFile failure).

// TODO: TEST_GAP: Missing test for double-read inconsistency (race condition where file changes between reads).

// TODO: TEST_GAP: Verify content sampling logic for large files (partial match failure).
