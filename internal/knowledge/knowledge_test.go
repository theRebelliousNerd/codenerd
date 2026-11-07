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

func TestFillRecordsProvenanceAndOutcomes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fixed := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	db := newKB(t)
	var docs, broken, offline int
	acq := Acquirer{Sources: []Source{
		countingSource{name: "docs", calls: &docs},
		countingSource{name: "broken", fail: errors.New("HTTP 503"), calls: &broken},
		countingSource{name: "offline", fail: ErrUnavailable, calls: &offline},
	}, Now: func() time.Time { return fixed }}
	subjects := []Subject{{Agent: "goexpert", Topic: "go concurrency"}}

	added, outcomes, err := Fill(ctx, db, acq, subjects)
	if err != nil || added != 1 || len(outcomes) != 3 {
		t.Fatalf("Fill = %d, %v, %v", added, outcomes, err)
	}
	// Acquirer.acquire runs in deterministic source-name order, whatever the
	// configured order, so repeated runs record outcomes in the same sequence.
	wantOrder := []string{"broken", "docs", "offline"}
	for i, want := range wantOrder {
		if outcomes[i].Source != want {
			t.Fatalf("Fill outcomes order = %v, want %v", outcomes, wantOrder)
		}
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
	// ensureAcquisitionsTable is idempotent: repeated calls succeed.
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		t.Fatalf("ensureAcquisitionsTable second call: %v", err)
	}
	if err := ensureAcquisitionsTable(ctx, db.GetDB()); err != nil {
		t.Fatalf("ensureAcquisitionsTable third call: %v", err)
	}

	if _, again, err := Fill(ctx, db, acq, subjects); err != nil || len(again) != 2 {
		t.Fatalf("second Fill outcomes = %v, err %v", again, err)
	}
	if docs != 1 || broken != 2 || offline != 2 {
		t.Fatalf("calls docs=%d broken=%d offline=%d, want 1 2 2", docs, broken, offline)
	}

	// ErrInapplicable maps to StatusInapplicable, not failed.
	skipped := 0
	inapplicableAcq := Acquirer{Sources: []Source{
		countingSource{name: "skip", fail: ErrInapplicable, calls: &skipped},
	}, Now: func() time.Time { return fixed }}
	_, inapOutcomes := inapplicableAcq.Acquire(ctx, subjects[0])
	if len(inapOutcomes) != 1 || inapOutcomes[0].Status != StatusInapplicable {
		t.Fatalf("inapplicable outcome = %+v", inapOutcomes)
	}

	// A canceled context fails closed without calling sources.
	cctx, stop := context.WithCancel(context.Background())
	stop()
	before := docs
	_, cancelOutcomes := acq.Acquire(cctx, subjects[0])
	if len(cancelOutcomes) != 3 {
		t.Fatalf("canceled acquire outcomes = %v", cancelOutcomes)
	}
	for _, o := range cancelOutcomes {
		if o.Status != StatusFailed {
			t.Fatalf("canceled acquire outcome = %+v, want failed", o)
		}
	}
	if docs != before {
		t.Fatalf("canceled acquire called sources: docs %d -> %d", before, docs)
	}
	if _, _, err := Fill(cctx, db, acq, subjects); err == nil {
		t.Fatalf("canceled Fill error = nil, want context canceled")
	}

	// Context7 through an in-process fetch, no network: one long section
	// becomes one atom stamped with its source.
	fetch := func(ctx context.Context, topic string) (string, error) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		return "Context7 teaches about " + topic + " with enough prose to form one full knowledge section for tests.", nil
	}
	got, err := (Context7Source{Fetch: fetch}).Acquire(ctx, Subject{Topic: "go concurrency"})
	if err != nil || len(got) != 1 || got[0].Source != "context7" {
		t.Fatalf("Context7 Acquire = %+v, %v", got, err)
	}
	if _, err := (Context7Source{}).Acquire(ctx, Subject{Topic: "x"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Context7 without fetch = %v, want ErrUnavailable", err)
	}
	if _, err := (Context7Source{Fetch: fetch}).Acquire(ctx, Subject{}); !errors.Is(err, ErrInapplicable) {
		t.Fatalf("Context7 without topic = %v, want ErrInapplicable", err)
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

func TestWebChainOneSearchPerSubjectAcrossProviders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
	// Availability gates on whether any configured provider offers search.
	if !chain.Available() {
		t.Fatalf("chain with a grounded provider reports unavailable")
	}
	if (WebChain{}).Available() {
		t.Fatalf("empty chain reports available")
	}
	if (WebChain{Providers: []WebSource{{Provider: "local", Searcher: none}}}).Available() {
		t.Fatalf("chain with no grounded provider reports available")
	}
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
	// Providers run in order: the failing provider was asked before the one
	// that answered, with one search per subject each.
	if len(failing.queries) != 1 || len(good.queries) != 1 {
		t.Fatalf("queries failing=%v good=%v, want one search per subject", failing.queries, good.queries)
	}

	uncited := WebSource{Provider: "openai", Searcher: &fakeSearcher{supports: true, result: &types.GroundedWebSearchResult{Text: strings.Repeat("an answer with no source ", 4)}}}
	if _, err := uncited.Acquire(ctx, Subject{Topic: "go"}); err == nil || errors.Is(err, ErrUnavailable) {
		t.Fatalf("an uncited answer was not refused as a failure: %v", err)
	}
	if _, err := (WebChain{Providers: []WebSource{{Provider: "local", Searcher: none}}}).Acquire(ctx, Subject{Topic: "go"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no provider offering search is not unavailable: %v", err)
	}
	// When every provider fails, the error names the failing providers.
	allFail := WebChain{Providers: []WebSource{
		{Provider: "a", Searcher: failing, Now: now},
		{Provider: "b", Searcher: failing, Now: now},
	}}
	if _, err := allFail.Acquire(ctx, Subject{Topic: "go"}); err == nil || !strings.Contains(err.Error(), "web/a") {
		t.Fatalf("all-fail error = %v, want it to name web/a", err)
	}
	// A canceled context fails closed without live search.
	cctx, stop := context.WithCancel(context.Background())
	stop()
	if _, err := (WebSource{Provider: "openai", Searcher: good}).Acquire(cctx, Subject{Topic: "go"}); err == nil {
		t.Fatalf("canceled WebSource Acquire error = nil, want cancellation")
	}
	if _, err := chain.Acquire(cctx, Subject{Topic: "go"}); err == nil {
		t.Fatalf("canceled WebChain Acquire error = nil, want cancellation")
	}
	// A canceled ModuleVersion lookup stays unknown without invoking run.
	ran := false
	spy := func(context.Context, string, string, ...string) ([]byte, error) {
		ran = true
		return []byte("v1.0.0\n"), nil
	}
	if got := ModuleVersion(cctx, spy, "/ws", "github.com/spf13/cobra"); got != "unknown" || ran {
		t.Fatalf("canceled ModuleVersion = %q ran=%v, want unknown without run", got, ran)
	}
}

func TestGoDocSourceReadsThePinnedVersion(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var calls []string
	run := func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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
	// Availability gates on a wired workspace; no live toolchain is probed
	// when a fake run is wired, and no network is touched.
	if !src.Available() {
		t.Fatalf("GoDocSource with workspace and run reports unavailable")
	}
	if (GoDocSource{}).Available() {
		t.Fatalf("GoDocSource without workspace reports available")
	}
	atoms, err := src.Acquire(ctx, Subject{Topic: "cobra", Module: "github.com/spf13/cobra", Ecosystem: "go"})
	if err != nil || len(atoms) != 1 {
		t.Fatalf("Acquire = %+v, %v (calls %v)", atoms, err, calls)
	}
	if _, err := src.Acquire(ctx, Subject{Topic: "react", Module: "react", Ecosystem: "npm"}); !errors.Is(err, ErrInapplicable) {
		t.Fatalf("go doc claims to read an npm package: %v", err)
	}
	if atoms[0].Source != "godoc/github.com/spf13/cobra@v1.10.2" || atoms[0].Concept != "godoc/github.com/spf13/cobra@v1.10.2" {
		t.Fatalf("atom = %+v", atoms[0])
	}
	if _, err := src.Acquire(ctx, Subject{Topic: "testing practice"}); !errors.Is(err, ErrInapplicable) {
		t.Fatalf("go doc claims to read a research topic: %v", err)
	}
	// Without a workspace go doc is unavailable, not failed.
	if _, err := (GoDocSource{Run: run}).Acquire(ctx, Subject{Topic: "cobra", Module: "github.com/spf13/cobra", Ecosystem: "go"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("go doc without workspace = %v, want ErrUnavailable", err)
	}
	// A canceled context fails closed before any go invocation.
	cctx, stop := context.WithCancel(context.Background())
	stop()
	ran := false
	cancelRun := func(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
		ran = true
		return []byte("v1.0.0\n"), nil
	}
	cancelSrc := GoDocSource{
		Workspace: "/ws",
		Run:       cancelRun,
		Packages: func(context.Context, string) ([]string, error) {
			return []string{"github.com/spf13/cobra"}, nil
		},
	}
	if _, err := cancelSrc.Acquire(cctx, Subject{Topic: "cobra", Module: "github.com/spf13/cobra", Ecosystem: "go"}); err == nil {
		t.Fatalf("canceled GoDoc Acquire error = nil, want cancellation")
	}
	if ran {
		t.Fatalf("canceled GoDoc Acquire invoked go")
	}
	// ModuleVersion stays unknown on cancellation without invoking run.
	ran = false
	if v := ModuleVersion(cctx, cancelRun, "/ws", "github.com/spf13/cobra"); v != "unknown" || ran {
		t.Fatalf("canceled ModuleVersion = %q ran=%v, want unknown without run", v, ran)
	}
	// ModuleVersion without a workspace or run is unknown, without probing.
	if v := ModuleVersion(ctx, nil, "", "github.com/spf13/cobra"); v != "unknown" {
		t.Fatalf("ModuleVersion without workspace/run = %q, want unknown", v)
	}
	if v := ModuleVersion(ctx, run, "/ws", "context"); v != "std" {
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
