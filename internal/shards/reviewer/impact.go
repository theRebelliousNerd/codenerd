package reviewer

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// IMPACT-AWARE CONTEXT BUILDER
// =============================================================================
// Builds review context outward from the change using Mangle impact queries,
// not whole files. This prevents context explosion while ensuring callers
// and callees of modified functions are included for accurate review.

// ImpactContext represents the targeted context built from impact analysis.
type ImpactContext struct {
	ModifiedFunctions []ModifiedFunction
	ImpactedCallers   []ImpactedCaller
	AffectedFiles     []string
	TotalDepth        int
}

// ModifiedFunction represents a function changed in the diff.
type ModifiedFunction struct {
	Name      string
	File      string
	StartLine int
	EndLine   int
	Body      string
}

// ImpactedCaller represents a caller affected by the modification.
type ImpactedCaller struct {
	Name     string
	File     string
	Body     string
	Depth    int // 1 = direct caller, 2 = caller of caller, etc.
	Priority int // From context_priority query (higher = more important)
}

// maxImpactedCallers limits context size to prevent prompt explosion.
const maxImpactedCallers = 10

// maxFunctionBodyLines limits individual function body size.
const maxFunctionBodyLines = 50

// BuildImpactContext queries Mangle for impact radius and builds targeted context.
// This method:
// 1. Asserts modified_function facts to the kernel
// 2. Queries for impacted callers using context_priority rules
// 3. Fetches only the affected function bodies, not whole files
// 4. Returns a prioritized, size-limited context for LLM injection
func (r *ReviewerShard) BuildImpactContext(ctx context.Context, modifiedFuncs []ModifiedFunction) (*ImpactContext, error) {
	if r.kernel == nil {
		return nil, fmt.Errorf("kernel not initialized")
	}

	impact := &ImpactContext{
		ModifiedFunctions: modifiedFuncs,
		AffectedFiles:     make([]string, 0),
	}

	if len(modifiedFuncs) == 0 {
		return impact, nil
	}

	logging.ReviewerDebug("BuildImpactContext: analyzing impact of %d modified functions", len(modifiedFuncs))

	// 1. Assert modified_function facts to kernel for rule derivation
	for _, mf := range modifiedFuncs {
		fact := core.Fact{
			Predicate: "modified_function",
			Args:      []interface{}{mf.Name, mf.File},
		}
		if err := r.kernel.Assert(fact); err != nil {
			logging.ReviewerDebug("Failed to assert modified_function(%s, %s): %v", mf.Name, mf.File, err)
		}
	}

	// 2. Query for impacted callers with priorities
	// Try context_priority_file first (priority-aware)
	results, err := r.kernel.Query("context_priority_file")
	if err != nil || len(results) == 0 {
		// Fall back to relevant_context_file (simpler query)
		logging.ReviewerDebug("context_priority_file query failed or empty, trying relevant_context_file")
		results, err = r.kernel.Query("relevant_context_file")
		if err != nil {
			logging.ReviewerDebug("relevant_context_file query failed: %v", err)
			// Fall back to code_calls for direct callers
			results, err = r.queryDirectCallers(modifiedFuncs)
			if err != nil {
				return impact, nil
			}
		}
	}

	// 3. Parse results and fetch function bodies
	seen := make(map[string]bool)
	for _, result := range results {
		caller := r.parseImpactedCallerFromFact(result)
		if caller == nil {
			continue
		}

		key := fmt.Sprintf("%s:%s", caller.File, caller.Name)
		if seen[key] {
			continue
		}
		seen[key] = true

		// Fetch only the function body, not the whole file
		body, err := r.fetchFunctionBody(ctx, caller.File, caller.Name)
		if err != nil {
			logging.ReviewerDebug("Could not fetch body for %s: %v", key, err)
			continue
		}
		caller.Body = body
		impact.ImpactedCallers = append(impact.ImpactedCallers, *caller)

		if !containsString(impact.AffectedFiles, caller.File) {
			impact.AffectedFiles = append(impact.AffectedFiles, caller.File)
		}
	}

	// 4. Sort by priority (highest first) and depth (closest first)
	sort.Slice(impact.ImpactedCallers, func(i, j int) bool {
		if impact.ImpactedCallers[i].Priority != impact.ImpactedCallers[j].Priority {
			return impact.ImpactedCallers[i].Priority > impact.ImpactedCallers[j].Priority
		}
		return impact.ImpactedCallers[i].Depth < impact.ImpactedCallers[j].Depth
	})

	// 5. Limit to prevent context explosion
	if len(impact.ImpactedCallers) > maxImpactedCallers {
		logging.ReviewerDebug("Limiting impacted callers from %d to %d", len(impact.ImpactedCallers), maxImpactedCallers)
		impact.ImpactedCallers = impact.ImpactedCallers[:maxImpactedCallers]
	}

	// Calculate max depth
	for _, caller := range impact.ImpactedCallers {
		if caller.Depth > impact.TotalDepth {
			impact.TotalDepth = caller.Depth
		}
	}

	logging.ReviewerDebug("BuildImpactContext: found %d impacted callers across %d files (max depth: %d)",
		len(impact.ImpactedCallers), len(impact.AffectedFiles), impact.TotalDepth)

	return impact, nil
}

