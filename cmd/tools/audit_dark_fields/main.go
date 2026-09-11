// audit_dark_fields reports exported struct fields that production code READS
// and production code never WRITES.
//
// This is the defect this repository keeps producing, and the 2026-09 pass
// found five instances of it in one branch:
//
//	SessionContext.TestState      drives /tdd_repair            no writer
//	SessionContext.FailingTests   arms the failing_tests state  no writer
//	SessionContext.TDDRetryCount  prints "fix the root cause"   no writer
//	CompilationContext.HasNewFiles gates a MANDATORY atom       no writer
//	ParsedFinding.ShardSource     names the reviewer            no writer
//
// Every one of them had readers — several had readers in the prompt assembler,
// the most-exercised code in the tree — and every one had tests. That is the
// trap: the READER is what gets tested, so the feature looks covered, and the
// field reads as its zero value forever. An empty string, an empty slice, a
// zero count: none of those is an error, none logs, and each one is
// indistinguishable from a legitimate "nothing to report".
//
// The dead-code budget next door catches the same disease in functions. This
// is the struct-field half, and it is the harder half, because an unreachable
// function at least shows up as unreachable. A field nobody writes is
// perfectly reachable; it is just always empty.
//
// # What counts as a write
//
// Assignment to a selector (x.F = v, x.F += v, x.F++), taking its address
// (&x.F, which is how json.Unmarshal and sql.Scan fill one), and a key in any
// composite literal (T{F: v}). Analysis is by field NAME rather than by type,
// which deliberately OVER-counts writes: two unrelated types sharing a field
// name are treated as one, so the tool under-reports rather than crying wolf.
// A gate that fails on something that is not a bug does not get switched on,
// and then the real drift goes unseen too.
//
// Fields carrying a struct tag are skipped entirely. A `json:"..."` tag means
// the field is written by a decoder this analysis cannot see, and the tag is
// the declaration of that.
//
// # The baseline
//
// Like the dead-code budget, the number is not meant to reach zero. Plenty of
// dark fields are legitimate — a capability waiting on a source that does not
// exist yet, a platform struct the OS fills, a test double's knob. Each is
// recorded with the reason it is dark, and the gate fails when the set CHANGES
// in either direction: a new dark field needs a reason, and a field that
// stopped being dark should leave, or the baseline decays into a list nobody
// reads.
//
//	go run ./cmd/tools/audit_dark_fields           check against the baseline
//	go run ./cmd/tools/audit_dark_fields -update   rewrite the baseline
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const baselinePath = "scripts/testdata/dark-fields-baseline.txt"

// roots are the production trees. tests/ is excluded with _test.go files: a
// field only an end-to-end test writes is still dark in production.
var roots = []string{"internal", "cmd"}

// skipDirs are trees whose whole purpose is to be written by a test.
var skipDirs = map[string]bool{
	"internal/testing":         true,
	"internal/types/typestest": true,
}

type field struct {
	Name string
	Type string
	File string
	Line int
}

func main() {
	update := flag.Bool("update", false, "rewrite the baseline")
	flag.Parse()

	fset := token.NewFileSet()
	var files []*ast.File
	var paths []string
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[filepath.ToSlash(path)] {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if perr != nil {
				return fmt.Errorf("parse %s: %w", path, perr)
			}
			files = append(files, f)
			paths = append(paths, filepath.ToSlash(path))
			return nil
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}

	declared := map[string][]field{}
	writes := map[string]bool{}
	reads := map[string]bool{}

	for i, f := range files {
		collectFields(fset, f, paths[i], declared)
		collectUses(f, writes, reads)
	}

	var dark []field
	for name, decls := range declared {
		if writes[name] || !reads[name] {
			continue
		}
		dark = append(dark, decls...)
	}
	sort.Slice(dark, func(i, j int) bool {
		if dark[i].Type != dark[j].Type {
			return dark[i].Type < dark[j].Type
		}
		return dark[i].Name < dark[j].Name
	})

	lines := make([]string, 0, len(dark))
	for _, d := range dark {
		lines = append(lines, d.Type+"."+d.Name)
	}
	lines = dedupe(lines)

	if *update {
		writeBaseline(lines, dark)
		fmt.Printf("Baseline updated: %d dark fields.\n", len(lines))
		return
	}
	compare(lines, dark)
}

