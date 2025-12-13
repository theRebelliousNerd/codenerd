package tester

import "testing"

func TestTesterParseTask_Defaults(t *testing.T) {
	shard := NewTesterShard()
	parsed, err := shard.parseTask("run_tests")
	if err != nil {
		t.Fatalf("parseTask: %v", err)
	}
	if parsed.Action != "run_tests" {
		t.Fatalf("Action=%q, want run_tests", parsed.Action)
	}
	if parsed.Target != "./..." {
		t.Fatalf("Target=%q, want ./...", parsed.Target)
	}
}

func TestTesterParseTask_FileAndPackage(t *testing.T) {
	shard := NewTesterShard()

	parsed, err := shard.parseTask("test file:internal/core/kernel.go")
	if err != nil {
		t.Fatalf("parseTask: %v", err)
	}
	if parsed.Action != "run_tests" || parsed.File != "internal/core/kernel.go" || parsed.Target != "internal/core/kernel.go" {
		t.Fatalf("unexpected parsed result: %+v", parsed)
	}

	parsed, err = shard.parseTask("coverage pkg:./internal/core")
	if err != nil {
		t.Fatalf("parseTask: %v", err)
	}
	if parsed.Action != "coverage" || parsed.Package != "./internal/core" || parsed.Target != "./internal/core" {
		t.Fatalf("unexpected parsed result: %+v", parsed)
	}
}
