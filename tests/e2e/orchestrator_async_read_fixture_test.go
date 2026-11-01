//go:build integration

package e2e_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/broker"
	appconfig "codenerd/internal/config"
	"codenerd/internal/core"
	jitconfig "codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/system"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

type asyncReadCompiler struct {
	mu        sync.Mutex
	available []string
	logf      func(string, ...any)
}

func (c *asyncReadCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.logf != nil {
		c.logf("async compile intent=%s tools=%v", cc.IntentVerb, cc.AvailableTools)
	}
	if !slices.Contains(cc.AvailableTools, "read_file") {
		return nil, fmt.Errorf("research catalog mismatch: %v", cc.AvailableTools)
	}
	c.mu.Lock()
	c.available = slices.Clone(cc.AvailableTools)
	c.mu.Unlock()
	return &prompt.CompilationResult{Prompt: "Read the requested fixture file and report the actual observation."}, nil
}

func (c *asyncReadCompiler) Generate(ctx context.Context, _ *prompt.CompilationResult, _ ...string) (*jitconfig.EffectiveAgentRuntimeConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logf != nil {
		c.logf("async config tools=%v", c.available)
	}
	return &jitconfig.EffectiveAgentRuntimeConfig{AllowedTools: slices.Clone(c.available)}, nil
}

// The model knows the target path, never its fresh contents. Its answer must
// come from the provider-facing result of an actual production read_file call.
type asyncReadModel struct {
	*oeMockLLMClient
	mu       sync.Mutex
	path     string
	delay    time.Duration
	issued   bool
	calls    int
	observed string
}

func (m *asyncReadModel) pause(ctx context.Context) error {
	if m.delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(m.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (m *asyncReadModel) first(ctx context.Context) (*types.LLMToolResponse, error) {
	if err := m.pause(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.issued {
		return nil, errors.New("async read script requires its paired tool result")
	}
	m.issued = true
	return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "owned-async-read", Name: "read_file", Input: map[string]any{"path": m.path}}}}, nil
}

func (m *asyncReadModel) CompleteWithTools(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.first(ctx)
}
func (m *asyncReadModel) ToolCallWithSystem(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return m.first(ctx)
}
func (m *asyncReadModel) CompleteWithToolResults(ctx context.Context, _ string, messages []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if err := m.pause(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if !m.issued {
		m.issued = true
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "owned-async-read", Name: "read_file", Input: map[string]any{"path": m.path}}}}, nil
	}
	if m.calls > 6 {
		return nil, errors.New("async read script exhausted")
	}
	for _, message := range messages {
		for _, result := range message.ToolResults {
			if result.ToolUseID == "owned-async-read" {
				if result.IsError {
					return nil, fmt.Errorf("actual read failed: %s", result.Content)
				}
				m.observed = result.Content
				return &types.LLMToolResponse{Text: "Observed the requested file:\n" + result.Content}, nil
			}
		}
	}
	return nil, errors.New("provider request omitted the paired read result")
}

func (m *asyncReadModel) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	if err := m.pause(ctx); err != nil {
		return "", err
	}
	if broker.PhaseFromContext(ctx) == broker.PhaseAdmissionAudit {
		return "COMPLETE", nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.observed == "" {
		return "", errors.New("no observed read result")
	}
	return "Observed the requested file:\n" + m.observed, nil
}

type asyncReadFixture struct {
	root, path, marker, body string
	executor                 *session.Executor
	spawner                  *session.Spawner
	jit                      *session.JITExecutor
	model                    *asyncReadModel
	registry                 *tools.Registry
	reads                    atomic.Int32
}

