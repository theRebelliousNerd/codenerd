package main

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// The shape, and the neighbours that must not be confused with it.
func TestSelectsKeyByComparison(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{
			name: "the defect: the key becomes the running best",
			body: `for name, n := range m { if n > max { max = n; best = name } }`,
			want: true,
		},
		{
			name: "same shape with a less-than",
			body: `for name, n := range m { if n < min { min = n; best = name } }`,
			want: true,
		},
		{
			name: "multi-assign form",
			body: `for name, n := range m { if n > max { max, best = n, name } }`,
			want: true,
		},

		// Not the defect.
		{
			name: "the VALUE is recorded, not the key",
			body: `for _, v := range m { if v > max { max = v } }`,
			want: false,
		},
		{
			name: "a size limit, not a selection",
			body: `for _, v := range m { if len(v) > maxBytes { return errTooBig } }`,
			want: false,
		},
		{
			name: "equality, not an ordering",
			body: `for name, n := range m { if n == want { best = name } }`,
			want: false,
		},
		{
			name: "a blank key cannot be selected",
			body: `for _, n := range m { if n > max { max = n } }`,
			want: false,
		},
		{
			name: "no comparison at all",
			body: `for name, n := range m { total += n; last = name }`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
				"package fixture\nfunc probe(){"+tc.body+"}", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := false
			ast.Inspect(file, func(n ast.Node) bool {
				if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
					got = true
					return false
				}
				return true
			})
			if got != tc.want {
				t.Errorf("selectsKeyByComparison = %v, want %v for: %s", got, tc.want, tc.body)
			}
		})
	}
}

// Stabilized comparison forms: >=, <=, parenthesised conditions and the
// else branch must keep selecting the key, and degenerate ranges must stay
// rejected without panicking. Repeated evaluation must be stable.
func TestSelectsKeyByComparisonStabilizedForms(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{
			name: "greater-or-equal still selects",
			body: `for name, n := range m { if n >= max { max, best = n, name } }`,
			want: true,
		},
		{
			name: "less-or-equal still selects",
			body: `for name, n := range m { if n <= min { min, best = n, name } }`,
			want: true,
		},
		{
			name: "parenthesised comparison still selects",
			body: `for name, n := range m { if ((n > max)) { best = name } }`,
			want: true,
		},
		{
			name: "key recorded in else branch still selects",
			body: `for name, n := range m { if n > max { max = n } else { best = name } }`,
			want: true,
		},
		{
			name: "single-variable range has no value to compare",
			body: `for name := range m { if name > maxName { best = name } }`,
			want: false,
		},
		{
			name: "value recorded instead of key stays rejected",
			body: `for name, n := range m { if name > maxName { best = n } }`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
				"package fixture\nfunc probe(){"+tc.body+"}", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var rng *ast.RangeStmt
			ast.Inspect(file, func(n ast.Node) bool {
				if r, ok := n.(*ast.RangeStmt); ok && rng == nil {
					rng = r
				}
				return true
			})
			if rng == nil {
				t.Fatalf("no range statement in: %s", tc.body)
			}
			first := selectsKeyByComparison(rng)
			second := selectsKeyByComparison(rng)
			if first != tc.want {
				t.Errorf("selectsKeyByComparison = %v, want %v for: %s", first, tc.want, tc.body)
			}
			if second != first {
				t.Errorf("selectsKeyByComparison unstable: first = %v, second = %v for: %s", first, second, tc.body)
			}
		})
	}
}

