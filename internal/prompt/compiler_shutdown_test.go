package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type shutdownVectorSearch struct {
	started chan struct{}
	once    sync.Once
}

func (s *shutdownVectorSearch) Search(ctx context.Context, _ string, _ int) ([]SearchResult, error) {
	s.once.Do(func() { close(s.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *shutdownVectorSearch) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{1}, nil
}

func TestCompilerCloseCancelsCompilationAndReleasesDatabase(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "corpus.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := NewAtomLoader(nil).EnsureSchema(t.Context(), db); err != nil {
		t.Fatal(err)
	}
	search := &shutdownVectorSearch{started: make(chan struct{})}
	cfg := DefaultCompilerConfig()
	cfg.VectorSearchTimeout = time.Minute
	corpus := NewEmbeddedCorpus([]*PromptAtom{{ID: "context/shutdown", Category: CategoryContext, Content: "shutdown probe", TokenCount: 2}})
	compiler, err := NewJITPromptCompiler(WithProjectDB(db), WithVectorSearcher(search), WithConfig(cfg), WithEmbeddedCorpus(corpus))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	compiled := make(chan struct{})
	go func() {
		defer close(compiled)
		_, _ = compiler.Compile(ctx, NewCompilationContext().WithSemanticQuery("shutdown probe", 1))
	}()
	select {
	case <-search.started:
	case <-time.After(5 * time.Second):
		t.Fatal("compilation did not enter the blocking search")
	}
	closed := make(chan error, 1)
	go func() { closed <- compiler.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		cancel()
		<-compiled
		<-closed
		t.Fatal("Close did not cancel and join active compilation")
	}
	select {
	case <-compiled:
	case <-time.After(time.Second):
		t.Fatal("Close returned before compilation exited")
	}
	if err := db.Ping(); err == nil {
		t.Fatal("Close left project database open")
	}
	if _, err := compiler.Compile(t.Context(), NewCompilationContext()); err == nil {
		t.Fatal("closed compiler accepted another compilation")
	}
	if err := compiler.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCompilerCloseRejectsNewDatabaseOwnership(t *testing.T) {
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compiler.RegisterDB("project", filepath.Join(t.TempDir(), "late.db")); err == nil {
		t.Fatal("closed compiler accepted a new database")
	}
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	compiler.RegisterShardDB("late", db)
	if err := db.Ping(); err == nil {
		t.Fatal("late owned database was leaked")
	}
	if _, ok := compiler.LookupShardDB("late"); ok {
		t.Fatal("late database registered after shutdown")
	}
}

func TestCompilerConcurrentRegistrationAndCloseReleaseEveryDatabase(t *testing.T) {
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	var databases []*sql.DB
	for range 16 {
		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Close() })
		if err := db.Ping(); err != nil {
			t.Fatal(err)
		}
		databases = append(databases, db)
	}
	start := make(chan struct{})
	var workers sync.WaitGroup
	for _, db := range databases {
		workers.Go(func() {
			<-start
			compiler.RegisterShardDB("replaced", db)
		})
	}
	close(start)
	if err := compiler.Close(); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	for _, db := range databases {
		if err := db.Ping(); err == nil {
			t.Fatal("registration raced shutdown and leaked its database")
		}
	}
}
