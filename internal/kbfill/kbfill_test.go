package kbfill_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"codenerd/internal/kbfill"
	"codenerd/internal/knowledge"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/store"
	"codenerd/internal/system"

	_ "github.com/mattn/go-sqlite3"
)

// =============================================================================
// KB FILL BASELINE
// =============================================================================
// Program item #14: every field of every expert KB and embedded database is
// filled. This test audits the embedded corpora and a shard-agent KB built by
// the production writers (the agent-definition writer, prompt sync, the
// knowledge acquisition init and the refresh run) and holds every gap in a baseline that may only shrink, as
// the starved-predicate and unconsumed-conclusion baselines do. A new gap
// fails; a fixed gap must leave the baseline, so the count is a measurement.
//
// Regenerate: CODENERD_UPDATE_KB_FILL=1 go test ./internal/kbfill/ -run TestKBFillBaseline

const kbFillBaselinePath = "testdata/kb_fill_gaps.txt"

// fakeEmbedder stands in for the configured embedding engine, so the fixture
// KB carries vectors the way a session with an engine writes them.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(context.Context, string) ([]float32, error) {
	v := make([]float32, 8)
	v[0] = 1
	return v, nil
}

func (f fakeEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i], _ = f.Embed(ctx, texts[i])
	}
	return out, nil
}

func (fakeEmbedder) Dimensions() int { return 8 }
func (fakeEmbedder) Name() string    { return "fake-embedder" }

func openReadOnly(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// docSource is a knowledge source answering one section per subject, standing
// in for the network and module-cache sources.
type docSource struct{}

func (docSource) Name() string { return "probe/docs" }

func (docSource) Acquire(_ context.Context, s knowledge.Subject) ([]knowledge.Atom, error) {
	return []knowledge.Atom{{
		Concept:    "probe/" + s.Key(),
		Content:    "A widget is calibrated against its reference table before any change to " + s.Topic + ".",
		Confidence: 0.9,
		Source:     "probe/docs",
	}}, nil
}

// unavailableSource cannot run here, as a source with no provider does.
type unavailableSource struct{}

func (unavailableSource) Name() string { return "probe/offline" }

func (unavailableSource) Acquire(context.Context, knowledge.Subject) ([]knowledge.Atom, error) {
	return nil, knowledge.ErrUnavailable
}

// buildShardAgentKB builds one shard agent's KB the way a workspace gets one:
// the agent-definition writer writes prompts.yaml, prompt sync loads it into
// .nerd/shards/<name>_knowledge.db, and the knowledge acquisition fills its
// knowledge and records every source's outcome. It returns the KB's path.
func buildShardAgentKB(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	ws := t.TempDir()
	if _, err := system.WriteAgentDefinition(ctx, ws, "FillExpert", "Expert in widget calibration",
		"widget calibration, widget testing",
		"## Approach\nRead the calibration table before changing a widget.\n\n## Verify\nRun the widget tests after each change.",
		"## Calibration\nA widget is calibrated against its reference table."); err != nil {
		t.Fatalf("WriteAgentDefinition: %v", err)
	}
	sync := prsync.NewAgentSynchronizer(ws, prompt.NewAtomLoader(fakeEmbedder{}))
	if err := sync.SyncAll(ctx); err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	path := filepath.Join(ws, ".nerd", "shards", "fillexpert_knowledge.db")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("no KB at %s: %v", path, err)
	}
	kb, err := store.NewLocalStore(path)
	if err != nil {
		t.Fatal(err)
	}
	kb.SetEmbeddingEngine(fakeEmbedder{})
	acq := knowledge.Acquirer{Sources: []knowledge.Source{docSource{}, unavailableSource{}}}
	if _, _, err := knowledge.Fill(ctx, kb, acq, []knowledge.Subject{{Agent: "fillexpert", Topic: "widget calibration"}}); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	_ = kb.Close()
	return path
}

func currentGaps(t *testing.T) []kbfill.Gap {
	t.Helper()
	ctx := context.Background()
	var gaps []kbfill.Gap
	for _, target := range []struct {
		path string
		spec kbfill.Spec
	}{
		{"../core/defaults/predicate_corpus.db", kbfill.PredicateCorpus},
		{"../core/defaults/prompt_corpus.db", kbfill.PromptCorpus},
		{"../core/defaults/intent_corpus.db", kbfill.IntentCorpus},
		{buildShardAgentKB(t), kbfill.ShardAgentKB},
	} {
		found, err := kbfill.Audit(ctx, openReadOnly(t, target.path), target.spec)
		if err != nil {
			t.Fatalf("audit %s: %v", target.spec.Name, err)
		}
		gaps = append(gaps, found...)
	}
	return gaps
}

