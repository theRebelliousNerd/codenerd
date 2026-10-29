package shards

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

type stubImageLLM struct {
	response     string
	err          error
	delay        time.Duration
	calls        int
	system, user string
}

func (s *stubImageLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return s.CompleteWithSystem(ctx, "", prompt)
}

func (s *stubImageLLM) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	s.calls++
	s.system, s.user = system, user
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(s.delay):
		}
	}
	if s.err != nil {
		return "", s.err
	}
	return s.response, nil
}

func (s *stubImageLLM) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan types.StreamResult) {
	return types.FailedStream(errors.New("not implemented"))
}

func (s *stubImageLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, errors.New("not implemented")
}

// stubRolePrompts compiles a fixed prompt and records the role asked for.
type stubRolePrompts struct {
	prompt string
	err    error
	roles  []string
}

func (s *stubRolePrompts) CompileRolePrompt(_ context.Context, req types.RolePromptRequest) (string, error) {
	s.roles = append(s.roles, req.Role)
	return s.prompt, s.err
}

func imageRolePrompts() *stubRolePrompts {
	return &stubRolePrompts{prompt: "You are an image generation assistant."}
}

func TestImageGeneratorAgent_RequiresClient(t *testing.T) {
	agent := NewImageGeneratorAgent("img-1", DefaultImageGeneratorConfig("image_generator"), imageRolePrompts())
	_, err := agent.Execute(context.Background(), "draw a square")
	if err == nil || !strings.Contains(err.Error(), "gemini_api_key") {
		t.Fatalf("expected missing client error, got %v", err)
	}
}

func TestImageGeneratorAgent_CallsLLM(t *testing.T) {
	roles := imageRolePrompts()
	llm := &stubImageLLM{response: "image-ok"}
	agent := NewImageGeneratorAgent("img-2", DefaultImageGeneratorConfig("image_generator"), roles)
	agent.SetLLMClient(llm)
	res, err := agent.Execute(context.Background(), "tiny green checkmark")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res != "image-ok" {
		t.Fatalf("got %q", res)
	}
	if len(roles.roles) != 1 || roles.roles[0] != "image_generator" {
		t.Fatalf("compiled roles = %v, want [image_generator]", roles.roles)
	}
	if llm.system != roles.prompt || llm.user != "tiny green checkmark" {
		t.Fatalf("model asked under system %q with user %q; want the compiled role prompt and the task", llm.system, llm.user)
	}
}

// Without a compiled role prompt the image model is not asked: there is no
// embedded prompt to ask it under instead.
func TestImageGeneratorAgent_NoRolePromptIsAnError(t *testing.T) {
	for name, roles := range map[string]types.RolePromptCompiler{
		"nil compiler":   nil,
		"failed compile": &stubRolePrompts{err: fmt.Errorf("%w: compile failed", types.ErrRolePromptUnavailable)},
	} {
		t.Run(name, func(t *testing.T) {
			llm := &stubImageLLM{response: "image-ok"}
			agent := NewImageGeneratorAgent("img-3", DefaultImageGeneratorConfig("image_generator"), roles)
			agent.SetLLMClient(llm)
			if _, err := agent.Execute(context.Background(), "draw"); !errors.Is(err, types.ErrRolePromptUnavailable) {
				t.Fatalf("err = %v, want ErrRolePromptUnavailable", err)
			}
			if llm.calls != 0 {
				t.Fatalf("the image model was asked %d times without a compiled prompt", llm.calls)
			}
		})
	}
}

func TestSpawnAsync_ImageRequiresClient(t *testing.T) {
	sm := NewShardManager()
	// No SetImageLLMClient — must fail closed.
	_, err := sm.SpawnAsyncWithContext(context.Background(), "image_generator", "draw", nil)
	if err == nil || !strings.Contains(err.Error(), "Nano Banana") {
		t.Fatalf("expected image client error, got %v", err)
	}
}

func TestSpawn_ImageWithClientCompletes(t *testing.T) {
	sm := NewShardManager()
	sm.SetImageLLMClient(&stubImageLLM{response: "done"})
	sm.RegisterShard("image_generator", func(id string, cfg types.ShardConfig) types.ShardAgent {
		a := NewImageGeneratorAgent(id, cfg, imageRolePrompts())
		return a
	})
	sm.DefineProfile("image_generator", DefaultImageGeneratorConfig("image_generator"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := sm.Spawn(ctx, "image_generator", "minimal icon")
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if res != "done" {
		t.Fatalf("got %q want done", res)
	}
}
