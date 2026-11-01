package session

import (
	"context"
	"sync"
	"testing"
	"time"

	appconfig "codenerd/internal/config"
	"codenerd/internal/experts"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

type escalationOwnerKey struct{}

type escalationContextRecord struct {
	model, provider, effort string
	sampling                types.Sampling
	owner                   any
	deadline                time.Time
	bounded                 bool
}

type escalationContextConsultant struct {
	mu      sync.Mutex
	records []escalationContextRecord
}

func (c *escalationContextConsultant) SpawnConsultation(ctx context.Context, _, _ string) (string, error) {
	r := escalationRecord(ctx)
	c.mu.Lock()
	c.records = append(c.records, r)
	c.mu.Unlock()
	return "offline configured consultation", nil
}

func escalationRecord(ctx context.Context) escalationContextRecord {
	r := escalationContextRecord{owner: ctx.Value(escalationOwnerKey{})}
	r.model, _ = types.ModelNameFromContext(ctx)
	r.provider, _ = types.ProviderFromContext(ctx)
	r.effort, _ = types.ReasoningEffortFromContext(ctx)
	r.sampling, _ = types.SamplingFromContext(ctx)
	r.deadline, r.bounded = ctx.Deadline()
	return r
}

func configuredEscalation(profile appconfig.ShardProfile) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context { return appconfig.ShardProfileContext(ctx, profile) }
}

func escalationSpawner(e *Executor) *Spawner {
	s := NewSpawner(e.kernel, e.virtualStore, e.llmClient, e.jitCompiler, e.configFactory, e.transducer, DefaultSpawnerConfig())
	cfg := e.configSnapshot()
	s.SetExecutorConfig(&cfg)
	s.SetPlannerClient(e.plannerClient)
	s.SetShardProfileContext(e.shardProfileContext)
	return s
}

func spawnEscalationFixture(t *testing.T, s *Spawner) *Executor {
	t.Helper()
	agent, err := s.Spawn(context.Background(), SpawnRequest{Name: "coder", Task: "offline configuration control", Type: SubAgentTypeEphemeral, IntentVerb: "/explain", IntentTarget: "a.go"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { agent.finish("", nil); s.Remove(agent.GetID()) })
	return agent.executor
}

func TestConsultEscalationPropagation_InlineAndIsolated(t *testing.T) {
	strong := appconfig.ShardProfile{Model: "configured-strong-model", Provider: "strong-provider", ReasoningEffort: "high", Temperature: .25, TopP: .75}
	for _, tc := range []struct {
		name string
		cfg  *appconfig.UserConfig
		want bool
	}{
		{"configured", &appconfig.UserConfig{ShardProfiles: map[string]appconfig.ShardProfile{appconfig.ConsultationEscalationProfile: strong}}, true},
		{"absent", &appconfig.UserConfig{}, false},
		{"default-only", &appconfig.UserConfig{DefaultShard: &strong}, false},
		{"empty-explicit", &appconfig.UserConfig{ShardProfiles: map[string]appconfig.ShardProfile{appconfig.ConsultationEscalationProfile: {}}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, _ := routeFixture(t)
			parent.SetPlannerClient(&MockLLMClient{})
			worker := appconfig.ShardProfile{Model: "worker-model", Provider: "worker-provider", ReasoningEffort: "low", Temperature: .6, TopP: .9}
			parent.SetShardProfileContext(func(ctx context.Context, _ string) context.Context { return appconfig.ShardProfileContext(ctx, worker) })
			if profile, ok := tc.cfg.ExplicitShardProfile(appconfig.ConsultationEscalationProfile); ok {
				parent.SetConsultEscalation(configuredEscalation(profile))
			}
			s := escalationSpawner(parent)
			j := NewJITExecutor(parent, s, parent.transducer)
			inline, isolated := j.executor.CloneForTask(), spawnEscalationFixture(t, j.spawner)
			if inline.consultAttempts != parent.consultAttempts || isolated.consultAttempts == parent.consultAttempts {
				t.Fatal("configuration propagation changed consultation history ownership")
			}

			for name, executor := range map[string]*Executor{"inline": inline, "isolated": isolated} {
				t.Run(name, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), escalationOwnerKey{}, "caller-owner"), time.Hour)
					defer cancel()
					ctx = appconfig.ShardProfileContext(ctx, worker)
					ordinary := escalationRecord(ctx)
					if executor.HasConsultEscalation() != tc.want {
						t.Errorf("configured escalation = %v, want %v", executor.HasConsultEscalation(), tc.want)
					}
					if executor.llmClient != parent.llmClient || executor.plannerClient != parent.plannerClient {
						t.Fatal("profile propagation replaced an owned serving or planner client")
					}
					if got := escalationRecord(executor.withShardProfile(context.Background(), "/coder")); got.model != worker.Model || got.provider != worker.Provider {
						t.Errorf("ordinary worker profile changed: %+v", got)
					}
					reader := &escalationContextConsultant{}
					executor.SetExpertConsultant(reader)
					for _, complexity := range []string{"", "high"} {
						sel := experts.Selection{Complexity: complexity, Needs: []experts.Need{{Agent: "leadexpert", Spawn: experts.SpawnLead, Files: []string{"a.go"}}}}
						executor.withExpertConsultations(ctx, "offline prompt", perception.Intent{Verb: "/fix", Target: "a.go"}, "one scoped question", sel, &ExecutionResult{}, nil)
					}
					reader.mu.Lock()
					defer reader.mu.Unlock()
					if len(reader.records) != 2 {
						t.Fatalf("consultation routing made %d calls", len(reader.records))
					}
					if reader.records[0] != ordinary {
						t.Errorf("ordinary consultation changed profile: %+v, want %+v", reader.records[0], ordinary)
					}
					got := reader.records[1]
					want := ordinary
					if tc.want {
						want = escalationRecord(appconfig.ShardProfileContext(ctx, strong))
					}
					if got != want {
						t.Errorf("consultation received %+v, want %+v", got, want)
					}
					child := executor.escalatedContext(ctx)
					cancel()
					if child.Err() != context.Canceled {
						t.Error("configured profile detached caller cancellation")
					}
				})
			}
		})
	}
}

