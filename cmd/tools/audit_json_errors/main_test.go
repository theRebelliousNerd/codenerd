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
