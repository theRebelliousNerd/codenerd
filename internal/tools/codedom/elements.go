package codedom

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
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
		Name:          "get_elements",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description:   "List code elements (functions, classes, methods) in a file",
		Category:      tools.CategoryCode,
		Priority:      80,
		Execute:       executeGetElements,
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	// get_elements/get_element read whatever path they are handed. The line
	// tools next door in lines.go were contained; these two were not, and they
	// return file contents (the Content field of every element), so an
	// uncontained read here is an arbitrary file disclosure with extra steps.
	path, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return "", err
	}

	filterType, _ := args["type"].(string)

	logging.ToolsDebug("get_elements: path=%s, type=%s", path, filterType)

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
	logging.Tools("get_elements completed: %s (%d elements)", path, len(elements))
	return string(output), nil
}

// extractCodeElements extracts code elements from a file using regex patterns.
// This is a simplified implementation - full AST parsing is done by VirtualStore.
func extractCodeElements(path string) ([]CodeElement, error) {
	data, err := projectdoc.ReadFileForTool(path)
	if err != nil {
		return nil, err
	}
	return ElementsFromSource(path, string(data)), nil
}

// ElementsFromSource extracts code elements from source text that the caller
// already holds.
//
// It is separate from extractCodeElements so that a caller which has just read
// a file for another reason does not read it a second time, and — more
// importantly — so that a caller can decide for itself which bytes are
// analysed. The observation codec needs the second property: it projects the
// bytes it observed, and a helper that always went back to disk would let the
// projection describe a file that had changed since the search ran.
func ElementsFromSource(path, content string) []CodeElement {
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

	// Pattern names iterate sorted: two patterns can match one line (the
	// C++ function pattern is broad enough to fire alongside class/struct),
	// and map order would shuffle those elements run to run.
	typeNames := make([]string, 0, len(patterns))
	for elemType := range patterns {
		typeNames = append(typeNames, elemType)
	}
	sort.Strings(typeNames)

	isPy := ext == "py"
	_, hasMethod := patterns["method"]
	isBrace := hasMethod && ext != "go" && ext != "py"

	type pyScope struct {
		name    string
		indent  int
		isClass bool
	}
	var pyStack []pyScope
	braceDepth := 0
	type braceScope struct {
		name  string
		depth int
	}
	var braceStack []braceScope
	pyClassRe := regexp.MustCompile(`^\s*class\s+(\w+)`)
	pyDefRe := regexp.MustCompile(`^\s*def\s+(\w+)\s*\(`)
	braceClassRe := regexp.MustCompile(`^\s*(?:export\s+)?(?:public\s+)?(?:abstract\s+)?class\s+(\w+)`)

	var elements []CodeElement
	for idx, line := range lines {
		var pyEnclosing string
		if isPy {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				curIndent := indentLevel(line)
				for len(pyStack) > 0 && curIndent <= pyStack[len(pyStack)-1].indent {
					pyStack = pyStack[:len(pyStack)-1]
				}
				if len(pyStack) > 0 && pyStack[len(pyStack)-1].isClass {
					pyEnclosing = pyStack[len(pyStack)-1].name
				}
			}
		}
		if isBrace {
			for len(braceStack) > 0 && braceStack[len(braceStack)-1].depth >= braceDepth {
				braceStack = braceStack[:len(braceStack)-1]
			}
		}
		for _, elemType := range typeNames {
			pattern := patterns[elemType]
			if matches := pattern.FindStringSubmatch(line); matches != nil {
				startLine := idx + 1
				var endLine int
				if ext == "py" {
					endLine = findPythonEndLine(lines, idx)
				} else {
					endLine = findBraceEndLine(lines, idx)
				}
				name := matches[1]
				if ext == "go" && elemType == "method" {
					if recv := goReceiverBase(line); recv != "" {
						name = recv + "." + name
					}
				}
				if isPy && elemType == "method" && pyEnclosing != "" {
					name = pyEnclosing + "." + name
				}
				if isBrace && elemType == "method" && len(braceStack) > 0 {
					name = braceStack[len(braceStack)-1].name + "." + name
				}
				elements = append(elements, CodeElement{
					Name:      name,
					Type:      elemType,
					File:      path,
					StartLine: startLine,
					EndLine:   endLine,
					Signature: strings.TrimSpace(line),
				})
			}
		}
		if isPy {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				curIndent := indentLevel(line)
				if m := pyClassRe.FindStringSubmatch(line); m != nil {
					pyStack = append(pyStack, pyScope{name: m[1], indent: curIndent, isClass: true})
				} else if m := pyDefRe.FindStringSubmatch(line); m != nil {
					pyStack = append(pyStack, pyScope{name: m[1], indent: curIndent, isClass: false})
				}
			}
		}
		if isBrace {
			if m := braceClassRe.FindStringSubmatch(line); m != nil {
				braceStack = append(braceStack, braceScope{name: m[1], depth: braceDepth})
			}
			braceDepth += braceNetChange(line)
		}
	}
	return elements
}

