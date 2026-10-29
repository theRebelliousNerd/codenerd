package session

import (
	"context"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/world"
)

// newFunc is one function or method the turn added to a non-test Go file,
// as the structure index addresses it.
type newFunc struct {
	ref       string
	file      string
	line      int
	signature string
	doc       string
	testOnly  bool
}

// unwiredMeasurements are the predicates measureNewFuncs replaces on every
// run. The unused-reason readings are not among them: a reason the model gave
// stays given when the code is measured again.
var unwiredMeasurements = []string{
	"turn_new_func", "turn_new_func_entry", "turn_new_func_generated",
	"turn_new_func_unobserved", "turn_func_use", "turn_method_mentioned",
}

// measureNewFuncs asserts what the /unwired policy decides over
// (turn_unwired.mg), replacing the previous measurement, and returns the
// functions it measured by ref.
//
// Which functions are new comes from the pin gate's change detection
// (fileUnits): a function or method present now and absent from the file's
// preimage, token-compared. Which code reaches one comes from the structure
// index: Uses for a function (bare uses in its package, qualified uses in its
// importers, each with the declaration holding it), and the name for a
// method, which is called through values of types the index does not know.
// A file whose preimage is unknown is skipped, the way the pin gate skips it:
// without a preimage nothing says which of its functions are new.
func (e *Executor) measureNewFuncs(ctx context.Context, result *ExecutionResult) (map[string]newFunc, error) {
	turn := result.turnAtom()
	e.retractTurnPredicates(turn, unwiredMeasurements...)
	workspace := e.workspaceForVerification()
	if workspace == "" {
		return nil, fmt.Errorf("no workspace to index")
	}
	idx := world.SharedStructureIndex(workspace)
	funcs := map[string]newFunc{}
	seen := map[string]bool{}
	for _, path := range result.WrittenPaths {
		if seen[path] || !strings.HasSuffix(strings.ToLower(path), ".go") || isTestPath(path) || ignoredByGoTool(path) {
			continue
		}
		seen[path] = true
		pre, ok := preImageFor(workspace, path, result.PreWriteContents)
		if !ok || !pre.Known() {
			continue
		}
		disk := turnFilePath(workspace, path)
		if included, err := build.Default.MatchFile(filepath.Dir(disk), filepath.Base(disk)); err != nil || !included {
			continue
		}
		data, err := os.ReadFile(disk)
		if err != nil {
			// Deleted since it was written: it adds nothing.
			continue
		}
		added := map[string]bool{}
		for _, u := range fileUnits(path, string(data), pre) {
			if u.added && u.name != "" {
				added[u.name] = true
			}
		}
		if len(added) == 0 {
			continue
		}
		shape, err := goFileShape(string(data))
		if err != nil {
			continue
		}
		symbols, _, err := idx.Outline(ctx, disk)
		if err != nil {
			return nil, fmt.Errorf("outline %s: %w", path, err)
		}
		rel := filepath.ToSlash(types.CanonicalPath(workspace, disk))
		for _, sym := range symbols {
			if !added[sym.Key] || (sym.Kind != "function" && sym.Kind != "method") {
				continue
			}
			fn := newFunc{ref: sym.Ref, file: sym.File, line: sym.StartLine, signature: sym.Signature, doc: sym.Doc}
			funcs[fn.ref] = fn
			if err := e.measureNewFunc(ctx, idx, turn, sym, shape, rel); err != nil {
				return nil, err
			}
		}
	}
	return funcs, nil
}

// measureNewFunc asserts one new function's facts.
func (e *Executor) measureNewFunc(ctx context.Context, idx *world.StructureIndex, turn types.MangleAtom, sym world.StructSymbol, shape goShape, rel string) error {
	ref := types.MangleString(sym.Ref)
	exposure := types.MangleAtom("/module")
	if sym.Exported && shape.pkg != "main" && importableFromOtherModules(rel) {
		exposure = "/api"
	}
	assert := func(predicate string, args ...any) error {
		if !e.assertTurnFact(types.Fact{Predicate: predicate, Args: append([]any{turn}, args...)}) {
			return fmt.Errorf("assert %s for %s", predicate, sym.Ref)
		}
		return nil
	}
	if err := assert("turn_new_func", ref, exposure); err != nil {
		return err
	}
	if shape.generated {
		if err := assert("turn_new_func_generated", ref); err != nil {
			return err
		}
	}
	if shape.entries[sym.Key] {
		if err := assert("turn_new_func_entry", ref); err != nil {
			return err
		}
	}
	if sym.Kind == "method" {
		name := sym.Key[strings.LastIndex(sym.Key, ".")+1:]
		m, stats, err := idx.NameMentions(ctx, name)
		if err != nil {
			return fmt.Errorf("mentions of %s: %w", sym.Ref, err)
		}
		if len(stats.Deficits) > 0 {
			return assert("turn_new_func_unobserved", ref)
		}
		if m.Production > 0 {
			if err := assert("turn_method_mentioned", ref, types.MangleAtom("/production")); err != nil {
				return err
			}
		}
		if m.Test > 0 {
			return assert("turn_method_mentioned", ref, types.MangleAtom("/test"))
		}
		return nil
	}
	_, uses, stats, err := idx.Uses(ctx, sym.Ref)
	if err != nil {
		return fmt.Errorf("uses of %s: %w", sym.Ref, err)
	}
	if len(stats.Deficits) > 0 {
		return assert("turn_new_func_unobserved", ref)
	}
	for _, u := range uses {
		side := types.MangleAtom("/production")
		if isTestPath(u.File) {
			side = "/test"
		}
		if err := assert("turn_func_use", ref, types.MangleString(u.Ref), side); err != nil {
			return err
		}
	}
	return nil
}

// goShape is what one Go file says about its functions beyond their uses.
type goShape struct {
	pkg       string
	generated bool
	// entries are the keys (Name, Recv.Name) of the functions something
	// outside the Go call graph runs: main in package main, and a function
	// carrying //go:linkname, //go:wasmexport or cgo's //export.
	entries map[string]bool
}

func goFileShape(src string) (goShape, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return goShape{}, err
	}
	shape := goShape{pkg: f.Name.Name, generated: ast.IsGenerated(f), entries: map[string]bool{}}
	linknamed := map[string]bool{}
	for _, group := range f.Comments {
		for _, c := range group.List {
			if d, ok := ast.ParseDirective(c.Pos(), c.Text); ok && d.Tool == "go" && d.Name == "linkname" {
				if fields := strings.Fields(d.Args); len(fields) > 0 {
					linknamed[fields[0]] = true
				}
			}
		}
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		key := funcKey(fn)
		if (fn.Recv == nil && fn.Name.Name == "main" && shape.pkg == "main") || linknamed[fn.Name.Name] || exportDirective(fn.Doc) {
			shape.entries[key] = true
		}
	}
	return shape, nil
}

// exportDirective reports a //go:wasmexport or cgo //export line in a
// function's doc comment.
func exportDirective(doc *ast.CommentGroup) bool {
	if doc == nil {
		return false
	}
	for _, c := range doc.List {
		if d, ok := ast.ParseDirective(c.Pos(), c.Text); ok && d.Tool == "go" && d.Name == "wasmexport" {
			return true
		}
		if fields := strings.Fields(c.Text); len(fields) > 0 && fields[0] == "//export" {
			return true
		}
	}
	return false
}

// importableFromOtherModules reports a package directory another module may
// import: one with no internal segment (the go tool refuses those outside
// the module).
func importableFromOtherModules(rel string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/") {
		if seg == "internal" {
			return false
		}
	}
	return true
}
