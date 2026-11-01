//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/broker"
	appconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/evidence"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/system"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

const fixtureBrokenSum = "package fixture\n\nfunc Sum(a, b int) int { return a - b }\n"
const fixtureFixedSum = "package fixture\n\nfunc Sum(a, b int) int { return a + b }\n"
const fixtureSumTests = `package fixture
import "testing"
func TestSum(t *testing.T) {
    for _, tc := range []struct{ a, b, want int }{{2,3,5},{-4,5,1},{0,0,0}} {
        if got := Sum(tc.a,tc.b); got != tc.want { t.Fatalf("Sum(%d,%d)=%d; want %d",tc.a,tc.b,got,tc.want) }
    }
}
`

type verifiedFixtureCompiler struct{}

func (verifiedFixtureCompiler) Compile(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if !slices.Equal(cc.AvailableTools, []string{"write_file"}) {
		return nil, fmt.Errorf("fixture compile/execute envelopes differ: %v", cc.AvailableTools)
	}
	return &prompt.CompilationResult{Prompt: "fixture worker"}, nil
}

// Only the model is scripted. Its one edit runs through the real registry,
// executive and validators; build/test/critic/audit remain the normal defaults.
type verifiedFixtureModel struct {
	*oeMockLLMClient
	mu          sync.Mutex
	path        string
	content     string
	emitted     bool
	workerCalls int
	auditAnswer string
	auditSystem string
	auditUser   string
}

func (m *verifiedFixtureModel) next(ctx context.Context) (*types.LLMToolResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workerCalls++
	if m.workerCalls > 16 {
		return nil, errors.New("fixture model exhausted its finite script")
	}
	if !m.emitted {
		m.emitted = true
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "fixture-sum-edit", Name: "write_file", Input: map[string]any{"path": m.path, "content": m.content}}}}, nil
	}
	return &types.LLMToolResponse{Text: "Changed Sum to add its arguments."}, nil
}

func (m *verifiedFixtureModel) CompleteWithTools(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.next(ctx)
}
func (m *verifiedFixtureModel) ToolCallWithSystem(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.next(ctx)
}
func (m *verifiedFixtureModel) CompleteWithToolResults(ctx context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.next(ctx)
}
func (m *verifiedFixtureModel) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if broker.PhaseFromContext(ctx) == broker.PhaseAdmissionAudit {
		m.auditSystem, m.auditUser = system, user
		if m.auditAnswer != "" {
			return m.auditAnswer, nil
		}
		return "COMPLETE", nil
	}
	if broker.PhaseFromContext(ctx) == broker.PhaseStepPlan {
		return "STEP calc.go :: correct addition", nil
	}
	if broker.PurposeFromContext(ctx) == broker.PurposeCritic {
		return "NO FINDINGS", nil
	}
	return "Changed Sum to add its arguments.", nil
}

type verifiedOrchestratorFixture struct {
	root     string
	executor *session.Executor
	jit      *session.JITExecutor
	model    *verifiedFixtureModel
}

func fixtureGoTest(t *testing.T, root string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "./...")
	cmd.Dir = root
	return cmd.CombinedOutput()
}

