package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/prompt"
	"codenerd/internal/types"

	"github.com/stretchr/testify/require"
)

type dbOwnershipAgent struct {
	*coreshards.BaseShardAgent
	execute func(context.Context) (string, error)
}

func (a *dbOwnershipAgent) Execute(ctx context.Context, _ string) (string, error) {
	return a.execute(ctx)
}

// Uses discovery, owner registration and callback wiring from BootCortex, then
// the manager path the supervised campaign's consultation adapter calls. No
// registration callback is replaced by the test. Before the repair, spawn
// closes bootDB and completion removes coder, dropping its next compiled atom.
func TestBootOwnedShardDBSurvivesManagerConsultation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	for _, name := range []string{"coder", "unrelated"} {
		dir := filepath.Join(ws, ".nerd", "agents", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		yaml := "- id: \"" + name + "/identity\"\n  category: identity\n  is_mandatory: true\n  content: |\n    OWNERSHIP_" + name + "_MARKER\n"
		if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.DefaultUserConfig()
	cfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortexWithConfig(ctx, BootConfig{
		Workspace: ws, UserConfigOverride: cfg, LLMClientOverride: &MockLLMClient{},
		DisableSystemShards: []string{"constitution_gate", "perception_firewall", "executive_policy",
			"session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	compiler := cortex.JITCompiler
	bootDB, ok := compiler.LookupShardDB("coder")
	if !ok || bootDB == nil {
		t.Fatal("boot did not register the custom coder DB")
	}
	sibling, ok := compiler.LookupShardDB("unrelated")
	if !ok || sibling == nil {
		t.Fatal("boot did not register sibling DB")
	}
	profile, ok := cortex.ShardManager.GetProfile("coder")
	if !ok || profile.Type != types.ShardTypeUser {
		t.Fatalf("custom coder profile = %+v, present=%v", profile, ok)
	}
	consultErr := errors.New("controlled advisor failure")
	for _, failure := range []error{nil, consultErr} {
		cortex.ShardManager.RegisterShard("coder", func(id string, cfg types.ShardConfig) types.ShardAgent {
			return &dbOwnershipAgent{BaseShardAgent: coreshards.NewBaseShardAgent(id, cfg), execute: func(context.Context) (string, error) {
				current, ok := compiler.LookupShardDB("coder")
				if !ok || current != bootDB {
					return "", errors.New("spawn replaced the boot-owned coder DB")
				}
				return "advice", failure
			}}
		})
		_, err := cortex.ShardManager.Spawn(ctx, "coder", "review the campaign plan")
		if !errors.Is(err, failure) {
			t.Fatalf("consultation error = %v, want %v", err, failure)
		}
		current, ok := compiler.LookupShardDB("coder")
		if !ok || current != bootDB {
			t.Fatal("completion retired the boot-owned coder DB")
		}
		if err := bootDB.PingContext(ctx); err != nil {
			t.Fatal(err)
		}
		current, ok = compiler.LookupShardDB("unrelated")
		if !ok || current != sibling {
			t.Fatal("consultation changed sibling registration")
		}
		result, err := compiler.Compile(ctx, &prompt.CompilationContext{
			IntentVerb: "/code", OperationalMode: "/active", TokenBudget: 32000,
			ShardID: "coder", ShardType: "/coder",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Prompt, "OWNERSHIP_coder_MARKER") {
			t.Fatal("subsequent actual coder compile lost its custom atom")
		}
		if strings.Contains(result.Prompt, "OWNERSHIP_unrelated_MARKER") {
			t.Fatal("coder compile read sibling identity")
		}
	}
}

func TestManagerTemporaryDBLivesUntilOverlappingExecutionsFinish(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "cancellation"} {
		t.Run(outcome, func(t *testing.T) {
			ws := t.TempDir()
			writeAndSyncUserAgent(t, ws, "expert", "- id: expert/identity\n  category: identity\n  is_mandatory: true\n  content: OVERLAPPING_EXPERT_MARKER\n")
			compiler, err := prompt.NewJITPromptCompiler()
			require.NoError(t, err)
			defer compiler.Close()
			manager := coreshards.NewShardManager()
			manager.SetNerdDir(filepath.Join(ws, ".nerd"))
			manager.SetJITRegistrar(prompt.CreateJITDBRegistrar(compiler))
			manager.DefineProfile("expert", types.ShardConfig{Name: "expert", Type: types.ShardTypeUser})
			started := make(chan struct{}, 2)
			finish := make(chan struct{})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			// Always release the workers before compiler shutdown on an assertion failure.
			defer func() {
				select {
				case <-finish:
				default:
					close(finish)
				}
			}()
			factoryCalls := 0
			manager.RegisterShard("expert", func(id string, cfg types.ShardConfig) types.ShardAgent {
				first := factoryCalls == 0
				factoryCalls++ // manager serializes factory calls
				return &dbOwnershipAgent{BaseShardAgent: coreshards.NewBaseShardAgent(id, cfg), execute: func(runCtx context.Context) (string, error) {
					started <- struct{}{}
					if first && outcome == "cancellation" {
						<-runCtx.Done()
						return "", runCtx.Err()
					}
					<-finish
					if first && outcome == "failure" {
						return "", errors.New("controlled consultation failure")
					}
					return "advice", nil
				}}
			})
			first, err := manager.SpawnAsyncWithContext(ctx, "expert", "first consultation", nil)
			require.NoError(t, err)
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("first execution did not start")
			}
			db, ok := compiler.LookupShardDB("expert")
			require.True(t, ok)
			second, err := manager.SpawnAsyncWithContext(t.Context(), "expert", "second consultation", nil)
			require.NoError(t, err)
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("second execution did not start")
			}
			current, ok := compiler.LookupShardDB("expert")
			require.True(t, ok)
			require.Same(t, db, current, "overlapping executions must reuse the pool")
			if outcome == "cancellation" {
				cancel()
				require.Eventually(t, func() bool { _, done := manager.GetResult(first); return done }, 5*time.Second, time.Millisecond)
				require.NoError(t, db.Ping(), "canceled first execution retired the second one's pool")
			}
			close(finish)
			for _, id := range []string{first, second} {
				if id == first && outcome == "cancellation" {
					continue // GetResult above consumed this result
				}
				require.Eventually(t, func() bool { _, done := manager.GetResult(id); return done }, 5*time.Second, time.Millisecond)
			}
			_, ok = compiler.LookupShardDB("expert")
			require.False(t, ok)
			require.Error(t, db.Ping(), "final completion leaked the temporary pool")
		})
	}
}
