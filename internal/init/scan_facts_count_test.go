package init

import (
	"codenerd/internal/core"
	"codenerd/internal/world"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRunPhase5Facts_PreservesScanFactCount ensures the workspace scan summary
// "Facts generated" reflects the real scan-produced fact count. Phase 2 records
// len(scan facts) in InitResult.FactsGenerated; phase 5 must add the profile
// facts on top instead of overwriting the scan count with the handful of
// profile.mg rows.
func TestRunPhase5Facts_PreservesScanFactCount(t *testing.T) {
	nerdDir := t.TempDir()
	profile := ProjectProfile{
		ProjectID:   "test-project",
		Name:        "test",
		Description: "test project",
		Language:    "go",
	}

	const scanFactCount = 1234
	result := &InitResult{FactsGenerated: scanFactCount}

	i := &Initializer{}
	runner := newPhaseRunner(i)
	i.runPhase5Facts(runner, result, nerdDir, profile)

	if len(result.Failures) != 0 {
		t.Fatalf("runPhase5Facts reported failures: %v", result.Failures)
	}
	if result.FactsGenerated <= scanFactCount {
		t.Fatalf("FactsGenerated = %d, want > scan count %d (scan facts were overwritten by profile-only count)", result.FactsGenerated, scanFactCount)
	}
}

// TestRunPhase2Scanning_RecordsScanFactCount pins the scan side of the
// workspace scan summary: runPhase2Scanning must record the real
// scan-produced fact count in InitResult.FactsGenerated. Without the
// assignment the summary falls back to the handful of profile.mg rows (or
// zero here), so removing that line makes this test fail.
func TestRunPhase2Scanning_RecordsScanFactCount(t *testing.T) {
	ctx := context.Background()
	workspace := t.TempDir()
	goFile := filepath.Join(workspace, "hello.go")
	content := "package tmp\n\nfunc Hello() string { return \"hi\" }\n"
	if err := os.WriteFile(goFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp go file: %v", err)
	}
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("create kernel: %v", err)
	}
	i := &Initializer{
		config:  InitConfig{Workspace: workspace},
		scanner: world.NewScanner(),
		kernel:  kernel,
	}
	result := &InitResult{}
	runner := newPhaseRunner(i)
	scanResult := i.runPhase2Scanning(ctx, runner, result)
	if scanResult == nil {
		t.Fatalf("runPhase2Scanning returned nil scan result: failures=%v", result.Failures)
	}
	want := len(scanResult.ToFacts())
	if want == 0 {
		t.Fatalf("scan of temp workspace produced no facts, test cannot pin the count")
	}
	if result.FactsGenerated != want {
		t.Fatalf("FactsGenerated = %d, want scan-produced fact count %d", result.FactsGenerated, want)
	}
}
