//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

func checkFallbackBaseline(system string) error {
	for _, required := range []string{
		"You are codeNERD, a logic-first coding agent",
		"Constitutional prime directive",
		"Default deny: an action whose safety cannot be determined is blocked",
		"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE",
	} {
		if !strings.Contains(system, required) {
			return fmt.Errorf("mandatory fallback fragment missing: %s", required)
		}
	}
	return nil
}

type fallbackFixtureProbe struct {
	mu                                  sync.Mutex
	delegate                            session.JITCompiler
	cancel                              context.CancelFunc
	compileCalls, configCalls, requests int
	expectedTools                       []string
	violations                          []string
}

func (p *fallbackFixtureProbe) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	p.mu.Lock()
	p.compileCalls++
	if !slices.Equal(cc.AvailableTools, p.expectedTools) {
		p.violations = append(p.violations, fmt.Sprintf("compile catalog=%v want=%v", cc.AvailableTools, p.expectedTools))
	}
	p.mu.Unlock()
	if p.cancel != nil {
		p.cancel()
		return nil, ctx.Err()
	}
	return p.delegate.Compile(ctx, cc)
}

func (p *fallbackFixtureProbe) Generate(ctx context.Context, compiled *prompt.CompilationResult, _ ...string) (*jitconfig.EffectiveAgentRuntimeConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.configCalls++
	if compiled == nil {
		p.violations = append(p.violations, "nil fallback compilation reached config")
		return nil, errors.New("nil fallback compilation")
	}
	if err := checkFallbackBaseline(compiled.Prompt); err != nil {
		p.violations = append(p.violations, err.Error())
		return nil, err
	}
	if compiled.TotalTokens <= 0 {
		p.violations = append(p.violations, "fallback token accounting missing")
	}
	return &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: slices.Clone(p.expectedTools)}, nil
}

func (p *fallbackFixtureProbe) observe(system string, definitions []types.ToolDefinition) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests++
	if err := checkFallbackBaseline(system); err != nil {
		p.violations = append(p.violations, err.Error())
		return err
	}
	var names []string
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	slices.Sort(names)
	if !slices.Equal(names, p.expectedTools) {
		err := fmt.Errorf("worker offer=%v want=%v", names, p.expectedTools)
		p.violations = append(p.violations, err.Error())
		return err
	}
	return nil
}

func newFallbackWitnessFixture(t *testing.T, compiler session.JITCompiler) (*verifiedOrchestratorFixture, *fallbackFixtureProbe) {
	t.Helper()
	p := &fallbackFixtureProbe{delegate: compiler, expectedTools: []string{"write_file"}}
	var worker session.JITCompiler = p
	if compiler == nil {
		worker = nil
	}
	f := newVerifiedOrchestratorFixtureWithRuntime(t, worker, p)
	f.model.workerRequestObserver = p.observe
	return f, p
}

func assertFallbackWitness(t *testing.T, f *verifiedOrchestratorFixture) {
	t.Helper()
	if body, err := os.ReadFile(filepath.Join(f.root, "calc.go")); err != nil || string(body) != fixtureFixedSum {
		t.Fatalf("fallback did not leave the actual fix: %q %v", body, err)
	}
	if body, err := os.ReadFile(filepath.Join(f.root, "calc_test.go")); err != nil || string(body) != fixtureSumTests {
		t.Fatalf("fallback changed its protected oracle: %q %v", body, err)
	}
	report, err := inlinePersistedWitness(f.root)
	if err != nil {
		t.Fatal(err)
	}
	output, err := fixtureGoTest(t, f.root)
	if err != nil || !strings.Contains(string(output), "PASS: TestSum") {
		t.Fatalf("independent fallback witness failed: %v\n%s", err, output)
	}
	t.Logf("fallback current acceptance: before=%s after=%s baseline=%s witness=%s command=%v", report.Before, report.After, report.Baseline[0].Status, report.Witnesses[0].Status, report.Witnesses[0].Command)
}

