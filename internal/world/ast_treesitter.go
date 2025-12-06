package world

import (
	"context"
	"codenerd/internal/core"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TreeSitterParser handles AST parsing using tree-sitter
type TreeSitterParser struct {
	pythonParser *sitter.Parser
	rustParser   *sitter.Parser
	jsParser     *sitter.Parser
	tsParser     *sitter.Parser
}

// NewTreeSitterParser creates a new tree-sitter parser
func NewTreeSitterParser() *TreeSitterParser {
	return &TreeSitterParser{
		pythonParser: sitter.NewParser(),
		rustParser:   sitter.NewParser(),
		jsParser:     sitter.NewParser(),
		tsParser:     sitter.NewParser(),
	}
}

// ParsePython parses Python code using tree-sitter
func (p *TreeSitterParser) ParsePython(path string, content []byte) ([]core.Fact, error) {
	p.pythonParser.SetLanguage(python.GetLanguage())
	tree, err := p.pythonParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	// Walk the tree and extract symbols
	facts = append(facts, p.extractPythonSymbols(root, path, string(content))...)

	return facts, nil
}

// ParseRust parses Rust code using tree-sitter
func (p *TreeSitterParser) ParseRust(path string, content []byte) ([]core.Fact, error) {
	p.rustParser.SetLanguage(rust.GetLanguage())
	tree, err := p.rustParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	// Walk the tree and extract symbols
	facts = append(facts, p.extractRustSymbols(root, path, string(content))...)

	return facts, nil
}

// ParseJavaScript parses JavaScript code using tree-sitter
func (p *TreeSitterParser) ParseJavaScript(path string, content []byte) ([]core.Fact, error) {
	p.jsParser.SetLanguage(javascript.GetLanguage())
	tree, err := p.jsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	// Walk the tree and extract symbols
	facts = append(facts, p.extractJSSymbols(root, path, string(content))...)

	return facts, nil
}

// ParseTypeScript parses TypeScript code using tree-sitter
func (p *TreeSitterParser) ParseTypeScript(path string, content []byte) ([]core.Fact, error) {
	p.tsParser.SetLanguage(typescript.GetLanguage())
	tree, err := p.tsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	// Walk the tree and extract symbols
	facts = append(facts, p.extractTSSymbols(root, path, string(content))...)

	return facts, nil
}

// extractPythonSymbols walks the Python AST and extracts symbols
func (p *TreeSitterParser) extractPythonSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact

	// Helper to get node text
	getText := func(n *sitter.Node) string {
		return n.Content([]byte(content))
	}

	// Recursive walker
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "class_definition":
			// Extract class name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("class:%s", name)
				signature := getText(n.Child(0)) // Get the first line
				if len(signature) > 100 {
					signature = signature[:100] + "..."
				}

				// Determine visibility
				visibility := "public"
				if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") {
					visibility = "protected"
				} else if strings.HasPrefix(name, "__") {
					visibility = "private"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "class", visibility, path, signature},
				})
			}

		case "function_definition":
			// Extract function name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)

				// Get function signature
				paramsNode := n.ChildByFieldName("parameters")
				signature := fmt.Sprintf("def %s", name)
				if paramsNode != nil {
					signature = fmt.Sprintf("def %s%s", name, getText(paramsNode))
				}

				// Determine visibility
				visibility := "public"
				if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") {
					visibility = "protected"
				} else if strings.HasPrefix(name, "__") {
					visibility = "private"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "function", visibility, path, signature},
				})
			}

		case "import_statement", "import_from_statement":
			// Extract imports
			moduleName := ""
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child.Type() == "dotted_name" {
					moduleName = getText(child)
					break
				}
			}
			if moduleName != "" {
				facts = append(facts, core.Fact{
					Predicate: "dependency_link",
					Args:      []interface{}{path, fmt.Sprintf("mod:%s", moduleName), moduleName},
				})
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return facts
}

// extractRustSymbols walks the Rust AST and extracts symbols
func (p *TreeSitterParser) extractRustSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact

	// Helper to get node text
	getText := func(n *sitter.Node) string {
		return n.Content([]byte(content))
	}

	// Check if node has pub visibility
	hasPubVisibility := func(n *sitter.Node) bool {
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "visibility_modifier" && getText(child) == "pub" {
				return true
			}
		}
		return false
	}

	// Recursive walker
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "function_item":
			// Extract function name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("fn:%s", name)

				// Get function signature
				paramsNode := n.ChildByFieldName("parameters")
				returnType := n.ChildByFieldName("return_type")
				signature := fmt.Sprintf("fn %s", name)
				if paramsNode != nil {
					signature = fmt.Sprintf("fn %s%s", name, getText(paramsNode))
					if returnType != nil {
						signature += " " + getText(returnType)
					}
				}

				visibility := "private"
				if hasPubVisibility(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "function", visibility, path, signature},
				})
			}

		case "struct_item":
			// Extract struct name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("struct:%s", name)
				signature := fmt.Sprintf("struct %s", name)

				visibility := "private"
				if hasPubVisibility(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "struct", visibility, path, signature},
				})
			}

		case "enum_item":
			// Extract enum name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("enum:%s", name)
				signature := fmt.Sprintf("enum %s", name)

				visibility := "private"
				if hasPubVisibility(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "enum", visibility, path, signature},
				})
			}

		case "mod_item":
			// Extract module name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("mod:%s", name)
				signature := fmt.Sprintf("mod %s", name)

				visibility := "private"
				if hasPubVisibility(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "module", visibility, path, signature},
				})
			}

		case "use_declaration":
			// Extract use statements
			useTree := n.ChildByFieldName("argument")
			if useTree != nil {
				usePath := getText(useTree)
				// Extract the root crate/module name
				parts := strings.Split(usePath, "::")
				if len(parts) > 0 {
					crateName := parts[0]
					facts = append(facts, core.Fact{
						Predicate: "dependency_link",
						Args:      []interface{}{path, fmt.Sprintf("crate:%s", crateName), usePath},
					})
				}
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return facts
}

