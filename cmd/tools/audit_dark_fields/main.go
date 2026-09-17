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

// deadcodeBaselinePath is read only to ANNOTATE. A field declared in a file
// that also has entries in the dead-code budget is very often a consequence of
// those rather than a finding of its own: cmd/nerd/chat/tips.go contributes
// five dark fields and eight dead functions, and the fields are dark because
// nothing calls the functions that would fill them. Whoever triages this list
// should spend their attention elsewhere first.
//
// It is a hint and not a filter. The overlap is at file granularity, so a live
// function beside a dead one still gets the mark, and dropping those entries
// would hide real findings behind an approximation.
const deadcodeBaselinePath = "scripts/testdata/deadcode-baseline.txt"

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
	order := map[string][]string{}
	ftypes := map[string][]string{}
	writes := map[string]bool{}
	reads := map[string]bool{}

	for i, f := range files {
		collectFields(fset, f, paths[i], declared, order, ftypes)
	}
	for _, f := range files {
		collectUses(f, order, ftypes, writes, reads)
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
func collectFields(fset *token.FileSet, f *ast.File, path string, out map[string][]field, order map[string][]string, ftypes map[string][]string) {
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		// Positional order includes EVERY field — unexported, tagged and
		// embedded alike — because a positional composite literal counts all of
		// them. An embedded field has no name and still occupies a slot, so it
		// gets an empty placeholder rather than being skipped.
		var positions []string
		// types runs alongside positions, one entry per slot, so a struct
		// handed to a syscall can be walked into its nested structs.
		var types []string
		for _, fl := range st.Fields.List {
			if len(fl.Names) == 0 {
				positions = append(positions, "")
				types = append(types, namedType(fl.Type))
				continue
			}
			for _, name := range fl.Names {
				positions = append(positions, name.Name)
				types = append(types, namedType(fl.Type))
			}
		}
		order[ts.Name.Name] = positions
		ftypes[ts.Name.Name] = types

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
func collectUses(f *ast.File, order, ftypes map[string][]string, writes, reads map[string]bool) {
	varTypes := collectVarTypes(f)
	// mark records the field an assignment target names, seeing through the
	// wrappers an assignable expression can carry.
	//
	// `metrics.QueueDepthByPriority[i] = len(q)` is an assignment to an index
	// expression whose operand is the selector, not to the selector itself, so
	// a bare type switch on *ast.SelectorExpr misses it entirely — and
	// SpawnQueueMetrics.QueueDepthByPriority was reported dark on the strength
	// of a line that fills it. Any field that is a slice, array or map gets
	// written this way, which is a large share of the fields worth caring
	// about.
	mark := func(e ast.Expr, into map[string]bool) {
		for {
			switch t := e.(type) {
			case *ast.ParenExpr:
				e = t.X
			case *ast.IndexExpr:
				e = t.X
			case *ast.SliceExpr:
				e = t.X
			case *ast.StarExpr:
				e = t.X
			case *ast.SelectorExpr:
				into[t.Sel.Name] = true
				return
			default:
				return
			}
		}
	}
	// Selectors in call position are METHOD CALLS, not field reads.
	//
	// Without this, any field whose name matches a method anywhere in the tree
	// reads as read: ZAIConfig.DisableSemaphore was reported dark because
	// `c.DisableSemaphore()` is a method on the LLM client interface, and this
	// analysis keys on names rather than types by design. A CallExpr's Fun is
	// recorded before the walk reaches it, since Inspect visits a node before
	// its children.
	//
	// It costs the ability to see a dark func-typed FIELD that is only ever
	// called and never otherwise mentioned, which is the under-reporting side
	// of the same trade every other decision here takes: a gate that cries wolf
	// is a gate nobody runs.
	calls := map[ast.Node]bool{}
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			calls[c.Fun] = true
			markSyscallBuffer(c, varTypes, order, ftypes, writes)
		}
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
			// No early return: the walker must still descend, or selector
			// reads and assignments nested inside a literal (a field whose
			// value is a function literal, most of all) stop being seen and
			// the fields they touch turn dark by omission. markCompositeWrites
			// recurses into inner literals itself so it can carry the element
			// type down; the walker reaching the same literal again is
			// harmless, since marking a name written twice is marking it once.
			markCompositeWrites(x, "", order, writes)
		case *ast.SelectorExpr:
			if !calls[x] {
				reads[x.Sel.Name] = true
			}
		}
		return true
	})
}

