//go:build integration

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tools"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

func conversationFixtureWriteCall(t *testing.T) types.ToolCall {
	t.Helper()
	return writeTurnCallIn(t.TempDir())
}

// Keep the existing intent/configuration contract, but own the real handler and
// give its target, registry and executor the same containment boundary.
func configureConversationFixture(t *testing.T, executor *session.Executor, call types.ToolCall) *tools.Registry {
	t.Helper()
	path, ok := call.Input["path"].(string)
	if !ok || path == "" {
		t.Fatal("conversation fixture requires a concrete write target")
	}
	root := filepath.Dir(path)
	registry := tools.NewRegistry()
	registry.SetWorkspaceRoot(root)
	registry.SetAllowlist(&tools.Allowlist{Enforced: true, Names: writeTurnAllowedTools()})
	if err := registry.Register(toolscore.WriteFileTool()); err != nil {
		t.Fatal(err)
	}
	cfg := writeTurnExecutorConfig(t)
	cfg.WorkspaceRoot = root
	executor.SetConfig(cfg)
	executor.SetModularTools(registry)
	return registry
}

// A private test registry makes the production-handler-first order explicit.
// The parent is nonparallel and keeps it installed until its existing parallel
// child has finished; all process-global state is then restored.
func TestConversationFixtureProductionHandlerFirst(t *testing.T) {
	global := tools.NewRegistry()
	global.SetWorkspaceRoot(t.TempDir())
	if err := global.Register(toolscore.WriteFileTool()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tools.SwapGlobal(global))
	t.Run("existing_conversation_assertions", TestE2E_CrossBoundary_Executor_MultiTurn_ConversationDrift)
}

func TestConversationFixturePrivateBindingControls(t *testing.T) {
	for _, mode := range []string{"real_write_still_unverified", "outside_workspace_refused"} {
		t.Run(mode, func(t *testing.T) {
			global := tools.Global()
			globalRoot, globalWrite := global.WorkspaceRoot(), global.Get("write_file")
			kernel, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			vs := core.NewVirtualStore(nil)
			wireDreamer(vs, kernel)
			call := conversationFixtureWriteCall(t)
			model := &cbMockLLMClient{toolResponse: &types.LLMToolResponse{Text: "working", ToolCalls: []types.ToolCall{call}}}
			compiler := &cbMockJITCompiler{result: &prompt.CompilationResult{Prompt: "test prompt"}}
			factory := &cbMockConfigFactory{cfg: &config.EffectiveAgentRuntimeConfig{AllowedTools: writeTurnAllowedTools()}}
			transducer := &cbMockTransducer{intents: []perception.Intent{{Verb: "/fix", Target: "auth.go", Category: "/mutation"}}}
			executor := session.NewExecutor(kernel, vs, model, compiler, factory, transducer)
			registry := configureConversationFixture(t, executor, call)
			if registry == global || registry.WorkspaceRoot() != filepath.Dir(call.Input["path"].(string)) {
				t.Fatal("conversation registry did not own its coherent workspace")
			}
			if mode == "outside_workspace_refused" {
				call.Input["path"] = filepath.Join(t.TempDir(), "must-not-exist.txt")
			}
			result, err := executor.Process(context.Background(), "fix the fixture")
			if result == nil {
				t.Fatalf("missing actual turn result: %v", err)
			}
			path := call.Input["path"].(string)
			if mode == "real_write_still_unverified" {
				data, readErr := os.ReadFile(path)
				if readErr != nil || string(data) != call.Input["content"].(string) || result.SuccessfulToolCalls != 1 || result.WriteToolEffects != 1 {
					t.Fatalf("private real write missing: read=%v result=%+v", readErr, result)
				}
				if err != nil || result.Error == nil || !strings.Contains(result.Error.Error(), "no role prompt compiler") || result.TurnOutcome == "/done" {
					t.Fatalf("legacy soft acceptance failure was hidden or qualified: returned=%v result=%+v", err, result)
				}
			} else {
				if err == nil || result.Error == nil || result.SuccessfulToolCalls != 0 || result.WriteToolEffects != 0 {
					t.Fatalf("outside-workspace write was not refused: returned=%v result=%+v", err, result)
				}
				if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
					t.Fatalf("outside-workspace target changed: %v", statErr)
				}
			}
			if tools.Global() != global || global.WorkspaceRoot() != globalRoot || global.Get("write_file") != globalWrite {
				t.Fatal("private conversation fixture changed the global registry")
			}
		})
	}
}
