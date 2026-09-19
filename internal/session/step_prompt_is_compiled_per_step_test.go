package session

import (
	"context"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// stepPromptRecorder is a stepScriptProvider that also records the system
// prompt each step's pass was sent under, keyed by the step's file.
type stepPromptRecorder struct {
	*stepScriptProvider
	mu      sync.Mutex
	systems map[string][]string
}

func (p *stepPromptRecorder) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.mu.Lock()
	file := currentStepFile(history)
	p.systems[file] = append(p.systems[file], system)
	p.mu.Unlock()
	return p.stepScriptProvider.CompleteWithToolResults(ctx, system, history, defs)
}

// Every planned step runs under a prompt compiled for THAT step, when the step
// starts: aimed at the step's file and carrying the file's language, so the
// selector serves the /mangle corpus to a policy-file step and the Go corpus to
// a Go step. Observed 2026-09-18: a three-step plan whose second step edited
// internal/context/working_set.mg ran all three steps on the prompt compiled
// for the task's first sentence, with none of the corpus's 119 /mangle atoms.
func TestPlannedStep_RunsUnderAPromptCompiledForItsOwnFileAndLanguage(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP rules/policy.mg :: add the rule\nSTEP tools/gen.py :: read the new fact\n"
	inner := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"rules/policy.mg": {{ID: "w-mg", Name: writeTool, Input: map[string]any{"path": "rules/policy.mg", "content": "p(1)."}}},
		"tools/gen.py":    {{ID: "w-go", Name: writeTool, Input: map[string]any{"path": "tools/gen.py", "content": "print(1)"}}},
	})
	client := &stepPromptRecorder{stepScriptProvider: inner, systems: map[string][]string{}}
	e := newPlannedStepsExecutor(t, client)

	var compiles []prompt.CompilationContext
	e.jitCompiler = &MockJITCompiler{
		CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			compiles = append(compiles, *cc)
			return &prompt.CompilationResult{Prompt: "COMPILED target=" + cc.IntentTarget + " language=" + cc.Language}, nil
		},
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	_, toolErrs, err := e.runToolLoop(context.Background(), "TURN PROMPT", "add the rule and read it",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe", Language: "/python"}, result)
	if err != nil {
		t.Fatalf("runToolLoop: %v (tool errors: %q)", err, toolErrs)
	}

	want := map[string]string{"rules/policy.mg": "/mangle", "tools/gen.py": "/python"}
	for file, lang := range want {
		compiled := false
		for _, cc := range compiles {
			if cc.IntentTarget == file && cc.Language == lang {
				compiled = true
			}
		}
		if !compiled {
			t.Errorf("no prompt was compiled for step %s with language %s; compiles: %+v", file, lang, compiles)
		}
		systems := client.systems[file]
		if len(systems) == 0 {
			t.Fatalf("step %s never reached the model", file)
		}
		for _, system := range systems {
			if !strings.Contains(system, "COMPILED target="+file+" language="+lang) {
				t.Errorf("step %s ran under a prompt that was not compiled for it:\n%s", file, system)
			}
			if strings.Contains(system, "TURN PROMPT") {
				t.Errorf("step %s ran under the turn's prompt, selected before the step existed", file)
			}
		}
	}
}

// A step whose prompt cannot be compiled runs on the turn's prompt: never on less.
func TestPlannedStep_KeepsTheTurnsPromptWhenItsOwnDoesNotCompile(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: first\nSTEP b.txt :: second\n"
	inner := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "a"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "b"}}},
	})
	client := &stepPromptRecorder{stepScriptProvider: inner, systems: map[string][]string{}}
	e := newPlannedStepsExecutor(t, client)
	e.jitCompiler = &MockJITCompiler{
		CompileFunc: func(context.Context, *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return nil, context.DeadlineExceeded
		},
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	if _, _, err := e.runToolLoop(context.Background(), "TURN PROMPT", "write both",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	for _, file := range []string{"a.txt", "b.txt"} {
		for _, system := range client.systems[file] {
			if !strings.Contains(system, "TURN PROMPT") {
				t.Errorf("step %s lost the turn's prompt when its own compile failed:\n%s", file, system)
			}
		}
	}
}

// A turn aimed at a policy file compiles for /mangle even in a Go project: the
// file the turn edits outranks the project's language, or the corpus's /mangle
// atoms are never candidates for the one turn that needs them.
func TestTurnAimedAtAPolicyFileCompilesForMangle(t *testing.T) {
	e := &Executor{}
	for target, want := range map[string]string{
		"internal/context/working_set.mg": "/mangle",
		"internal/session/executor.go":    "/go",
		"notes.txt":                       "",
	} {
		cc := e.buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix", Target: target})
		if cc.Language != want {
			t.Errorf("target %s compiled for language %q, want %q", target, cc.Language, want)
		}
	}
}

// The focused file's context is carried once per request. A working loop
// renders it into every request itself (fresh, so the outline's line ranges
// follow the edits); a compiled step prompt that also appended it sent the same
// section twice on every call. Observed 2026-09-18 once the section carried the
// target's outline: mean input per call rose from 37.5k to 44.7k tokens.
func TestPlannedStep_FileContextIsSentOncePerRequest(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: first\nSTEP b.txt :: second\n"
	inner := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "a"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "b"}}},
	})
	client := &stepPromptRecorder{stepScriptProvider: inner, systems: map[string][]string{}}
	e := newPlannedStepsExecutor(t, client)
	e.fileContext = &stubFileContext{section: "## Holographic Context (focused)\n\nbody"}
	e.jitCompiler = &MockJITCompiler{
		CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{Prompt: "COMPILED " + cc.IntentTarget}, nil
		},
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	if _, _, err := e.runToolLoop(context.Background(), "TURN PROMPT", "write both",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	for file, systems := range client.systems {
		for _, system := range systems {
			if n := strings.Count(system, "## Holographic Context"); n != 1 {
				t.Errorf("a request for step %s carried the file context %d times, want exactly once", file, n)
			}
		}
	}
}
