package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/world"
)

// TestGatherHolographicContext_ReachesTheReport is the wiring test for a field
// that was written and never read.
//
// Every one of the IntelligenceGatherer's six construction sites has passed it
// a *world.HolographicProvider since it was written, and the only three
// occurrences of the field were its declaration, the constructor parameter and
// the assignment. The campaign's most decision-relevant context — what the
// target file offers, what its package holds, who calls into it — was gathered
// nowhere.
func TestGatherHolographicContext_ReachesTheReport(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	src := "package p\n\n// Exported does the thing.\nfunc Exported() error { return nil }\n\ntype Thing struct{ A int }\n"
	if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	g := &IntelligenceGatherer{holographic: world.NewHolographicProvider(nil, dir)}
	report := &IntelligenceReport{}
	var errs []string

	g.gatherHolographicContext(context.Background(), report, []string{target}, func(e string) { errs = append(errs, e) })

	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(report.HolographicSections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(report.HolographicSections))
	}
	if report.HolographicSections[0].Path != target {
		t.Errorf("section lost its path: %q", report.HolographicSections[0].Path)
	}
	if !strings.Contains(report.HolographicSections[0].Section, "Exported") {
		t.Errorf("section does not describe the target:\n%s", report.HolographicSections[0].Section)
	}

	// And it must reach the prompt, not just the struct.
	formatted := report.FormatForContext()
	if !strings.Contains(formatted, "## Target Architecture") {
		t.Fatalf("holographic context is gathered but never rendered:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Exported") {
		t.Errorf("rendered report omits the target's surface:\n%s", formatted)
	}
}

// TestGatherHolographicContext_BoundsTargets keeps the planning prompt bounded:
// each section is 1-2 KB and a campaign can name many paths.
func TestGatherHolographicContext_BoundsTargets(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := 0; i < maxHolographicTargets*3; i++ {
		name := filepath.Join(dir, "f"+string(rune('a'+i))+".go")
		src := "package p\n\nfunc F" + string(rune('A'+i)) + "() {}\n"
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, name)
	}

	g := &IntelligenceGatherer{holographic: world.NewHolographicProvider(nil, dir)}
	report := &IntelligenceReport{}
	g.gatherHolographicContext(context.Background(), report, paths, func(string) {})

	if len(report.HolographicSections) > maxHolographicTargets {
		t.Fatalf("gathered %d sections, want at most %d", len(report.HolographicSections), maxHolographicTargets)
	}
}

// TestGatherHolographicContext_Degrades covers the ordinary absences: no
// provider, no targets, and a target the provider cannot describe. None of
// these is an error worth showing an operator.
func TestGatherHolographicContext_Degrades(t *testing.T) {
	report := &IntelligenceReport{}
	var errs []string
	addErr := func(e string) { errs = append(errs, e) }

	(&IntelligenceGatherer{}).gatherHolographicContext(context.Background(), report, []string{"x.go"}, addErr)
	if len(report.HolographicSections) != 0 || len(errs) != 0 {
		t.Fatalf("nil provider should be silent, got %d sections / %v", len(report.HolographicSections), errs)
	}

	dir := t.TempDir()
	g := &IntelligenceGatherer{holographic: world.NewHolographicProvider(nil, dir)}
	g.gatherHolographicContext(context.Background(), report, []string{filepath.Join(dir, "missing.go")}, addErr)
	if len(report.HolographicSections) != 0 {
		t.Fatalf("a missing target should yield no section, got %+v", report.HolographicSections)
	}
	if len(errs) != 0 {
		t.Fatalf("a missing target is not an operator-facing error: %v", errs)
	}
}

// TestGatherHolographicContext_Cancellation: a cancelled campaign must report
// why it stopped rather than silently returning a partial architecture.
func TestGatherHolographicContext_Cancellation(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a.go")
	if err := os.WriteFile(target, []byte("package p\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := &IntelligenceGatherer{holographic: world.NewHolographicProvider(nil, dir)}
	report := &IntelligenceReport{}
	var errs []string
	g.gatherHolographicContext(ctx, report, []string{target}, func(e string) { errs = append(errs, e) })

	if len(errs) == 0 {
		t.Fatal("cancellation must be reported")
	}
	if !strings.Contains(errs[0], "cancelled") {
		t.Errorf("unexpected error text: %q", errs[0])
	}
}
