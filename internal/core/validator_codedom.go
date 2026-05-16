package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	filePath := v.extractFilePath(req)
	if filePath == "" {
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]interface{}{"reason": "cannot determine file path"},
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
				Details: map[string]interface{}{
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
					Details:    map[string]interface{}{"ref": ref},
				}
			}
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodCodeDOMRefresh,
		Details: map[string]interface{}{
			"file": filePath,
			"size": len(content),
		},
	}
}

// extractFilePath determines the file path from the action request.
func (v *CodeDOMValidator) extractFilePath(req ActionRequest) string {
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
			// Normalize whitespace for comparison
			normalizedNew := strings.TrimSpace(newContent)
			if !strings.Contains(string(content), normalizedNew) {
				return ValidationResult{
					Verified:   false,
					Confidence: 0.9,
					Method:     ValidationMethodContentCheck,
					Error:      "inserted content not found in file",
					Details:    map[string]interface{}{"content_preview": truncateStr(normalizedNew, 100)},
				}
			}
		}
	}

	// For delete_lines, count lines and verify reduction
	if req.Type == ActionDeleteLines {
		startLine, hasStart := req.Payload["start_line"].(int)
		endLine, hasEnd := req.Payload["end_line"].(int)

		if hasStart && hasEnd {
			expectedDeleted := endLine - startLine + 1
			lines := strings.Split(string(content), "\n")

			// We can't verify the exact deletion without knowing the previous line count
			// But we can verify the file has fewer lines than before (if we tracked that)
			// For now, just verify file is readable and has content
			if len(lines) == 0 {
				return ValidationResult{
					Verified:   false,
					Confidence: 0.8,
					Method:     ValidationMethodContentCheck,
					Error:      "file appears empty after line deletion",
				}
			}

			return ValidationResult{
				Verified:   true,
				Confidence: 0.8,
				Method:     ValidationMethodContentCheck,
				Details: map[string]interface{}{
					"expected_deleted": expectedDeleted,
					"current_lines":    len(lines),
				},
			}
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 0.85,
		Method:     ValidationMethodContentCheck,
		Details: map[string]interface{}{
			"file":       filePath,
			"line_count": strings.Count(string(content), "\n") + 1,
		},
	}
}

// Name returns the validator name.
func (v *LineEditValidator) Name() string { return "line_edit_validator" }

// Priority returns the validator priority.
func (v *LineEditValidator) Priority() int { return 15 }
