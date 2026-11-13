package experts

import (
	"context"
	"errors"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// stepKernel: a Go file that imports a Mangle library, a Mangle expert on the
// roster, and the Jev thresholds the kernel reads.
func stepKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	if err := k.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{AgentAtom("MangleExpert")}},
		{Predicate: "shard_agent_domain", Args: []any{AgentAtom("MangleExpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_prefix", Args: []any{"codeberg.org/TauCeti/mangle-go", types.MangleAtom("/mangle")}},
		{Predicate: "dependency_link", Args: []any{"internal/core/kernel_query.go", "pkg:codeberg.org/TauCeti/mangle-go/ast", "codeberg.org/TauCeti/mangle-go/ast"}},
		{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_yes_percent"), int64(70)}},
		{Predicate: "config_param", Args: []any{types.MangleAtom("/jev_no_percent"), int64(30)}},
	}); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	return k
}

type fakeStepReader struct {
	readings map[string]int64
	err      error
	asked    [][]string
}

func (r *fakeStepReader) ReadStep(_ context.Context, file, change string, techs map[string][]string) (map[string]int64, error) {
	asked := []string{file, change}
	for tech, imports := range techs {
		asked = append(asked, tech)
		asked = append(asked, imports...)
	}
	r.asked = append(r.asked, asked)
	return r.readings, r.err
}

func stepTask(reader StepReader) Task {
	return Task{
		Verb:    "/fix",
		Targets: []string{"internal/core/kernel_query.go"},
		Change:  "return early from Query when the predicate name is empty",
		Reader:  reader,
	}
}

// A planned step whose change Jev reads as not writing Mangle code consults
// no Mangle specialist, though its file imports a Mangle library; one read as
// writing it, one Jev could not read, and a whole turn (no change text) keep
// the import rule.
func TestSelect_StepReadingDecidesImportedTechnology(t *testing.T) {
	cases := []struct {
		name       string
		reader     *fakeStepReader
		change     bool
		wantMangle bool
	}{
		{"jev reads no", &fakeStepReader{readings: map[string]int64{"mangle": 8}}, true, false},
		{"jev reads yes", &fakeStepReader{readings: map[string]int64{"mangle": 91}}, true, true},
		{"jev unsettled", &fakeStepReader{readings: map[string]int64{"mangle": 55}}, true, true},
		{"jev failed", &fakeStepReader{err: errors.New("deadline exceeded")}, true, true},
		{"no reader", nil, true, true},
		{"whole turn", &fakeStepReader{readings: map[string]int64{"mangle": 8}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := stepKernel(t)
			var reader StepReader
			if tc.reader != nil {
				reader = tc.reader
			}
			task := stepTask(reader)
			if !tc.change {
				task.Change = ""
			}
			sel, err := SelectContext(t.Context(), k, task)
			if err != nil {
				t.Fatalf("Select: %v", err)
			}
			got := slices.Contains(sel.Agents(), "mangleexpert")
			if got != tc.wantMangle {
				t.Fatalf("mangleexpert needed = %v, want %v (needs %+v)", got, tc.wantMangle, sel.Needs)
			}
			if tc.reader != nil && tc.change {
				if len(tc.reader.asked) != 1 || !slices.Equal(tc.reader.asked[0][2:], []string{"mangle", "codeberg.org/TauCeti/mangle-go/ast"}) {
					t.Fatalf("reader asked %v, want the one candidate technology", tc.reader.asked)
				}
			}
			if rows, _ := k.Query("expert_step_reading"); len(rows) != 0 {
				t.Fatalf("the step's readings outlived the selection: %v", rows)
			}
		})
	}
}