func TestStabilizePinsSelectsHardenedNestedElse(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{
			name: "nested key inside index still selects",
			body: `for k, v := range m { if v > max { best = m[k]; max = v } }`,
			want: true,
		},
		{
			name: "nested key inside call still selects",
			body: `for k, v := range m { if v > max { best = id(k); max = v } }`,
			want: true,
		},
		{
			name: "inclusive guard with key only in else still selects",
			body: `for k, v := range m { if v >= max { max = v } else { best = k } }`,
			want: true,
		},
		{
			name: "second selecting if is skipped once found stays true",
			body: `for k, v := range m { if v > max { best = k }; if v < min { best = k } }`,
			want: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
				"package fixture\nfunc probe(){"+tc.body+"}", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			var rng *ast.RangeStmt
			ast.Inspect(file, func(n ast.Node) bool {
				if r, ok := n.(*ast.RangeStmt); ok && rng == nil {
					rng = r
				}
				return true
			})
			if rng == nil {
				t.Fatalf("no range statement in: %s", tc.body)
			}
			if got := selectsKeyByComparison(rng); got != tc.want {
				t.Errorf("selectsKeyByComparison = %v, want %v for: %s", got, tc.want, tc.body)
			}
		})
	}

	t.Run("non-ident key is rejected without panic", func(t *testing.T) {
		file, err := parser.ParseFile(token.NewFileSet(), "fixture.go",
			"package fixture\nfunc probe(s *struct{ k string }){for s.k, v := range m { if v > max { best = s.k } }}", 0)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
				t.Errorf("selectsKeyByComparison = true, want false for non-ident key")
				return false
			}
			return true
		})
	})

	t.Run("nil cond and nil if body are rejected without panic", func(t *testing.T) {
		nilCond := &ast.RangeStmt{
			Key:   &ast.Ident{Name: "k"},
			Value: &ast.Ident{Name: "v"},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.IfStmt{Cond: nil, Body: &ast.BlockStmt{}},
			}},
		}
		if selectsKeyByComparison(nilCond) {
			t.Errorf("selectsKeyByComparison with nil cond = true, want false")
		}
		nilBody := &ast.RangeStmt{
			Key:   &ast.Ident{Name: "k"},
			Value: &ast.Ident{Name: "v"},
			Body: &ast.BlockStmt{List: []ast.Stmt{
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X:  &ast.Ident{Name: "v"},
						Op: token.GTR,
						Y:  &ast.Ident{Name: "max"},
					},
					Body: nil,
				},
			}},
		}
		if selectsKeyByComparison(nilBody) {
			t.Errorf("selectsKeyByComparison with nil if body = true, want false")
		}
	})
}

// The scan walks roots in fixed root order, not sorted-key order, so the
// reported findings must be sorted on every run regardless of walk order.
func TestScanFindingsOrderIsDeterministic(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"internal/zzz/b.go": "package fixture\nfunc Aaa() {\n\tfor k, v := range m { if v > max { max, best = v, k } }\n}\n",
		"cmd/aaa/a.go":      "package fixture\nfunc Zzz() {\n\tfor k, v := range m { if v > max { max, best = v, k } }\n}\n",
	}
	for path, source := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	first, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	second, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if diff := cmp.Diff(first, second, cmp.AllowUnexported(finding{})); diff != "" {
		t.Fatalf("scan() unstable across runs (-first +second):\n%s", diff)
	}
	want := []finding{
		{key: "cmd/aaa\tZzz\trange k", file: "cmd/aaa/a.go", line: 3},
		{key: "internal/zzz\tAaa\trange k", file: "internal/zzz/b.go", line: 3},
	}
	if diff := cmp.Diff(want, first, cmp.AllowUnexported(finding{})); diff != "" {
		t.Errorf("scan() mismatch (-want +got):\n%s", diff)
	}
}

// The key must survive ordinary edits, or the baseline churns and stops being
// read. It carries the package, the enclosing function with its receiver, and
// the range variable — never a line number.
func TestEnclosingFuncNamesTheReceiver(t *testing.T) {
	src := `package fixture
func plain() { for k, v := range m { if v > max { max = v; best = k } } }
func (s *Store) method() { for k, v := range m { if v > max { max = v; best = k } } }
`
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
			names = append(names, enclosingFunc(file, rng.Pos()))
		}
		return true
	})
	if len(names) != 2 || names[0] != "plain" || names[1] != "Store.method" {
		t.Errorf("enclosing functions = %v, want [plain Store.method]", names)
	}
}

func TestScanFindingsAreSortedAndFiltered(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"internal/choices/z.go": `package choices
func Zebra() {
	for name, n := range m { if n > max { max, best = n, name } }
	for name, n := range m { if n < min { min, best = n, name } }
}
func (s *Store) Alpha() {
	for key, value := range m { if value > max { max, best = value, key } }
}
`,
		"cmd/select/main.go": `package main
func choose() {
	for k, v := range m { if v > max { max, best = v, k } }
}
`,
		"pkg/clean/clean.go":               "package clean\nfunc count() { for _, n := range m { total += n } }\n",
		"internal/choices/ignored_test.go": "not Go source",
		"internal/choices/notes.txt":       "not Go source",
		"internal/vendor/ignored.go":       "not Go source",
		"internal/testdata/ignored.go":     "not Go source",
		"internal/.hidden/ignored.go":      "not Go source",
	}
	for path, source := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}

	got, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	want := []finding{
		{key: "cmd/select\tchoose\trange k", file: "cmd/select/main.go", line: 3},
		{key: "internal/choices\tStore.Alpha\trange key", file: "internal/choices/z.go", line: 7},
		{key: "internal/choices\tZebra\trange name", file: "internal/choices/z.go", line: 3},
		{key: "internal/choices\tZebra\trange name", file: "internal/choices/z.go", line: 4},
	}
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(finding{})); diff != "" {
		t.Errorf("scan() mismatch (-want +got):\n%s", diff)
	}
}

