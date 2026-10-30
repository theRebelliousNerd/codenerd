package session

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// cappedIdentityClient reports an identity and an input cap, as a worker or
// profile-routed slot client does.
type cappedIdentityClient struct {
	identityStubClient
	inputCap int
}

func (c *cappedIdentityClient) InputWindowTokens() int { return c.inputCap }

// A shard profile that routes the turn to another provider or model is
// compiled for that client: its identity pins the atoms and its input cap
// bounds the prompt. Before, the compile, the spawned subagent's config and
// the turn record all named the slot's client -- a coder profile on
// gpt-6.1-sol behind a Meta main slot was compiled for Muse.
func TestServingResolution_FollowsTheProfileRoute(t *testing.T) {
	slot := &identityStubClient{provider: "meta", model: "muse"}
	routedTo := &cappedIdentityClient{identityStubClient: identityStubClient{provider: "openai", model: "gpt-b"}, inputCap: 1000}
	scheduled := core.NewScheduledLLMCall("serving-resolution", slot)
	scheduled.Router = func(provider, model string) (core.LLMClient, error) {
		if provider != "openai" || model != "gpt-b" {
			t.Fatalf("routed to %s/%s", provider, model)
		}
		return routedTo, nil
	}
	exec := NewExecutor(nil, nil, scheduled, nil, nil, nil)
	intent := perception.Intent{Verb: "/fix"}

	plain := exec.buildCompilationContextWith(context.Background(), intent, nil)
	if plain.Provider != "meta" || plain.Model != "muse" {
		t.Fatalf("unrouted compile pinned to %s/%s, want the slot's meta/muse", plain.Provider, plain.Model)
	}
	if plain.TokenBudget <= routedTo.inputCap {
		t.Fatalf("unrouted budget %d already at the routed cap; the test cannot tell them apart", plain.TokenBudget)
	}

	ctx := types.WithProvider(types.WithModelName(context.Background(), "gpt-b"), "openai")
	cc := exec.buildCompilationContextWith(ctx, intent, nil)
	if cc.Provider != "openai" || cc.Model != "gpt-b" {
		t.Fatalf("routed compile pinned to %s/%s, want openai/gpt-b", cc.Provider, cc.Model)
	}
	if cc.TokenBudget != routedTo.inputCap {
		t.Fatalf("routed budget %d, want the routed client's cap %d", cc.TokenBudget, routedTo.inputCap)
	}
	if p, m := exec.servingIdentity(ctx, "/fix"); p != "openai" || m != "gpt-b" {
		t.Fatalf("turn record identity %s/%s, want openai/gpt-b", p, m)
	}

	// The spawner compiles a subagent for its own profile.
	sp := NewSpawner(nil, nil, scheduled, nil, nil, nil, DefaultSpawnerConfig())
	sp.SetShardProfileContext(func(ctx context.Context, shardType string) context.Context {
		if shardType != "coder" {
			return ctx
		}
		return types.WithProvider(types.WithModelName(ctx, "gpt-b"), "openai")
	})
	if p, m := clientIdentity(sp.servingClient(context.Background(), SpawnRequest{Name: "coder"})); p != "openai" || m != "gpt-b" {
		t.Fatalf("spawned coder compiled for %s/%s, want its profile's openai/gpt-b", p, m)
	}
	if p, m := clientIdentity(sp.servingClient(context.Background(), SpawnRequest{Name: "reviewer"})); p != "meta" || m != "muse" {
		t.Fatalf("a subagent with no profile compiled for %s/%s, want the slot's meta/muse", p, m)
	}
}
