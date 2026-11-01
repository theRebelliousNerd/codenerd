//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

const nilSessionParentMarker = "nil-session-parent-only"

// Observe the actual inline clone's compilation and native worker offer. The
// parent intentionally has DreamMode and history; neither belongs to this task.
type nilSessionProbe struct {
	mu                 sync.Mutex
	want               *types.SessionContext
	compiles, requests int
	violations         []string
}

func (p *nilSessionProbe) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	p.mu.Lock()
	p.compiles++
	got, typed := cc.SessionContext.(*types.SessionContext)
	if (cc.SessionContext != nil && !typed) || got != p.want || cc.OperationalMode != "/active" {
		err := fmt.Errorf("task context not isolated: got=%p want=%p mode=%s", got, p.want, cc.OperationalMode)
		p.violations = append(p.violations, err.Error())
		p.mu.Unlock()
		return nil, err
	}
	p.mu.Unlock()
	return (verifiedFixtureCompiler{}).Compile(ctx, cc)
}

func (p *nilSessionProbe) observe(system string, definitions []types.ToolDefinition) error {
	// Retain the legacy fixture's one-millisecond inert worker delay. No live
	// provider is involved; next() checks owner cancellation before editing.
	time.Sleep(time.Millisecond)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests++
	var names []string
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	if !strings.Contains(system, "fixture worker") || strings.Contains(system, nilSessionParentMarker) || !slices.Equal(names, []string{"write_file"}) {
		err := fmt.Errorf("unexpected inline worker prompt or catalog: tools=%v", names)
		p.violations = append(p.violations, err.Error())
		return err
	}
	return nil
}

func newNilSessionFixture(t *testing.T) (*verifiedOrchestratorFixture, *nilSessionProbe, []perception.ConversationTurn) {
	t.Helper()
	p := &nilSessionProbe{}
	f := newVerifiedOrchestratorFixtureWithRuntime(t, p, nil)
	f.model.workerRequestObserver = p.observe
	f.executor.SetSessionContext(&types.SessionContext{DreamMode: true, CompressedHistory: nilSessionParentMarker})
	history := []perception.ConversationTurn{{Role: "user", Content: nilSessionParentMarker}}
	f.executor.SetHistory(history)
	return f, p, f.executor.GetHistory()
}

func assertNilSessionOwner(t *testing.T, f *verifiedOrchestratorFixture, p *nilSessionProbe, history []perception.ConversationTurn, canceled bool) {
	t.Helper()
	p.mu.Lock()
	compiles, requests, violations := p.compiles, p.requests, slices.Clone(p.violations)
	p.mu.Unlock()
	if len(violations) != 0 || (!canceled && (compiles == 0 || requests == 0)) || (canceled && requests != 0) {
		t.Errorf("inline context boundaries: compiles=%d requests=%d violations=%v canceled=%t", compiles, requests, violations, canceled)
	}
	if !reflect.DeepEqual(f.executor.GetHistory(), history) {
		t.Error("delegated task changed parent conversation history")
	}
}

func setupVerifiedNilSessionEnvironment(t *testing.T, ctx context.Context) (*session.Executor, *session.JITExecutor, context.Context) {
	t.Helper()
	global := tools.Global()
	root, write := global.WorkspaceRoot(), global.Get("write_file")
	f, p, history := newNilSessionFixture(t)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() {
		assertNilSessionOwner(t, f, p, history, false)
		assertFallbackWitness(t, f)
		if tools.Global() != global || global.WorkspaceRoot() != root || global.Get("write_file") != write {
			t.Error("nil-session fixture changed another registry owner's binding")
		}
	})
	return f.executor, f.jit, f.withContract(ctx)
}

func TestNilSessionExecutionWitnessControls(t *testing.T) {
	for _, mode := range []string{"absent", "typed_caller", "canceled", "red_test", "missing_role", "outside_workspace"} {
		t.Run(mode, func(t *testing.T) {
			global := tools.Global()
			globalRoot, globalWrite := global.WorkspaceRoot(), global.Get("write_file")
			f, p, history := newNilSessionFixture(t)
			sentinel := filepath.Join(t.TempDir(), "unrelated.go")
			const untouched = "unrelated owner\n"
			if err := os.WriteFile(sentinel, []byte(untouched), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			runCtx := f.withContract(ctx)
			switch mode {
			case "typed_caller":
				p.want = &types.SessionContext{ExtraContext: map[string]string{"owner": "current-caller"}}
				runCtx = types.WithSessionContext(runCtx, p.want)
			case "canceled":
				cancel()
			case "red_test":
				f.model.content = fixtureBrokenSum + "// still broken\n"
			case "missing_role":
				f.executor.SetRolePrompts(nil)
			case "outside_workspace":
				f.model.path = sentinel
			}
			observed, err := f.jit.ExecuteObservedWithContext(runCtx, session.TaskRequest{IntentVerb: "/fix", Task: "correct Sum in calc.go", Target: "calc.go"}, nil, types.PriorityNormal)
			if mode == "absent" || mode == "typed_caller" {
				if err != nil || !observed.Done() || observed.Stage != "behavior_verified" || observed.Acceptance == nil || observed.Acceptance.Status != "verified" || observed.Build == nil || !observed.Build.Ran || !observed.Build.OK || observed.Tests == nil || !observed.Tests.Ran || !observed.Tests.OK || observed.Workspace != f.root || len(observed.Changed) != 1 || filepath.Base(observed.Changed[0]) != "calc.go" {
					t.Fatalf("nil explicit session context lost actual completion evidence: %+v err=%v", observed, err)
				}
				assertFallbackWitness(t, f)
				report, reportErr := inlinePersistedWitness(f.root)
				if reportErr != nil || observed.Snapshot != report.After {
					t.Fatalf("observed completion is not current persisted evidence: %+v err=%v", observed, reportErr)
				}
			} else {
				if observed.Done() || (observed.Acceptance != nil && observed.Acceptance.Status == "verified") {
					t.Fatalf("%s fabricated completion with nil explicit session context: %+v err=%v", mode, observed, err)
				}
				if mode == "missing_role" && !errors.Is(err, types.ErrRolePromptUnavailable) {
					t.Fatalf("missing role lost typed refusal: %v", err)
				}
				if mode == "canceled" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost typed owner error: %v", err)
				}
			}
			assertNilSessionOwner(t, f, p, history, mode == "canceled")
			if body, readErr := os.ReadFile(filepath.Join(f.root, "calc_test.go")); readErr != nil || string(body) != fixtureSumTests {
				t.Fatalf("protected oracle changed: %q err=%v", body, readErr)
			}
			if mode == "outside_workspace" || mode == "canceled" {
				if body, readErr := os.ReadFile(filepath.Join(f.root, "calc.go")); readErr != nil || string(body) != fixtureBrokenSum {
					t.Fatalf("refusal changed source before admission: %q err=%v", body, readErr)
				}
			}
			if body, readErr := os.ReadFile(sentinel); readErr != nil || string(body) != untouched {
				t.Fatalf("unrelated owner changed: %q err=%v", body, readErr)
			}
			if tools.Global() != global || global.WorkspaceRoot() != globalRoot || global.Get("write_file") != globalWrite {
				t.Fatal("nil-session fixture changed global registry ownership")
			}
		})
	}
}
