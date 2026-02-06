//go:build integration
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FileValidatorIntegrationSuite struct {
	suite.Suite
	testDir string
	ctx     context.Context
}

func (s *FileValidatorIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
	s.testDir = s.T().TempDir()
}

// =============================================================================
// FileWriteValidator Tests
// =============================================================================

func (s *FileValidatorIntegrationSuite) TestFileWriteValidator_Success() {
	v := NewFileWriteValidator()
	filePath := filepath.Join(s.testDir, "test_write.txt")
	content := "Hello, World!"

	// Simulate successful write action
	err := os.WriteFile(filePath, []byte(content), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: filePath,
		Payload: map[string]interface{}{
			"content": content,
		},
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.True(validation.Verified, "Validator should verify successful write")
	s.Equal(ValidationMethodHash, validation.Method)
	s.Empty(validation.Error)
}

func (s *FileValidatorIntegrationSuite) TestFileWriteValidator_ContentMismatch() {
	v := NewFileWriteValidator()
	filePath := filepath.Join(s.testDir, "test_write_mismatch.txt")
	actualContent := "Actual Content"
	expectedContent := "Expected Content"

	// Write actual content
	err := os.WriteFile(filePath, []byte(actualContent), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: filePath,
		Payload: map[string]interface{}{
			"content": expectedContent,
		},
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.False(validation.Verified, "Validator should fail on content mismatch")
	s.Equal(ValidationMethodHash, validation.Method)
	s.Contains(validation.Error, "content hash mismatch")

	// Verify details contain hash info
	expectedHash := sha256.Sum256([]byte(expectedContent))
	actualHash := sha256.Sum256([]byte(actualContent))
	s.Equal(hex.EncodeToString(expectedHash[:8]), validation.Details["expected_hash"])
	s.Equal(hex.EncodeToString(actualHash[:8]), validation.Details["actual_hash"])
}

func (s *FileValidatorIntegrationSuite) TestFileWriteValidator_FileDoesNotExist() {
	v := NewFileWriteValidator()
	filePath := filepath.Join(s.testDir, "nonexistent.txt")

	// Do NOT write the file

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: filePath,
		Payload: map[string]interface{}{
			"content": "some content",
		},
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.False(validation.Verified, "Validator should fail if file does not exist")
	s.Contains(validation.Error, "file does not exist")
}

func (s *FileValidatorIntegrationSuite) TestFileWriteValidator_NoExpectedContent() {
	v := NewFileWriteValidator()
	filePath := filepath.Join(s.testDir, "exist_check.txt")

	err := os.WriteFile(filePath, []byte("some content"), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionWriteFile,
		Target: filePath,
		Payload: map[string]interface{}{}, // No content expectation
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.True(validation.Verified)
	s.Equal(ValidationMethodExistence, validation.Method)
}

// =============================================================================
// FileEditValidator Tests
// =============================================================================

func (s *FileValidatorIntegrationSuite) TestFileEditValidator_Success() {
	v := NewFileEditValidator()
	filePath := filepath.Join(s.testDir, "test_edit.txt")

	// Simulate "edited" file state (contains NEW, lacks OLD)
	finalContent := "This is the new content."
	err := os.WriteFile(filePath, []byte(finalContent), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionEditFile,
		Target: filePath,
		Payload: map[string]interface{}{
			"old": "old content",
			"new": "new content",
		},
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.True(validation.Verified)
	s.Equal(ValidationMethodContentCheck, validation.Method)
}

func (s *FileValidatorIntegrationSuite) TestFileEditValidator_OldContentStillPresent() {
	v := NewFileEditValidator()
	filePath := filepath.Join(s.testDir, "test_edit_fail_old.txt")

	// Simulate bad edit (OLD content still there)
	content := "This still has the old content here."
	err := os.WriteFile(filePath, []byte(content), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionEditFile,
		Target: filePath,
		Payload: map[string]interface{}{
			"old": "old content",
			"new": "new content",
		},
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.False(validation.Verified)
	s.Contains(validation.Error, "old content still present")
}

func (s *FileValidatorIntegrationSuite) TestFileEditValidator_NewContentMissing() {
	v := NewFileEditValidator()
	filePath := filepath.Join(s.testDir, "test_edit_fail_new.txt")

	// Simulate bad edit (NEW content not added)
	content := "This is missing the target string."
	err := os.WriteFile(filePath, []byte(content), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionEditFile,
		Target: filePath,
		Payload: map[string]interface{}{
			"old": "old content",
			"new": "required new content",
		},
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.False(validation.Verified)
	s.Contains(validation.Error, "new content not found")
}

// =============================================================================
// FileDeleteValidator Tests
// =============================================================================

func (s *FileValidatorIntegrationSuite) TestFileDeleteValidator_Success() {
	v := NewFileDeleteValidator()
	filePath := filepath.Join(s.testDir, "test_delete.txt")

	// Ensure file is gone (it never existed, which is fine for post-delete check)
	// But strictly, we might want to create it then delete it to be sure logic holds.
	// The validator just checks os.Stat returns error.

	req := ActionRequest{
		Type:   ActionDeleteFile,
		Target: filePath,
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.True(validation.Verified)
	s.Equal(ValidationMethodExistence, validation.Method)
}

func (s *FileValidatorIntegrationSuite) TestFileDeleteValidator_FileStillExists() {
	v := NewFileDeleteValidator()
	filePath := filepath.Join(s.testDir, "test_delete_fail.txt")

	// Create file so it exists
	err := os.WriteFile(filePath, []byte("I am still here"), 0644)
	s.Require().NoError(err)

	req := ActionRequest{
		Type:   ActionDeleteFile,
		Target: filePath,
	}
	result := ActionResult{
		Success: true,
	}

	validation := v.Validate(s.ctx, req, result)
	s.False(validation.Verified)
	s.Contains(validation.Error, "file still exists")
}

func TestFileValidatorIntegrationSuite(t *testing.T) {
	suite.Run(t, new(FileValidatorIntegrationSuite))
}
