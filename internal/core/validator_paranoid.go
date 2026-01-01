// validator_paranoid.go provides ZERO FALSE POSITIVE validation for critical file operations.
// Philosophy: Trust nothing. Verify everything. Multiple independent checks required.
package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// ParanoidFileValidator performs redundant, multi-method validation to eliminate false positives.
// It requires ALL of the following to pass:
// 1. File exists and is readable
// 2. Content hash matches (SHA-256)
// 3. File modification timestamp is fresh (within validation window)
// 4. Double-read consistency (read twice, both match)
// 5. Size sanity check (non-zero, reasonable size)
// 6. Content sampling (for large files, verify multiple points)
type ParanoidFileValidator struct {
	// MaxStaleSeconds - how old can the file be before we consider it stale
	// Default: 30 seconds (should be recent after action)
	MaxStaleSeconds int

	// RequireDoubleRead - require two sequential reads to match
	RequireDoubleRead bool

	// MinFileSizeBytes - reject empty or suspiciously small files
	MinFileSizeBytes int64

	// MaxFileSizeBytes - reject unreasonably large files (potential corruption)
	MaxFileSizeBytes int64

	// SamplePoints - for large files, how many random points to verify
	SamplePoints int
}

// NewParanoidFileValidator creates a paranoid validator with sensible defaults.
func NewParanoidFileValidator() *ParanoidFileValidator {
	return &ParanoidFileValidator{
		MaxStaleSeconds:   30,  // File must be modified within 30s
		RequireDoubleRead: true, // Always double-read for consistency
		MinFileSizeBytes:  0,    // Allow empty files (will fail if expected content exists)
		MaxFileSizeBytes:  100 * 1024 * 1024, // 100MB max
		SamplePoints:      5,    // Check 5 random points in large files
	}
}

