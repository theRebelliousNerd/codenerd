package session

import (
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/usage"
)

// turn_cost is read from the tracker carried by the turn context. The campaign
// path never tagged its context, so every spawned-shard turn logged
// prompt=0 completion=0 even after the LLM adapter started metering
// (campaign 5a2f4c8d, 2026-09-04). The executor now owns the tracker and tags
// its own context.
func TestExecutorMeteredContext_UsesOwnedTracker(t *testing.T) {
	tr, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// Close before t.TempDir is removed: the auto-save timer otherwise
	// fires into a deleted directory and logs a warning into whatever test
	// runs next (it broke TestDebugSelf_ProjectsProblemsLogIntoKernel).
	t.Cleanup(func() { _ = tr.Close() })
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	e.SetSessionID("s1")
	e.SetUsageTracker(tr)

	tr.Record(usage.WithSessionID(context.Background(), "s1"), usage.Report{Model: "m", Provider: "meta", Operation: "chat", Usage: types.UsageMetadata{InputTokens: 100, OutputTokens: 7}})

	ctx := e.meteredContext(usage.WithSessionID(context.Background(), "s1"))
	got := snapshotTurnUsage(ctx, "s1")
	if got.prompt != 100 || got.completion != 7 {
		t.Fatalf("snapshotTurnUsage = %+v, want prompt 100 completion 7", got)
	}
}

func TestExecutorMeteredContext_KeepsExistingTracker(t *testing.T) {
	owned, _ := usage.NewTracker(t.TempDir())
	other, _ := usage.NewTracker(t.TempDir())
	t.Cleanup(func() { _ = owned.Close(); _ = other.Close() })
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	e.SetUsageTracker(owned)

	ctx := e.meteredContext(usage.NewContext(context.Background(), other))
	if usage.FromContext(ctx) != other {
		t.Fatal("a tracker already on the context must win over the executor's own")
	}
}

func TestExecutorMeteredContext_NilTrackerLeavesContext(t *testing.T) {
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	ctx := context.Background()
	if e.meteredContext(ctx) != ctx {
		t.Fatal("with no tracker the context must be returned unchanged")
	}
}

func TestExecutorCloneForTask_CopiesUsageTracker(t *testing.T) {
	tr, _ := usage.NewTracker(t.TempDir())
	t.Cleanup(func() { _ = tr.Close() })
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	e.SetUsageTracker(tr)
	if e.CloneForTask().usageTracker != tr {
		t.Fatal("CloneForTask must carry the usage tracker to the clone")
	}
}

