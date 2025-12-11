package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const ToolDescriptionAToolThatAnalyzesGoCodeA = "Analyzes Go code and reports cyclomatic complexity for each function. Takes Go source code as input and returns a formatted report."

// aToolThatAnalyzesGoCodeA analyzes Go code and reports cyclomatic complexity for each function.
func aToolThatAnalyzesGoCodeA(ctx context.Context, input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("input Go code cannot be empty")
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Parse functions using regex instead of go/parser
	functions, err := extractFunctions(input)
	if err != nil {
		return "", fmt.Errorf("failed to extract functions: %w", err)
	}

	var report strings.Builder
	report.WriteString("Cyclomatic Complexity Report:\n")
	report.WriteString("=============================\n")

	if len(functions) == 0 {
		report.WriteString("No functions found in the provided code.\n")
	} else {
		for _, fn := range functions {
			complexity := calculateComplexity(fn.body)
			report.WriteString(fmt.Sprintf("Function: %s - Complexity: %d\n", fn.name, complexity))
		}
	}

	return report.String(), nil
}

// Function represents a parsed Go function
type Function struct {
	name string
	body string
}

// extractFunctions extracts function declarations and bodies from Go source code
func extractFunctions(source string) ([]Function, error) {
	var functions []Function

	// Regex to match function declarations
	funcRegex := regexp.MustCompile(`func\s+(\w+)\s*\([^)]*\)\s*(?:\([^)]*\))?\s*\{`)
	matches := funcRegex.FindAllStringSubmatchIndex(source, -1)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		funcName := source[match[2]:match[3]]
		startPos := match[0]

		// Find the function body
		bodyStart := strings.Index(source[startPos:], "{")
		if bodyStart == -1 {
			continue
		}
		bodyStart += startPos + 1

		// Find matching closing brace
		braceCount := 1
		bodyEnd := bodyStart
		for i := bodyStart; i < len(source); i++ {
			if source[i] == '{' {
				braceCount++
			} else if source[i] == '}' {
				braceCount--
				if braceCount == 0 {
					bodyEnd = i
					break
				}
			}
		}

		if braceCount != 0 {
			continue // Unbalanced braces
		}

		functions = append(functions, Function{
			name: funcName,
			body: source[bodyStart:bodyEnd],
		})
	}

	return functions, nil
}

// calculateComplexity computes the cyclomatic complexity of a function body
func calculateComplexity(body string) int {
	complexity := 1 // Base complexity

	// Count decision points using regex
	patterns := []string{
		`\bif\b`,           // if statements
		`\bfor\b`,          // for loops
		`\bcase\b`,         // case statements
		`\bselect\b`,       // select statements
		`\bgo\b`,           // goroutines
		`\bdefer\b`,        // defer statements
		`&&`,               // logical AND
		`\|\|`,             // logical OR
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(body, -1)
		complexity += len(matches)
	}

	return complexity
}

// Register registers the tool with the tool registry
func RegisterAToolThatAnalyzesGoCodeA(registry map[string]interface{}) {
	registry["a_tool_that_analyzes_go_code_a"] = aToolThatAnalyzesGoCodeA
}