// extractJSSymbols walks the JavaScript AST and extracts symbols
func (p *TreeSitterParser) extractJSSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact

	// Helper to get node text
	getText := func(n *sitter.Node) string {
		return n.Content([]byte(content))
	}

	// Check if node has export
	hasExport := func(n *sitter.Node) bool {
		parent := n.Parent()
		if parent != nil && parent.Type() == "export_statement" {
			return true
		}
		return false
	}

	// Recursive walker
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "class_declaration":
			// Extract class name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("class:%s", name)
				signature := fmt.Sprintf("class %s", name)

				visibility := "private"
				if hasExport(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "class", visibility, path, signature},
				})
			}

		case "function_declaration":
			// Extract function name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)

				paramsNode := n.ChildByFieldName("parameters")
				signature := fmt.Sprintf("function %s", name)
				if paramsNode != nil {
					signature = fmt.Sprintf("function %s%s", name, getText(paramsNode))
				}

				visibility := "private"
				if hasExport(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "function", visibility, path, signature},
				})
			}

		case "lexical_declaration":
			// Handle const/let function declarations
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child.Type() == "variable_declarator" {
					nameNode := child.ChildByFieldName("name")
					valueNode := child.ChildByFieldName("value")

					if nameNode != nil && valueNode != nil {
						valueType := valueNode.Type()
						if valueType == "arrow_function" || valueType == "function" {
							name := getText(nameNode)
							id := fmt.Sprintf("func:%s", name)
							signature := fmt.Sprintf("const %s = ...", name)

							visibility := "private"
							if hasExport(n) {
								visibility = "public"
							}

							facts = append(facts, core.Fact{
								Predicate: "symbol_graph",
								Args:      []interface{}{id, "function", visibility, path, signature},
							})
						}
					}
				}
			}

		case "import_statement":
			// Extract imports
			sourceNode := n.ChildByFieldName("source")
			if sourceNode != nil {
				source := getText(sourceNode)
				source = strings.Trim(source, "\"'")
				facts = append(facts, core.Fact{
					Predicate: "dependency_link",
					Args:      []interface{}{path, fmt.Sprintf("mod:%s", source), source},
				})
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return facts
}

// extractTSSymbols walks the TypeScript AST and extracts symbols
func (p *TreeSitterParser) extractTSSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact

	// Helper to get node text
	getText := func(n *sitter.Node) string {
		return n.Content([]byte(content))
	}

	// Check if node has export
	hasExport := func(n *sitter.Node) bool {
		parent := n.Parent()
		if parent != nil && parent.Type() == "export_statement" {
			return true
		}
		return false
	}

	// Recursive walker
	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "class_declaration":
			// Extract class name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("class:%s", name)
				signature := fmt.Sprintf("class %s", name)

				visibility := "private"
				if hasExport(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "class", visibility, path, signature},
				})
			}

		case "interface_declaration":
			// Extract interface name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("interface:%s", name)
				signature := fmt.Sprintf("interface %s", name)

				visibility := "private"
				if hasExport(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "interface", visibility, path, signature},
				})
			}

		case "type_alias_declaration":
			// Extract type alias name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("type:%s", name)
				signature := fmt.Sprintf("type %s", name)

				visibility := "private"
				if hasExport(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "type", visibility, path, signature},
				})
			}

		case "function_declaration":
			// Extract function name
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)

				paramsNode := n.ChildByFieldName("parameters")
				returnType := n.ChildByFieldName("return_type")
				signature := fmt.Sprintf("function %s", name)
				if paramsNode != nil {
					signature = fmt.Sprintf("function %s%s", name, getText(paramsNode))
					if returnType != nil {
						signature += ": " + getText(returnType)
					}
				}

				visibility := "private"
				if hasExport(n) {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "function", visibility, path, signature},
				})
			}

		case "lexical_declaration":
			// Handle const/let function declarations
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child.Type() == "variable_declarator" {
					nameNode := child.ChildByFieldName("name")
					valueNode := child.ChildByFieldName("value")

					if nameNode != nil && valueNode != nil {
						valueType := valueNode.Type()
						if valueType == "arrow_function" || valueType == "function" {
							name := getText(nameNode)
							id := fmt.Sprintf("func:%s", name)
							signature := fmt.Sprintf("const %s = ...", name)

							visibility := "private"
							if hasExport(n) {
								visibility = "public"
							}

							facts = append(facts, core.Fact{
								Predicate: "symbol_graph",
								Args:      []interface{}{id, "function", visibility, path, signature},
							})
						}
					}
				}
			}

		case "import_statement":
			// Extract imports
			sourceNode := n.ChildByFieldName("source")
			if sourceNode != nil {
				source := getText(sourceNode)
				source = strings.Trim(source, "\"'")
				facts = append(facts, core.Fact{
					Predicate: "dependency_link",
					Args:      []interface{}{path, fmt.Sprintf("mod:%s", source), source},
				})
			}
		}

		// Recurse into children
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return facts
}

// Close releases resources held by the parser
func (p *TreeSitterParser) Close() {
	if p.pythonParser != nil {
		p.pythonParser.Close()
	}
	if p.rustParser != nil {
		p.rustParser.Close()
	}
	if p.jsParser != nil {
		p.jsParser.Close()
	}
	if p.tsParser != nil {
		p.tsParser.Close()
	}
}
