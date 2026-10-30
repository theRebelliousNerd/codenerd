package knowledge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/store"
	"codenerd/internal/types"
)

func newKB(t *testing.T) *store.LocalStore {
	t.Helper()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "probe_knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// countingSource answers one atom per subject, or fails, and counts calls.
type countingSource struct {
	name  string
	fail  error
	calls *int
}

func (c countingSource) Name() string { return c.name }

func (c countingSource) Acquire(_ context.Context, s Subject) ([]Atom, error) {
	*c.calls++
	if c.fail != nil {
		return nil, c.fail
	}
	return []Atom{{Concept: c.name + "/" + s.Key(), Content: "What " + c.name + " teaches about " + s.Topic + ".", Confidence: 0.9, Source: c.name}}, nil
}

// Fill stores each source's atoms with their provenance and records every
// source's outcome, unavailable and failed included; a second fill does not
// ask a source that already acquired the subject, and asks the failed one
// again.
func TestFillRecordsProvenanceAndOutcomes(t *testing.T) {
	ctx := context.Background()
	db := newKB(t)
	var docs, broken, offline int
	acq := Acquirer{Sources: []Source{
		countingSource{name: "docs", calls: &docs},
		countingSource{name: "broken", fail: errors.New("HTTP 503"), calls: &broken},
		countingSource{name: "offline", fail: ErrUnavailable, calls: &offline},
	}, Now: func() time.Time { return time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC) }}
	subjects := []Subject{{Agent: "goexpert", Topic: "go concurrency"}}

	added, outcomes, err := Fill(ctx, db, acq, subjects)
	if err != nil || added != 1 || len(outcomes) != 3 {
		t.Fatalf("Fill = %d, %v, %v", added, outcomes, err)
	}
	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil || len(atoms) != 1 || atoms[0].Source != "docs" {
		t.Fatalf("stored atoms = %+v, err %v", atoms, err)
	}
	recorded, err := Outcomes(ctx, db.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	status := map[string]string{}
	for _, o := range recorded {
		status[o.Source] = o.Status
	}
	if status["docs"] != StatusAcquired || status["broken"] != StatusFailed || status["offline"] != StatusUnavailable {
		t.Fatalf("recorded outcomes = %+v", recorded)
	}

	if _, again, err := Fill(ctx, db, acq, subjects); err != nil || len(again) != 2 {
		t.Fatalf("second Fill outcomes = %v, err %v", again, err)
	}
	if docs != 1 || broken != 2 || offline != 2 {
		t.Fatalf("calls docs=%d broken=%d offline=%d, want 1 2 2", docs, broken, offline)
	}
}

type fakeSearcher struct {
	supports bool
	result   *types.GroundedWebSearchResult
	err      error
	queries  []string
}

func (f *fakeSearcher) SupportsGroundedWebSearch() bool { return f.supports }

func (f *fakeSearcher) GroundedWebSearch(_ context.Context, q string) (*types.GroundedWebSearchResult, error) {
	f.queries = append(f.queries, q)
	return f.result, f.err
}

// Web search keeps only cited answers, stamped with their question, sources
// and retrieval date; across providers the first that answers supplies the
// subject, and the providers that failed first are named.
func TestWebChainOneSearchPerSubjectAcrossProviders(t *testing.T) {
	ctx := context.Background()
	now := func() time.Time { return time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC) }
	none := &fakeSearcher{supports: false}
	failing := &fakeSearcher{supports: true, err: errors.New("quota exceeded")}
	good := &fakeSearcher{supports: true, result: &types.GroundedWebSearchResult{
		Text:      "Cancel a context to stop every goroutine derived from it; pass ctx as the first argument.",
		Citations: []types.GroundedCitation{{URL: "https://go.dev/blog/context"}},
	}}
	later := &fakeSearcher{supports: true}
	chain := WebChain{Providers: []WebSource{
		{Provider: "local", Searcher: none, Now: now},
		{Provider: "meta", Searcher: failing, Now: now},
		{Provider: "openai", Searcher: good, Now: now},
		{Provider: "zai", Searcher: later, Now: now},
	}}
	atoms, err := chain.Acquire(ctx, Subject{Topic: "go context", Module: "context"})
	if err != nil || len(atoms) != 1 {
		t.Fatalf("Acquire = %+v, %v", atoms, err)
	}
	a := atoms[0]
	if a.Source != "web/openai" || !strings.Contains(a.Content, "Sources: https://go.dev/blog/context") || !strings.Contains(a.Content, "Retrieved: 2026-10-29") {
		t.Fatalf("atom = %+v", a)
	}
	if len(later.queries) != 0 {
		t.Fatalf("a provider after the one that answered was asked: %v", later.queries)
	}

	uncited := WebSource{Provider: "openai", Searcher: &fakeSearcher{supports: true, result: &types.GroundedWebSearchResult{Text: strings.Repeat("an answer with no source ", 4)}}}
	if _, err := uncited.Acquire(ctx, Subject{Topic: "go"}); err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("an uncited answer was not refused as a failure: %v", err)
	}
	if _, err := (WebChain{Providers: []WebSource{{Provider: "local", Searcher: none}}}).Acquire(ctx, Subject{Topic: "go"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no provider offering search is not unavailable: %v", err)
	}
}

