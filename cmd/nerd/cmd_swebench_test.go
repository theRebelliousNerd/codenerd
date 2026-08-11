package main

import (
	"testing"

	"codenerd/internal/tactile/swebench"
)

func TestSWEBenchSetupPayload(t *testing.T) {
	inst := &swebench.Instance{
		InstanceID:       "test__repo-1",
		Repo:             "org/repo",
		BaseCommit:       "abc123def456",
		ProblemStatement: "fix the bug",
		FailToPass:       []string{"test_a", "test_b"},
		PassToPass:       []string{"test_c", "test_d"},
	}

	payload := swebenchSetupPayload(inst)

	// Scalar fields must round-trip correctly.
	if got := payload["instance_id"]; got != inst.InstanceID {
		t.Fatalf("instance_id = %v, want %q", got, inst.InstanceID)
	}
	if got := payload["repo"]; got != inst.Repo {
		t.Fatalf("repo = %v, want %q", got, inst.Repo)
	}
	if got := payload["base_commit"]; got != inst.BaseCommit {
		t.Fatalf("base_commit = %v, want %q", got, inst.BaseCommit)
	}
	if got := payload["problem_statement"]; got != inst.ProblemStatement {
		t.Fatalf("problem_statement = %v, want %q", got, inst.ProblemStatement)
	}

	// The critical invariant: slices must be []any, not []string, so that
	// handleSWEBenchSetup's `req.Payload["fail_to_pass"].([]any)` assertion succeeds.
	ftpRaw, ok := payload["fail_to_pass"]
	if !ok {
		t.Fatal("payload missing fail_to_pass")
	}
	ftp, ok := ftpRaw.([]any)
	if !ok {
		t.Fatalf("fail_to_pass has type %T, want []any (payload must convert []string element-by-element)", ftpRaw)
	}
	if len(ftp) != 2 {
		t.Fatalf("fail_to_pass length = %d, want 2", len(ftp))
	}
	if ftp[0] != "test_a" || ftp[1] != "test_b" {
		t.Fatalf("fail_to_pass order/content wrong: got %v, want [test_a test_b]", ftp)
	}

	ptpRaw, ok := payload["pass_to_pass"]
	if !ok {
		t.Fatal("payload missing pass_to_pass")
	}
	ptp, ok := ptpRaw.([]any)
	if !ok {
		t.Fatalf("pass_to_pass has type %T, want []any (payload must convert []string element-by-element)", ptpRaw)
	}
	if len(ptp) != 2 {
		t.Fatalf("pass_to_pass length = %d, want 2", len(ptp))
	}
	if ptp[0] != "test_c" || ptp[1] != "test_d" {
		t.Fatalf("pass_to_pass order/content wrong: got %v, want [test_c test_d]", ptp)
	}
}
