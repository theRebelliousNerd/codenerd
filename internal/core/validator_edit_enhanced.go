// validator_edit_enhanced.go provides diff-based validation for file edit operations.
// Ensures exact changes were applied with zero tolerance for partial edits.
package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

// EnhancedEditValidator performs surgical validation of file edits using diff-based verification.
// It verifies:
// 1. Old content was completely removed (no partial remains)
// 2. New content was fully inserted (exact match)
// 3. Surrounding context is preserved (unchanged lines remain)
// 4. File structure is valid (no corruption)
// 5. Edit boundaries are clean (no merged/split issues)
type EnhancedEditValidator struct {
	// VerifyContext - how many lines before/after to check for corruption
	VerifyContext int

	// RequireExactMatch - new content must match byte-for-byte
	RequireExactMatch bool

	// MaxStaleSeconds - file must be recently modified
	MaxStaleSeconds int
}

// NewEnhancedEditValidator creates an enhanced edit validator.
func NewEnhancedEditValidator() *EnhancedEditValidator {
	return &EnhancedEditValidator{
		VerifyContext:     3,    // Check 3 lines before/after for corruption
		RequireExactMatch: true, // Exact byte match required
		MaxStaleSeconds:   30,   // Must be modified within 30s
	}
}

// CanValidate returns true for file edit operations.
func (v *EnhancedEditValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionEditFile
}

