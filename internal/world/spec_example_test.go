package world

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/specs"
)

// The repository's own machine-checkable specs (Docs/specs/*.mg) are loaded
// the way a scan loads them, checked against this repository's code model, and
// derive no requirement_violated. A refactor of a subsystem that a spec states
// obligations for fails here, naming the requirement and the element.
func TestRepositorySpecsAreGreen(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var m specMeasurer
	files, warnings := m.readSpecFiles(root)
	if len(warnings) != 0 {
		t.Fatalf("a spec file in the repository is refused: %v", warnings)
	}
	var dred *specs.SpecFile
	var targets []string
	for _, f := range files {
		if strings.HasSuffix(f.Path, "dred_evaluator.mg") {
			dred = f
		}
		for _, q := range f.Requires {
			targets = append(targets, q.CodeRef)
		}
		for _, q := range f.Calls {
			targets = append(targets, q.CodeRef)
		}
	}
	if dred == nil || len(dred.Requirements) != 3 {
		t.Fatalf("Docs/specs/dred_evaluator.mg did not load with its three requirements: %+v", dred)
	}

	elems, indexed, measured, _, err := SharedStructureIndex(root).SpecSnapshot(context.Background(), targets)
	if err != nil {
		t.Fatal(err)
	}
	res := specs.Resolve(specs.Input{
		Code:  specs.NewCodeIndex(elems, indexed),
		Specs: files,
		Measure: func(e specs.Element) specs.Measurement {
			return measured[e.Ref]
		},
	})

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := kernel.LoadFacts(res.Facts()); err != nil {
		t.Fatal(err)
	}
	violated, err := kernel.Query("requirement_violated")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range violated {
		t.Errorf("requirement violated: %v", f.Args)
	}
	if reqs, _ := kernel.Query("spec_requirement"); len(reqs) < 3 {
		t.Fatalf("the kernel holds %d requirements, want the example's 3", len(reqs))
	}
}
