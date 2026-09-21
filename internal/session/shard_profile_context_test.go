package session

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// The persona's profile reaches the turn's context on the executor path. Until
// 2026-09-21 only the TUI's delegation applied it: `nerd fix` and campaigns ran
// every persona on the serving client's model whatever shard_profiles said.
func TestExecutor_AppliesThePersonasShardProfile(t *testing.T) {
	e := &Executor{config: DefaultExecutorConfig()}
	var asked []string
	e.SetShardProfileContext(func(ctx context.Context, shardType string) context.Context {
		asked = append(asked, shardType)
		return types.WithProvider(types.WithModelName(ctx, "stealth/union-alpha"), "openrouter")
	})

	ctx := e.withShardProfile(context.Background(), "/coder")
	model, _ := types.ModelNameFromContext(ctx)
	provider, _ := types.ProviderFromContext(ctx)
	if model != "stealth/union-alpha" || provider != "openrouter" || len(asked) != 1 || asked[0] != "coder" {
		t.Fatalf("model=%q provider=%q asked=%v", model, provider, asked)
	}

	// A turn delegated with a model already chosen keeps it.
	chosen := types.WithModelName(context.Background(), "the-callers-model")
	if got, _ := types.ModelNameFromContext(e.withShardProfile(chosen, "/coder")); got != "the-callers-model" {
		t.Errorf("the caller's model was replaced by the profile's: %q", got)
	}
	// No persona, no profile.
	if _, set := types.ModelNameFromContext(e.withShardProfile(context.Background(), "")); set {
		t.Error("a turn with no shard type was given a profile")
	}
	// A clone carries the hook: subagent and campaign executors are clones or
	// fresh builds, and a hook lost there is the gap this closes.
	if (&Executor{}).shardProfileContext != nil {
		t.Fatal("zero executor has a hook")
	}
}

func TestSpawner_HandsTheProfileHookToSubagents(t *testing.T) {
	s := &Spawner{}
	if s.currentShardProfileContext() != nil {
		t.Fatal("a new spawner has a hook")
	}
	s.SetShardProfileContext(func(ctx context.Context, _ string) context.Context { return ctx })
	if s.currentShardProfileContext() == nil {
		t.Error("the spawner did not keep the hook it was given")
	}
}
