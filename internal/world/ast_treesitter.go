package world

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// TreeSitterParser handles AST parsing using tree-sitter
type TreeSitterParser struct {
	goParser     *sitter.Parser
	pythonParser *sitter.Parser
	rustParser   *sitter.Parser
	jsParser     *sitter.Parser
	tsParser     *sitter.Parser
}

// NewTreeSitterParser creates a new tree-sitter parser
func NewTreeSitterParser() *TreeSitterParser {
	logging.WorldDebug("Creating new TreeSitterParser")
	return &TreeSitterParser{
		goParser:     sitter.NewParser(),
		pythonParser: sitter.NewParser(),
		rustParser:   sitter.NewParser(),
		jsParser:     sitter.NewParser(),
		tsParser:     sitter.NewParser(),
	}
}

// Close releases resources held by the parser
func (p *TreeSitterParser) Close() {
	logging.WorldDebug("Closing TreeSitterParser")
	p.goParser.Close()
	p.pythonParser.Close()
	p.rustParser.Close()
	p.jsParser.Close()
	p.tsParser.Close()
}

// ParseGo parses Go code using tree-sitter
func (p *TreeSitterParser) ParseGo(path string, content []byte) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("TreeSitter: parsing Go file: %s (%d bytes)", filepath.Base(path), len(content))

	p.goParser.SetLanguage(golang.GetLanguage())
	tree, err := p.goParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("TreeSitter: Go parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	// Walk the tree and extract symbols
	facts = append(facts, p.extractGoSymbols(root, path, string(content))...)

	logging.WorldDebug("TreeSitter: Go parsed %s - %d facts in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

// extractGoSymbols walks the Go AST and extracts symbols
func (p *TreeSitterParser) extractGoSymbols(node *sitter.Node, path, content string) []core.Fact {
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
		case "function_declaration":
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)

				paramsNode := n.ChildByFieldName("parameters")
				resultNode := n.ChildByFieldName("result")
				signature := fmt.Sprintf("func %s", name)
				if paramsNode != nil {
					signature = fmt.Sprintf("func %s%s", name, getText(paramsNode))
				}
				if resultNode != nil {
					signature += " " + getText(resultNode)
				}

				visibility := "private"
				if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "function", visibility, path, signature},
				})
			}

		case "method_declaration":
			nameNode := n.ChildByFieldName("name")
			receiverNode := n.ChildByFieldName("receiver")
			if nameNode != nil && receiverNode != nil {
				name := getText(nameNode)
				receiver := getText(receiverNode)
				id := fmt.Sprintf("method:%s.%s", receiver, name)

				paramsNode := n.ChildByFieldName("parameters")
				resultNode := n.ChildByFieldName("result")
				signature := fmt.Sprintf("func %s %s", receiver, name)
				if paramsNode != nil {
					signature += getText(paramsNode)
				}
				if resultNode != nil {
					signature += " " + getText(resultNode)
				}

				visibility := "private"
				if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
					visibility = "public"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "method", visibility, path, signature},
				})
			}

		case "type_declaration":
			for i := 0; i < int(n.NamedChildCount()); i++ {
				spec := n.NamedChild(i)
				if spec.Type() == "type_spec" {
					nameNode := spec.ChildByFieldName("name")
					typeNode := spec.ChildByFieldName("type")

					if nameNode != nil {
						name := getText(nameNode)
						kind := "type"
						signature := fmt.Sprintf("type %s", name)
						visibility := "private"
						if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
							visibility = "public"
						}

						var fields []string
						var methods []string

						if typeNode != nil {
							if typeNode.Type() == "struct_type" {
								kind = "struct"
								signature += " struct"
								// Extract fields
								block := typeNode.ChildByFieldName("fields")
								if block != nil {
									for j := 0; j < int(block.NamedChildCount()); j++ {
										fieldDecl := block.NamedChild(j)
										if fieldDecl.Type() == "field_declaration" {
											fieldNameNode := fieldDecl.ChildByFieldName("name")
											fieldTypeNode := fieldDecl.ChildByFieldName("type")
											var fieldName, fieldType string
											if fieldNameNode != nil {
												fieldName = getText(fieldNameNode)
											}
											if fieldTypeNode != nil {
												fieldType = getText(fieldTypeNode)
											}
											if fieldName != "" {
												fields = append(fields, fmt.Sprintf("%s %s", fieldName, fieldType))
												// Emit field fact
												fieldID := fmt.Sprintf("field:%s.%s", name, fieldName)
												fieldVis := "private"
												if len(fieldName) > 0 && fieldName[0] >= 'A' && fieldName[0] <= 'Z' {
													fieldVis = "public"
												}
												facts = append(facts, core.Fact{
													Predicate: "symbol_graph",
													Args:      []interface{}{fieldID, "field", fieldVis, path, fmt.Sprintf("%s %s", fieldName, fieldType)},
												})
											}
										}
									}
								}
							} else if typeNode.Type() == "interface_type" {
								kind = "interface"
								signature += " interface"
								// Extract methods
								// methods are direct children of interface_type
								for j := 0; j < int(typeNode.NamedChildCount()); j++ {
									methodSpec := typeNode.NamedChild(j)
									if methodSpec.Type() == "method_spec" {
										methodNameNode := methodSpec.ChildByFieldName("name")
										paramsNode := methodSpec.ChildByFieldName("parameters")
										resultNode := methodSpec.ChildByFieldName("result")

										if methodNameNode != nil {
											methodName := getText(methodNameNode)
											methodSig := fmt.Sprintf("%s", methodName)
											if paramsNode != nil {
												methodSig += getText(paramsNode)
											}
											if resultNode != nil {
												methodSig += " " + getText(resultNode)
											}
											methods = append(methods, methodSig)

											// Emit interface method fact
											methodID := fmt.Sprintf("iface_method:%s.%s", name, methodName)
											methodVis := "private"
											if len(methodName) > 0 && methodName[0] >= 'A' && methodName[0] <= 'Z' {
												methodVis = "public"
											}
											facts = append(facts, core.Fact{
												Predicate: "symbol_graph",
												Args:      []interface{}{methodID, "interface_method", methodVis, path, methodSig},
											})
										}
									}
								}
							}
						}

						id := fmt.Sprintf("%s:%s", kind, name)
						facts = append(facts, core.Fact{
							Predicate: "symbol_graph",
							Args:      []interface{}{id, kind, visibility, path, signature},
						})
					}
				}
			}

		case "import_declaration":
			for i := 0; i < int(n.NamedChildCount()); i++ {
				spec := n.NamedChild(i)
				if spec.Type() == "import_spec" {
					pathNode := spec.ChildByFieldName("path")
					if pathNode != nil {
						importPath := strings.Trim(getText(pathNode), "\"")
						facts = append(facts, core.Fact{
							Predicate: "dependency_link",
							Args:      []interface{}{path, fmt.Sprintf("pkg:%s", importPath), importPath},
						})
					}
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}

	walk(node)
	return facts
}

