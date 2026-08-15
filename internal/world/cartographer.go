package world

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Cartographer implements the "Holographic" Code Graph projection.
// It parses code to emit rich structural facts:
// - code_defines(File, Symbol, Type, StartLine, EndLine)
// - code_calls(Caller, Callee)
// - code_implements(Struct, Interface)
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
	parsers           tsParserSet
}

// NewCartographer creates a new Cartographer for holographic code graph projection.
func NewCartographer() *Cartographer {
	logging.WorldDebug("Creating new Cartographer with MultiLangDataFlowExtractor")
	return &Cartographer{
		dataFlowExtractor: NewMultiLangDataFlowExtractor(),
	}
}

// MapFile parses a single file and returns holographic facts.
// Go is mapped with go/ast; Python, TypeScript, JavaScript and Rust with
// tree-sitter (see cartographer_multilang.go).
func (c *Cartographer) MapFile(path string) ([]core.Fact, error) {
	return c.MapFileAs(path, path)
}

// MapFileAs maps the file at fsPath but labels every emitted fact with
// factPath. Deep facts must carry the same canonical (workspace-relative)
// identity as file_topology, while the parser needs a path it can actually
// open — before this split, deep scans run from outside the workspace either
// failed to open the file or stamped absolute paths into the fact store.
func (c *Cartographer) MapFileAs(fsPath, factPath string) ([]core.Fact, error) {
	logging.WorldDebug("Cartographer mapping file: %s", filepath.Base(fsPath))
	ext := strings.ToLower(filepath.Ext(fsPath))
	if ext == ".go" {
		return c.mapGoFile(fsPath, factPath)
	}
	if lang := DetectLanguage(fsPath); lang != "" && deepMappableExt(ext) {
		return c.mapNonGoFile(fsPath, factPath, lang)
	}
	logging.WorldDebug("Cartographer: unsupported file type %s for %s", ext, filepath.Base(fsPath))
	return nil, nil
}

func (c *Cartographer) mapGoFile(fsPath, path string) ([]core.Fact, error) {
	start := time.Now()
	logging.WorldDebug("Cartographer: mapping Go file: %s", filepath.Base(fsPath))

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, fsPath, nil, parser.ParseComments)
	if err != nil {
		logging.Get(logging.CategoryWorld).Error("Cartographer: Go parse failed: %s - %v", fsPath, err)
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
				Args: []any{
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
				Args: []any{
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
					Args: []any{
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
		dataFlowFacts, err := c.dataFlowExtractor.ExtractDataFlow(fsPath)
		if err != nil {
			logging.WorldDebug("Cartographer: data flow extraction failed for %s: %v (continuing with symbol facts only)", filepath.Base(path), err)
			// Continue - data flow is an enhancement, not critical
		} else {
			facts = append(facts, relabelPathArgs(dataFlowFacts, fsPath, path)...)
			logging.WorldDebug("Cartographer: extracted %d data flow facts from %s", len(dataFlowFacts), filepath.Base(fsPath))
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
	c.parsers.close()
}

// relabelPathArgs rewrites the filesystem path the data-flow extractor stamped
// into its facts to the canonical fact identity. The extractor is given a
// readable path and has no notion of workspace-relative identity, so without
// this its facts key a different file than the code_defines emitted beside them.
func relabelPathArgs(facts []core.Fact, from, to string) []core.Fact {
	if from == to || len(facts) == 0 {
		return facts
	}
	out := make([]core.Fact, 0, len(facts))
	for _, f := range facts {
		args := make([]any, len(f.Args))
		copy(args, f.Args)
		for i, a := range args {
			if s, ok := a.(string); ok && s == from {
				args[i] = to
			}
		}
		out = append(out, core.Fact{Predicate: f.Predicate, Args: args})
	}
	return out
}

// SupportedLanguages returns the list of languages supported for data flow extraction.
func (c *Cartographer) SupportedLanguages() []string {
	return []string{"go", "python", "typescript", "javascript", "rust"}
}

// IsLanguageSupported checks if a file's language is supported for data flow extraction.
func (c *Cartographer) IsLanguageSupported(path string) bool {
	lang := DetectLanguage(path)
	return slices.Contains(c.SupportedLanguages(), lang)
}
