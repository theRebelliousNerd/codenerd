package reviewer

import (
	"context"
	"strings"
)

// =============================================================================
// METRICS CALCULATION
// =============================================================================

// calculateMetrics calculates code complexity metrics.
// Cyclomatic complexity is calculated per-function: CC = 1 + decision_points
func (r *ReviewerShard) calculateMetrics(ctx context.Context, files []string) *CodeMetrics {
	metrics := &CodeMetrics{}

	// Track per-function cyclomatic complexities for proper max/avg calculation
	var functionComplexities []int

	for _, filePath := range files {
		content, err := r.readFile(ctx, filePath)
		if err != nil {
			continue
		}

		lines := strings.Split(content, "\n")
		metrics.TotalLines += len(lines)

		// Count line types and track complexity per function
		inMultiLineComment := false
		currentNesting := 0
		maxNestingInFile := 0
		currentFunctionLines := 0
		currentFunctionCC := 1 // Cyclomatic complexity starts at 1 for each function
		inFunction := false
		braceDepth := 0 // Track brace depth to detect function end

		for _, line := range lines {
			trimmed := strings.TrimSpace(line)

			// Blank lines
			if trimmed == "" {
				metrics.BlankLines++
				continue
			}

			// Multi-line comments
			if strings.Contains(line, "/*") {
				inMultiLineComment = true
			}
			if strings.Contains(line, "*/") {
				inMultiLineComment = false
				metrics.CommentLines++
				continue
			}
			if inMultiLineComment {
				metrics.CommentLines++
				continue
			}

			// Single-line comments
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				metrics.CommentLines++
				continue
			}

			metrics.CodeLines++

			// Track nesting (rough estimate)
			openBraces := strings.Count(line, "{")
			closeBraces := strings.Count(line, "}")
			currentNesting += openBraces - closeBraces
			if currentNesting > maxNestingInFile {
				maxNestingInFile = currentNesting
			}

			// Track function boundaries (Go/C-style)
			isFuncDecl := strings.Contains(line, "func ") || strings.Contains(line, "function ") ||
				strings.Contains(line, "def ") || strings.Contains(line, "fn ")

			if isFuncDecl {
				// Finish previous function if any
				if inFunction {
					if currentFunctionLines > 50 {
						metrics.LongFunctions++
					}
					functionComplexities = append(functionComplexities, currentFunctionCC)
				}
				// Start new function
				metrics.FunctionCount++
				inFunction = true
				currentFunctionLines = 0
				currentFunctionCC = 1 // Reset CC for new function
				braceDepth = 0
			}

			if inFunction {
				currentFunctionLines++

				// Track brace depth to detect function end
				braceDepth += openBraces - closeBraces

				// Count decision points for cyclomatic complexity
				// Each adds 1 to complexity: if, else if, for, while, case, catch, &&, ||, ?:
				currentFunctionCC += countDecisionPoints(line)

				// Function ended (brace depth returned to 0 after being > 0)
				if braceDepth <= 0 && currentFunctionLines > 1 {
					if currentFunctionLines > 50 {
						metrics.LongFunctions++
					}
					functionComplexities = append(functionComplexities, currentFunctionCC)
					inFunction = false
					currentFunctionCC = 1
					currentFunctionLines = 0
				}
			}
		}

		if maxNestingInFile > metrics.MaxNesting {
			metrics.MaxNesting = maxNestingInFile
		}

		// Check last function (if file ended while still in function)
		if inFunction {
			if currentFunctionLines > 50 {
				metrics.LongFunctions++
			}
			functionComplexities = append(functionComplexities, currentFunctionCC)
		}
	}

	// Calculate max and average cyclomatic complexity from per-function values
	if len(functionComplexities) > 0 {
		totalCC := 0
		for _, cc := range functionComplexities {
			totalCC += cc
			if cc > metrics.CyclomaticMax {
				metrics.CyclomaticMax = cc
			}
		}
		metrics.CyclomaticAvg = float64(totalCC) / float64(len(functionComplexities))
	}

	return metrics
}

// countDecisionPoints counts cyclomatic complexity decision points in a line.
// Each decision point adds 1 to complexity.
func countDecisionPoints(line string) int {
	count := 0

	// Decision keywords (each adds 1 to CC)
	decisionKeywords := []string{
		"if ", "else if ", "elif ", // Conditionals
		"for ", "while ", "loop ", // Loops
		"case ", "catch ", "except ", // Switch cases, exception handlers
		"?", // Ternary operator
	}

	for _, keyword := range decisionKeywords {
		count += strings.Count(line, keyword)
	}

	// Logical operators (each adds 1 to CC since they create additional paths)
	count += strings.Count(line, " && ")
	count += strings.Count(line, " || ")
	count += strings.Count(line, " and ")
	count += strings.Count(line, " or ")

	return count
}