func braceNetChange(line string) int {
	inSingle := false
	inDouble := false
	inBacktick := false
	escaped := false
	delta := 0
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if inSingle {
			if c == '\\' {
				escaped = true
			} else if c == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if c == '\\' {
				escaped = true
			} else if c == '`' {
				inBacktick = false
			}
			continue
		}
		switch c {
		case '\'':
			inSingle = true
		case '"':
			inDouble = true
		case '`':
			inBacktick = true
		case '/':
			if i+1 < len(line) && line[i+1] == '/' {
				i = len(line)
			}
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

// goReceiverBase extracts the receiver type base name from a Go method
// declaration line, e.g. "func (b *B) Close() error" yields "B". The second
// return reports whether a receiver was present. Pointer markers, generic
// instantiations ("Box[T]"), and package qualifiers ("pkg.T") are stripped so
// the qualifier stays a plain identifier that get_element can match.
func goReceiverBase(line string) string {
	base, _ := splitReceiverName(line)
	return base
}

// splitReceiverName parses the receiver out of a Go method declaration line.
// It returns the base type name and true when the line declares a method with
// a receiver; otherwise it returns "", false.
func splitReceiverName(line string) (string, bool) {
	open := strings.Index(line, "(")
	if open == -1 {
		return "", false
	}
	// The receiver is the first parenthesized group, and only when it appears
	// before the "func" keyword's argument list: a method declaration starts
	// with "func" followed by "(".
	rest := strings.TrimSpace(line[:open])
	if !strings.HasSuffix(rest, "func") {
		return "", false
	}
	close := strings.Index(line[open:], ")")
	if close == -1 {
		return "", false
	}
	recv := strings.TrimSpace(line[open+1 : open+close])
	if recv == "" {
		return "", false
	}
	fields := strings.Fields(recv)
	typeExpr := fields[len(fields)-1]
	typeExpr = strings.TrimPrefix(typeExpr, "*")
	if idx := strings.Index(typeExpr, "["); idx != -1 {
		typeExpr = typeExpr[:idx]
	}
	if idx := strings.LastIndex(typeExpr, "."); idx != -1 {
		typeExpr = typeExpr[idx+1:]
	}
	if typeExpr == "" {
		return "", false
	}
	return typeExpr, true
}

// normalizeReceiver canonicalizes a Go receiver reference so lookups agree
// with the base names get_elements stores: "B", "*B", "(B)", "(*B)",
// "pkg.B" and "Box[T]" all normalize to "B" (or "Box").
func normalizeReceiver(s string) string {
	s = strings.TrimSpace(s)
	for len(s) >= 2 && strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimSpace(s)
	for len(s) >= 2 && strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	s = strings.TrimPrefix(s, "*")
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "["); idx != -1 {
		s = s[:idx]
	}
	if idx := strings.LastIndex(s, "."); idx != -1 {
		s = s[idx+1:]
	}
	for len(s) >= 2 && strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	s = strings.TrimPrefix(s, "*")
	return strings.TrimSpace(s)
}

// parseElementRef splits a get_element query into receiver and method parts.
// Bare names ("Close") report qualified=false. Receiver-qualified names
// ("B.Close", "*B.Close", "(*B).Close", "pkg.B.Close") report the normalized
// receiver and method with qualified=true.
func parseElementRef(ref string) (recv, method string, qualified bool) {
	ref = strings.TrimSpace(ref)
	dot := strings.LastIndex(ref, ".")
	if dot == -1 {
		return "", ref, false
	}
	method = strings.TrimSpace(ref[dot+1:])
	recvPart := strings.TrimSpace(ref[:dot])
	if method == "" || recvPart == "" {
		return "", ref, false
	}
	if idx := strings.Index(method, "("); idx != -1 {
		method = strings.TrimSpace(method[:idx])
	}
	if method == "" {
		return "", ref, false
	}
	if idx := strings.LastIndex(recvPart, "."); idx != -1 {
		recvPart = strings.TrimSpace(recvPart[idx+1:])
	}
	recv = normalizeReceiver(recvPart)
	if recv == "" {
		return "", ref, false
	}
	return recv, method, true
}

// splitStoredName splits a stored element name into receiver and method
// parts. Names without a dot (functions, structs, bare names) report
// isMethod=false with the full name as method.
func splitStoredName(stored string) (recv, method string, isMethod bool) {
	dot := strings.LastIndex(stored, ".")
	if dot == -1 {
		return "", stored, false
	}
	recv = strings.TrimSpace(stored[:dot])
	method = strings.TrimSpace(stored[dot+1:])
	if recv == "" || method == "" {
		return "", stored, false
	}
	return recv, method, true
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
		Name:          "get_element",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description:   "Get a specific code element by name",
		Category:      tools.CategoryCode,
		Priority:      80,
		Execute:       executeGetElement,
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
	rawPath, _ := args["path"].(string)
	if rawPath == "" {
		return "", fmt.Errorf("path is required")
	}
	path, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return "", err
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	logging.ToolsDebug("get_element: path=%s, name=%s", path, name)

	elements, err := extractCodeElements(path)
	if err != nil {
		return "", fmt.Errorf("failed to extract elements: %w", err)
	}

	// Query may be bare ("Close") or receiver-qualified ("B.Close",
	// "(*B).Close", "*B.Close", "pkg.B.Close"). Parentheses, pointer
	// markers, generic arguments and package qualifiers are stripped so
	// every name get_elements shows is accepted here.
	qRecv, qMethod, qQualified := parseElementRef(name)
	var matches []CodeElement
	if qQualified {
		for _, e := range elements {
			eRecv, eMethod, eIsMethod := splitStoredName(e.Name)
			if !eIsMethod {
				continue
			}
			if eMethod != qMethod {
				continue
			}
			if normalizeReceiver(eRecv) != qRecv {
				if goReceiverBase(e.Signature) != qRecv {
					continue
				}
			}
			matches = append(matches, e)
		}
	} else {
		// Bare name: prefer exact full-name matches so a function stays
		// fetchable when a method shares its suffix (func Close vs A.Close).
		// Only when no exact match exists, fall back to method-suffix
		// matches so a lone method stays fetchable by its bare name.
		for _, e := range elements {
			if e.Name == qMethod {
				matches = append(matches, e)
			}
		}
		if len(matches) == 0 {
			for _, e := range elements {
				_, eMethod, eIsMethod := splitStoredName(e.Name)
				if !eIsMethod {
					continue
				}
				if eMethod != qMethod {
					continue
				}
				matches = append(matches, e)
			}
		}
	}
	if len(matches) == 1 {
		output, _ := json.MarshalIndent(matches[0], "", "  ")
		return string(output), nil
	}
	if len(matches) > 1 {
		qualified := make([]string, 0, len(matches))
		for _, e := range matches {
			qualified = append(qualified, e.Name)
		}
		sort.Strings(qualified)
		return "", fmt.Errorf("element %q is ambiguous (%d matches); use one of: %s", name, len(matches), strings.Join(qualified, ", "))
	}

	return "", fmt.Errorf("element not found: %s", name)
}
