package world

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// =============================================================================
// IMPACT-AWARE CONTEXT BUILDING
// =============================================================================
// These methods integrate Mangle's impact analysis with holographic context,
// providing prioritized caller information for targeted code review.

// maxPrioritizedCallers limits the number of callers included to prevent prompt explosion.
const maxPrioritizedCallers = 10

// maxCallerBodyLines limits individual caller body size.
const maxCallerBodyLines = 50

// queryImpactPriorities returns the kernel's impact-ranked callers, without
// fetching any function bodies.
//
// This is the half of the impact analysis the prompt path needs.
// PromptSection renders caller names, files, priorities and depths — never
// bodies — so making it pay for up to ten file reads and AST parses would put
// I/O on the turn's critical path for text that is discarded.
//
// Returns nil when there is no kernel, when neither impact predicate is
// derivable, or when the analysis found nothing. All three are ordinary: the
// chain only produces facts after something has actually been modified.
func (h *HolographicProvider) queryImpactPriorities(ctx context.Context) []PrioritizedCaller {
	if h == nil || h.kernel == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return nil
	}

	// context_priority_file carries the depth-derived ranking;
	// relevant_context_file is the unranked fallback the same rules derive.
	priorityFacts, err := h.kernel.Query("context_priority_file")
	if err != nil {
		logging.WorldDebug("queryImpactPriorities: context_priority_file query failed: %v", err)
		priorityFacts, err = h.kernel.Query("relevant_context_file")
		if err != nil {
			logging.WorldDebug("queryImpactPriorities: relevant_context_file query also failed: %v", err)
			return nil
		}
	}
	if len(priorityFacts) == 0 {
		return nil
	}

	callers := h.parsePriorityFacts(priorityFacts)
	if len(callers) == 0 {
		return nil
	}
	return rankPrioritizedCallers(callers)
}

// applyImpactPriorities attaches ranked callers and the overall impact priority
// to a holographic context.
//
// Called from getContextInternal so that every consumer of holographic context
// — above all PromptSection — sees the ranking. Before 2026-09-09 nothing
// populated PrioritizedCallers on the path that feeds the prompt, so
// PromptSection's "### Callers (impact-prioritized)" branch was unreachable and
// every turn fell through to an unordered list of caller names.
func (h *HolographicProvider) applyImpactPriorities(ctx context.Context, hc *HolographicContext) {
	if hc == nil {
		return
	}
	callers := h.queryImpactPriorities(ctx)
	if len(callers) == 0 {
		return
	}

	maxPriority := 0
	for _, c := range callers {
		if c.Priority > maxPriority {
			maxPriority = c.Priority
		}
	}
	hc.PrioritizedCallers = callers
	hc.ImpactPriority = maxPriority

	logging.WorldDebug("applyImpactPriorities: %d prioritized callers (max priority: %d)",
		len(callers), maxPriority)
}

// BuildWithImpactPriorities builds holographic context enhanced with impact
// analysis from the kernel, including the body of each prioritized caller.
//
// The ranking itself now comes from GetContextWithContext, which every caller
// gets. What this adds is the bodies: it is for consumers that want to show or
// reason over the calling code, not just name it. PromptSection deliberately
// does not use it — see queryImpactPriorities.
func (h *HolographicProvider) BuildWithImpactPriorities(ctx context.Context, file string) (*HolographicContext, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	logging.WorldDebug("BuildWithImpactPriorities: starting for %s", filepath.Base(file))

	// Cancellable: this used to call the non-cancellable GetContext, so a
	// cancelled caller still paid for a full package parse.
	hc, err := h.GetContextWithContext(ctx, file)
	if err != nil {
		// Report cancellation as itself. A caller that cancelled wants
		// context.Canceled, not a build failure that happens to wrap it —
		// distinguishing "we gave up" from "the package would not parse" is
		// the difference between a retry and a bug report.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("failed to build base context: %w", err)
	}
	if len(hc.PrioritizedCallers) == 0 {
		return hc, nil
	}

	callers, err := h.ResolvePrioritizedCallers(ctx, hc.PrioritizedCallers)
	if err != nil {
		return hc, err
	}
	hc.PrioritizedCallers = callers

	return hc, nil
}