// ParsePython parses Python code using tree-sitter
func (p *TreeSitterParser) ParsePython(path string, content []byte) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("TreeSitter: parsing Python file: %s (%d bytes)", filepath.Base(path), len(content))

	p.pythonParser.SetLanguage(python.GetLanguage())
	tree, err := p.pythonParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("TreeSitter: Python parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	facts = append(facts, p.extractPythonSymbols(root, path, string(content))...)
	logging.WorldDebug("TreeSitter: Python parsed %s - %d facts in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

// extractPythonSymbols walks the Python AST and extracts symbols
func (p *TreeSitterParser) extractPythonSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact
	getText := func(n *sitter.Node) string { return n.Content([]byte(content)) }

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "class_definition":
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("class:%s", name)
				signature := getText(n.Child(0))
				if len(signature) > 100 {
					signature = signature[:100] + "..."
				}
				visibility := "public"
				if strings.HasPrefix(name, "_") {
					visibility = "protected"
				}
				if strings.HasPrefix(name, "__") {
					visibility = "private"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "class", visibility, path, signature},
				})
			}
		case "function_definition":
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)
				paramsNode := n.ChildByFieldName("parameters")
				signature := fmt.Sprintf("def %s", name)
				if paramsNode != nil {
					signature += getText(paramsNode)
				}
				visibility := "public"
				if strings.HasPrefix(name, "_") {
					visibility = "protected"
				}
				if strings.HasPrefix(name, "__") {
					visibility = "private"
				}

				facts = append(facts, core.Fact{
					Predicate: "symbol_graph",
					Args:      []interface{}{id, "function", visibility, path, signature},
				})
			}
		case "import_statement", "import_from_statement":
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child.Type() == "dotted_name" {
					moduleName := getText(child)
					facts = append(facts, core.Fact{
						Predicate: "dependency_link",
						Args:      []interface{}{path, fmt.Sprintf("mod:%s", moduleName), moduleName},
					})
				}
			}
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return facts
}