// queryDirectCallers falls back to code_calls for direct caller lookup.
func (r *ReviewerShard) queryDirectCallers(modifiedFuncs []ModifiedFunction) ([]core.Fact, error) {
	callFacts, err := r.kernel.Query("code_calls")
	if err != nil {
		return nil, fmt.Errorf("code_calls query failed: %w", err)
	}

	// Build set of modified function names
	modified := make(map[string]bool)
	for _, mf := range modifiedFuncs {
		modified[mf.Name] = true
	}

	// Find callers of modified functions
	var results []core.Fact
	for _, fact := range callFacts {
		if len(fact.Args) < 2 {
			continue
		}
		callee, _ := fact.Args[1].(string)
		if modified[callee] {
			results = append(results, fact)
		}
	}

	return results, nil
}

// parseImpactedCallerFromFact extracts caller info from a Mangle fact.
// Handles multiple fact formats:
// - context_priority_file(File, Func, Priority)
// - relevant_context_file(File)
// - code_calls(Caller, Callee)
func (r *ReviewerShard) parseImpactedCallerFromFact(fact core.Fact) *ImpactedCaller {
	caller := &ImpactedCaller{
		Depth:    1,
		Priority: 50, // Default medium priority
	}

	switch fact.Predicate {
	case "context_priority_file":
		if len(fact.Args) < 3 {
			return nil
		}
		caller.File, _ = fact.Args[0].(string)
		caller.Name, _ = fact.Args[1].(string)
		if priority, ok := fact.Args[2].(int); ok {
			caller.Priority = priority
		} else if priorityAtom, ok := fact.Args[2].(string); ok {
			caller.Priority = priorityAtomToInt(priorityAtom)
		}

	case "relevant_context_file":
		if len(fact.Args) < 1 {
			return nil
		}
		caller.File, _ = fact.Args[0].(string)
		// Extract function name from code_defines if possible
		caller.Name = "" // Will be filled by fetchFunctionBody

	case "code_calls":
		if len(fact.Args) < 2 {
			return nil
		}
		callerRef, _ := fact.Args[0].(string)
		// Parse caller reference (format: "pkg.Func" or "File:Func")
		caller.Name, caller.File = parseCallerRef(callerRef)

	default:
		// Unknown format, try to extract what we can
		if len(fact.Args) >= 2 {
			caller.File, _ = fact.Args[0].(string)
			caller.Name, _ = fact.Args[1].(string)
		} else if len(fact.Args) >= 1 {
			caller.File, _ = fact.Args[0].(string)
		} else {
			return nil
		}
	}

	if caller.File == "" {
		return nil
	}

	return caller
}

// priorityAtomToInt converts Mangle priority atoms to integers.
func priorityAtomToInt(atom string) int {
	switch strings.ToLower(strings.TrimPrefix(atom, "/")) {
	case "high", "critical":
		return 100
	case "medium", "normal":
		return 50
	case "low":
		return 25
	default:
		return 50
	}
}

// parseCallerRef parses a caller reference into name and file.
func parseCallerRef(ref string) (name, file string) {
	// Handle "pkg.Func" format
	if idx := strings.LastIndex(ref, "."); idx != -1 {
		name = ref[idx+1:]
		// Package is not the file, leave file empty for lookup
	}
	// Handle "File:Func" format
	if idx := strings.Index(ref, ":"); idx != -1 {
		file = ref[:idx]
		name = ref[idx+1:]
	}
	// Handle plain function name
	if name == "" {
		name = ref
	}
	return name, file
}

