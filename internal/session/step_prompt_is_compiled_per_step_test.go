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
	// requests is each call's whole request text: the system prompt and every
	// turn's prose. The working section rides on the last user turn.
	requests map[string][]string
	// turns is each call's request split by part: the system prompt first,
	// then one entry per message, so a test can tell one message carrying a
	// section twice from two messages carrying it once each.
	turns map[string][][]string
}

func (p *stepPromptRecorder) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	p.mu.Lock()
	file := currentStepFile(history)
	p.systems[file] = append(p.systems[file], system)
	if p.requests != nil {
		var request strings.Builder
		request.WriteString(system)
		for _, m := range history {
			request.WriteString(m.Text)
		}
		p.requests[file] = append(p.requests[file], request.String())
	}
	if p.turns != nil {
		parts := []string{system}
		for _, m := range history {
			parts = append(parts, m.Text)
		}
		p.turns[file] = append(p.turns[file], parts)
	}
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
	// A file's language is the kernel's answer (policy/coder_language.mg).
	e.kernel = realKernel(t)

	var compiles []prompt.CompilationContext
	e.jitCompiler = &MockJITCompiler{
		CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			compiles = append(compiles, *cc)
			return &prompt.CompilationResult{Prompt: "COMPILED target=" + cc.IntentTarget + " language=" + cc.Language}, nil
		},
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	_, toolErrs, err := e.runToolLoop(context.Background(), "TURN PROMPT", "add the rule in rules/policy.mg and read it in tools/gen.py",
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
	if _, _, err := e.runToolLoop(context.Background(), "TURN PROMPT", "write a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	for _, file := range []string{"a.txt", "b.txt"} {
		if len(client.systems[file]) == 0 {
			t.Fatalf("step %s never ran: the turn was not planned", file)
		}
		for _, system := range client.systems[file] {
			if !strings.Contains(system, "TURN PROMPT") {
				t.Errorf("step %s lost the turn's prompt when its own compile failed:\n%s", file, system)
			}
		}
	}
}

// A turn aimed at a policy file compiles for /mangle even in a Go project: the
// file the turn edits outranks the project's language, or the corpus's /mangle
// atoms are never candidates for the one turn that needs them. A document gets
// a prose key, so the project's code corpus stays out of its turn; a file the
// policy has no row for keeps the project's language ("").
func TestTurnAimedAtAPolicyFileCompilesForMangle(t *testing.T) {
	t.Parallel()
	e := &Executor{kernel: realKernel(t)}
	for target, want := range map[string]string{
		"internal/context/working_set.mg": "/mangle",
		"internal/session/executor.go":    "/go",
		"Docs/journeys/05-ladder.md":      "/markdown",
		"notes.txt":                       "/text",
		"Makefile":                        "",
	} {
		cc := turnCC(e, t.Context(), perception.Intent{Verb: "/fix", Target: target})
		if cc.Language != want {
			t.Errorf("target %s compiled for language %q, want %q", target, cc.Language, want)
		}
	}
}

// The focused file's view is carried by the working ledger alone: never in
// the compiled step prompt, and never twice in one message. A working loop
// renders it into the request itself (fresh, so the outline's line ranges
// follow the edits); a compiled step prompt that also appended it sent the same
// section twice on every call. Observed 2026-09-18 once the section carried the
// target's outline: mean input per call rose from 37.5k to 44.7k tokens.
//
// The ledger appends a new view when the focus or the file changes, to the
// turn that follows the change, and keeps earlier turns as they were sent (the
// cached prefix stays put). So a request after an edit carries one view per
// revision of the file, each in its own message, and that is not duplication.
func TestPlannedStep_FileContextIsSentOncePerRequest(t *testing.T) {
	_, writeTool := registerStepTools(t)
	plan := "STEP a.txt :: first\nSTEP b.txt :: second\n"
	inner := newStepScriptProvider(plan, map[string][]types.ToolCall{
		"a.txt": {{ID: "w-a", Name: writeTool, Input: map[string]any{"path": "a.txt", "content": "a"}}},
		"b.txt": {{ID: "w-b", Name: writeTool, Input: map[string]any{"path": "b.txt", "content": "b"}}},
	})
	client := &stepPromptRecorder{stepScriptProvider: inner, systems: map[string][]string{}, requests: map[string][]string{}, turns: map[string][][]string{}}
	e := newPlannedStepsExecutor(t, client)
	e.fileContext = &stubFileContext{section: "## Holographic Context (focused)\n\nbody"}
	e.jitCompiler = &MockJITCompiler{
		CompileFunc: func(_ context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
			return &prompt.CompilationResult{Prompt: "COMPILED " + cc.IntentTarget}, nil
		},
	}

	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	if _, _, err := e.runToolLoop(context.Background(), "TURN PROMPT", "write a.txt and b.txt",
		&config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}},
		&prompt.CompilationContext{ShardID: "probe"}, result); err != nil {
		t.Fatalf("runToolLoop: %v", err)
	}
	for _, file := range []string{"a.txt", "b.txt"} {
		if len(client.requests[file]) == 0 {
			t.Fatalf("step %s never ran: the turn was not planned", file)
		}
	}
	for file, requests := range client.turns {
		for r, parts := range requests {
			if n := strings.Count(parts[0], "## Holographic Context"); n != 0 {
				t.Errorf("step %s request %d: the compiled system prompt carried the file view %d times; only the working ledger renders it", file, r, n)
			}
			total := 0
			for i, msg := range parts[1:] {
				n := strings.Count(msg, "## Holographic Context")
				if n > 1 {
					t.Errorf("step %s request %d: message %d carried the file view %d times, want at most once", file, r, i, n)
				}
				total += n
			}
			if total == 0 {
				t.Errorf("step %s request %d carried no file view", file, r)
			}
			// One view for the step's start, and one more after each edit of
			// the focus file that an earlier request of the step made.
			if limit := 1 + r; total > limit {
				t.Errorf("step %s request %d carried %d file views, more than its %d revisions", file, r, total, limit)
			}
		}
	}
}