// Validate performs comprehensive edit verification.
func (v *EnhancedEditValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      "action reported failure: " + result.Error,
		}
	}

	path := req.Target
	if path == "" {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      "no target path specified",
		}
	}

	// Get old and new content from payload
	oldContent, hasOld := req.Payload["old"].(string)
	newContent, hasNew := req.Payload["new"].(string)

	if !hasOld || !hasNew {
		// Cannot perform enhanced validation without both old and new
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0, // Defer to other validators
			Method:     "enhanced_edit_validation_skipped",
			Details:    map[string]any{"reason": "missing old or new content"},
		}
	}

	// CHECK 1: File exists and is fresh
	info, err := os.Stat(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      fmt.Sprintf("file does not exist: %v", err),
		}
	}

	// CHECK 2: Timestamp freshness
	age := time.Since(info.ModTime()).Seconds()
	if age > float64(v.MaxStaleSeconds) {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      fmt.Sprintf("file not recently modified: %.1fs old", age),
			Details: map[string]any{
				"age_seconds": age,
				"max_age":     v.MaxStaleSeconds,
			},
		}
	}

	// CHECK 3: Read actual content
	actualBytes, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      fmt.Sprintf("cannot read file: %v", err),
		}
	}

	actualContent := string(actualBytes)

	// Compare in LF space. File writers preserve the destination newline
	// convention, so a CRLF working copy holds CRLF bytes while the payload
	// carries LF text; raw Contains failed every CRLF edit at CHECK 5 and
	// passed CHECK 4 by luck. FileEditValidator already normalizes the
	// same way.
	normActual := normalizeLineEndings(actualContent)
	normOld := normalizeLineEndings(oldContent)
	normNew := normalizeLineEndings(newContent)
	// An edit that expands text (old "User" -> new "UserService")
	// legitimately leaves old behind inside new; absence checks must not
	// fire on that shape.
	oldInNew := normOld != "" && strings.Contains(normNew, normOld)

	// CHECK 4: Old content must be COMPLETELY GONE
	if normOld != "" && !oldInNew {
		// Check for any substring of old content (partial edit detection)
		if strings.Contains(normActual, normOld) {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "enhanced_edit_validation",
				Error:      "old content still present after edit (edit not applied or only partially applied)",
				Details: map[string]any{
					"old_content_preview": truncateStr(oldContent, 100),
					"check_failed":        "old_content_removal",
				},
			}
		}

		// Also check for fragmented old content (corruption detection)
		// Split old content into chunks and verify none are present
		oldLines := strings.Split(normOld, "\n")
		if len(oldLines) > 2 {
			// Check significant chunks (> 20 chars) aren't lingering
			for _, line := range oldLines {
				line = strings.TrimSpace(line)
				if len(line) > 20 && strings.Contains(normActual, line) {
					return ValidationResult{
						Verified:   false,
						Confidence: 0.95,
						Method:     "enhanced_edit_validation",
						Error:      "fragment of old content detected (potential partial edit)",
						Details: map[string]any{
							"fragment_preview": truncateStr(line, 80),
							"check_failed":     "old_content_fragment",
						},
					}
				}
			}
		}
	}

	// CHECK 5: New content must be FULLY PRESENT
	if normNew != "" {
		if !strings.Contains(normActual, normNew) {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     "enhanced_edit_validation",
				Error:      "new content not found in file (edit not applied)",
				Details: map[string]any{
					"new_content_preview": truncateStr(newContent, 100),
					"check_failed":        "new_content_presence",
				},
			}
		}

		// CHECK 6: Exact match verification (byte-for-byte in LF space)
		if v.RequireExactMatch {
			newIndex := strings.Index(normActual, normNew)
			if newIndex == -1 {
				// Should never happen (already checked Contains above)
				return ValidationResult{
					Verified:   false,
					Confidence: 1.0,
					Method:     "enhanced_edit_validation",
					Error:      "new content not found (exact match failed)",
				}
			}

			// Verify the exact bytes match (no encoding issues, whitespace corruption, etc.)
			extractedNew := normActual[newIndex : newIndex+len(normNew)]
			if extractedNew != normNew {
				expectedHash := sha256.Sum256([]byte(normNew))
				actualHash := sha256.Sum256([]byte(extractedNew))
				return ValidationResult{
					Verified:   false,
					Confidence: 1.0,
					Method:     "enhanced_edit_validation",
					Error:      "new content found but does not match exactly (possible encoding/whitespace corruption)",
					Details: map[string]any{
						"expected_hash": hex.EncodeToString(expectedHash[:8]),
						"actual_hash":   hex.EncodeToString(actualHash[:8]),
						"check_failed":  "new_content_exact_match",
					},
				}
			}
		}
	}

	// CHECK 7: Context preservation (verify surrounding lines weren't corrupted)
	if beforeContext, ok := req.Payload["context_before"].(string); ok && beforeContext != "" {
		if !strings.Contains(normActual, normalizeLineEndings(beforeContext)) {
			return ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     "enhanced_edit_validation",
				Error:      "context before edit was corrupted or removed",
				Details: map[string]any{
					"context_preview": truncateStr(beforeContext, 80),
					"check_failed":    "context_before_preservation",
				},
			}
		}
	}

	if afterContext, ok := req.Payload["context_after"].(string); ok && afterContext != "" {
		if !strings.Contains(normActual, normalizeLineEndings(afterContext)) {
			return ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     "enhanced_edit_validation",
				Error:      "context after edit was corrupted or removed",
				Details: map[string]any{
					"context_preview": truncateStr(afterContext, 80),
					"check_failed":    "context_after_preservation",
				},
			}
		}
	}

	// CHECK 8: File structure integrity (no null bytes, control characters)
	if bytes.Contains(actualBytes, []byte{0}) {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      "file contains null bytes (corruption detected)",
			Details: map[string]any{
				"check_failed": "null_byte_detection",
			},
		}
	}

	// CHECK 9: Edit was singular (verify old content appears exactly 0 times)
	oldCount := strings.Count(normActual, normOld)
	if normOld != "" && oldCount > 0 && !oldInNew {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      fmt.Sprintf("old content appears %d times (should be 0)", oldCount),
			Details: map[string]any{
				"old_count":    oldCount,
				"check_failed": "singularity_check",
			},
		}
	}

	// CHECK 10: New content appears at least once
	newCount := strings.Count(normActual, normNew)
	if newCount == 0 {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     "enhanced_edit_validation",
			Error:      "new content not found in file",
		}
	}

	// Warn if new content appears multiple times (might be expected, but flag it)
	multipleOccurrences := newCount > 1

	// ALL CHECKS PASSED
	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     "enhanced_edit_validation",
		Details: map[string]any{
			"checks_passed": []string{
				"file_exists",
				"timestamp_freshness",
				"old_content_removal",
				"old_content_fragment_check",
				"new_content_presence",
				"new_content_exact_match",
				"context_preservation",
				"structure_integrity",
				"singularity_check",
			},
			"old_removed":          oldContent != "",
			"new_present":          newContent != "",
			"new_occurrence_count": newCount,
			"multiple_occurrences": multipleOccurrences,
			"age_seconds":          age,
			"file_size":            len(actualBytes),
		},
	}
}

// Name returns the validator name.
func (v *EnhancedEditValidator) Name() string {
	return "enhanced_edit_validator"
}

// Priority returns the validator priority.
// Runs after basic edit validator (priority 10) but before paranoid validator (100).
func (v *EnhancedEditValidator) Priority() int {
	return 15
}
