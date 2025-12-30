package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ElementType defines the semantic type of a code element.
type ElementType string

const (
	ElementFunction  ElementType = "function"
	ElementMethod    ElementType = "method"
	ElementStruct    ElementType = "struct"
	ElementInterface ElementType = "interface"
	ElementType_     ElementType = "type" // Type alias
	ElementConst     ElementType = "const"
	ElementVar       ElementType = "var"
	ElementPackage   ElementType = "package"

	// Mangle (.mg/.dl) elements
	ElementMangleDecl  ElementType = "decl"
	ElementMangleRule  ElementType = "rule"
	ElementMangleFact  ElementType = "fact"
	ElementMangleQuery ElementType = "query"
)

// Visibility defines the visibility of a code element.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

// ActionType defines the interactive actions available on an element.
type ActionType string

const (
	ActionView         ActionType = "view"
	ActionReplace      ActionType = "replace"
	ActionInsertBefore ActionType = "insert_before"
	ActionInsertAfter  ActionType = "insert_after"
	ActionDelete       ActionType = "delete"
)

// CodeElement represents a semantic unit of code with stable reference.
// Analogous to a DOM element in a browser.
type CodeElement struct {
	// Ref is the stable reference ID (e.g., "fn:context.Compressor.Compress")
	Ref string `json:"ref"`

	// Type is the semantic type (function, method, struct, interface, etc.)
	Type ElementType `json:"type"`

	// File is the source file path
	File string `json:"file"`

	// StartLine and EndLine are 1-indexed inclusive line numbers
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`

	// Signature is the declaration line (e.g., "func (c *Compressor) Compress(...)")
	Signature string `json:"signature"`

	// Body is the full text of the element (for display/editing)
	Body string `json:"body,omitempty"`

	// Parent is the ref of the containing element (e.g., struct for methods)
	Parent string `json:"parent,omitempty"`

	// Visibility is public or private (Go: capitalization)
	Visibility Visibility `json:"visibility"`

	// Actions are the available interactive operations
	Actions []ActionType `json:"actions"`

	// Package is the package name
	Package string `json:"package"`

	// Name is the element's name (without package prefix)
	Name string `json:"name"`
}

// ToFacts converts a CodeElement to Mangle facts.
func (e *CodeElement) ToFacts() []core.Fact {
	facts := make([]core.Fact, 0, 5)

	// code_element(ref, elem_type, file, start_line, end_line)
	facts = append(facts, core.Fact{
		Predicate: "code_element",
		Args:      []interface{}{e.Ref, "/" + string(e.Type), e.File, int64(e.StartLine), int64(e.EndLine)},
	})

	// element_signature(ref, signature)
	facts = append(facts, core.Fact{
		Predicate: "element_signature",
		Args:      []interface{}{e.Ref, e.Signature},
	})

	// element_visibility(ref, visibility)
	facts = append(facts, core.Fact{
		Predicate: "element_visibility",
		Args:      []interface{}{e.Ref, "/" + string(e.Visibility)},
	})

	// element_parent(ref, parent_ref) - only if has parent
	if e.Parent != "" {
		facts = append(facts, core.Fact{
			Predicate: "element_parent",
			Args:      []interface{}{e.Ref, e.Parent},
		})
	}

	// code_interactable(ref, action_type) - for each action
	for _, action := range e.Actions {
		facts = append(facts, core.Fact{
			Predicate: "code_interactable",
			Args:      []interface{}{e.Ref, "/" + string(action)},
		})
	}

	return facts
}

// CodeElementParser extracts semantic code elements with precise line ranges.
// It delegates to language-specific CodeParsers via ParserFactory.
type CodeElementParser struct {
	// Cache of file content for body extraction
	fileCache map[string][]string

	// factory delegates parsing to language-specific parsers
	factory *ParserFactory

	// projectRoot for repo-anchored refs (legacy support)
	projectRoot string
}

// NewCodeElementParser creates a new CodeElementParser.
// Deprecated: Use NewCodeElementParserWithFactory for polyglot support.
func NewCodeElementParser() *CodeElementParser {
	logging.WorldDebug("Creating new CodeElementParser (legacy mode)")
	return &CodeElementParser{
		fileCache:   make(map[string][]string),
		projectRoot: ".",
	}
}

// NewCodeElementParserWithFactory creates a CodeElementParser with a ParserFactory.
// This is the preferred constructor for polyglot CodeDOM support.
func NewCodeElementParserWithFactory(factory *ParserFactory) *CodeElementParser {
	logging.WorldDebug("Creating CodeElementParser with factory (polyglot mode)")
	return &CodeElementParser{
		fileCache:   make(map[string][]string),
		factory:     factory,
		projectRoot: factory.ProjectRoot(),
	}
}

// NewCodeElementParserWithRoot creates a CodeElementParser with a project root.
// This creates a default factory with all built-in parsers.
func NewCodeElementParserWithRoot(projectRoot string) *CodeElementParser {
	factory := DefaultParserFactory(projectRoot)
	return NewCodeElementParserWithFactory(factory)
}

// Factory returns the underlying ParserFactory, or nil if using legacy mode.
func (p *CodeElementParser) Factory() *ParserFactory {
	return p.factory
}

// ParseFile parses a source file and returns all code elements.
// If a ParserFactory is configured, it delegates to the appropriate CodeParser.
// Otherwise, falls back to legacy direct parsing for backward compatibility.
func (p *CodeElementParser) ParseFile(path string) ([]CodeElement, error) {
	start := time.Now()
	logging.WorldDebug("CodeElementParser: parsing file: %s", filepath.Base(path))

	// Try factory-based parsing first (polyglot mode)
	if p.factory != nil {
		content, err := os.ReadFile(path)
		if err != nil {
			logging.Get(logging.CategoryWorld).Error("CodeElementParser: read failed: %s - %v", path, err)
			return nil, err
		}

		// Cache for body extraction
		lines := strings.Split(string(content), "\n")
		p.fileCache[path] = lines

		if p.factory.HasParser(path) {
			elems, err := p.factory.Parse(path, content)
			if err != nil {
				logging.Get(logging.CategoryWorld).Error("CodeElementParser: factory parse failed: %s - %v", path, err)
				return nil, err
			}
			logging.WorldDebug("CodeElementParser: parsed %s via factory - %d elements in %v",
				filepath.Base(path), len(elems), time.Since(start))
			return elems, nil
		}
		// Fall through to legacy parsing if no parser registered
		logging.WorldDebug("CodeElementParser: no factory parser for %s, using legacy", filepath.Base(path))
	}

	// Legacy parsing (backward compatibility)
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mg", ".dl", ".mangle":
		elems, err := p.parseMangleFile(path)
		if err != nil {
			logging.Get(logging.CategoryWorld).Error("CodeElementParser: mangle parse failed: %s - %v", path, err)
			return nil, err
		}
		logging.WorldDebug("CodeElementParser: parsed %s - %d mangle elements in %v",
			filepath.Base(path), len(elems), time.Since(start))
		return elems, nil
	}

	return p.parseGoFileLegacy(path, start)
}

// parseGoFileLegacy is the original Go parsing implementation for backward compatibility.
func (p *CodeElementParser) parseGoFileLegacy(path string, start time.Time) ([]CodeElement, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("CodeElementParser: parse failed: %s - %v", path, err)
		return nil, err
	}

	// Cache file content for body extraction
	content, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("CodeElementParser: read failed: %s - %v", path, err)
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	p.fileCache[path] = lines

	var elements []CodeElement
	pkgName := node.Name.Name
	logging.WorldDebug("CodeElementParser: package=%s, %d lines for %s", pkgName, len(lines), filepath.Base(path))

	// Default actions for all elements
	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}

	// Track struct receivers for method parent linking
	structRefs := make(map[string]string) // receiver name -> struct ref

	// First pass: collect all struct names
	var structCount int
	for _, decl := range node.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
			for _, spec := range genDecl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
						name := typeSpec.Name.Name
						ref := fmt.Sprintf("struct:%s.%s", pkgName, name)
						structRefs[name] = ref
						structCount++
					}
				}
			}
		}
	}
	logging.WorldDebug("CodeElementParser: found %d struct types", structCount)

	// Process all declarations
	var funcCount, methodCount, typeCount int
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			elem := p.parseFuncDecl(fset, d, path, pkgName, lines, structRefs, defaultActions)
			elements = append(elements, elem)
			if elem.Type == ElementMethod {
				methodCount++
			} else {
				funcCount++
			}

		case *ast.GenDecl:
			elems := p.parseGenDecl(fset, d, path, pkgName, lines, defaultActions)
			elements = append(elements, elems...)
			typeCount += len(elems)
		}
	}

	logging.WorldDebug("CodeElementParser: parsed %s - %d elements (funcs=%d, methods=%d, types=%d) in %v",
		filepath.Base(path), len(elements), funcCount, methodCount, typeCount, time.Since(start))
	return elements, nil
}

// parseFuncDecl parses a function or method declaration.
func (p *CodeElementParser) parseFuncDecl(
	fset *token.FileSet,
	decl *ast.FuncDecl,
	path, pkgName string,
	lines []string,
	structRefs map[string]string,
	actions []ActionType,
) CodeElement {
	name := decl.Name.Name
	startLine := fset.Position(decl.Pos()).Line
	endLine := fset.Position(decl.End()).Line

	// Determine visibility
	visibility := VisibilityPrivate
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		visibility = VisibilityPublic
	}

	// Determine if method and extract receiver info
	elemType := ElementFunction
	var parentRef string
	ref := fmt.Sprintf("fn:%s.%s", pkgName, name)

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		elemType = ElementMethod
		recv := decl.Recv.List[0]
		recvName := extractReceiverTypeName(recv.Type)
		if recvName != "" {
			ref = fmt.Sprintf("fn:%s.%s.%s", pkgName, recvName, name)
			if sref, ok := structRefs[recvName]; ok {
				parentRef = sref
			}
		}
	}

	// Extract signature (first line of function)
	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	// Extract body
	body := extractBody(lines, startLine, endLine)

	return CodeElement{
		Ref:        ref,
		Type:       elemType,
		File:       path,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       body,
		Parent:     parentRef,
		Visibility: visibility,
		Actions:    actions,
		Package:    pkgName,
		Name:       name,
	}
}

// parseGenDecl parses type, const, and var declarations.
func (p *CodeElementParser) parseGenDecl(
	fset *token.FileSet,
	decl *ast.GenDecl,
	path, pkgName string,
	lines []string,
	actions []ActionType,
) []CodeElement {
	var elements []CodeElement

	switch decl.Tok {
	case token.TYPE:
		for _, spec := range decl.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				elem := p.parseTypeSpec(fset, decl, typeSpec, path, pkgName, lines, actions)
				elements = append(elements, elem)
			}
		}

	case token.CONST:
		// Group constants together
		startLine := fset.Position(decl.Pos()).Line
		endLine := fset.Position(decl.End()).Line

		for _, spec := range decl.Specs {
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range valueSpec.Names {
					elemName := name.Name
					visibility := VisibilityPrivate
					if len(elemName) > 0 && elemName[0] >= 'A' && elemName[0] <= 'Z' {
						visibility = VisibilityPublic
					}

					specStart := fset.Position(spec.Pos()).Line
					specEnd := fset.Position(spec.End()).Line
					signature := ""
					if specStart > 0 && specStart <= len(lines) {
						signature = strings.TrimSpace(lines[specStart-1])
					}

					elements = append(elements, CodeElement{
						Ref:        fmt.Sprintf("const:%s.%s", pkgName, elemName),
						Type:       ElementConst,
						File:       path,
						StartLine:  specStart,
						EndLine:    specEnd,
						Signature:  signature,
						Body:       extractBody(lines, startLine, endLine),
						Visibility: visibility,
						Actions:    actions,
						Package:    pkgName,
						Name:       elemName,
					})
				}
			}
		}

	case token.VAR:
		startLine := fset.Position(decl.Pos()).Line
		endLine := fset.Position(decl.End()).Line

		for _, spec := range decl.Specs {
			if valueSpec, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range valueSpec.Names {
					elemName := name.Name
					visibility := VisibilityPrivate
					if len(elemName) > 0 && elemName[0] >= 'A' && elemName[0] <= 'Z' {
						visibility = VisibilityPublic
					}

					specStart := fset.Position(spec.Pos()).Line
					specEnd := fset.Position(spec.End()).Line
					signature := ""
					if specStart > 0 && specStart <= len(lines) {
						signature = strings.TrimSpace(lines[specStart-1])
					}

					elements = append(elements, CodeElement{
						Ref:        fmt.Sprintf("var:%s.%s", pkgName, elemName),
						Type:       ElementVar,
						File:       path,
						StartLine:  specStart,
						EndLine:    specEnd,
						Signature:  signature,
						Body:       extractBody(lines, startLine, endLine),
						Visibility: visibility,
						Actions:    actions,
						Package:    pkgName,
						Name:       elemName,
					})
				}
			}
		}
	}

	return elements
}

// parseTypeSpec parses a type specification (struct, interface, alias).
func (p *CodeElementParser) parseTypeSpec(
	fset *token.FileSet,
	decl *ast.GenDecl,
	spec *ast.TypeSpec,
	path, pkgName string,
	lines []string,
	actions []ActionType,
) CodeElement {
	name := spec.Name.Name
	startLine := fset.Position(decl.Pos()).Line
	endLine := fset.Position(decl.End()).Line

	// For single type declarations without parens, use spec positions
	if decl.Lparen == 0 {
		startLine = fset.Position(spec.Pos()).Line
		endLine = fset.Position(spec.End()).Line
	}

	visibility := VisibilityPrivate
	if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
		visibility = VisibilityPublic
	}

	elemType := ElementType_
	refPrefix := "type"

	switch spec.Type.(type) {
	case *ast.StructType:
		elemType = ElementStruct
		refPrefix = "struct"
	case *ast.InterfaceType:
		elemType = ElementInterface
		refPrefix = "interface"
	}

	ref := fmt.Sprintf("%s:%s.%s", refPrefix, pkgName, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	return CodeElement{
		Ref:        ref,
		Type:       elemType,
		File:       path,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    pkgName,
		Name:       name,
	}
}

// extractReceiverTypeName extracts the type name from a method receiver.
func extractReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return extractReceiverTypeName(t.X)
	}
	return ""
}

// extractBody extracts the body text from line range (1-indexed, inclusive).
func extractBody(lines []string, startLine, endLine int) string {
	if startLine < 1 || endLine < startLine || startLine > len(lines) {
		return ""
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}

// GetElement returns a single element by ref from parsed elements.
func GetElement(elements []CodeElement, ref string) *CodeElement {
	for i := range elements {
		if elements[i].Ref == ref {
			return &elements[i]
		}
	}
	return nil
}

// GetElementsByType returns elements of a specific type.
func GetElementsByType(elements []CodeElement, elemType ElementType) []CodeElement {
	var result []CodeElement
	for _, e := range elements {
		if e.Type == elemType {
			result = append(result, e)
		}
	}
	return result
}

// GetElementsInRange returns elements that overlap with a line range.
func GetElementsInRange(elements []CodeElement, startLine, endLine int) []CodeElement {
	var result []CodeElement
	for _, e := range elements {
		if e.EndLine >= startLine && e.StartLine <= endLine {
			result = append(result, e)
		}
	}
	return result
}

// GetMethodsOfStruct returns all methods belonging to a struct.
func GetMethodsOfStruct(elements []CodeElement, structRef string) []CodeElement {
	var result []CodeElement
	for _, e := range elements {
		if e.Type == ElementMethod && e.Parent == structRef {
			result = append(result, e)
		}
	}
	return result
}

// CodePatterns contains detected patterns in a file.
type CodePatterns struct {
	IsGenerated      bool
	Generator        string // protobuf, openapi, swagger, grpc, wire, ent, sqlc, gqlgen
	GeneratorMarker  string
	HasCGo           bool
	BuildTags        []string
	EmbedDirectives  []string
	APIClientFuncs   []APIPattern
	APIHandlerFuncs  []APIPattern
}

// APIPattern describes an API-related function.
type APIPattern struct {
	Ref      string
	Endpoint string // URL pattern or route
	Method   string // GET, POST, PUT, DELETE, PATCH
}

// DetectCodePatterns analyzes file content for special patterns.
func DetectCodePatterns(content string, elements []CodeElement) CodePatterns {
	patterns := CodePatterns{}

	// Check for generated code markers
	generatedMarkers := map[string]string{
		"Code generated by protoc":         "protobuf",
		"Code generated by protoc-gen-go":  "protobuf",
		"generated by protoc-gen-grpc":     "grpc",
		"Code generated by entc":           "ent",
		"Code generated by sqlc":           "sqlc",
		"generated by Wire":                "wire",
		"Code generated by gqlgen":         "gqlgen",
		"Code generated by oapi-codegen":   "openapi",
		"Code generated by swagger":        "swagger",
		"DO NOT EDIT":                      "unknown",
		"GENERATED FILE":                   "unknown",
		"This file was autogenerated":      "unknown",
		"Auto-generated":                   "unknown",
	}

	for marker, gen := range generatedMarkers {
		if strings.Contains(content, marker) {
			patterns.IsGenerated = true
			patterns.Generator = gen
			patterns.GeneratorMarker = marker
			break
		}
	}

	// Check for CGo
	if strings.Contains(content, "import \"C\"") || strings.Contains(content, "/*\n#include") {
		patterns.HasCGo = true
	}

	// Check for build tags (both old and new style)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "// +build ") {
			tag := strings.TrimPrefix(line, "// +build ")
			patterns.BuildTags = append(patterns.BuildTags, tag)
		}
		if strings.HasPrefix(line, "//go:build ") {
			tag := strings.TrimPrefix(line, "//go:build ")
			patterns.BuildTags = append(patterns.BuildTags, tag)
		}
		if strings.HasPrefix(line, "//go:embed ") {
			embed := strings.TrimPrefix(line, "//go:embed ")
			patterns.EmbedDirectives = append(patterns.EmbedDirectives, embed)
		}
	}

	// Detect API client patterns in function bodies
	httpMethods := []string{"http.Get", "http.Post", "http.Put", "http.Delete", "http.NewRequest"}
	for _, elem := range elements {
		if elem.Type != ElementFunction && elem.Type != ElementMethod {
			continue
		}

		body := elem.Body
		for _, method := range httpMethods {
			if strings.Contains(body, method) {
				apiMethod := "GET"
				if strings.Contains(method, "Post") {
					apiMethod = "POST"
				} else if strings.Contains(method, "Put") {
					apiMethod = "PUT"
				} else if strings.Contains(method, "Delete") {
					apiMethod = "DELETE"
				}
				patterns.APIClientFuncs = append(patterns.APIClientFuncs, APIPattern{
					Ref:    elem.Ref,
					Method: apiMethod,
				})
				break
			}
		}

		// Detect HTTP handlers (common patterns)
		handlerPatterns := []string{
			"http.HandleFunc",
			"mux.HandleFunc",
			"router.GET",
			"router.POST",
			"router.PUT",
			"router.DELETE",
			"e.GET",
			"e.POST", // Echo
			"r.Get",
			"r.Post", // Chi
			"gin.Context",
		}
		for _, hp := range handlerPatterns {
			if strings.Contains(body, hp) {
				patterns.APIHandlerFuncs = append(patterns.APIHandlerFuncs, APIPattern{
					Ref:    elem.Ref,
					Method: "ANY",
				})
				break
			}
		}
	}

	return patterns
}

// ToPatternFacts converts CodePatterns to Mangle facts.
func (p *CodePatterns) ToPatternFacts(file string, elements []CodeElement) []core.Fact {
	var facts []core.Fact

	normalizeHTTPMethodAtom := func(method string) string {
		m := strings.TrimSpace(method)
		m = strings.TrimPrefix(m, "/")
		if m == "" {
			m = "any"
		}
		return "/" + strings.ToLower(m)
	}

	if p.IsGenerated {
		facts = append(facts, core.Fact{
			Predicate: "generated_code",
			Args:      []interface{}{file, "/" + p.Generator, p.GeneratorMarker},
		})
	}

	if p.HasCGo {
		facts = append(facts, core.Fact{
			Predicate: "cgo_code",
			Args:      []interface{}{file},
		})
	}

	for _, tag := range p.BuildTags {
		facts = append(facts, core.Fact{
			Predicate: "build_tag",
			Args:      []interface{}{file, tag},
		})
	}

	for _, embed := range p.EmbedDirectives {
		facts = append(facts, core.Fact{
			Predicate: "embed_directive",
			Args:      []interface{}{file, embed},
		})
	}

	for _, api := range p.APIClientFuncs {
		facts = append(facts, core.Fact{
			Predicate: "api_client_function",
			Args:      []interface{}{api.Ref, api.Endpoint, normalizeHTTPMethodAtom(api.Method)},
		})
	}

	for _, api := range p.APIHandlerFuncs {
		facts = append(facts, core.Fact{
			Predicate: "api_handler_function",
			Args:      []interface{}{api.Ref, api.Endpoint, normalizeHTTPMethodAtom(api.Method)},
		})
	}

	return facts
}

// ElementsToFacts converts a slice of CodeElements to Mangle facts.
func ElementsToFacts(elements []CodeElement) []core.Fact {
	var facts []core.Fact
	for _, e := range elements {
		facts = append(facts, e.ToFacts()...)
	}
	return facts
}
