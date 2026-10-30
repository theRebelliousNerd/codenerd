package autopoiesis

import (
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

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
	for i := range parts {
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
	if after, ok := strings.CutPrefix(typeName, "[]"); ok {
		elemType := after
		if valid {
			elemValue := getTestValue(elemType, true)
			return fmt.Sprintf("%s{%s}", typeName, elemValue)
		}
		return fmt.Sprintf("%s{}", typeName)
	}

	// Handle maps: map[KeyType]ValueType
	if after, ok := strings.CutPrefix(typeName, "map["); ok {
		if valid {
			// Parse map type: map[K]V
			rest := after
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
	if after, ok := strings.CutPrefix(typeName, "*"); ok {
		baseType := after
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
	if after, ok := strings.CutPrefix(typeName, "chan "); ok {
		elemType := after
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
