package reviewer

import (
	"codenerd/internal/logging"
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// LANGUAGE DETECTION
// =============================================================================

// Language represents a programming language for CC calculation
type Language int

const (
	LangUnknown Language = iota
	LangGo
	LangPython
	LangJavaScript
	LangTypeScript
	LangJava
	LangCSharp
	LangRust
	LangC
	LangCPP
	LangRuby
	LangSwift
	LangKotlin
	LangPHP
)

// detectLanguage determines the programming language from file extension
func detectLanguage(filePath string) Language {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return LangGo
	case ".py", ".pyw":
		return LangPython
	case ".js", ".jsx", ".mjs", ".cjs":
		return LangJavaScript
	case ".ts", ".tsx", ".mts", ".cts":
		return LangTypeScript
	case ".java":
		return LangJava
	case ".cs":
		return LangCSharp
	case ".rs":
		return LangRust
	case ".c", ".h":
		return LangC
	case ".cpp", ".cc", ".cxx", ".hpp", ".hxx":
		return LangCPP
	case ".rb":
		return LangRuby
	case ".swift":
		return LangSwift
	case ".kt", ".kts":
		return LangKotlin
	case ".php":
		return LangPHP
	default:
		return LangUnknown
	}
}

// =============================================================================
// PRE-COMPILED REGEX PATTERNS
// =============================================================================

var (
	// Comment patterns
	singleLineCommentRe  = regexp.MustCompile(`//.*$`)
	hashCommentRe        = regexp.MustCompile(`#.*$`)
	inlineBlockCommentRe = regexp.MustCompile(`/\*.*?\*/`)

	// String literal patterns (replace with placeholder to preserve structure)
	doubleQuoteStringRe = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	singleQuoteStringRe = regexp.MustCompile(`'(?:[^'\\]|\\.)*'`)
	backtickStringRe    = regexp.MustCompile("`[^`]*`")
	tripleDoubleQuoteRe = regexp.MustCompile(`"""[\s\S]*?"""`)
	tripleSingleQuoteRe = regexp.MustCompile(`'''[\s\S]*?'''`)

	// Decision keywords with word boundaries
	// Compound patterns (must be checked first to avoid double-counting)
	elseIfRe   = regexp.MustCompile(`\belse\s+if\b`)
	elifRe     = regexp.MustCompile(`\belif\b`)
	ifLetRe    = regexp.MustCompile(`\bif\s+let\b`)    // Rust/Swift
	whileLetRe = regexp.MustCompile(`\bwhile\s+let\b`) // Rust

	// Simple keywords
	ifRe     = regexp.MustCompile(`\bif\b`)
	forRe    = regexp.MustCompile(`\bfor\b`)
	whileRe  = regexp.MustCompile(`\bwhile\b`)
	loopRe   = regexp.MustCompile(`\bloop\b`) // Rust infinite loop
	caseRe   = regexp.MustCompile(`\bcase\b`)
	catchRe  = regexp.MustCompile(`\bcatch\b`)
	exceptRe = regexp.MustCompile(`\bexcept\b`) // Python
	matchRe  = regexp.MustCompile(`\bmatch\b`)  // Rust, Python 3.10+
	whenRe   = regexp.MustCompile(`\bwhen\b`)   // Kotlin
	guardRe  = regexp.MustCompile(`\bguard\b`)  // Swift
	selectRe = regexp.MustCompile(`\bselect\b`) // Go channels
	rescueRe = regexp.MustCompile(`\brescue\b`) // Ruby

	// Logical operators (short-circuit creates branches)
	andOpRe     = regexp.MustCompile(`&&`)
	orOpRe      = regexp.MustCompile(`\|\|`)
	pythonAndRe = regexp.MustCompile(`\band\b`)
	pythonOrRe  = regexp.MustCompile(`\bor\b`)

	// Ternary operator - need to distinguish from ?. and ??
	// Match ? followed by something that isn't . or ?
	ternaryRe = regexp.MustCompile(`\?(?:[^.?]|$)`)

	// Optional chaining ?. - NOT a decision point
	optionalChainRe = regexp.MustCompile(`\?\.`)

	// Null coalescing ?? - debatable, but often counted as it creates a branch
	nullCoalesceRe = regexp.MustCompile(`\?\?`)
)

// =============================================================================
// COMMENT AND STRING STRIPPING
// =============================================================================

