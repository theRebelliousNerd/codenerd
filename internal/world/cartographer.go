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

// Cartographer implements the "Holographic" Code Graph projection.
// It parses code to emit rich structural facts:
// - code_defines(File, Symbol, Type, StartLine, EndLine)
// - code_calls(Caller, Callee)
// - code_implements(Struct, Interface)
//
// It also emits legacy atoms for backward compatibility:
// - symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
// - dependency_link(CallerID, CalleeID, ImportPath)
type Cartographer struct {
}

// NewCartographer creates a new Cartographer for holographic code graph projection.
func NewCartographer() *Cartographer {
	logging.WorldDebug("Creating new Cartographer")
	return &Cartographer{}
}

// MapFile parses a single file and returns holographic facts.
// Currently supports Go with deep AST analysis.
func (c *Cartographer) MapFile(path string) ([]core.Fact, error) {
	logging.WorldDebug("Cartographer mapping file: %s", filepath.Base(path))
	ext := filepath.Ext(path)
	if ext == ".go" {
		return c.mapGoFile(path)
	}
	logging.WorldDebug("Cartographer: unsupported file type %s for %s", ext, filepath.Base(path))
	return nil, nil
}

func (c *Cartographer) mapGoFile(path string) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("Cartographer: mapping Go file: %s", filepath.Base(path))

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Cartographer: Go parse failed: %s - %v", path, err)
		return nil, err
	}

	var facts []core.Fact
	pkgName := node.Name.Name
	logging.WorldDebug("Cartographer: package=%s for %s", pkgName, filepath.Base(path))

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
	logging.WorldDebug("Cartographer: found %d imports", len(node.Imports))

	// Track current function for call graph
	var currentFunction string

	ast.Inspect(node, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// Definition
			name := x.Name.Name
			recv := ""
			if x.Recv != nil {
				// Method
				for _, field := range x.Recv.List {
					if typeIdent, ok := field.Type.(*ast.Ident); ok {
						recv = typeIdent.Name
					} else if starExpr, ok := field.Type.(*ast.StarExpr); ok {
						if typeIdent, ok := starExpr.X.(*ast.Ident); ok {
							recv = typeIdent.Name
						}
					}
				}
			}

			id := fmt.Sprintf("%s.%s", pkgName, name)
			if recv != "" {
				id = fmt.Sprintf("%s.%s.%s", pkgName, recv, name)
			}
			currentFunction = id

			start := fset.Position(x.Pos()).Line
			end := fset.Position(x.End()).Line

			visibility := "private"
			if ast.IsExported(name) {
				visibility = "public"
			}

			// New Holographic Atom
		facts = append(facts, core.Fact{
				Predicate: "code_defines",
				Args: []interface{}{
					path,
					core.MangleAtom(id),
					core.MangleAtom("/function"),
					int64(start),
					int64(end),
				},
			})

			// Legacy Atom
		facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args: []interface{}{
					id,
					"function",
					visibility,
					path,
					fmt.Sprintf("func %s", name),
				},
			})

		case *ast.TypeSpec:
			// Type definition (Struct/Interface)
			name := x.Name.Name
			id := fmt.Sprintf("%s.%s", pkgName, name)
			start := fset.Position(x.Pos()).Line
			end := fset.Position(x.End()).Line

			typeType := "/type"
			legacyType := "type"
			if _, ok := x.Type.(*ast.StructType); ok {
				typeType = "/struct"
				legacyType = "struct"
			} else if _, ok := x.Type.(*ast.InterfaceType); ok {
				typeType = "/interface"
				legacyType = "interface"
			}

			visibility := "private"
			if ast.IsExported(name) {
				visibility = "public"
			}

			// New Holographic Atom
		facts = append(facts, core.Fact{
				Predicate: "code_defines",
				Args: []interface{}{
					path,
					core.MangleAtom(id),
					core.MangleAtom(typeType),
					int64(start),
					int64(end),
				},
			})

			// Legacy Atom
		facts = append(facts, core.Fact{
				Predicate: "symbol_graph",
				Args: []interface{}{
					id,
					legacyType,
					visibility,
					path,
					fmt.Sprintf("type %s", name),
				},
			})

		case *ast.CallExpr:
			// Function call
			if currentFunction == "" {
				return true
			}

			// Extract callee name
			var callee string
			switch fun := x.Fun.(type) {
			case *ast.Ident:
				// Local call or import
				callee = fun.Name // simplified
			case *ast.SelectorExpr:
				// pkg.Func or obj.Method
				if x, ok := fun.X.(*ast.Ident); ok {
					callee = fmt.Sprintf("%s.%s", x.Name, fun.Sel.Name)
				}
			}

			if callee != "" {
				// code_calls(Caller, Callee)
				facts = append(facts, core.Fact{
					Predicate: "code_calls",
					Args: []interface{}{
						core.MangleAtom(currentFunction),
						core.MangleAtom(callee),
					},
				})
			}
		}
		return true
	})

	logging.WorldDebug("Cartographer: mapped %s - %d facts generated in %v", filepath.Base(path), len(facts), time.Since(start))
	return facts, nil
}