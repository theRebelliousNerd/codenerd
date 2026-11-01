//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// Read is called only after ParseUnit owns the process-wide ANTLR lock.
// A channel, rather than expensive parsing or host load, holds that ownership.
type heldSpawnerParse struct {
	reader  io.Reader
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *heldSpawnerParse) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.entered) })
	<-r.release
	return r.reader.Read(p)
}

func TestE2E_SpawnerCatalogCancellationWhileParserOwned(t *testing.T) {
	// Intentionally serial: no other fixture should inherit this held parser.
	t.Run("real", func(t *testing.T) {
		kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		checkSpawnerParserCancellation(t, kernel)
	})
	t.Run("cortex", func(t *testing.T) {
		shard, err := core.NewKernelShard(core.KernelShardConfig{Domain: "cortex", WorkspaceRoot: t.TempDir()})
		if err != nil {
			t.Fatal(err)
		}
		kernel := core.NewCortexKernel("cortex")
		if err := kernel.RegisterShard(shard); err != nil {
			t.Fatal(err)
		}
		checkSpawnerParserCancellation(t, kernel)
	})
}

func checkSpawnerParserCancellation(t *testing.T, kernel types.Kernel) {
	t.Helper()
	compiler := &spawnerMockJITCompiler{delay: time.Hour}
	cfg := session.DefaultSpawnerConfig()
	cfg.MaxActiveSubagents = 1
	s := session.NewSpawner(kernel, nil, &mockRealLLM{}, compiler, &spawnerMockConfigFactory{}, &spawnerMockTransducer{}, cfg)
	holder := &heldSpawnerParse{reader: strings.NewReader("held_parser(/ok)."), entered: make(chan struct{}), release: make(chan struct{})}
	parsed := make(chan error, 1)
	parserDone := make(chan struct{})
	go func() { defer close(parserDone); _, err := mangle.ParseUnit(holder); parsed <- err }()
	var release sync.Once
	unblock := func() { release.Do(func() { close(holder.release) }) }
	t.Cleanup(func() {
		unblock()
		select {
		case <-parserDone:
		case <-time.After(5 * time.Second):
			t.Error("parser owner escaped fixture cleanup")
		}
	})
	select {
	case <-holder.entered:
	case <-time.After(time.Second):
		t.Fatal("parser holder did not acquire ownership")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	finished := make(chan error, 1)
	spawnDone := make(chan struct{})
	go func() {
		defer close(spawnDone)
		_, err := s.Spawn(ctx, session.SpawnRequest{Name: "withdrawn", IntentVerb: "/test"})
		finished <- err
	}()
	t.Cleanup(func() {
		cancel()
		unblock()
		select {
		case <-spawnDone:
		case <-time.After(5 * time.Second):
			t.Error("spawn escaped fixture cleanup")
		}
	})
	select {
	case err := <-finished:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("spawn error = %v", err)
		}
	case <-time.After(time.Second):
		t.Error("Spawn did not respect the unchanged one-second bound while another owner held the parser")
		unblock()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Fatal("spawn did not finish after releasing parser")
		}
	}
	t.Logf("50ms owner deadline; spawn returned after %s", time.Since(start))
	compiler.mu.Lock()
	calls := compiler.compileCount
	compiler.mu.Unlock()
	if calls != 0 {
		t.Errorf("expired catalog dispatched %d compile calls", calls)
	}
	unblock()
	select {
	case err := <-parsed:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Fatal("parser owner did not finish")
	}
	// Ordinary parsing still works; cancellation neither consumes the lock nor
	// leaves temporary catalog facts or the sole spawn reservation behind.
	if _, err := mangle.ParseAtom("normal_parser(/ok)"); err != nil {
		t.Fatal(err)
	}
	for _, predicate := range []string{"turn_verb", "turn_declared_check"} {
		facts, err := kernel.Query(predicate)
		if err != nil || len(facts) != 0 {
			t.Fatalf("temporary %s leaked: %v, %v", predicate, facts, err)
		}
	}
	compiler.mu.Lock()
	compiler.delay = 0
	compiler.mu.Unlock()
	if a, err := s.Spawn(context.Background(), session.SpawnRequest{Name: "healthy", IntentVerb: "/test"}); err != nil || a == nil {
		t.Fatalf("healthy follow-up = %v", err)
	}
}