// impactPriorityToScale converts impact.mg's depth-encoded priority into the
// 0-100 scale the renderers bucket on, and returns the depth it encodes.
//
// impact.mg derives Priority = 4 - Depth over a bounded 3-level walk, so the
// only values it emits are 3 (direct caller), 2 and 1. A value outside that
// range is passed through unchanged with an unknown depth: other producers
// (context_priority/2 via priorityAtomToInt) already speak the 0-100 scale, and
// silently rescaling those would be the same class of bug in the other
// direction.
func impactPriorityToScale(raw int) (priority, depth int) {
	switch raw {
	case 3:
		return 100, 1
	case 2:
		return 70, 2
	case 1:
		return 40, 3
	case 0:
		// No priority argument parsed. Medium, unknown depth.
		return 50, 1
	default:
		return raw, 1
	}
}

// rankPrioritizedCallers sorts by priority then depth and caps the list.
//
// Split out from ResolvePrioritizedCallers because the prompt path wants the
// ranking and not the bodies: PromptSection renders names, files and priorities
// only, so fetching up to ten function bodies to build it would be file I/O on
// the turn's critical path for text that is never emitted.
func rankPrioritizedCallers(callers []PrioritizedCaller) []PrioritizedCaller {
	sort.SliceStable(callers, func(i, j int) bool {
		if callers[i].Priority != callers[j].Priority {
			return callers[i].Priority > callers[j].Priority
		}
		if callers[i].Depth != callers[j].Depth {
			return callers[i].Depth < callers[j].Depth
		}
		// Stable tiebreak so the same kernel state renders the same section
		// every turn; an order that shuffles costs prompt-cache hits.
		return callers[i].Name < callers[j].Name
	})

	if len(callers) > maxPrioritizedCallers {
		logging.WorldDebug("rankPrioritizedCallers: limiting callers from %d to %d",
			len(callers), maxPrioritizedCallers)
		callers = callers[:maxPrioritizedCallers]
	}
	return callers
}

// ResolvePrioritizedCallers sorts, limits, and fetches bodies for prioritized callers.
// It optimizes by sorting and limiting *before* fetching bodies to avoid unnecessary I/O.
func (h *HolographicProvider) ResolvePrioritizedCallers(ctx context.Context, callers []PrioritizedCaller) ([]PrioritizedCaller, error) {
	callers = rankPrioritizedCallers(callers)

	// Fetch function bodies for prioritized callers with caching
	cache := newFileContentCache()

	for i := range callers {
		select {
		case <-ctx.Done():
			return callers, ctx.Err()
		default:
		}

		body, fetchErr := h.fetchFunctionBody(callers[i].File, callers[i].Name, cache)
		if fetchErr != nil {
			logging.WorldDebug("ResolvePrioritizedCallers: could not fetch body for %s:%s: %v",
				callers[i].File, callers[i].Name, fetchErr)
			continue
		}
		callers[i].Body = body
	}

	return callers, nil
}

