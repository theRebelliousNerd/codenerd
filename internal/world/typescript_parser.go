package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TypeScriptCodeParser implements CodeParser for TypeScript and JavaScript source files.
// It uses Tree-sitter for accurate AST parsing.
type TypeScriptCodeParser struct {
	projectRoot string
	tsParser    *sitter.Parser
	jsParser    *sitter.Parser
}

// NewTypeScriptCodeParser creates a new TypeScript/JavaScript parser.
func NewTypeScriptCodeParser(projectRoot string) *TypeScriptCodeParser {
	tsParser := sitter.NewParser()
	tsParser.SetLanguage(typescript.GetLanguage())
	jsParser := sitter.NewParser()
	jsParser.SetLanguage(javascript.GetLanguage())
	return &TypeScriptCodeParser{
		projectRoot: projectRoot,
		tsParser:    tsParser,
		jsParser:    jsParser,
	}
}

// Language returns "ts" for Ref URI generation.
func (p *TypeScriptCodeParser) Language() string {
	return "ts"
}

// SupportedExtensions returns TypeScript and JavaScript extensions.
func (p *TypeScriptCodeParser) SupportedExtensions() []string {
	return []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
}

// Parse extracts CodeElements from TypeScript/JavaScript source code.
func (p *TypeScriptCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	start := time.Now()
	logging.WorldDebug("TypeScriptCodeParser: parsing file: %s", filepath.Base(path))

	// Choose parser based on extension
	parser := p.tsParser
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".js" || ext == ".jsx" || ext == ".mjs" || ext == ".cjs" {
		parser = p.jsParser
	}

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("TypeScriptCodeParser: parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	lines := strings.Split(string(content), "\n")
	relPath := p.relativePath(path)

	var elements []CodeElement
	root := tree.RootNode()

	// Default actions for all elements
	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}

	// Track class refs for method parent linking
	classRefs := make(map[string]string)

	// Walk tree to extract elements
	p.walkNode(root, path, relPath, "", content, lines, defaultActions, &elements, classRefs)

	logging.WorldDebug("TypeScriptCodeParser: parsed %s - %d elements in %v",
		filepath.Base(path), len(elements), time.Since(start))
	return elements, nil
}

