package session

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/projectdoc"
)

// The claim nerd.md makes over CLAUDE.md is that its write protection is
// enforced by the executive rather than obeyed by the model. These tests are
// that claim. If they pass while the model is told nothing, protection still
// holds; if they fail, nerd.md is just prose with extra syntax.

const protectedDoc = `---
schema: nerd/v1
project: codeNERD
forbid:
  - match: .nerd/config.json
    reason: user-owned runtime config; edit by hand only
  - match: secrets/
    reason: credentials are managed out of band
---
`

// newProtectedExecutor returns an executor whose kernel holds the doc's facts,
// mirroring what loadProjectDoc does at boot.
func newProtectedExecutor(t *testing.T) *Executor {
	t.Helper()
	doc, err := projectdoc.Parse([]byte(protectedDoc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	kernel := &MockKernel{}
	if err := kernel.LoadFacts(doc.Facts()); err != nil {
		t.Fatalf("load facts: %v", err)
	}
	e := &Executor{kernel: kernel, config: DefaultExecutorConfig()}
	e.SetProjectDoc(doc)
	return e
}

func TestProjectForbidsWrite_BlocksProtectedPaths(t *testing.T) {
	e := newProtectedExecutor(t)

	cases := []struct {
		name string
		call ToolCall
		want bool
	}{
		{"write_file to protected path", ToolCall{Name: "write_file", Args: map[string]any{"path": ".nerd/config.json"}}, true},
		{"edit_file to protected path", ToolCall{Name: "edit_file", Args: map[string]any{"file_path": ".nerd/config.json"}}, true},
		{"delete_file under protected dir", ToolCall{Name: "delete_file", Args: map[string]any{"path": "secrets/prod.env"}}, true},
		{"absolute path still blocked", ToolCall{Name: "write_file", Args: map[string]any{"path": "C:/repo/.nerd/config.json"}}, true},
		{"windows separators still blocked", ToolCall{Name: "write_file", Args: map[string]any{"path": `.nerd\config.json`}}, true},
		{"unprotected path allowed", ToolCall{Name: "write_file", Args: map[string]any{"path": "internal/session/executor.go"}}, false},

		// Reading a protected file is fine and often necessary. The rule is
		// about not editing it, and a gate that also blocked reads would push
		// the agent into guessing about the very file it must not corrupt.
		{"read_file on protected path allowed", ToolCall{Name: "read_file", Args: map[string]any{"path": ".nerd/config.json"}}, false},
		{"grep on protected path allowed", ToolCall{Name: "grep", Args: map[string]any{"path": ".nerd/config.json"}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, denied := e.projectForbidsWrite(tc.call)
			if denied != tc.want {
				t.Fatalf("projectForbidsWrite = %v, want %v", denied, tc.want)
			}
			if denied && strings.TrimSpace(reason) == "" {
				t.Error("a denial must carry the reason from nerd.md so the model can stop rather than retry")
			}
		})
	}
}

// Tools disagree about what to call the target argument. A gate that only fires
// for the one name we guessed is a gate with holes in it.
func TestProjectForbidsWrite_FindsTargetUnderEveryArgName(t *testing.T) {
	e := newProtectedExecutor(t)
	for _, key := range projectDocPathArgs {
		t.Run(key, func(t *testing.T) {
			call := ToolCall{Name: "write_file", Args: map[string]any{key: ".nerd/config.json"}}
			if _, denied := e.projectForbidsWrite(call); !denied {
				t.Errorf("a write named by %q slipped past the gate", key)
			}
		})
	}
}

// The whole loop must refuse, not just the predicate. This is the behaviour a
// user relies on when they write a forbid rule.
//
// The constitutional gate is disabled here to isolate the nerd.md gate: with a
// bare MockKernel nothing derives permitted/1, so default-deny would reject the
// call first and this test would pass for the wrong reason. That ordering is
// itself asserted below.
func TestExecuteToolCall_RefusesProtectedWrite(t *testing.T) {
	e := newProtectedExecutor(t)
	e.config.EnableSafetyGate = false
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file"}}

	_, err := e.executeToolCall(context.Background(),
		ToolCall{Name: "write_file", Args: map[string]any{"path": ".nerd/config.json", "content": "{}"}}, cfg)
	if err == nil {
		t.Fatal("a write to a nerd.md-protected path must fail")
	}
	if !strings.Contains(err.Error(), "nerd.md") {
		t.Errorf("the error must name nerd.md so the user knows which rule fired, got: %v", err)
	}
	if !strings.Contains(err.Error(), "user-owned runtime config") {
		t.Errorf("the error must carry the rule's reason, got: %v", err)
	}
}

// A project file must not be able to widen what the constitution allows. The
// nerd.md gate sits after checkSafety precisely so a workspace document can add
// restrictions but never remove them.
func TestExecuteToolCall_ConstitutionalGateOutranksProjectDoc(t *testing.T) {
	e := newProtectedExecutor(t)
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file"}}

	// An UNprotected path: nerd.md has nothing to say about it, so whatever
	// blocks this can only be the constitutional gate.
	_, err := e.executeToolCall(context.Background(),
		ToolCall{Name: "write_file", Args: map[string]any{"path": "internal/session/executor.go", "content": "x"}}, cfg)
	if err == nil {
		t.Fatal("default-deny must still apply to paths nerd.md says nothing about")
	}
	if !strings.Contains(err.Error(), "safety gate") {
		t.Errorf("the constitutional gate must be the one that fired, got: %v", err)
	}
}

// The JIT allowlist runs before both gates. A tool the agent was never granted
// is rejected on capability grounds, not on path grounds — otherwise a nerd.md
// forbid rule could read as the reason a tool is unavailable.
func TestExecuteToolCall_AllowlistOutranksProjectDoc(t *testing.T) {
	e := newProtectedExecutor(t)
	e.config.EnableSafetyGate = false
	cfg := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"read_file"}}

	_, err := e.executeToolCall(context.Background(),
		ToolCall{Name: "write_file", Args: map[string]any{"path": ".nerd/config.json"}}, cfg)
	if err == nil {
		t.Fatal("a tool outside the effective allowlist must be refused")
	}
	if !strings.Contains(err.Error(), "not allowed by effective JIT config") {
		t.Errorf("the allowlist must be the one that fired, got: %v", err)
	}
}

