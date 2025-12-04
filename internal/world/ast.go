package world

import (
	"codenerd/internal/core"
	"fmt"
	"go/parser"
	"go/token"
	"strings"
)

// ASTParser handles code parsing.
type ASTParser struct{}

func NewASTParser() *ASTParser {
	return &ASTParser{}
}

func (p *ASTParser) Parse(path string) ([]core.Fact, error) {
	if strings.HasSuffix(path, ".go") {
		return p.parseGo(path)
	}
	// TODO: Support other languages
	return nil, nil
}

func (p *ASTParser) parseGo(path string) ([]core.Fact, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var facts []core.Fact
	pkgName := node.Name.Name

	// 1. Package Symbol
	facts = append(facts, core.Fact{
		Predicate: "symbol_graph",
		Args: []interface{}{
			fmt.Sprintf("pkg:%s", pkgName),
			"package",
			"public",
			path,
			"package " + pkgName,
		},
	})

	// 2. Imports (Dependencies)
	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		facts = append(facts, core.Fact{
			Predicate: "dependency_link",
			Args: []interface{}{
				fmt.Sprintf("pkg:%s", pkgName),
				fmt.Sprintf("pkg:%s", importPath), // Simplified ID
				importPath,
			},
		})
	}

	// 3. Declarations
	for _, decl := range node.Decls {
		// TODO: Extract functions, types, etc.
		_ = decl
	}

	return facts, nil
}
