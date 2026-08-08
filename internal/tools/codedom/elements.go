package codedom

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// CodeElement represents a code element (function, class, method, etc.)
type CodeElement struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // function, class, method, interface, struct
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Signature string `json:"signature,omitempty"`
}

// Pre-compiled regex patterns
var (
	goPatterns = map[string]*regexp.Regexp{
		"function":  regexp.MustCompile(`^func\s+(\w+)\s*\(`),
		"method":    regexp.MustCompile(`^func\s+\([^)]+\)\s+(\w+)\s*\(`),
		"struct":    regexp.MustCompile(`^type\s+(\w+)\s+struct`),
		"interface": regexp.MustCompile(`^type\s+(\w+)\s+interface`),
	}

	pyPatterns = map[string]*regexp.Regexp{
		"function": regexp.MustCompile(`^def\s+(\w+)\s*\(`),
		"class":    regexp.MustCompile(`^class\s+(\w+)`),
		"method":   regexp.MustCompile(`^\s+def\s+(\w+)\s*\(`),
	}

	jsPatterns = map[string]*regexp.Regexp{
		"function": regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`),
		"class":    regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`),
		"method":   regexp.MustCompile(`^\s+(?:async\s+)?(\w+)\s*\([^)]*\)\s*\{`),
		"arrow":    regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`),
	}

	javaPatterns = map[string]*regexp.Regexp{
		"class":     regexp.MustCompile(`^(?:public\s+)?(?:abstract\s+)?class\s+(\w+)`),
		"interface": regexp.MustCompile(`^(?:public\s+)?interface\s+(\w+)`),
		"method":    regexp.MustCompile(`^\s+(?:public|private|protected)?\s*(?:static\s+)?(?:\w+\s+)+(\w+)\s*\(`),
	}

	rsPatterns = map[string]*regexp.Regexp{
		"function": regexp.MustCompile(`^(?:pub\s+)?fn\s+(\w+)`),
		"struct":   regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`),
		"impl":     regexp.MustCompile(`^impl\s+(?:<[^>]+>\s+)?(\w+)`),
		"trait":    regexp.MustCompile(`^(?:pub\s+)?trait\s+(\w+)`),
	}

	cppPatterns = map[string]*regexp.Regexp{
		"function": regexp.MustCompile(`^(?:\w+\s+)+(\w+)\s*\([^)]*\)\s*\{?$`),
		"class":    regexp.MustCompile(`^class\s+(\w+)`),
		"struct":   regexp.MustCompile(`^struct\s+(\w+)`),
	}

	genericPatterns = map[string]*regexp.Regexp{
		"function": regexp.MustCompile(`(?:function|func|def|fn)\s+(\w+)`),
		"class":    regexp.MustCompile(`class\s+(\w+)`),
	}
)

// GetElementsTool returns a tool for listing code elements in a file.
func GetElementsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "get_elements",
		Description: "List code elements (functions, classes, methods) in a file",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeGetElements,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File path to analyze",
				},
				"type": {
					Type:        "string",
					Description: "Filter by element type (function, class, method, struct, interface)",
				},
			},
		},
	}
}

func executeGetElements(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	filterType, _ := args["type"].(string)

	logging.VirtualStoreDebug("get_elements: path=%s, type=%s", path, filterType)

	elements, err := extractCodeElements(path)
	if err != nil {
		return "", fmt.Errorf("failed to extract elements: %w", err)
	}

	// Filter by type if specified
	if filterType != "" {
		var filtered []CodeElement
		for _, e := range elements {
			if strings.EqualFold(e.Type, filterType) {
				filtered = append(filtered, e)
			}
		}
		elements = filtered
	}

	if len(elements) == 0 {
		return "No code elements found", nil
	}

	output, _ := json.MarshalIndent(elements, "", "  ")
	logging.VirtualStore("get_elements completed: %s (%d elements)", path, len(elements))
	return string(output), nil
}

