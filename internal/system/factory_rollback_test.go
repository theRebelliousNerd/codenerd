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
	"codenerd/internal/sessionlog"
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
	if want := "boot late rollback probe: " + forcedErr.Error(); err.Error() != want {
		t.Fatalf("rollback added an unexpected cleanup failure: %v", err)
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

func TestBoot_RegistersServiceHealthBeforeShardStartup(t *testing.T) {
	previous := sessionlog.Current()
	sessionlog.Install(nil)
	t.Cleanup(func() { sessionlog.Install(previous) })
	bctx := &bootContext{workspace: t.TempDir(), appCfg: config.DefaultUserConfig(), cfg: BootConfig{SessionID: "resident-boot"}}
	defer func() {
		if bctx.sessionLog != nil {
			bctx.sessionLog.Close()
		}
	}()
	for _, step := range defaultBootSteps() {
		if step.name == "session log" {
			if err := step.run(bctx); err != nil {
				t.Fatal(err)
			}
		}
		if step.name == "shard management" {
			if sessionlog.Current() == nil || bctx.sessionID != "resident-boot" {
				t.Fatal("resident shards start before health registry exists")
			}
			if _, err := sessionlog.RunService(context.Background(), "watcher", "boot", "watch", func(context.Context) (string, error) { return "", nil }); err != nil {
				t.Fatal(err)
			}
			if got := sessionlog.Current().Services(); len(got) != 1 || got[0].Name != "watcher" {
				t.Fatalf("boot service not registered: %+v", got)
			}
			return
		}
	}
	t.Fatal("missing shard startup step")
}

func TestBoot_FinalExecutorsPreserveResidentServiceSession(t *testing.T) {
	for _, sessionID := range []string{"", "resident-resumed"} {
		name := "minted"
		if sessionID != "" {
			name = "resumed"
		}
		t.Run(name, func(t *testing.T) {
			previous := sessionlog.Current()
			sessionlog.Install(nil)
			t.Cleanup(func() { sessionlog.Install(previous) })
			cfg := config.DefaultUserConfig()
			cfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
			steps := defaultBootSteps()
			reached := false
			var residentSession string
			for i, step := range steps {
				if step.name != "final executors" {
					continue
				}
				finalize := step.run
				steps[i].run = func(bctx *bootContext) error {
					reached = true
					residentSession = bctx.sessionID
					before := bctx.sessionLog
					if before == nil || residentSession == "" {
						t.Fatal("resident service startup has no session log or identity")
					}
					// Keep the original lease owned even if finalization replaces it.
					defer func() {
						if bctx.sessionLog != before {
							before.Close()
						}
					}()
					if _, err := sessionlog.RunService(context.Background(), "watcher", "boot", "watch", func(context.Context) (string, error) { return "", nil }); err != nil {
						return err
					}
					if err := finalize(bctx); err != nil {
						return err
					}
					if bctx.sessionID != residentSession {
						t.Errorf("final executors replaced resident session %q with %q", residentSession, bctx.sessionID)
					}
					if bctx.sessionLog != before {
						t.Error("final executors restarted the resident service session log/lease")
					}
					if got := bctx.sessionExecutor.SessionID(); got != residentSession {
						t.Errorf("task executor session = %q, want resident session %q", got, residentSession)
					}
					if got := sessionlog.Current().Services(); len(got) != 1 || got[0].Name != "watcher" {
						t.Errorf("final executors lost resident health: %+v", got)
					}
					return nil
				}
			}
			cortex, err := bootCortexWithSteps(context.Background(), BootConfig{
				Workspace: t.TempDir(), SessionID: sessionID,
				DisableSystemShards: []string{"constitution_gate", "perception_firewall", "executive_policy", "session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator"},
				UserConfigOverride:  cfg, LLMClientOverride: &MockLLMClient{}, KernelOverride: &MockSystemKernel{},
			}, steps)
			if err != nil {
				t.Fatalf("boot: %v", err)
			}
			t.Cleanup(func() {
				if err := cortex.Close(); err != nil {
					t.Errorf("close cortex: %v", err)
				}
			})
			if !reached {
				t.Fatal("boot did not reach final executors")
			}
			if got := cortex.SessionID(); got != residentSession {
				t.Errorf("Cortex session = %q, want resident session %q", got, residentSession)
			}
			if sessionID == "" && !strings.HasPrefix(residentSession, "session-") {
				t.Errorf("default resident session was not minted: %q", residentSession)
			}
		})
	}
}
