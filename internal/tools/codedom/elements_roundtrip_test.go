package codedom

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Go: function and method sharing a base name must list as distinct names.
func TestElementsFromSource_GoFuncVsMethodDistinct(t *testing.T) {
	t.Parallel()
	src := "package test\n\ntype A struct{}\n\nfunc Close() error {\n\treturn nil\n}\n\nfunc (a *A) Close() error {\n\treturn nil\n}\n"
	els := ElementsFromSource("test.go", src)
	names := map[string]string{}
	for _, e := range els {
		names[e.Name] = e.Type
	}
	if names["Close"] != "function" {
		t.Errorf("expected Close function, got %v (%v)", names["Close"], els)
	}
	if names["A.Close"] != "method" {
		t.Errorf("expected A.Close method, got %v (%v)", names["A.Close"], els)
	}
}

// Go value-receiver variant from the report.
func TestElementsFromSource_GoValueReceiverDistinct(t *testing.T) {
	t.Parallel()
	src := "package test\n\ntype A struct{}\n\nfunc Size() int {\n\treturn 0\n}\n\nfunc (a A) Size() int {\n\treturn 0\n}\n"
	els := ElementsFromSource("test.go", src)
	names := map[string]string{}
	for _, e := range els {
		names[e.Name] = e.Type
	}
	if names["Size"] != "function" {
		t.Errorf("expected Size function, got %v (%v)", names["Size"], els)
	}
	if names["A.Size"] != "method" {
		t.Errorf("expected A.Size method, got %v (%v)", names["A.Size"], els)
	}
}

// Python: method inside a class must be qualified so it is distinct from a
// module-level function of the same name.
func TestElementsFromSource_PythonMethodQualified(t *testing.T) {
	t.Parallel()
	src := "class A:\n    def close(self):\n        pass\n\ndef close():\n    pass\n"
	els := ElementsFromSource("test.py", src)
	names := map[string]string{}
	for _, e := range els {
		names[e.Name] = e.Type
	}
	if names["A.close"] != "method" {
		t.Errorf("expected A.close method, got %v (%v)", names["A.close"], els)
	}
	if names["close"] != "function" {
		t.Errorf("expected close function, got %v (%v)", names["close"], els)
	}
	if len(els) != 3 && len(els) < 2 {
		t.Errorf("expected at least method+function, got %v", els)
	}
}

// Python: a nested def inside a plain function is not a class method, so it
// must stay unqualified. Pins the isClass branch of the enclosing check.
func TestElementsFromSource_PythonNestedDefUnqualified(t *testing.T) {
	t.Parallel()
	src := "def outer():\n    x = 1\n    def inner():\n        pass\n"
	els := ElementsFromSource("test.py", src)
	for _, e := range els {
		if e.Name == "outer.inner" || e.Name == ".inner" {
			t.Errorf("nested def must stay unqualified, got %q (%v)", e.Name, els)
		}
	}
	found := false
	for _, e := range els {
		if e.Name == "inner" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unqualified inner, got %v", els)
	}
}

// JavaScript: class method must be qualified so it is distinct from a
// same-named top-level function.
func TestElementsFromSource_JSMethodQualified(t *testing.T) {
	t.Parallel()
	src := "function close() {\n    return 1;\n}\n\nclass A {\n    close() {\n        return 2;\n    }\n}\n"
	els := ElementsFromSource("test.js", src)
	names := map[string]string{}
	for _, e := range els {
		names[e.Name] = e.Type
	}
	if names["close"] != "function" {
		t.Errorf("expected close function, got %v (%v)", names["close"], els)
	}
	if names["A.close"] != "method" {
		t.Errorf("expected A.close method, got %v (%v)", names["A.close"], els)
	}
}

// JavaScript: a second class must not inherit the first class scope. Pins the
// brace-stack pop branch: without popping, the second method keeps the stale
// qualifier.
func TestElementsFromSource_JSTwoClassesScoped(t *testing.T) {
	t.Parallel()
	src := "class A {\n    close() {\n        return 1;\n    }\n}\n\nclass B {\n    close() {\n        return 2;\n    }\n}\n"
	els := ElementsFromSource("test.js", src)
	names := map[string]bool{}
	for _, e := range els {
		names[e.Name] = true
	}
	if !names["A.close"] {
		t.Errorf("expected A.close, got %v", els)
	}
	if !names["B.close"] {
		t.Errorf("expected B.close, got %v", els)
	}
}

func writeRoundTripFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

