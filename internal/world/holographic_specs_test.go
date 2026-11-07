package world

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

const specLongGuide = "# Guide\n\n## Evaluation\n" +
	"The kernel is `internal/core/foo.go:Kernel` and `Func` in `internal/core/foo.go`. " +
	"Evaluation derives the fixpoint from the extensional facts, stratum by stratum, and never reads a clock; " +
	"the caller pins time. A retraction re-derives only the cone it touched, and the result must be identical " +
	"to a full evaluation of the same facts.\n\n" +
	"## Stale\nThis cites `internal/core/foo.go:Removed`.\n"

// specKernel scans a workspace holding the Go file, the guide and a spec file
// into a real kernel, as a boot would, and returns the kernel and the root.
// Every link is confirmed by that first scan; Func's body is then edited and the
// workspace scanned again, so the link to Func is the one that has drifted.
func specKernel(t *testing.T, guide string) (*core.RealKernel, string) {
	t.Helper()
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	recent := time.Now().Add(-time.Hour).Truncate(time.Second)
	root := specWorkspace(t, map[string]string{
		"go.mod":                    "module example.com/ws\n\ngo 1.26\n",
		"internal/core/foo.go":      specGoFile,
		"internal/core/foo_test.go": "package core\n\nimport \"testing\"\n\nfunc TestEval(t *testing.T) { var k Kernel; k.Eval(); Func() }\n",
		"Docs/guide.md":             guide,
		"Docs/specs/kernel.mg": `
spec_requirement(/unit, "Every exported function is tested").
requires(/unit, "internal/core.Unspecced", /has_test).
`,
	}, old, recent)
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner()
	if _, err := scanner.Refresh(context.Background(), root, nil, kernel, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	writeSpecFile(t, root, "internal/core/foo.go", strings.Replace(specGoFile, "func Func() {}", "func Func() { helper() }", 1), old, recent)
	if _, err := scanner.Refresh(context.Background(), root, nil, kernel, IncrementalOptions{}); err != nil {
		t.Fatal(err)
	}
	return kernel, root
}

// The spec's excerpt sits beside the code, with what the kernel derived about it.
func TestHolographicContextShowsTheSpecBesideTheCode(t *testing.T) {
	kernel, root := specKernel(t, specLongGuide)
	section := NewHolographicProvider(kernel, root).PromptSection(context.Background(), filepath.Join(root, "internal/core/foo.go"))

	for _, want := range []string{
		"### Specs for foo.go",
		// the section that specifies Eval (its doc comment names it) and its prose
		"`internal/core.Kernel.Eval`: `Docs/guide.md:Guide > Evaluation` (specifies)",
		"Evaluation derives the fixpoint from the extensional facts",
		// the section that cites the type
		"`internal/core.Kernel`: `Docs/guide.md:Guide > Evaluation` (cites)",
		// the kernel's judgments: Func changed since its link was confirmed and the
		// section did not, with how to settle it; a citation names a removed element,
		// an exported function no section reaches, a requirement unmet
		"`internal/core.Func`: `Docs/guide.md:Guide > Evaluation` (cites) DRIFT: the code changed since this link was confirmed",
		"first seen ",
		"no commit since; the change is uncommitted or git history is not loaded",
		"`confirm_spec_link spec_ref=\"Docs/guide.md:Guide > Evaluation\" code_ref=\"internal/core.Func\"`",
		"`internal/core/foo.go:Removed`: `Docs/guide.md:Guide > Stale` (cites) DANGLING: the cited element no longer exists",
		"No section specifies:",
		"`internal/core.Unspecced`",
		"REQUIREMENT unit violated: `internal/core.Unspecced` lacks has_test (Every exported function is tested)",
		"Specs reach ",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("holographic context lacks %q:\n%s", want, section)
		}
	}
}

