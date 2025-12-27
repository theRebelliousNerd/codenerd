package codedom

import (
	"bufio"
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
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var elements []CodeElement
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Language detection based on extension
	ext := strings.ToLower(path[strings.LastIndex(path, ".")+1:])

	// Patterns for different languages
	var patterns map[string]*regexp.Regexp

	switch ext {
	case "go":
		patterns = map[string]*regexp.Regexp{
			"function":  regexp.MustCompile(`^func\s+(\w+)\s*\(`),
			"method":    regexp.MustCompile(`^func\s+\([^)]+\)\s+(\w+)\s*\(`),
			"struct":    regexp.MustCompile(`^type\s+(\w+)\s+struct`),
			"interface": regexp.MustCompile(`^type\s+(\w+)\s+interface`),
		}
	case "py":
		patterns = map[string]*regexp.Regexp{
			"function": regexp.MustCompile(`^def\s+(\w+)\s*\(`),
			"class":    regexp.MustCompile(`^class\s+(\w+)`),
			"method":   regexp.MustCompile(`^\s+def\s+(\w+)\s*\(`),
		}
	case "js", "ts", "jsx", "tsx":
		patterns = map[string]*regexp.Regexp{
			"function": regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`),
			"class":    regexp.MustCompile(`^(?:export\s+)?class\s+(\w+)`),
			"method":   regexp.MustCompile(`^\s+(?:async\s+)?(\w+)\s*\([^)]*\)\s*\{`),
			"arrow":    regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`),
		}
	case "java", "kt", "scala":
		patterns = map[string]*regexp.Regexp{
			"class":     regexp.MustCompile(`^(?:public\s+)?(?:abstract\s+)?class\s+(\w+)`),
			"interface": regexp.MustCompile(`^(?:public\s+)?interface\s+(\w+)`),
			"method":    regexp.MustCompile(`^\s+(?:public|private|protected)?\s*(?:static\s+)?(?:\w+\s+)+(\w+)\s*\(`),
		}
	case "rs":
		patterns = map[string]*regexp.Regexp{
			"function": regexp.MustCompile(`^(?:pub\s+)?fn\s+(\w+)`),
			"struct":   regexp.MustCompile(`^(?:pub\s+)?struct\s+(\w+)`),
			"impl":     regexp.MustCompile(`^impl\s+(?:<[^>]+>\s+)?(\w+)`),
			"trait":    regexp.MustCompile(`^(?:pub\s+)?trait\s+(\w+)`),
		}
	case "c", "cpp", "cc", "cxx", "h", "hpp":
		patterns = map[string]*regexp.Regexp{
			"function": regexp.MustCompile(`^(?:\w+\s+)+(\w+)\s*\([^)]*\)\s*\{?$`),
			"class":    regexp.MustCompile(`^class\s+(\w+)`),
			"struct":   regexp.MustCompile(`^struct\s+(\w+)`),
		}
	default:
		// Generic patterns
		patterns = map[string]*regexp.Regexp{
			"function": regexp.MustCompile(`(?:function|func|def|fn)\s+(\w+)`),
			"class":    regexp.MustCompile(`class\s+(\w+)`),
		}
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for elemType, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				elements = append(elements, CodeElement{
					Name:      matches[1],
					Type:      elemType,
					File:      path,
					StartLine: lineNum,
					EndLine:   lineNum, // Would need block tracking for accurate end
					Signature: strings.TrimSpace(line),
				})
			}
		}
	}

	return elements, scanner.Err()
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
