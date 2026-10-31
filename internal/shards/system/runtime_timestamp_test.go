package system

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

type timestampShard struct {
	base    *BaseSystemShard
	execute func(context.Context, string) (string, error)
	read    func()
	check   func(*testing.T, time.Time)
}

func timestampShardFactories() map[string]func() timestampShard {
	return map[string]func() timestampShard{
		"perception": func() timestampShard {
			s := NewPerceptionFirewallShard()
			return timestampShard{s.BaseSystemShard, s.Execute,
				func() { _ = s.generateShutdownSummary("timestamp probe") }, nil}
		},
		"constitution": func() timestampShard {
			s := NewConstitutionGateShard()
			return timestampShard{s.BaseSystemShard, s.Execute,
				func() { _ = s.generateShutdownSummary("timestamp probe") }, nil}
		},
		"executive": func() timestampShard {
			s := NewExecutivePolicyShard()
			return timestampShard{s.BaseSystemShard, s.Execute,
				func() { _ = s.generateShutdownSummary("timestamp probe") }, nil}
		},
		"planner": func() timestampShard {
			s := NewSessionPlannerShard()
			return timestampShard{s.BaseSystemShard, s.Execute,
				func() { _ = s.generateShutdownSummary("timestamp probe"); _ = s.GetCurrentPlan() },
				func(t *testing.T, started time.Time) {
					view := s.GetCurrentPlan()
					if !view.StartedAt.Equal(started) || view.RuntimeSec < 0 {
						t.Fatalf("planner view lost timestamp/runtime: %+v, started=%v", view, started)
					}
				}}
		},
		"router": func() timestampShard {
			s := NewTactileRouterShard()
			return timestampShard{s.BaseSystemShard, s.Execute,
				func() {
					_ = s.generateShutdownSummary("timestamp probe")
					_ = s.getSessionID()
					_ = time.Since(s.StartedAt()).Milliseconds()
				},
				func(t *testing.T, started time.Time) {
					if got, want := s.getSessionID(), fmt.Sprintf("session-%d", started.Unix()); got != want {
						t.Fatalf("router identity changed: %q, want %q", got, want)
					}
				}}
		},
		"campaign_runner": func() timestampShard {
			s := NewCampaignRunnerShard()
			return timestampShard{s.BaseSystemShard, s.Execute, func() { _ = s.StartedAt() }, nil}
		},
	}
}

// The six real Execute entry paths race the actual shutdown reader and every
// timestamp-based summary/view/identity consumer. A pre-canceled context keeps
// the resident loops finite and prevents model/tool work; it does not bypass
// their production startup and shutdown paths.
func TestResidentShardTimestampStartupStopAndReaders(t *testing.T) {
	for name, factory := range timestampShardFactories() {
		t.Run(name, func(t *testing.T) {
			kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			for iteration := 0; iteration < 16; iteration++ {
				s := factory()
				s.base.SetParentKernel(kernel)
				if !s.base.StartedAt().IsZero() {
					t.Fatal("unstarted instance has a timestamp")
				}
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				start := make(chan struct{})
				done := make(chan struct{})
				errs := make(chan error, 4)
				var wg sync.WaitGroup
				wg.Add(4)
				before := time.Now()
				go func() {
					defer wg.Done()
					<-start
					_, err := s.execute(ctx, "system_start")
					if err != nil && !errors.Is(err, context.Canceled) {
						errs <- err
					}
				}()
				go func() {
					defer wg.Done()
					<-start
					for i := 0; i < 32; i++ {
						if err := s.base.Stop(); err != nil {
							errs <- err
							return
						}
					}
				}()
				for reader := 0; reader < 2; reader++ {
					go func() {
						defer wg.Done()
						<-start
						for i := 0; i < 128; i++ {
							_ = s.base.StartedAt()
							_ = s.base.GetState()
							s.read()
						}
					}()
				}
				go func() { wg.Wait(); close(done) }()
				close(start)
				select {
				case <-done:
				case <-time.After(20 * time.Second):
					t.Fatal("timestamp startup/stop/read deadlocked")
				}
				close(errs)
				for err := range errs {
					t.Fatal(err)
				}
				started := s.base.StartedAt()
				if started.Before(before) || started.After(time.Now()) {
					t.Fatalf("Execute did not initialize this run's timestamp: %v", started)
				}
				if s.base.GetState() != types.ShardStateCompleted {
					t.Fatalf("terminal state changed: %v", s.base.GetState())
				}
				if s.check != nil {
					s.check(t, started)
				}
			}
		})
	}
}

// Updating runtime metadata for one instance must never change another, and
// publishing Running must not expose a zero timestamp to the shutdown owner.
func TestBaseShardTimestampPublicationAndIsolation(t *testing.T) {
	b := NewBaseSystemShard("active", StartupAuto)
	other := NewBaseSystemShard("unrelated", StartupAuto)
	otherStart := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	setStartedAtForTest(other, otherStart)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			b.beginRun()
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 1000; i++ {
			b.mu.RLock()
			if b.State == types.ShardStateRunning && b.startTime.IsZero() {
				t.Error("Running published before its timestamp")
			}
			b.mu.RUnlock()
			_ = b.StartedAt()
		}
	}()
	close(start)
	wg.Wait()
	if !other.StartedAt().Equal(otherStart) || other.GetState() != types.ShardStateIdle {
		t.Fatal("another instance's metadata was changed")
	}
	if b.StartedAt().IsZero() || b.GetState() != types.ShardStateRunning {
		t.Fatal("active timestamp was lost")
	}
}
