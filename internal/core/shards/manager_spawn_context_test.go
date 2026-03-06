package shards

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/types"
)

type capabilityCheckAgent struct {
	*BaseShardAgent
	want types.ModelCapability
}

func (a *capabilityCheckAgent) Execute(ctx context.Context, task string) (string, error) {
	got, ok := ctx.Value(types.CtxKeyModelCapability).(types.ModelCapability)
	if !ok {
		return "", fmt.Errorf("missing %s context value", types.CtxKeyModelCapability)
	}
	if got != a.want {
		return "", fmt.Errorf("model capability hint = %q, want %q", got, a.want)
	}
	return "ok", nil
}

type modelNameCheckAgent struct {
	*BaseShardAgent
	want string
}

func (a *modelNameCheckAgent) Execute(ctx context.Context, task string) (string, error) {
	got, ok := ctx.Value(types.CtxKeyModelName).(string)
	if !ok {
		return "", fmt.Errorf("missing %s context value", types.CtxKeyModelName)
	}
	if got != a.want {
		return "", fmt.Errorf("model name hint = %q, want %q", got, a.want)
	}
	return "ok", nil
}

func TestShardManager_ModelCapabilityContextHint(t *testing.T) {
	sm := NewShardManager()

	want := types.CapabilityHighReasoning
	sm.RegisterShard("cap_test", func(id string, cfg types.ShardConfig) types.ShardAgent {
		base := NewBaseShardAgent(id, cfg)
		return &capabilityCheckAgent{
			BaseShardAgent: base,
			want:           want,
		}
	})
	sm.DefineProfile("cap_test", types.ShardConfig{
		Name:    "cap_test",
		Type:    types.ShardTypeEphemeral,
		Timeout: 2 * time.Second,
		Model: types.ModelConfig{
			Capability: want,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := sm.SpawnWithContext(ctx, "cap_test", "task", nil)
	if err != nil {
		t.Fatalf("SpawnWithContext error: %v", err)
	}
	if res != "ok" {
		t.Fatalf("result=%q, want ok", res)
	}
}

func TestShardManager_ModelNameContextHint(t *testing.T) {
	sm := NewShardManager()

	want := "gpt-5.4"
	sm.RegisterShard("model_name_test", func(id string, cfg types.ShardConfig) types.ShardAgent {
		base := NewBaseShardAgent(id, cfg)
		return &modelNameCheckAgent{
			BaseShardAgent: base,
			want:           want,
		}
	})
	sm.DefineProfile("model_name_test", types.ShardConfig{
		Name:    "model_name_test",
		Type:    types.ShardTypeEphemeral,
		Timeout: 2 * time.Second,
		Model: types.ModelConfig{
			Name:       want,
			Capability: types.CapabilityBalanced,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := sm.SpawnWithContext(ctx, "model_name_test", "task", nil)
	if err != nil {
		t.Fatalf("SpawnWithContext error: %v", err)
	}
	if res != "ok" {
		t.Fatalf("result=%q, want ok", res)
	}
}
