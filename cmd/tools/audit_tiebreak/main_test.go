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
		{
			name: "key used only to index or transform a value, not recorded as the winner",
			body: `for i, v := range s { if v > max { max = v; best = other[i] } }`,
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
