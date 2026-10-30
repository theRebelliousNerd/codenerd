package core

import (
	"testing"

	"codenerd/internal/types"

	"github.com/stretchr/testify/require"
)

// The gatherer's measurements about the files a plan touches decide what the
// context compiler treats as relevant: a critical safety warning is /p95, a
// volatile file /p85. The rank is the kernel's, reached through context_relevant
// (policy/context_compilation.mg).
func TestIntelligenceWiring(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	require.NoError(t, err)

	topology := func(path string) Fact {
		return Fact{Predicate: "file_topology", Args: []any{path, "h", MangleAtom("/go"), int64(1), MangleAtom("/false")}}
	}
	artifact := func(task, path string) Fact {
		return Fact{Predicate: "task_artifact", Args: []any{task, MangleAtom("/source"), path, "h"}}
	}
	facts := []Fact{
		topology("critical_file.go"), topology("churn_file.go"), topology("quiet_file.go"),
		artifact("t1", "critical_file.go"), artifact("t2", "churn_file.go"), artifact("t3", "quiet_file.go"),
		{Predicate: "intelligence_safety_warning", Args: []any{"camp1", "critical_file.go", "delete", "no_delete", "critical"}},
		{Predicate: "intelligence_churn_hotspot", Args: []any{"churn_file.go", int64(15), "high churn"}},
	}
	require.NoError(t, k.AssertBatch(facts))

	rows, err := k.Query("context_relevant")
	require.NoError(t, err)
	// A file may be relevant at several ranks; the compiler keeps the highest.
	ranks := map[string]map[string]bool{}
	for _, f := range rows {
		if len(f.Args) == 2 {
			path := types.ExtractString(f.Args[0])
			if ranks[path] == nil {
				ranks[path] = map[string]bool{}
			}
			ranks[path][types.ExtractString(f.Args[1])] = true
		}
	}
	if !ranks["critical_file.go"]["/p95"] {
		t.Errorf("a critical safety warning must make the file /p95 relevant, got %v", ranks["critical_file.go"])
	}
	if !ranks["churn_file.go"]["/p85"] || ranks["churn_file.go"]["/p95"] {
		t.Errorf("churn above 10 must make the file /p85 and not /p95 relevant, got %v", ranks["churn_file.go"])
	}
	if r, ok := ranks["quiet_file.go"]; ok {
		t.Errorf("a quiet plan file is not relevant on its own account, got %v", r)
	}
}