// stripCommentsAndStrings removes comments and string literals from a line
// to prevent false positives in keyword detection. Preserves line structure.
func stripCommentsAndStrings(line string, lang Language) string {
	result := line

	// Remove inline block comments first /* ... */
	result = inlineBlockCommentRe.ReplaceAllString(result, " ")

	// Remove single-line comments based on language
	switch lang {
	case LangPython, LangRuby:
		// Python/Ruby use # for comments
		result = hashCommentRe.ReplaceAllString(result, "")
	default:
		// Most languages use //
		result = singleLineCommentRe.ReplaceAllString(result, "")
	}

	// Remove string literals (replace with empty strings to preserve structure)
	// Order matters: triple quotes before single/double quotes
	if lang == LangPython {
		result = tripleDoubleQuoteRe.ReplaceAllString(result, `""`)
		result = tripleSingleQuoteRe.ReplaceAllString(result, `''`)
	}

	result = doubleQuoteStringRe.ReplaceAllString(result, `""`)
	result = singleQuoteStringRe.ReplaceAllString(result, `''`)

	// Template literals (JS/TS) and raw strings (Go)
	result = backtickStringRe.ReplaceAllString(result, "``")

	return result
}

// =============================================================================
// METRICS CALCULATION
// =============================================================================

// calculateMetrics calculates code complexity metrics.
// Cyclomatic complexity is calculated per-function: CC = 1 + decision_points
// Uses McCabe's formula: M = 1 + number of predicate nodes (decision points)
func (r *ReviewerShard) calculateMetrics(ctx context.Context, files []string) *CodeMetrics {
	logging.ReviewerDebug("Calculating code metrics for %d files", len(files))
	metrics := &CodeMetrics{}

	// Track per-function cyclomatic complexities for proper max/avg calculation
	var functionComplexities []int

	for _, filePath := range files {
		content, err := r.readFile(ctx, filePath)
		if err != nil {
			logging.ReviewerDebug("calculateMetrics: failed to read file %s: %v", filePath, err)
			continue
		}

		// Detect language for this file
		lang := detectLanguage(filePath)

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

		for i, line := range lines {
			currentLineNum := i + 1
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

			// Track function boundaries (language-aware)
			isFuncDecl := isFunctionDeclaration(line, lang)

			if isFuncDecl {
				// Finish previous function if any
				if inFunction {
					if currentFunctionLines > 50 {
						metrics.LongFunctions++
					}
					functionComplexities = append(functionComplexities, currentFunctionCC)

					// Record previous function metric
					if len(metrics.Functions) > 0 {
						lastIdx := len(metrics.Functions) - 1
						metrics.Functions[lastIdx].Complexity = currentFunctionCC
						metrics.Functions[lastIdx].EndLine = currentLineNum - 1 // Previous line
					}
				}

				// Start new function
				metrics.FunctionCount++
				inFunction = true
				currentFunctionLines = 0
				currentFunctionCC = 1 // Reset CC for new function
				braceDepth = 0

				// Extract function name and start tracking
				funcName := extractFunctionName(line, lang)
				if funcName == "" {
					funcName = "anonymous"
				}
				metrics.Functions = append(metrics.Functions, FunctionMetric{
					Name:      funcName,
					File:      filePath,
					StartLine: currentLineNum,
					Nesting:   currentNesting, // Starting nesting level
				})
			}

			if inFunction {
				currentFunctionLines++

				// Track brace depth to detect function end
				braceDepth += openBraces - closeBraces

				// Count decision points for cyclomatic complexity
				currentFunctionCC += countDecisionPoints(line, lang)

				// Update max nesting for current function
				if inFunction && len(metrics.Functions) > 0 {
					lastIdx := len(metrics.Functions) - 1
					if currentNesting > metrics.Functions[lastIdx].Nesting {
						metrics.Functions[lastIdx].Nesting = currentNesting
					}
				}

				// Function ended (brace depth returned to 0 after being > 0)
				if braceDepth <= 0 && currentFunctionLines > 1 {
					if currentFunctionLines > 50 {
						metrics.LongFunctions++
					}
					functionComplexities = append(functionComplexities, currentFunctionCC)

					// Update final metrics for this function
					if len(metrics.Functions) > 0 {
						lastIdx := len(metrics.Functions) - 1
						metrics.Functions[lastIdx].Complexity = currentFunctionCC
						metrics.Functions[lastIdx].EndLine = currentLineNum
					}

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

			// Update final metrics for last function
			if len(metrics.Functions) > 0 {
				lastIdx := len(metrics.Functions) - 1
				metrics.Functions[lastIdx].Complexity = currentFunctionCC
				metrics.Functions[lastIdx].EndLine = len(lines)
			}
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

	logging.ReviewerDebug("Metrics complete: %d total lines, %d code lines, %d functions, max CC=%d, avg CC=%.2f",
		metrics.TotalLines, metrics.CodeLines, metrics.FunctionCount, metrics.CyclomaticMax, metrics.CyclomaticAvg)
	return metrics
}

// isFunctionDeclaration detects function declarations based on language
func isFunctionDeclaration(line string, lang Language) bool {
	switch lang {
	case LangGo:
		return strings.Contains(line, "func ")
	case LangPython:
		return strings.Contains(line, "def ") || strings.Contains(line, "async def ")
	case LangRust:
		return strings.Contains(line, "fn ") || strings.Contains(line, "async fn ")
	case LangJavaScript, LangTypeScript:
		return strings.Contains(line, "function ") ||
			strings.Contains(line, "=> {") ||
			strings.Contains(line, "async function ")
	case LangRuby:
		return strings.Contains(line, "def ")
	case LangSwift:
		return strings.Contains(line, "func ")
	case LangKotlin:
		return strings.Contains(line, "fun ")
	case LangPHP:
		return strings.Contains(line, "function ")
	default:
		// Generic detection for C-family languages
		return strings.Contains(line, "func ") ||
			strings.Contains(line, "function ") ||
			strings.Contains(line, "def ") ||
			strings.Contains(line, "fn ")
	}
}

// countDecisionPoints counts cyclomatic complexity decision points in a line.
// Uses McCabe's definition: each predicate node (decision point) adds 1 to CC.
//
// Decision points counted:
//   - Conditionals: if, else if, elif, guard (Swift), when (Kotlin)
//   - Loops: for, while, loop (Rust)
//   - Exception handling: catch, except, rescue
//   - Pattern matching: case (in switch/select/match)
//   - Short-circuit operators: &&, ||, and, or
//   - Ternary operator: ?: (but NOT ?. or ??)
//
// NOT counted (not decision points per McCabe):
//   - else (fallthrough of if, not a new decision)
//   - default (fallthrough of switch)
//   - finally (always executes)
//   - Optional chaining ?. (no branch, just null propagation)
func countDecisionPoints(line string, lang Language) int {
	// First, strip comments and string literals to avoid false positives
	cleaned := stripCommentsAndStrings(line, lang)
	count := 0

	// ==========================================================================
	// COMPOUND PATTERNS (must be counted and removed first to avoid double-count)
	// ==========================================================================

	// Count "else if" patterns first, then remove to prevent "if" double-counting
	elseIfCount := len(elseIfRe.FindAllString(cleaned, -1))
	count += elseIfCount
	cleaned = elseIfRe.ReplaceAllString(cleaned, " ")

	// Count "elif" (Python)
	elifCount := len(elifRe.FindAllString(cleaned, -1))
	count += elifCount
	cleaned = elifRe.ReplaceAllString(cleaned, " ")

	// Count "if let" / "while let" (Rust/Swift) - each is one decision point
	if lang == LangRust || lang == LangSwift {
		ifLetCount := len(ifLetRe.FindAllString(cleaned, -1))
		count += ifLetCount
		cleaned = ifLetRe.ReplaceAllString(cleaned, " ")

		whileLetCount := len(whileLetRe.FindAllString(cleaned, -1))
		count += whileLetCount
		cleaned = whileLetRe.ReplaceAllString(cleaned, " ")
	}

	// ==========================================================================
	// STANDARD DECISION KEYWORDS
	// ==========================================================================

	// Standalone if (after removing else-if and if-let)
	count += len(ifRe.FindAllString(cleaned, -1))

	// Loops
	count += len(forRe.FindAllString(cleaned, -1))
	count += len(whileRe.FindAllString(cleaned, -1))

	// ==========================================================================
	// LANGUAGE-SPECIFIC CONSTRUCTS
	// ==========================================================================

	switch lang {
	case LangGo:
		// Go: select (channel operations), case (in switch/select)
		count += len(selectRe.FindAllString(cleaned, -1))
		count += len(caseRe.FindAllString(cleaned, -1))

	case LangRust:
		// Rust: loop (infinite), match, case patterns
		count += len(loopRe.FindAllString(cleaned, -1))
		count += len(matchRe.FindAllString(cleaned, -1))
		// Note: Rust match arms are complex; we count 'match' as 1
		// More accurate would require AST parsing

	case LangPython:
		// Python: except (exception), match/case (3.10+)
		count += len(exceptRe.FindAllString(cleaned, -1))
		count += len(matchRe.FindAllString(cleaned, -1))
		count += len(caseRe.FindAllString(cleaned, -1))

	case LangRuby:
		// Ruby: rescue (exception handling), case/when
		count += len(rescueRe.FindAllString(cleaned, -1))
		count += len(caseRe.FindAllString(cleaned, -1))
		count += len(whenRe.FindAllString(cleaned, -1))

	case LangSwift:
		// Swift: guard, case, catch
		count += len(guardRe.FindAllString(cleaned, -1))
		count += len(caseRe.FindAllString(cleaned, -1))
		count += len(catchRe.FindAllString(cleaned, -1))

	case LangKotlin:
		// Kotlin: when (like switch), catch
		count += len(whenRe.FindAllString(cleaned, -1))
		count += len(catchRe.FindAllString(cleaned, -1))

	case LangJavaScript, LangTypeScript, LangJava, LangCSharp, LangC, LangCPP, LangPHP:
		// C-family languages: case, catch
		count += len(caseRe.FindAllString(cleaned, -1))
		count += len(catchRe.FindAllString(cleaned, -1))

	default:
		// Unknown language: count common patterns
		count += len(caseRe.FindAllString(cleaned, -1))
		count += len(catchRe.FindAllString(cleaned, -1))
		count += len(exceptRe.FindAllString(cleaned, -1))
	}

	// ==========================================================================
	// LOGICAL OPERATORS (short-circuit evaluation creates branches)
	// ==========================================================================

	count += len(andOpRe.FindAllString(cleaned, -1))
	count += len(orOpRe.FindAllString(cleaned, -1))

	// Python-style logical operators
	if lang == LangPython || lang == LangRuby {
		count += len(pythonAndRe.FindAllString(cleaned, -1))
		count += len(pythonOrRe.FindAllString(cleaned, -1))
	}

	// ==========================================================================
	// TERNARY OPERATOR (language-specific handling)
	// ==========================================================================

	// Languages without ternary operator
	if lang == LangGo || lang == LangRust {
		// Go and Rust don't have ternary operators
		// Rust's ? is error propagation, not ternary
		return count
	}

	if lang == LangPython {
		// Python ternary is "x if condition else y"
		// The 'if' is already counted above, so no additional count needed
		return count
	}

	// For other languages, count ternary ? but NOT ?. or ??
	// Strategy: remove ?. and ?? first, then count remaining ?
	ternaryLine := cleaned
	ternaryLine = strings.ReplaceAll(ternaryLine, "?.", " ") // Remove optional chaining
	ternaryLine = strings.ReplaceAll(ternaryLine, "??", " ") // Remove null coalescing
	count += strings.Count(ternaryLine, "?")

	// Optionally count null coalescing as a decision point
	// (Some tools do, some don't. We'll count it as it does create a branch.)
	if lang == LangJavaScript || lang == LangTypeScript || lang == LangCSharp || lang == LangKotlin || lang == LangSwift {
		count += len(nullCoalesceRe.FindAllString(cleaned, -1))
	}

	return count
}

// extractFunctionName attempts to extract the function name from a declaration line
func extractFunctionName(line string, lang Language) string {
	cleaned := strings.TrimSpace(line)
	// Simple regexes for common patterns
	// Note: accurate parsing requires AST, this is a best-effort heuristic

	switch lang {
	case LangGo:
		// func Name(...)
		// func (r *Receiver) Name(...)
		if strings.Contains(cleaned, "func ") {
			// Try method receiver pattern first
			reMethod := regexp.MustCompile(`func\s+\([^)]+\)\s+(\w+)`)
			if matches := reMethod.FindStringSubmatch(cleaned); len(matches) > 1 {
				return matches[1]
			}
			// Try standard func pattern
			reFunc := regexp.MustCompile(`func\s+(\w+)`)
			if matches := reFunc.FindStringSubmatch(cleaned); len(matches) > 1 {
				return matches[1]
			}
		}
	case LangPython:
		// def name(...):
		re := regexp.MustCompile(`def\s+(\w+)`)
		if matches := re.FindStringSubmatch(cleaned); len(matches) > 1 {
			return matches[1]
		}
	case LangJavaScript, LangTypeScript:
		// function name(...)
		re := regexp.MustCompile(`function\s+(\w+)`)
		if matches := re.FindStringSubmatch(cleaned); len(matches) > 1 {
			return matches[1]
		}
		// const name = (...) =>
		reArrow := regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(?`)
		if matches := reArrow.FindStringSubmatch(cleaned); len(matches) > 1 {
			return matches[1]
		}
	}

	// Fallback: try to find the word before the first parenthesis
	idx := strings.Index(cleaned, "(")
	if idx > 0 {
		sub := cleaned[:idx]
		parts := strings.Fields(sub)
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	return ""
}
