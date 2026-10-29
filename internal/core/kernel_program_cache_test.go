package core

import "testing"

// Boot constructs many kernels from the same embedded program. The analyzed
// program is immutable (kernel clones already share it), so the second kernel
// must reuse the first kernel's ProgramInfo instead of parsing and analyzing
// the identical bytes again — and still answer queries independently.
func TestRebuildProgramCacheSharesAnalyzedProgram(t *testing.T) {
	first, err := NewRealKernel()
	if err != nil {
		t.Fatalf("first NewRealKernel: %v", err)
	}
	second, err := NewRealKernel()
	if err != nil {
		t.Fatalf("second NewRealKernel: %v", err)
	}
	if first.programInfo == nil || second.programInfo == nil {
		t.Fatal("kernels booted without programInfo")
	}
	if first.programInfo != second.programInfo {
		t.Fatal("identical programs did not share the analyzed ProgramInfo")
	}
	if first.cone != second.cone {
		t.Fatal("identical programs did not share the cone index")
	}

	if err := second.Assert(Fact{Predicate: "config_param", Args: []any{MangleAtom("/zz_cache_probe"), int64(1)}}); err != nil {
		t.Fatalf("assert on cache-sharing kernel: %v", err)
	}
	rows, err := second.Query("config_param")
	if err != nil {
		t.Fatalf("query on cache-sharing kernel: %v", err)
	}
	found := false
	for _, row := range rows {
		if len(row.Args) > 0 && row.Args[0] == "/zz_cache_probe" {
			found = true
		}
	}
	if !found {
		t.Fatal("cache-sharing kernel did not return its own asserted row")
	}
	firstRows, err := first.Query("config_param")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range firstRows {
		if len(row.Args) > 0 && row.Args[0] == "/zz_cache_probe" {
			t.Fatal("fact asserted into one kernel leaked into another sharing its program")
		}
	}
}
