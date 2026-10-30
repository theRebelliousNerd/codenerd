package sessionlog

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestServices_LifecycleHealthAndActivity(t *testing.T) {
	for _, tc := range []struct {
		name       string
		err        error
		panicValue string
		want       string
	}{
		{name: "stopped", want: "stopped"},
		{name: "failed", err: errors.New("unavailable"), want: "failed"},
		{name: "cancelled", err: context.Canceled, want: "failed"},
		{name: "panic", panicValue: "service panic", want: "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := installStore(t, "health", false)
			var recovered any
			func() {
				defer func() { recovered = recover() }()
				_, err := RunService(context.Background(), "watcher", "instance", "watch", func(ctx context.Context) (string, error) {
					initial := s.Services()
					if len(initial) != 1 || initial[0].Health != "ready" {
						t.Fatalf("initial service: %+v", initial)
					}
					before := time.Now()
					TranscriptFrom(ctx).CallFailed(ctx, "model", errors.New("retryable"))
					failed := s.Services()[0]
					if failed.Health != "failed" || failed.LastActivity.Before(before) {
						t.Fatalf("activity did not update health/time: %+v", failed)
					}
					EmitLiveProgress(ctx, LiveEvent{Kind: EventAssistant})
					if got := s.Services()[0].Health; got != "ready" {
						t.Fatalf("recovered health = %q", got)
					}
					if tc.panicValue != "" {
						panic(tc.panicValue)
					}
					return "", tc.err
				})
				if !errors.Is(err, tc.err) {
					t.Fatalf("error lost: %v", err)
				}
			}()
			if tc.panicValue != "" && recovered != tc.panicValue {
				t.Fatalf("panic lost: %v", recovered)
			}
			if tc.panicValue == "" && recovered != nil {
				t.Fatalf("unexpected panic: %v", recovered)
			}
			if got := s.Services()[0].Health; got != tc.want {
				t.Fatalf("settled health = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestServices_SnapshotIsBoundedSortedAndDetached(t *testing.T) {
	s, _ := installStore(t, "services", false)
	for i := 0; i < 10; i++ {
		for _, role := range []string{"watcher", "planner"} {
			if _, err := RunService(context.Background(), role, fmt.Sprint(i), "watch", func(context.Context) (string, error) { return "", nil }); err != nil {
				t.Fatal(err)
			}
		}
	}
	snapshot := s.Services()
	if len(snapshot) != 2 || snapshot[0].Name != "planner" || snapshot[1].Name != "watcher" {
		t.Fatalf("activations duplicated or unordered: %+v", snapshot)
	}
	snapshot[0].Health = "changed"
	if s.Services()[0].Health != "stopped" {
		t.Fatal("snapshot aliases registry")
	}
	if _, err := RunAgent(WithTask(context.Background(), "task"), "coder", "worker", "work", func(context.Context) (string, error) { return "", nil }); err != nil {
		t.Fatal(err)
	}
	if len(s.Services()) != 2 {
		t.Fatal("task entered service registry")
	}
}

func TestServices_OverlappingActivationsStayReadyUntilBothStop(t *testing.T) {
	s, _ := installStore(t, "services", false)
	ctx := context.Background()
	_, err := RunService(ctx, "watcher", "outer", "watch", func(ctx context.Context) (string, error) {
		_, err := RunService(ctx, "watcher", "inner", "watch", func(context.Context) (string, error) { return "", nil })
		if err != nil {
			return "", err
		}
		if got := s.Services(); len(got) != 1 || got[0].Health != "ready" {
			t.Fatalf("one activation stopped the other: %+v", got)
		}
		return "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Services()[0].Health; got != "stopped" {
		t.Fatalf("settled service = %q", got)
	}
}

func TestServices_NilStoreIsSafe(t *testing.T) {
	previous := Current()
	Install(nil)
	t.Cleanup(func() { Install(previous) })
	if got := Current().Services(); len(got) != 0 {
		t.Fatalf("nil store services: %+v", got)
	}
	if _, err := RunService(context.Background(), "watcher", "instance", "watch", func(context.Context) (string, error) { return "", nil }); err != nil {
		t.Fatal(err)
	}
}
