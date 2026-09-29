package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
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

// TestDirectImporters_Bounded keeps the section to the share. Twelve
// fixed-width importers, so every rendered line is the same length. 10% of
// (lineLen*40) bytes is lineLen*4, so four render and eight remain.
// importers_of reads the rest.
func TestDirectImporters_Bounded(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)

	const n = 12
	facts := make([]core.Fact, 0, n)
	const pkgPath = "pkg:example.com/m/internal/lib"
	var lineLen int
	for i := 0; i < n; i++ {
		p := filepath.ToSlash(filepath.Join(dir, "cmd", fmt.Sprintf("c%02d", i), "main.go"))
		facts = append(facts, core.Fact{Predicate: "dependency_link", Args: []any{p, pkgPath, "example.com/m/internal/lib"}})
		if lineLen == 0 {
			lineLen = len("- `" + p + "`\n")
		}
	}
	h := NewHolographicProvider(&stubQuerier{facts: map[string][]core.Fact{
		"dependency_link": facts,
	}}, dir)
	// share 10: budget/10 is the allowance, and lineLen*4 of it renders 4.
	budget := lineLen * 40
	section, seen := budgetedRender(t, h, target, func(c *config.WorkingConfig) {
		c.HolographicImportersSharePercent = 10
	}, budget)
	m := dimensionMeasurement(t, seen, holoImporters)
	if m.total != n || m.avg != lineLen || m.budget != budget {
		t.Fatalf("measured importers = (%d, %d bytes, budget %d), want (%d, %d, %d)", m.total, m.avg, m.budget, n, lineLen, budget)
	}
	if got := strings.Count(section, "- `"+filepath.ToSlash(filepath.Join(dir, "cmd"))); got != 4 {
		t.Fatalf("rendered %d importers, want the derived 4:\n%s", got, section)
	}
	if !strings.Contains(section, "/c00/") || !strings.Contains(section, "/c03/") {
		t.Fatalf("the first four sorted importers must render:\n%s", section)
	}
	if strings.Contains(section, "/c04/") {
		t.Fatalf("c04 renders past the derived count:\n%s", section)
	}
	if !strings.Contains(section, "and 8 more file(s)") || !strings.Contains(section, "`importers_of`") {
		t.Fatalf("missing the true remainder and importers_of:\n%s", section)
	}
}

// Two importers under an allowance that holds them render whole, with no
// remainder line.
func TestDirectImporters_FewRenderWhole(t *testing.T) {
	dir := t.TempDir()
	target := writeModule(t, dir)

	const pkgPath = "pkg:example.com/m/internal/lib"
	facts := make([]core.Fact, 0, 2)
	var lineLen int
	for i := 0; i < 2; i++ {
		p := filepath.ToSlash(filepath.Join(dir, "cmd", fmt.Sprintf("c%02d", i), "main.go"))
		facts = append(facts, core.Fact{Predicate: "dependency_link", Args: []any{p, pkgPath, "example.com/m/internal/lib"}})
		lineLen = len("- `" + p + "`\n")
	}
	h := NewHolographicProvider(&stubQuerier{facts: map[string][]core.Fact{
		"dependency_link": facts,
	}}, dir)
	budget := lineLen * 40
	section, seen := budgetedRender(t, h, target, func(c *config.WorkingConfig) {
		c.HolographicImportersSharePercent = 10
	}, budget)
	m := dimensionMeasurement(t, seen, holoImporters)
	if m.total != 2 {
		t.Fatalf("measured importers = %d, want 2", m.total)
	}
	if got := strings.Count(section, "- `"+filepath.ToSlash(filepath.Join(dir, "cmd"))); got != 2 {
		t.Fatalf("rendered %d importers, want both:\n%s", got, section)
	}
	if strings.Contains(section, "more file(s)") {
		t.Fatalf("a pool that fits states a remainder:\n%s", section)
	}
}
