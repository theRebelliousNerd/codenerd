package autopoiesis

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// TODO P2: "Unify Yaegi vs binary execution policy (config switch + docs)."
//
// Two policies used to coexist: OuroborosLoop always ran compiled binaries,
// while yaegi_executor.go declared DefaultSafeExecutionConfig() with
// ExecuteInterpreted as "the safe default" — read by nothing. On top of that
// the interpreter kept its own import allowlist that omitted "context", so it
// could not have run a single tool this pipeline produces even if it had been
// wired. One policy now, on OuroborosConfig, with the interpreter deriving its
// allowlist from the SafetyChecker.

func TestExecutionPolicy_WhenUnconfigured_ShouldDefaultToCompiledBinaries(t *testing.T) {
	cfg := DefaultOuroborosConfig(t.TempDir())
	if cfg.ExecutionMode != ExecuteCompiled {
		t.Errorf("default ExecutionMode = %s, want compiled", cfg.ExecutionMode)
	}
	if cfg.AllowCompilationFallback {
		t.Error("fallback is only meaningful in interpreted mode; it should be off by default")
	}
	if got := ExecuteCompiled.String(); got != "compiled" {
		t.Errorf("ExecuteCompiled.String() = %q", got)
	}
	if got := ExecuteInterpreted.String(); got != "interpreted" {
		t.Errorf("ExecuteInterpreted.String() = %q", got)
	}
}

func TestYaegiExecutor_WhenBuiltFromSafetyPolicy_ShouldShareTheImportAllowlist(t *testing.T) {
	checkerCfg := OuroborosConfig{AllowFileSystem: true, AllowNetworking: true, AllowExec: true}
	checker := NewSafetyChecker(checkerCfg)
	ye := NewYaegiExecutorForPolicy(checker.allowedPkgs)
	allowed := ye.AllowedPackages()

	// Everything the compiled path permits and the interpreter can safely host.
	for _, pkg := range []string{"fmt", "strings", "encoding/json", "time"} {
		if !slices.Contains(allowed, pkg) {
			t.Errorf("%q is on the safety allowlist but not on the interpreter's", pkg)
		}
	}
	// context is structurally required by the entry-point contract.
	if !slices.Contains(allowed, "context") {
		t.Error("context missing: no tool matching the compiler's entry-point contract could run")
	}
	// Ambient-authority packages are stripped even when the compiled path
	// grants them, because the interpreter has no process boundary.
	for _, pkg := range []string{"os", "os/exec", "net", "net/http", "net/url"} {
		if slices.Contains(allowed, pkg) {
			t.Errorf("%q reached the in-process interpreter allowlist", pkg)
		}
	}
}

func TestYaegiExecutor_WhenToolUsesPipelineEntryPoint_ShouldRunIt(t *testing.T) {
	ye := NewYaegiExecutor()
	code := `package main

import (
	"context"
	"strconv"
	"strings"
)

func CountLines(ctx context.Context, input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "0", nil
	}
	return strconv.Itoa(len(strings.Split(input, "\n"))), nil
}
`
	out, err := ye.ExecuteToolCode(context.Background(), code, "a\nb\nc")
	if err != nil {
		t.Fatalf("interpreter could not run a tool in the pipeline's own entry-point shape: %v", err)
	}
	if out != "3" {
		t.Errorf("output = %q, want 3", out)
	}
}

func TestYaegiExecutor_WhenToolImportsForbiddenPackage_ShouldRefuse(t *testing.T) {
	ye := NewYaegiExecutor()
	code := `package main

import (
	"context"
	"os/exec"
)

func Run(ctx context.Context, input string) (string, error) {
	_ = exec.Command("whoami")
	return "", nil
}
`
	if _, err := ye.ExecuteToolCode(context.Background(), code, ""); err == nil {
		t.Fatal("interpreter ran a tool importing os/exec")
	} else if !strings.Contains(err.Error(), "forbidden imports") {
		t.Errorf("error = %v, want a forbidden-imports refusal", err)
	}
}

func TestExecuteTool_WhenInterpretedModeSelected_ShouldRunFromSourceWithoutABinary(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.ExecutionMode = ExecuteInterpreted
	cfg.EnableThunderdome = false
	cfg.WorkspaceRoot = ""

	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)

	if err := os.MkdirAll(cfg.ToolsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package main

import (
	"context"
	"strings"
)

func Shout(ctx context.Context, input string) (string, error) {
	return strings.ToUpper(input), nil
}
`
	if err := os.WriteFile(filepath.Join(cfg.ToolsDir, "shout.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	// Registered with no binary at all: only the interpreted path can serve it.
	handle, err := loop.registry.Register(
		&GeneratedTool{Name: "shout", Description: "uppercase"},
		&CompileResult{Success: true, OutputPath: "", Hash: "deadbeef"},
	)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if handle.BinaryPath != "" {
		t.Fatalf("fixture should have no binary, got %q", handle.BinaryPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := loop.ExecuteTool(ctx, "shout", "hello")
	if err != nil {
		t.Fatalf("interpreted execution failed: %v", err)
	}
	if out != "HELLO" {
		t.Errorf("output = %q, want HELLO", out)
	}
}

func TestExecuteTool_WhenInterpretedSourceMissing_ShouldFailUnlessFallbackAllowed(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.ExecutionMode = ExecuteInterpreted
	cfg.EnableThunderdome = false
	cfg.WorkspaceRoot = ""

	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)
	if _, err := loop.registry.Register(
		&GeneratedTool{Name: "ghost"},
		&CompileResult{Success: true, OutputPath: "", Hash: "cafe"},
	); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := loop.ExecuteTool(context.Background(), "ghost", "x")
	if err == nil {
		t.Fatal("expected a failure: there is no source and no binary")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Errorf("error should explain the missing source, got %v", err)
	}
}