// parsePriorityFacts extracts PrioritizedCaller structs from Mangle query results.
// Handles multiple fact formats:
// - context_priority_file(File, Func, Priority)
// - relevant_context_file(File)
// - impact_graph(Target, Caller, Depth)
func (h *HolographicProvider) parsePriorityFacts(facts []core.Fact) []PrioritizedCaller {
	callers := make([]PrioritizedCaller, 0, len(facts))
	seen := make(map[string]bool)

	for _, fact := range facts {
		var caller PrioritizedCaller
		caller.Depth = 1     // Default depth
		caller.Priority = 50 // Default medium priority

		switch fact.Predicate {
		case "context_priority_file":
			// Format: context_priority_file(File, Func, Priority)
			//
			// impact.mg emits Priority as 4 - Depth, so the values are 3, 2, 1
			// for a direct caller, a grandcaller and a great-grandcaller
			// (impact.mg:68-78). Every Go consumer here buckets on 80/50/25
			// (priorityLevelString, FormatWithPriorities), so an unconverted 3
			// rendered as MINIMAL and a direct caller looked less important
			// than the default 50 assigned to facts carrying no priority at
			// all. Convert into the 0-100 scale the renderers speak, and
			// recover the depth the priority encodes rather than leaving the
			// hardcoded 1.
			if len(fact.Args) < 3 {
				continue
			}
			caller.File = h.stringArg(fact.Args[0])
			caller.Name = h.stringArg(fact.Args[1])
			caller.Priority, caller.Depth = impactPriorityToScale(h.intArg(fact.Args[2], 0))

		case "relevant_context_file":
			// Format: relevant_context_file(File)
			if len(fact.Args) < 1 {
				continue
			}
			caller.File = h.stringArg(fact.Args[0])
			// Name will be discovered when fetching body

		case "impact_graph":
			// Format: impact_graph(Target, Caller, Depth)
			if len(fact.Args) < 3 {
				continue
			}
			caller.Name = h.stringArg(fact.Args[1])
			caller.Depth = h.intArg(fact.Args[2], 1)
			// File will need to be looked up from code_defines

		case "context_priority":
			// Format: context_priority(FactID, Priority)
			if len(fact.Args) < 2 {
				continue
			}
			caller.File = h.stringArg(fact.Args[0])
			caller.Priority = h.priorityAtomToInt(h.stringArg(fact.Args[1]))

		default:
			// Generic fallback: try to extract file and function
			if len(fact.Args) >= 2 {
				caller.File = h.stringArg(fact.Args[0])
				caller.Name = h.stringArg(fact.Args[1])
			} else if len(fact.Args) >= 1 {
				caller.File = h.stringArg(fact.Args[0])
			} else {
				continue
			}
		}

		// Skip if we don't have at least a file
		if caller.File == "" {
			continue
		}

		// Skip if the function name is empty for predicates expecting it
		if fact.Predicate != "relevant_context_file" && caller.Name == "" {
			continue
		}

		// Deduplicate by file:name key
		key := fmt.Sprintf("%s:%s", caller.File, caller.Name)
		if seen[key] {
			continue
		}
		seen[key] = true

		callers = append(callers, caller)
	}

	return callers
}

// stringArg safely extracts a string from an interface{} argument.
func (h *HolographicProvider) stringArg(arg any) string {
	switch v := arg.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// intArg safely extracts an int from an interface{} argument.
func (h *HolographicProvider) intArg(arg any, defaultVal int) int {
	switch v := arg.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		// Try to parse as an integer first
		if val, err := strconv.Atoi(v); err == nil {
			return val
		}
		// Try to parse priority atoms
		return h.priorityAtomToInt(v)
	default:
		return defaultVal
	}
}

// priorityAtomToInt converts Mangle priority atoms to integer values.
func (h *HolographicProvider) priorityAtomToInt(atom string) int {
	// Strip leading / for Mangle name constants
	atom = strings.TrimPrefix(atom, "/")
	atom = strings.ToLower(atom)

	switch atom {
	case "critical", "highest":
		return 100
	case "high":
		return 80
	case "medium", "normal":
		return 50
	case "low":
		return 25
	case "lowest":
		return 10
	default:
		return 50 // Default medium
	}
}

// fileContentCache stores file contents and parsed ASTs to avoid redundant I/O and parsing.
type fileContentCache struct {
	contents map[string]string
	asts     map[string]*ast.File
	fsets    map[string]*token.FileSet
}

func newFileContentCache() *fileContentCache {
	return &fileContentCache{
		contents: make(map[string]string),
		asts:     make(map[string]*ast.File),
		fsets:    make(map[string]*token.FileSet),
	}
}

