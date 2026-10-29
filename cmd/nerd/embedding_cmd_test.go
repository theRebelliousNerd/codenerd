package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/embedding"
	"codenerd/internal/store"

	"github.com/spf13/cobra"
)

func TestEmbeddingReembedHonorsCommandCancellation(t *testing.T) {
	workspaceDir := t.TempDir()
	withWorkspace(t, workspaceDir)

	savedTimeout := timeout
	t.Cleanup(func() { timeout = savedTimeout })
	timeout = 0
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(parent)
	engineStarted := false
	storeStarted := false
	ops := embeddingReembedOps{
		newEngine: func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error) {
			engineStarted = true
			return nil, errors.New("engine factory should not run for a canceled command")
		},
		reembed: func(context.Context, []string, embedding.EmbeddingEngine, store.ReembedProgressFn) (store.ReembedResult, error) {
			storeStarted = true
			return store.ReembedResult{}, errors.New("reembed store should not run for a canceled command")
		},
	}

	err := runEmbeddingReembedWithOps(cmd, nil, ops)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("embedding reembed error = %v, want context.Canceled", err)
	}
	if engineStarted || storeStarted {
		t.Fatalf("canceled command started engine=%v, store=%v", engineStarted, storeStarted)
	}
}

func TestEmbeddingReembedTimeoutReachesStoreAndStopsLaterDatabaseWork(t *testing.T) {
	workspaceDir := t.TempDir()
	withWorkspace(t, workspaceDir)
	savedTimeout := timeout
	t.Cleanup(func() { timeout = savedTimeout })
	timeout = time.Second
	seedReembedDatabase(t, filepath.Join(workspaceDir, ".nerd", "a.db"), "first")
	seedReembedDatabase(t, filepath.Join(workspaceDir, ".nerd", "b.db"), "second")

	engine := &deadlineProbeEmbeddingEngine{}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	ops := embeddingReembedOps{
		newEngine: func(ctx context.Context, _ embedding.Config) (embedding.EmbeddingEngine, error) {
			var ok bool
			engine.factoryDeadline, ok = ctx.Deadline()
			if !ok {
				return nil, errors.New("engine factory received no command deadline")
			}
			return engine, nil
		},
		reembed: store.ReembedAllDBsForce,
	}

	err := runEmbeddingReembedWithOps(cmd, nil, ops)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("embedding reembed error = %v, want context.DeadlineExceeded", err)
	}
	if engine.calls != 1 {
		t.Fatalf("store started %d embedding batches, want only the first database before cancellation", engine.calls)
	}
	if engine.embedDeadline.IsZero() || !engine.embedDeadline.Equal(engine.factoryDeadline) {
		t.Fatalf("store engine deadline = %v, factory deadline = %v; want the same command deadline", engine.embedDeadline, engine.factoryDeadline)
	}
	if !engine.closed {
		t.Fatal("embedding engine was not closed after cancellation")
	}
}

func TestEmbeddingReembedReturnsEngineCloseFailure(t *testing.T) {
	withWorkspace(t, t.TempDir())
	savedTimeout := timeout
	t.Cleanup(func() { timeout = savedTimeout })
	timeout = 0

	closeErr := errors.New("engine close failed")
	engine := &deadlineProbeEmbeddingEngine{closeErr: closeErr}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	ops := embeddingReembedOps{
		newEngine: func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error) {
			return engine, nil
		},
		reembed: func(context.Context, []string, embedding.EmbeddingEngine, store.ReembedProgressFn) (store.ReembedResult, error) {
			return store.ReembedResult{Complete: true}, nil
		},
	}

	err := runEmbeddingReembedWithOps(cmd, nil, ops)
	if !errors.Is(err, closeErr) {
		t.Fatalf("embedding reembed error = %v, want engine close error", err)
	}
	if !engine.closed {
		t.Fatal("embedding engine was not closed")
	}
}

func seedReembedDatabase(t *testing.T, path, atomID string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	local, err := store.NewLocalStore(path)
	if err != nil {
		t.Fatalf("NewLocalStore(%s): %v", path, err)
	}
	if err := local.StorePromptAtom(&store.PromptAtom{
		AtomID: atomID, Version: 1, Content: "content for " + atomID, Category: "test",
	}); err != nil {
		_ = local.Close()
		t.Fatalf("StorePromptAtom(%s): %v", atomID, err)
	}
	if err := local.Close(); err != nil {
		t.Fatalf("close seeded store: %v", err)
	}
}

type deadlineProbeEmbeddingEngine struct {
	calls           int
	closed          bool
	closeErr        error
	factoryDeadline time.Time
	embedDeadline   time.Time
}

func (engine *deadlineProbeEmbeddingEngine) Embed(ctx context.Context, _ string) ([]float32, error) {
	engine.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

func (engine *deadlineProbeEmbeddingEngine) EmbedBatch(ctx context.Context, _ []string) ([][]float32, error) {
	engine.calls++
	<-ctx.Done()
	return nil, ctx.Err()
}

func (engine *deadlineProbeEmbeddingEngine) EmbedBatchWithTask(ctx context.Context, _ []string, _ string) ([][]float32, error) {
	engine.calls++
	engine.embedDeadline, _ = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (*deadlineProbeEmbeddingEngine) Dimensions() int { return 2 }
func (*deadlineProbeEmbeddingEngine) Name() string    { return "deadline-probe" }

func (engine *deadlineProbeEmbeddingEngine) Close() error {
	engine.closed = true
	return engine.closeErr
}

// A re-embed that left rows behind is not a success: the command says so and
// exits non-zero, naming what was left.
func TestEmbeddingReembedFailsWhenRowsWereLeftBehind(t *testing.T) {
	withWorkspace(t, t.TempDir())
	savedTimeout := timeout
	t.Cleanup(func() { timeout = savedTimeout })
	timeout = 0

	engine := &deadlineProbeEmbeddingEngine{}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	ops := embeddingReembedOps{
		newEngine: func(context.Context, embedding.Config) (embedding.EmbeddingEngine, error) { return engine, nil },
		reembed: func(context.Context, []string, embedding.EmbeddingEngine, store.ReembedProgressFn) (store.ReembedResult, error) {
			return store.ReembedResult{
				Skipped: []string{"/ws/.nerd/knowledge.db vectors: incomplete"},
				Stores:  []store.StoreMigration{{Path: "/ws/.nerd/knowledge.db", Table: "vectors", Failed: 1, Total: 3, Detail: "1 of 3 failed"}},
			}, nil
		},
	}
	if err := runEmbeddingReembedWithOps(cmd, nil, ops); err == nil {
		t.Fatal("an incomplete re-embed exited successfully")
	}
}
