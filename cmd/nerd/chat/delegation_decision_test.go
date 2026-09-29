package chat

import (
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// One kernel for the package's delegation-decision tests. NewRealKernel loads
// the embedded corpus; booting it per assertion would re-evaluate that corpus
// for every row of the verb table. Tests here do not call t.Parallel, so they
// do not share the kernel with a concurrent test.
var (
	delegationKernelOnce sync.Once
	delegationKernel     chatKernel
	delegationKernelErr  error
)

func delegationTestKernel(t *testing.T) chatKernel {
	t.Helper()
	delegationKernelOnce.Do(func() {
		delegationKernel, delegationKernelErr = core.NewRealKernel()
	})
	if delegationKernelErr != nil {
		t.Fatalf("NewRealKernel: %v", delegationKernelErr)
	}
	return delegationKernel
}

func delegatedFile(t *testing.T, findings []map[string]any) string {
	t.Helper()
	m := Model{kernel: delegationTestKernel(t)}
	return m.delegationTargetFile(findings)
}

func executionModes(t *testing.T, k chatKernel, verb string) []string {
	t.Helper()
	facts, err := k.Query("execution_mode")
	if err != nil {
		t.Fatalf("query execution_mode: %v", err)
	}
	var out []string
	for _, fact := range facts {
		if len(fact.Args) < 2 {
			continue
		}
		if types.ExtractString(fact.Args[0]) != verb {
			continue
		}
		out = append(out, types.ExtractString(fact.Args[1]))
	}
	return out
}

func TestFullTieIsBrokenByEarliestIndex(t *testing.T) {
	// Same count, same worst severity. The earlier citation wins. The index
	// is the finding's position, so two files cannot share one.
	findings := []map[string]any{
		{"file": "a/earlier.go", "severity": "medium"},
		{"file": "z/later.go", "severity": "medium"},
		{"file": "a/earlier.go", "severity": "low"},
		{"file": "z/later.go", "severity": "low"},
	}
	if got := delegatedFile(t, findings); got != "a/earlier.go" {
		t.Errorf("delegated file = %q, want a/earlier.go — equal count and severity, earlier index", got)
	}
}

func TestUnknownSeveritySortsBelowLow(t *testing.T) {
	// A word the ladder does not name is /unknown, rank 0. On a count tie it
	// loses to /low. Three unknown citations still beat one critical, because
	// count is the first key.
	tie := []map[string]any{
		{"file": "a.go", "severity": "low"},
		{"file": "b.go", "severity": "mystery"},
	}
	if got := delegatedFile(t, tie); got != "a.go" {
		t.Errorf("count-tie file = %q, want a.go — /low outranks /unknown", got)
	}
	count := []map[string]any{
		{"file": "rare/once.go", "severity": "critical"},
		{"file": "common/many.go", "severity": "nope"},
		{"file": "common/many.go", "severity": "nope"},
		{"file": "common/many.go", "severity": "nope"},
	}
	if got := delegatedFile(t, count); got != "common/many.go" {
		t.Errorf("count file = %q, want common/many.go — count outranks severity, including /unknown", got)
	}
}

func TestEmptyCitationKeepsNoFile(t *testing.T) {
	findings := []map[string]any{
		{"severity": "critical", "message": "no location"},
		{"file": "   ", "severity": "high"},
	}
	if got := delegatedFile(t, findings); got != "" {
		t.Errorf("delegated file = %q, want empty when no finding names a file", got)
	}
}

func TestNilKernelKeepsTheGenericTarget(t *testing.T) {
	var m Model
	findings := []map[string]any{{"file": "a.go", "severity": "critical", "message": "boom"}}
	if got := m.delegationTargetFile(findings); got != "" {
		t.Errorf("delegationTargetFile = %q, want empty without a kernel", got)
	}
	if _, ok := m.derivedExecutionMode("/fix"); ok {
		t.Error("derivedExecutionMode reported a mode without a kernel")
	}
	if _, ok := m.derivedExecutionMode(""); ok {
		t.Error("empty verb derived a mode")
	}
	if _, ok := m.derivedExecutionMode("   "); ok {
		t.Error("blank verb derived a mode")
	}
	task := m.formatShardTaskWithContext("/fix", "codebase", "none", t.TempDir(), &ShardResult{
		ShardType: "reviewer",
		Findings:  findings,
	})
	if task != "fix file:codebase findings:[critical:boom]" {
		t.Fatalf("task = %q, want the generic target kept", task)
	}
}

func TestFormatShardTaskNamesTheDerivedFile(t *testing.T) {
	m := Model{kernel: delegationTestKernel(t)}
	task := m.formatShardTaskWithContext("/fix", "codebase", "none", t.TempDir(), &ShardResult{
		ShardType: "reviewer",
		Findings:  extractFindings(reviewerOutput),
	})
	if !strings.Contains(task, "fix file:internal/auth/session.go ") {
		t.Fatalf("task = %q, want the derived file in the fixer grammar", task)
	}
}

func TestExecutionModeMatchesTheVerbTable(t *testing.T) {
	k := delegationTestKernel(t)
	if err := k.Retract("asked_delegation_verb"); err != nil {
		t.Fatalf("retract: %v", err)
	}
	// A verb that was never asked and is not in the table has no row. The
	// /parallel default is derived only once the verb is asked.
	if got := executionModes(t, k, "/no_such_verb"); len(got) != 0 {
		t.Fatalf("execution_mode(/no_such_verb) = %q before it was asked, want none", got)
	}

	m := Model{kernel: k}
	want := []struct{ verb, mode string }{
		{"/review", "/parallel"},
		{"/security", "/parallel"},
		{"/test", "/parallel"},
		{"/create", "/advisory"},
		{"/debug", "/advisory"},
		{"/fix", "/advisory_with_critique"},
		{"/refactor", "/advisory_with_critique"},
	}
	for _, tt := range want {
		got, ok := m.derivedExecutionMode(tt.verb)
		if !ok || got != tt.mode {
			t.Errorf("execution_mode(%s) = %q ok=%v, want %s", tt.verb, got, ok, tt.mode)
		}
	}

	// "fix" is not the table's /fix. It is an asked verb with no configured
	// row, so the default rule derives /parallel — the topology the map
	// returned for a key it did not hold. /fix itself keeps its one row.
	got, ok := m.derivedExecutionMode("fix")
	if !ok || got != "/parallel" {
		t.Fatalf(`execution_mode("fix") = %q ok=%v, want /parallel`, got, ok)
	}
	if rows := executionModes(t, k, "/fix"); len(rows) != 1 || rows[0] != "/advisory_with_critique" {
		t.Errorf(`asking "fix" changed /fix: %q`, rows)
	}

	got, ok = m.derivedExecutionMode("/no_such_verb")
	if !ok || got != "/parallel" {
		t.Fatalf("execution_mode(/no_such_verb) = %q ok=%v, want /parallel", got, ok)
	}
	// Asking /fix afterwards retracts the unknown verb. /fix keeps the one
	// table mode; the unknown's /parallel row is gone.
	got, ok = m.derivedExecutionMode("/fix")
	if !ok || got != "/advisory_with_critique" {
		t.Fatalf("execution_mode(/fix) after /no_such_verb = %q ok=%v, want /advisory_with_critique", got, ok)
	}
	if rows := executionModes(t, k, "/fix"); len(rows) != 1 || rows[0] != "/advisory_with_critique" {
		t.Errorf("execution_mode(/fix) rows = %q, want only /advisory_with_critique", rows)
	}
	if rows := executionModes(t, k, "/no_such_verb"); len(rows) != 0 {
		t.Errorf("execution_mode(/no_such_verb) rows = %q after it was retracted, want none", rows)
	}
}