// markSyscallBuffer records every field of a struct whose address is handed to
// a syscall through unsafe.Pointer as written.
//
// This is the same principle collectFields already applies to a struct tag —
// a tag declares that something outside this analysis writes the field — and
// it needs saying, because nine of the twenty-nine live entries in the
// baseline were this and nothing else.
//
// internal/tactile/platform_windows.go declares the Win32 structs and reads
// them back: usage.MaxRSSBytes = int64(extInfo.PeakJobMemoryUsed),
// DiskReadBytes = int64(extInfo.IoInfo.ReadTransferCount), UserTimeMs =
// accountInfo.TotalUserTime / 10000. No Go statement ever assigns those
// fields, and none ever will: QueryInformationJobObject, GetProcessIoCounters
// and K32GetProcessMemoryInfo write the bytes, and what the walker sees of
// that is uintptr(unsafe.Pointer(&extInfo)) — an address leaving the language.
//
// A walker with no type information cannot resolve &extInfo to its struct, so
// it resolves the VARIABLE instead, file-wide rather than per function. Two
// locals of different types sharing a name over-marks, which is the direction
// this whole tool is biased in already: over-counting writes under-reports
// dark fields, and a gate that cries wolf is a gate nobody runs.
//
// The recursion is not optional. IO_COUNTERS.ReadTransferCount is written
// through extInfo.IoInfo, one level down from the struct whose address was
// taken, so stopping at the top level would leave the nested ones dark.
func markSyscallBuffer(call *ast.CallExpr, varTypes map[string]string, order, ftypes map[string][]string, writes map[string]bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Pointer" {
		return
	}
	if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "unsafe" {
		return
	}
	if len(call.Args) != 1 {
		return
	}
	// Peel &x, &x[0], (&x) and x.f down to the identifier whose storage is
	// being handed out.
	e := call.Args[0]
	for {
		switch t := e.(type) {
		case *ast.ParenExpr:
			e = t.X
		case *ast.UnaryExpr:
			if t.Op != token.AND {
				return
			}
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.SelectorExpr:
			e = t.X
		default:
			id, ok := e.(*ast.Ident)
			if !ok {
				return
			}
			markFieldsOf(varTypes[id.Name], order, ftypes, writes, map[string]bool{})
			return
		}
	}
}

// markFieldsOf marks every field of a named struct type written, and recurses
// into its struct-typed fields. seen stops a type that contains itself.
func markFieldsOf(typeName string, order, ftypes map[string][]string, writes map[string]bool, seen map[string]bool) {
	if typeName == "" || seen[typeName] {
		return
	}
	seen[typeName] = true
	names := order[typeName]
	types := ftypes[typeName]
	for i, name := range names {
		if name != "" {
			writes[name] = true
		}
		if i < len(types) {
			markFieldsOf(types[i], order, ftypes, writes, seen)
		}
	}
}

// collectVarTypes records the declared type of each variable in a file, so the
// identifier inside unsafe.Pointer(&x) can be resolved to a struct name. It is
// file-wide on purpose: see markSyscallBuffer on why over-marking is the safe
// direction here.
func collectVarTypes(f *ast.File) map[string]string {
	types := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.ValueSpec:
			if t.Type == nil {
				return true
			}
			if name := namedType(t.Type); name != "" {
				for _, id := range t.Names {
					types[id.Name] = name
				}
			}
		case *ast.AssignStmt:
			if t.Tok != token.DEFINE || len(t.Lhs) != len(t.Rhs) {
				return true
			}
			for i, lhs := range t.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				rhs := t.Rhs[i]
				if u, ok := rhs.(*ast.UnaryExpr); ok && u.Op == token.AND {
					rhs = u.X
				}
				if lit, ok := rhs.(*ast.CompositeLit); ok {
					if name := namedType(lit.Type); name != "" {
						types[id.Name] = name
					}
				}
			}
		}
		return true
	})
	return types
}