func setupVerifiedJITFailureEnvironment(t *testing.T, ctx context.Context, compiler session.JITCompiler) (*session.Executor, *session.JITExecutor, context.Context) {
	t.Helper()
	global := tools.Global()
	root, write := global.WorkspaceRoot(), global.Get("write_file")
	f, p := newFallbackWitnessFixture(t, compiler)
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	t.Cleanup(cancel)
	t.Cleanup(func() {
		p.mu.Lock()
		compiles, configs, requests := p.compileCalls, p.configCalls, p.requests
		violations := slices.Clone(p.violations)
		p.mu.Unlock()
		if compiles == 0 || configs == 0 || requests == 0 || len(violations) != 0 {
			t.Errorf("degraded request boundaries: compile=%d config=%d worker=%d violations=%v", compiles, configs, requests, violations)
		}
		assertFallbackWitness(t, f)
		if tools.Global() != global || global.WorkspaceRoot() != root || global.Get("write_file") != write {
			t.Error("fallback fixture changed another registry owner")
		}
	})
	return f.executor, f.jit, f.withContract(ctx)
}

func TestJITFallbackWitnessControls(t *testing.T) {
	for _, mode := range []string{"ordinary_error", "absent_compiler", "cancel_in_compile", "mandatory_budget", "empty_capability", "red_test", "missing_role"} {
		t.Run(mode, func(t *testing.T) {
			global := tools.Global()
			root, write := global.WorkspaceRoot(), global.Get("write_file")
			var compiler session.JITCompiler = &oeMockJITCompiler{errToReturn: fmt.Errorf("JIT failed")}
			if mode == "absent_compiler" {
				compiler = nil
			}
			f, p := newFallbackWitnessFixture(t, compiler)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			switch mode {
			case "cancel_in_compile":
				p.cancel = cancel
			case "mandatory_budget":
				cfg := session.DefaultExecutorConfig()
				cfg.WorkspaceRoot = f.root
				cfg.TokenBudget = 1
				f.executor.SetConfig(cfg)
			case "empty_capability":
				p.expectedTools = nil
				f.executor.SetAgentConfig(&jitconfig.EffectiveAgentRuntimeConfig{})
			case "red_test":
				f.model.content = fixtureBrokenSum + "// remains broken\n"
			case "missing_role":
				f.executor.SetRolePrompts(nil)
			}
			sentinel := filepath.Join(t.TempDir(), "unrelated.go")
			if err := os.WriteFile(sentinel, []byte("another owner\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "empty_capability" {
				// CloneForTask intentionally clears a parent's precompiled config.
				// Exercise its actual owner directly, as subagent execution does.
				result, processErr := f.executor.ProcessWithIntent(f.withContract(ctx), "correct Sum in calc.go", &perception.Intent{Verb: "/fix", Target: "calc.go"})
				p.mu.Lock()
				compiles, configs, requests := p.compileCalls, p.configCalls, p.requests
				violations := slices.Clone(p.violations)
				p.mu.Unlock()
				f.model.mu.Lock()
				emitted := f.model.emitted
				f.model.mu.Unlock()
				if result == nil || errors.Join(processErr, result.Error) == nil || result.TurnOutcome == "/done" || result.SuccessfulToolCalls != 0 || result.WriteToolEffects != 0 || (result.Acceptance != nil && result.Acceptance.Status == "verified") || compiles == 0 || configs != 0 || requests == 0 || !emitted || len(violations) != 0 {
					t.Fatalf("empty capability did not reach the real refusal: result=%+v err=%v compile=%d config=%d requests=%d emitted=%v violations=%v", result, processErr, compiles, configs, requests, emitted, violations)
				}
				for path, want := range map[string]string{filepath.Join(f.root, "calc.go"): fixtureBrokenSum, filepath.Join(f.root, "calc_test.go"): fixtureSumTests, sentinel: "another owner\n"} {
					if body, err := os.ReadFile(path); err != nil || string(body) != want {
						t.Fatalf("capability refusal changed %s: %q %v", path, body, err)
					}
				}
				if tools.Global() != global || global.WorkspaceRoot() != root || global.Get("write_file") != write {
					t.Fatal("capability control changed global ownership")
				}
				return
			}
			observed, err := f.jit.ExecuteObserved(f.withContract(ctx), session.TaskRequest{IntentVerb: "/fix", Task: "correct Sum in calc.go", Target: "calc.go"})
			p.mu.Lock()
			compiles, configs, requests := p.compileCalls, p.configCalls, p.requests
			violations := slices.Clone(p.violations)
			p.mu.Unlock()
			if len(violations) != 0 {
				t.Fatalf("fixture boundary assertion failed: %v", violations)
			}
			f.model.mu.Lock()
			workerCalls, emitted := f.model.workerCalls, f.model.emitted
			f.model.mu.Unlock()
			if mode == "ordinary_error" || mode == "absent_compiler" {
				if err != nil || !observed.Done() || observed.Stage != "behavior_verified" || observed.Acceptance == nil || observed.Acceptance.Status != "verified" || observed.Build == nil || !observed.Build.Ran || !observed.Build.OK || observed.Tests == nil || !observed.Tests.Ran || !observed.Tests.OK || configs == 0 || requests == 0 {
					t.Fatalf("fallback did not produce native witnessed execution: %+v err=%v config=%d requests=%d", observed, err, configs, requests)
				}
				if mode == "ordinary_error" && compiles == 0 {
					t.Fatal("ordinary compiler failure was not exercised")
				}
				if mode == "absent_compiler" && compiles != 0 {
					t.Fatal("absent compiler control invoked a compiler")
				}
				assertFallbackWitness(t, f)
			} else {
				if observed.Done() || (observed.Acceptance != nil && observed.Acceptance.Status == "verified") {
					t.Fatalf("%s manufactured completion: %+v err=%v", mode, observed, err)
				}
				if mode == "cancel_in_compile" || mode == "mandatory_budget" {
					if compiles == 0 || configs != 0 || requests != 0 || workerCalls != 0 || emitted {
						t.Fatalf("stop escaped compilation: compile=%d config=%d worker=%d calls=%d emitted=%v", compiles, configs, requests, workerCalls, emitted)
					}
					if mode == "cancel_in_compile" && !errors.Is(err, context.Canceled) {
						t.Fatalf("compile cancellation lost its identity: %v", err)
					}
					if mode == "mandatory_budget" && (err == nil || !strings.Contains(err.Error(), "embedded mandatory baseline")) {
						t.Fatalf("mandatory budget refusal lost its cause: %v", err)
					}
				}
				if mode == "missing_role" && !errors.Is(err, types.ErrRolePromptUnavailable) {
					t.Fatalf("role refusal lost typed identity: %v", err)
				}

				if mode == "red_test" {
					output, testErr := fixtureGoTest(t, f.root)
					if testErr == nil || !strings.Contains(string(output), "FAIL: TestSum") {
						t.Fatalf("red control unexpectedly passed: %v\n%s", testErr, output)
					}
				}
				if mode == "cancel_in_compile" || mode == "mandatory_budget" {
					body, readErr := os.ReadFile(filepath.Join(f.root, "calc.go"))
					if readErr != nil || string(body) != fixtureBrokenSum {
						t.Fatalf("refusal changed source: %q %v", body, readErr)
					}
				}
			}
			if body, readErr := os.ReadFile(filepath.Join(f.root, "calc_test.go")); readErr != nil || string(body) != fixtureSumTests {
				t.Fatalf("oracle changed: %q %v", body, readErr)
			}
			if body, readErr := os.ReadFile(sentinel); readErr != nil || string(body) != "another owner\n" {
				t.Fatalf("unrelated owner changed: %q %v", body, readErr)
			}
			if tools.Global() != global || global.WorkspaceRoot() != root || global.Get("write_file") != write {
				t.Fatal("fallback control changed global registry state")
			}
		})
	}
}
