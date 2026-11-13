package session

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"codenerd/internal/types"
)

// What the turn's change touches, measured for the /expert_critique round.
//
// A shard agent owes a review when the turn's selection spawned it, and the
// selection reads what the task's files are and import. That is the file, not
// the change: live 2026-11-12 a step on a file importing a framework got the
// framework's reviewers whatever the step changed. The round now asks the
// kernel which critics the change itself reaches (turn_expert_critic_due,
// policy/shard_experts.mg): Go measures, per written file, its extension and,
// for Go source, the imports the change adds or removes or whose package a
// changed line names; the policy maps those to technologies with the same
// vocabulary the selection uses, and keeps a critic whose technologies the
// change touches. A critic whose domain is a concept (security, testing) is
// not a code technology and stays due.

// changeTouchFacts are turn_change_file_ext and turn_change_import for every
// file the turn wrote and changed.
func changeTouchFacts(workspace string, turn types.MangleAtom, result *ExecutionResult) []types.Fact {
	if result == nil {
		return nil
	}
	var facts []types.Fact
	seen := map[string]bool{}
	for _, p := range result.WrittenPaths {
		rel := canonicalizeWrittenPath(p, workspace)
		if rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		before := ""
		if pre, ok := result.PreWriteContents[rel]; ok {
			before = pre.Content
		}
		data, err := os.ReadFile(turnFilePath(workspace, rel))
		after := ""
		if err == nil {
			after = string(data)
		}
		if before == after {
			continue
		}
		ext := strings.ToLower(filepath.Ext(rel))
		facts = append(facts, types.Fact{Predicate: "turn_change_file_ext", Args: []any{turn, rel, ext}})
		if ext != ".go" {
			continue
		}
		for _, imp := range goChangeImports(before, after) {
			facts = append(facts, types.Fact{Predicate: "turn_change_import", Args: []any{turn, rel, imp}})
		}
	}
	return facts
}

// goChangeImports is the import paths a change between two versions of a Go
// file touches: those it added or removed, and those whose package a changed
// line (on either side) names in a selector. A source that does not parse
// touches every import it has, and a selector whose package no import's name
// accounts for touches every import without an explicit name: the
// over-approximation only ever keeps a reviewer.
func goChangeImports(before, after string) []string {
	out := map[string]bool{}
	sides := []struct{ src, other string }{{after, before}, {before, after}}
	importsOf := map[int]map[string]bool{}
	for i, side := range sides {
		importsOf[i] = map[string]bool{}
		if strings.TrimSpace(side.src) == "" {
			continue
		}
		spans := changedSpans(side.other, side.src)
		for _, imp := range namesTouched(side.src, spans) {
			out[imp] = true
		}
		for _, imp := range fileImports(side.src) {
			importsOf[i][imp.path] = true
		}
	}
	for p := range importsOf[0] {
		if !importsOf[1][p] {
			out[p] = true
		}
	}
	for p := range importsOf[1] {
		if !importsOf[0][p] {
			out[p] = true
		}
	}
	paths := make([]string, 0, len(out))
	for p := range out {
		paths = append(paths, p)
	}
	return paths
}

// fileImport is one import of a source file and the name it is used by.
type fileImport struct {
	name, path string
	explicit   bool
}

func fileImports(src string) []fileImport {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ImportsOnly)
	if err != nil {
		return nil
	}
	return importsOfFile(f)
}

func importsOfFile(f *ast.File) []fileImport {
	var out []fileImport
	for _, is := range f.Imports {
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			continue
		}
		imp := fileImport{path: p, name: importDefaultName(p)}
		if is.Name != nil {
			imp.name, imp.explicit = is.Name.Name, true
		}
		out = append(out, imp)
	}
	return out
}

var majorVersionElem = regexp.MustCompile(`^v[0-9]+$`)

// importDefaultName is the name a package is most likely declared under,
// from its path alone: the last element, without a major-version element or
// suffix (yaml.v3, /v2) or a go- prefix or -go suffix. It is a guess -- the
// package clause decides -- which is why a selector it fails to account for
// touches every implicitly named import.
func importDefaultName(p string) string {
	elems := strings.Split(p, "/")
	last := elems[len(elems)-1]
	if majorVersionElem.MatchString(last) && len(elems) > 1 {
		last = elems[len(elems)-2]
	}
	if i := strings.Index(last, ".v"); i > 0 && majorVersionElem.MatchString(last[i+1:]) {
		last = last[:i]
	}
	last = strings.TrimSuffix(strings.TrimPrefix(last, "go-"), "-go")
	return strings.ReplaceAll(path.Base(last), "-", "")
}

// namesTouched is the import paths whose package a selector on one of spans'
// lines names.
func namesTouched(src string, spans []criticSpan) []string {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return importPaths(fileImports(src))
	}
	imports := importsOfFile(f)
	byName := map[string][]string{}
	for _, imp := range imports {
		byName[imp.name] = append(byName[imp.name], imp.path)
	}
	inSpan := func(line int) bool {
		for _, s := range spans {
			if line >= s.start && line <= s.end {
				return true
			}
		}
		return false
	}
	out := map[string]bool{}
	unaccounted := false
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := sel.X.(*ast.Ident)
		// A resolved identifier is a local or a declaration of this file,
		// not a package.
		if !ok || x.Obj != nil || !inSpan(fset.Position(sel.Pos()).Line) {
			return true
		}
		paths, known := byName[x.Name]
		if !known {
			unaccounted = true
			return true
		}
		for _, p := range paths {
			out[p] = true
		}
		return true
	})
	if unaccounted {
		for _, imp := range imports {
			if !imp.explicit {
				out[imp.path] = true
			}
		}
	}
	paths := make([]string, 0, len(out))
	for p := range out {
		paths = append(paths, p)
	}
	return paths
}

func importPaths(imports []fileImport) []string {
	out := make([]string, 0, len(imports))
	for _, imp := range imports {
		out = append(out, imp.path)
	}
	return out
}
