package codedom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitReceiverNameBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		line     string
		wantBase string
		wantOK   bool
	}{
		{"no paren", "no parens here", "", false},
		{"not func prefix", "foo(bar) baz", "", false},
		{"missing close", "func (abc", "", false},
		{"empty receiver", "func () Foo()", "", false},
		{"star only receiver", "func (*) Close()", "", false},
		{"pointer receiver", "func (b *B) Close() error", "B", true},
		{"value receiver", "func (b B) Close()", "B", true},
		{"generic receiver", "func (b Box[T]) Close()", "Box", true},
		{"package qualifier", "func (b pkg.B) Close()", "B", true},
		{"pointer package generic", "func (b *pkg.Box[T]) Close()", "Box", true},
		{"plain func", "func Foo() {}", "", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := splitReceiverName(tc.line)
			if got != tc.wantBase || ok != tc.wantOK {
				t.Errorf("splitReceiverName(%q) = (%q, %v), want (%q, %v)", tc.line, got, ok, tc.wantBase, tc.wantOK)
			}
		})
	}

	if got := goReceiverBase("func (b *B) Close() error"); got != "B" {
		t.Errorf("goReceiverBase = %q, want %q", got, "B")
	}
	if got := goReceiverBase("func Foo() {}"); got != "" {
		t.Errorf("goReceiverBase plain func = %q, want empty", got)
	}
}

func TestNormalizeReceiverBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"B", "B"},
		{"(B)", "B"},
		{"((B))", "B"},
		{"*B", "B"},
		{"*(B)", "B"},
		{"* (B)", "B"},
		{"(*B)", "B"},
		{"((*B))", "B"},
		{"pkg.B", "B"},
		{"*pkg.B", "B"},
		{"(*pkg.B)", "B"},
		{"Box[T]", "Box"},
		{"*Box[T]", "Box"},
		{"(Box[T])", "Box"},
		{"pkg.Box[T]", "Box"},
		{"pkg.(B)", "B"},
		{"", ""},
		{"*", ""},
		{"()", ""},
		{"( )", ""},
		{" * ( * pkg.Box[T] ) ", "Box"},
		{"  B  ", "B"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("norm/"+tc.in, func(t *testing.T) {
			t.Parallel()
			if got := normalizeReceiver(tc.in); got != tc.want {
				t.Errorf("normalizeReceiver(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseElementRefBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in            string
		wantRecv      string
		wantMethod    string
		wantQualified bool
	}{
		{"Close", "", "Close", false},
		{".Close", "", ".Close", false},
		{"B.", "", "B.", false},
		{".", "", ".", false},
		{"B.Close", "B", "Close", true},
		{"B.Close()", "B", "Close", true},
		{"B.Close(x int)", "B", "Close", true},
		{"B.()", "", "B.()", false},
		{"pkg.B.Close", "B", "Close", true},
		{"conns.B.Close", "B", "Close", true},
		{"*.Close", "", "*.Close", false},
		{"(*).Close", "", "(*).Close", false},
		{"(*B).Close", "B", "Close", true},
		{"*B.Close", "B", "Close", true},
		{" B . Close ", "B", "Close", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("ref/"+tc.in, func(t *testing.T) {
			t.Parallel()
			recv, method, qualified := parseElementRef(tc.in)
			if recv != tc.wantRecv || method != tc.wantMethod || qualified != tc.wantQualified {
				t.Errorf("parseElementRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, recv, method, qualified, tc.wantRecv, tc.wantMethod, tc.wantQualified)
			}
		})
	}
}

func TestSplitStoredNameBranches(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in           string
		wantRecv     string
		wantMeth     string
		wantMethFlag bool
	}{
		{"Close", "", "Close", false},
		{".Close", "", ".Close", false},
		{"B.", "", "B.", false},
		{"B.Close", "B", "Close", true},
		{"a.b.C", "a.b", "C", true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run("stored/"+tc.in, func(t *testing.T) {
			t.Parallel()
			recv, method, isMethod := splitStoredName(tc.in)
			if recv != tc.wantRecv || method != tc.wantMeth || isMethod != tc.wantMethFlag {
				t.Errorf("splitStoredName(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tc.in, recv, method, isMethod, tc.wantRecv, tc.wantMeth, tc.wantMethFlag)
			}
		})
	}
}

func writeConnsFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "conns.go")
	content := "package test\n\ntype A struct{}\n\nfunc (a *A) Close() error {\n\treturn nil\n}\n\ntype B struct{}\n\nfunc (b *B) Close() error {\n\treturn nil\n}\n\nfunc Foo() {}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write conns file: %v", err)
	}
	return path
}

func TestGetElementsListsReceiverQualifiedNames(t *testing.T) {
	t.Parallel()

	path := writeConnsFile(t)
	elements, err := extractCodeElements(path)
	if err != nil {
		t.Fatalf("extractCodeElements error: %v", err)
	}
	found := map[string]bool{}
	for _, e := range elements {
		found[e.Name] = true
	}
	if !found["A.Close"] {
		t.Errorf("expected A.Close in elements, got %v", elements)
	}
	if !found["B.Close"] {
		t.Errorf("expected B.Close in elements, got %v", elements)
	}

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read conns file: %v", err)
	}
	fromSrc := ElementsFromSource(path, string(src))
	srcNames := map[string]bool{}
	for _, e := range fromSrc {
		srcNames[e.Name] = true
	}
	if !srcNames["A.Close"] || !srcNames["B.Close"] {
		t.Errorf("ElementsFromSource missing qualified names: %v", fromSrc)
	}
}

func TestGetElementReceiverQualifiedFetches(t *testing.T) {
	t.Parallel()

	path := writeConnsFile(t)

	for _, name := range []string{"B.Close", "(*B).Close", "*B.Close", "conns.B.Close", "pkg.B.Close", "A.Close", "(*A).Close"} {
		result, err := executeGetElement(wsCtxFor(t, path), map[string]any{
			"path": path,
			"name": name,
		})
		if err != nil {
			t.Errorf("executeGetElement(%q) error: %v", name, err)
			continue
		}
		if !strings.Contains(result, "Close") {
			t.Errorf("executeGetElement(%q) missing Close: %s", name, result)
		}
	}

	bResult, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "B.Close"})
	if err != nil {
		t.Fatalf("B.Close fetch failed: %v", err)
	}
	if !strings.Contains(bResult, "func (b *B) Close()") {
		t.Errorf("B.Close should return B receiver, got: %s", bResult)
	}
	aResult, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "A.Close"})
	if err != nil {
		t.Fatalf("A.Close fetch failed: %v", err)
	}
	if !strings.Contains(aResult, "func (a *A) Close()") {
		t.Errorf("A.Close should return A receiver, got: %s", aResult)
	}
}