func newAsyncReadFixture(t *testing.T, delay time.Duration) *asyncReadFixture {
	t.Helper()
	f := &asyncReadFixture{root: t.TempDir()}
	f.path = filepath.Join(f.root, "observation.txt")
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	f.marker = "observation-" + hex.EncodeToString(nonce[:])
	f.body = f.marker + "\n"
	if err := os.WriteFile(f.path, []byte(f.body), 0600); err != nil {
		t.Fatal(err)
	}
	kernel, err := core.NewRealKernelWithWorkspace(f.root)
	if err != nil {
		t.Fatal(err)
	}
	vcfg := core.DefaultVirtualStoreConfig()
	vcfg.WorkingDir, vcfg.WorkspaceRoot = f.root, f.root
	vstore := core.NewVirtualStoreWithConfig(nil, vcfg)
	wireDreamer(vstore, kernel)
	f.registry = tools.NewRegistry()
	f.registry.SetWorkspaceRoot(f.root)
	f.registry.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"read_file"}})
	read := toolscore.ReadFileTool()
	read.Effect = tools.EffectRead
	execute := read.Execute
	read.Execute = func(ctx context.Context, args map[string]any) (string, error) {
		f.reads.Add(1)
		return execute(ctx, args)
	}
	if err := f.registry.Register(read); err != nil {
		t.Fatal(err)
	}
	f.model = &asyncReadModel{oeMockLLMClient: &oeMockLLMClient{}, path: f.path, delay: delay}
	compiler := &asyncReadCompiler{logf: t.Logf}
	transducer := &oerMockTransducer{intentToReturn: "/research"}
	f.executor = session.NewExecutor(kernel, vstore, f.model, compiler, compiler, transducer)
	cfg := session.DefaultExecutorConfig()
	cfg.WorkspaceRoot = f.root
	if !cfg.EnableSafetyGate || !cfg.VerifyBuildAfterEdits || !cfg.VerifyTestsAfterEdits || !cfg.CriticReviewAfterEdits || !cfg.AuditFinalReport {
		t.Fatal("production verification defaults changed")
	}
	f.executor.SetConfig(cfg)
	f.executor.SetModularTools(f.registry)
	f.spawner = session.NewSpawner(kernel, vstore, f.model, compiler, compiler, transducer, session.DefaultSpawnerConfig())
	f.spawner.SetExecutorConfig(&cfg)
	roleKernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	roleCompiler, err := prompt.NewJITPromptCompiler(prompt.WithKernel(system.NewKernelAdapter(roleKernel)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := roleCompiler.Close(); err != nil {
			t.Error(err)
		}
	})
	roles, err := prompt.NewRoleCompiler(roleCompiler, appconfig.DefaultJITConfig())
	if err != nil {
		t.Fatal(err)
	}
	f.executor.SetRolePrompts(roles)
	f.spawner.SetRolePrompts(roles)
	f.jit = session.NewJITExecutor(f.executor, f.spawner, transducer)
	return f
}

func setupAsyncReadEnvironment(t *testing.T, delay time.Duration) (*session.Executor, *session.JITExecutor) {
	t.Helper()
	global := tools.Global()
	root, read := global.WorkspaceRoot(), global.Get("read_file")
	f := newAsyncReadFixture(t, delay)
	t.Cleanup(func() {
		if f.reads.Load() != 1 {
			t.Errorf("expected one actual private read, got %d", f.reads.Load())
		}
		f.model.mu.Lock()
		observed := f.model.observed
		f.model.mu.Unlock()
		if !strings.Contains(observed, f.marker) {
			t.Error("model did not receive the fresh marker through a tool result")
		}
		if body, err := os.ReadFile(f.path); err != nil || string(body) != f.body {
			t.Errorf("read-only fixture changed: %q %v", body, err)
		}
		if tools.Global() != global || global.WorkspaceRoot() != root || global.Get("read_file") != read {
			t.Error("async fixture changed another registry owner")
		}
		if active := f.spawner.ListActive(); len(active) != 0 {
			t.Errorf("async task did not finish: %v", active)
		}
	})
	return f.executor, f.jit
}

func TestAsyncResearchReadExecution(t *testing.T) {
	f := newAsyncReadFixture(t, time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	id, err := f.jit.ExecuteAsync(ctx, session.TaskRequest{IntentVerb: "/research", Task: "read observation.txt", Target: "observation.txt"})
	if err != nil {
		t.Fatal(err)
	}
	answer, err := f.jit.WaitForResult(ctx, id)
	if err != nil || !strings.Contains(answer, f.marker) || f.reads.Load() != 1 {
		t.Fatalf("actual async read: answer=%q err=%v reads=%d", answer, err, f.reads.Load())
	}
}