// go doc runs per package the repository imports from the module, at the
// version go.mod pins, and its provenance carries that version.
func TestGoDocSourceReadsThePinnedVersion(t *testing.T) {
	var calls []string
	run := func(_ context.Context, dir, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		switch {
		case args[0] == "list":
			return []byte("v1.10.2\n"), nil
		case args[1] == "github.com/spf13/cobra":
			return []byte("package cobra // import \"github.com/spf13/cobra\"\n\nPackage cobra is a commander."), nil
		}
		return []byte("doc: no such package"), errors.New("exit status 1")
	}
	src := GoDocSource{
		Workspace: "/ws",
		Run:       run,
		Packages: func(context.Context, string) ([]string, error) {
			return []string{"github.com/spf13/cobra", "github.com/spf13/cobra/doc"}, nil
		},
	}
	atoms, err := src.Acquire(context.Background(), Subject{Topic: "cobra", Module: "github.com/spf13/cobra", Ecosystem: "go"})
	if err != nil || len(atoms) != 1 {
		t.Fatalf("Acquire = %+v, %v (calls %v)", atoms, err, calls)
	}
	if _, err := src.Acquire(context.Background(), Subject{Topic: "react", Module: "react", Ecosystem: "npm"}); !errors.Is(err, ErrInapplicable) {
		t.Fatalf("go doc claims to read an npm package: %v", err)
	}
	if atoms[0].Source != "godoc/github.com/spf13/cobra@v1.10.2" || atoms[0].Concept != "godoc/github.com/spf13/cobra@v1.10.2" {
		t.Fatalf("atom = %+v", atoms[0])
	}
	if _, err := src.Acquire(context.Background(), Subject{Topic: "testing practice"}); !errors.Is(err, ErrInapplicable) {
		t.Fatalf("go doc claims to read a research topic: %v", err)
	}
	if v := ModuleVersion(context.Background(), run, "/ws", "context"); v != "std" {
		t.Fatalf("standard-library version = %q", v)
	}
	// A vocabulary key spelled without a dot is not the standard library.
	if IsStdPackage("google/mangle") || !IsStdPackage("net/http") || IsStdPackage("github.com/spf13/cobra") {
		t.Fatal("IsStdPackage does not ask the toolchain")
	}
}

// The repository source names the files that import the module, and is
// unavailable without a world model.
func TestRepoSourceNamesImporters(t *testing.T) {
	src := RepoSource{Importers: func(context.Context, string) ([]string, error) {
		return []string{"cmd/nerd/main.go", "cmd/nerd/cmd_check_kb.go"}, nil
	}}
	atoms, err := src.Acquire(context.Background(), Subject{Topic: "cobra", Module: "github.com/spf13/cobra"})
	if err != nil || len(atoms) != 1 || !strings.Contains(atoms[0].Content, "cmd/nerd/cmd_check_kb.go") {
		t.Fatalf("Acquire = %+v, %v", atoms, err)
	}
	if _, err := KernelImports(nil).Importers(context.Background(), "x"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no kernel is not unavailable: %v", err)
	}
}

// The removed seed's templates are pruned; real knowledge stays.
func TestPrunePlaceholdersKeepsKnowledge(t *testing.T) {
	db := newKB(t)
	for _, r := range [][2]string{
		{"agent_identity", "You are goexpert, a specialist agent in the codeNERD ecosystem."},
		{"agent_mission", "goexpert exists to carry out goexpert tasks in this workspace."},
		{"methodology/overview", "goexpert methodology: decompose the task, consult the knowledge base, verify."},
		{"domain/go", "goexpert domain knowledge (1 of 1): go. When working on go, check the repository's own docs and code for project-specific rules."},
		{"domain/go/section_0", "Close channels from the sender side only."},
	} {
		if err := db.StoreKnowledgeAtom(r[0], r[1], 0.9, "test"); err != nil {
			t.Fatal(err)
		}
	}
	n, err := PrunePlaceholders(context.Background(), db.GetDB())
	if err != nil || n != 4 {
		t.Fatalf("PrunePlaceholders = %d, %v", n, err)
	}
	atoms, _ := db.GetAllKnowledgeAtoms()
	if len(atoms) != 1 || atoms[0].Concept != "domain/go/section_0" {
		t.Fatalf("left = %+v", atoms)
	}
}