// fetchFunctionBody retrieves just the function body from a file.
// Uses AST parsing for Go files, falls back to regex for others.
func (r *ReviewerShard) fetchFunctionBody(ctx context.Context, file, funcName string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("empty file path")
	}

	// Read file content
	content, err := r.readFile(ctx, file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// For Go files, use AST parsing for accuracy
	if strings.HasSuffix(file, ".go") {
		return r.extractGoFunctionBody(content, funcName)
	}

	// For other languages, use regex-based extraction
	return r.extractFunctionBodyRegex(content, funcName)
}

// extractGoFunctionBody uses Go's AST parser to extract a function body.
func (r *ReviewerShard) extractGoFunctionBody(content, funcName string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("failed to parse Go file: %w", err)
	}

	var targetFunc *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == funcName {
				targetFunc = fn
				return false // Stop searching
			}
		}
		return true
	})

	if targetFunc == nil {
		return "", fmt.Errorf("function %s not found", funcName)
	}

	// Extract position information
	startLine := fset.Position(targetFunc.Pos()).Line
	endLine := fset.Position(targetFunc.End()).Line

	// Extract lines from content
	return extractLineRange(content, startLine, endLine, maxFunctionBodyLines)
}

// extractFunctionBodyRegex uses regex to extract function bodies from non-Go files.
func (r *ReviewerShard) extractFunctionBodyRegex(content, funcName string) (string, error) {
	if funcName == "" {
		return "", fmt.Errorf("empty function name")
	}

	// Common function patterns for various languages
	patterns := []string{
		// Go-style: func Name(...)
		fmt.Sprintf(`(?m)^func\s+(\([^)]*\)\s+)?%s\s*\(`, regexp.QuoteMeta(funcName)),
		// Python-style: def name(...)
		fmt.Sprintf(`(?m)^def\s+%s\s*\(`, regexp.QuoteMeta(funcName)),
		// JavaScript/TypeScript: function name(...) or name(...) =>
		fmt.Sprintf(`(?m)(function\s+%s|%s\s*[:=]\s*(async\s+)?(\([^)]*\)|[^=])\s*=>)`, regexp.QuoteMeta(funcName), regexp.QuoteMeta(funcName)),
		// Java/C#: type name(...)
		fmt.Sprintf(`(?m)(public|private|protected)?\s*\w+\s+%s\s*\(`, regexp.QuoteMeta(funcName)),
	}

	lines := strings.Split(content, "\n")
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}

		for i, line := range lines {
			if re.MatchString(line) {
				// Found function start, extract body
				endLine := findFunctionEnd(lines, i)
				return extractLineRange(content, i+1, endLine+1, maxFunctionBodyLines)
			}
		}
	}

	return "", fmt.Errorf("function %s not found with regex patterns", funcName)
}

// findFunctionEnd finds the end of a function by tracking brace depth.
func findFunctionEnd(lines []string, startIdx int) int {
	depth := 0
	inFunction := false

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			if ch == '{' {
				depth++
				inFunction = true
			} else if ch == '}' {
				depth--
				if inFunction && depth == 0 {
					return i
				}
			}
		}
	}

	// If no matching brace found, return a reasonable range
	return startIdx + maxFunctionBodyLines
}

// extractLineRange extracts a range of lines from content with truncation.
func extractLineRange(content string, startLine, endLine, maxLines int) (string, error) {
	lines := strings.Split(content, "\n")

	// Convert to 0-indexed
	startIdx := startLine - 1
	endIdx := endLine

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}
	// endLine is inclusive; require at least one full line of range.
	if endLine <= startLine || startIdx >= endIdx {
		return "", fmt.Errorf("invalid line range: %d-%d", startLine, endLine)
	}

	// Apply max lines limit
	lineCount := endIdx - startIdx
	truncated := false
	if lineCount > maxLines {
		endIdx = startIdx + maxLines
		truncated = true
	}

	result := strings.Join(lines[startIdx:endIdx], "\n")
	if truncated {
		result += "\n// ... (truncated)"
	}

	return result, nil
}