func TestKBFillBaseline(t *testing.T) {
	gaps := currentGaps(t)
	current := map[string]kbfill.Gap{}
	for _, g := range gaps {
		current[g.Key()] = g
	}
	if os.Getenv("CODENERD_UPDATE_KB_FILL") == "1" {
		lines := []string{
			"# Knowledge-database fill gaps (program item #14). One gap per line:",
			"# <database> <table>.<column> <kind>, or <database> <table> empty_table.",
			"# The list may only shrink. Regenerate: CODENERD_UPDATE_KB_FILL=1 go test ./internal/kbfill/ -run TestKBFillBaseline",
		}
		keys := make([]string, 0, len(current))
		for k := range current {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		lines = append(lines, keys...)
		if err := os.WriteFile(kbFillBaselinePath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("rewrote %s with %d gap(s)", kbFillBaselinePath, len(keys))
		return
	}
	data, err := os.ReadFile(kbFillBaselinePath)
	if err != nil {
		t.Fatalf("read %s: %v", kbFillBaselinePath, err)
	}
	baseline := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			baseline[line] = true
		}
	}
	var grown, shrunk []string
	for k, g := range current {
		if !baseline[k] {
			grown = append(grown, g.String())
		}
	}
	for k := range baseline {
		if _, ok := current[k]; !ok {
			shrunk = append(shrunk, k)
		}
	}
	sort.Strings(grown)
	sort.Strings(shrunk)
	if len(grown) > 0 {
		t.Errorf("new knowledge-database gap(s) -- a column or table that should be filled is not; fix the producer:\n  %s", strings.Join(grown, "\n  "))
	}
	if len(shrunk) > 0 {
		t.Errorf("gap(s) closed -- good; refresh the baseline so the count stays a measurement (CODENERD_UPDATE_KB_FILL=1):\n  %s", strings.Join(shrunk, "\n  "))
	}
}

// A knowledge base holding only the removed seed's templated placeholders is
// no knowledge: the gate names it.
func TestAuditNamesAPlaceholderOnlyKB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thin_knowledge.db")
	kb, err := store.NewLocalStore(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range [][2]string{
		{"agent_identity", "You are thinexpert, a specialist agent in the codeNERD ecosystem."},
		{"domain/widgets", "thinexpert domain knowledge (1 of 1): widgets. When working on widgets, check the repository's own docs and code for project-specific rules."},
	} {
		if err := kb.StoreKnowledgeAtom(r[0], r[1], 0.9, "baseline_seed"); err != nil {
			t.Fatal(err)
		}
	}
	_ = kb.Close()
	gaps, err := kbfill.Audit(context.Background(), openReadOnly(t, path), kbfill.ShardAgentKB)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range gaps {
		found = found || g.Key() == "shard_agent_kb knowledge_atoms placeholder_only"
	}
	if !found {
		t.Fatalf("a placeholder-only KB is not named; gaps = %v", gaps)
	}
}

// Audit names an unfilled column, honours a column's scope and an optional
// column's reason, and names an empty table that must hold rows.
func TestAuditNamesEachKindOfGap(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE facts (kind TEXT, a TEXT, b TEXT, c TEXT, d BLOB)`,
		`INSERT INTO facts VALUES ('x', 'filled', '', NULL, x'')`,
		`INSERT INTO facts VALUES ('y', 'filled', 'ok', NULL, x'01')`,
		`CREATE TABLE must_fill (v TEXT)`,
		`CREATE TABLE may_be_empty (v TEXT)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	spec := kbfill.Spec{Name: "probe", Tables: map[string]kbfill.Table{
		"facts": {Columns: map[string]kbfill.Column{
			"c": {When: "kind = 'z'"},
			"d": {},
		}},
		"must_fill":    {},
		"may_be_empty": {MayBeEmpty: "nothing writes it yet"},
	}}
	gaps, err := kbfill.Audit(context.Background(), db, spec)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, g := range gaps {
		keys = append(keys, g.Key())
	}
	want := []string{"probe facts.b partial", "probe facts.d partial", "probe must_fill empty_table"}
	if strings.Join(keys, "|") != strings.Join(want, "|") {
		t.Errorf("gaps = %v, want %v", keys, want)
	}
}
