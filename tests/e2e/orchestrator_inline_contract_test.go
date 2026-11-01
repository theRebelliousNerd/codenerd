//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/evidence"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

func inlinePersistedWitness(root string) (evidence.Report, error) {
	var report evidence.Report
	paths, err := filepath.Glob(filepath.Join(root, ".nerd", "evidence", "*.json"))
	if err != nil || len(paths) != 1 {
		return report, fmt.Errorf("expected one persisted caller report: paths=%d err=%v", len(paths), err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		return report, err
	}
	if err = json.Unmarshal(data, &report); err != nil {
		return report, err
	}
	if report.Status != "verified" || report.Before == "" || report.After == "" || report.Before == report.After || len(report.Baseline) != 1 || len(report.Witnesses) != 1 || report.Baseline[0].Status != "failed" || report.Baseline[0].Snapshot != report.Before || report.Witnesses[0].Status != "passed" || report.Witnesses[0].Snapshot != report.After {
		return report, fmt.Errorf("report lacks failed-before/passed-after snapshot evidence: %+v", report)
	}
	witness := report.Witnesses[0]
	if len(witness.Command) == 0 || witness.ExecutableSHA256 == "" || witness.ExecutableSHA256 != witness.ExecutableAfterSHA256 || witness.Toolchain == "" || witness.Environment == "" || witness.OutputHash == "" {
		return report, errors.New("post-write witness lacks an executed stable toolchain/command/output identity")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snapshot, err := evidence.Snapshot(ctx, root)
	if err != nil || snapshot != report.After {
		return report, fmt.Errorf("acceptance no longer describes the current artifact: snapshot=%s after=%s err=%v", snapshot, report.After, err)
	}
	return report, nil
}

// Only single-turn inline cases use this fixture. It keeps their string API,
// /fix route and session contexts; the carrier edit now has genuine evidence.
func setupVerifiedInlineEnvironment(t *testing.T, ctx context.Context) (*session.Executor, *session.JITExecutor, context.Context) {
	t.Helper()
	global := tools.Global()
	globalRoot, globalWrite := global.WorkspaceRoot(), global.Get("write_file")
	fixture := newVerifiedOrchestratorFixture(t)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() {
		if tools.Global() != global || global.WorkspaceRoot() != globalRoot || global.Get("write_file") != globalWrite {
			t.Error("inline fixture changed another registry owner's binding")
		}
		if body, err := os.ReadFile(filepath.Join(fixture.root, "calc.go")); err != nil || string(body) != fixtureFixedSum {
			t.Errorf("inline string API did not leave the actual fix: %q %v", body, err)
		}
		if body, err := os.ReadFile(filepath.Join(fixture.root, "calc_test.go")); err != nil || string(body) != fixtureSumTests {
			t.Errorf("inline turn changed its protected behavioral oracle: %q %v", body, err)
		}
		report, err := inlinePersistedWitness(fixture.root)
		if err != nil {
			t.Error(err)
		} else {
			t.Logf("inline persisted acceptance: before=%s after=%s baseline=%s witness=%s command=%v", report.Before, report.After, report.Baseline[0].Status, report.Witnesses[0].Status, report.Witnesses[0].Command)
		}
		after, err := fixtureGoTest(t, fixture.root)
		if err != nil || !strings.Contains(string(after), "PASS: TestSum") {
			t.Errorf("independent test after inline execution failed: %v\n%s", err, after)
		}
	})
	return fixture.executor, fixture.jit, fixture.withContract(ctx)
}

func TestInlineWitnessLegacyRefusal(t *testing.T) {
	// Reproduce the full-suite binding explicitly: the normal production
	// handler is already registered before legacy setup. A fresh empty
	// registry instead selects the legacy stub, which fails later at roles.
	owned := tools.NewRegistry()
	write := toolscore.WriteFileTool()
	write.Effect = tools.EffectWrite
	if err := owned.Register(write); err != nil {
		t.Fatal(err)
	}
	owned.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"write_file"}})
	restore := tools.SwapGlobal(owned)
	t.Cleanup(func() {
		if tools.Global() == owned {
			restore()
		} else {
			t.Error("legacy diagnostic lost registry ownership")
		}
	})
	executor, _ := setupTestEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := executor.ProcessWithIntent(ctx, "diagnose the legacy carrier write", &perception.Intent{Verb: "/fix"})
	if result == nil || errors.Join(err, result.Error) == nil || result.TurnOutcome == "/done" || result.ToolCallsExecuted == 0 || result.SuccessfulToolCalls != 0 || result.WriteToolEffects != 0 || len(result.ToolErrors) == 0 {
		t.Fatalf("legacy fixture did not reproduce refusal before an accepted write: result=%+v err=%v", result, err)
	}
	t.Logf("legacy actual tool errors: %v; turn outcome=%s", result.ToolErrors, result.TurnOutcome)
}

