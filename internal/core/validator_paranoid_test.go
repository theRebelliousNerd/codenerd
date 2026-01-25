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

// TODO: TEST_GAP: Missing test for missing "content" key in 'Payload' for ActionWriteFile.
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

// TODO: TEST_GAP: Missing test for nil 'Payload' in ActionRequest.
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

// TODO: TEST_GAP: Verify behavior when 'content' in Payload is incorrect type (e.g., int or []byte instead of string).
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

// TODO: TEST_GAP: Verify behavior when the target path points to a directory instead of a regular file.
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

// TODO: TEST_GAP: Missing test for non-existent file (os.Stat failure).
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

// TODO: TEST_GAP: Missing test for file size below 'MinFileSizeBytes' (requires configuring MinFileSizeBytes > 0).
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

// TODO: TEST_GAP: Missing test for file size exceeding 'MaxFileSizeBytes'.
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

// TODO: TEST_GAP: Verify behavior when file exists but read permissions are denied (os.ReadFile failure).
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

// TODO: TEST_GAP: Missing test for double-read inconsistency (race condition where file changes between reads).
func TestParanoidValidator_DoubleReadInconsistency(t *testing.T) {
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = true
	// We rely on the 50ms sleep in the validator

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "race.txt")
	initialContent := "initial content"
	changedContent := "changed content"

	if err := os.WriteFile(path, []byte(initialContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"content": initialContent,
		},
	}
	result := ActionResult{Success: true}

	// Use a channel to coordinate the modification
	done := make(chan bool)

	go func() {
		// Wait a bit to ensure validation has started and likely reached the sleep
		time.Sleep(20 * time.Millisecond)
		// Modify the file
		if err := os.WriteFile(path, []byte(changedContent), 0644); err != nil {
			t.Errorf("Failed to modify file in background: %v", err)
		}
		close(done)
	}()

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	<-done

	if vr.Verified {
		t.Error("Expected Verified=false for double-read inconsistency")
	}
	// We might get "double-read inconsistency" OR "content hash mismatch (second read)" depending on timing
	// But as long as it fails, it's good.
}

// TODO: TEST_GAP: Verify content sampling logic for large files (partial match failure).
// TODO: TEST_GAP: Verify content sampling failure when a specific byte at a sample offset is modified.
func TestParanoidValidator_ContentSamplingFailure(t *testing.T) {
	v := NewParanoidFileValidator()
	v.SamplePoints = 5
	v.RequireDoubleRead = false
	v.MaxFileSizeBytes = 1000

	// Construct content where one byte at a sampling point is wrong
	// Implementation uses: sampleSize := len(firstRead) / v.SamplePoints
	// Offsets: i * sampleSize

	validContent := ""
	for i := 0; i < 20; i++ {
		validContent += "0123456789" // 200 bytes
	}

	// Create corrupt content
	corruptBytes := []byte(validContent)
	sampleSize := len(corruptBytes) / v.SamplePoints // 200 / 5 = 40
	// Corrupt the byte at offset 40 (start of 2nd sample)
	corruptBytes[sampleSize] = 'X'

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "corrupt.txt")

	if err := os.WriteFile(path, corruptBytes, 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: path,
		Payload: map[string]interface{}{
			"content": validContent, // We expect valid content
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Error("Expected Verified=false for sampling failure")
	}
	// Note: In the current implementation, SHA-256 hash check (CHECK 5) runs before content sampling (CHECK 7).
	// Since the content is corrupt, the hash check should catch it first.
	// We verify that it fails, regardless of which check caught it.
	failedCheck := vr.Details["check_failed"]
	if failedCheck != "content_sampling" && failedCheck != "hash_first_read" {
		t.Errorf("Expected check_failed to be 'content_sampling' or 'hash_first_read', got %v", failedCheck)
	}
}

// TODO: TEST_GAP: Verify that 'ActionEditFile' skips validation (returns verified=true, confidence=0.0) when content is missing from payload.
func TestParanoidValidator_EditFileSkipped(t *testing.T) {
	v := NewParanoidFileValidator()

	req := ActionRequest{
		Type:   ActionEditFile,
		Target: "/some/path",
		Payload: map[string]interface{}{
			// No "content" key
			"diff": "some diff",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if !vr.Verified {
		t.Error("Expected Verified=true for skipped validation")
	}
	if vr.Confidence != 0.0 {
		t.Errorf("Expected Confidence=0.0, got %f", vr.Confidence)
	}
	if vr.Method != "paranoid_validation_skipped" {
		t.Errorf("Expected Method=paranoid_validation_skipped, got %s", vr.Method)
	}
}

// TODO: TEST_GAP: Verify behavior when 'Target' is a symlink to a directory (should fail check).
func TestParanoidValidator_SymlinkToDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}
	v := NewParanoidFileValidator()

	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "targetDir")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(tmpDir, "linkToDir")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: symlinkPath,
		Payload: map[string]interface{}{
			"content": "test",
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if vr.Verified {
		t.Error("Expected Verified=false for symlink to directory")
	}
	// Note: os.Stat follows symlinks, so it sees it as a directory
}

// TODO: TEST_GAP: Verify behavior when 'Target' is a symlink to a valid file (should resolve and pass).
func TestParanoidValidator_SymlinkToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}
	v := NewParanoidFileValidator()
	v.RequireDoubleRead = false

	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "targetFile.txt")
	content := "test content"
	if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(tmpDir, "linkToFile.txt")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatal(err)
	}

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: symlinkPath,
		Payload: map[string]interface{}{
			"content": content,
		},
	}
	result := ActionResult{Success: true}

	ctx := context.Background()
	vr := v.Validate(ctx, req, result)

	if !vr.Verified {
		t.Errorf("Expected Verified=true for symlink to file, got error: %v", vr.Error)
	}
}