func newVerifiedOrchestratorFixture(t *testing.T) *verifiedOrchestratorFixture {
	t.Helper()
	root := t.TempDir()
	for name, body := range map[string]string{"go.mod": "module fixture.local/sum\n\ngo 1.24\n", "calc.go": fixtureBrokenSum, "calc_test.go": fixtureSumTests} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before, err := fixtureGoTest(t, root)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(before), "FAIL: TestSum") {
		t.Fatalf("fixture did not independently fail before the edit: %v\n%s", err, before)
	}
	t.Logf("independent fail-before:\n%s", before)
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vcfg := core.DefaultVirtualStoreConfig()
	vcfg.WorkingDir, vcfg.WorkspaceRoot = root, root
	vstore := core.NewVirtualStoreWithConfig(nil, vcfg)
	wireDreamer(vstore, kernel)
	registry := tools.NewRegistry()
	registry.SetWorkspaceRoot(root)
	registry.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"write_file"}})
	write := toolscore.WriteFileTool()
	write.Effect = tools.EffectWrite
	if err := registry.Register(write); err != nil {
		t.Fatal(err)
	}
	model := &verifiedFixtureModel{oeMockLLMClient: &oeMockLLMClient{}, path: filepath.Join(root, "calc.go"), content: fixtureFixedSum}
	transducer := &oeMockTransducer{intentToReturn: "/fix"}
	executor := session.NewExecutor(kernel, vstore, model, verifiedFixtureCompiler{}, nil, transducer)
	cfg := session.DefaultExecutorConfig()
	cfg.WorkspaceRoot = root
	if !cfg.EnableSafetyGate || !cfg.VerifyBuildAfterEdits || !cfg.VerifyTestsAfterEdits || !cfg.CriticReviewAfterEdits || !cfg.AuditFinalReport {
		t.Fatal("fixture requires the production verification defaults")
	}
	executor.SetConfig(cfg)
	executor.SetModularTools(registry)
	executor.SetAgentConfig(&jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file"}})
	// Role selection uses real shipped atoms and the production private-scope
	// adapter, separately from the deterministic worker prompt above.
	roleKernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := prompt.NewJITPromptCompiler(prompt.WithKernel(system.NewKernelAdapter(roleKernel)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := compiler.Close(); err != nil {
			t.Errorf("close fixture role compiler: %v", err)
		}
	})
	roles, err := prompt.NewRoleCompiler(compiler, appconfig.DefaultJITConfig())
	if err != nil {
		t.Fatal(err)
	}
	executor.SetRolePrompts(roles)
	return &verifiedOrchestratorFixture{root: root, executor: executor, jit: session.NewJITExecutor(executor, nil, transducer), model: model}
}

func (f *verifiedOrchestratorFixture) withContract(ctx context.Context) context.Context {
	return evidence.WithContract(ctx, evidence.Contract{
		Task: "correct Sum in calc.go", Authority: "fixture caller",
		Obligations: []evidence.Obligation{{ID: "sum", Description: "Sum adds both arguments", Package: ".", Test: "TestSum", TestFile: "calc_test.go", Reproducer: true}},
	})
}

func TestE2E_VerifiedOrchestratorFixture_PostWriteWitness(t *testing.T) {
	global := tools.Global()
	f := newVerifiedOrchestratorFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	result, err := f.executor.ProcessWithIntent(f.withContract(ctx), "correct Sum in calc.go", &perception.Intent{Verb: "/fix", Target: "calc.go"})
	if err != nil || result == nil || result.Error != nil {
		t.Fatalf("fixture turn failed: result=%+v err=%v", result, err)
	}
	if result.TurnOutcome != types.MangleAtom("/done") || result.WriteToolEffects != 1 || len(result.MissingEvidence) != 0 {
		t.Fatalf("turn lacks verified evidence: outcome=%s writes=%d missing=%v", result.TurnOutcome, result.WriteToolEffects, result.MissingEvidence)
	}
	if result.BuildCheck.Verdict() != session.VerifyPassed || result.TestCheck.Verdict() != session.VerifyPassed || result.TestCheck.Result == nil || len(result.TestCheck.Command) == 0 {
		t.Fatalf("native build/test witness missing: build=%+v test=%+v", result.BuildCheck, result.TestCheck)
	}
	if result.Acceptance == nil || result.Acceptance.Status != "verified" ||
		len(result.Acceptance.Baseline) != 1 || result.Acceptance.Baseline[0].Status != "failed" ||
		len(result.Acceptance.Witnesses) != 1 || result.Acceptance.Witnesses[0].Status != "passed" ||
		result.Acceptance.Before == result.Acceptance.After || result.Acceptance.Witnesses[0].Snapshot != result.Acceptance.After {
		t.Fatalf("caller-owned acceptance is not snapshot-bound: %+v", result.Acceptance)
	}
	if body, err := os.ReadFile(filepath.Join(f.root, "calc.go")); err != nil || string(body) != fixtureFixedSum {
		t.Fatalf("actual artifact differs: %q %v", body, err)
	}
	after, err := fixtureGoTest(t, f.root)
	if err != nil || !strings.Contains(string(after), "PASS: TestSum") {
		t.Fatalf("independent post-write test failed: %v\n%s", err, after)
	}
	if tools.Global() != global {
		t.Fatal("fixture replaced the process-global registry")
	}
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	contract, ok := corpus.Get("system/final_report_auditor/contract")
	if !ok || !strings.Contains(f.model.auditSystem, strings.TrimSpace(contract.Content)) || !strings.Contains(f.model.auditUser, "Changed Sum to add its arguments.") {
		t.Fatal("auditor did not receive shipped role atoms and the actual report")
	}
	report, err := json.Marshal(result.Acceptance)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("snapshot-bound acceptance JSON: %s", report)
	t.Logf("native checks: build=%v test=%v; independent post-write:\n%s", result.BuildCheck.Command, result.TestCheck.Command, after)
}