// Reports are emitted at the same context boundary as provider adapters, not
// directly assigned to buckets. Before turn attribution these persisted shard
// rows are absent (or nested work is charged to its parent).
func TestShardUsage_ProductionEntryPathsPersistIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, path, verb, shardName, shardType string
		reports                                int64
		fail                                   bool
	}{
		{"coder", "preset", "/fix", "coder", "coder", 3, true},
		{"tester", "preset", "/test", "tester", "tester", 3, true},
		{"reviewer", "preset", "/review", "reviewer", "reviewer", 3, false},
		{"interactive", "interactive", "/review", "reviewer", "reviewer", 3, true},
		{"inline", "inline", "/review", "reviewer", "reviewer", 3, true},
		{"consultation", "consult", "/consult/goexpert", "goexpert", "goexpert", 4, true},
		{"async tester", "async", "/test", "tester", "tester", 4, true},
		{"named worker", "worker", "/review", "review-worker", "reviewer", 4, true},
		{"worker without preset", "worker", "", "custom-worker", "custom-worker", 5, true},
		{"custom agent", "preset", "/goexpert", "goexpert", "goexpert", 3, true},
		{"custom async agent", "async", "/goexpert", "goexpert", "goexpert", 4, true},
		{"custom intent spawn", "intent spawn", "/goexpert", "goexpert", "goexpert", 4, true},
		{"builtin intent spawn", "intent spawn", "/test", "tester", "tester", 4, false},
		{"builtin intent spawn provider failure", "intent spawn", "/review", "reviewer", "reviewer", 4, true},
		{"non persona intent spawn", "intent spawn", "/explain", "executor", "executor", 4, false},
		{"empty intent spawn", "intent spawn", "", "executor", "executor", 5, true},
		{"non shard", "preset", "/explain", "unknown", "unknown", 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			tr, err := usage.NewTracker(ws, usage.WithEventLog())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := tr.Close(); err != nil {
					t.Error(err)
				}
			})
			report := func(ctx context.Context, operation string) {
				usage.RecordFromContext(ctx, usage.Report{Model: "gpt-4o", Provider: "openai", Operation: operation, Usage: types.UsageMetadata{InputTokens: 11, OutputTokens: 2}})
			}
			providerErr := errors.New("provider failed after reporting usage")
			complete := func(ctx context.Context) (string, error) {
				report(ctx, "response")
				if tc.fail {
					return "", providerErr
				}
				return "No findings.", nil
			}
			llm := &MockLLMClient{
				CompleteWithSystemFunc: func(ctx context.Context, _, _ string) (string, error) { return complete(ctx) },
				CompleteWithToolsFunc: func(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
					text, err := complete(ctx)
					return &types.LLMToolResponse{Text: text}, err
				},
			}
			compiler := &MockJITCompiler{CompileFunc: func(ctx context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				report(ctx, "compile")
				return &prompt.CompilationResult{Prompt: "Review the task."}, nil
			}}
			factory := &MockConfigFactory{GenerateFunc: func(ctx context.Context, _ *prompt.CompilationResult, _ ...string) (*config.EffectiveAgentRuntimeConfig, error) {
				report(ctx, "config")
				return &config.EffectiveAgentRuntimeConfig{}, nil
			}}
			transducer := &MockTransducer{ParseIntentWithContextFunc: func(ctx context.Context, _ string, _ []perception.ConversationTurn) (perception.Intent, error) {
				report(ctx, "perception")
				verb := tc.verb
				if verb == "" {
					verb = "/review"
				}
				return perception.Intent{Verb: verb, Category: "/query"}, nil
			}}
			kernel := &MockKernel{}
			e := NewExecutor(kernel, &MockVirtualStore{}, llm, compiler, factory, transducer)
			e.SetSessionID("owned-session")
			e.SetUsageTracker(tr)
			cfg := DefaultExecutorConfig()
			cfg.WorkspaceRoot = ws
			e.SetConfig(cfg)
			sp := NewSpawner(kernel, &MockVirtualStore{}, llm, compiler, factory, transducer, DefaultSpawnerConfig())
			sp.SetUsageTracker(tr)
			sp.SetSessionID("owned-session")
			sp.SetExecutorConfig(&cfg)
			j := NewJITExecutor(e, sp, transducer)
			ctx, cancel := context.WithTimeout(usage.WithSessionID(context.Background(), "caller-session"), 10*time.Second)
			defer cancel()
			// Use only the owned meter for execution; setup calls must receive it
			// too. A separate outside report must never acquire the child's scope.
			report(usage.NewContext(ctx, tr), "outside")
			if tc.path == "consult" || tc.path == "worker" || tc.path == "async" || tc.path == "inline" {
				ctx = withShardUsage(ctx, "/fix", "parent-coder")
			}
			switch tc.path {
			case "interactive":
				_, err = e.Process(ctx, "Review the task")
			case "inline":
				_, err = j.ExecuteObserved(ctx, TaskRequest{IntentVerb: tc.verb, Task: "Review the task"})
			case "consult":
				_, err = j.SpawnConsultation(ctx, "goexpert", "Review the task")
			case "async":
				var id string
				id, err = j.ExecuteAsync(ctx, TaskRequest{IntentVerb: tc.verb, Task: "Review the task"})
				if err == nil {
					_, err = j.WaitForResult(ctx, id)
				}
			case "worker", "intent spawn":
				var agent *SubAgent
				if tc.path == "intent spawn" {
					ctx = withShardUsage(ctx, "/fix", "parent-coder")
					agent, err = sp.SpawnForIntent(ctx, perception.Intent{Verb: tc.verb, Category: "/query"}, "Review the task")
				} else {
					agent, err = sp.Spawn(ctx, SpawnRequest{Name: tc.shardName, Type: SubAgentTypeEphemeral, IntentVerb: tc.verb})
				}
				if err == nil {
					agent.Run(ctx, "Review the task")
					_, err = agent.WaitWithContext(ctx)
				}
			default:
				_, err = e.ProcessWithIntent(ctx, "Review the task", &perception.Intent{Verb: tc.verb, Category: "/query"})
			}
			if tc.fail {
				if !errors.Is(err, providerErr) {
					t.Fatalf("error=%v, want provider failure", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if err := tr.Flush(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(ws, ".nerd", "usage.json"))
			if err != nil {
				t.Fatal(err)
			}
			var persisted usage.UsageData
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatal(err)
			}
			a := persisted.Aggregate
			wantOutside := int64(13)
			if tc.path == "interactive" {
				wantOutside += 13
			}
			wantShard := tc.reports * 13
			if tc.shardType == "unknown" {
				wantShard += wantOutside
			}
			if got := a.ByShardType[tc.shardType].Total; got != wantShard {
				t.Errorf("by_shard_type[%q]=%d, want %d", tc.shardType, got, wantShard)
			}
			if got := a.ByShardName[tc.shardName].Total; got != wantShard {
				t.Errorf("by_shard_name[%q]=%d, want %d", tc.shardName, got, wantShard)
			}
			if tc.shardType != "unknown" && a.ByShardType["unknown"].Total != wantOutside {
				t.Errorf("outside usage=%+v, want %d", a.ByShardType["unknown"], wantOutside)
			}
			if got := a.TotalProject.Total; got != tc.reports*13+wantOutside {
				t.Errorf("project=%d", got)
			}
			for label, rows := range map[string]map[string]usage.TokenCounts{"type": a.ByShardType, "name": a.ByShardName} {
				var total int64
				var cost float64
				for _, row := range rows {
					total += row.Total
					cost += row.Cost
				}
				if total != a.TotalProject.Total || math.Abs(cost-a.TotalProject.Cost) > 1e-12 {
					t.Errorf("%s totals do not reconcile", label)
				}
			}
			if a.ByShardName["parent-coder"].Total != 0 {
				t.Error("child calls charged to parent")
			}
			if a.BySession["caller-session"].Total != a.TotalProject.Total {
				t.Error("session identity was replaced")
			}
			if a.TotalProject.Cost <= 0 {
				t.Error("priced calls must retain cost")
			}
		})
	}
}

