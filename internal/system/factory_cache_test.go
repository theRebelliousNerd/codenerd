package system

import (
	"context"
	"errors"
	"testing"
)

func TestGetOrBootCortexFailureIsNotCached(t *testing.T) {
	workspace := t.TempDir()
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, disabled []string) (*Cortex, error) {
		bootCalls++
		if bootCalls == 1 {
			return nil, errors.New("forced transient boot failure")
		}
		return &Cortex{Workspace: ws}, nil
	}

	if cortex, err := getOrBootCortex(context.Background(), workspace, "secret", []string{"router"}, boot); err == nil || cortex != nil {
		t.Fatalf("first boot = (%v, %v), want (nil, error)", cortex, err)
	}

	cortex, err := getOrBootCortex(context.Background(), workspace, "secret", []string{"router"}, boot)
	if err != nil {
		t.Fatalf("retry after failed boot: %v", err)
	}
	t.Cleanup(func() { _ = cortex.Close() })

	again, err := getOrBootCortex(context.Background(), workspace, "secret", []string{"router"}, boot)
	if err != nil {
		t.Fatalf("cache hit after successful retry: %v", err)
	}
	if again != cortex {
		t.Fatal("successful retry was not cached")
	}
	if bootCalls != 2 {
		t.Fatalf("boot calls = %d, want 2", bootCalls)
	}
}

func TestGetOrBootCortexDisabledShardSetIsPartOfIdentity(t *testing.T) {
	workspace := t.TempDir()
	bootCalls := 0
	boot := func(_ context.Context, ws, _ string, disabled []string) (*Cortex, error) {
		bootCalls++
		return &Cortex{Workspace: ws}, nil
	}

	first, err := getOrBootCortex(context.Background(), workspace, "secret", []string{" beta ", "alpha", "beta"}, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close() })

	reordered, err := getOrBootCortex(context.Background(), workspace, "secret", []string{"beta", "alpha"}, boot)
	if err != nil {
		t.Fatal(err)
	}
	if reordered != first {
		t.Fatal("equivalent disabled-shard sets did not reuse the Cortex")
	}

	different, err := getOrBootCortex(context.Background(), workspace, "secret", []string{"alpha"}, boot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = different.Close() })
	if different == first {
		t.Fatal("different disabled-shard sets reused the same Cortex")
	}
	if bootCalls != 2 {
		t.Fatalf("boot calls = %d, want 2 identities", bootCalls)
	}
}