// Go round trip: every name get_elements lists must fetch exactly that element.
func TestGetElement_GoFuncVsMethodRoundTrip(t *testing.T) {
	t.Parallel()
	path := writeRoundTripFile(t, "round.go",
		"package test\n\ntype A struct{}\n\nfunc Close() error {\n\treturn nil\n}\n\nfunc (a *A) Close() error {\n\treturn nil\n}\n")
	fnRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Close"})
	if err != nil {
		t.Fatalf("get_element Close failed: %v", err)
	}
	if !strings.Contains(fnRes, "func Close()") {
		t.Errorf("Close should fetch the plain function, got: %s", fnRes)
	}
	if strings.Contains(fnRes, "(a *A)") {
		t.Errorf("Close fetched the method instead of the function: %s", fnRes)
	}
	mRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "A.Close"})
	if err != nil {
		t.Fatalf("get_element A.Close failed: %v", err)
	}
	if !strings.Contains(mRes, "func (a *A) Close()") {
		t.Errorf("A.Close should fetch the method, got: %s", mRes)
	}
}

// Go value-receiver round trip.
func TestGetElement_GoValueReceiverRoundTrip(t *testing.T) {
	t.Parallel()
	path := writeRoundTripFile(t, "size.go",
		"package test\n\ntype A struct{}\n\nfunc Size() int {\n\treturn 0\n}\n\nfunc (a A) Size() int {\n\treturn 1\n}\n")
	fnRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Size"})
	if err != nil {
		t.Fatalf("get_element Size failed: %v", err)
	}
	if !strings.Contains(fnRes, "func Size()") {
		t.Errorf("Size should fetch the plain function, got: %s", fnRes)
	}
	mRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "A.Size"})
	if err != nil {
		t.Fatalf("get_element A.Size failed: %v", err)
	}
	if !strings.Contains(mRes, "func (a A) Size()") {
		t.Errorf("A.Size should fetch the method, got: %s", mRes)
	}
}

// Python round trip: both spellings must fetch, and the bare name must not be
// an ambiguous pair of identical candidates.
func TestGetElement_PythonQualifiedRoundTrip(t *testing.T) {
	t.Parallel()
	path := writeRoundTripFile(t, "round.py",
		"class A:\n    def close(self):\n        return 1\n\ndef close():\n    return 2\n")
	fnRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "close"})
	if err != nil {
		t.Fatalf("get_element close failed: %v", err)
	}
	if !strings.Contains(fnRes, "def close()") {
		t.Errorf("close should fetch the module function, got: %s", fnRes)
	}
	mRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "A.close"})
	if err != nil {
		t.Fatalf("get_element A.close failed: %v", err)
	}
	if !strings.Contains(mRes, "def close(self)") {
		t.Errorf("A.close should fetch the method, got: %s", mRes)
	}
}

// JavaScript round trip.
func TestGetElement_JSQualifiedRoundTrip(t *testing.T) {
	t.Parallel()
	path := writeRoundTripFile(t, "round.js",
		"function close() {\n    return 1;\n}\n\nclass A {\n    close() {\n        return 2;\n    }\n}\n")
	fnRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "close"})
	if err != nil {
		t.Fatalf("get_element close failed: %v", err)
	}
	if !strings.Contains(fnRes, "function close()") {
		t.Errorf("close should fetch the function, got: %s", fnRes)
	}
	mRes, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "A.close"})
	if err != nil {
		t.Fatalf("get_element A.close failed: %v", err)
	}
	if !strings.Contains(mRes, "close()") {
		t.Errorf("A.close should fetch the method, got: %s", mRes)
	}
}

// Ambiguous bare names must list candidates that each fetch exactly one
// element. Uses two methods and no plain function so the bare name is
// genuinely ambiguous.
func TestGetElement_AmbiguousCandidatesEachFetch(t *testing.T) {
	t.Parallel()
	path := writeRoundTripFile(t, "amb.go",
		"package test\n\ntype A struct{}\n\ntype B struct{}\n\nfunc (a *A) Close() error {\n\treturn nil\n}\n\nfunc (b *B) Close() error {\n\treturn nil\n}\n")
	_, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Close"})
	if err == nil {
		t.Fatal("expected ambiguous error for bare Close")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got: %v", err)
	}
	for _, cand := range []string{"A.Close", "B.Close"} {
		if !strings.Contains(err.Error(), cand) {
			t.Errorf("ambiguous error should list %s, got: %v", cand, err)
		}
		res, ferr := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": cand})
		if ferr != nil {
			t.Errorf("candidate %q listed in ambiguous error does not fetch: %v", cand, ferr)
			continue
		}
		if !strings.Contains(res, "Close()") {
			t.Errorf("candidate %q fetched body without Close: %s", cand, res)
		}
	}
}

// Unknown names still report not found (pins the no-match branch).
func TestGetElement_RoundTripNotFound(t *testing.T) {
	t.Parallel()
	path := writeRoundTripFile(t, "nf.go", "package test\n\nfunc Close() error {\n\treturn nil\n}\n")
	_, err := executeGetElement(wsCtxFor(t, path), map[string]any{"path": path, "name": "Nope"})
	if err == nil {
		t.Fatal("expected not found for Nope")
	}
	if !strings.Contains(err.Error(), "element not found") {
		t.Errorf("expected element not found, got: %v", err)
	}
}
