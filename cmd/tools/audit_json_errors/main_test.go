package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The two ways an error gets discarded, and the neighbours that must not be
// confused with them. The quiet one is the bare ExprStmt: there is no `_` in
// the source, so nothing marks it as a decision.
func TestDiscarded(t *testing.T) {
	for _, tc := range []struct {
		body string
		want string
	}{
		{`b, _ := json.Marshal(v)`, "Marshal"},
		{`b, _ = json.Marshal(v)`, "Marshal"},
		{`b, _ := json.MarshalIndent(v, "", "  ")`, "MarshalIndent"},
		{`json.Unmarshal(b, &v)`, "Unmarshal"},

		// Handled, so not findings.
		{`b, err := json.Marshal(v)`, ""},
		{`if err := json.Unmarshal(b, &v); err != nil { return err }`, ""},

		// Discarding the VALUE and keeping the error is fine, and is not the
		// same statement shape -- the blank has to be in the error position.
		{`_, err := json.Marshal(v)`, ""},

		// Other packages and other functions in this one.
		{`b, _ := yaml.Marshal(v)`, ""},
		{`b, _ := json.NewEncoder(w)`, ""},
		{`b, _ := proto.Marshal(v)`, ""},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
			"package fixture\nfunc probe(){"+tc.body+"}", 0)
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		var got string
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(ast.Stmt)
			if !ok {
				return true
			}
			if fn, _, ok := discarded(stmt); ok {
				got = fn
				return false
			}
			return true
		})
		if got != tc.want {
			t.Errorf("discarded(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

// The key is what makes the baseline survive ordinary edits. It carries the
// package, the enclosing function including its receiver, and the argument
// text -- but never a line number, so inserting code above a call does not
// report it as new.
func TestEnclosingFuncNamesTheReceiver(t *testing.T) {
	src := `package fixture
func plain() { b, _ := json.Marshal(v); _ = b }
func (s *LocalStore) method() { b, _ := json.Marshal(v); _ = b }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(ast.Stmt)
		if !ok {
			return true
		}
		if _, _, ok := discarded(stmt); ok {
			names = append(names, enclosingFunc(file, stmt.Pos()))
		}
		return true
	})
	want := []string{"plain", "LocalStore.method"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("enclosing functions = %v, want %v", names, want)
	}
}

// diff compares multisets, not sets. Two identical calls in one function are
// two entries, so replacing a fixed one with a fresh copy cannot slip through
// as a no-op.
func TestDiffCountsDuplicates(t *testing.T) {
	base := []string{"p\tf\tjson.Marshal(v)", "p\tf\tjson.Marshal(v)"}

	if added, removed := diff(base, base); len(added) != 0 || len(removed) != 0 {
		t.Errorf("identical multisets differ: added=%v removed=%v", added, removed)
	}

	added, removed := diff(base, base[:1])
	if len(added) != 0 || len(removed) != 1 {
		t.Errorf("losing one of two duplicates: added=%v removed=%v", added, removed)
	}

	added, removed = diff(base[:1], base)
	if len(added) != 1 || len(removed) != 0 {
		t.Errorf("gaining a duplicate: added=%v removed=%v", added, removed)
	}

	added, removed = diff([]string{"a"}, []string{"b"})
	if len(added) != 1 || len(removed) != 1 {
		t.Errorf("a swap should report both sides: added=%v removed=%v", added, removed)
	}
}

func writeAuditFixture(t *testing.T, dir, path, source string) {
	t.Helper()
	fullPath := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func TestScanFindingsAndBaselineRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fixtures := map[string]string{
		"internal/example/a.go":               "package fixture\nfunc save() {\n b, _ := json.Marshal(v)\n b, _ = json.Marshal(v)\n}\n",
		"internal/example/b.go":               "package fixture\nfunc (s *Store) save() {\n json.Unmarshal(data, &value)\n}\n",
		"cmd/example/main.go":                 "package fixture\nfunc log() {\n _, _ = json.Marshal(struct {\n Value string\n }{Value: \"value\"})\n}\n",
		"pkg/example/init.go":                 "package fixture\nvar result = func() int {\n json.Unmarshal(data, &value)\n return 0\n}()\n",
		"internal/example/ignored_test.go":    "not Go syntax",
		"internal/example/readme.txt":         "not Go syntax",
		"internal/vendor/nested/ignored.go":   "not Go syntax",
		"internal/testdata/nested/ignored.go": "not Go syntax",
		"internal/.hidden/nested/ignored.go":  "not Go syntax",
	}
	for path, source := range fixtures {
		writeAuditFixture(t, dir, path, source)
	}
	t.Chdir(dir)

	got, err := scan()
	if err != nil {
		t.Fatalf("scan(): %v", err)
	}
	want := []finding{
		{key: "cmd/example\tlog\tjson.Marshal(struct { Value string }{Value: \"value\"})", file: "cmd/example/main.go", line: 3},
		{key: "internal/example\tStore.save\tjson.Unmarshal(data)", file: "internal/example/b.go", line: 3},
		{key: "internal/example\tsave\tjson.Marshal(v)", file: "internal/example/a.go", line: 3},
		{key: "internal/example\tsave\tjson.Marshal(v)", file: "internal/example/a.go", line: 4},
		{key: "pkg/example\t<file scope>\tjson.Unmarshal(data)", file: "pkg/example/init.go", line: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scan() = %#v, want %#v", got, want)
	}

	keys := make([]string, len(got))
	for i, f := range got {
		keys[i] = f.key
	}
	writeAuditFixture(t, dir, baselinePath, header+strings.Join(keys, "\n")+"\n")
	baseline, err := readBaseline()
	if err != nil {
		t.Fatalf("readBaseline(): %v", err)
	}
	if !reflect.DeepEqual(baseline, keys) {
		t.Errorf("baseline round trip = %q, want %q", baseline, keys)
	}
	if added, removed := diff(baseline, keys); len(added) != 0 || len(removed) != 0 {
		t.Errorf("baseline round trip drift: added=%q removed=%q", added, removed)
	}
}

func TestScanRejectsEmptyOrMalformedTrees(t *testing.T) {
	for _, tt := range []struct {
		name         string
		files        map[string]string
		wantErr      bool
		wantParseErr bool
	}{
		{name: "missing roots", wantErr: true},
		{name: "only excluded files", files: map[string]string{
			"internal/testdata/ignored.go": "not Go syntax",
			"internal/ignored_test.go":     "not Go syntax",
			"internal/readme.txt":          "not Go syntax",
		}, wantErr: true},
		{name: "clean source with missing sibling roots", files: map[string]string{
			"internal/clean.go": "package fixture\nfunc clean() {}\n",
		}},
		{name: "parse failure discards partial findings", files: map[string]string{
			"internal/a.go": "package fixture\nfunc save() { _, _ = json.Marshal(v) }\n",
			"internal/z.go": "package fixture\nfunc broken(\n",
		}, wantErr: true, wantParseErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for path, source := range tt.files {
				writeAuditFixture(t, dir, path, source)
			}
			t.Chdir(dir)
			got, err := scan()
			if (err != nil) != tt.wantErr {
				t.Fatalf("scan() error = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != 0 {
				t.Errorf("scan() returned findings from an empty, clean or invalid tree: %#v", got)
			}
			if tt.wantParseErr {
				var parseErrors scanner.ErrorList
				if !errors.As(err, &parseErrors) {
					t.Errorf("scan() error = %v, want wrapped scanner.ErrorList", err)
				}
			}
		})
	}
}

func TestReadBaselinePreservesSortedDuplicates(t *testing.T) {
	for _, tt := range []struct {
		name    string
		body    string
		missing bool
		want    []string
	}{
		{name: "missing", missing: true},
		{name: "empty"},
		{name: "comments and blank lines", body: "# header\n\n# another comment\n"},
		{name: "LF without final newline", body: "# header\nz\na\na", want: []string{"a", "a", "z"}},
		{name: "CRLF", body: "# header\r\n\r\nz\r\na\r\na\r\n", want: []string{"a", "a", "z"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if !tt.missing {
				writeAuditFixture(t, dir, baselinePath, tt.body)
			}
			t.Chdir(dir)
			got, err := readBaseline()
			if tt.missing {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("readBaseline() error = %v, want fs.ErrNotExist", err)
				}
			} else if err != nil {
				t.Fatalf("readBaseline(): %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readBaseline() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOneLineCollapsesWhitespace(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty"},
		{name: "whitespace only", input: " \t\r\n "},
		{name: "already one line", input: "value.Field", want: "value.Field"},
		{name: "multiline expression", input: "\nstruct {\r\n\tValue string\n}{Value: value}\t", want: "struct { Value string }{Value: value}"},
		{name: "unicode whitespace", input: " left\u2003right\u00a0", want: "left right"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := oneLine(tt.input); got != tt.want {
				t.Errorf("oneLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestJsonCallOnlyMatchesErrorReturningCalls(t *testing.T) {
	for _, tc := range []struct {
		name   string
		src    string
		wantFn string
		wantOK bool
	}{
		{name: "Marshal", src: `json.Marshal(v)`, wantFn: "Marshal", wantOK: true},
		{name: "MarshalIndent", src: `json.MarshalIndent(v, "", "  ")`, wantFn: "MarshalIndent", wantOK: true},
		{name: "Unmarshal", src: `json.Unmarshal(b, &v)`, wantFn: "Unmarshal", wantOK: true},
		{name: "NewEncoder has no error to discard", src: `json.NewEncoder(w)`, wantOK: false},
		{name: "other package", src: `yaml.Marshal(v)`, wantOK: false},
		{name: "unqualified call", src: `Marshal(v)`, wantOK: false},
		{name: "no args", src: `json.Marshal()`, wantOK: false},
		{name: "Decode is out of scope", src: `json.NewDecoder(r).Decode(&v)`, wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
				"package fixture\nfunc probe(){ _ = "+tc.src+" }", 0)
			if err != nil {
				t.Fatal(err)
			}
			var call *ast.CallExpr
			ast.Inspect(file, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok && call == nil {
					call = c
				}
				return true
			})
			if call == nil {
				t.Fatalf("no call found in %q", tc.src)
			}
			// For the Decode case the outer call is `.Decode`, not `json.*`,
			// so jsonCall must reject it even though the receiver mentions json.
			fn, _, ok := jsonCall(call)
			if ok != tc.wantOK || fn != tc.wantFn {
				t.Errorf("jsonCall(%q) = (%q, %v), want (%q, %v)", tc.src, fn, ok, tc.wantFn, tc.wantOK)
			}
		})
	}
}

func TestDiscardedRejectsOtherShapes(t *testing.T) {
	for _, tc := range []struct {
		body string
		want string
	}{
		// A bare Marshal call discards both results, error included.
		{`json.Marshal(v)`, "Marshal"},
		// Single-value assignment keeps no error to discard.
		{`b := json.Marshal(v)`, ""},
		// Three names on the left is not the (value, error) shape.
		{`a, b, _ := json.Marshal(v)`, ""},
		// Right-hand side is not a call at all.
		{`b, _ := x`, ""},
		// Call outside encoding/json.
		{`b, _ := fmt.Sprintf("%v", v)`, ""},
		// Marshal with no argument cannot match jsonCall.
		{`b, _ := json.Marshal()`, ""},
		// Bare non-JSON call and bare identifier are not findings.
		{`fmt.Println(v)`, ""},
		{`x`, ""},
	} {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
			"package fixture\nfunc probe(){"+tc.body+"}", 0)
		if err != nil {
			t.Fatalf("%s: %v", tc.body, err)
		}
		var got string
		ast.Inspect(file, func(n ast.Node) bool {
			stmt, ok := n.(ast.Stmt)
			if !ok {
				return true
			}
			if fn, _, ok := discarded(stmt); ok {
				got = fn
				return false
			}
			return true
		})
		if got != tc.want {
			t.Errorf("discarded(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

func TestEnclosingFuncFileScopeAndValueReceiver(t *testing.T) {
	src := `package fixture
var result = func() int {
 json.Unmarshal(data, &value)
 return 0
}()
func (s Store) valueMethod() { b, _ := json.Marshal(v); _ = b }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		stmt, ok := n.(ast.Stmt)
		if !ok {
			return true
		}
		if _, _, ok := discarded(stmt); ok {
			names = append(names, enclosingFunc(file, stmt.Pos()))
		}
		return true
	})
	want := []string{"<file scope>", "Store.valueMethod"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("enclosing functions = %v, want %v", names, want)
	}
}

