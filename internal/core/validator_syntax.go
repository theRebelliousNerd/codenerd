// validator_syntax.go provides syntax validators for code files.
package core

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// SyntaxValidator verifies that code files have valid syntax after edits.
// Supports Go, JSON, YAML, and can be extended for other languages via Tree-sitter.
type SyntaxValidator struct {
	// parsers maps file extensions to parser functions
	parsers map[string]func([]byte) error
}

// NewSyntaxValidator creates a new syntax validator with built-in parsers.
func NewSyntaxValidator() *SyntaxValidator {
	v := &SyntaxValidator{
		parsers: make(map[string]func([]byte) error),
	}

	// Register built-in parsers
	v.parsers[".go"] = validateGoSyntax
	v.parsers[".json"] = validateJSONSyntax
	v.parsers[".yaml"] = validateYAMLSyntax
	v.parsers[".yml"] = validateYAMLSyntax
	v.parsers[".toml"] = validateTOMLSyntax

	return v
}

// RegisterParser adds a custom parser for a file extension.
func (v *SyntaxValidator) RegisterParser(ext string, parserFunc func([]byte) error) {
	v.parsers[ext] = parserFunc
}

// CanValidate returns true for actions that modify code files.
func (v *SyntaxValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionWriteFile ||
		actionType == ActionFSWrite ||
		actionType == ActionEditFile ||
		actionType == ActionEditElement ||
		actionType == ActionEditLines ||
		actionType == ActionInsertLines
}

// Validate parses the file to check for syntax errors.
func (v *SyntaxValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if !result.Success {
		return ValidationResult{
			Verified:   true, // Don't double-report failures
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]interface{}{"reason": "action already failed"},
		}
	}

	path := req.Target
	if path == "" {
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]interface{}{"reason": "no target path"},
		}
	}

	ext := strings.ToLower(filepath.Ext(path))
	parserFunc, ok := v.parsers[ext]
	if !ok {
		// No parser for this file type - skip validation
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]interface{}{"reason": "no parser for extension: " + ext},
		}
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodSyntax,
			Error:      "cannot read file for syntax check: " + err.Error(),
		}
	}

	// Parse the file
	if err := parserFunc(content); err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 1.0,
			Method:     ValidationMethodSyntax,
			Error:      "syntax validation failed",
			Details: map[string]interface{}{
				"parse_error": err.Error(),
				"extension":   ext,
			},
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 1.0,
		Method:     ValidationMethodSyntax,
		Details: map[string]interface{}{
			"extension": ext,
			"size":      len(content),
		},
	}
}

// Name returns the validator name.
func (v *SyntaxValidator) Name() string { return "syntax_validator" }

// Priority returns the validator priority.
// Run after file validators but before CodeDOM validators.
func (v *SyntaxValidator) Priority() int { return 20 }

// validateGoSyntax parses Go source code.
func validateGoSyntax(content []byte) error {
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, "check.go", content, parser.AllErrors)
	return err
}

// validateJSONSyntax parses JSON content.
func validateJSONSyntax(content []byte) error {
	var v interface{}
	return json.Unmarshal(content, &v)
}

// validateYAMLSyntax parses YAML content.
func validateYAMLSyntax(content []byte) error {
	var v interface{}
	return yaml.Unmarshal(content, &v)
}

// validateTOMLSyntax provides basic TOML syntax checking.
func validateTOMLSyntax(content []byte) error {
	lines := strings.Split(string(content), "\n")
	inArray := false
	inMultilineString := false

	for i, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if inMultilineString {
			if strings.Contains(line, `"""`) || strings.Contains(line, `'''`) {
				inMultilineString = false
			}
			continue
		}

		if strings.Contains(line, `"""`) || strings.Contains(line, `'''`) {
			count := strings.Count(line, `"""`) + strings.Count(line, `'''`)
			if count%2 == 1 {
				inMultilineString = true
			}
			continue
		}

		if strings.HasPrefix(line, "[[") {
			if !strings.HasSuffix(line, "]]") {
				return &tomlSyntaxError{line: i + 1, msg: "unclosed array table"}
			}
			inArray = true
			continue
		}

		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return &tomlSyntaxError{line: i + 1, msg: "unclosed table header"}
			}
			inArray = false
			continue
		}

		if !strings.Contains(line, "=") && !inArray {
			return &tomlSyntaxError{line: i + 1, msg: "invalid line: missing '='"}
		}
	}

	return nil
}

type tomlSyntaxError struct {
	line int
	msg  string
}

func (e *tomlSyntaxError) Error() string {
	return "TOML syntax error at line " + itoaValidator(e.line) + ": " + e.msg
}

// MangleSyntaxValidator validates Mangle (.mg) files.
type MangleSyntaxValidator struct{}

// NewMangleSyntaxValidator creates a validator for Mangle files.
func NewMangleSyntaxValidator() *MangleSyntaxValidator {
	return &MangleSyntaxValidator{}
}

// CanValidate returns true for Mangle file operations.
func (v *MangleSyntaxValidator) CanValidate(actionType ActionType) bool {
	return actionType == ActionWriteFile ||
		actionType == ActionFSWrite ||
		actionType == ActionEditFile
}

// Validate checks Mangle syntax by looking for common errors.
func (v *MangleSyntaxValidator) Validate(ctx context.Context, req ActionRequest, result ActionResult) ValidationResult {
	if !result.Success {
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
		}
	}

	path := req.Target
	if !strings.HasSuffix(path, ".mg") {
		return ValidationResult{
			Verified:   true,
			Confidence: 0.0,
			Method:     ValidationMethodSkipped,
			Details:    map[string]interface{}{"reason": "not a Mangle file"},
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodSyntax,
			Error:      "cannot read file: " + err.Error(),
		}
	}

	issues := validateMangleSyntax(string(content))
	if len(issues) > 0 {
		return ValidationResult{
			Verified:   false,
			Confidence: 0.9,
			Method:     ValidationMethodSyntax,
			Error:      "Mangle syntax issues detected",
			Details:    map[string]interface{}{"issues": issues},
		}
	}

	return ValidationResult{
		Verified:   true,
		Confidence: 0.9,
		Method:     ValidationMethodSyntax,
	}
}

// Name returns the validator name.
func (v *MangleSyntaxValidator) Name() string { return "mangle_syntax_validator" }

// Priority returns the validator priority.
func (v *MangleSyntaxValidator) Priority() int { return 20 }

// validateMangleSyntax checks for common Mangle syntax errors.
func validateMangleSyntax(content string) []string {
	var issues []string
	lines := strings.Split(content, "\n")

	inComment := false
	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "/*") {
			inComment = true
		}
		if strings.HasSuffix(trimmed, "*/") {
			inComment = false
			continue
		}
		if inComment {
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		if strings.HasPrefix(trimmed, "Decl ") && !strings.HasSuffix(trimmed, ".") {
			issues = append(issues, "line "+itoaValidator(lineNum)+": Decl missing period")
		}

		if strings.Contains(trimmed, " = sum(") || strings.Contains(trimmed, " = count(") {
			issues = append(issues, "line "+itoaValidator(lineNum)+": SQL-style aggregation detected (use |> do fn:group_by)")
		}
	}

	return issues
}

// itoaValidator converts int to string without importing strconv
func itoaValidator(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	if negative {
		result = "-" + result
	}
	return result
}
