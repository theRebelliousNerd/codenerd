package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"time"
)

// Cartographer implements the "Holographic" Code Graph projection.
// It parses code to emit rich structural facts:
// - code_defines(File, Symbol, Type, StartLine, EndLine)
// - code_calls(Caller, Callee)
// - code_implements(Struct, Interface)
//
//
// Data flow facts (via MultiLangDataFlowExtractor):
// - assigns(Var, TypeClass, File, Line)
// - guards_return(Var, GuardType, File, Line)
// - guards_block(Var, GuardType, File, StartLine, EndLine)
// - uses(File, Func, Var, Line)
// - safe_access(Var, AccessType, File, Line) - for language-specific safe patterns
// - function_scope(File, Func, Start, End) - function boundaries
// - guard_dominates(File, Func, GuardLine, EndLine) - early return domination
//
// Supports: Go, Python, TypeScript, JavaScript, Rust
type Cartographer struct {
	dataFlowExtractor *MultiLangDataFlowExtractor
}

// NewCartographer creates a new Cartographer for holographic code graph projection.
func NewCartographer() *Cartographer {
	logging.WorldDebug("Creating new Cartographer with MultiLangDataFlowExtractor")
	return &Cartographer{
		dataFlowExtractor: NewMultiLangDataFlowExtractor(),
	}
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

		case *ast.TypeSpec:
			// Type definition (Struct/Interface)
			name := x.Name.Name
			id := fmt.Sprintf("%s.%s", pkgName, name)
			start := fset.Position(x.Pos()).Line
			end := fset.Position(x.End()).Line

			typeType := "/type"
			if _, ok := x.Type.(*ast.StructType); ok {
				typeType = "/struct"
			} else if _, ok := x.Type.(*ast.InterfaceType); ok {
				typeType = "/interface"
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

		case *ast.CallExpr:
			// Function call
			if currentFunction == "" {
				return true
			}

			// Extract callee name
			var callee string
			switch fun := x.Fun.(type) {
			case *ast.Ident:
				// Local call (best-effort qualification for in-repo symbol matching)
				callee = fmt.Sprintf("%s.%s", pkgName, fun.Name)
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

	symbolFactCount := len(facts)
	logging.WorldDebug("Cartographer: extracted %d symbol facts from %s", symbolFactCount, filepath.Base(path))

	// Extract data flow facts (enhancement, not critical - errors don't break symbol extraction)
	if c.dataFlowExtractor != nil {
		dataFlowFacts, err := c.dataFlowExtractor.ExtractDataFlow(path)
		if err != nil {
			logging.WorldDebug("Cartographer: data flow extraction failed for %s: %v (continuing with symbol facts only)", filepath.Base(path), err)
			// Continue - data flow is an enhancement, not critical
		} else {
			facts = append(facts, dataFlowFacts...)
			logging.WorldDebug("Cartographer: extracted %d data flow facts from %s", len(dataFlowFacts), filepath.Base(path))
		}
	}

	logging.WorldDebug("Cartographer: mapped %s - %d total facts (%d symbol, %d data flow) in %v",
		filepath.Base(path), len(facts), symbolFactCount, len(facts)-symbolFactCount, time.Since(start))
	return facts, nil
}

// Close releases resources held by the Cartographer.
func (c *Cartographer) Close() {
	if c.dataFlowExtractor != nil {
		c.dataFlowExtractor.Close()
	}
}

// SupportedLanguages returns the list of languages supported for data flow extraction.
func (c *Cartographer) SupportedLanguages() []string {
	return []string{"go", "python", "typescript", "javascript", "rust"}
}

// IsLanguageSupported checks if a file's language is supported for data flow extraction.
func (c *Cartographer) IsLanguageSupported(path string) bool {
	lang := DetectLanguage(path)
	for _, supported := range c.SupportedLanguages() {
		if lang == supported {
			return true
		}
	}
	return false
}