func TestDiffReturnsSortedResults(t *testing.T) {
	added, removed := diff([]string{"q"}, []string{"z", "a"})
	if !reflect.DeepEqual(added, []string{"a", "z"}) {
		t.Errorf("added unsorted input = %v, want [a z]", added)
	}
	if !reflect.DeepEqual(removed, []string{"q"}) {
		t.Errorf("removed = %v, want [q]", removed)
	}
	added, removed = diff([]string{"z", "a"}, []string{"q"})
	if len(added) != 1 || added[0] != "q" {
		t.Errorf("added = %v, want [q]", added)
	}
	if !reflect.DeepEqual(removed, []string{"a", "z"}) {
		t.Errorf("removed unsorted input = %v, want [a z]", removed)
	}
	if added, removed := diff(nil, nil); len(added) != 0 || len(removed) != 0 {
		t.Errorf("empty diff = (%v, %v), want empty", added, removed)
	}
}

func TestScanIgnoresExcludedTreesWithFindings(t *testing.T) {
	dir := t.TempDir()
	findingSrc := "package fixture\nfunc probe() { b, _ := json.Marshal(v); _ = b }\n"
	writeAuditFixture(t, dir, "internal/keep.go", findingSrc)
	writeAuditFixture(t, dir, "internal/skip_test.go", findingSrc)
	writeAuditFixture(t, dir, "internal/testdata/skip.go", findingSrc)
	writeAuditFixture(t, dir, "internal/vendor/skip.go", findingSrc)
	writeAuditFixture(t, dir, "internal/.hidden/skip.go", findingSrc)
	writeAuditFixture(t, dir, "internal/notes.txt", findingSrc)
	t.Chdir(dir)
	got, err := scan()
	if err != nil {
		t.Fatalf("scan(): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("scan() = %#v, want exactly one finding from internal/keep.go", got)
	}
	if got[0].file != "internal/keep.go" {
		t.Errorf("scan() file = %q, want %q", got[0].file, "internal/keep.go")
	}
	wantKey := "internal\tprobe\tjson.Marshal(v)"
	if got[0].key != wantKey {
		t.Errorf("scan() key = %q, want %q", got[0].key, wantKey)
	}
}

func TestScanSortsFindingsDeterministically(t *testing.T) {
	dir := t.TempDir()
	writeAuditFixture(t, dir, "internal/a.go", "package fixture\nfunc zebra() { b, _ := json.Marshal(zebra); _ = b }\n")
	writeAuditFixture(t, dir, "internal/b.go", "package fixture\nfunc apple() { b, _ := json.Marshal(apple); _ = b }\n")
	writeAuditFixture(t, dir, "internal/c.go", "package fixture\nfunc same() { b, _ := json.Marshal(v); _ = b }\n")
	writeAuditFixture(t, dir, "internal/d.go", "package fixture\nfunc same() { b, _ := json.Marshal(v); _ = b }\n")
	t.Chdir(dir)
	got, err := scan()
	if err != nil {
		t.Fatalf("scan(): %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("scan() = %#v, want 4 findings", got)
	}
	wantKeys := []string{
		"internal\tapple\tjson.Marshal(apple)",
		"internal\tsame\tjson.Marshal(v)",
		"internal\tsame\tjson.Marshal(v)",
		"internal\tzebra\tjson.Marshal(zebra)",
	}
	for i, want := range wantKeys {
		if got[i].key != want {
			t.Errorf("scan()[%d].key = %q, want %q (full: %#v)", i, got[i].key, want, got)
		}
	}
	// Identical keys order by file, proving the file tie-breaker.
	if got[1].file != "internal/c.go" || got[2].file != "internal/d.go" {
		t.Errorf("same-key order = %q, %q, want internal/c.go then internal/d.go", got[1].file, got[2].file)
	}
}

func TestScanFileTiebreakerPinsSortChange(t *testing.T) {
	dir := t.TempDir()
	aSrc := "package p\nimport \"encoding/json\"\n" + strings.Repeat("\n", 20) + "func Same() { json.Marshal(1) }\n"
	bSrc := "package p\nimport \"encoding/json\"\nfunc Same() { json.Marshal(1) }\n"
	writeAuditFixture(t, dir, "a.go", aSrc)
	writeAuditFixture(t, dir, "b.go", bSrc)
	oldRoots := roots
	roots = []string{dir}
	defer func() { roots = oldRoots }()
	findings, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("want 2 findings, got %d: %v", len(findings), findings)
	}
	if findings[0].key != findings[1].key {
		t.Fatalf("want equal keys to exercise file tiebreaker, got %q vs %q", findings[0].key, findings[1].key)
	}
	if !strings.HasSuffix(findings[0].file, "a.go") || !strings.HasSuffix(findings[1].file, "b.go") {
		t.Fatalf("want file order a.go then b.go, got %q then %q (lines %d, %d)", findings[0].file, findings[1].file, findings[0].line, findings[1].line)
	}
}

func TestScanPinsEntryAndFileTiebreakerSort(t *testing.T) {
	dir := t.TempDir()
	writeAuditFixture(t, dir, "internal/dup/a.go", "package fixture\nfunc save() {\n b, _ := json.Marshal(v)\n _ = b\n}\n")
	writeAuditFixture(t, dir, "internal/dup/b.go", "package fixture\nfunc save() {\n b, _ := json.Marshal(v)\n _ = b\n}\n")
	writeAuditFixture(t, dir, "internal/dup/c.go", "package fixture\nfunc other() {\n b, _ := json.Marshal(w)\n _ = b\n}\n")
	writeAuditFixture(t, dir, "internal/dup/d.go", "package fixture\nfunc multi() {\n b, _ := json.Marshal(v)\n _ = b\n b, _ := json.Marshal(v)\n _ = b\n}\n")
	t.Chdir(dir)
	got, err := scan()
	if err != nil {
		t.Fatalf("scan(): %v", err)
	}
	if len(got) == 0 {
		t.Fatalf("scan() returned no findings, want file-tiebreaker fixtures")
	}
	// The comparator must order by key, then file, then line.
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.key > cur.key {
			t.Fatalf("out of key order at %d: %q > %q", i, prev.key, cur.key)
		}
		if prev.key == cur.key && prev.file > cur.file {
			t.Fatalf("out of file order for equal key %q at %d: %q > %q", prev.key, i, prev.file, cur.file)
		}
		if prev.key == cur.key && prev.file == cur.file && prev.line > cur.line {
			t.Fatalf("out of line order for %q %q at %d: %d > %d", prev.key, prev.file, i, prev.line, cur.line)
		}
	}
	// Same key in two files must be ordered by file: the file tiebreaker.
	var dupFiles []string
	for _, f := range got {
		if f.key == "internal/dup\tsave\tjson.Marshal(v)" {
			dupFiles = append(dupFiles, f.file)
		}
	}
	if len(dupFiles) != 2 {
		t.Fatalf("want 2 dup save findings, got %v (%#v)", dupFiles, got)
	}
	if dupFiles[0] != "internal/dup/a.go" || dupFiles[1] != "internal/dup/b.go" {
		t.Fatalf("file tiebreaker order = %v, want [internal/dup/a.go internal/dup/b.go]", dupFiles)
	}
	// Same key and file must be ordered by line.
	var multiLines []int
	for _, f := range got {
		if f.key == "internal/dup\tmulti\tjson.Marshal(v)" {
			multiLines = append(multiLines, f.line)
		}
	}
	if len(multiLines) != 2 || multiLines[0] >= multiLines[1] {
		t.Fatalf("line tiebreaker order = %v, want two increasing lines", multiLines)
	}
}
