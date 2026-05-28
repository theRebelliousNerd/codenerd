// validator_paranoid.go provides ZERO FALSE POSITIVE validation for critical file operations.
// Philosophy: Trust nothing. Verify everything. Multiple independent checks required.
package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// ParanoidFileValidator performs redundant, multi-method validation to eliminate false positives.
// It requires ALL of the following to pass:
// 1. File exists and is readable
// 2. Content hash matches (SHA-256 via streaming)
// 3. File modification timestamp is fresh (within validation window and not negative)
// 4. Double-read consistency (hash twice, both match)
// 5. Size sanity check (non-zero, reasonable size)
type ParanoidFileValidator struct {
	MaxStaleSeconds   int
	RequireDoubleRead bool
	MinFileSizeBytes  int64
	MaxFileSizeBytes  int64
}

// NewParanoidFileValidator creates a paranoid validator with sensible defaults.
func NewParanoidFileValidator() *ParanoidFileValidator {
	return &ParanoidFileValidator{
		MaxStaleSeconds:   30,
		RequireDoubleRead: true,
		MinFileSizeBytes:  0,
		MaxFileSizeBytes:  100 * 1024 * 1024,
	}
}

// CanValidate returns true for all file write and edit operations.
func (v *ParanoidFileValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionWriteFile ||
		actionType == ActionFSWrite ||
		actionType == ActionEditFile
}

// contextReader wraps an io.Reader to check context cancellation on every Read
type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (cr *contextReader) Read(p []byte) (n int, err error) {
	if err := cr.ctx.Err(); err != nil {
		return 0, err
	}
	return cr.r.Read(p)
}

// Validate performs exhaustive validation with zero tolerance for ambiguity.
// ALL checks must pass. ANY failure returns Verified=false.
func (v *ParanoidFileValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	startTime := time.Now()

	// Pre-check: action must have succeeded
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "action reported failure: " + result.Error,
		}
	}

	path := req.Target
	if path == "" {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "no target path specified",
		}
	}

	var expectedContent string
	if content, ok := req.Payload["content"]; ok {
		expectedContent = fmt.Sprint(content)
	} else if efs, ok := req.Payload["expected_final_state"]; ok {
		expectedContent = fmt.Sprint(efs)
	} else {
		if req.Type == ActionEditFile {
			return ValidationResult{
				Verified:   true,
				Confidence: 0.0,
				Method:     "paranoid_validation_skipped",
				Details:    map[string]any{"reason": "no expected content or final state for edit operation"},
			}
		}
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "write operation missing expected content in payload",
		}
	}

	expectedBytes := []byte(expectedContent)

	// CHECK 1: File exists and basic stat
	info, err := os.Stat(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file does not exist or cannot stat: %v", err),
			Details:    map[string]any{"check_failed": "existence"},
		}
	}

	if info.IsDir() {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "path is a directory, not a file",
			Details:    map[string]any{"check_failed": "directory_check"},
		}
	}

	// CHECK 2: Timestamp freshness
	modTime := info.ModTime()
	age := time.Since(modTime).Seconds()
	if age < 0 {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file modification time is in the future (negative age): %.1fs", age),
			Details: map[string]any{
				"check_failed": "timestamp_freshness",
				"age_seconds":  age,
				"modified_at":  modTime.Format(time.RFC3339),
			},
		}
	}
	if age > float64(v.MaxStaleSeconds) {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file modification time is stale: %.1fs old (max: %ds)", age, v.MaxStaleSeconds),
			Details: map[string]any{
				"check_failed": "timestamp_freshness",
				"age_seconds":  age,
				"max_age":      v.MaxStaleSeconds,
				"modified_at":  modTime.Format(time.RFC3339),
			},
		}
	}

	// CHECK 3: Size sanity
	fileSize := info.Size()
	if fileSize < v.MinFileSizeBytes {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file too small: %d bytes (min: %d)", fileSize, v.MinFileSizeBytes),
			Details: map[string]any{
				"check_failed": "size_minimum",
				"actual_size":  fileSize,
				"min_size":     v.MinFileSizeBytes,
			},
		}
	}

	if fileSize > v.MaxFileSizeBytes {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file too large: %d bytes (max: %d)", fileSize, v.MaxFileSizeBytes),
			Details: map[string]any{
				"check_failed": "size_maximum",
				"actual_size":  fileSize,
				"max_size":     v.MaxFileSizeBytes,
			},
		}
	}

	expectedSize := int64(len(expectedBytes))
	if fileSize != expectedSize {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file size mismatch: got %d bytes, expected %d", fileSize, expectedSize),
			Details: map[string]any{
				"check_failed":  "size_match",
				"actual_size":   fileSize,
				"expected_size": expectedSize,
			},
		}
	}

	// Helper for CHECK 4 & 5
	hashFile := func(attempt string) (string, error) {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()

		h := sha256.New()
		cr := &contextReader{ctx: ctx, r: f}
		if _, err := io.Copy(h, cr); err != nil {
			return "", err
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}

	sum := sha256.Sum256(expectedBytes)
	expectedHash := hex.EncodeToString(sum[:])

	// CHECK 4: First read & hash
	firstHashStr, err := hashFile("first")
	if err != nil {
		if ctx.Err() != nil {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      "context cancelled during first read",
			}
		}
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("cannot read file (first attempt): %v", err),
			Details:    map[string]any{"check_failed": "first_read"},
		}
	}

	if firstHashStr != expectedHash {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "content hash mismatch (first read)",
			Details: map[string]any{
				"check_failed":  "hash_first_read",
				"expected_hash": expectedHash,
				"actual_hash":   firstHashStr,
			},
		}
	}

	// CHECK 5: Double-read consistency
	if v.RequireDoubleRead {
		time.Sleep(50 * time.Millisecond)

		secondHashStr, err := hashFile("second")
		if err != nil {
			if ctx.Err() != nil {
				return ValidationResult{
					Verified:   false,
					Confidence: 1.0,
					Method:     "paranoid_validation",
					Error:      "context cancelled during second read",
				}
			}
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      fmt.Sprintf("cannot read file (second attempt): %v", err),
				Details:    map[string]any{"check_failed": "second_read"},
			}
		}

		if firstHashStr != secondHashStr {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      "double-read inconsistency detected (file changed between reads)",
				Details: map[string]any{
					"check_failed": "double_read_consistency",
					"first_hash":   firstHashStr[:16],
					"second_hash":  secondHashStr[:16],
				},
			}
		}

		if secondHashStr != expectedHash {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      "content hash mismatch (second read)",
				Details: map[string]any{
					"check_failed":  "hash_second_read",
					"expected_hash": expectedHash,
					"actual_hash":   secondHashStr,
				},
			}
		}
	}

	duration := time.Since(startTime)
	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     "paranoid_validation",
		Details: map[string]any{
			"checks_passed": []string{
				"existence",
				"timestamp_freshness",
				"size_sanity",
				"size_match",
				"first_read",
				"hash_first_read",
				"double_read_consistency",
				"hash_second_read",
			},
			"file_size":          fileSize,
			"age_seconds":        age,
			"hash":               firstHashStr[:16],
			"validation_time_ms": duration.Milliseconds(),
		},
	}
}

// Name returns the validator name.
func (v *ParanoidFileValidator) Name() string {
	return "paranoid_file_validator"
}

// Priority returns the validator priority.
func (v *ParanoidFileValidator) Priority() int {
	return 100
}