func TestE2E_VerifiedOrchestratorFixture_AdmissionControls(t *testing.T) {
	for _, mode := range []string{"red test", "missing role", "unfinished report", "verified contract admission", "outside workspace", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			f := newVerifiedOrchestratorFixture(t)
			sentinel := filepath.Join(t.TempDir(), "outside.go")
			const untouched = "unrelated work\n"
			if err := os.WriteFile(sentinel, []byte(untouched), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "red test" {
				f.model.content = fixtureBrokenSum + "// still broken\n"
			} else if mode == "missing role" {
				f.executor.SetRolePrompts(nil)
			} else if mode == "unfinished report" || mode == "verified contract admission" {
				f.model.auditAnswer = "INCOMPLETE"
			} else if mode == "outside workspace" {
				f.model.path = sentinel
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			runCtx := f.withContract(ctx)
			if mode == "unfinished report" {
				// Green mechanical gates alone do not overrule an admission.
				// A caller-owned behavior witness has a separate, stronger arm.
				runCtx = ctx
			}
			result, err := f.executor.ProcessWithIntent(runCtx, "correct Sum in calc.go", &perception.Intent{Verb: "/fix", Target: "calc.go"})
			if result == nil {
				if mode != "canceled" || !errors.Is(err, context.Canceled) {
					t.Fatalf("unexpected absent result: %v", err)
				}
			} else {
				if mode == "missing role" {
					if !errors.Is(errors.Join(err, result.Error), types.ErrRolePromptUnavailable) {
						t.Fatalf("missing role was not refused: %v %v", err, result.Error)
					}
				} else if mode == "verified contract admission" {
					if err != nil || result.Error != nil || result.TurnOutcome != types.MangleAtom("/done") || result.Acceptance == nil || result.Acceptance.Status != "verified" {
						t.Fatalf("verified caller contract lost authority: outcome=%s err=%v resultErr=%v", result.TurnOutcome, err, result.Error)
					}
				} else if result.TurnOutcome == types.MangleAtom("/done") {
					t.Fatalf("%s falsely completed: err=%v resultErr=%v", mode, err, result.Error)
				}
				if (mode == "unfinished report" || mode == "verified contract admission") && !result.SelfReportedIncomplete {
					t.Fatal("auditor admission was discarded")
				}
				if mode == "red test" && result.Acceptance != nil && result.Acceptance.Status == "verified" {
					t.Fatal("red reproducer manufactured a passing acceptance witness")
				}
			}
			if body, readErr := os.ReadFile(sentinel); readErr != nil || string(body) != untouched {
				t.Fatalf("unrelated work changed: %q %v", body, readErr)
			}
			if mode == "outside workspace" || mode == "canceled" {
				if body, readErr := os.ReadFile(filepath.Join(f.root, "calc.go")); readErr != nil || string(body) != fixtureBrokenSum {
					t.Fatalf("refused turn changed its source: %q %v", body, readErr)
				}
			}
		})
	}
}

func TestE2E_VerifiedOrchestratorFixture_ConcurrentPrivateWorkspaces(t *testing.T) {
	global := tools.Global()
	fixtures := []*verifiedOrchestratorFixture{newVerifiedOrchestratorFixture(t), newVerifiedOrchestratorFixture(t)}
	var wg sync.WaitGroup
	errors := make(chan error, len(fixtures))
	for _, f := range fixtures {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			result, err := f.executor.ProcessWithIntent(f.withContract(ctx), "correct Sum in calc.go", &perception.Intent{Verb: "/fix", Target: "calc.go"})
			if err != nil || result == nil || result.Error != nil || result.TurnOutcome != types.MangleAtom("/done") || result.Acceptance == nil || result.Acceptance.Status != "verified" {
				errors <- fmt.Errorf("private workspace %s: result=%+v err=%v", f.root, result, err)
			}
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	if tools.Global() != global {
		t.Fatal("concurrent fixtures replaced the global registry")
	}
}