func TestShardUsage_ConcurrentTurnsDoNotShareIdentity(t *testing.T) {
	tr, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tr.Close(); err != nil {
			t.Error(err)
		}
	})
	providerErr := errors.New("provider failure")
	complete := func(ctx context.Context) (string, error) {
		usage.RecordFromContext(ctx, usage.Report{Model: "gpt-4o", Provider: "openai", Operation: "response", Usage: types.UsageMetadata{InputTokens: 11, OutputTokens: 2}})
		return "", providerErr
	}
	llm := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, _, _ string) (string, error) { return complete(ctx) },
		CompleteWithToolsFunc: func(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
			_, err := complete(ctx)
			return nil, err
		},
	}
	personas := map[string]string{"/fix": "coder", "/test": "tester", "/review": "reviewer", "/consult/goexpert": "goexpert"}
	t.Run("turns", func(t *testing.T) {
		for verb, name := range personas {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				e := NewExecutor(&MockKernel{}, &MockVirtualStore{}, llm, &MockJITCompiler{}, &MockConfigFactory{}, nil)
				e.SetSessionID("shared-session")
				e.SetUsageTracker(tr)
				cfg := DefaultExecutorConfig()
				cfg.WorkspaceRoot = t.TempDir()
				e.SetConfig(cfg)
				_, err := e.ProcessWithIntent(context.Background(), "Inspect the change", &perception.Intent{Verb: verb, Category: "/query"})
				if !errors.Is(err, providerErr) {
					t.Fatalf("error=%v", err)
				}
			})
		}
	})
	a := tr.Stats()
	for _, name := range personas {
		if a.ByShardName[name].Total != 13 || a.ByShardType[name].Total != 13 {
			t.Errorf("%s: name=%+v type=%+v", name, a.ByShardName[name], a.ByShardType[name])
		}
	}
	if a.TotalProject.Total != 52 || a.BySession["shared-session"].Total != 52 || a.ByShardType["unknown"].Total != 0 {
		t.Fatalf("shared accounting=%+v", a)
	}
}