func TestConsultEscalationPropagation_CompositionSnapshots(t *testing.T) {
	parent, _ := routeFixture(t)
	profile := func(model string) func(context.Context) context.Context {
		return configuredEscalation(appconfig.ShardProfile{Model: model})
	}
	s := escalationSpawner(parent)
	parent.SetConsultEscalation(profile("first-profile"))
	NewJITExecutor(parent, s, parent.transducer)
	first := spawnEscalationFixture(t, s)
	parent.SetConsultEscalation(profile("second-profile"))
	NewJITExecutor(parent, s, parent.transducer)
	second := spawnEscalationFixture(t, s)
	parent.SetConsultEscalation(nil)
	NewJITExecutor(parent, s, parent.transducer)
	third := spawnEscalationFixture(t, s)
	if a, b := escalationRecord(first.escalatedContext(context.Background())), escalationRecord(second.escalatedContext(context.Background())); a.model != "first-profile" || b.model != "second-profile" || third.HasConsultEscalation() {
		t.Fatalf("profile snapshot/update/clear: first=%+v second=%+v third=%v", a, b, third.HasConsultEscalation())
	}
	if NewJITExecutor(nil, nil, nil) == nil || NewJITExecutor(parent, nil, nil) == nil || NewJITExecutor(nil, s, nil) == nil {
		t.Fatal("nil dependency construction changed")
	}
}

func TestConsultEscalationPropagation_ConcurrentCompositionAndSpawn(t *testing.T) {
	parent, _ := routeFixture(t)
	s := escalationSpawner(parent)
	s.configFactory = nil // This fixture tests handoff, not config generation.
	profiles := []appconfig.ShardProfile{
		{Model: "profile-a", Provider: "provider-a", Temperature: .2, TopP: .8},
		{Model: "profile-b", Provider: "provider-b", Temperature: .3, TopP: .9},
	}
	parent.SetConsultEscalation(configuredEscalation(profiles[0]))
	NewJITExecutor(parent, s, nil)
	var wg sync.WaitGroup
	for worker := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 64 {
				parent.SetConsultEscalation(configuredEscalation(profiles[worker]))
				NewJITExecutor(parent, s, nil)
			}
		}()
	}
	t.Cleanup(wg.Wait)
	for range 64 {
		agent, err := s.Spawn(context.Background(), SpawnRequest{Name: "coder", Type: SubAgentTypeEphemeral})
		if err != nil {
			t.Fatal(err)
		}
		got := escalationRecord(agent.executor.escalatedContext(context.Background()))
		valid := false
		for _, profile := range profiles {
			valid = valid || got == escalationRecord(appconfig.ShardProfileContext(context.Background(), profile))
		}
		agent.finish("", nil)
		s.Remove(agent.GetID())
		if !valid {
			t.Fatalf("spawn did not receive one complete configured snapshot: %+v", got)
		}
	}
	wg.Wait()
}
