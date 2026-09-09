package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// CodeDOMValidator verifies that semantic code edits preserve code integrity.
// It re-parses modified files and verifies element references are still valid.
type CodeDOMValidator struct {
	// preEditHashes stores file hashes before edit for change detection
	preEditHashes map[string]string
}

// NewCodeDOMValidator creates a new CodeDOM validator.
func NewCodeDOMValidator() *CodeDOMValidator {
	return &CodeDOMValidator{
		preEditHashes: make(map[string]string),
	}
}

// CapturePreEditState stores the file hash before an edit.
// Call this before executing the edit to enable change detection.
func (v *CodeDOMValidator) CapturePreEditState(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		// File might not exist yet - that's OK for new files
		v.preEditHashes[path] = ""
		return nil
	}
	hash := sha256.Sum256(content)
	v.preEditHashes[path] = hex.EncodeToString(hash[:])
	return nil
}

// CanValidate returns true for CodeDOM action types.
func (v *CodeDOMValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionEditElement ||
		actionType == ActionEditLines ||
		actionType == ActionInsertLines ||
		actionType == ActionDeleteLines
}

// Validate re-parses the file and checks for corruption.
func (v *CodeDOMValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodCodeDOMRefresh,
			Error:      "action reported failure: " + result.Error,
		}
	}

	// For CodeDOM edits, target can be either a file path or a Ref
	// We need to determine the file path
	filePath := v.extractFilePath(req, result)
	if filePath == "" {
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]any{"reason": "cannot determine file path"},
		}
	}

	// 1. Check file exists and is readable
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodCodeDOMRefresh,
			Error:      "cannot read file after edit: " + err.Error(),
		}
	}

	// 2. Verify file content actually changed (unless it's a delete)
	if req.Type != ActionDeleteLines {
		currentHash := sha256.Sum256(content)
		currentHashStr := hex.EncodeToString(currentHash[:])

		if preHash, ok := v.preEditHashes[filePath]; ok && preHash != "" {
			if preHash == currentHashStr {
				return ValidationResult{
					Verified:   false,
					Confidence: 0.9,
					Method:     ValidationMethodCodeDOMRefresh,
					Error:      "file hash unchanged after edit - edit may not have been applied",
				}
			}
		}
	}

	// 3. For Go files, verify syntax is still valid
	if strings.HasSuffix(filePath, ".go") {
		fset := token.NewFileSet()
		_, err := parser.ParseFile(fset, filePath, content, parser.AllErrors)
		if err != nil {
			return ValidationResult{
				Verified:   false,
				Confidence: 1.0,
				Method:     ValidationMethodCodeDOMRefresh,
				Error:      "Go syntax error after CodeDOM edit",
				Details: map[string]any{
					"parse_error": err.Error(),
				},
			}
		}
	}

	// 4. For element-based edits, verify the target element still exists
	if req.Type == ActionEditElement {
		if ref, ok := req.Payload["ref"].(string); ok {
			if !v.verifyElementExists(content, ref) {
				return ValidationResult{
					Verified:   false,
					Confidence: 0.85,
					Method:     ValidationMethodCodeDOMRefresh,
					Error:      "target element no longer exists after edit",
					Details:    map[string]any{"ref": ref},
				}
			}
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodCodeDOMRefresh,
		Details: map[string]any{
			"file": filePath,
			"size": len(content),
		},
	}
}

// extractFilePath determines the file path from the action request.
func (v *CodeDOMValidator) extractFilePath(req ActionRequest, result ActionResult) string {
	// Semantic element references do not necessarily encode a file path. The
	// successful handler is authoritative for the concrete file it edited.
	if path, ok := result.Metadata["file"].(string); ok && path != "" {
		return path
	}

	// Target might be a direct file path
	if req.Target != "" && !strings.Contains(req.Target, ":") {
		return req.Target
	}

	// Target might be a Ref like "go:internal/foo.go:FuncName"
	if strings.Contains(req.Target, ":") {
		parts := strings.SplitN(req.Target, ":", 3)
		if len(parts) >= 2 {
			return parts[1]
		}
	}

	// Check payload for file path
	if path, ok := req.Payload["file"].(string); ok {
		return path
	}
	if path, ok := req.Payload["path"].(string); ok {
		return path
	}

	return ""
}

