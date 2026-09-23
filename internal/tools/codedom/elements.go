package codedom

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// CodeElement is one declaration as the read and search codecs and the
// holographic outline see it: a name, a kind and a line span.
type CodeElement struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // function, method, struct, interface, type, const, var, class, ...
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	// DeclLine is the line of the declaration keyword, after any doc comment
	// StartLine includes.
	DeclLine  int    `json:"decl_line"`
	Signature string `json:"signature,omitempty"`
}

// Regex extractors for the languages CodeDOM has no element model for. Go and
// Mangle never come through here: they are parsed (codemodel), and the four Go
// regexes that used to serve get_elements -- no body, no doc, no const or var
// -- are gone.
var (
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

// extractCodeElements reads a file and returns its declarations.
func extractCodeElements(path string) ([]CodeElement, error) {
	data, err := projectdoc.ReadFileForTool(path)
	if err != nil {
		return nil, err
	}
	return ElementsFromSource(path, string(data)), nil
}

// ElementsFromSource returns the declarations of source text the caller
// already holds, from the element model for Go and Mangle and from the
// per-language regexes otherwise.
//
// It takes the text rather than a path so the observation codec can project
// exactly the bytes it observed: a helper that went back to disk would let the
// projection describe a file that had changed since the search ran.
func ElementsFromSource(path, content string) []CodeElement {
	if f, ok := codemodel.Parse(path, content); ok {
		out := make([]CodeElement, 0, len(f.Elements))
		for _, e := range f.Elements {
			if e.Kind == codemodel.KindHeader || e.Kind == codemodel.KindSyntaxError {
				continue
			}
			out = append(out, CodeElement{
				Name: e.Key, Type: string(e.Kind), File: path,
				StartLine: e.StartLine, EndLine: e.EndLine, DeclLine: e.DeclLine,
				Signature: e.Signature,
			})
		}
		return out
	}
	return regexElements(path, content)
}

// regexElements is the line-pattern extractor for languages with no model.
func regexElements(path, content string) []CodeElement {
	var lines []string
	if content != "" {
		lines = strings.Split(content, "\n")
		// A trailing newline yields an empty last element that is not a line.
		if strings.HasSuffix(content, "\n") && len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for i, l := range lines {
			lines[i] = strings.TrimSuffix(l, "\r")
		}
	}

	ext := ""
	if dot := strings.LastIndex(path, "."); dot != -1 {
		ext = strings.ToLower(path[dot+1:])
	}

	var patterns map[string]*regexp.Regexp
	switch ext {
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
	isBrace := hasMethod && !isPy

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
			matches := patterns[elemType].FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			startLine := idx + 1
			endLine := findBraceEndLine(lines, idx)
			if isPy {
				endLine = findPythonEndLine(lines, idx)
			}
			name := matches[1]
			if isPy && elemType == "method" && pyEnclosing != "" {
				name = pyEnclosing + "." + name
			}
			if isBrace && elemType == "method" && len(braceStack) > 0 {
				name = braceStack[len(braceStack)-1].name + "." + name
			}
			elements = append(elements, CodeElement{
				Name: name, Type: elemType, File: path,
				StartLine: startLine, EndLine: endLine, DeclLine: startLine,
				Signature: strings.TrimSpace(line),
			})
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
	inSingle, inDouble, inBacktick, escaped := false, false, false, false
	delta := 0
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if inSingle || inDouble || inBacktick {
			switch {
			case c == '\\':
				escaped = true
			case inSingle && c == '\'':
				inSingle = false
			case inDouble && c == '"':
				inDouble = false
			case inBacktick && c == '`':
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
		j := 0
		for j < len(line) {
			c := line[j]
			switch {
			case inBlockComment:
				if c == '*' && j+1 < len(line) && line[j+1] == '/' {
					inBlockComment = false
					j += 2
					continue
				}
			case inSingle || inDouble:
				if c == '\\' {
					j += 2
					continue
				}
				if (inSingle && c == '\'') || (inDouble && c == '"') {
					inSingle, inDouble = false, false
				}
			case inBacktick:
				if c == '`' {
					inBacktick = false
				}
			case c == '/' && j+1 < len(line) && line[j+1] == '/':
				j = len(line)
				continue
			case c == '/' && j+1 < len(line) && line[j+1] == '*':
				inBlockComment = true
				j += 2
				continue
			case c == '"':
				inDouble = true
			case c == '\'':
				inSingle = true
			case c == '`':
				inBacktick = true
			case c == '{':
				depth++
				opened = true
			case c == '}':
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

// indentLevel returns the number of leading spaces/tabs of a line.
func indentLevel(line string) int {
	count := 0
	for _, ch := range line {
		if ch != ' ' && ch != '\t' {
			break
		}
		count++
	}
	return count
}

// findPythonEndLine computes the end line for Python by indentation: the
// element ends at the last line more indented than the declaration.
func findPythonEndLine(lines []string, startIdx int) int {
	if startIdx < 0 || startIdx >= len(lines) {
		return startIdx + 1
	}
	baseIndent := indentLevel(lines[startIdx])
	endIdx := startIdx
	for i := startIdx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if indentLevel(lines[i]) <= baseIndent {
			break
		}
		endIdx = i
	}
	return endIdx + 1
}

// =============================================================================
// get_elements / get_element
// =============================================================================

// GetElementsTool lists every element of a file.
func GetElementsTool() *tools.Tool {
	return &tools.Tool{
		Name:          "get_elements",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description: "List every element of one file with its line span, kind, ref, rev, signature and doc line: for Go the header (package clause, file doc, build tags, imports) and every func, method, type, const and var; for Mangle every Decl, rule, fact and query. " +
			"A file that does not parse is listed too, with its errors and its broken region as a syntax_error element. Pass a row's ref to get_element to see the source, or to the element edit verbs.",
		Category: tools.CategoryCode,
		Priority: 80,
		Effect:   tools.EffectRead,
		Execute:  executeGetElements,
		Schema: tools.ToolSchema{
			Required: []string{"path"},
			Properties: map[string]tools.Property{
				"path": {Type: "string", Description: "Workspace-relative file path"},
				"kind": {Type: "string", Description: "Optional filter: header, function, method, struct, interface, type, const, var, decl, rule, fact, query, syntax_error"},
			},
		},
	}
}

// loadedFile is one file read for an element tool.
type loadedFile struct {
	abs, rel string
	data     []byte
}

func loadFile(ctx context.Context, rawPath string) (*loadedFile, error) {
	if strings.TrimSpace(rawPath) == "" {
		return nil, fmt.Errorf("path is required")
	}
	// Contained like every other file tool: an uncontained read here is an
	// arbitrary file disclosure, since get_element returns file contents.
	abs, err := tools.ResolveWorkspacePath(ctx, "", rawPath)
	if err != nil {
		return nil, err
	}
	data, err := projectdoc.ReadFileForTool(abs)
	if err != nil {
		return nil, err
	}
	return &loadedFile{abs: abs, rel: tools.WorkspaceDisplayPath(ctx, abs), data: data}, nil
}

func executeGetElements(ctx context.Context, args map[string]any) (string, error) {
	rawPath, _ := args["path"].(string)
	lf, err := loadFile(ctx, rawPath)
	if err != nil {
		return "", err
	}
	kind, _ := args["kind"].(string)
	kind = strings.TrimSpace(kind)

	model, ok := codemodel.Parse(lf.rel, string(lf.data))
	if !ok {
		return regexElementList(lf, kind), nil
	}
	refs := canonicalRefs(ctx, lf.rel, model)

	var sb strings.Builder
	fmt.Fprintf(&sb, "elements of %s (%s", lf.rel, model.Language)
	if model.Package != "" {
		fmt.Fprintf(&sb, ", package %s", model.Package)
	}
	if model.BuildConstraint != "" {
		fmt.Fprintf(&sb, ", //go:build %s", model.BuildConstraint)
	}
	fmt.Fprintf(&sb, ", %d lines", model.LineCount())
	if model.Parsed {
		sb.WriteString(", parses):\n")
	} else {
		sb.WriteString(", DOES NOT PARSE):\n")
		for _, se := range model.Errors {
			fmt.Fprintf(&sb, "-- syntax error at line %d:%d: %s\n", se.Line, se.Column, se.Msg)
		}
	}
	var rows []string
	for i := range model.Elements {
		e := &model.Elements[i]
		if kind != "" && !strings.EqualFold(kind, string(e.Kind)) {
			continue
		}
		rows = append(rows, symbolRow(symbolOf(lf.rel, e, refs)))
	}
	if len(rows) == 0 {
		sb.WriteString("none")
		if kind != "" {
			sb.WriteString(" of kind " + kind)
		}
		sb.WriteString("\n" + tools.StructuralNoRows + "\n")
	}
	for _, r := range rows {
		sb.WriteString(r + "\n")
	}
	if len(rows) > 0 {
		fmt.Fprintf(&sb, "-- %d rows, complete\n", len(rows))
	}
	if !model.Parsed {
		sb.WriteString(lastGoodNote(ctx, lf.rel, model))
		sb.WriteString("-- repair: get_element ref=" + lf.rel + ":syntax_error shows the broken region; replace_element with that ref replaces it.\n")
	}
	logging.Tools("get_elements completed: %s (%d elements, parsed=%v)", lf.rel, len(rows), model.Parsed)
	return sb.String(), nil
}

// lastGoodNote lists the declarations the last clean parse had that the
// broken file no longer shows, from the index when one is registered.
func lastGoodNote(ctx context.Context, rel string, model *codemodel.File) string {
	provider := optionalStructureProvider()
	if provider == nil {
		return ""
	}
	st, err := provider.FileStatus(ctx, rel)
	if err != nil || len(st.LastGood) == 0 {
		return ""
	}
	var missing []string
	for _, s := range st.LastGood {
		if model.Element(s.Key) == nil {
			missing = append(missing, fmt.Sprintf("%s (was lines %d-%d)", s.Key, s.StartLine, s.EndLine))
		}
	}
	if len(missing) == 0 {
		return ""
	}
	return "-- the last clean parse also had: " + strings.Join(missing, ", ") + "; they are inside the broken region now\n"
}

func regexElementList(lf *loadedFile, kind string) string {
	elements := ElementsFromSource(lf.abs, string(lf.data))
	var sb strings.Builder
	fmt.Fprintf(&sb, "elements of %s (no parser for this language; spans are from line patterns and not editable by ref):\n", lf.rel)
	n := 0
	for _, e := range elements {
		if kind != "" && !strings.EqualFold(kind, e.Type) {
			continue
		}
		n++
		fmt.Fprintf(&sb, "%s:%d-%d  %s  %s  %s\n", lf.rel, e.StartLine, e.EndLine, e.Type, e.Name, e.Signature)
	}
	if n == 0 {
		sb.WriteString("none\n" + tools.StructuralNoRows + "\n")
	} else {
		fmt.Fprintf(&sb, "-- %d rows, complete\n", n)
	}
	return sb.String()
}

// canonicalRefs asks the index for the workspace refs of a file's elements,
// so the refs printed here are the ones every other tool resolves. Without
// an index (a bare registry, a file outside the walk), refs are derived from
// the path alone.
func canonicalRefs(ctx context.Context, rel string, model *codemodel.File) map[string]string {
	if provider := optionalStructureProvider(); provider != nil {
		if refs, err := provider.CanonicalRefs(ctx, rel); err == nil && len(refs) > 0 {
			return refs
		}
	}
	return nil
}

// RefOf is an element's workspace ref: directory, receiver and name for a Go
// declaration, file and key for everything else (a Go header, a broken
// region, a Mangle statement).
func RefOf(rel string, e *codemodel.Element, refs map[string]string) string {
	if r, ok := refs[e.Key]; ok {
		return r
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".go") || e.Kind == codemodel.KindHeader || e.Kind == codemodel.KindSyntaxError {
		return rel + ":" + e.Key
	}
	dir := filepath.ToSlash(filepath.Dir(rel))
	if dir == "." || dir == "" {
		return "./" + e.Key
	}
	return dir + "." + e.Key
}

func symbolOf(rel string, e *codemodel.Element, refs map[string]string) StructureSymbol {
	doc := e.Doc
	if e.Kind == codemodel.KindSyntaxError {
		doc = e.Err
	}
	return StructureSymbol{
		Ref: RefOf(rel, e, refs), Key: e.Key, Kind: string(e.Kind), File: rel,
		Signature: e.Signature, Doc: doc, Revision: e.Revision,
		StartLine: e.StartLine, EndLine: e.EndLine, Exported: e.Exported,
	}
}

// GetElementTool returns one element's source.
func GetElementTool() *tools.Tool {
	return &tools.Tool{
		Name:          "get_element",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description: "Return an element's source -- doc comment included, line-numbered -- with its ref and rev (pass the rev to an edit verb as its precondition). " +
			"Address it by ref as any structural tool prints it; path is optional and narrows the lookup to one file. " +
			"An element over " + fmt.Sprint(elementPageLines) + " lines answers with an outline of its nested parts (statements, case clauses, closures); ask for one with part, or for all of it with full.",
		Category: tools.CategoryCode,
		Priority: 80,
		Effect:   tools.EffectRead,
		Execute:  executeGetElement,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"ref":  {Type: "string", Description: "Element ref (internal/world.StructureIndex.Refresh, internal/core/defaults/policy/impact.mg:decl:impact_caller/2), or with path, the element's name in that file (StructureIndex.Refresh, header)"},
				"refs": {Type: "array", Description: "Several refs to return in one call", Items: &tools.PropertyItems{Type: "string"}},
				"path": {Type: "string", Description: "Optional workspace-relative file the element is in"},
				"part": {Type: "string", Description: "A nested part by its path in the element's outline: 3 (third statement), 3.2 (its second case clause or branch)"},
				"full": {Type: "boolean", Description: "Return a large element whole instead of its outline"},
			},
		},
	}
}

// elementPageLines is the size above which get_element answers with the
// outline of an element's parts rather than its text: the median Go read in
// the audited runs spanned 35 lines, and a 1,058-line function in one answer
// is 40 KB the model rarely needs all of. It is a page, not a cut: the answer
// names every part and how to get it, and full=true returns the whole.
const elementPageLines = 400

// partExpandLines is the size above which the outline lists a part's own
// children, so a giant switch shows its cases without every statement.
const partExpandLines = 60

// resolvedElement is an element found for a ref, with its file.
type resolvedElement struct {
	file  *loadedFile
	model *codemodel.File
	elem  *codemodel.Element
	refs  map[string]string
}

// resolveElement finds the one element a ref names. With a path the lookup
// is within that file; without one it goes through the index. A ref naming
// several elements is refused with every candidate's ref, never guessed.
func resolveElement(ctx context.Context, ref, path string) (*resolvedElement, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("ref is required")
	}
	if path == "" {
		if p, key, ok := splitFileRef(ref); ok {
			path, ref = p, key
		}
	}
	if path == "" {
		provider := optionalStructureProvider()
		if provider == nil {
			return nil, fmt.Errorf("ref %q needs the structure index to resolve; pass path as well", ref)
		}
		candidates, _, err := provider.Resolve(ctx, ref)
		if err != nil {
			return nil, err
		}
		switch len(candidates) {
		case 0:
			return nil, fmt.Errorf("no element is named %q; find_symbol searches by name and pattern", ref)
		case 1:
			path, ref = candidates[0].File, candidates[0].Key
		default:
			return nil, refAmbiguity(ref, candidates)
		}
	}
	lf, err := loadFile(ctx, path)
	if err != nil {
		return nil, err
	}
	model, ok := codemodel.Parse(lf.rel, string(lf.data))
	if !ok {
		return nil, fmt.Errorf("%s is not a file CodeDOM parses (Go or Mangle); it has no elements to address by ref", lf.rel)
	}
	refs := canonicalRefs(ctx, lf.rel, model)
	matches := lookupInFile(model, lf.rel, ref, refs)
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%s has no element %q; get_elements path=%s lists what it has", lf.rel, ref, lf.rel)
	case 1:
		return &resolvedElement{file: lf, model: model, elem: matches[0], refs: refs}, nil
	}
	var syms []StructureSymbol
	for _, m := range matches {
		syms = append(syms, symbolOf(lf.rel, m, refs))
	}
	return nil, refAmbiguity(ref, syms)
}

// lookupInFile matches a ref against one file: its canonical ref, its key,
// the ref with the directory prefix or @file discriminator removed, or any
// name the model's Lookup accepts.
func lookupInFile(model *codemodel.File, rel, ref string, refs map[string]string) []*codemodel.Element {
	for i := range model.Elements {
		e := &model.Elements[i]
		if r := RefOf(rel, e, refs); r == ref {
			return []*codemodel.Element{e}
		}
	}
	key := ref
	if base, file, ok := strings.Cut(key, "@"); ok && !strings.Contains(file, "/") && strings.HasSuffix(file, ".go") {
		if file != filepath.Base(rel) {
			return nil
		}
		key = base
	}
	prefix := "./"
	if dir := filepath.ToSlash(filepath.Dir(rel)); dir != "." && dir != "" {
		prefix = dir + "."
	}
	key = strings.TrimPrefix(key, prefix)
	return model.Lookup(key)
}

// splitFileRef reads a file-scoped ref, "path/to/file.go:Key".
func splitFileRef(ref string) (path, key string, ok bool) {
	for _, ext := range []string{".go:", ".mg:", ".dl:", ".mangle:"} {
		if i := strings.Index(ref, ext); i > 0 {
			return ref[:i+len(ext)-1], ref[i+len(ext):], true
		}
	}
	return "", "", false
}

func refAmbiguity(ref string, candidates []StructureSymbol) error {
	rows := make([]string, 0, len(candidates))
	for _, c := range candidates {
		rows = append(rows, fmt.Sprintf("%s (%s:%d-%d)", c.Ref, c.File, c.StartLine, c.EndLine))
	}
	return fmt.Errorf("%q names %d elements, so it is a search, not an address; use one of these refs: %s", ref, len(candidates), strings.Join(rows, "; "))
}

func executeGetElement(ctx context.Context, args map[string]any) (string, error) {
	path, _ := args["path"].(string)
	part, _ := args["part"].(string)
	full, _ := args["full"].(bool)
	refs := parseStringArray(args["refs"])
	if ref, _ := args["ref"].(string); strings.TrimSpace(ref) != "" {
		refs = append([]string{ref}, refs...)
	}
	if len(refs) == 0 {
		return "", fmt.Errorf("ref is required")
	}
	if len(refs) > 1 && part != "" {
		return "", fmt.Errorf("part addresses one element; ask for one ref at a time with part")
	}
	var sb strings.Builder
	for i, ref := range refs {
		if i > 0 {
			sb.WriteString("\n")
		}
		re, err := resolveElement(ctx, ref, path)
		if err != nil {
			if len(refs) == 1 {
				return "", err
			}
			fmt.Fprintf(&sb, "%s: %v\n", ref, err)
			continue
		}
		text, err := renderElement(re, part, full)
		if err != nil {
			return "", err
		}
		sb.WriteString(text)
	}
	return sb.String(), nil
}

// renderElement is the view of one element: its header line, then its
// numbered source, or for a large element the outline of its parts.
func renderElement(re *resolvedElement, part string, full bool) (string, error) {
	e := re.elem
	ref := RefOf(re.file.rel, e, re.refs)
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s  %s  %s:%d-%d  rev %s\n", ref, e.Kind, re.file.rel, e.StartLine, e.EndLine, e.Revision)
	if e.Kind == codemodel.KindSyntaxError {
		fmt.Fprintf(&sb, "-- does not parse: %s\n", e.Err)
	}
	if part != "" {
		p, err := re.model.FindPart(e, part)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&sb, "-- part %s: %s (lines %d-%d)\n", p.Path, p.Label, p.StartLine, p.EndLine)
		sb.WriteString(re.model.NumberedLines(p.StartLine, p.EndLine))
		return sb.String(), nil
	}
	lines := e.EndLine - e.StartLine + 1
	if lines > elementPageLines && !full {
		if parts := re.model.Parts(e); len(parts) > 0 {
			sb.WriteString(re.model.NumberedLines(e.StartLine, min(e.DeclLine, e.EndLine)))
			fmt.Fprintf(&sb, "-- %d lines; its parts (ask get_element ref=%s part=<path>, or full=true for all of it):\n", lines, ref)
			for _, row := range codemodel.RenderParts(parts, partExpandLines) {
				sb.WriteString(row + "\n")
			}
			return sb.String(), nil
		}
	}
	sb.WriteString(re.model.NumberedLines(e.StartLine, e.EndLine))
	return sb.String(), nil
}