func TestScanDistinguishesCleanAndEmptyTrees(t *testing.T) {
	for _, tt := range []struct {
		name    string
		files   map[string]string
		wantErr bool
	}{
		{name: "missing roots", wantErr: true},
		{name: "empty roots", files: map[string]string{"internal": "", "cmd": "", "pkg": ""}, wantErr: true},
		{name: "only tests", files: map[string]string{"internal/only_test.go": "package fixture\n"}, wantErr: true},
		{name: "only excluded source", files: map[string]string{"internal/testdata/ignored.go": "package fixture\n"}, wantErr: true},
		{name: "clean production source", files: map[string]string{"pkg/clean.go": "package fixture\n"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			for path, source := range tt.files {
				if source == "" {
					if err := os.MkdirAll(path, 0o755); err != nil {
						t.Fatalf("create empty root %s: %v", path, err)
					}
					continue
				}
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatalf("create fixture directory for %s: %v", path, err)
				}
				if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
					t.Fatalf("write fixture %s: %v", path, err)
				}
			}
			got, err := scan()
			if (err != nil) != tt.wantErr {
				t.Fatalf("scan() error = %v, want error %v", err, tt.wantErr)
			}
			if len(got) != 0 {
				t.Errorf("scan() findings = %+v, want none", got)
			}
		})
	}
}

func TestScanPropagatesParseErrorsWithoutPartialFindings(t *testing.T) {
	for _, tt := range []struct {
		name string
		root string
	}{
		{name: "internal", root: "internal"},
		{name: "cmd", root: "cmd"},
		{name: "pkg", root: "pkg"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.MkdirAll(tt.root, 0o755); err != nil {
				t.Fatalf("create fixture root: %v", err)
			}
			valid := filepath.Join(tt.root, "a.go")
			if err := os.WriteFile(valid, []byte("package fixture\nfunc choose() { for k, v := range m { if v > max { max, best = v, k } } }\n"), 0o600); err != nil {
				t.Fatalf("write valid source: %v", err)
			}
			invalid := filepath.Join(tt.root, "z.go")
			if err := os.WriteFile(invalid, []byte("package fixture\nfunc broken(\n"), 0o600); err != nil {
				t.Fatalf("write invalid source: %v", err)
			}
			got, err := scan()
			var parseErrors scanner.ErrorList
			if !errors.As(err, &parseErrors) {
				t.Fatalf("scan() error = %v, want wrapped scanner.ErrorList", err)
			}
			if len(parseErrors) == 0 || parseErrors[0].Pos.Filename != invalid {
				t.Errorf("parse errors = %v, want error in %s", parseErrors, invalid)
			}
			if got != nil {
				t.Errorf("scan() findings = %+v, want nil on parse failure", got)
			}
		})
	}
}

func TestStabilizeCoversNilRangeGuards(t *testing.T) {
	if selectsKeyByComparison(nil) {
		t.Fatalf("selectsKeyByComparison(nil) = true, want false")
	}
	if selectsKeyByComparison(&ast.RangeStmt{Body: nil}) {
		t.Fatalf("selectsKeyByComparison with nil Body = true, want false")
	}
	empty := &ast.RangeStmt{Key: nil, Value: nil, Body: &ast.BlockStmt{}}
	if selectsKeyByComparison(empty) {
		t.Fatalf("selectsKeyByComparison with nil Key/Value = true, want false")
	}
}

func TestStabilizeCoversHitEarlyExit(t *testing.T) {
	src := "package p\nfunc f(m map[string]int) {\nmax := 0\nbest := \"\"\nfor k, v := range m {\nif v > max {\nbest = k + \"x\"\nmax = v\n}\n}\n_ = best\n}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		rng, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		if selectsKeyByComparison(rng) {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatalf("composite RHS fixture should select, got false")
	}
}

