package core

import (
	"testing"

	"codenerd/internal/tactile"
)

func TestVirtualStoreInjectTactileFact_AnalyzerFactsMatchLiveKernelSchema(t *testing.T) {
	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel failed: %v", err)
	}
	store := &VirtualStore{kernel: kernel}

	facts := (tactile.AuditEvent{
		Type: tactile.AuditEventComplete,
		Command: tactile.Command{
			Binary:    "go",
			Arguments: []string{"test", "./..."},
			RequestID: "test-request",
		},
		Result: &tactile.ExecutionResult{
			Success:  true,
			ExitCode: 1,
			Stdout: "--- PASS: TestGood (0.01s)\n" +
				"--- FAIL: TestBroken (0.01s)\n" +
				"FAIL\ncoverage: 87.5% of statements",
		},
	}).ToFacts()
	facts = append(facts, (tactile.AuditEvent{
		Type: tactile.AuditEventComplete,
		Command: tactile.Command{
			Binary:    "go",
			Arguments: []string{"build", "./..."},
			RequestID: "build-request",
		},
		Result: &tactile.ExecutionResult{
			Success:  true,
			ExitCode: 1,
			Stderr:   `C:\CodeProjects\codeNERD\main.go:12:7: undefined: thing`,
		},
	}).ToFacts()...)
	facts = append(facts, tactile.Fact{
		Predicate: "execution_tag",
		Args:      []any{"success", "mode", "none"},
	})

	for _, fact := range facts {
		if err := store.injectTactileFact(fact); err != nil {
			t.Fatalf("inject %s/%d: %v", fact.Predicate, len(fact.Args), err)
		}
	}

	wantCounts := map[string]int{
		"execution_test_summary":  1,
		"execution_test_state":    1,
		"execution_failed_test":   1,
		"execution_test_coverage": 1,
		"execution_build_summary": 1,
		"execution_diagnostic":    1,
		"execution_tag":           1,
	}
	for predicate, want := range wantCounts {
		got, queryErr := kernel.Query(predicate)
		if queryErr != nil {
			t.Fatalf("query %s: %v", predicate, queryErr)
		}
		if len(got) != want {
			t.Errorf("%s facts = %d, want %d: %+v", predicate, len(got), want, got)
		}
	}
}

func TestVirtualStoreInjectTactileFact_WithoutKernelReturnsError(t *testing.T) {
	store := &VirtualStore{}
	if err := store.injectTactileFact(tactile.Fact{Predicate: "execution_success", Args: []any{"r1"}}); err == nil {
		t.Fatal("injectTactileFact returned nil without a kernel")
	}
}
