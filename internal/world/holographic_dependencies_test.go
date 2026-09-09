package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// writeModule lays out a two-package workspace with a go.mod, so the import
// path derivation has something real to resolve against.
func writeModule(t *testing.T, dir string) (target string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/m\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(dir, "internal", "lib")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	target = filepath.Join(pkgDir, "lib.go")
	src := "package lib\n\nimport (\n\t\"fmt\"\n\t\"github.com/pkg/errors\"\n)\n\n" +
		"// Exported is the symbol under change.\n" +
		"func Exported() error { _ = fmt.Sprint(); return errors.New(\"x\") }\n\n" +
		"// TODO: this marker is counted.\n// FIXME: so is this one.\n"
	if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return target
}

// TestDependencyDimensions_Populated pins three struct fields that were
// declared on the day HolographicContext was written and never assigned
// anywhere in the repo — the "X-Ray Vision" type advertising a dependency
// dimension it did not deliver.
func TestDependencyDimensions_Populated(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)

	h := NewHolographicProvider(nil, dir)
	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}

	if len(hc.DirectImports) != 2 {
		t.Fatalf("DirectImports = %+v, want fmt and github.com/pkg/errors", hc.DirectImports)
	}
	if len(hc.ExternalDeps) != 1 || hc.ExternalDeps[0] != "github.com/pkg/errors" {
		t.Errorf("ExternalDeps = %v, want only the domain-qualified import", hc.ExternalDeps)
	}
	if hc.TODOCount != 2 {
		t.Errorf("TODOCount = %d, want 2 (CountTODOs had zero callers before this)", hc.TODOCount)
	}
}

// TestIsExternalImport pins the heuristic: a first segment containing a dot is
// a domain, so it came from outside this module and outside the standard
// library.
func TestIsExternalImport(t *testing.T) {
	cases := map[string]bool{
		"fmt":                        false,
		"strings":                    false,
		"internal/world":             false,
		"codenerd/internal/core":     false,
		"github.com/pkg/errors":      true,
		"go.uber.org/zap":            true,
		"codeberg.org/TauCeti/x":     true,
		"golang.org/x/sync/errgroup": true,
	}
	for path, want := range cases {
		if got := isExternalImport(path); got != want {
			t.Errorf("isExternalImport(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestPackageImportPath derives the module-qualified path the scanner records
// as a dependency_link CalleeID.
func TestPackageImportPath(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)
	h := NewHolographicProvider(nil, dir)

	if got := h.packageImportPath(target); got != "example.com/m/internal/lib" {
		t.Errorf("packageImportPath = %q, want example.com/m/internal/lib", got)
	}
	// A file in the module root is the module itself.
	root := filepath.Join(dir, "main.go")
	if err := os.WriteFile(root, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := h.packageImportPath(root); got != "example.com/m" {
		t.Errorf("root package path = %q, want example.com/m", got)
	}
	// A path outside the workspace is not nameable by this module.
	if got := h.packageImportPath(filepath.Join(t.TempDir(), "x.go")); got != "" {
		t.Errorf("out-of-workspace path resolved to %q, want empty", got)
	}
}

// TestDirectImporters_FromKernel is the reverse-dependency dimension: which
// files depend on this package. It is the one thing in the holographic context
// a model cannot get by reading the file, and it was never populated.
func TestDirectImporters_FromKernel(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	pkgPath := "pkg:example.com/m/internal/lib"
	facts := []core.Fact{
		// Two real importers, outside the target's package.
		{Predicate: "dependency_link", Args: []any{filepath.Join(dir, "cmd", "app", "main.go"), pkgPath, "example.com/m/internal/lib"}},
		{Predicate: "dependency_link", Args: []any{filepath.Join(dir, "internal", "other", "o.go"), pkgPath, "example.com/m/internal/lib"}},
		// A sibling in the same package is not an importer.
		{Predicate: "dependency_link", Args: []any{filepath.Join(dir, "internal", "lib", "sibling.go"), pkgPath, "example.com/m/internal/lib"}},
		// An edge to a different package must not appear.
		{Predicate: "dependency_link", Args: []any{filepath.Join(dir, "cmd", "app", "main.go"), "pkg:fmt", "fmt"}},
	}
	for _, f := range facts {
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("assert: %v", err)
		}
	}

	h := NewHolographicProvider(kernel, dir)
	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}

	if len(hc.DirectImporters) != 2 {
		t.Fatalf("DirectImporters = %v, want the two out-of-package importers", hc.DirectImporters)
	}
	for _, importer := range hc.DirectImporters {
		if strings.Contains(importer, "/internal/lib/") {
			t.Errorf("a same-package sibling was counted as an importer: %q", importer)
		}
	}
	// Sorted, so the rendered section is byte-stable across turns.
	if hc.DirectImporters[0] > hc.DirectImporters[1] {
		t.Errorf("importers are not sorted: %v", hc.DirectImporters)
	}

	section := h.PromptSection(context.Background(), target)
	if !strings.Contains(section, "### Imported by") {
		t.Fatalf("reverse dependency is gathered but never rendered:\n%s", section)
	}
	if !strings.Contains(section, "main.go") {
		t.Errorf("rendered section omits an importer:\n%s", section)
	}
}

// TestDirectImporters_QuietWithoutKernel: an empty importer list and an
// unscanned workspace must not look the same to the model, so the section is
// omitted rather than rendered empty.
func TestDirectImporters_QuietWithoutKernel(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)

	h := NewHolographicProvider(nil, dir)
	hc, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if len(hc.DirectImporters) != 0 {
		t.Fatalf("no kernel must mean no importers, got %v", hc.DirectImporters)
	}
	if strings.Contains(h.PromptSection(context.Background(), target), "Imported by") {
		t.Error("an unscanned workspace must not render an empty dependents section")
	}
}

// TestDirectImporters_Bounded keeps the section short. The question it answers
// is "is this load-bearing, and for whom" — six examples and a total answer it
// as well as fifty, at a tenth of the tokens.
func TestDirectImporters_Bounded(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	pkgPath := "pkg:example.com/m/internal/lib"
	for i := 0; i < maxRenderedImporters*4; i++ {
		f := core.Fact{Predicate: "dependency_link", Args: []any{
			filepath.Join(dir, "cmd", "a"+string(rune('a'+i)), "main.go"), pkgPath, "example.com/m/internal/lib",
		}}
		if err := kernel.Assert(f); err != nil {
			t.Fatalf("assert: %v", err)
		}
	}

	h := NewHolographicProvider(kernel, dir)
	section := h.PromptSection(context.Background(), target)

	listed := strings.Count(section, "- `"+filepath.ToSlash(dir))
	if listed > maxRenderedImporters {
		t.Errorf("rendered %d importers, want at most %d:\n%s", listed, maxRenderedImporters, section)
	}
	if !strings.Contains(section, "more file(s)") {
		t.Errorf("truncation is silent:\n%s", section)
	}
}
