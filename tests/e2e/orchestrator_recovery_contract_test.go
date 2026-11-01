//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/evidence"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

func TestInlineRecoveryBeforeWorker(t *testing.T) {
	for _, mode := range []string{"canceled", "expired_deadline", "live_deadline", "red_retry", "missing_role_retry", "stale_witness"} {
		t.Run(mode, func(t *testing.T) {
			global := tools.Global()
			globalRoot, globalWrite := global.WorkspaceRoot(), global.Get("write_file")
			fixture := newVerifiedOrchestratorFixture(t)
			sentinel := filepath.Join(t.TempDir(), "unrelated.go")
			const untouched = "another owner\n"
			if err := os.WriteFile(sentinel, []byte(untouched), 0600); err != nil {
				t.Fatal(err)
			}
			owner, cancelOwner := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancelOwner()
			runCtx := fixture.withContract(owner)
			before, snapshotErr := evidence.Snapshot(owner, fixture.root)
			if snapshotErr != nil {
				t.Fatal(snapshotErr)
			}
			var interrupted context.Context
			var cancel context.CancelFunc
			cause := context.DeadlineExceeded
			if mode == "canceled" {
				interrupted, cancel = context.WithCancel(runCtx)
				cancel()
				cause = context.Canceled
			} else {
				interrupted, cancel = context.WithTimeout(runCtx, time.Millisecond)
				if mode != "live_deadline" {
					<-interrupted.Done()
				}
			}
			_, firstErr := fixture.jit.Execute(interrupted, session.TaskRequest{IntentVerb: "/fix", Task: "interrupted carrier edit", Target: "calc.go"})
			cancel()
			if !errors.Is(interrupted.Err(), cause) || !errors.Is(firstErr, cause) {
				t.Fatalf("interrupted owner lost typed stop: mode=%s owner=%v err=%v", mode, interrupted.Err(), firstErr)
			}
			fixture.model.mu.Lock()
			calls, emitted := fixture.model.workerCalls, fixture.model.emitted
			fixture.model.mu.Unlock()
			afterFirst, firstSnapshotErr := evidence.Snapshot(owner, fixture.root)
			if calls != 0 || emitted || firstSnapshotErr != nil || afterFirst != before {
				t.Fatalf("interrupted baseline consumed an edit or changed source: calls=%d emitted=%v before=%s after=%s err=%v", calls, emitted, before, afterFirst, firstSnapshotErr)
			}
			if body, err := os.ReadFile(filepath.Join(fixture.root, "calc.go")); err != nil || string(body) != fixtureBrokenSum {
				t.Fatalf("interrupted carrier changed: %q %v", body, err)
			}
			if mode == "red_retry" {
				fixture.model.content = fixtureBrokenSum + "// still broken\n"
			} else if mode == "missing_role_retry" {
				fixture.executor.SetRolePrompts(nil)
			}
			observed, retryErr := fixture.jit.ExecuteObserved(runCtx, session.TaskRequest{IntentVerb: "/fix", Task: "correct Sum in calc.go", Target: "calc.go"})
			if mode == "red_retry" || mode == "missing_role_retry" {
				if observed.Done() || (observed.Acceptance != nil && observed.Acceptance.Status == "verified") {
					t.Fatalf("%s bypassed retry evidence: %+v err=%v", mode, observed, retryErr)
				}
				if mode == "missing_role_retry" && !errors.Is(retryErr, types.ErrRolePromptUnavailable) {
					t.Fatalf("retry role refusal lost typed cause: %v", retryErr)
				}
			} else {
				if retryErr != nil || !observed.Done() || observed.Stage != "behavior_verified" || observed.Acceptance == nil || observed.Acceptance.Status != "verified" || observed.Build == nil || !observed.Build.Ran || !observed.Build.OK || observed.Tests == nil || !observed.Tests.Ran || !observed.Tests.OK {
					t.Fatalf("same executor did not recover with native evidence: %+v err=%v", observed, retryErr)
				}
				report, reportErr := inlinePersistedWitness(fixture.root)
				if reportErr != nil || report.After != observed.Snapshot {
					t.Fatalf("recovery lost its current persisted witness: %+v err=%v", report, reportErr)
				}
				after, testErr := fixtureGoTest(t, fixture.root)
				if testErr != nil || !strings.Contains(string(after), "PASS: TestSum") {
					t.Fatalf("independent recovery test failed: %v\n%s", testErr, after)
				}
				if mode == "stale_witness" {
					// A deliberate negative after measuring success; never a source reset
					// used to make the retry succeed.
					if err := os.WriteFile(filepath.Join(fixture.root, "calc.go"), []byte(fixtureBrokenSum), 0600); err != nil {
						t.Fatal(err)
					}
					stale, staleErr := inlinePersistedWitness(fixture.root)
					if staleErr == nil || stale.Status != "verified" {
						t.Fatalf("persisted status was mistaken for current applicability: %+v err=%v", stale, staleErr)
					}
				}
			}
			if body, err := os.ReadFile(filepath.Join(fixture.root, "calc_test.go")); err != nil || string(body) != fixtureSumTests {
				t.Fatalf("protected oracle changed: %q %v", body, err)
			}
			if body, err := os.ReadFile(sentinel); err != nil || string(body) != untouched {
				t.Fatalf("unrelated owner changed: %q %v", body, err)
			}
			if tools.Global() != global || global.WorkspaceRoot() != globalRoot || global.Get("write_file") != globalWrite {
				t.Fatal("private recovery fixture changed global registry ownership")
			}
		})
	}
}