func TestShardUsage_ContextBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, verb, worker, wantName, wantType string
	}{
		{"nil unscoped", "/explain", "", "", ""},
		{"generic worker", "", "worker", "worker", "worker"},
		{"persona", "/fix", "", "coder", "coder"},
		{"named persona", "/review", "review-worker", "review-worker", "reviewer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := withShardUsage(nil, tc.verb, tc.worker)
			name, typ := usage.ShardFromContext(ctx)
			if name != tc.wantName || typ != tc.wantType {
				t.Fatalf("identity=(%q,%q)", name, typ)
			}
		})
	}
	parent := usage.WithTurnID(usage.WithSessionID(context.Background(), "session"), "turn")
	parent = withShardUsage(parent, "/fix", "coder-worker")
	child := withShardUsage(parent, "/fix", "coder")
	child = withShardUsage(child, "/fix", "")
	if name, typ := usage.ShardFromContext(child); name != "coder" || typ != "coder" {
		t.Fatalf("child=(%q,%q)", name, typ)
	}
	if name, _ := usage.ShardFromContext(parent); name != "coder-worker" {
		t.Fatal("child mutated parent")
	}
	if usage.SessionIDFromContext(child) != "session" || usage.TurnIDFromContext(child) != "turn" {
		t.Fatal("lost session/turn scope")
	}
}

// A critic changes billing purpose, not the shard that owns the turn. Drive
// an actual editing turn so follow-up rounds and the post-edit review must
// inherit identity without the test pre-tagging the context.
func TestShardUsage_EditingTurnIncludesFollowupsAndCritic(t *testing.T) {
	for _, failCritic := range []bool{false, true} {
		t.Run(fmt.Sprintf("critic failure=%v", failCritic), func(t *testing.T) {
			t.Parallel()
			tr, err := usage.NewTracker(t.TempDir(), usage.WithEventLog())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := tr.Close(); err != nil {
					t.Error(err)
				}
			})
			h := newRepairHarness(t, func(e *Executor) { e.SetUsageTracker(tr) })
			for name, content := range map[string]string{
				"main.go":      strings.Replace(closureDouble, "return x * 2", "return x + x", 1),
				"main_test.go": closureTestDouble,
			} {
				if err := os.WriteFile(filepath.Join(h.ws, name), []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			report := func(ctx context.Context, operation string) {
				usage.RecordFromContext(ctx, usage.Report{Model: "gpt-4o", Provider: "openai", Operation: operation, Usage: types.UsageMetadata{InputTokens: 11, OutputTokens: 2}})
			}
			initial := func() *types.LLMToolResponse {
				return &types.LLMToolResponse{Text: "updating Double", ToolCalls: []types.ToolCall{
					h.writeCall("double", "write_file", filepath.Join(h.ws, "main.go"), closureDouble),
				}}
			}
			h.executor.llmClient = &MockToolResultsLLM{
				MockLLMClient: &MockLLMClient{
					CompleteWithSystemFunc: func(ctx context.Context, sys, _ string) (string, error) {
						if sys == fakeCriticPrompt {
							report(ctx, "critic")
							if failCritic {
								return "", errors.New("critic provider failed after usage")
							}
							return "NO FINDINGS", nil
						}
						report(ctx, "planning")
						return "", nil
					},
					CompleteWithToolsFunc: func(ctx context.Context, _, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
						report(ctx, "initial")
						return initial(), nil
					},
				},
				CompleteWithToolResultsFunc: func(ctx context.Context, _ string, history []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
					report(ctx, "followup")
					for _, m := range history {
						if len(m.ToolResults) > 0 {
							return &types.LLMToolResponse{Text: "Updated Double; the tests pass."}, nil
						}
					}
					return initial(), nil
				},
			}
			result, err := h.drive(t, "fix Double to multiply by two")
			if err != nil {
				t.Fatalf("editing turn: %v", err)
			}
			if result == nil {
				t.Fatal("missing turn result")
			}
			var critics, followups int
			for _, event := range tr.Events() {
				if event.ShardName != "coder" || event.ShardType != "coder" {
					t.Errorf("%s attributed to (%q,%q), want coder", event.OperationType, event.ShardName, event.ShardType)
				}
				if event.OperationType == "critic" {
					critics++
				}
				if event.OperationType == "followup" {
					followups++
				}
			}
			if critics != 1 || followups == 0 {
				t.Fatalf("review/follow-up did not run: critics=%d followups=%d", critics, followups)
			}
			a := tr.Stats()
			if a.ByShardType["coder"].Total != a.TotalProject.Total || a.ByShardName["coder"].Total != a.TotalProject.Total || a.TotalProject.Total == 0 {
				t.Fatal("turn usage does not reconcile under coder")
			}
		})
	}
}