func TestStabilizeCoversEnclosingFuncNil(t *testing.T) {
	if got := enclosingFunc(nil, token.NoPos); got != "<file scope>" {
		t.Fatalf("enclosingFunc(nil, NoPos) = %q, want %q", got, "<file scope>")
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", "package p\nfunc Foo() {}\n", 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if got := enclosingFunc(f, token.NoPos); got != "<file scope>" {
		t.Fatalf("enclosingFunc(valid file, NoPos) = %q, want %q", got, "<file scope>")
	}
}

func TestStabilizeCoversSortTiebreakSecondKey(t *testing.T) {
	dir := t.TempDir()
	loopSrc := "package p\nfunc Same(m map[string]int) string {\nmax := 0\nbest := \"\"\nfor k, v := range m {\nif v > max {\nmax = v\nbest = k\n}\n}\nreturn best\n}\n"
	for _, name := range []string{"a.go", "b.go"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(loopSrc), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	oldRoots := roots
	roots = []string{dir}
	defer func() { roots = oldRoots }()
	found, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("scan found %d findings, want 2 (same key, different files)", len(found))
	}
	if found[0].key != found[1].key {
		t.Fatalf("expected identical keys to exercise file tiebreak, got %q vs %q", found[0].key, found[1].key)
	}
	if !(found[0].file < found[1].file) {
		t.Fatalf("findings not ordered by file tiebreak: %q before %q", found[0].file, found[1].file)
	}
}

func TestStabilizePinsScanSortFilePrimary(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"cmd/select/a.go":       "package main\nfunc Zebra() {\n\tfor k, v := range m { if v > max { max, best = v, k } }\n}\n",
		"internal/choices/b.go": "package choices\nfunc Alpha() {\n\tfor k, v := range m { if v > max { max, best = v, k } }\n}\n",
	}
	for path, source := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}
	got, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("scan() findings = %+v, want 2", got)
	}
	if got[0].file != "cmd/select/a.go" || got[1].file != "internal/choices/b.go" {
		t.Errorf("scan() file order = [%s %s], want [cmd/select/a.go internal/choices/b.go] (file-primary, not key order)", got[0].file, got[1].file)
	}
}

func TestStabilizePinsScanSortKeyTiebreakSameFile(t *testing.T) {
	t.Chdir(t.TempDir())
	source := "package choices\nfunc Zebra() {\n\tfor name, n := range m { if n > max { max, best = n, name } }\n}\nfunc Alpha() {\n\tfor name, n := range m { if n > max { max, best = n, name } }\n}\n"
	if err := os.MkdirAll("internal/choices", 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	if err := os.WriteFile("internal/choices/z.go", []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("scan() findings = %+v, want 2", got)
	}
	if got[0].file != "internal/choices/z.go" || got[1].file != "internal/choices/z.go" {
		t.Fatalf("scan() files = [%s %s], want same file twice", got[0].file, got[1].file)
	}
	if diff := cmp.Diff("internal/choices\tAlpha\trange name", got[0].key); diff != "" {
		t.Errorf("scan()[0].key mismatch (-want +got):\n%s (full: %+v)", diff, got)
	}
	if diff := cmp.Diff("internal/choices\tZebra\trange name", got[1].key); diff != "" {
		t.Errorf("scan()[1].key mismatch (-want +got):\n%s (full: %+v)", diff, got)
	}
}

func TestStabilizePinsScanFiltersExcludedPaths(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{
		"cmd/select/main.go":               "package main\nfunc choose() {\n\tfor k, v := range m { if v > max { max, best = v, k } }\n}\n",
		"internal/vendor/ignored.go":       "package fixture\nfunc ignored() { for k, v := range m { if v > max { max, best = v, k } } }\n",
		"internal/testdata/ignored.go":     "package fixture\nfunc ignored() { for k, v := range m { if v > max { max, best = v, k } } }\n",
		"internal/.hidden/ignored.go":      "package fixture\nfunc ignored() { for k, v := range m { if v > max { max, best = v, k } } }\n",
		"internal/choices/ignored_test.go": "package choices\nfunc helper() { for k, v := range m { if v > max { max, best = v, k } } }\n",
		"internal/choices/notes.txt":       "not Go source",
	}
	for path, source := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create fixture directory for %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", path, err)
		}
	}
	got, err := scan()
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("scan() findings = %+v, want exactly 1 (excluded paths must not report)", got)
	}
	if got[0].file != "cmd/select/main.go" {
		t.Errorf("scan() file = %s, want cmd/select/main.go", got[0].file)
	}
}

