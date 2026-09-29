package session

import (
	"context"
	"errors"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

func TestCompilationContextUsesPermittedToolEnvelope(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	for _, requestContext := range []bool{false, true} {
		suffix := map[bool]string{true: "/request-context", false: "/default-context"}[requestContext]
		ctx := t.Context()
		if requestContext {
			ctx = types.WithSessionContext(ctx, &types.SessionContext{})
		}
		t.Run("coder"+suffix, func(t *testing.T) {
			cc := (&Executor{kernel: k}).buildCompilationContext(ctx, perception.Intent{Verb: "/fix"})
			if !slices.Contains(cc.AvailableTools, "read_file") {
				t.Fatalf("coder envelope = %v, want read_file", cc.AvailableTools)
			}
			if slices.Contains(cc.AvailableTools, "bash") || slices.Contains(cc.AvailableTools, "run_command") {
				t.Fatalf("coder envelope gained a free-form shell: %v", cc.AvailableTools)
			}
		})
		t.Run("general fallback"+suffix, func(t *testing.T) {
			cc := (&Executor{kernel: k}).buildCompilationContext(ctx, perception.Intent{})
			if !slices.Contains(cc.AvailableTools, "read_file") || slices.Contains(cc.AvailableTools, "write_file") {
				t.Fatalf("empty verb envelope = %v, want the read-only floor", cc.AvailableTools)
			}
		})
		t.Run("query error"+suffix, func(t *testing.T) {
			e := &Executor{kernel: &MockKernel{QueryError: errors.New("unavailable")}}
			cc := e.buildCompilationContext(ctx, perception.Intent{Verb: "/fix"})
			if len(cc.AvailableTools) != 0 {
				t.Fatalf("query error supplied tools %v", cc.AvailableTools)
			}
		})
		t.Run("specialist"+suffix, func(t *testing.T) {
			precompiled := &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{"write_file"}}
			e := &Executor{kernel: k, EffectiveAgentRuntimeConfig: precompiled}
			cc := e.buildCompilationContext(ctx, perception.Intent{Verb: "/fix"})
			if !slices.Equal(cc.AvailableTools, []string{"write_file"}) {
				t.Fatalf("prompt envelope=%v, want the precompiled allowlist", cc.AvailableTools)
			}
			cc.AvailableTools[0] = "changed"
			if !slices.Equal(precompiled.AllowedTools, []string{"write_file"}) {
				t.Fatal("compilation context aliases specialist permissions")
			}
		})
		t.Run("empty specialist"+suffix, func(t *testing.T) {
			precompiled := &config.EffectiveAgentRuntimeConfig{}
			e := &Executor{kernel: k, EffectiveAgentRuntimeConfig: precompiled}
			cc := e.buildCompilationContext(ctx, perception.Intent{Verb: "/fix"})
			if len(cc.AvailableTools) != 0 {
				t.Fatalf("empty specialist supplied %v", cc.AvailableTools)
			}
		})
	}
	if cc := (&Executor{}).buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix"}); len(cc.AvailableTools) != 0 {
		t.Fatal("nil kernel supplied ambient tools")
	}
}

func TestBuildCompilationContext_LanguageAndFrameworksFromKernel(t *testing.T) {
	k := &MockKernel{}
	if err := k.LoadFacts([]types.Fact{
		{Predicate: "project_language", Args: []any{types.MangleAtom("/go")}},
		{Predicate: "project_framework", Args: []any{types.MangleAtom("/bubbletea")}},
	}); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	cc := (&Executor{kernel: k}).buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix"})
	if cc.Language != "/go" {
		t.Fatalf("Language=%q, want %q", cc.Language, "/go")
	}
	if !slices.Equal(cc.Frameworks, []string{"/bubbletea"}) {
		t.Fatalf("Frameworks=%v, want %v", cc.Frameworks, []string{"/bubbletea"})
	}
	ccNil := (&Executor{}).buildCompilationContext(t.Context(), perception.Intent{Verb: "/fix"})
	if ccNil.Language != "" {
		t.Fatalf("nil kernel Language=%q, want empty", ccNil.Language)
	}
	if len(ccNil.Frameworks) != 0 {
		t.Fatalf("nil kernel Frameworks=%v, want empty", ccNil.Frameworks)
	}
}

// The executor's stored session context reaches the layers that read it only
// from ctx; a caller's own attachment is left alone.
func TestExecutor_WithSessionContext(t *testing.T) {
	e := &Executor{}
	if got := types.GetSessionContext(e.withSessionContext(context.Background())); got != nil {
		t.Fatalf("an executor with no session context attached %+v", got)
	}
	stored := &types.SessionContext{DreamMode: true}
	e.SetSessionContext(stored)
	if got := types.GetSessionContext(e.withSessionContext(context.Background())); got != stored {
		t.Fatalf("the stored session context did not reach ctx: %+v", got)
	}
	caller := &types.SessionContext{}
	ctx := types.WithSessionContext(context.Background(), caller)
	if got := types.GetSessionContext(e.withSessionContext(ctx)); got != caller {
		t.Fatal("the caller's session context was replaced")
	}
}