// deadcodeFiles reads the dead-code budget for the set of files that have
// unreachable functions in them. A missing or unreadable baseline is not an
// error: this annotation is a convenience, and failing the dark-field gate
// because a DIFFERENT gate's file moved would be its own small disaster.
func deadcodeFiles() map[string]bool {
	out := map[string]bool{}
	raw, err := os.ReadFile(deadcodeBaselinePath)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out[strings.Split(line, "\t")[0]] = true
	}
	return out
}

// markCompositeWrites records the fields a composite literal fills, including
// the POSITIONAL form.
//
// Missing positional literals was a real false positive and not a theoretical
// one. internal/store declares `type Migration struct { Table, Column, Def
// string }` and fills fifty of them as `{"cold_storage", "last_accessed",
// "DATETIME DEFAULT CURRENT_TIMESTAMP"}`. Table and Def are written on every
// one of those lines, and a key-only walk reported both as dark — a gate
// crying wolf about a struct that works, which is how a gate stops being run.
//
// The type name comes from the literal itself (Migration{...}, core.FileEdit
// {...}) or, for the elided inner literals of a slice or map, from the element
// type of the enclosing one. That is the shape `var x = []Migration{{...}}`
// takes, which is the shape the false positive came in.
func markCompositeWrites(lit *ast.CompositeLit, inherited string, order map[string][]string, writes map[string]bool) {
	name := inherited
	if lit.Type != nil {
		name = namedType(lit.Type)
	}

	// What an inner literal of this one would be: the element type of a slice,
	// array or map, or nothing.
	elem := ""
	switch t := lit.Type.(type) {
	case *ast.ArrayType:
		elem = namedType(t.Elt)
	case *ast.MapType:
		elem = namedType(t.Value)
	}

	for i, elt := range lit.Elts {
		switch e := elt.(type) {
		case *ast.KeyValueExpr:
			if id, ok := e.Key.(*ast.Ident); ok {
				writes[id.Name] = true
			}
			if inner, ok := e.Value.(*ast.CompositeLit); ok {
				markCompositeWrites(inner, elem, order, writes)
			}
		case *ast.CompositeLit:
			markCompositeWrites(e, elem, order, writes)
		default:
			// A positional element fills the i-th field of the named struct.
			if fields, ok := order[name]; ok && i < len(fields) && fields[i] != "" {
				writes[fields[i]] = true
			}
		}
	}

	// A positional literal whose elements are themselves composites still fills
	// those positions, so walk them for the field names as well as recursing.
	if name != "" {
		if fields, ok := order[name]; ok {
			for i, elt := range lit.Elts {
				if _, keyed := elt.(*ast.KeyValueExpr); keyed {
					continue
				}
				if i < len(fields) && fields[i] != "" {
					writes[fields[i]] = true
				}
			}
		}
	}
}

// namedType reduces a type expression to the bare type name this analysis keys
// on, seeing through pointers and package qualifiers.
func namedType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.StarExpr:
		return namedType(t.X)
	case *ast.ArrayType:
		return namedType(t.Elt)
	}
	return ""
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
	b.WriteString("#\n")
	b.WriteString("# A line marked (dead file) declares the field in a file that also has\n")
	b.WriteString("# entries in the dead-code budget. Those fields are usually dark BECAUSE\n")
	b.WriteString("# nothing calls the functions that would fill them — triage them last.\n")
	dead := deadcodeFiles()
	for _, l := range lines {
		d := site[l]
		mark := ""
		if dead[d.File] {
			mark = "\t(dead file)"
		}
		fmt.Fprintf(&b, "%s\t%s:%d%s\n", l, d.File, d.Line, mark)
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