func TestStabilizePinsScanMissingRootsError(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := scan()
	if err == nil {
		t.Fatalf("scan() error = nil with no roots, want error (findings %+v)", got)
	}
	if len(got) != 0 {
		t.Errorf("scan() findings = %+v, want none on missing roots", got)
	}
}

func TestStabilizePinsScanParseErrorNoPartial(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("internal", 0o755); err != nil {
		t.Fatalf("create fixture root: %v", err)
	}
	valid := filepath.Join("internal", "a.go")
	if err := os.WriteFile(valid, []byte("package fixture\nfunc choose() { for k, v := range m { if v > max { max, best = v, k } } }\n"), 0o600); err != nil {
		t.Fatalf("write valid source: %v", err)
	}
	invalid := filepath.Join("internal", "z.go")
	if err := os.WriteFile(invalid, []byte("package fixture\nfunc broken(\n"), 0o600); err != nil {
		t.Fatalf("write invalid source: %v", err)
	}
	got, err := scan()
	var parseErrors scanner.ErrorList
	if !errors.As(err, &parseErrors) {
		t.Fatalf("scan() error = %v, want wrapped scanner.ErrorList", err)
	}
	if got != nil {
		t.Errorf("scan() findings = %+v, want nil on parse failure", got)
	}
}

func TestStabilizePinsSelectsComparisonOperators(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "greater selects key", body: `for name, n := range m { if n > max { max, best = n, name } }`},
		{name: "less selects key", body: `for name, n := range m { if n < min { min, best = n, name } }`},
		{name: "greater-or-equal selects key", body: `for name, n := range m { if n >= max { max, best = n, name } }`},
		{name: "less-or-equal selects key", body: `for name, n := range m { if n <= min { min, best = n, name } }`},
		{name: "parenthesised comparison selects key", body: `for name, n := range m { if ((n > max)) { best = name } }`},
		{name: "key in else branch selects key", body: `for name, n := range m { if n > max { max = n } else { best = name } }`},
		{name: "single assign of key selects key", body: `for name, n := range m { if n > max { best = name } }`},
		{name: "composite RHS still selects key", body: `for k, v := range m { if v > max { best = k + "x"; max = v } }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nfunc probe(){"+tc.body+"}", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := false
			ast.Inspect(file, func(n ast.Node) bool {
				if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
					got = true
					return false
				}
				return true
			})
			if !got {
				t.Errorf("selectsKeyByComparison = false, want true for: %s", tc.body)
			}
		})
	}
}

func TestStabilizePinsSelectsRejectsNonSelections(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "equality does not select", body: `for name, n := range m { if n == want { best = name } }`},
		{name: "inequality does not select", body: `for name, n := range m { if n != want { best = name } }`},
		{name: "value recorded instead of key", body: `for name, n := range m { if n > max { best = n } }`},
		{name: "key compared instead of value", body: `for name, n := range m { if name > maxName { best = n } }`},
		{name: "blank key cannot be selected", body: `for _, n := range m { if n > max { max = n } }`},
		{name: "single-variable range has no value", body: `for name := range m { if name > maxName { best = name } }`},
		{name: "no comparison at all", body: `for name, n := range m { total += n; last = name }`},
		{name: "size limit returns", body: `for _, v := range m { if len(v) > maxBytes { return errTooBig } }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nfunc probe(){"+tc.body+"}", 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
					t.Errorf("selectsKeyByComparison = true, want false for: %s", tc.body)
					return false
				}
				return true
			})
		})
	}
}

func TestStabilizePinsSelectsNilGuards(t *testing.T) {
	if selectsKeyByComparison(nil) {
		t.Fatalf("selectsKeyByComparison(nil) = true, want false")
	}
	if selectsKeyByComparison(&ast.RangeStmt{Body: nil}) {
		t.Fatalf("selectsKeyByComparison with nil Body = true, want false")
	}
	empty := &ast.RangeStmt{Key: nil, Value: nil, Body: &ast.BlockStmt{}}
	if selectsKeyByComparison(empty) {
		t.Fatalf("selectsKeyByComparison with nil Key/Value = true, want false")
	}
}

func TestStabilizePinsSelectsNilIfBody(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\nfunc probe(){for k, v := range m { if v > max { } }}", 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var rng *ast.RangeStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if r, ok := n.(*ast.RangeStmt); ok && rng == nil {
			rng = r
		}
		return true
	})
	if rng == nil {
		t.Fatalf("no range statement in fixture")
	}
	if selectsKeyByComparison(rng) {
		t.Errorf("selectsKeyByComparison with empty if body = true, want false (no key recorded)")
	}
}

