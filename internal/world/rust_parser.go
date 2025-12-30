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
	"github.com/smacker/go-tree-sitter/rust"
)

// RustCodeParser implements CodeParser for Rust source files.
// It uses Tree-sitter for accurate AST parsing.
type RustCodeParser struct {
	projectRoot string
	parser      *sitter.Parser
}

// NewRustCodeParser creates a new Rust parser.
func NewRustCodeParser(projectRoot string) *RustCodeParser {
	parser := sitter.NewParser()
	parser.SetLanguage(rust.GetLanguage())
	return &RustCodeParser{
		projectRoot: projectRoot,
		parser:      parser,
	}
}

// Language returns "rs" for Ref URI generation.
func (p *RustCodeParser) Language() string {
	return "rs"
}

// SupportedExtensions returns [".rs"].
func (p *RustCodeParser) SupportedExtensions() []string {
	return []string{".rs"}
}

// Parse extracts CodeElements from Rust source code.
func (p *RustCodeParser) Parse(path string, content []byte) ([]CodeElement, error) {
	start := time.Now()
	logging.WorldDebug("RustCodeParser: parsing file: %s", filepath.Base(path))

	tree, err := p.parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("RustCodeParser: parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	lines := strings.Split(string(content), "\n")
	relPath := p.relativePath(path)

	var elements []CodeElement
	root := tree.RootNode()

	// Default actions for all elements
	defaultActions := []ActionType{ActionView, ActionReplace, ActionInsertBefore, ActionInsertAfter, ActionDelete}

	// Track struct/enum refs for impl block linking
	typeRefs := make(map[string]string)

	// Walk tree to extract elements
	p.walkNode(root, path, relPath, "", content, lines, defaultActions, &elements, typeRefs)

	logging.WorldDebug("RustCodeParser: parsed %s - %d elements in %v",
		filepath.Base(path), len(elements), time.Since(start))
	return elements, nil
}

// walkNode recursively walks the AST and extracts CodeElements.
func (p *RustCodeParser) walkNode(
	node *sitter.Node,
	absPath, relPath, parentRef string,
	content []byte,
	lines []string,
	actions []ActionType,
	elements *[]CodeElement,
	typeRefs map[string]string,
) {
	getText := func(n *sitter.Node) string {
		return string(content[n.StartByte():n.EndByte()])
	}

	hasPubVisibility := func(n *sitter.Node) bool {
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "visibility_modifier" {
				text := getText(child)
				return strings.HasPrefix(text, "pub")
			}
		}
		return false
	}

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		nodeType := child.Type()

		switch nodeType {
		case "struct_item":
			elem := p.parseStructItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
				typeRefs[elem.Name] = elem.Ref
			}

		case "enum_item":
			elem := p.parseEnumItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
				typeRefs[elem.Name] = elem.Ref
			}

		case "trait_item":
			elem := p.parseTraitItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
				typeRefs[elem.Name] = elem.Ref
			}

		case "impl_item":
			// Parse impl block and its methods
			elems := p.parseImplItem(child, absPath, relPath, content, lines, actions, getText, typeRefs)
			*elements = append(*elements, elems...)

		case "function_item":
			elem := p.parseFuncItem(child, absPath, relPath, parentRef, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "mod_item":
			elem := p.parseModItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
				// Recurse into module body
				body := child.ChildByFieldName("body")
				if body != nil {
					p.walkNode(body, absPath, relPath, elem.Ref, content, lines, actions, elements, typeRefs)
				}
			}

		case "const_item":
			elem := p.parseConstItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "static_item":
			elem := p.parseStaticItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		case "type_item":
			elem := p.parseTypeItem(child, absPath, relPath, content, lines, actions, getText, hasPubVisibility(child))
			if elem != nil {
				*elements = append(*elements, *elem)
			}

		default:
			// Recurse into other nodes
			p.walkNode(child, absPath, relPath, parentRef, content, lines, actions, elements, typeRefs)
		}
	}
}

