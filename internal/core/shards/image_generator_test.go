package shards

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

type stubImageLLM struct {
	response string
	err      error
	delay    time.Duration
}

func (s *stubImageLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return s.CompleteWithSystem(ctx, "", prompt)
}

func (s *stubImageLLM) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
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

func (s *stubImageLLM) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error, 1)
	close(ch)
	errCh <- errors.New("not implemented")
	return ch, errCh
}

func (s *stubImageLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, errors.New("not implemented")
}

func TestImageGeneratorAgent_RequiresClient(t *testing.T) {
	agent := NewImageGeneratorAgent("img-1", DefaultImageGeneratorConfig("image_generator"))
	_, err := agent.Execute(context.Background(), "draw a square")
	if err == nil || !strings.Contains(err.Error(), "gemini_api_key") {
		t.Fatalf("expected missing client error, got %v", err)
	}
}

func TestImageGeneratorAgent_CallsLLM(t *testing.T) {
	agent := NewImageGeneratorAgent("img-2", DefaultImageGeneratorConfig("image_generator"))
	agent.SetLLMClient(&stubImageLLM{response: "image-ok"})
	res, err := agent.Execute(context.Background(), "tiny green checkmark")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res != "image-ok" {
		t.Fatalf("got %q", res)
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
		a := NewImageGeneratorAgent(id, cfg)
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
