package campaign

import (
	"context"
	"fmt"
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
	formatted := formatIntelligenceContext(report)
	if !strings.Contains(formatted, "## Target Architecture") {
		t.Fatalf("holographic context is gathered but never rendered:\n%s", formatted)
	}
	if !strings.Contains(formatted, "Exported") {
		t.Errorf("rendered report omits the target's surface:\n%s", formatted)
	}
}

// TestGatherHolographicContext_RendersEveryTarget pins the old silent cap.
// Five used to be kept and the rest dropped with no name. Every target that
// the provider can describe is gathered, and the planning formatter shows each
// section whole.
func TestGatherHolographicContext_RendersEveryTarget(t *testing.T) {
	dir := t.TempDir()
	const n = 12 // the deleted cap was 5
	var paths []string
	markers := make([]string, n)
	for i := 0; i < n; i++ {
		markers[i] = fmt.Sprintf("Marker%02d", i)
		name := filepath.Join(dir, fmt.Sprintf("f%02d.go", i))
		src := fmt.Sprintf("package p\n\n// %s does the thing.\nfunc %s() error { return nil }\n", markers[i], markers[i])
		if err := os.WriteFile(name, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, name)
	}

	g := &IntelligenceGatherer{holographic: world.NewHolographicProvider(nil, dir)}
	report := &IntelligenceReport{}
	g.gatherHolographicContext(context.Background(), report, paths, func(string) {})

	if len(report.HolographicSections) != n {
		t.Fatalf("gathered %d sections, want %d (nothing dropped)", len(report.HolographicSections), n)
	}
	if len(report.HolographicUnread) != 0 {
		t.Fatalf("complete gather left targets unread: %v", report.HolographicUnread)
	}
	formatted := formatIntelligenceContext(report)
	if strings.Contains(formatted, "### Not rendered") {
		t.Fatalf("complete gather named a withheld target:\n%s", formatted)
	}
	for i, hs := range report.HolographicSections {
		if hs.Path != paths[i] {
			t.Errorf("section %d path = %q, want %q", i, hs.Path, paths[i])
		}
		if !strings.Contains(hs.Section, markers[i]) {
			t.Errorf("section %d does not describe %s:\n%s", i, markers[i], hs.Section)
		}
		if !strings.Contains(formatted, hs.Section) {
			t.Errorf("formatted report cut the section for %s", hs.Path)
		}
	}
}

// TestFormatIntelligenceContext_HolographicSectionWhole pins the old 4096-character cut.
// The tail sits past that bound; a truncateField slice would drop it.
func TestFormatIntelligenceContext_HolographicSectionWhole(t *testing.T) {
	const oldCap = 4096
	body := strings.Repeat("architecture-line\n", (oldCap/len("architecture-line\n"))+40)
	body += "HOLO_TAIL_MARKER"
	if len(body) <= oldCap {
		t.Fatalf("fixture is %d bytes, want longer than the deleted cap %d", len(body), oldCap)
	}

	report := &IntelligenceReport{
		HolographicSections: []HolographicSection{{
			Path:    "big.go",
			Section: body,
		}},
	}
	formatted := formatIntelligenceContext(report)
	if !strings.Contains(formatted, body) {
		t.Fatalf("formatted report cut a %d-byte holographic section (deleted cap was %d)", len(body), oldCap)
	}
	if strings.Contains(formatted, "### Not rendered") {
		t.Fatalf("a rendered section was also listed as withheld:\n%s", formatted)
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
	missing := filepath.Join(dir, "missing.go")
	g.gatherHolographicContext(context.Background(), report, []string{missing}, addErr)
	if len(report.HolographicSections) != 0 {
		t.Fatalf("a missing target should yield no section, got %+v", report.HolographicSections)
	}
	if len(report.HolographicUnread) != 0 {
		t.Fatalf("a missing target was not withheld, it had nothing to render: %v", report.HolographicUnread)
	}
	if len(errs) != 0 {
		t.Fatalf("a missing target is not an operator-facing error: %v", errs)
	}
}

// TestGatherHolographicContext_Cancellation: a cancelled campaign must report
// why it stopped rather than silently returning a partial architecture. Every
// target it did not render is named, with the tools that read it.
func TestGatherHolographicContext_Cancellation(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, name := range []string{"a.go", "b.go"} {
		target := filepath.Join(dir, name)
		if err := os.WriteFile(target, []byte("package p\n\nfunc A() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, target)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	g := &IntelligenceGatherer{holographic: world.NewHolographicProvider(nil, dir)}
	report := &IntelligenceReport{}
	var errs []string
	g.gatherHolographicContext(ctx, report, paths, func(e string) { errs = append(errs, e) })

	if len(errs) == 0 {
		t.Fatal("cancellation must be reported")
	}
	if !strings.Contains(errs[0], "cancelled") {
		t.Errorf("unexpected error text: %q", errs[0])
	}
	if len(report.HolographicSections) != 0 {
		t.Fatalf("cancelled gather rendered %d sections", len(report.HolographicSections))
	}
	if len(report.HolographicUnread) != len(paths) {
		t.Fatalf("unread = %v, want %v", report.HolographicUnread, paths)
	}
	formatted := formatIntelligenceContext(report)
	for _, path := range paths {
		if !strings.Contains(errs[0], path) {
			t.Errorf("error does not name left-out target %s: %q", path, errs[0])
		}
		if !strings.Contains(formatted, path) {
			t.Errorf("report does not name left-out target %s:\n%s", path, formatted)
		}
	}
	if !strings.Contains(formatted, "package_outline") || !strings.Contains(formatted, "get_elements") {
		t.Errorf("left-out targets are not told how to be read:\n%s", formatted)
	}
}