// fetchFunctionBody retrieves the body of a function from a file.
// Uses AST parsing for Go files, falls back to regex for other languages.
func (h *HolographicProvider) fetchFunctionBody(file, funcName string, cache *fileContentCache) (string, error) {
	if file == "" {
		return "", fmt.Errorf("empty file path")
	}

	// Resolve relative paths against workDir and verify workspace bounds (security check)
	resolvedPath := file
	if h.workDir != "" {
		cleanWorkDir := filepath.Clean(h.workDir)
		absPath := file
		if !filepath.IsAbs(file) {
			absPath = filepath.Join(cleanWorkDir, file)
		} else {
			absPath = filepath.Clean(file)
		}

		// Verify that absPath has cleanWorkDir as prefix to block path traversal
		rel, relErr := filepath.Rel(cleanWorkDir, absPath)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("security violation: path traversal detected: %s is outside workspace %s", file, h.workDir)
		}
		resolvedPath = absPath
	} else if !filepath.IsAbs(file) {
		return "", fmt.Errorf("cannot resolve relative path %s with empty workDir", file)
	}

	var content string

	if cache != nil {
		if c, ok := cache.contents[resolvedPath]; ok {
			content = c
		}
	}

	if content == "" {
		info, statErr := os.Stat(resolvedPath)
		if statErr != nil {
			return "", fmt.Errorf("failed to stat file %s: %w", resolvedPath, statErr)
		}
		if info.Size() > 5*1024*1024 { // 5MB limit
			return "", fmt.Errorf("file too large: %s (%d bytes)", resolvedPath, info.Size())
		}

		b, err := os.ReadFile(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read file %s: %w", resolvedPath, err)
		}
		content = string(b)
		if cache != nil {
			cache.contents[resolvedPath] = content
		}
	}

	// For Go files, use AST parsing
	if strings.HasSuffix(file, ".go") {
		return h.extractGoFunctionBody(content, funcName, resolvedPath, cache)
	}

	// For other files, use regex-based extraction
	return h.extractFunctionBodyRegex(content, funcName)
}

// extractGoFunctionBody uses Go's AST parser to extract a function body.
func (h *HolographicProvider) extractGoFunctionBody(content, funcName, file string, cache *fileContentCache) (string, error) {
	if funcName == "" {
		return "", fmt.Errorf("empty function name")
	}

	var node *ast.File
	var fset *token.FileSet
	var err error

	if cache != nil {
		if n, ok := cache.asts[file]; ok {
			node = n
			fset = cache.fsets[file]
		}
	}

	if node == nil {
		fset = token.NewFileSet()
		node, err = parser.ParseFile(fset, "", content, parser.ParseComments)
		if err != nil {
			return "", fmt.Errorf("failed to parse Go file: %w", err)
		}
		if cache != nil {
			cache.asts[file] = node
			cache.fsets[file] = fset
		}
	}

	var targetFunc *ast.FuncDecl
	ast.Inspect(node, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			if fn.Name.Name == funcName {
				targetFunc = fn
				return false
			}
		}
		return true
	})

	if targetFunc == nil {
		return "", fmt.Errorf("function %s not found", funcName)
	}

	startLine := fset.Position(targetFunc.Pos()).Line
	endLine := fset.Position(targetFunc.End()).Line

	return h.extractLineRange(content, startLine, endLine)
}

var globalFunctionPatterns = []*regexp.Regexp{
	// Go: func Name(...)
	regexp.MustCompile(`^func\s+(?:\([^)]*\)\s+)?([^\s(]+)\s*\(`),
	// Python: def name(...)
	regexp.MustCompile(`^def\s+([^\s(]+)\s*\(`),
	// JavaScript/TypeScript: function name(...) or name(...) =>
	regexp.MustCompile(`(?:function\s+([^\s(]+)|([^\s(=:]+)\s*[:=]\s*(?:async\s+)?(?:\([^)]*\)|[^=])\s*=>)`),
	// Java/C#: modifier type name(...)
	regexp.MustCompile(`(?:public|private|protected)?\s*\w+\s+([^\s(]+)\s*\(`),
}

