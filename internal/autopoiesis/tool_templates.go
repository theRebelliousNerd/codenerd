package autopoiesis

import (
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// =============================================================================
// TOOL TEMPLATES
// =============================================================================

// ToolTemplate is a template for generating common tool types
type ToolTemplate struct {
	Name     string
	Template *template.Template
}

var toolTemplates = map[string]string{
	"validator": `package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// {{.Name}}Description describes the {{.Name}} tool
const {{.Name}}Description = "{{.Description}}"

// ValidationError contains details about validation failures
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (got: %v)", e.Field, e.Message, e.Value)
}

// ValidationResult holds the complete validation result
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// {{.FuncName}} validates {{.InputType}} input
func {{.FuncName}}(ctx context.Context, input {{.InputType}}) (bool, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	// Check for empty/zero value
	if input == {{.ZeroValue}} {
		return false, &ValidationError{
			Field:   "input",
			Message: "value is empty or zero",
			Value:   input,
		}
	}

	// Type-specific validation
	result := validate{{.PascalName}}Input(input)
	if !result.Valid {
		if len(result.Errors) > 0 {
			return false, result.Errors[0]
		}
		return false, fmt.Errorf("validation failed")
	}

	return true, nil
}

// validate{{.PascalName}}Input performs type-specific validation
func validate{{.PascalName}}Input(input {{.InputType}}) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// String validation
	{{if eq .InputType "string"}}
	// Check for valid UTF-8
	if !utf8.ValidString(input) {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: "invalid UTF-8 encoding",
			Value:   input,
		})
	}

	// Check reasonable length (configurable)
	const maxLength = 1048576 // 1MB
	if len(input) > maxLength {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("exceeds maximum length of %d bytes", maxLength),
			Value:   len(input),
		})
	}

	// Check for control characters (except common whitespace)
	for i, r := range input {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "input",
				Message: fmt.Sprintf("contains control character at position %d", i),
				Value:   r,
			})
			break
		}
	}
	{{end}}

	// JSON validation (if input looks like JSON)
	{{if eq .InputType "string"}}
	trimmed := strings.TrimSpace(input)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		var js interface{}
		if err := json.Unmarshal([]byte(input), &js); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "input",
				Message: "invalid JSON syntax",
				Value:   err.Error(),
			})
		}
	}
	{{end}}

	// Byte slice validation
	{{if eq .InputType "[]byte"}}
	const maxSize = 10485760 // 10MB
	if len(input) > maxSize {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("exceeds maximum size of %d bytes", maxSize),
			Value:   len(input),
		})
	}
	{{end}}

	// Map validation
	{{if or (eq .InputType "map[string]any") (eq .InputType "map[string]interface{}")}}
	const maxKeys = 10000
	if len(input) > maxKeys {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "input",
			Message: fmt.Sprintf("exceeds maximum key count of %d", maxKeys),
			Value:   len(input),
		})
	}

	// Validate all keys are non-empty
	for key := range input {
		if strings.TrimSpace(key) == "" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "input",
				Message: "contains empty key",
				Value:   key,
			})
		}
	}
	{{end}}

	return result
}

// Helper validation functions
var _ = regexp.Compile // Ensure regexp is used

`,
	"converter": `package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// {{.Name}}Description describes the {{.Name}} tool
const {{.Name}}Description = "{{.Description}}"

// ConversionError provides details about conversion failures
type ConversionError struct {
	InputType  string
	OutputType string
	Reason     string
	Position   int
}

func (e ConversionError) Error() string {
	if e.Position >= 0 {
		return fmt.Sprintf("conversion from %s to %s failed at position %d: %s",
			e.InputType, e.OutputType, e.Position, e.Reason)
	}
	return fmt.Sprintf("conversion from %s to %s failed: %s",
		e.InputType, e.OutputType, e.Reason)
}

// {{.FuncName}} converts input from one format to another
func {{.FuncName}}(ctx context.Context, input {{.InputType}}) ({{.OutputType}}, error) {
	var result {{.OutputType}}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	default:
	}

	// Check for empty/zero value
	if input == {{.ZeroValue}} {
		return result, &ConversionError{
			InputType:  "{{.InputType}}",
			OutputType: "{{.OutputType}}",
			Reason:     "empty input",
			Position:   -1,
		}
	}

	// Perform the conversion
	converted, err := convert{{.PascalName}}(input)
	if err != nil {
		return result, err
	}

	return converted, nil
}

// convert{{.PascalName}} performs the actual conversion
func convert{{.PascalName}}(input {{.InputType}}) ({{.OutputType}}, error) {
	var result {{.OutputType}}

	{{if and (eq .InputType "string") (eq .OutputType "[]byte")}}
	// String to bytes conversion
	result = []byte(input)
	{{else if and (eq .InputType "[]byte") (eq .OutputType "string")}}
	// Bytes to string conversion
	result = string(input)
	{{else if and (eq .InputType "string") (eq .OutputType "map[string]any")}}
	// JSON string to map conversion
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return result, &ConversionError{
			InputType:  "string",
			OutputType: "map[string]any",
			Reason:     fmt.Sprintf("invalid JSON: %v", err),
			Position:   -1,
		}
	}
	{{else if and (eq .InputType "map[string]any") (eq .OutputType "string")}}
	// Map to JSON string conversion
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return result, &ConversionError{
			InputType:  "map[string]any",
			OutputType: "string",
			Reason:     fmt.Sprintf("marshal error: %v", err),
			Position:   -1,
		}
	}
	result = string(data)
	{{else if eq .OutputType "string"}}
	// Generic to string conversion
	result = fmt.Sprintf("%v", input)
	{{else}}
	// Generic conversion - implement specific logic
	_ = input // use input
	// Add type-specific conversion logic here
	{{end}}

	return result, nil
}

// Helper to ensure imports are used
var (
	_ = bytes.Buffer{}
	_ = strings.TrimSpace
	_ = json.Marshal
)

`,
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// extractJSONFromTemplate extracts a JSON object from text
// Note: Renamed to avoid duplicate with autopoiesis_helpers.go
func extractJSONFromTemplate(text string) string {
	// Find first { and last }
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")

	if start == -1 || end == -1 || end <= start {
		return "{}"
	}

	return text[start : end+1]
}

// extractCodeBlock extracts a code block from markdown-style response
func extractCodeBlock(text, lang string) string {
	// Look for ```go or ``` blocks
	patterns := []string{
		"```" + lang + "\n",
		"```" + lang + "\r\n",
		"```\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(text, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.Index(text[start:], "```")
			if end != -1 {
				return strings.TrimSpace(text[start : start+end])
			}
		}
	}

	// If no code block found, return the whole text (might be raw code)
	return strings.TrimSpace(text)
}

// extractDescription extracts tool description from Go source
func extractDescription(code string) string {
	// Look for Description constant
	descPattern := regexp.MustCompile(`(?m)const\s+\w*Description\s*=\s*"([^"]+)"`)
	if matches := descPattern.FindStringSubmatch(code); len(matches) > 1 {
		return matches[1]
	}

	// Look for package comment
	if strings.HasPrefix(code, "//") {
		lines := strings.Split(code, "\n")
		if len(lines) > 0 {
			return strings.TrimPrefix(lines[0], "// ")
		}
	}

	return "No description available"
}

// toCamelCase converts snake_case to camelCase
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 0; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// getTestValue returns an appropriate test value for a type
func getTestValue(typeName string, valid bool) string {
	switch typeName {
	case "string":
		if valid {
			return `"test input"`
		}
		return `""`
	case "[]byte":
		if valid {
			return `[]byte("test input")`
		}
		return `[]byte{}`
	case "int", "int32", "int64":
		if valid {
			return "42"
		}
		return "0"
	case "uint", "uint32", "uint64":
		if valid {
			return "42"
		}
		return "0"
	case "float64", "float32":
		if valid {
			return "3.14"
		}
		return "0.0"
	case "bool":
		if valid {
			return "true"
		}
		return "false"
	case "map[string]any", "map[string]interface{}":
		if valid {
			return `map[string]any{"key": "value"}`
		}
		return `map[string]any{}`
	case "[]string":
		if valid {
			return `[]string{"item1", "item2"}`
		}
		return `[]string{}`
	case "[]int":
		if valid {
			return `[]int{1, 2, 3}`
		}
		return `[]int{}`
	case "[]float64":
		if valid {
			return `[]float64{1.1, 2.2, 3.3}`
		}
		return `[]float64{}`
	case "[]any", "[]interface{}":
		if valid {
			return `[]any{"str", 42, true}`
		}
		return `[]any{}`
	case "map[string]string":
		if valid {
			return `map[string]string{"key": "value"}`
		}
		return `map[string]string{}`
	case "map[string]int":
		if valid {
			return `map[string]int{"count": 42}`
		}
		return `map[string]int{}`
	case "io.Reader":
		if valid {
			return `strings.NewReader("test input")`
		}
		return `strings.NewReader("")`
	case "io.Writer":
		if valid {
			return `new(bytes.Buffer)`
		}
		return `new(bytes.Buffer)`
	case "time.Time":
		if valid {
			return `time.Now()`
		}
		return `time.Time{}`
	case "time.Duration":
		if valid {
			return `time.Second`
		}
		return `0`
	case "error":
		if valid {
			return `errors.New("test error")`
		}
		return `nil`
	case "context.Context":
		return `context.Background()`
	default:
		return getComplexTestValue(typeName, valid)
	}
}

// getComplexTestValue handles complex types like slices, maps, pointers, and structs
func getComplexTestValue(typeName string, valid bool) string {
	// Handle slices: []ElementType
	if strings.HasPrefix(typeName, "[]") {
		elemType := strings.TrimPrefix(typeName, "[]")
		if valid {
			elemValue := getTestValue(elemType, true)
			return fmt.Sprintf("%s{%s}", typeName, elemValue)
		}
		return fmt.Sprintf("%s{}", typeName)
	}

	// Handle maps: map[KeyType]ValueType
	if strings.HasPrefix(typeName, "map[") {
		if valid {
			// Parse map type: map[K]V
			rest := strings.TrimPrefix(typeName, "map[")
			bracketDepth := 0
			keyEnd := 0
			for i, c := range rest {
				if c == '[' {
					bracketDepth++
				} else if c == ']' {
					if bracketDepth == 0 {
						keyEnd = i
						break
					}
					bracketDepth--
				}
			}
			if keyEnd > 0 {
				keyType := rest[:keyEnd]
				valueType := rest[keyEnd+1:]
				keyValue := getTestValue(keyType, true)
				valValue := getTestValue(valueType, true)
				return fmt.Sprintf("%s{%s: %s}", typeName, keyValue, valValue)
			}
		}
		return fmt.Sprintf("%s{}", typeName)
	}

	// Handle pointers: *Type
	if strings.HasPrefix(typeName, "*") {
		baseType := strings.TrimPrefix(typeName, "*")
		if valid {
			// For primitive types, create a pointer via helper
			switch baseType {
			case "string":
				return `func() *string { s := "test"; return &s }()`
			case "int", "int32", "int64":
				return `func() *int { i := 42; return &i }()`
			case "float64", "float32":
				return `func() *float64 { f := 3.14; return &f }()`
			case "bool":
				return `func() *bool { b := true; return &b }()`
			default:
				// For struct types, use address-of
				return fmt.Sprintf("&%s{}", baseType)
			}
		}
		return "nil"
	}

	// Handle function types: func(...)...
	if strings.HasPrefix(typeName, "func(") {
		if valid {
			// Return a no-op function
			return fmt.Sprintf("%s { }", typeName)
		}
		return "nil"
	}

	// Handle channel types: chan Type
	if strings.HasPrefix(typeName, "chan ") {
		elemType := strings.TrimPrefix(typeName, "chan ")
		if valid {
			return fmt.Sprintf("make(%s, 1)", typeName)
		}
		_ = elemType // avoid unused
		return fmt.Sprintf("(%s)(nil)", typeName)
	}

	// Handle struct types (anything else)
	if valid {
		return fmt.Sprintf("%s{}", typeName)
	}
	return fmt.Sprintf("%s{}", typeName)
}

// getZeroValue returns the zero value for a type
func getZeroValue(typeName string) string {
	switch typeName {
	case "string":
		return `""`
	case "[]byte":
		return "nil"
	case "int", "int32", "int64", "uint", "uint32", "uint64":
		return "0"
	case "float64", "float32":
		return "0.0"
	case "bool":
		return "false"
	case "error":
		return "nil"
	default:
		if strings.HasPrefix(typeName, "[]") || strings.HasPrefix(typeName, "map[") ||
			strings.HasPrefix(typeName, "*") {
			return "nil"
		}
		return fmt.Sprintf("%s{}", typeName)
	}
}