// ParseRust parses Rust code using tree-sitter
func (p *TreeSitterParser) ParseRust(path string, content []byte) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("TreeSitter: parsing Rust file: %s (%d bytes)", filepath.Base(path), len(content))

	p.rustParser.SetLanguage(rust.GetLanguage())
	tree, err := p.rustParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("TreeSitter: Rust parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	facts = append(facts, p.extractRustSymbols(root, path, string(content))...)
	logging.WorldDebug("TreeSitter: Rust parsed %s - %d facts in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

// extractRustSymbols walks the Rust AST and extracts symbols
func (p *TreeSitterParser) extractRustSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact
	getText := func(n *sitter.Node) string { return n.Content([]byte(content)) }
	hasPubVisibility := func(n *sitter.Node) bool {
		for i := 0; i < int(n.ChildCount()); i++ {
			if n.Child(i).Type() == "visibility_modifier" && getText(n.Child(i)) == "pub" {
				return true
			}
		}
		return false
	}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "function_item":
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("fn:%s", name)
				paramsNode := n.ChildByFieldName("parameters")
				signature := fmt.Sprintf("fn %s", name)
				if paramsNode != nil {
					signature += getText(paramsNode)
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
			useTree := n.ChildByFieldName("argument")
			if useTree != nil {
				usePath := getText(useTree)
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
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return facts
}

// ParseJavaScript parses JavaScript code using tree-sitter
func (p *TreeSitterParser) ParseJavaScript(path string, content []byte) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("TreeSitter: parsing JavaScript file: %s (%d bytes)", filepath.Base(path), len(content))

	p.jsParser.SetLanguage(javascript.GetLanguage())
	tree, err := p.jsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("TreeSitter: JavaScript parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	facts = append(facts, p.extractJSSymbols(root, path, string(content))...)
	logging.WorldDebug("TreeSitter: JavaScript parsed %s - %d facts in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

// extractJSSymbols walks the JavaScript AST and extracts symbols
func (p *TreeSitterParser) extractJSSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact
	getText := func(n *sitter.Node) string { return n.Content([]byte(content)) }
	hasExport := func(n *sitter.Node) bool {
		parent := n.Parent()
		if parent != nil && parent.Type() == "export_statement" {
			return true
		}
		return false
	}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "class_declaration":
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
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)
				paramsNode := n.ChildByFieldName("parameters")
				signature := fmt.Sprintf("function %s", name)
				if paramsNode != nil {
					signature += getText(paramsNode)
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
			for i := 0; i < int(n.NamedChildCount()); i++ {
				child := n.NamedChild(i)
				if child.Type() == "variable_declarator" {
					nameNode := child.ChildByFieldName("name")
					valueNode := child.ChildByFieldName("value")
					if nameNode != nil && valueNode != nil {
						if valueNode.Type() == "arrow_function" || valueNode.Type() == "function" {
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
			sourceNode := n.ChildByFieldName("source")
			if sourceNode != nil {
				source := strings.Trim(getText(sourceNode), "\"'")
				facts = append(facts, core.Fact{
					Predicate: "dependency_link",
					Args:      []interface{}{path, fmt.Sprintf("mod:%s", source), source},
				})
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return facts
}

// ParseTypeScript parses TypeScript code using tree-sitter
func (p *TreeSitterParser) ParseTypeScript(path string, content []byte) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("TreeSitter: parsing TypeScript file: %s (%d bytes)", filepath.Base(path), len(content))

	p.tsParser.SetLanguage(typescript.GetLanguage())
	tree, err := p.tsParser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("TreeSitter: TypeScript parse failed: %s - %v", path, err)
		return nil, err
	}
	defer tree.Close()

	var facts []core.Fact
	root := tree.RootNode()

	facts = append(facts, p.extractTSSymbols(root, path, string(content))...)
	logging.WorldDebug("TreeSitter: TypeScript parsed %s - %d facts in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}

// extractTSSymbols walks the TypeScript AST and extracts symbols
func (p *TreeSitterParser) extractTSSymbols(node *sitter.Node, path, content string) []core.Fact {
	var facts []core.Fact
	getText := func(n *sitter.Node) string { return n.Content([]byte(content)) }
	hasExport := func(n *sitter.Node) bool {
		parent := n.Parent()
		if parent != nil && parent.Type() == "export_statement" {
			return true
		}
		return false
	}

	var walk func(*sitter.Node)
	walk = func(n *sitter.Node) {
		nodeType := n.Type()

		switch nodeType {
		case "class_declaration":
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
			nameNode := n.ChildByFieldName("name")
			if nameNode != nil {
				name := getText(nameNode)
				id := fmt.Sprintf("func:%s", name)
				paramsNode := n.ChildByFieldName("parameters")
				signature := fmt.Sprintf("function %s", name)
				if paramsNode != nil {
					signature += getText(paramsNode)
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
		case "interface_declaration":
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
		case "import_statement":
			sourceNode := n.ChildByFieldName("source")
			if sourceNode != nil {
				source := strings.Trim(getText(sourceNode), "\"'")
				facts = append(facts, core.Fact{
					Predicate: "dependency_link",
					Args:      []interface{}{path, fmt.Sprintf("mod:%s", source), source},
				})
			}
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			walk(n.Child(i))
		}
	}
	walk(node)
	return facts
}