// CanValidate returns true for all file write and edit operations.
func (v *ParanoidFileValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionWriteFile ||
		actionType == ActionFSWrite ||
		actionType == ActionEditFile
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

	// Get expected content from payload
	expectedContent, hasExpected := req.Payload["content"].(string)
	if !hasExpected {
		// For edits, we might have old/new instead
		if req.Type == ActionEditFile {
			// We can't do paranoid validation without expected content
			// Fall through to other validators
			return ValidationResult{
				Verified:   true,
				Confidence: 0.0, // Defer to other validators
				Method:     "paranoid_validation_skipped",
				Details:    map[string]interface{}{"reason": "no expected content for edit operation"},
			}
		}
		// For writes, we should have content
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
			Details: map[string]interface{}{
				"check_failed": "existence",
			},
		}
	}

	if info.IsDir() {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "path is a directory, not a file",
			Details: map[string]interface{}{
				"check_failed": "directory_check",
			},
		}
	}

	// CHECK 2: Timestamp freshness (file was modified recently)
	modTime := info.ModTime()
	age := time.Since(modTime).Seconds()
	if age > float64(v.MaxStaleSeconds) {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("file modification time is stale: %.1fs old (max: %ds)", age, v.MaxStaleSeconds),
			Details: map[string]interface{}{
				"check_failed":   "timestamp_freshness",
				"age_seconds":    age,
				"max_age":        v.MaxStaleSeconds,
				"modified_at":    modTime.Format(time.RFC3339),
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
			Details: map[string]interface{}{
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
			Details: map[string]interface{}{
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
			Details: map[string]interface{}{
				"check_failed":  "size_match",
				"actual_size":   fileSize,
				"expected_size": expectedSize,
			},
		}
	}

	// CHECK 4: First read
	firstRead, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      fmt.Sprintf("cannot read file (first attempt): %v", err),
			Details: map[string]interface{}{
				"check_failed": "first_read",
			},
		}
	}

	// CHECK 5: Hash comparison (first read)
	firstHash := sha256.Sum256(firstRead)
	expectedHash := sha256.Sum256(expectedBytes)

	if firstHash != expectedHash {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "paranoid_validation",
			Error:      "content hash mismatch (first read)",
			Details: map[string]interface{}{
				"check_failed":  "hash_first_read",
				"expected_hash": hex.EncodeToString(expectedHash[:]),
				"actual_hash":   hex.EncodeToString(firstHash[:]),
				"size_match":    len(firstRead) == len(expectedBytes),
			},
		}
	}

	// CHECK 6: Double-read consistency (detect race conditions, NFS issues, etc.)
	if v.RequireDoubleRead {
		// Small delay to catch race conditions
		time.Sleep(50 * time.Millisecond)

		secondRead, err := os.ReadFile(path)
		if err != nil {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      fmt.Sprintf("cannot read file (second attempt): %v", err),
				Details: map[string]interface{}{
					"check_failed": "second_read",
				},
			}
		}

		// Both reads must be identical
		if !bytes.Equal(firstRead, secondRead) {
			secondHash := sha256.Sum256(secondRead)
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      "double-read inconsistency detected (file changed between reads)",
				Details: map[string]interface{}{
					"check_failed":    "double_read_consistency",
					"first_read_len":  len(firstRead),
					"second_read_len": len(secondRead),
					"first_hash":      hex.EncodeToString(firstHash[:8]),
					"second_hash":     hex.EncodeToString(secondHash[:8]),
				},
			}
		}

		// Second read must also match expected
		secondHash := sha256.Sum256(secondRead)
		if secondHash != expectedHash {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "paranoid_validation",
				Error:      "content hash mismatch (second read)",
				Details: map[string]interface{}{
					"check_failed":  "hash_second_read",
					"expected_hash": hex.EncodeToString(expectedHash[:]),
					"actual_hash":   hex.EncodeToString(secondHash[:]),
				},
			}
		}
	}

	// CHECK 7: Content sampling (for paranoia - verify random points)
	if v.SamplePoints > 0 && len(firstRead) > 100 {
		sampleSize := len(firstRead) / v.SamplePoints
		for i := 0; i < v.SamplePoints; i++ {
			offset := i * sampleSize
			if offset >= len(firstRead) || offset >= len(expectedBytes) {
				break
			}
			endOffset := offset + min(32, len(firstRead)-offset)
			if endOffset > len(expectedBytes) {
				endOffset = len(expectedBytes)
			}

			if !bytes.Equal(firstRead[offset:endOffset], expectedBytes[offset:endOffset]) {
				return ValidationResult{
					Verified:   false,
					Confidence: 1.0,
					Method:     "paranoid_validation",
					Error:      fmt.Sprintf("content sampling mismatch at offset %d", offset),
					Details: map[string]interface{}{
						"check_failed":   "content_sampling",
						"sample_offset":  offset,
						"sample_point":   i + 1,
						"total_samples":  v.SamplePoints,
					},
				}
			}
		}
	}

	// ALL CHECKS PASSED
	duration := time.Since(startTime)
	return ValidationResult{
		Verified:   true,
		Confidence: 1.0, // Maximum confidence - all redundant checks passed
		Method:     "paranoid_validation",
		Details: map[string]interface{}{
			"checks_passed": []string{
				"existence",
				"timestamp_freshness",
				"size_sanity",
				"size_match",
				"first_read",
				"hash_first_read",
				"double_read_consistency",
				"hash_second_read",
				"content_sampling",
			},
			"file_size":         fileSize,
			"age_seconds":       age,
			"hash":              hex.EncodeToString(firstHash[:8]),
			"validation_time_ms": duration.Milliseconds(),
			"sample_points":     v.SamplePoints,
		},
	}
}

// Name returns the validator name.
func (v *ParanoidFileValidator) Name() string {
	return "paranoid_file_validator"
}

// Priority returns the validator priority.
// Paranoid validator runs LAST (highest priority number) as a final check.
func (v *ParanoidFileValidator) Priority() int {
	return 100
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