// verifyElementExists does a basic check that an element still exists.
// This is a simplified check - full verification would use the actual CodeDOM.
func (v *CodeDOMValidator) verifyElementExists(content []byte, ref string) bool {
	// Extract element name from ref
	// Ref format: "lang:path:Element.Name" or "lang:path:Name"
	parts := strings.Split(ref, ":")
	if len(parts) < 3 {
		return true // Can't verify, assume OK
	}

	elementName := parts[len(parts)-1]
	// Handle nested names like "User.Login"
	if strings.Contains(elementName, ".") {
		nameParts := strings.Split(elementName, ".")
		elementName = nameParts[len(nameParts)-1]
	}

	// Simple check: element name should appear in content
	contentStr := string(content)
	return strings.Contains(contentStr, elementName)
}

// Name returns the validator name.
func (v *CodeDOMValidator) Name() string { return "codedom_validator" }

// Priority returns the validator priority.
// Run after syntax validators.
func (v *CodeDOMValidator) Priority() int { return 25 }

// LineEditValidator specifically validates line-based edits.
type LineEditValidator struct{}

// NewLineEditValidator creates a validator for line edits.
func NewLineEditValidator() *LineEditValidator {
	return &LineEditValidator{}
}

// CanValidate returns true for line edit actions.
func (v *LineEditValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionEditLines ||
		actionType == ActionInsertLines ||
		actionType == ActionDeleteLines
}

// Validate checks that line edits were applied correctly.
func (v *LineEditValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if !result.Success {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodContentCheck,
			Error:      "action reported failure: " + result.Error,
		}
	}

	filePath := req.Target
	if filePath == "" {
		if path, ok := req.Payload["file"].(string); ok {
			filePath = path
		}
	}

	if filePath == "" {
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
		}
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodContentCheck,
			Error:      "cannot read file: " + err.Error(),
		}
	}

	// For insert_lines, verify the new content is present
	if req.Type == ActionInsertLines {
		if newContent, ok := req.Payload["content"].(string); ok && newContent != "" {
			// Compare with line endings normalized on both sides. Working
			// copies on Windows are CRLF while the model inserts LF text; a raw
			// Contains failed every multi-line insert on this repo, the write
			// was discounted, the turn read as hollow and the campaign
			// fallback overwrote the coder's correct work (2026-09-04).
			normalizedNew := strings.TrimSpace(normalizeLineEndings(newContent))
			if !strings.Contains(normalizeLineEndings(string(content)), normalizedNew) {
				return ValidationResult{
					Verified:   false,
					Confidence: 0.9,
					Method:     ValidationMethodContentCheck,
					Error:      "inserted content not found in file",
					Details:    map[string]any{"content_preview": truncateStr(normalizedNew, 100)},
				}
			}
		}
	}

	// For delete_lines, verify the deletion against what was asked for.
	if req.Type == ActionDeleteLines {
		return validateDeleteLines(req, result, content)
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 0.85,
		Method:     ValidationMethodContentCheck,
		Details: map[string]any{
			"file":       filePath,
			"line_count": strings.Count(string(content), "\n") + 1,
		},
	}
}

