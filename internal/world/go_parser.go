package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"time"
)

// GoCodeParser implements CodeParser for Go source files.
// It uses the standard go/ast package for precise parsing.
type GoCodeParser struct {
	projectRoot string
}

// NewGoCodeParser creates a new Go parser with the given project root.
func NewGoCodeParser(projectRoot string) *GoCodeParser {
	return &GoCodeParser{
		projectRoot: projectRoot,
	}
}

// Language returns "go" for Ref URI generation.
func (p *GoCodeParser) Language() string {
	return "go"
}

// SupportedExtensions returns [".go"].
func (p *GoCodeParser) SupportedExtensions() []string {
	return []string{".go"}
}

// Parse extracts CodeElements from Go source code.
func (p *GoCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	start := time.Now()
	logging.WorldDebug("GoCodeParser: parsing file: %s", filepath.Base(path))

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("GoCodeParser: parse failed: %s - %v", path, err)
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	pkgName := node.Name.Name
	logging.WorldDebug("GoCodeParser: package=%s, %d lines for %s", pkgName, len(lines), filepath.Base(path))

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
						ref := p.buildRef("struct", pkgName, name, "")
						structRefs[name] = ref
						structCount++
					}
				}
			}
		}
	}
	logging.WorldDebug("GoCodeParser: found %d struct types", structCount)

	// Process all declarations
	var elements []CodeElement
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
			elems := p.parseGenDecl(fset, d, path, pkgName, lines, defaultActions, structRefs)
			elements = append(elements, elems...)
			typeCount += len(elems)
		}
	}

	logging.WorldDebug("GoCodeParser: parsed %s - %d elements (funcs=%d, methods=%d, types=%d) in %v",
		filepath.Base(path), len(elements), funcCount, methodCount, typeCount, time.Since(start))
	return elements, nil
}

// EmitLanguageFacts generates Go-specific Stratum 0 Mangle facts.
func (p *GoCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	var facts []core.Fact

	for _, elem := range elements {
		switch elem.Type {
		case ElementStruct:
			// go_struct(Ref)
			facts = append(facts, core.Fact{
				Predicate: "go_struct",
				Args:      []interface{}{elem.Ref},
			})

			// Extract struct tags from body for wire name inference
			tags := p.extractStructTags(elem.Body)
			for _, tag := range tags {
				// go_tag(Ref, TagContent)
				facts = append(facts, core.Fact{
					Predicate: "go_tag",
					Args:      []interface{}{elem.Ref, tag},
				})
			}

		case ElementInterface:
			// go_interface(Ref)
			facts = append(facts, core.Fact{
				Predicate: "go_interface",
				Args:      []interface{}{elem.Ref},
			})

		case ElementFunction, ElementMethod:
			// Detect goroutine spawning
			if strings.Contains(elem.Body, "go ") && (strings.Contains(elem.Body, "go func") || containsGoRoutineCall(elem.Body)) {
				// go_goroutine(Ref)
				facts = append(facts, core.Fact{
					Predicate: "go_goroutine",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Detect context.Context usage (good pattern)
			if strings.Contains(elem.Signature, "context.Context") || strings.Contains(elem.Signature, "ctx context") {
				facts = append(facts, core.Fact{
					Predicate: "go_uses_context",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Detect error handling
			if strings.Contains(elem.Signature, "error") {
				facts = append(facts, core.Fact{
					Predicate: "go_returns_error",
					Args:      []interface{}{elem.Ref},
				})
			}
		}

		// Method-to-struct linking fact
		if elem.Type == ElementMethod && elem.Parent != "" {
			facts = append(facts, core.Fact{
				Predicate: "method_of",
				Args:      []interface{}{elem.Ref, elem.Parent},
			})
		}
	}

	return facts
}

// buildRef creates a repo-anchored Ref URI.
func (p *GoCodeParser) buildRef(prefix, pkgName, name, parent string) string {
	if parent != "" {
		return fmt.Sprintf("%s:%s.%s.%s", prefix, pkgName, parent, name)
	}
	return fmt.Sprintf("%s:%s.%s", prefix, pkgName, name)
}

// parseFuncDecl parses a function or method declaration.
func (p *GoCodeParser) parseFuncDecl(
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
	var recvType string
	var isPointer bool
	ref := p.buildRef("fn", pkgName, name, "")

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		elemType = ElementMethod
		recv := decl.Recv.List[0]
		recvType, isPointer = extractReceiverTypeInfo(recv.Type)
		if recvType != "" {
			ref = p.buildRef("fn", pkgName, name, recvType)
			if sref, ok := structRefs[recvType]; ok {
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

	elem := CodeElement{
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

	// Store receiver info in metadata for advanced analysis
	_ = recvType
	_ = isPointer

	return elem
}

// parseGenDecl parses type, const, and var declarations.
func (p *GoCodeParser) parseGenDecl(
	fset *token.FileSet,
	decl *ast.GenDecl,
	path, pkgName string,
	lines []string,
	actions []ActionType,
	structRefs map[string]string,
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
						Ref:        p.buildRef("const", pkgName, elemName, ""),
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
						Ref:        p.buildRef("var", pkgName, elemName, ""),
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
func (p *GoCodeParser) parseTypeSpec(
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

	ref := p.buildRef(refPrefix, pkgName, name, "")

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

// extractStructTags extracts struct field tags from struct body.
func (p *GoCodeParser) extractStructTags(body string) []string {
	var tags []string

	// Simple regex-free extraction of backtick-quoted tags
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		// Look for backtick-quoted strings (struct tags)
		start := strings.Index(line, "`")
		if start == -1 {
			continue
		}
		end := strings.LastIndex(line, "`")
		if end <= start {
			continue
		}
		tag := line[start+1 : end]
		if tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}

// extractReceiverTypeInfo extracts the type name and pointer-ness from a method receiver.
func extractReceiverTypeInfo(expr ast.Expr) (typeName string, isPointer bool) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, false
	case *ast.StarExpr:
		name, _ := extractReceiverTypeInfo(t.X)
		return name, true
	}
	return "", false
}

// containsGoRoutineCall detects goroutine spawning patterns.
func containsGoRoutineCall(body string) bool {
	// Look for "go someFunc(" or "go obj.Method("
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "go ") {
			rest := strings.TrimPrefix(trimmed, "go ")
			// Should have a function call after "go "
			if strings.Contains(rest, "(") {
				return true
			}
		}
	}
	return false
}
