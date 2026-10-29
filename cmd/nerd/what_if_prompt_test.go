package main

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/types"
)

type whatIfRolePrompts struct{ got types.RolePromptRequest }

func (w *whatIfRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	w.got = req
	return "compiled what-if prompt", nil
}

// The what-if elaboration asks with the what_if_analyst role compile; with no
// compiler it is not asked.
func TestWhatIfAnalystPrompt_IsTheRoleCompile(t *testing.T) {
	roles := &whatIfRolePrompts{}
	got, err := whatIfAnalystPrompt(context.Background(), roles)
	if err != nil || got != "compiled what-if prompt" {
		t.Fatalf("whatIfAnalystPrompt = %q, %v", got, err)
	}
	if roles.got.Role != "what_if_analyst" || roles.got.Intent != "judge" {
		t.Errorf("compiled %+v, want role what_if_analyst intent judge", roles.got)
	}
	if _, err := whatIfAnalystPrompt(context.Background(), nil); !errors.Is(err, types.ErrRolePromptUnavailable) {
		t.Errorf("no compiler: err = %v, want ErrRolePromptUnavailable", err)
	}
}