// validateDeleteLines verifies a completed delete_lines against the range that
// was requested.
//
// This branch was both unreachable and vacuous. Unreachable: it read the range
// with a bare `.(int)` assertion, but every payload that reaches VirtualStore
// from the kernel or a tool call has been through JSON, so the numbers are
// float64 — handleDeleteLines itself reads them as float64 — and hasStart/hasEnd
// were false on every production delete, which fell through to the generic
// Verified: true, Confidence: 0.85 tail. Vacuous: on the Go-caller path that did
// pass ints, the only check was `len(strings.Split(content, "\n")) == 0`, which
// is impossible for any string, so it returned Verified: true, Confidence: 0.8
// for every delete — including one that removed nothing at all.
//
// The previous line count is not needed. FileEditor.DeleteLines clamps end_line
// to the file length and reports what it actually removed as
// metadata["lines_deleted"], and the untouched prefix pins the floor: after
// deleting [start,end] the file must still hold at least start-1 lines, and when
// fewer lines were removed than asked (the range ran past EOF) it must hold
// EXACTLY start-1. Confidence now says which of those was checked rather than
// asserting 0.8 for a file-is-not-empty test.
func validateDeleteLines(req ActionRequest, result ActionResult, content []byte) ValidationResult {
	startLine, hasStart := payloadInt(req.Payload["start_line"])
	endLine, hasEnd := payloadInt(req.Payload["end_line"])

	remaining := countTextLines(string(content))

	if !hasStart || !hasEnd || startLine < 1 || endLine < startLine {
		// Nothing to check the file against: the handler rejects such a
		// request, so reaching here means the range never made it into the
		// payload. Report what was actually verified — the file is readable —
		// and nothing more.
		return ValidationResult{
			Verified:   true,
			Confidence: 0.3,
			Method:     ValidationMethodContentCheck,
			Details: map[string]any{
				"reason":          "delete range absent or malformed in payload; only readability checked",
				"current_lines":   remaining,
				"start_line_type": fmt.Sprintf("%T", req.Payload["start_line"]),
			},
		}
	}

	expectedDeleted := endLine - startLine + 1
	actualDeleted, hasCount := payloadInt(result.Metadata["lines_deleted"])

	details := map[string]any{
		"expected_deleted": expectedDeleted,
		"current_lines":    remaining,
		"start_line":       startLine,
		"end_line":         endLine,
	}
	if hasCount {
		details["actual_deleted"] = actualDeleted
	}

	// The reported count is checked before the file, because a range that
	// starts past EOF deletes nothing and would otherwise trip the
	// untouched-prefix floor with a misleading over-deletion message.
	if hasCount {
		if actualDeleted <= 0 {
			return ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodContentCheck,
				Error: fmt.Sprintf("delete_lines removed nothing: requested %d-%d (%d lines) but the handler reported 0 deleted",
					startLine, endLine, expectedDeleted),
				Details: details,
			}
		}
		if actualDeleted > expectedDeleted {
			return ValidationResult{
				Verified:   false,
				Confidence: 0.95,
				Method:     ValidationMethodContentCheck,
				Error: fmt.Sprintf("delete_lines removed %d lines for a %d-line range (%d-%d)",
					actualDeleted, expectedDeleted, startLine, endLine),
				Details: details,
			}
		}
	}

	// The prefix before start_line is untouched by definition, so the file
	// cannot have come out shorter than it.
	if remaining < startLine-1 {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.95,
			Method:     ValidationMethodContentCheck,
			Error: fmt.Sprintf("delete removed more than the requested range: %d lines remain but lines 1-%d were not in range %d-%d",
				remaining, startLine-1, startLine, endLine),
			Details: details,
		}
	}

	if !hasCount {
		details["reason"] = "handler reported no lines_deleted; only the untouched-prefix floor was checked"
		return ValidationResult{
			Verified:   true,
			Confidence: 0.5,
			Method:     ValidationMethodContentCheck,
			Details:    details,
		}
	}

	if actualDeleted < expectedDeleted {
		// The range ran off the end of the file, so the delete truncated at
		// start_line. Anything else means the count and the file disagree.
		if remaining != startLine-1 {
			return ValidationResult{
				Verified:   false,
				Confidence: 0.9,
				Method:     ValidationMethodContentCheck,
				Error: fmt.Sprintf("delete_lines reported %d of %d requested lines removed, which only happens at EOF, but %d lines remain instead of %d",
					actualDeleted, expectedDeleted, remaining, startLine-1),
				Details: details,
			}
		}
		details["clamped_at_eof"] = true
		return ValidationResult{
			Verified:   true,
			Confidence: 0.9,
			Method:     ValidationMethodContentCheck,
			Details:    details,
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 0.9,
		Method:     ValidationMethodContentCheck,
		Details:    details,
	}
}

// countTextLines counts lines the way FileEditor does: bufio.Scanner semantics,
// so a trailing newline does not invent an extra empty line. Line endings are
// normalized first because a CRLF working copy must not count differently.
func countTextLines(content string) int {
	content = normalizeLineEndings(content)
	if content == "" {
		return 0
	}
	n := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		n++
	}
	return n
}

// normalizeLineEndings folds CRLF and lone CR to LF so content checks compare
// text, not the platform's newline convention.
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// Name returns the validator name.
func (v *LineEditValidator) Name() string { return "line_edit_validator" }

// Priority returns the validator priority.
func (v *LineEditValidator) Priority() int { return 15 }
