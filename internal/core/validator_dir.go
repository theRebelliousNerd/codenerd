// validator_dir.go provides post-action validators for directory operations.
package core

import (
	"context"
	"os"
	"path/filepath"
)

// DirectoryValidator verifies that directory operations succeeded.
// This includes parent directory verification for file writes.
type DirectoryValidator struct{}

// NewDirectoryValidator creates a new directory validator.
func NewDirectoryValidator() *DirectoryValidator {
	return &DirectoryValidator{}
}

// CanValidate returns true for directory-related action types.
func (v *DirectoryValidator) CanValidate(actionType ActionType) bool {
	// Validate directory existence for file writes
	return actionType == ActionWriteFile ||
		actionType == ActionFSWrite
}

// Validate checks that the directory exists and is accessible.
func (v *DirectoryValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
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
			Error:      "no target path specified",
		}
	}

	// For file writes, check the parent directory exists
	dirPath := filepath.Dir(path)

	info, err := os.Stat(dirPath)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "directory does not exist: " + err.Error(),
			Details:    map[string]interface{}{"path": dirPath},
		}
	}

	if !info.IsDir() {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodExistence,
			Error:      "path is not a directory",
			Details:    map[string]interface{}{"path": dirPath},
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodExistence,
		Details: map[string]interface{}{
			"path":    dirPath,
			"mode":    info.Mode().String(),
			"modTime": info.ModTime().Unix(),
		},
	}
}

// Name returns the validator name.
func (v *DirectoryValidator) Name() string { return "directory_validator" }

// Priority returns the validator priority.
// Run before file validators to catch directory issues first.
func (v *DirectoryValidator) Priority() int { return 5 }
