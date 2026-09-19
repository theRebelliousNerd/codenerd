package codedom

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
)

// External audit N07 (2026-09-19): run_impacted_tests counted as a test
// execution by its name. Three of its paths return success and run no test --
// no edit known to it, a selection that picked nothing, a dry run -- and none
// of them may leave a test-run receipt. A run that executes go test does.
func TestRunImpactedTests_OnlyARealRunLeavesATestRunReceipt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   map[string]any
		impact []ImpactedTestInfo
	}{
		{name: "no known edit", args: map[string]any{}},
		{name: "empty selection", args: map[string]any{"edited_refs": []string{"foo.go"}}},
		{name: "dry run", args: map[string]any{"edited_refs": []string{"foo.go"}, "dry_run": true},
			impact: []ImpactedTestInfo{{TestRef: "TestFoo", Priority: "high"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := testImpactProvider()
			RegisterTestImpactProvider(&mockImpactProvider{analyzer: &mockAnalyzer{impactedTests: tc.impact}})
			defer RegisterTestImpactProvider(old)

			ctx, runs := tools.WithTestRunLog(context.Background())
			if _, err := executeRunImpactedTests(ctx, tc.args); err != nil {
				t.Fatalf("executeRunImpactedTests: %v", err)
			}
			if got := runs(); len(got) != 0 {
				t.Fatalf("%s ran no test and left test-run receipts %+v", tc.name, got)
			}
		})
	}

	t.Run("a real run", func(t *testing.T) {
		if testing.Short() {
			t.Skip("needs the go toolchain")
		}
		dir := t.TempDir()
		for name, content := range map[string]string{
			"go.mod":      "module cdreceipt\n\ngo 1.21\n",
			"lib.go":      "package cdreceipt\n\nfunc Inc(n int) int { return n + 1 }\n",
			"lib_test.go": "package cdreceipt\n\nimport \"testing\"\n\nfunc TestInc(t *testing.T) {\n\tif Inc(1) != 2 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n",
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		ctx, runs := tools.WithTestRunLog(context.Background())
		if _, err := runGoTests(ctx, dir, []string{dir}, "60s", false); err != nil {
			t.Fatalf("runGoTests: %v", err)
		}
		if got := runs(); len(got) != 1 || len(got[0].Argv) < 2 || got[0].Argv[1] != "test" || got[0].ExitCode != 0 {
			t.Fatalf("a go test run must leave one receipt with its argv and exit code, got %+v", got)
		}
	})
}