// walkNode recursively walks the AST and extracts CodeElements.
func (p *TypeScriptCodeParser) walkNode(
	node *sitter.Node,
	absPath, relPath, parentRef string,
	content []byte,
	lines []string,
	actions []ActionType,
	elements *[]CodeElement,
	classRefs map[string]string,
) {
	getText := func(n *sitter.Node) string {
		return string(content[n.StartByte():n.EndByte()])
	}

	hasExport := func(n *sitter.Node) bool {
		parent := n.Parent()
		if parent != nil && parent.Type() == "export_statement" {
			return true
		}
		return false
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		nodeType := child.Type()

		switch nodeType {
		case "class_declaration":
			elem := p.parseClassDecl(child, absPath, relPath, content, lines, actions, hasExport(child))
			if elem != nil {
				*elements = append(*elements, *elem)
				classRefs[elem.Name] = elem.Ref
				// Recurse into class body for methods
				body := child.ChildByFieldName("body")
				if body != nil {
					p.walkNode(body, absPath, relPath, elem.Ref, content, lines, actions, elements, classRefs)
				}
			}

		case "interface_declaration":
			elem := p.parseInterfaceDecl(child, absPath, relPath, content, lines, actions, hasExport(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "type_alias_declaration":
			elem := p.parseTypeAlias(child, absPath, relPath, content, lines, actions, hasExport(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "function_declaration":
			elem := p.parseFuncDecl(child, absPath, relPath, parentRef, content, lines, actions, getText, hasExport(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "method_definition":
			elem := p.parseMethodDef(child, absPath, relPath, parentRef, content, lines, actions, getText)
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "lexical_declaration", "variable_declaration":
			// Handle const/let/var declarations that might be arrow functions or React components
			elems := p.parseVarDecl(child, absPath, relPath, content, lines, actions, getText, hasExport(child))
			*elements = append(*elements, elems...)

		case "export_statement":
			// Recurse into exported declarations
			p.walkNode(child, absPath, relPath, parentRef, content, lines, actions, elements, classRefs)

		default:
			// Recurse into other compound statements
			p.walkNode(child, absPath, relPath, parentRef, content, lines, actions, elements, classRefs)
		}
	}
}

// parseClassDecl parses a TypeScript/JavaScript class declaration.
func (p *TypeScriptCodeParser) parseClassDecl(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	isExported bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := string(content[nameNode.StartByte():nameNode.EndByte()])
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	// Build repo-anchored ref
	ref := fmt.Sprintf("ts:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isExported {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementStruct, // Map TS class to struct
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "typescript",
		Name:       name,
	}
}

// parseInterfaceDecl parses a TypeScript interface declaration.
func (p *TypeScriptCodeParser) parseInterfaceDecl(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	isExported bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := string(content[nameNode.StartByte():nameNode.EndByte()])
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("ts:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isExported {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementInterface,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "typescript",
		Name:       name,
	}
}

// parseTypeAlias parses a TypeScript type alias declaration.
func (p *TypeScriptCodeParser) parseTypeAlias(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	isExported bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := string(content[nameNode.StartByte():nameNode.EndByte()])
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("ts:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isExported {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementType_,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "typescript",
		Name:       name,
	}
}

// parseFuncDecl parses a TypeScript/JavaScript function declaration.
func (p *TypeScriptCodeParser) parseFuncDecl(
	node *sitter.Node,
	absPath, relPath, parentRef string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isExported bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	var ref string
	if parentRef != "" {
		parts := strings.Split(parentRef, ":")
		parentName := parts[len(parts)-1]
		ref = fmt.Sprintf("ts:%s:%s.%s", relPath, parentName, name)
	} else {
		ref = fmt.Sprintf("ts:%s:%s", relPath, name)
	}

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isExported {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementFunction,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Parent:     parentRef,
		Visibility: visibility,
		Actions:    actions,
		Package:    "typescript",
		Name:       name,
	}
}

// parseMethodDef parses a class method definition.
func (p *TypeScriptCodeParser) parseMethodDef(
	node *sitter.Node,
	absPath, relPath, parentRef string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	var ref string
	if parentRef != "" {
		parts := strings.Split(parentRef, ":")
		parentName := parts[len(parts)-1]
		ref = fmt.Sprintf("ts:%s:%s.%s", relPath, parentName, name)
	} else {
		ref = fmt.Sprintf("ts:%s:%s", relPath, name)
	}

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	// Check for visibility modifiers in the method
	visibility := VisibilityPublic // Default for class methods
	body := extractBody(lines, startLine, endLine)
	if strings.Contains(signature, "private ") {
		visibility = VisibilityPrivate
	} else if strings.Contains(signature, "protected ") {
		visibility = VisibilityPrivate
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementMethod,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       body,
		Parent:     parentRef,
		Visibility: visibility,
		Actions:    actions,
		Package:    "typescript",
		Name:       name,
	}
}

// parseVarDecl parses variable declarations that might be arrow functions or React components.
func (p *TypeScriptCodeParser) parseVarDecl(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isExported bool,
) []CodeElement {
	var elements []CodeElement

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() != "variable_declarator" {
			continue
		}

		nameNode := child.ChildByFieldName("name")
		valueNode := child.ChildByFieldName("value")

		if nameNode == nil || valueNode == nil {
			continue
		}

		valueType := valueNode.Type()

		// Only extract arrow functions and function expressions
		if valueType != "arrow_function" && valueType != "function" && valueType != "function_expression" {
			continue
		}

		name := getText(nameNode)
		startLine := int(node.StartPoint().Row) + 1
		endLine := int(node.EndPoint().Row) + 1

		ref := fmt.Sprintf("ts:%s:%s", relPath, name)

		signature := ""
		if startLine > 0 && startLine <= len(lines) {
			signature = strings.TrimSpace(lines[startLine-1])
		}

		visibility := VisibilityPrivate
		if isExported {
			visibility = VisibilityPublic
		}

		// Detect if this is a React component (PascalCase name with JSX return)
		elemType := ElementFunction
		body := extractBody(lines, startLine, endLine)
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			// Could be a React component - check for JSX
			if strings.Contains(body, "<") && strings.Contains(body, "/>") {
				// Likely a React component
				elemType = ElementFunction // Still a function, but we'll emit ts_component fact
			}
		}

		elements = append(elements, CodeElement{
			Ref:        ref,
			Type:       elemType,
			File:       absPath,
			StartLine:  startLine,
			EndLine:    endLine,
			Signature:  signature,
			Body:       body,
			Visibility: visibility,
			Actions:    actions,
			Package:    "typescript",
			Name:       name,
		})
	}

	return elements
}

// EmitLanguageFacts generates TypeScript/JavaScript-specific Stratum 0 facts.
func (p *TypeScriptCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	var facts []core.Fact

	for _, elem := range elements {
		switch elem.Type {
		case ElementStruct: // TypeScript class
			// ts_class(Ref)
			facts = append(facts, core.Fact{
				Predicate: "ts_class",
				Args:      []interface{}{elem.Ref},
			})

			// Check for extends
			if strings.Contains(elem.Signature, "extends") {
				facts = append(facts, core.Fact{
					Predicate: "ts_extends",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Check for implements
			if strings.Contains(elem.Signature, "implements") {
				facts = append(facts, core.Fact{
					Predicate: "ts_implements",
					Args:      []interface{}{elem.Ref},
				})
			}

		case ElementInterface:
			// ts_interface(Ref)
			facts = append(facts, core.Fact{
				Predicate: "ts_interface",
				Args:      []interface{}{elem.Ref},
			})

			// Extract interface properties for wire name inference
			props := p.extractInterfaceProps(elem.Body)
			for _, prop := range props {
				facts = append(facts, core.Fact{
					Predicate: "ts_interface_prop",
					Args:      []interface{}{elem.Ref, prop},
				})
			}

		case ElementType_:
			// ts_type_alias(Ref)
			facts = append(facts, core.Fact{
				Predicate: "ts_type_alias",
				Args:      []interface{}{elem.Ref},
			})

		case ElementFunction:
			// Check for async
			if strings.HasPrefix(elem.Signature, "async ") || strings.Contains(elem.Signature, " async ") {
				facts = append(facts, core.Fact{
					Predicate: "ts_async_function",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Check if this is a React component (PascalCase + JSX)
			if len(elem.Name) > 0 && elem.Name[0] >= 'A' && elem.Name[0] <= 'Z' {
				if strings.Contains(elem.Body, "<") && (strings.Contains(elem.Body, "/>") || strings.Contains(elem.Body, "</")) {
					facts = append(facts, core.Fact{
						Predicate: "ts_component",
						Args:      []interface{}{elem.Ref, elem.Name},
					})
				}
			}

			// Detect React hooks usage
			hooks := p.extractHookCalls(elem.Body)
			for _, hook := range hooks {
				facts = append(facts, core.Fact{
					Predicate: "ts_hook",
					Args:      []interface{}{elem.Ref, hook},
				})
			}

		case ElementMethod:
			// Method belongs to class
			if elem.Parent != "" {
				facts = append(facts, core.Fact{
					Predicate: "method_of",
					Args:      []interface{}{elem.Ref, elem.Parent},
				})
			}

			// Check for async
			if strings.HasPrefix(elem.Signature, "async ") || strings.Contains(elem.Signature, " async ") {
				facts = append(facts, core.Fact{
					Predicate: "ts_async_function",
					Args:      []interface{}{elem.Ref},
				})
			}
		}
	}

	return facts
}

// extractInterfaceProps extracts property names from interface body.
func (p *TypeScriptCodeParser) extractInterfaceProps(body string) []string {
	var props []string
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip interface declaration line
		if strings.HasPrefix(trimmed, "interface ") || trimmed == "{" || trimmed == "}" {
			continue
		}
		// Extract property name before ':'
		if colonIdx := strings.Index(trimmed, ":"); colonIdx > 0 {
			propName := strings.TrimSpace(trimmed[:colonIdx])
			// Remove optional marker
			propName = strings.TrimSuffix(propName, "?")
			// Remove readonly
			propName = strings.TrimPrefix(propName, "readonly ")
			if propName != "" && !strings.HasPrefix(propName, "//") {
				props = append(props, propName)
			}
		}
	}
	return props
}

// extractHookCalls extracts React hook calls from function body.
func (p *TypeScriptCodeParser) extractHookCalls(body string) []string {
	var hooks []string
	hookPatterns := []string{
		"useState", "useEffect", "useContext", "useReducer",
		"useCallback", "useMemo", "useRef", "useImperativeHandle",
		"useLayoutEffect", "useDebugValue", "useDeferredValue",
		"useTransition", "useId", "useSyncExternalStore",
	}

	for _, hook := range hookPatterns {
		if strings.Contains(body, hook+"(") {
			hooks = append(hooks, hook)
		}
	}

	return hooks
}

// relativePath returns the path relative to project root.
func (p *TypeScriptCodeParser) relativePath(absPath string) string {
	rel, err := filepath.Rel(p.projectRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}