// parseStructItem parses a Rust struct definition.
func (p *RustCodeParser) parseStructItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementStruct,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseEnumItem parses a Rust enum definition.
func (p *RustCodeParser) parseEnumItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementType_, // Use Type_ for enums
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseTraitItem parses a Rust trait definition.
func (p *RustCodeParser) parseTraitItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementInterface, // Traits are like interfaces
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseImplItem parses a Rust impl block and its methods.
func (p *RustCodeParser) parseImplItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	typeRefs map[string]string,
) []CodeElement {
	var elements []CodeElement

	// Get the type being implemented
	typeNode := node.ChildByFieldName("type")
	if typeNode == nil {
		return elements
	}

	typeName := getText(typeNode)
	// Clean up generic parameters
	if idx := strings.Index(typeName, "<"); idx > 0 {
		typeName = typeName[:idx]
	}

	// Check if implementing a trait
	traitNode := node.ChildByFieldName("trait")
	traitName := ""
	if traitNode != nil {
		traitName = getText(traitNode)
	}

	// Find the parent ref
	parentRef := ""
	if ref, ok := typeRefs[typeName]; ok {
		parentRef = ref
	} else {
		parentRef = fmt.Sprintf("rs:%s:%s", relPath, typeName)
	}

	// Extract methods from impl body
	body := node.ChildByFieldName("body")
	if body == nil {
		return elements
	}

	for i := 0; i < int(body.NamedChildCount()); i++ {
		child := body.NamedChild(i)
		if child.Type() == "function_item" {
			elem := p.parseFuncItem(child, absPath, relPath, parentRef, content, lines, actions, getText, p.hasPubVisibility(child, content))
			if elem != nil {
				// Update ref to include type name
				elem.Ref = fmt.Sprintf("rs:%s:%s.%s", relPath, typeName, elem.Name)
				elem.Type = ElementMethod
				elem.Parent = parentRef

				// Store trait info in body metadata if implementing trait
				if traitName != "" {
					// We'll emit this in facts
				}

				elements = append(elements, *elem)
			}
		}
	}

	return elements
}

// hasPubVisibility checks if a node has pub visibility.
func (p *RustCodeParser) hasPubVisibility(node *sitter.Node, content []byte) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child.Type() == "visibility_modifier" {
			text := string(content[child.StartByte():child.EndByte()])
			return strings.HasPrefix(text, "pub")
		}
	}
	return false
}

// parseFuncItem parses a Rust function definition.
func (p *RustCodeParser) parseFuncItem(
	node *sitter.Node,
	absPath, relPath, parentRef string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	var ref string
	elemType := ElementFunction
	if parentRef != "" {
		parts := strings.Split(parentRef, ":")
		parentName := parts[len(parts)-1]
		ref = fmt.Sprintf("rs:%s:%s.%s", relPath, parentName, name)
		elemType = ElementMethod
	} else {
		ref = fmt.Sprintf("rs:%s:%s", relPath, name)
	}

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
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
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseModItem parses a Rust module definition.
func (p *RustCodeParser) parseModItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementType_, // Module as type
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseConstItem parses a Rust const definition.
func (p *RustCodeParser) parseConstItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementConst,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseStaticItem parses a Rust static definition.
func (p *RustCodeParser) parseStaticItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
		visibility = VisibilityPublic
	}

	return &CodeElement{
		Ref:        ref,
		Type:       ElementVar,
		File:       absPath,
		StartLine:  startLine,
		EndLine:    endLine,
		Signature:  signature,
		Body:       extractBody(lines, startLine, endLine),
		Visibility: visibility,
		Actions:    actions,
		Package:    "rust",
		Name:       name,
	}
}

// parseTypeItem parses a Rust type alias definition.
func (p *RustCodeParser) parseTypeItem(
	node *sitter.Node,
	absPath, relPath string,
	content []byte,
	lines []string,
	actions []ActionType,
	getText func(*sitter.Node) string,
	isPub bool,
) *CodeElement {
	nameNode := node.ChildByFieldName("name")
	if nameNode == nil {
		return nil
	}

	name := getText(nameNode)
	startLine := int(node.StartPoint().Row) + 1
	endLine := int(node.EndPoint().Row) + 1

	ref := fmt.Sprintf("rs:%s:%s", relPath, name)

	signature := ""
	if startLine > 0 && startLine <= len(lines) {
		signature = strings.TrimSpace(lines[startLine-1])
	}

	visibility := VisibilityPrivate
	if isPub {
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
		Package:    "rust",
		Name:       name,
	}
}

