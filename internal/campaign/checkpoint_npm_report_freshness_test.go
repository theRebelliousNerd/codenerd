package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tactile"
)

// npmFreshExecutor simulates the npm command and optionally writes its report.
type npmFreshExecutor struct {
	captured *tactile.Command
	result   tactile.ExecutionResult
	report   string
}

func (f *npmFreshExecutor) Execute(ctx context.Context, cmd tactile.Command) (*tactile.ExecutionResult, error) {
	c := cmd
	f.captured = &c
	if f.report != "" {
		var reportPath string
		for _, arg := range cmd.Arguments {
			if strings.HasPrefix(arg, "--outputFile=") {
				reportPath = strings.TrimPrefix(arg, "--outputFile=")
				break
			}
		}
		if reportPath == "" {
			return nil, fmt.Errorf("npm command did not request a report")
		}
		if !filepath.IsAbs(reportPath) {
			reportPath = filepath.Join(cmd.WorkingDirectory, reportPath)
		}
		if err := os.WriteFile(reportPath, []byte(f.report), 0o600); err != nil {
			return nil, fmt.Errorf("write simulated npm report: %w", err)
		}
	}
	return &f.result, nil
}

func (f *npmFreshExecutor) Capabilities() tactile.ExecutorCapabilities {
	return tactile.ExecutorCapabilities{}
}

func (f *npmFreshExecutor) Validate(cmd tactile.Command) error { return nil }

func TestRunTestsCheckpoint_IgnoresStaleNpmReport(t *testing.T) {
	t.Parallel()
	t.Run("parse jest json table", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name       string
			data       string
			wantPassed int
			wantFailed int
		}{
			{"happy parses counts", `{"numPassedTests":5,"numFailedTests":0}`, 5, 0},
			{"edge failures parsed", `{"numPassedTests":3,"numFailedTests":2}`, 3, 2},
			{"empty returns zeros", ``, 0, 0},
			{"nil invalid returns zeros", `not-json`, 0, 0},
			{"audited stale shape", `{"numPassedTests":0,"numFailedTests":4}`, 0, 4},
		}
		cr := &CheckpointRunner{}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				p, f := cr.parseJestJSON([]byte(tt.data))
				if p != tt.wantPassed || f != tt.wantFailed {
					t.Errorf("parseJestJSON = (%d,%d), want (%d,%d)", p, f, tt.wantPassed, tt.wantFailed)
				}
			})
		}
	})

	tests := []struct {
		name        string
		staleReport string
		freshReport string
		exitCode    int
		wantPassed  bool
		wantDetails string
	}{
		{name: "no report", wantPassed: true, wantDetails: "All 1 tests passed"},
		{name: "stale failures do not fail a passing run", staleReport: `{"numPassedTests":0,"numFailedTests":4}`, wantPassed: true, wantDetails: "All 1 tests passed"},
		{name: "stale successes do not change the count", staleReport: `{"numPassedTests":7,"numFailedTests":0}`, wantPassed: true, wantDetails: "All 1 tests passed"},
		{name: "fresh failures are used", staleReport: `{"numPassedTests":0,"numFailedTests":4}`, freshReport: `{"numPassedTests":5,"numFailedTests":2}`, wantDetails: "Tests: 5 passed, 2 failed"},
		{name: "nonzero exit remains a failure", staleReport: `{"numPassedTests":7,"numFailedTests":0}`, exitCode: 1, wantDetails: "Tests: 1 passed, 0 failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			if err := os.WriteFile(filepath.Join(ws, "package.json"), []byte(`{"name":"x","scripts":{"test":"jest"}}`), 0o600); err != nil {
				t.Fatal(err)
			}
			nerdDir := filepath.Join(ws, ".nerd")
			if err := os.MkdirAll(nerdDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if tt.staleReport != "" {
				stalePath := filepath.Join(nerdDir, "npm-test.json")
				if err := os.WriteFile(stalePath, []byte(tt.staleReport), 0o600); err != nil {
					t.Fatal(err)
				}
				old := time.Now().Add(-2 * time.Hour)
				if err := os.Chtimes(stalePath, old, old); err != nil {
					t.Fatal(err)
				}
			}

			fake := &npmFreshExecutor{
				result: tactile.ExecutionResult{Success: true, ExitCode: tt.exitCode, Stdout: "Test Suites: 1 passed, 1 total\n"},
				report: tt.freshReport,
			}
			cr := &CheckpointRunner{executor: fake, workspace: ws}
			passed, details, err := cr.runTestsCheckpoint(context.Background())
			if err != nil {
				t.Fatalf("runTestsCheckpoint: %v", err)
			}
			if fake.captured == nil || fake.captured.Binary != "npm" {
				t.Fatalf("expected npm command, got %#v", fake.captured)
			}
			if passed != tt.wantPassed {
				t.Errorf("passed = %t, want %t; details: %s", passed, tt.wantPassed, details)
			}
			if !strings.Contains(details, tt.wantDetails) {
				t.Errorf("details = %q, want substring %q", details, tt.wantDetails)
			}
		})
	}
}
