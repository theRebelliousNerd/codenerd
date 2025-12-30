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
	"github.com/smacker/go-tree-sitter/python"
)

// PythonCodeParser implements CodeParser for Python source files.
// It uses Tree-sitter for accurate AST parsing.
type PythonCodeParser struct {
	projectRoot string
	parser      *sitter.Parser
}

// NewPythonCodeParser creates a new Python parser.
func NewPythonCodeParser(projectRoot string) *PythonCodeParser {
	parser := sitter.NewParser()
	parser.SetLanguage(python.GetLanguage())
	return &PythonCodeParser{
		projectRoot: projectRoot,
		parser:      parser,
	}
}

// Language returns "py" for Ref URI generation.
func (p *PythonCodeParser) Language() string {
	return "py"
}

// SupportedExtensions returns [".py", ".pyw"].
func (p *PythonCodeParser) SupportedExtensions() []string {
	return []string{".py", ".pyw"}
}

// Parse extracts CodeElements from Python source code.
func (p *PythonCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	start := time.Now()
	logging.WorldDebug("PythonCodeParser: parsing file: %s", filepath.Base(path))

	tree, err := p.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("PythonCodeParser: parse failed: %s - %v", path, err)
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

	logging.WorldDebug("PythonCodeParser: parsed %s - %d elements in %v",
		filepath.Base(path), len(elements), time.Since(start))
	return elements, nil
}

// walkNode recursively walks the AST and extracts CodeElements.
func (p *PythonCodeParser) walkNode(
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

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		nodeType := child.Type()

		switch nodeType {
		case "class_definition":
			elem := p.parseClassDef(child, absPath, relPath, content, lines, actions)
			if elem != nil {
				*elements = append(*elements, *elem)
				classRefs[elem.Name] = elem.Ref
				// Recurse into class body for methods
				body := child.ChildByFieldName("body")
				if body != nil {
					p.walkNode(body, absPath, relPath, elem.Ref, content, lines, actions, elements, classRefs)
				}
			}

		case "function_definition":
			elem := p.parseFuncDef(child, absPath, relPath, parentRef, content, lines, actions, getText)
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "decorated_definition":
			// Handle decorated functions/classes
			for j := 0; j < int(child.NamedChildCount()); j++ {
				inner := child.NamedChild(j)
				if inner.Type() == "function_definition" {
					elem := p.parseFuncDef(inner, absPath, relPath, parentRef, content, lines, actions, getText)
					if elem != nil {
						// Extend start line to include decorators
						elem.StartLine = int(child.StartPoint().Row) + 1
						elem.Body = extractBody(lines, elem.StartLine, elem.EndLine)
						*elements = append(*elements, *elem)
					}
				} else if inner.Type() == "class_definition" {
					elem := p.parseClassDef(inner, absPath, relPath, content, lines, actions)
					if elem != nil {
						elem.StartLine = int(child.StartPoint().Row) + 1
						elem.Body = extractBody(lines, elem.StartLine, elem.EndLine)
						*elements = append(*elements, *elem)
						classRefs[elem.Name] = elem.Ref
						body := inner.ChildByFieldName("body")
						if body != nil {
							p.walkNode(body, absPath, relPath, elem.Ref, content, lines, actions, elements, classRefs)
						}
					}
				}
			}

		default:
			// Recurse into other compound statements
			p.walkNode(child, absPath, relPath, parentRef, content, lines, actions, elements, classRefs)
		}
	}
}

// parseClassDef parses a Python class definition.
func (p *PythonCodeParser) parseClassDef(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := string(content[nameNode.StartByte():nameNode.EndByte()])
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	// Build repo-anchored ref
	ref := fmt.Sprintf("py:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementStruct, // Map Python class to struct
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: p.determineVisibility(name),
		Actions:    actions,
		Package:    "python",
		Name:       name,
	}
}

// parseFuncDef parses a Python function/method definition.
func (p *PythonCodeParser) parseFuncDef(
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

	// Determine if method or function
	elemType := ElementFunction
	var ref string
	if parentRef != "" {
		elemType = ElementMethod
		// Extract parent name from ref
		parts := strings.Split(parentRef, ":")
		parentName := parts[len(parts)-1]
		ref = fmt.Sprintf("py:%s:%s.%s", relPath, parentName, name)
	} else {
		ref = fmt.Sprintf("py:%s:%s", relPath, name)
	}

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	return &CodeElement{
		Ref:        ref,
		Type:       elemType,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Parent:     parentRef,
		Visibility: p.determineVisibility(name),
		Actions:    actions,
		Package:    "python",
		Name:       name,
	}
}

// EmitLanguageFacts generates Python-specific Stratum 0 facts.
func (p *PythonCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	var facts []core.Fact

	for _, elem := range elements {
		switch elem.Type {
		case ElementStruct: // Python class
			// py_class(Ref)
			facts = append(facts, core.Fact{
				Predicate: "py_class",
				Args:      []interface{}{elem.Ref},
			})

			// Check for common base classes
			if strings.Contains(elem.Body, "BaseModel") || strings.Contains(elem.Body, "pydantic") {
				facts = append(facts, core.Fact{
					Predicate: "has_pydantic_base",
					Args:      []interface{}{elem.Ref},
				})
			}
			if strings.Contains(elem.Body, "@dataclass") {
				facts = append(facts, core.Fact{
					Predicate: "py_decorator",
					Args:      []interface{}{elem.Ref, "dataclass"},
				})
			}

		case ElementFunction, ElementMethod:
			// Check for async
			if strings.HasPrefix(elem.Signature, "async ") {
				facts = append(facts, core.Fact{
					Predicate: "py_async_def",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Extract decorators from body
			decorators := p.extractDecorators(elem.Body)
			for _, dec := range decorators {
				facts = append(facts, core.Fact{
					Predicate: "py_decorator",
					Args:      []interface{}{elem.Ref, dec},
				})
			}

			// Check for common patterns
			if strings.Contains(elem.Signature, "self") && elem.Type == ElementMethod {
				facts = append(facts, core.Fact{
					Predicate: "method_of",
					Args:      []interface{}{elem.Ref, elem.Parent},
				})
			}

			// Detect type hints for return type
			if strings.Contains(elem.Signature, "->") {
				facts = append(facts, core.Fact{
					Predicate: "py_typed_function",
					Args:      []interface{}{elem.Ref},
				})
			}
		}
	}

	return facts
}

// extractDecorators extracts decorator names from function/class body.
func (p *PythonCodeParser) extractDecorators(body string) []string {
	var decorators []string
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@") {
			// Extract decorator name
			dec := strings.TrimPrefix(trimmed, "@")
			// Remove arguments if present
			if idx := strings.Index(dec, "("); idx > 0 {
				dec = dec[:idx]
			}
			if dec != "" {
				decorators = append(decorators, dec)
			}
		}
	}
	return decorators
}

// determineVisibility returns visibility based on Python naming conventions.
func (p *PythonCodeParser) determineVisibility(name string) Visibility {
	if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
		return VisibilityPrivate // Name mangled
	}
	if strings.HasPrefix(name, "_") {
		return VisibilityPrivate // Convention private
	}
	return VisibilityPublic
}

// relativePath returns the path relative to project root.
func (p *PythonCodeParser) relativePath(absPath string) string {
	rel, err := filepath.Rel(p.projectRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}