// Enforcement reads the kernel, not the struct field. A subagent constructed
// without SetProjectDoc must still be governed — otherwise the guarantee is
// only as good as every construction site remembering one line.
func TestProjectForbidsWrite_EnforcedWithoutThePromptCopy(t *testing.T) {
	doc, err := projectdoc.Parse([]byte(protectedDoc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	kernel := &MockKernel{}
	if err := kernel.LoadFacts(doc.Facts()); err != nil {
		t.Fatalf("load facts: %v", err)
	}
	// Deliberately no SetProjectDoc.
	e := &Executor{kernel: kernel, config: DefaultExecutorConfig()}

	call := ToolCall{Name: "write_file", Args: map[string]any{"path": ".nerd/config.json"}}
	if _, denied := e.projectForbidsWrite(call); !denied {
		t.Fatal("write protection must come from the kernel facts, not from the prompt-rendering field")
	}
	if e.withProjectInstructions("system") != "system" {
		t.Error("with no doc attached the prompt must be unchanged")
	}
}

// A kernel that cannot answer is not evidence that a path is protected. Turning
// every transient query failure into a blocked write would make the agent
// unusable the moment the kernel hiccups, so the gate fails open — but it must
// say so, or a degraded kernel silently becomes an unprotected workspace.
func TestProjectForbidsWrite_FailsOpenOnKernelError(t *testing.T) {
	e := newProtectedExecutor(t)
	e.kernel.(*MockKernel).QueryError = errors.New("kernel unavailable")

	call := ToolCall{Name: "write_file", Args: map[string]any{"path": ".nerd/config.json"}}
	if _, denied := e.projectForbidsWrite(call); denied {
		t.Error("a kernel query failure must not be read as a denial")
	}
}

func TestProjectForbidsWrite_NoDocMeansNoRestrictions(t *testing.T) {
	e := &Executor{kernel: &MockKernel{}, config: DefaultExecutorConfig()}
	call := ToolCall{Name: "write_file", Args: map[string]any{"path": ".nerd/config.json"}}
	if _, denied := e.projectForbidsWrite(call); denied {
		t.Error("a workspace with no nerd.md must have no protected paths")
	}
}

func TestWithProjectInstructions_AppendsRenderedDoc(t *testing.T) {
	e := newProtectedExecutor(t)
	got := e.withProjectInstructions("COMPILED PROMPT")

	if !strings.HasPrefix(got, "COMPILED PROMPT") {
		t.Error("the compiled prompt must be preserved verbatim at the front")
	}
	if !strings.Contains(got, ".nerd/config.json") {
		t.Error("protected paths must reach the model; learning about them by being denied mid-edit costs a whole turn")
	}
	if !strings.Contains(got, "ENFORCED") {
		t.Error("the model must be told these are enforced, not advisory")
	}
}
