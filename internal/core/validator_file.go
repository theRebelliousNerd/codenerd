// validator_file.go provides post-action validators for file operations.
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// FileWriteValidator verifies that file write operations actually wrote the expected content.
// It performs read-back verification with hash comparison.
type FileWriteValidator struct{}

// NewFileWriteValidator creates a new file write validator.
func NewFileWriteValidator() *FileWriteValidator {
	return &FileWriteValidator{}
}

// CanValidate returns true for file write action types.
func (v *FileWriteValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionWriteFile ||
		actionType == ActionFSWrite
}

// Validate reads back the file and compares hash with expected content.
func (v *FileWriteValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	// If the action itself failed, we don't need to validate further
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "action reported failure: " + result.Error,
		}
	}

	path := req.Target
	if path == "" {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "no target path specified in action request",
		}
	}

	// 1. Check file exists
	info, err := os.Stat(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "file does not exist after write: " + err.Error(),
		}
	}

	if info.IsDir() {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "path is a directory, not a file",
		}
	}

	// 2. Read back content
	actualContent, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodExistence,
			Error:      "cannot read back file: " + err.Error(),
		}
	}

	// 3. Get expected content from payload
	expectedContent, hasContent := req.Payload["content"].(string)
	if !hasContent {
		// No expected content in payload - just verify existence and non-empty
		if len(actualContent) == 0 {
			return ValidationResult{
				Verified:   false,
				Confidence: 0.7,
				Method:     ValidationMethodExistence,
				Error:      "file exists but is empty",
			}
		}
		return ValidationResult{
			Verified:   true,
			Confidence: 0.6,
			Method:     ValidationMethodExistence,
			Details:    map[string]interface{}{"size": len(actualContent)},
		}
	}

	// 4. Hash comparison
	expectedHash := sha256.Sum256([]byte(expectedContent))
	actualHash := sha256.Sum256(actualContent)

	if expectedHash != actualHash {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodHash,
			Error:      "content hash mismatch",
			Details: map[string]interface{}{
				"expected_hash": hex.EncodeToString(expectedHash[:8]),
				"actual_hash":   hex.EncodeToString(actualHash[:8]),
				"expected_size": len(expectedContent),
				"actual_size":   len(actualContent),
			},
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodHash,
		Details: map[string]interface{}{
			"hash": hex.EncodeToString(actualHash[:8]),
			"size": len(actualContent),
		},
	}
}

// Name returns the validator name.
func (v *FileWriteValidator) Name() string { return "file_write_validator" }

// Priority returns the validator priority (runs first for file writes).
func (v *FileWriteValidator) Priority() int { return 10 }

// FileEditValidator verifies that file edit operations correctly applied the changes.
// It checks that old content is gone and new content is present.
type FileEditValidator struct{}

// NewFileEditValidator creates a new file edit validator.
func NewFileEditValidator() *FileEditValidator {
	return &FileEditValidator{}
}

// CanValidate returns true for file edit action types.
func (v *FileEditValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionEditFile
}

// Validate checks that the edit was applied correctly.
func (v *FileEditValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	// If the action itself failed, we don't need to validate further
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodContentCheck,
			Error:      "action reported failure: " + result.Error,
		}
	}

	path := req.Target
	if path == "" {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodContentCheck,
			Error:      "no target path specified in action request",
		}
	}

	// Read back file
	actualContent, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodContentCheck,
			Error:      "cannot read back file: " + err.Error(),
		}
	}

	actualStr := string(actualContent)

	// Get old and new content from payload
	oldContent, hasOld := req.Payload["old"].(string)
	newContent, hasNew := req.Payload["new"].(string)

	if !hasOld && !hasNew {
		// No expected patterns - just verify file exists and is readable
		return ValidationResult{
			Verified:   true,
			Confidence: 0.5,
			Method:     ValidationMethodExistence,
			Details:    map[string]interface{}{"size": len(actualContent)},
		}
	}

	// Verify old content is NOT present (was replaced)
	if hasOld && oldContent != "" && strings.Contains(actualStr, oldContent) {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.95,
			Method:     ValidationMethodContentCheck,
			Error:      "old content still present after edit",
			Details: map[string]interface{}{
				"old_content_preview": truncateStr(oldContent, 100),
			},
		}
	}

	// Verify new content IS present
	if hasNew && newContent != "" && !strings.Contains(actualStr, newContent) {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.95,
			Method:     ValidationMethodContentCheck,
			Error:      "new content not found after edit",
			Details: map[string]interface{}{
				"new_content_preview": truncateStr(newContent, 100),
			},
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodContentCheck,
		Details: map[string]interface{}{
			"old_removed": hasOld && oldContent != "",
			"new_present": hasNew && newContent != "",
		},
	}
}

// Name returns the validator name.
func (v *FileEditValidator) Name() string { return "file_edit_validator" }

// Priority returns the validator priority.
func (v *FileEditValidator) Priority() int { return 10 }

// FileDeleteValidator verifies that file delete operations actually removed the file.
type FileDeleteValidator struct{}

// NewFileDeleteValidator creates a new file delete validator.
func NewFileDeleteValidator() *FileDeleteValidator {
	return &FileDeleteValidator{}
}

// CanValidate returns true for file delete action types.
func (v *FileDeleteValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionDeleteFile
}

// Validate checks that the file no longer exists.
func (v *FileDeleteValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "action reported failure: " + result.Error,
		}
	}

	path := req.Target
	if path == "" {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "no target path specified in action request",
		}
	}

	// Check file does NOT exist
	_, err := os.Stat(path)
	if err == nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "file still exists after delete",
		}
	}

	if !os.IsNotExist(err) {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodExistence,
			Error:      "unexpected error checking file: " + err.Error(),
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodExistence,
	}
}

// Name returns the validator name.
func (v *FileDeleteValidator) Name() string { return "file_delete_validator" }

// Priority returns the validator priority.
func (v *FileDeleteValidator) Priority() int { return 10 }

// truncateStr shortens a string to maxLen, adding "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
