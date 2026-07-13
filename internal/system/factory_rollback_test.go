package system

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
)

type closeTrackingEmbedding struct {
	closed atomic.Bool
}

func (*closeTrackingEmbedding) Embed(context.Context, string) ([]float32, error) {
	return nil, nil
}

func (*closeTrackingEmbedding) EmbedBatch(context.Context, []string) ([][]float32, error) {
	return nil, nil
}

func (*closeTrackingEmbedding) Dimensions() int { return 1 }
func (*closeTrackingEmbedding) Name() string    { return "rollback-probe" }

func (e *closeTrackingEmbedding) Close() error {
	e.closed.Store(true)
	return nil
}

func TestBootCortexWithConfigLateFailureRollsBackAcquiredResources(t *testing.T) {
	workspace := t.TempDir()
	userCfg := config.DefaultUserConfig()
	userCfg.Embedding = &config.EmbeddingConfig{Provider: "none"}

	var localDB *store.LocalStore
	var learningStore *store.LearningStore
	var compiler *prompt.JITPromptCompiler
	embeddingProbe := &closeTrackingEmbedding{}
	forcedErr := errors.New("forced failure after final executors")

	steps := append(defaultBootSteps(), bootStep{
		name: "late rollback probe",
		run: func(bctx *bootContext) error {
			localDB = bctx.localDB
			learningStore = bctx.learningStore
			compiler = bctx.jitCompiler
			if learningStore == nil {
				t.Fatal("learning store was not acquired before late failure")
			}
			if err := learningStore.Save("rollback_probe", "probe_fact", []any{"value"}, "test"); err != nil {
				t.Fatalf("prime learning DB before rollback: %v", err)
			}
			// Install an owned io.Closer probe after all production steps. The
			// aggregate rollback must close optional embedding resources too.
			bctx.embeddingEngine = embeddingProbe
			return forcedErr
		},
	})

	cortex, err := bootCortexWithSteps(context.Background(), BootConfig{
		Workspace: workspace,
		APIKey:    "test-key",
		DisableSystemShards: []string{
			"constitution_gate",
			"perception_firewall",
			"executive_policy",
			"world_model_ingestor",
			"session_planner",
			"tactile_router",
			"campaign_runner",
			"mangle_repair",
			"legislator",
		},
		UserConfigOverride: userCfg,
		LLMClientOverride:  &MockLLMClient{},
		KernelOverride:     &MockSystemKernel{},
	}, steps)
	if cortex != nil {
		t.Fatal("late boot failure returned a partial Cortex")
	}
	if !errors.Is(err, forcedErr) || !strings.Contains(err.Error(), "late rollback probe") {
		t.Fatalf("boot error = %v, want named step wrapping forced error", err)
	}
	if localDB == nil || localDB.GetDB() == nil {
		t.Fatal("local DB was not acquired before late failure")
	}
	if pingErr := localDB.GetDB().Ping(); pingErr == nil {
		t.Fatal("local DB remained open after boot rollback")
	}
	if !embeddingProbe.closed.Load() {
		t.Fatal("owned embedding resource remained open after boot rollback")
	}
	if compiler == nil {
		t.Fatal("JIT compiler was not acquired before late failure")
	}
	compilerValue := reflect.ValueOf(compiler).Elem()
	if projectDB := compilerValue.FieldByName("projectDB"); !projectDB.IsNil() {
		t.Fatal("JIT project DB remained owned after boot rollback")
	}
	if shardDBs := compilerValue.FieldByName("shardDBs"); shardDBs.Len() != 0 {
		t.Fatalf("JIT shard DB count after rollback = %d, want 0", shardDBs.Len())
	}
	learningValue := reflect.ValueOf(learningStore).Elem()
	if dbs := learningValue.FieldByName("dbs"); dbs.Len() != 0 {
		t.Fatalf("learning DB count after rollback = %d, want 0", dbs.Len())
	}
}

func TestCortexCloseIsIdempotent(t *testing.T) {
	probe := &closeTrackingEmbedding{}
	cortex := &Cortex{EmbeddingEngine: probe}
	if err := cortex.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := cortex.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if !probe.closed.Load() {
		t.Fatal("Close did not release the embedding resource")
	}
}