func TestGetElementBareNameIsAmbiguous(t *testing.T) {
	t.Parallel()

	path := writeConnsFile(t)
	_, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Close"})
	if err == nil {
		t.Fatal("expected ambiguous error for bare Close")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected ambiguous error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "A.Close") || !strings.Contains(err.Error(), "B.Close") {
		t.Errorf("ambiguous error should list both qualified names, got: %v", err)
	}
}

func TestGetElementQualifiedMismatchBranches(t *testing.T) {
	t.Parallel()

	path := writeConnsFile(t)

	// Wrong method under a real receiver: hits qualified method-mismatch continue.
	if _, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "B.Nope"}); err == nil {
		t.Error("expected not found for B.Nope")
	} else if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("B.Nope unexpected error: %v", err)
	}

	// Wrong receiver: hits receiver-mismatch plus signature-fallback continue.
	if _, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "C.Close"}); err == nil {
		t.Error("expected not found for C.Close")
	} else if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("C.Close unexpected error: %v", err)
	}

	// Qualified query naming a plain function: hits qualified skips non-method.
	if _, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "X.Foo"}); err == nil {
		t.Error("expected not found for X.Foo")
	} else if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("X.Foo unexpected error: %v", err)
	}

	// Bare name matching no method: hits bare method-mismatch continue.
	if _, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Nope"}); err == nil {
		t.Error("expected not found for bare Nope")
	} else if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("bare Nope unexpected error: %v", err)
	}
}

func TestGetElementBareSingleMethodMatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "single.go")
	content := "package test\n\ntype B struct{}\n\nfunc (b *B) Close() error {\n\treturn nil\n}\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write single file: %v", err)
	}
	result, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Close"})
	if err != nil {
		t.Fatalf("bare Close on single-method file failed: %v", err)
	}
	if !strings.Contains(result, "func (b *B) Close()") {
		t.Errorf("expected B.Close body, got: %s", result)
	}
}

func TestGetElementEveryListedNameFetches(t *testing.T) {
	t.Parallel()

	path := writeConnsFile(t)
	elements, err := extractCodeElements(path)
	if err != nil {
		t.Fatalf("extractCodeElements error: %v", err)
	}
	if len(elements) == 0 {
		t.Fatal("expected elements")
	}
	for _, e := range elements {
		result, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": e.Name})
		if err != nil {
			t.Errorf("get_element(%q) listed by get_elements failed: %v", e.Name, err)
			continue
		}
		if !strings.Contains(result, e.Name) && !strings.Contains(result, strings.TrimPrefix(e.Name, strings.Split(e.Name, ".")[0]+".")) {
			t.Errorf("get_element(%q) result missing name: %s", e.Name, result)
		}
	}
}
