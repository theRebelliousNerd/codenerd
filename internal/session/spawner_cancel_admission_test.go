package session

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/jit/config"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

type spawnAdmissionKernel struct {
	MockKernel
	queries   atomic.Int32
	onQuery   func()
	onRetract func()
}

func (k *spawnAdmissionKernel) Query(q string) ([]types.Fact, error) {
	if !strings.HasPrefix(q, "turn_catalog(") {
		return k.MockKernel.Query(q)
	}
	k.queries.Add(1)
	if k.onQuery != nil {
		k.onQuery()
	}
	return []types.Fact{{Predicate: "turn_catalog", Args: []any{types.MangleAtom("/turn_fixture"), types.MangleAtom("/read_file")}}}, nil
}

func (k *spawnAdmissionKernel) RetractFact(f types.Fact) error {
	if k.onRetract != nil {
		k.onRetract()
	}
	return k.MockKernel.RetractFact(f)
}

// Dependencies deliberately return success after withdrawing their owner.
// The handoff itself must refuse; cooperative dependency errors are insufficient.
func TestSpawnerCancellationAdmission(t *testing.T) {
	for _, stage := range []string{"before_spawn", "catalog", "catalog_cleanup", "compile", "fallback_compile", "factory", "deadline_compile"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if stage == "deadline_compile" {
				var deadlineCancel context.CancelFunc
				ctx, deadlineCancel = context.WithTimeout(ctx, 20*time.Millisecond)
				defer deadlineCancel()
			}
			k := &spawnAdmissionKernel{}
			if stage == "catalog" {
				k.onQuery = cancel
			}
			if stage == "catalog_cleanup" {
				k.onRetract = cancel
			}
			if stage == "before_spawn" {
				cancel()
			}
			compiles, factories := 0, 0
			jit := &MockJITCompiler{CompileFunc: func(c context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				compiles++
				if stage == "fallback_compile" && compiles == 1 {
					return nil, errors.New("ordinary compile failure")
				}
				if stage == "compile" || stage == "fallback_compile" {
					cancel()
				}
				if stage == "deadline_compile" {
					<-c.Done()
				}
				return &prompt.CompilationResult{Prompt: "fixture"}, nil
			}}
			factory := &MockConfigFactory{GenerateFunc: func(context.Context, *prompt.CompilationResult, ...string) (*config.EffectiveAgentRuntimeConfig, error) {
				factories++
				if stage == "factory" {
					cancel()
				}
				return &config.EffectiveAgentRuntimeConfig{}, nil
			}}
			cfg := DefaultSpawnerConfig()
			cfg.MaxActiveSubagents = 1
			s := NewSpawner(k, &MockVirtualStore{}, &MockLLMClient{}, jit, factory, &MockTransducer{}, cfg)
			agent, err := s.Spawn(ctx, SpawnRequest{Name: "withdrawn", IntentVerb: "/general"})
			wantErr := context.Canceled
			if stage == "deadline_compile" {
				wantErr = context.DeadlineExceeded
			}
			if agent != nil || !errors.Is(err, wantErr) {
				t.Errorf("withdrawn spawn = (%v, %v), want nil and %v", agent != nil, err, wantErr)
			}
			if len(s.subagents) != 0 || s.pendingSpawns != 0 {
				t.Errorf("withdrawn ownership leaked: agents=%d reservations=%d", len(s.subagents), s.pendingSpawns)
			}
			wantCompiles, wantFactories := 0, 0
			switch stage {
			case "compile", "deadline_compile":
				wantCompiles = 1
			case "fallback_compile":
				wantCompiles = 2
			case "factory":
				wantCompiles, wantFactories = 1, 1
			}
			if compiles != wantCompiles || factories != wantFactories {
				t.Errorf("later stage ran: compile=%d want=%d factory=%d want=%d", compiles, wantCompiles, factories, wantFactories)
			}
			if stage == "before_spawn" && k.queries.Load() != 0 {
				t.Error("pre-canceled spawn queried the kernel")
			}
			// One capacity slot, no manual reservation or agent reset.
			k.onQuery, k.onRetract = nil, nil
			s.jitCompiler = &MockJITCompiler{}
			s.configFactory = &MockConfigFactory{}
			if live, err := s.Spawn(context.Background(), SpawnRequest{Name: "healthy", IntentVerb: "/general"}); err != nil || live == nil {
				t.Fatalf("capacity was not reusable: %v", err)
			}
		})
	}
}

func TestSpawnerCancellationAdmissionKeepsFallback(t *testing.T) {
	for _, failures := range []int{0, 1, 2} {
		t.Run(string(rune('0'+failures)), func(t *testing.T) {
			attempts, generated := 0, 0
			jit := &MockJITCompiler{CompileFunc: func(context.Context, *prompt.CompilationContext) (*prompt.CompilationResult, error) {
				attempts++
				if attempts <= failures {
					return nil, errors.New("ordinary failure")
				}
				return &prompt.CompilationResult{Prompt: "fixture"}, nil
			}}
			factory := &MockConfigFactory{GenerateFunc: func(context.Context, *prompt.CompilationResult, ...string) (*config.EffectiveAgentRuntimeConfig, error) {
				generated++
				return &config.EffectiveAgentRuntimeConfig{}, nil
			}}
			s := NewSpawner(&spawnAdmissionKernel{}, &MockVirtualStore{}, &MockLLMClient{}, jit, factory, &MockTransducer{}, DefaultSpawnerConfig())
			a, err := s.Spawn(context.Background(), SpawnRequest{Name: "live", IntentVerb: "/general"})
			wantAttempts, wantGenerated := 1, 1
			if failures > 0 {
				wantAttempts = 2
			}
			if failures == 2 {
				wantGenerated = 0
			}
			if a == nil || err != nil || attempts != wantAttempts || generated != wantGenerated {
				t.Fatalf("fallback changed: agent=%v err=%v attempts=%d generated=%d", a != nil, err, attempts, generated)
			}
			if failures == 2 && len(a.config.EffectiveAgentRuntimeConfig.AllowedTools) != 0 {
				t.Fatal("failed fallback granted tools")
			}
		})
	}
}

func TestSpawnerCancellationAdmissionOwnerIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{})
	finished := make(chan error, 1)
	jit := &MockJITCompiler{CompileFunc: func(c context.Context, _ *prompt.CompilationContext) (*prompt.CompilationResult, error) {
		if c.Done() != nil {
			close(entered)
			<-c.Done()
		}
		return &prompt.CompilationResult{Prompt: "fixture"}, nil
	}}
	cfg := DefaultSpawnerConfig()
	cfg.MaxActiveSubagents = 2
	s := NewSpawner(&spawnAdmissionKernel{}, &MockVirtualStore{}, &MockLLMClient{}, jit, &MockConfigFactory{}, &MockTransducer{}, cfg)
	go func() { _, err := s.Spawn(ctx, SpawnRequest{Name: "withdrawn"}); finished <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("compiler did not enter")
	}
	live, err := s.Spawn(context.Background(), SpawnRequest{Name: "live"})
	if err != nil || live == nil {
		t.Fatalf("independent spawn refused: %v", err)
	}
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("withdrawn result = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("withdrawn caller did not finish")
	}
	if s.pendingSpawns != 0 || len(s.subagents) != 1 || s.subagents[live.GetID()] != live {
		t.Fatalf("owner isolation: reservations=%d agents=%d", s.pendingSpawns, len(s.subagents))
	}
}