// extractFunctionBodyRegex uses regex to find function bodies in non-Go files.
func (h *HolographicProvider) extractFunctionBodyRegex(content, funcName string) (string, error) {
	if funcName == "" {
		return "", fmt.Errorf("empty function name")
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Fast path: skip lines that don't contain the function name
		if !strings.Contains(line, funcName) {
			continue
		}
		for _, re := range globalFunctionPatterns {
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				for j := 1; j < len(matches); j++ {
					if matches[j] == funcName {
						endLine := h.findFunctionEnd(lines, i)
						return h.extractLineRange(content, i+1, endLine+1)
					}
				}
			}
		}
	}

	return "", fmt.Errorf("function %s not found with regex patterns", funcName)
}

// findFunctionEnd finds the closing brace of a function by tracking depth.
func (h *HolographicProvider) findFunctionEnd(lines []string, startIdx int) int {
	depth := 0
	inFunction := false
	inBlockComment := false
	inString := rune(0) // 0 if not in string, else the quote char: '"', '\'', '`'
	inTripleString := rune(0)

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		lineRunes := []rune(line)

		for j := 0; j < len(lineRunes); j++ {
			ch := lineRunes[j]

			// Handle block comment content
			if inBlockComment {
				if ch == '*' && j+1 < len(lineRunes) && lineRunes[j+1] == '/' {
					inBlockComment = false
					j++ // skip /
				}
				continue
			}

			// Handle triple string content
			if inTripleString != 0 {
				if ch == inTripleString && j+2 < len(lineRunes) && lineRunes[j+1] == inTripleString && lineRunes[j+2] == inTripleString {
					// Check for escape
					backslashes := 0
					for k := j - 1; k >= 0; k-- {
						if lineRunes[k] != '\\' {
							break
						}
						backslashes++
					}
					if backslashes%2 == 0 {
						inTripleString = 0
						j += 2 // skip the other two quotes
					}
				}
				continue
			}

			// Handle string/char literal content
			if inString != 0 {
				if ch == inString {
					// Check for escape
					// Count consecutive backslashes preceding this quote
					backslashes := 0
					for k := j - 1; k >= 0; k-- {
						if lineRunes[k] != '\\' {
							break
						}
						backslashes++
					}
					// If even number of backslashes (0, 2...), the quote is NOT escaped
					if backslashes%2 == 0 {
						inString = 0
					}
				}
				continue
			}

			// Start of block comment
			if ch == '/' && j+1 < len(lineRunes) && lineRunes[j+1] == '*' {
				inBlockComment = true
				j++ // skip *
				continue
			}

			// Start of line comment
			if ch == '/' && j+1 < len(lineRunes) && lineRunes[j+1] == '/' {
				break // ignore rest of line
			}

			// Start of triple string
			if (ch == '"' || ch == '\'') && j+2 < len(lineRunes) && lineRunes[j+1] == ch && lineRunes[j+2] == ch {
				inTripleString = ch
				j += 2
				continue
			}

			// Start of string/char literal
			if ch == '"' || ch == '\'' || ch == '`' {
				inString = ch
				continue
			}

			// Brace counting
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

		// Reset string state ONLY for single-line quotes (" and ')
		// Backticks (`) span multiple lines.
		if inString == '"' || inString == '\'' {
			inString = 0
		}
	}

	// Fallback: return a reasonable range
	endIdx := min(startIdx+maxCallerBodyLines, len(lines)-1)
	return endIdx
}

// extractLineRange extracts lines from content with truncation.
func (h *HolographicProvider) extractLineRange(content string, startLine, endLine int) (string, error) {
	lines := strings.Split(content, "\n")

	startIdx := startLine - 1
	endIdx := endLine

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}
	if startIdx >= endIdx {
		return "", fmt.Errorf("invalid line range: %d-%d", startLine, endLine)
	}

	// Apply max lines limit
	lineCount := endIdx - startIdx
	truncated := false
	if lineCount > maxCallerBodyLines {
		endIdx = startIdx + maxCallerBodyLines
		truncated = true
	}

	result := strings.Join(lines[startIdx:endIdx], "\n")
	if truncated {
		result += "\n// ... (truncated)"
	}

	return result, nil
}