// The unedited neighbours of a changed function are not reported as drifted.
func TestHolographicContextReportsDriftOnlyOnTheLinkWhoseCodeChanged(t *testing.T) {
	kernel, root := specKernel(t, specLongGuide)
	section := NewHolographicProvider(kernel, root).PromptSection(context.Background(), filepath.Join(root, "internal/core/foo.go"))
	if got := strings.Count(section, "DRIFT"); got != 1 {
		t.Fatalf("only the link to Func drifted, got %d DRIFT lines:\n%s", got, section)
	}
	for _, neighbour := range []string{"`internal/core.Kernel.Eval`: `Docs/guide.md:Guide > Evaluation` (specifies)\n", "`internal/core.Kernel`: `Docs/guide.md:Guide > Evaluation` (cites)\n"} {
		if !strings.Contains(section, neighbour) {
			t.Errorf("the neighbour line must carry no flag: %q in\n%s", neighbour, section)
		}
	}
}

func TestHolographicContextForASectionNamesTheCodeItLinks(t *testing.T) {
	kernel, root := specKernel(t, specLongGuide)
	section := NewHolographicProvider(kernel, root).PromptSection(context.Background(), filepath.Join(root, "Docs/guide.md"))
	for _, want := range []string{
		"### Code the sections of guide.md name",
		"`Docs/guide.md:Guide > Evaluation`: `internal/core.Kernel` (cites)",
		"`Docs/guide.md:Guide > Evaluation`: `internal/core.Kernel.Eval` (specifies)",
		"`Docs/guide.md:Guide > Stale`: `internal/core/foo.go:Removed` (cites) DANGLING: the cited element no longer exists",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("section context lacks %q:\n%s", want, section)
		}
	}
}

// With a working policy the block is bounded, and says what it left out: the
// entries it did not list and the characters of an excerpt it cut.
func TestHolographicSpecsAreBoundedByPolicyAndSayWhatWasCut(t *testing.T) {
	kernel, root := specKernel(t, specLongGuide)
	target := filepath.Join(root, "internal/core/foo.go")

	// A generous share lists every link; the excerpt is cut to the per-entry
	// allowance (the share divided by the entries kept), and the line says how
	// many characters it left out.
	section, seen := budgetedRender(t, NewHolographicProvider(kernel, root), target, func(c *config.WorkingConfig) {
		c.HolographicSpecsSharePercent = 100
		c.HolographicSpecExcerptFloor = 10
	}, 1200)
	pool := dimensionMeasurement(t, seen, holoSpecs)
	if pool.total < 3 {
		t.Fatalf("specs pool = %+v", pool)
	}
	excerpt := dimensionMeasurement(t, seen, holoSpecExcerpt)
	if excerpt.total < 1 {
		t.Fatalf("excerpt allowance not asked: %+v", seen)
	}
	if !strings.Contains(section, "more characters; `get_element` on the section returns it whole") {
		t.Fatalf("a cut excerpt must say how much was cut:\n%s", section)
	}
	if strings.Contains(section, "identical to a full evaluation") {
		t.Fatalf("the excerpt was not cut:\n%s", section)
	}
	if got := strings.Count(section, "Evaluation derives the fixpoint"); got != 1 {
		t.Fatalf("a section that specifies several elements is excerpted once, got %d:\n%s", got, section)
	}

	// A one-percent share holds no entry of a small budget: nothing is listed
	// and the remainder line says so.
	section, _ = budgetedRender(t, NewHolographicProvider(kernel, root), target, func(c *config.WorkingConfig) {
		c.HolographicSpecsSharePercent = 1
	}, 400)
	if !strings.Contains(section, "more links not listed") {
		t.Fatalf("the entries left out must be counted:\n%s", section)
	}
	if strings.Contains(section, "(specifies)") {
		t.Fatalf("an entry was listed past the policy's count:\n%s", section)
	}
}

func TestHolographicContextWithoutSpecsHasNoSpecBlock(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	dir, path := writeFuncs(t, 2)
	section := NewHolographicProvider(kernel, dir).PromptSection(context.Background(), path)
	if strings.Contains(section, "### Specs") {
		t.Fatalf("no link, no block:\n%s", section)
	}
}