// ParseModifiedFunctionsFromDiff extracts modified functions from a git diff.
// Parses unified diff format to identify:
// 1. Files that were changed
// 2. Line ranges that were modified
// 3. Function names at those locations (via @@ hunk headers)
func ParseModifiedFunctionsFromDiff(diff string) []ModifiedFunction {
	var functions []ModifiedFunction
	seenFuncs := make(map[string]bool)

	var currentFile string
	scanner := bufio.NewScanner(strings.NewReader(diff))

	// Regex patterns
	diffFilePattern := regexp.MustCompile(`^diff --git a/(.+) b/`)
	hunkHeaderPattern := regexp.MustCompile(`^@@\s+-(\d+)(?:,\d+)?\s+\+(\d+)(?:,\d+)?\s+@@\s*(.*)`)
	funcNamePattern := regexp.MustCompile(`func\s+(?:\([^)]*\)\s+)?(\w+)\s*\(`)

	for scanner.Scan() {
		line := scanner.Text()

		// Track current file
		if matches := diffFilePattern.FindStringSubmatch(line); len(matches) > 1 {
			currentFile = matches[1]
			continue
		}

		// Parse hunk headers for function context
		if matches := hunkHeaderPattern.FindStringSubmatch(line); len(matches) > 3 {
			startLine := parseInt(matches[2], 0)
			funcContext := matches[3]

			// Try to extract function name from hunk context
			var funcName string
			if funcMatches := funcNamePattern.FindStringSubmatch(funcContext); len(funcMatches) > 1 {
				funcName = funcMatches[1]
			}

			// If we have a function name, record it
			if funcName != "" && currentFile != "" {
				key := fmt.Sprintf("%s:%s", currentFile, funcName)
				if !seenFuncs[key] {
					seenFuncs[key] = true
					functions = append(functions, ModifiedFunction{
						Name:      funcName,
						File:      currentFile,
						StartLine: startLine,
					})
				}
			}
		}
	}

	logging.ReviewerDebug("ParseModifiedFunctionsFromDiff: found %d modified functions", len(functions))
	return functions
}

// parseInt safely parses an integer with a default.
func parseInt(s string, def int) int {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return def
	}
	return result
}

// FormatForPrompt formats the impact context for LLM injection.
// Creates a structured markdown section that can be appended to review prompts.
func (ic *ImpactContext) FormatForPrompt() string {
	if ic == nil || (len(ic.ModifiedFunctions) == 0 && len(ic.ImpactedCallers) == 0) {
		return ""
	}

	var sb strings.Builder

	sb.WriteString("\n## Impact Analysis Context\n\n")

	// Modified functions summary
	if len(ic.ModifiedFunctions) > 0 {
		sb.WriteString("### Modified Functions\n")
		sb.WriteString("The following functions were changed in this diff:\n")
		for _, mf := range ic.ModifiedFunctions {
			sb.WriteString(fmt.Sprintf("- `%s` in `%s`", mf.Name, mf.File))
			if mf.StartLine > 0 {
				sb.WriteString(fmt.Sprintf(" (line %d)", mf.StartLine))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Impacted callers (the key context)
	if len(ic.ImpactedCallers) > 0 {
		sb.WriteString("### Impacted Callers\n")
		sb.WriteString("Review these callers to ensure changes don't break them:\n\n")

		for i, caller := range ic.ImpactedCallers {
			sb.WriteString(fmt.Sprintf("#### %d. `%s` ", i+1, caller.Name))
			if caller.File != "" {
				sb.WriteString(fmt.Sprintf("(%s)", caller.File))
			}
			sb.WriteString("\n")

			if caller.Priority >= 80 {
				sb.WriteString("**Priority: HIGH** - Critical caller\n")
			} else if caller.Priority >= 50 {
				sb.WriteString("*Priority: Medium*\n")
			}

			if caller.Depth > 1 {
				sb.WriteString(fmt.Sprintf("Depth: %d hops from modified function\n", caller.Depth))
			}

			if caller.Body != "" {
				sb.WriteString("```go\n")
				sb.WriteString(caller.Body)
				sb.WriteString("\n```\n\n")
			}
		}
	}

	// Summary stats
	sb.WriteString(fmt.Sprintf("**Impact Summary:** %d modified functions, %d impacted callers, %d files affected\n",
		len(ic.ModifiedFunctions), len(ic.ImpactedCallers), len(ic.AffectedFiles)))

	return sb.String()
}

// FormatCompact returns a compact summary for logging or brief context.
func (ic *ImpactContext) FormatCompact() string {
	if ic == nil {
		return "No impact context"
	}

	callerNames := make([]string, 0, len(ic.ImpactedCallers))
	for _, c := range ic.ImpactedCallers {
		callerNames = append(callerNames, c.Name)
	}

	return fmt.Sprintf("Modified: %d funcs, Impacted: %v, Files: %d",
		len(ic.ModifiedFunctions), callerNames, len(ic.AffectedFiles))
}

// containsString checks if a slice contains a string (local helper).
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
