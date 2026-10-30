package world

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

// Done announces admission to the refresh select. This gives the test a
// deterministic waiter barrier without sleeping or adding a production hook.
type refreshWaitContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *refreshWaitContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestRefresh_PublicationPrecedesNextObservation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	root := t.TempDir()
	stamp := time.Now().Add(-time.Hour)
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		stamp = stamp.Add(time.Second)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	withoutImport := "package main\nfunc main() {}\n"
	withImport := "package main\nimport \"example.com/ws/b\"\nfunc main() { b.B() }\n"
	write("go.mod", "module example.com/ws\n\ngo 1.26\n")
	write("b/b.go", "package b\nfunc B() {}\n")
	write("a.go", withoutImport)
	if err := os.MkdirAll(filepath.Join(root, ".nerd", "cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "knowledge.db")
	db, err := store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	vs := core.NewVirtualStore(nil)
	vs.SetLocalDB(db)
	scanner := NewScanner()
	paused, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var workers sync.WaitGroup
	defer func() {
		cancel()
		unblock()
		workers.Wait()
	}()
	pauseNext := false // changed only while no publisher is running
	if err := scanner.SetRefreshPublisher(func(ctx context.Context, res *IncrementalResult) error {
		if pauseNext {
			pauseNext = false
			close(paused)
			select {
			case <-release:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return vs.ReplaceLinkFacts(res.RetractFacts, res.NewFacts, "scan")
	}); err != nil {
		t.Fatal(err)
	}
	refresh := func() *IncrementalResult {
		t.Helper()
		res, err := scanner.Refresh(ctx, root, db, kernel, IncrementalOptions{SkipWhenUnchanged: true})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	hasDependency := func(want bool) {
		t.Helper()
		facts, err := vs.QueryKnowledgeGraph("a.go", "outgoing")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, f := range facts {
			if f.Args[1] == "/depends_on" && f.Args[2] == "b/b.go" {
				found = true
			}
		}
		if found != want {
			t.Fatalf("dependency present=%v, want %v: %v", found, want, facts)
		}
	}
	if res := refresh(); !res.Full || !res.WorldApplied || !res.Published {
		t.Fatalf("initial receipt = %+v", res)
	}
	hasDependency(false)
	if err := db.StoreLink("a.go", "depends_on", "notes.md", 1, map[string]any{"source": "memory_operation"}); err != nil {
		t.Fatal(err)
	}
	write("a.go", withImport)
	pauseNext = true
	older := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, err := scanner.Refresh(ctx, root, db, kernel, IncrementalOptions{SkipWhenUnchanged: true})
		older <- err
	}()
	select {
	case <-paused:
	case err := <-older:
		t.Fatalf("older scan did not reach publication barrier: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// A's edge exists in the real fast cache and kernel, but is not yet in SQL.
	cached, _, err := db.LoadWorldFactsForFile("a.go", "fast")
	if err != nil {
		t.Fatal(err)
	}
	cachedEdge := false
	for _, f := range cached {
		if f.Predicate == "dependency_link" && f.Args[1] == "b/b.go" {
			cachedEdge = true
		}
	}
	if !cachedEdge {
		t.Fatal("precondition: A did not cache its resolved local import")
	}
	if facts, err := kernel.Query(`dependency_link("a.go", "b/b.go", I)`); err != nil || len(facts) != 1 {
		t.Fatalf("A's applied dependency = %v, err=%v", facts, err)
	}
	hasDependency(false)
	write("a.go", withoutImport)
	// An unrelated file must survive alongside B's retirement, too.
	write("other.go", "package main\nfunc Other() {}\n")
	waitBase, cancelWait := context.WithCancel(ctx)
	defer cancelWait()
	waitCtx := &refreshWaitContext{Context: waitBase, waiting: make(chan struct{})}
	waited := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, err := scanner.Refresh(waitCtx, root, db, kernel, IncrementalOptions{SkipWhenUnchanged: true})
		waited <- err
	}()
	select {
	case <-waitCtx.waiting:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cancelWait()
	select {
	case err := <-waited:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting refresh = %v, want cancellation", err)
		}
	case <-ctx.Done():
		t.Fatal("waiting refresh did not honor cancellation")
	}
	newerCtx := &refreshWaitContext{Context: ctx, waiting: make(chan struct{})}
	newer := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, err := scanner.Refresh(newerCtx, root, db, kernel, IncrementalOptions{SkipWhenUnchanged: true})
		newer <- err
	}()
	select {
	case <-newerCtx.waiting:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	unblock() // A must finish publication before B can observe/retract its cache.
	for _, completed := range []<-chan error{older, newer} {
		select {
		case err := <-completed:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	hasDependency(false)
	if res := refresh(); !res.Unchanged {
		t.Fatalf("settled refresh changed the source: %+v", res)
	}
	if rows, err := vs.QueryKnowledgeGraph("other.go", "incoming"); err != nil || len(rows) == 0 {
		t.Fatalf("unrelated file lost its projection: %v, err=%v", rows, err)
	}
	// Currentness does not forbid a legitimate later re-addition or deletion.
	write("a.go", withImport)
	refresh()
	hasDependency(true)
	if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
		t.Fatal(err)
	}
	refresh()
	hasDependency(false)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.NewLocalStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	vs.SetLocalDB(db)
	hasDependency(false)
	links, err := db.QueryLinks("a.go", "outgoing")
	if err != nil || len(links) != 1 || links[0].Metadata["source"] != "memory_operation" {
		t.Fatalf("manual edge was not preserved across deletion/reopen: %v, err=%v", links, err)
	}
}