// collectFields records every exported field of a named struct type, except
// the ones carrying a struct tag — a tag declares that something outside this
// analysis writes the field.
func collectFields(fset *token.FileSet, f *ast.File, path string, out map[string][]field) {
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		for _, fl := range st.Fields.List {
			if fl.Tag != nil {
				continue
			}
			for _, name := range fl.Names {
				if !name.IsExported() {
					continue
				}
				pos := fset.Position(name.Pos())
				out[name.Name] = append(out[name.Name], field{
					Name: name.Name,
					Type: ts.Name.Name,
					File: path,
					Line: pos.Line,
				})
			}
		}
		return true
	})
}

// collectUses splits every selector and composite-literal key into a write or
// a read. It deliberately resolves nothing: a name written anywhere counts as
// written everywhere, which makes this under-report rather than cry wolf.
func collectUses(f *ast.File, writes, reads map[string]bool) {
	mark := func(e ast.Expr, into map[string]bool) {
		if sel, ok := e.(*ast.SelectorExpr); ok {
			into[sel.Sel.Name] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				mark(lhs, writes)
			}
		case *ast.IncDecStmt:
			mark(x.X, writes)
		case *ast.UnaryExpr:
			// &x.F is how a decoder or a Scan fills a field.
			if x.Op == token.AND {
				mark(x.X, writes)
			}
		case *ast.CompositeLit:
			for _, elt := range x.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if id, ok := kv.Key.(*ast.Ident); ok {
						writes[id.Name] = true
					}
				}
			}
		case *ast.SelectorExpr:
			reads[x.Sel.Name] = true
		}
		return true
	})
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func writeBaseline(lines []string, dark []field) {
	site := map[string]field{}
	for _, d := range dark {
		key := d.Type + "." + d.Name
		if _, ok := site[key]; !ok {
			site[key] = d
		}
	}
	var b strings.Builder
	b.WriteString("# Exported struct fields production code reads and never writes.\n")
	b.WriteString("# Generated by: go run ./cmd/tools/audit_dark_fields -update\n")
	b.WriteString("#\n")
	b.WriteString("# A field here always reads as its zero value. That is not an error, it does\n")
	b.WriteString("# not log, and it is indistinguishable from a legitimate \"nothing to report\",\n")
	b.WriteString("# which is why every instance of this found so far was found by hand.\n")
	b.WriteString("#\n")
	b.WriteString("# Entries are TYPE.FIELD with the declaration site. Before adding one, decide\n")
	b.WriteString("# which of two things it is, because they look identical from the reader:\n")
	b.WriteString("#   a missing WIRE   - a producer exists and nothing connects it. Fix it.\n")
	b.WriteString("#   a missing SOURCE - nothing in this architecture can answer. Say so at\n")
	b.WriteString("#                      the field, and cost the producer as a feature.\n")
	for _, l := range lines {
		d := site[l]
		fmt.Fprintf(&b, "%s\t%s:%d\n", l, d.File, d.Line)
	}
	if err := os.WriteFile(baselinePath, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func compare(lines []string, dark []field) {
	raw, err := os.ReadFile(baselinePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no baseline at %s; create it with -update\n", baselinePath)
		os.Exit(2)
	}
	want := map[string]bool{}
	for _, l := range strings.Split(string(raw), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		want[strings.Split(l, "\t")[0]] = true
	}
	site := map[string]field{}
	for _, d := range dark {
		key := d.Type + "." + d.Name
		if _, ok := site[key]; !ok {
			site[key] = d
		}
	}

	var added, removed []string
	got := map[string]bool{}
	for _, l := range lines {
		got[l] = true
		if !want[l] {
			added = append(added, l)
		}
	}
	for l := range want {
		if !got[l] {
			removed = append(removed, l)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)

	if len(added) == 0 && len(removed) == 0 {
		fmt.Printf("Dark-field budget holds: %d fields read and never written, no drift.\n", len(lines))
		return
	}
	if len(added) > 0 {
		fmt.Printf("New dark fields (%d):\n", len(added))
		for _, l := range added {
			d := site[l]
			fmt.Printf("  %s\t%s:%d\n", l, d.File, d.Line)
		}
		fmt.Print(`
Each is read by production code and written by none of it, so it reads as its
zero value on every turn. Decide which it is — a missing WIRE (a producer
exists; connect it) or a missing SOURCE (nothing here can answer; say so at the
field and cost the producer as a feature) — then fix it or record it with:
  go run ./cmd/tools/audit_dark_fields -update
and say why in the commit.
`)
	}
	if len(removed) > 0 {
		fmt.Printf("\nFields that are no longer dark (%d):\n", len(removed))
		for _, l := range removed {
			fmt.Printf("  %s\n", l)
		}
		fmt.Print(`
Good. Refresh the baseline so the count stays a real measurement:
  go run ./cmd/tools/audit_dark_fields -update
`)
	}
	os.Exit(1)
}
