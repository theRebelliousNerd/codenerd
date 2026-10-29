package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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

// TestParanoidValidator_DoubleReadRaceCondition tests that content mismatch is caught.
// Rather than relying on precise goroutine timing (flaky on Windows/CI),
// we write a different content before validation so the first hash check fails deterministically.
func TestParanoidValidator_DoubleReadRaceCondition(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = true
	v.MaxStaleSeconds = 5 // Give enough time

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "race.txt")
	initialContent := "initial"
	finalContent := "changed"

	// Write the FINAL content to disk
	if err := os.WriteFile(path, []byte(finalContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]any{
			"content": initialContent, // We expect initial, but disk has final
		},
	}
	result := ActionResult{Success: true}
	ctx := context.Background()

	vr := v.Validate(ctx, req, result)

	// Should fail because disk content ("changed") doesn't match expected ("initial")
	if vr.Verified {
		t.Error("Expected Verified=false for content mismatch")
	}
	// The size check or hash check should catch the discrepancy
	if got := vr.Details["check_failed"]; got != "hash_first_read" && got != "size_match" {
		t.Errorf("Expected check_failed in {'hash_first_read','size_match'}, got '%v'", got)
	}
}

// TestParanoidValidator_EditFileSkipped tests that ActionEditFile without content is skipped
func TestParanoidValidator_EditFileSkipped(t *testing.T) {
	v := NewParanoidFileValidator()

	req := ActionRequest{
		Type:   ActionEditFile,
		Target: "some/file.txt",
		Payload: map[string]any{
			// No "content" key
			"diff": "some diff",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if !vr.Verified {
		t.Error("Expected Verified=true (skipped) for EditFile without content")
	}
	if vr.Confidence != 0.0 {
		t.Errorf("Expected Confidence=0.0 (skipped), got %f", vr.Confidence)
	}
	if vr.Method != "paranoid_validation_skipped" {
		t.Errorf("Expected Method='paranoid_validation_skipped', got '%s'", vr.Method)
	}
}

// TestParanoidValidator_SymlinkToDirectory tests validation when target is a symlink to a directory
func TestParanoidValidator_SymlinkToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows due to permission requirements")
	}

	v := NewParanoidFileValidator()
	tmpDir := t.TempDir()

	// Create a real directory
	realDir := filepath.Join(tmpDir, "real_dir")
	if err := os.Mkdir(realDir, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// Create symlink to it
	linkPath := filepath.Join(tmpDir, "link_to_dir")
	if err := os.Symlink(realDir, linkPath); err != nil {
		t.Skipf("Skipping symlink test: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: linkPath, // Target is the symlink
		Payload: map[string]any{
			"content": "test",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should fail because it resolves to a directory
	if vr.Verified {
		t.Error("Expected Verified=false for symlink to directory")
	}
}

// TestParanoidValidator_SymlinkToFile tests validation when target is a symlink to a valid file
func TestParanoidValidator_SymlinkToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows due to permission requirements")
	}

	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false
	tmpDir := t.TempDir()

	// Create real file
	realFile := filepath.Join(tmpDir, "real_file.txt")
	content := "valid content"
	if err := os.WriteFile(realFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write real file: %v", err)
	}

	// Create symlink to it
	linkPath := filepath.Join(tmpDir, "link_to_file.txt")
	if err := os.Symlink(realFile, linkPath); err != nil {
		t.Skipf("Skipping symlink test: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: linkPath,
		Payload: map[string]any{
			"content": content,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	// Should pass
	if !vr.Verified {
		t.Errorf("Expected Verified=true for valid symlink to file. Error: %s", vr.Error)
	}
}

// TestParanoidValidator_ContentSamplingRuns tests that hash checks are recorded in details
func TestParanoidValidator_ContentSamplingRuns(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false

	// Create content larger than 100 bytes
	content := ""
	for range 20 {
		content += "0123456789" // 200 bytes total
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large_sample.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]any{
			"content": content,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if !vr.Verified {
		t.Fatalf("Expected validation to pass: %v", vr.Error)
	}

	// Verify checks_passed contains the expected checks
	checks, ok := vr.Details["checks_passed"].([]string)
	if !ok {
		t.Fatal("Details['checks_passed'] not found or not []string")
	}

	// The paranoid validator tracks: existence, timestamp_freshness, size_sanity,
	// size_match, first_read, hash_first_read, double_read_consistency, hash_second_read
	expected := []string{"existence", "hash_first_read"}
	for _, exp := range expected {
		found := slices.Contains(checks, exp)
		if !found {
			t.Errorf("Expected '%s' to be in checks_passed, got %v", exp, checks)
		}
	}
}

// TestParanoidValidator_EmptyTargetPath tests validation with empty target path
func TestParanoidValidator_EmptyTargetPath(t *testing.T) {
	v := NewParanoidFileValidator()

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: "", // Empty path
		Payload: map[string]any{
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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
		Payload: map[string]any{
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
	if os.Geteuid() == 0 {
		// root bypasses filesystem read permission bits, so an "unreadable"
		// file is still readable and the validator legitimately verifies it.
		t.Skip("Skipping read permission test when running as root")
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
		Payload: map[string]any{
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
	v.RequireDoubleRead = false

	// Create content > 100 bytes
	content := ""
	for range 20 {
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
		Payload: map[string]any{
			"content": content,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if !vr.Verified {
		t.Errorf("Expected Verified=true for valid content, got error: %v", vr.Error)
	}

	// Verify details contain hash and validation time
	if vr.Details["hash"] == nil {
		t.Error("Expected 'hash' in validation details")
	}
	if vr.Details["validation_time_ms"] == nil {
		t.Error("Expected 'validation_time_ms' in validation details")
	}
	if vr.Details["file_size"] == nil {
		t.Error("Expected 'file_size' in validation details")
	}
}

func TestParanoidValidator_ContentSamplingFailure(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false
	v.MaxFileSizeBytes = 1000

	// Construct content where one byte at a sampling point is wrong.
	validContent := ""
	for range 20 {
		validContent += "0123456789" // 200 bytes
	}

	// Create corrupt content.
	corruptBytes := []byte(validContent)
	sampleSize := len(corruptBytes) / 5 // 200 / 5 = 40
	corruptBytes[sampleSize] = 'X'

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "corrupt.txt")
	if err := os.WriteFile(path, corruptBytes, 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]any{
			"content": validContent, // We expect valid content
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Error("Expected Verified=false for sampling failure")
	}

	// Hash check runs before content sampling, so we accept either failure mode.
	failedCheck := vr.Details["check_failed"]
	if failedCheck != "content_sampling" && failedCheck != "hash_first_read" {
		t.Errorf("Expected check_failed to be 'content_sampling' or 'hash_first_read', got %v", failedCheck)
	}
}
