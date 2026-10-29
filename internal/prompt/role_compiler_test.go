package prompt

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

type recordingCompiler struct {
	got    *CompilationContext
	result *CompilationResult
	err    error
}

func (c *recordingCompiler) Compile(_ context.Context, cc *CompilationContext) (*CompilationResult, error) {
	c.got = cc
	return c.result, c.err
}

func TestRoleCompiler_CompilesUnderTheRolesRegime(t *testing.T) {
	fake := &recordingCompiler{result: &CompilationResult{Prompt: "role prompt"}}
	jit := config.DefaultJITConfig()
	jit.TokenBudget = 12345
	rc, err := NewRoleCompiler(fake, jit)
	if err != nil {
		t.Fatalf("NewRoleCompiler: %v", err)
	}
	got, err := rc.CompileRolePrompt(context.Background(), types.RolePromptRequest{Role: "/critic", Intent: "judge", SemanticQuery: "q"})
	if err != nil || got != "role prompt" {
		t.Fatalf("CompileRolePrompt = %q, %v", got, err)
	}
	if fake.got.ShardType != "/critic" || fake.got.IntentVerb != "/judge" || fake.got.SemanticQuery != "q" || fake.got.TokenBudget != 12345 {
		t.Errorf("compile context = shard %q intent %q query %q budget %d", fake.got.ShardType, fake.got.IntentVerb, fake.got.SemanticQuery, fake.got.TokenBudget)
	}
}

func TestRoleCompiler_FailuresAreUnavailable(t *testing.T) {
	if _, err := NewRoleCompiler(nil, config.DefaultJITConfig()); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("nil compiler: err = %v", err)
	}
	for name, fake := range map[string]*recordingCompiler{
		"compile error": {err: errors.New("boom")},
		"empty prompt":  {result: &CompilationResult{Prompt: "  "}},
		"nil result":    {},
	} {
		rc, err := NewRoleCompiler(fake, config.DefaultJITConfig())
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rc.CompileRolePrompt(context.Background(), types.RolePromptRequest{Role: "critic"}); !errors.Is(err, types.ErrRolePromptUnavailable) {
			t.Errorf("%s: err = %v, want ErrRolePromptUnavailable", name, err)
		}
	}
	rc, _ := NewRoleCompiler(&recordingCompiler{result: &CompilationResult{Prompt: "x"}}, config.DefaultJITConfig())
	if _, err := rc.CompileRolePrompt(context.Background(), types.RolePromptRequest{}); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("no role: err = %v", err)
	}
}
