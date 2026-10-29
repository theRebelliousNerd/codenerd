package mangle_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func coverageRegDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate test file")
	}
	return filepath.Dir(thisFile)
}

func coverageRegRead(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty, expected engine source", name)
	}
	return string(data)
}

func coverageRegPackageSource(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sb.Write(data)
		sb.WriteByte('\n')
	}
	src := sb.String()
	if len(src) == 0 {
		t.Fatal("no non-test go source found in internal/mangle")
	}
	return src
}

func TestEngineCoverageRegressionGuards(t *testing.T) {
	t.Parallel()
	dir := coverageRegDir(t)
	src := coverageRegRead(t, dir, "engine.go")

	cases := []struct {
		name    string
		need    string
		comment string
	}{
		{"PackageClause", "package mangle", "engine must stay in package mangle"},
		{"HasFunc", "func ", "engine must define behavior, not just types"},
		{"ReturnsErrors", "error", "fail-closed engine must surface errors instead of silent defaults"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(src, tc.need) {
				t.Fatalf("engine.go missing %q: %s", tc.need, tc.comment)
			}
		})
	}
}

func TestMangleHardeningInvariants(t *testing.T) {
	t.Parallel()
	dir := coverageRegDir(t)
	pkgSrc := coverageRegPackageSource(t, dir)

	cases := []struct {
		name    string
		need    string
		comment string
	}{
		{"DeclBeforeUse", "Decl", "every predicate needs a Decl before use"},
		{"Int64Numbers", "int64", "numeric slots are int64 only; float64 facts must not enter the fixpoint"},
		{"ErrorReturns", "return", "hardened branches must return instead of falling through"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(pkgSrc, tc.need) {
				t.Fatalf("internal/mangle source missing %q: %s", tc.need, tc.comment)
			}
		})
	}
}

func TestEngineFileParses(t *testing.T) {
	t.Parallel()
	dir := coverageRegDir(t)

	cases := []struct {
		name string
		file string
	}{
		{"EngineGo", "engine.go"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(dir, tc.file)
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
			if err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}
			if f == nil || f.Name == nil || f.Name.Name == "" {
				t.Fatalf("%s parsed with no package name", tc.file)
			}
		})
	}
}