// extractCodeElements extracts code elements from a file using regex patterns.
// This is a simplified implementation - full AST parsing is done by VirtualStore.
func extractCodeElements(path string) ([]CodeElement, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	// Split into lines; handle empty file.
	var lines []string
	if content == "" {
		lines = []string{}
	} else {
		lines = strings.Split(content, "\n")
		// strings.Split with trailing newline yields an extra empty element that is not a real line.
		if strings.HasSuffix(content, "\n") && len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		// Strip trailing \r for CRLF files to emulate bufio.Scanner behavior.
		for i, l := range lines {
			lines[i] = strings.TrimSuffix(l, "\r")
		}
	}

	// Language detection based on extension
	ext := ""
	if dot := strings.LastIndex(path, "."); dot != -1 {
		ext = strings.ToLower(path[dot+1:])
	}

	// Patterns for different languages
	var patterns map[string]*regexp.Regexp

	switch ext {
	case "go":
		patterns = goPatterns
	case "py":
		patterns = pyPatterns
	case "js", "ts", "jsx", "tsx":
		patterns = jsPatterns
	case "java", "kt", "scala":
		patterns = javaPatterns
	case "rs":
		patterns = rsPatterns
	case "c", "cpp", "cc", "cxx", "h", "hpp":
		patterns = cppPatterns
	default:
		patterns = genericPatterns
	}

	var elements []CodeElement
	for idx, line := range lines {
		for elemType, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				startLine := idx + 1
				var endLine int
				if ext == "py" {
					endLine = findPythonEndLine(lines, idx)
				} else {
					endLine = findBraceEndLine(lines, idx)
				}
				elements = append(elements, CodeElement{
					Name:      matches[1],
					Type:      elemType,
					File:      path,
					StartLine: startLine,
					EndLine:   endLine,
					Signature: strings.TrimSpace(line),
				})
			}
		}
	}

	return elements, nil
}

// findBraceEndLine computes the end line for brace-based languages by counting
// braces from the declaration line until they balance. Braces inside string
// literals (", ', `), rune literals and comments (//, /* */) are ignored.
func findBraceEndLine(lines []string, startIdx int) int {
	depth := 0
	opened := false
	inBlockComment := false
	inBacktick := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		inDouble := false
		inSingle := false
		inLineComment := false
		j := 0
		for j < len(line) {
			if inBlockComment {
				if line[j] == '*' && j+1 < len(line) && line[j+1] == '/' {
					inBlockComment = false
					j += 2
					continue
				}
				j++
				continue
			}
			if inLineComment {
				break
			}
			if inSingle {
				if line[j] == '\\' {
					// escaped character inside rune/char literal
					if j+1 < len(line) {
						j += 2
					} else {
						j++
					}
					continue
				}
				if line[j] == '\'' {
					inSingle = false
				}
				j++
				continue
			}
			if inDouble {
				if line[j] == '\\' {
					if j+1 < len(line) {
						j += 2
					} else {
						j++
					}
					continue
				}
				if line[j] == '"' {
					inDouble = false
				}
				j++
				continue
			}
			if inBacktick {
				if line[j] == '`' {
					inBacktick = false
				}
				j++
				continue
			}
			// Not inside any literal or comment
			if line[j] == '/' && j+1 < len(line) {
				if line[j+1] == '/' {
					inLineComment = true
					break
				}
				if line[j+1] == '*' {
					inBlockComment = true
					j += 2
					continue
				}
			}
			if line[j] == '"' {
				inDouble = true
				j++
				continue
			}
			if line[j] == '\'' {
				inSingle = true
				j++
				continue
			}
			if line[j] == '`' {
				inBacktick = true
				j++
				continue
			}
			if line[j] == '{' {
				depth++
				opened = true
			} else if line[j] == '}' {
				depth--
				if opened && depth == 0 {
					return i + 1
				}
				if depth < 0 {
					depth = 0
				}
			}
			j++
		}
	}
	if !opened {
		return startIdx + 1
	}
	return len(lines)
}

// indentLevel returns the indentation level (number of leading spaces/tabs) of a line.
func indentLevel(line string) int {
	count := 0
	for _, ch := range line {
		if ch == ' ' || ch == '\t' {
			count++
		} else {
			break
		}
	}
	return count
}

// findPythonEndLine computes the end line for Python by indentation.
// The element ends at the last line more indented than the declaration.
func findPythonEndLine(lines []string, startIdx int) int {
	if startIdx < 0 || startIdx >= len(lines) {
		return startIdx + 1
	}
	baseIndent := indentLevel(lines[startIdx])
	endIdx := startIdx
	for i := startIdx + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		curIndent := indentLevel(line)
		if curIndent > baseIndent {
			endIdx = i
		} else {
			break
		}
	}
	return endIdx + 1
}

// GetElementTool returns a tool for getting a specific code element.
func GetElementTool() *tools.Tool {
	return &tools.Tool{
		Name:        "get_element",
		Description: "Get a specific code element by name",
		Category:    tools.CategoryCode,
		Priority:    80,
		Execute:     executeGetElement,
		Schema: tools.ToolSchema{
			Required: []string{"path", "name"},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File path to search",
				},
				"name": {
					Type:        "string",
					Description: "Element name to find",
				},
			},
		},
	}
}

func executeGetElement(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	logging.VirtualStoreDebug("get_element: path=%s, name=%s", path, name)

	elements, err := extractCodeElements(path)
	if err != nil {
		return "", fmt.Errorf("failed to extract elements: %w", err)
	}

	for _, e := range elements {
		if e.Name == name {
			output, _ := json.MarshalIndent(e, "", "  ")
			return string(output), nil
		}
	}

	return "", fmt.Errorf("element not found: %s", name)
}