// FormatWithPriorities formats the holographic context with priority annotations.
// This produces a markdown-formatted string optimized for LLM injection.
func (hc *HolographicContext) FormatWithPriorities() string {
	if hc == nil {
		return ""
	}

	var sb strings.Builder

	// Include standard context first
	sb.WriteString(hc.FormatForPrompt())

	// Add prioritized callers section if present
	if len(hc.PrioritizedCallers) == 0 {
		return sb.String()
	}

	sb.WriteString("\n## Impact-Prioritized Context\n\n")
	sb.WriteString(fmt.Sprintf("Overall Impact Priority: %s\n\n",
		priorityLevelString(hc.ImpactPriority)))

	sb.WriteString("### Prioritized Callers\n")
	sb.WriteString("These functions call into the target code, sorted by impact priority:\n\n")

	for i, caller := range hc.PrioritizedCallers {
		sb.WriteString(fmt.Sprintf("#### %d. `%s`", i+1, caller.Name))
		if caller.File != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", filepath.Base(caller.File)))
		}
		sb.WriteString("\n")

		// Priority indicator
		switch {
		case caller.Priority >= 80:
			sb.WriteString("**Priority: HIGH** - Critical impact path\n")
		case caller.Priority >= 50:
			sb.WriteString("*Priority: Medium*\n")
		default:
			sb.WriteString("Priority: Low\n")
		}

		if caller.Depth > 1 {
			sb.WriteString(fmt.Sprintf("Call depth: %d hops from target\n", caller.Depth))
		}

		if caller.Body != "" {
			sb.WriteString("```go\n")
			sb.WriteString(caller.Body)
			if !strings.HasSuffix(caller.Body, "\n") {
				sb.WriteString("\n")
			}
			sb.WriteString("```\n\n")
		} else {
			sb.WriteString("(body not available)\n\n")
		}
	}

	sb.WriteString(fmt.Sprintf("**Summary:** %d prioritized callers included\n",
		len(hc.PrioritizedCallers)))

	return sb.String()
}

// FormatPrioritizedCallersCompact returns a compact list of prioritized callers.
func (hc *HolographicContext) FormatPrioritizedCallersCompact() string {
	if hc == nil || len(hc.PrioritizedCallers) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Prioritized callers:\n")

	for _, caller := range hc.PrioritizedCallers {
		priorityMark := ""
		if caller.Priority >= 80 {
			priorityMark = "[HIGH] "
		} else if caller.Priority >= 50 {
			priorityMark = "[MED] "
		} else {
			priorityMark = "[LOW] "
		}

		sb.WriteString(fmt.Sprintf("  %s%s", priorityMark, caller.Name))
		if caller.File != "" {
			sb.WriteString(fmt.Sprintf(" [%s]", filepath.Base(caller.File)))
		}
		if caller.Depth > 1 {
			sb.WriteString(fmt.Sprintf(" (depth=%d)", caller.Depth))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// priorityLevelString converts a numeric priority to a human-readable level.
func priorityLevelString(priority int) string {
	switch {
	case priority >= 90:
		return "CRITICAL"
	case priority >= 80:
		return "HIGH"
	case priority >= 50:
		return "MEDIUM"
	case priority >= 25:
		return "LOW"
	default:
		return "MINIMAL"
	}
}

// HasPrioritizedCallers returns true if the context has impact-prioritized callers.
func (hc *HolographicContext) HasPrioritizedCallers() bool {
	return hc != nil && len(hc.PrioritizedCallers) > 0
}

// GetHighPriorityCallers returns only callers with priority >= threshold.
func (hc *HolographicContext) GetHighPriorityCallers(threshold int) []PrioritizedCaller {
	if hc == nil || len(hc.PrioritizedCallers) == 0 {
		return nil
	}

	result := make([]PrioritizedCaller, 0)
	for _, caller := range hc.PrioritizedCallers {
		if caller.Priority >= threshold {
			result = append(result, caller)
		}
	}
	return result
}
