package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

// TestParanoidValidator_EmptyTargetPath tests validation with empty target path
func TestParanoidValidator_EmptyTargetPath(t *testing.T) {
	v := NewParanoidFileValidator()

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: "", // Empty path
		Payload: map[string]interface{}{
			"content": "test content",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should fail validation with empty path
	if vr.Verified {
		t.Error("Expected Verified=false for empty target path")
	}
}

// TestParanoidValidator_MissingContentKey tests validation with missing content in payload
func TestParanoidValidator_MissingContentKey(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"notContent": "wrong key", // Missing "content" key
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should fail validation without content key
	if vr.Verified {
		t.Error("Expected Verified=false for missing content key")
	}
}

// TestParanoidValidator_NilPayload tests validation with nil payload
func TestParanoidValidator_NilPayload(t *testing.T) {
	v := NewParanoidFileValidator()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:    ActionWriteFile,
		Target:  path,
		Payload: nil, // nil payload
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should not panic with nil payload
	if vr.Verified {
		t.Error("Expected Verified=false for nil payload")
	}
}

// TestParanoidValidator_ContentWrongType tests validation with non-string content
func TestParanoidValidator_ContentWrongType(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("123"), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"content": 123, // Integer instead of string
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should handle non-string content gracefully
	// May verify false or convert to string - either is acceptable
	_ = vr // Just ensure no panic
}

// TestParanoidValidator_TargetIsDirectory tests validation when target is a directory
func TestParanoidValidator_TargetIsDirectory(t *testing.T) {
	v := NewParanoidFileValidator()

	tmpDir := t.TempDir() // This is a directory

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: tmpDir, // Directory instead of file
		Payload: map[string]interface{}{
			"content": "test",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should fail validation for directory target
	if vr.Verified {
		t.Error("Expected Verified=false for directory target")
	}
}

// TestParanoidValidator_NonExistentFile tests validation for non-existent file
func TestParanoidValidator_NonExistentFile(t *testing.T) {
	v := NewParanoidFileValidator()

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: "/nonexistent/path/to/file.txt",
		Payload: map[string]interface{}{
			"content": "test",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should fail validation for non-existent file
	if vr.Verified {
		t.Error("Expected Verified=false for non-existent file")
	}
}

func TestParanoidValidator_FileSizeBelowMin(t *testing.T) {
	v := NewParanoidFileValidator()
	v.MinFileSizeBytes = 100

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "small.txt")
	content := "too small"

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

	if vr.Verified {
		t.Error("Expected Verified=false for file size below min")
	}
}

func TestParanoidValidator_FileSizeExceedsMax(t *testing.T) {
	v := NewParanoidFileValidator()
	v.MaxFileSizeBytes = 10

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.txt")
	content := "this is larger than 10 bytes"

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

	if vr.Verified {
		t.Error("Expected Verified=false for file size exceeding max")
	}
}

func TestParanoidValidator_ReadPermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping read permission test on Windows")
	}

	v := NewParanoidFileValidator()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "unreadable.txt")
	content := "content"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Remove read permissions
	if err := os.Chmod(path, 0200); err != nil {
		t.Fatalf("Failed to chmod: %v", err)
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

	if vr.Verified {
		t.Error("Expected Verified=false for unreadable file")
	}
}

func TestParanoidValidator_ContentSampling(t *testing.T) {
	v := NewParanoidFileValidator()
	v.SamplePoints = 5
	v.RequireDoubleRead = false

	// Create content > 100 bytes (threshold for sampling)
	content := ""
	for i := 0; i < 20; i++ {
		content += "0123456789" // 200 bytes
	}

	v.MaxFileSizeBytes = 1000 // Ensure valid size

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sampled.txt")

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
		t.Errorf("Expected Verified=true for sampled file, got error: %v", vr.Error)
	}

	// Verify details
	if samples, ok := vr.Details["sample_points"].(int); !ok || samples != 5 {
		t.Errorf("Expected sample_points=5 in details, got %v", vr.Details["sample_points"])
	}
}

// TODO: TEST_GAP: Verify behavior when context is cancelled (ctx.Done()) before or during validation.

// TODO: TEST_GAP: Verify behavior when file is modified between first and second read (Race Condition Simulation).

// TODO: TEST_GAP: Verify behavior when target is a symlink (ensure it follows or rejects based on policy).

// TODO: TEST_GAP: Verify behavior when file is deleted between os.Stat and os.ReadFile.

// TODO: TEST_GAP: Verify behavior with extremely large files (OOM protection check).

// TODO: TEST_GAP: Verify behavior when SamplePoints is negative (should default or be ignored).