func TestInlineWitnessAdmissionControls(t *testing.T) {
	for _, mode := range []string{"verified", "no_contract", "red_test", "missing_role", "outside_workspace", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			global := tools.Global()
			globalRoot, globalWrite := global.WorkspaceRoot(), global.Get("write_file")
			fixture := newVerifiedOrchestratorFixture(t)
			sentinel := filepath.Join(t.TempDir(), "outside.go")
			const untouched = "unrelated owner\n"
			if err := os.WriteFile(sentinel, []byte(untouched), 0600); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "red_test":
				fixture.model.content = fixtureBrokenSum + "// remains broken\n"
			case "missing_role":
				fixture.executor.SetRolePrompts(nil)
			case "outside_workspace":
				fixture.model.path = sentinel
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			runCtx := fixture.withContract(ctx)
			if mode == "no_contract" {
				runCtx = ctx
			}
			if mode == "canceled" {
				cancel()
			}
			observed, err := fixture.jit.ExecuteObservedWithContext(runCtx, session.TaskRequest{IntentVerb: "/fix", Task: "correct Sum in calc.go", Target: "calc.go"}, nil, types.PriorityNormal)
			if mode == "verified" {
				if err != nil || !observed.Done() || observed.Stage != "behavior_verified" || observed.Acceptance == nil || observed.Acceptance.Status != "verified" || observed.Workspace != fixture.root || len(observed.Changed) != 1 || filepath.Base(observed.Changed[0]) != "calc.go" || observed.Build == nil || !observed.Build.Ran || !observed.Build.OK || observed.Tests == nil || !observed.Tests.Ran || !observed.Tests.OK {
					t.Fatalf("inline clone lacks native observed completion evidence: %+v err=%v", observed, err)
				}
				report, readErr := inlinePersistedWitness(fixture.root)
				if readErr != nil || observed.Snapshot != report.After {
					t.Fatalf("inline observed/persisted snapshots disagree: observed=%s report=%+v err=%v", observed.Snapshot, report, readErr)
				}
				after, testErr := fixtureGoTest(t, fixture.root)
				if testErr != nil || !strings.Contains(string(after), "PASS: TestSum") {
					t.Fatalf("independent inline witness failed: %v\n%s", testErr, after)
				}
			} else if mode == "no_contract" {
				if err != nil || !observed.Done() || observed.Stage != "checks_passed" || observed.Acceptance != nil || observed.Build == nil || !observed.Build.Ran || !observed.Build.OK || observed.Tests == nil || !observed.Tests.Ran || !observed.Tests.OK || observed.Snapshot == "" {
					t.Fatalf("mechanical completion fabricated behavior acceptance or lost native checks: %+v err=%v", observed, err)
				}
			} else {
				if observed.Done() || (observed.Acceptance != nil && observed.Acceptance.Status == "verified") {
					t.Fatalf("%s manufactured verified completion: %+v err=%v", mode, observed, err)
				}
				if mode == "missing_role" && !errors.Is(err, types.ErrRolePromptUnavailable) {
					t.Fatalf("role refusal lost typed identity: %v", err)
				}
				if mode == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("owner cancellation lost typed identity: %v", err)
				}
			}
			if body, readErr := os.ReadFile(sentinel); readErr != nil || string(body) != untouched {
				t.Fatalf("unrelated owner changed: %q %v", body, readErr)
			}
			if mode == "outside_workspace" || mode == "canceled" {
				if body, readErr := os.ReadFile(filepath.Join(fixture.root, "calc.go")); readErr != nil || string(body) != fixtureBrokenSum {
					t.Fatalf("refusal changed the carrier source: %q %v", body, readErr)
				}
			}
			if body, readErr := os.ReadFile(filepath.Join(fixture.root, "calc_test.go")); readErr != nil || string(body) != fixtureSumTests {
				t.Fatalf("behavioral oracle changed: %q %v", body, readErr)
			}
			if tools.Global() != global || global.WorkspaceRoot() != globalRoot || global.Get("write_file") != globalWrite {
				t.Fatal("private inline fixture changed global registry state")
			}
		})
	}
}