func TestStabilizePinsEnclosingFuncVariants(t *testing.T) {
	if got := enclosingFunc(nil, token.NoPos); got != "<file scope>" {
		t.Fatalf("enclosingFunc(nil, NoPos) = %q, want %q", got, "<file scope>")
	}
	src := "package fixture\nfunc plain() { for k, v := range m { if v > max { max = v; best = k } } }\nfunc (s *Store) method() { for k, v := range m { if v > max { max = v; best = k } } }\nfunc (s Store) value() { for k, v := range m { if v > max { max = v; best = k } } }\n"
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		if rng, ok := n.(*ast.RangeStmt); ok && selectsKeyByComparison(rng) {
			names = append(names, enclosingFunc(file, rng.Pos()))
		}
		return true
	})
	want := []string{"plain", "Store.method", "Store.value"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("enclosing functions mismatch (-want +got):\n%s", diff)
	}
	if got := enclosingFunc(file, token.NoPos); got != "<file scope>" {
		t.Errorf("enclosingFunc(valid file, NoPos) = %q, want %q", got, "<file scope>")
	}
}

func TestStabilizePinsEnclosingFuncHardened(t *testing.T) {
	t.Parallel()
	if got := enclosingFunc(nil, token.NoPos); got != "<file scope>" {
		t.Fatalf("enclosingFunc(nil, NoPos) = %q, want %q", got, "<file scope>")
	}
	src := "package p\n\ntype Recv struct{}\n\nfunc Plain() {}\n\nfunc (r Recv) Value() {}\n\nfunc (r *Recv) Ptr() {}\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	if got := enclosingFunc(f, token.NoPos); got != "<file scope>" {
		t.Fatalf("enclosingFunc(valid file, NoPos) = %q, want %q", got, "<file scope>")
	}
	if got := enclosingFunc(f, token.Pos(1)); got != "<file scope>" {
		t.Fatalf("enclosingFunc(pos outside funcs) = %q, want %q", got, "<file scope>")
	}
	want := map[string]string{
		"Plain": "Plain",
		"Value": "Recv.Value",
		"Ptr":   "Recv.Ptr",
	}
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		probe := fn.Body.Pos() + 1
		got := enclosingFunc(f, probe)
		if want[fn.Name.Name] == "" {
			t.Fatalf("unexpected func %q", fn.Name.Name)
		}
		if got != want[fn.Name.Name] {
			t.Errorf("enclosingFunc(%s body) = %q, want %q", fn.Name.Name, got, want[fn.Name.Name])
		}
	}
}

func TestStabilizePinsEnclosingFuncNilNameSkipped(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", "package p\nfunc Foo() {}\n", 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	fn := f.Decls[0].(*ast.FuncDecl)
	probe := fn.Body.Pos() + 1
	saved := fn.Name
	fn.Name = nil
	defer func() { fn.Name = saved }()
	if got := enclosingFunc(f, probe); got != "<file scope>" {
		t.Fatalf("enclosingFunc(nil Name) = %q, want %q", got, "<file scope>")
	}
}

func TestStabilizePinsEnclosingFuncNilRecvTypeFallsBack(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", "package p\ntype R struct{}\nfunc (r R) Foo() {}\n", 0)
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name != nil && fd.Name.Name == "Foo" {
			fn = fd
		}
	}
	if fn == nil {
		t.Fatal("Foo not found")
	}
	probe := fn.Body.Pos() + 1
	fn.Recv.List[0].Type = nil
	if got := enclosingFunc(f, probe); got != "Foo" {
		t.Fatalf("enclosingFunc(nil recv type) = %q, want %q", got, "Foo")
	}
}

func TestStabilizePinsNonOrderingIfDoesNotSelect(t *testing.T) {
	src := "package p\nfunc f(m map[string]int) string {\nbest := \"\"\nfor k, v := range m {\nif v == 1 {\nbest = k\n}\n}\nreturn best\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var rng *ast.RangeStmt
	ast.Inspect(file, func(n ast.Node) bool {
		if r, ok := n.(*ast.RangeStmt); ok {
			rng = r
			return false
		}
		return true
	})
	if rng == nil {
		t.Fatal("no range stmt found")
	}
	if selectsKeyByComparison(rng) {
		t.Fatalf("non-ordering == must not select key")
	}
}