// EmitLanguageFacts generates Rust-specific Stratum 0 facts.
func (p *RustCodeParser) EmitLanguageFacts(elements []CodeElement) []core.Fact {
	var facts []core.Fact

	for _, elem := range elements {
		switch elem.Type {
		case ElementStruct:
			// rs_struct(Ref)
			facts = append(facts, core.Fact{
				Predicate: "rs_struct",
				Args:      []interface{}{elem.Ref},
			})

			// Check for derive macros
			derives := p.extractDerives(elem.Body)
			for _, derive := range derives {
				facts = append(facts, core.Fact{
					Predicate: "rs_derive",
					Args:      []interface{}{elem.Ref, derive},
				})
			}

			// Check for serde attributes (wire names)
			serdeAttrs := p.extractSerdeAttrs(elem.Body)
			for field, rename := range serdeAttrs {
				facts = append(facts, core.Fact{
					Predicate: "rs_serde_rename",
					Args:      []interface{}{elem.Ref, field, rename},
				})
			}

		case ElementInterface: // Rust trait
			// rs_trait(Ref)
			facts = append(facts, core.Fact{
				Predicate: "rs_trait",
				Args:      []interface{}{elem.Ref},
			})

		case ElementFunction, ElementMethod:
			// Check for async
			if strings.HasPrefix(elem.Signature, "async ") || strings.Contains(elem.Signature, " async ") {
				facts = append(facts, core.Fact{
					Predicate: "rs_async_fn",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Check for unsafe
			if strings.Contains(elem.Body, "unsafe {") || strings.HasPrefix(elem.Signature, "unsafe ") {
				facts = append(facts, core.Fact{
					Predicate: "rs_unsafe_block",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Method belongs to type
			if elem.Type == ElementMethod && elem.Parent != "" {
				facts = append(facts, core.Fact{
					Predicate: "method_of",
					Args:      []interface{}{elem.Ref, elem.Parent},
				})
			}

			// Check for Result return type (error handling)
			if strings.Contains(elem.Signature, "-> Result<") || strings.Contains(elem.Signature, "-> Result ") {
				facts = append(facts, core.Fact{
					Predicate: "rs_returns_result",
					Args:      []interface{}{elem.Ref},
				})
			}

			// Check for unwrap usage (potential panic)
			if strings.Contains(elem.Body, ".unwrap()") || strings.Contains(elem.Body, ".expect(") {
				facts = append(facts, core.Fact{
					Predicate: "rs_uses_unwrap",
					Args:      []interface{}{elem.Ref},
				})
			}
		}
	}

	return facts
}

// extractDerives extracts derive macro names from struct/enum.
func (p *RustCodeParser) extractDerives(body string) []string {
	var derives []string
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#[derive(") {
			// Extract content between derive( and )
			start := strings.Index(trimmed, "(")
			end := strings.LastIndex(trimmed, ")")
			if start > 0 && end > start {
				content := trimmed[start+1 : end]
				// Split by comma
				parts := strings.Split(content, ",")
				for _, part := range parts {
					derive := strings.TrimSpace(part)
					if derive != "" {
						derives = append(derives, derive)
					}
				}
			}
		}
	}
	return derives
}

// extractSerdeAttrs extracts serde rename attributes for wire names.
func (p *RustCodeParser) extractSerdeAttrs(body string) map[string]string {
	attrs := make(map[string]string)
	lines := strings.Split(body, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for #[serde(rename = "...")] patterns
		if strings.Contains(trimmed, "#[serde(") && strings.Contains(trimmed, "rename") {
			// Find the field name on the next line
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				// Extract field name (first word before : or ,)
				parts := strings.FieldsFunc(nextLine, func(r rune) bool {
					return r == ':' || r == ',' || r == ' '
				})
				if len(parts) > 0 {
					fieldName := strings.TrimPrefix(parts[0], "pub ")
					// Extract rename value
					if renameIdx := strings.Index(trimmed, "rename"); renameIdx >= 0 {
						afterRename := trimmed[renameIdx:]
						if quoteStart := strings.Index(afterRename, "\""); quoteStart >= 0 {
							afterQuote := afterRename[quoteStart+1:]
							if quoteEnd := strings.Index(afterQuote, "\""); quoteEnd >= 0 {
								rename := afterQuote[:quoteEnd]
								attrs[fieldName] = rename
							}
						}
					}
				}
			}
		}
	}
	return attrs
}

// relativePath returns the path relative to project root.
func (p *RustCodeParser) relativePath(absPath string) string {
	rel, err := filepath.Rel(p.projectRoot, absPath)
	if err != nil {
		return absPath
	}
	return filepath.ToSlash(rel)
}
