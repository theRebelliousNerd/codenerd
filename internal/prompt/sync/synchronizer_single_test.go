package sync

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"testing"

	"codenerd/internal/prompt"
)

func agentYAML(agent string, atoms int, body string) string {
	var b strings.Builder
	for i := 0; i < atoms; i++ {
		fmt.Fprintf(&b, "- id: %s/atom%02d\n  category: methodology\n  is_mandatory: false\n  content: |\n    %s %d\n", agent, i, body, i)
	}
	return b.String()
}

func writeAgentYAML(t testing.TB, root, agent, yaml string) {
	t.Helper()
	dir := filepath.Join(root, ".nerd", "agents", agent)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// kbObserver holds a second connection to an agent's KB open. SQLite bumps a
// connection's data_version whenever ANOTHER connection commits, so a
// version that stays put proves a sync wrote nothing.
type kbObserver struct {
	db   *sql.DB
	path string
}

func observeKB(t *testing.T, root, agent string) *kbObserver {
	t.Helper()
	path := filepath.Join(root, ".nerd", "shards", strings.ToLower(agent)+"_knowledge.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return &kbObserver{db: db, path: path}
}

func (o *kbObserver) dataVersion(t *testing.T) int64 {
	t.Helper()
	var v int64
	if err := o.db.QueryRow("PRAGMA data_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// rowIDs are AUTOINCREMENT, so a rewrite of the atom set changes them.
func (o *kbObserver) rowIDs(t *testing.T) string {
	t.Helper()
	rows, err := o.db.Query("SELECT id, atom_id FROM prompt_atoms ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var id int64
		var atomID string
		if err := rows.Scan(&id, &atomID); err != nil {
			t.Fatal(err)
		}
		parts = append(parts, fmt.Sprintf("%d:%s", id, atomID))
	}
	return strings.Join(parts, ",")
}

func (o *kbObserver) atomIDs(t *testing.T) map[string]string {
	t.Helper()
	rows, err := o.db.Query("SELECT atom_id, content FROM prompt_atoms")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			t.Fatal(err)
		}
		out[id] = strings.TrimSpace(content)
	}
	return out
}

func (o *kbObserver) syncedHash(t *testing.T, agent string) string {
	t.Helper()
	var h string
	if err := o.db.QueryRow("SELECT value FROM sync_metadata WHERE key = ?", "yaml_hash_"+agent).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSyncAgent_UnchangedYAMLIsSkippedWithNoWrites(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Alpha", agentYAML("alpha", 4, "Alpha guidance"))
	writeAgentYAML(t, root, "Beta", agentYAML("beta", 3, "Beta guidance"))
	ctx := context.Background()

	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if err := syncer.SyncAll(ctx); err != nil {
		t.Fatalf("first SyncAll: %v", err)
	}
	alpha, beta := observeKB(t, root, "Alpha"), observeKB(t, root, "Beta")
	alphaVer, betaVer := alpha.dataVersion(t), beta.dataVersion(t)
	alphaIDs, betaIDs := alpha.rowIDs(t), beta.rowIDs(t)

	// Boot path: a second SyncAll over unchanged YAML.
	if err := syncer.SyncAll(ctx); err != nil {
		t.Fatalf("second SyncAll: %v", err)
	}
	// Per-agent path: the shard manager's spawn hook.
	for _, name := range []string{"Alpha", "Beta"} {
		skipped, err := syncer.SyncAgent(ctx, name)
		if err != nil {
			t.Fatalf("SyncAgent(%s): %v", name, err)
		}
		if !skipped {
			t.Fatalf("SyncAgent(%s) rewrote an unchanged agent", name)
		}
	}

	if got := alpha.dataVersion(t); got != alphaVer {
		t.Errorf("Alpha KB written on an unchanged sync: data_version %d -> %d", alphaVer, got)
	}
	if got := beta.dataVersion(t); got != betaVer {
		t.Errorf("Beta KB written on an unchanged sync: data_version %d -> %d", betaVer, got)
	}
	if got := alpha.rowIDs(t); got != alphaIDs {
		t.Errorf("Alpha atom rows were rewritten:\nbefore %s\nafter  %s", alphaIDs, got)
	}
	if got := beta.rowIDs(t); got != betaIDs {
		t.Errorf("Beta atom rows were rewritten:\nbefore %s\nafter  %s", betaIDs, got)
	}
}

func TestSyncAgent_ChangedYAMLReplacesAtomsAndUpdatesHash(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Gamma", agentYAML("gamma", 3, "Old guidance"))
	ctx := context.Background()
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if skipped, err := syncer.SyncAgent(ctx, "Gamma"); err != nil || skipped {
		t.Fatalf("first SyncAgent = skipped %v, err %v; want a real sync", skipped, err)
	}
	kb := observeKB(t, root, "Gamma")
	if got := kb.atomIDs(t); len(got) != 3 {
		t.Fatalf("after first sync: %d atoms, want 3", len(got))
	}

	// Edit one atom's text and drop another.
	updated := "- id: gamma/atom00\n  category: methodology\n  is_mandatory: false\n  content: |\n    New guidance\n" +
		"- id: gamma/atom01\n  category: methodology\n  is_mandatory: false\n  content: |\n    Old guidance 1\n"
	writeAgentYAML(t, root, "Gamma", updated)
	skipped, err := syncer.SyncAgent(ctx, "Gamma")
	if err != nil {
		t.Fatalf("SyncAgent after edit: %v", err)
	}
	if skipped {
		t.Fatal("a changed prompts.yaml was skipped")
	}

	atoms := kb.atomIDs(t)
	if _, ok := atoms["gamma/atom02"]; ok {
		t.Error("removed atom gamma/atom02 is still in the KB")
	}
	if atoms["gamma/atom00"] != "New guidance" {
		t.Errorf("gamma/atom00 = %q, want the edited text", atoms["gamma/atom00"])
	}
	if len(atoms) != 2 {
		t.Errorf("%d atoms after edit, want 2: %v", len(atoms), atoms)
	}
	sum := sha256.Sum256([]byte(updated))
	if got, want := kb.syncedHash(t, "Gamma"), hex.EncodeToString(sum[:]); got != want {
		t.Errorf("stored hash = %s, want the new YAML's %s", got, want)
	}

	// The new state is now the baseline: syncing again skips.
	if skipped, err := syncer.SyncAgent(ctx, "Gamma"); err != nil || !skipped {
		t.Errorf("sync after the hash update = skipped %v, err %v; want skipped", skipped, err)
	}
}

func TestSyncAgent_NoPromptsYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd", "agents", "Empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	_, err := syncer.SyncAgent(context.Background(), "Empty")
	if !errors.Is(err, ErrNoAgentPrompts) {
		t.Fatalf("SyncAgent without prompts.yaml = %v, want ErrNoAgentPrompts", err)
	}
	if got := syncer.GetDiscoveredAgents(); len(got) != 0 {
		t.Errorf("an agent with no prompts.yaml was recorded as discovered: %v", got)
	}
	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Errorf("SyncAll must pass over a directory without prompts.yaml: %v", err)
	}
}

func TestSyncAgent_RecordsDiscoveredOncePerAgent(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Delta", agentYAML("delta", 1, "Delta"))
	writeAgentYAML(t, root, "Eps", agentYAML("eps", 1, "Eps"))
	ctx := context.Background()
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))

	for i := 0; i < 2; i++ {
		if _, err := syncer.SyncAgent(ctx, "Delta"); err != nil {
			t.Fatal(err)
		}
	}
	got := syncer.GetDiscoveredAgents()
	if len(got) != 1 || got[0].ID != "Delta" {
		t.Fatalf("discovered after SyncAgent(Delta) x2 = %v, want [Delta] once", got)
	}
	wantDB := filepath.Join(root, ".nerd", "shards", "delta_knowledge.db")
	if got[0].DBPath != wantDB {
		t.Errorf("DBPath = %s, want %s", got[0].DBPath, wantDB)
	}

	// SyncAll rebuilds the list from disk and keeps both.
	if err := syncer.SyncAll(ctx); err != nil {
		t.Fatal(err)
	}
	if got := syncer.GetDiscoveredAgents(); len(got) != 2 {
		t.Fatalf("discovered after SyncAll = %v, want Delta and Eps", got)
	}
	// The returned slice is a copy.
	syncer.GetDiscoveredAgents()[0].ID = "mutated"
	if syncer.GetDiscoveredAgents()[0].ID == "mutated" {
		t.Error("GetDiscoveredAgents exposes the synchronizer's own slice")
	}
}

func TestSyncAll_BrokenAgentFailsTheSync(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Good", agentYAML("good", 1, "Good"))
	writeAgentYAML(t, root, "Broken", "invalid: [")
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	err := syncer.SyncAll(context.Background())
	if err == nil {
		t.Fatal("a broken discovered agent was reported as a successful sync")
	}
	if !strings.Contains(err.Error(), "Broken") {
		t.Errorf("error does not name the failing agent: %v", err)
	}
	if got := observeKB(t, root, "Good").atomIDs(t); len(got) != 1 {
		t.Errorf("a broken sibling stopped the good agent's sync: %v", got)
	}
}

// A knowledge DB that already holds knowledge_atoms (what init writes) keeps
// them: the sync owns prompt_atoms only.
func TestSyncAgent_KeepsExistingKnowledgeAtoms(t *testing.T) {
	root := t.TempDir()
	shards := filepath.Join(root, ".nerd", "shards")
	if err := os.MkdirAll(shards, 0o755); err != nil {
		t.Fatal(err)
	}
	kbPath := filepath.Join(shards, "testagent_knowledge.db")
	db, err := sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE knowledge_atoms (id INTEGER PRIMARY KEY, concept TEXT, content TEXT, confidence REAL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO knowledge_atoms (concept, content, confidence) VALUES ('c', 'kept', 1.0)`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	writeAgentYAML(t, root, "testAgent", "- id: test/atom\n  category: identity\n  is_mandatory: true\n  content: \"This is a test atom\"\n")
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if _, err := syncer.SyncAgent(context.Background(), "testAgent"); err != nil {
		t.Fatal(err)
	}
	kb := observeKB(t, root, "testAgent")
	if got := kb.atomIDs(t)["test/atom"]; got != "This is a test atom" {
		t.Errorf("prompt atom content = %q", got)
	}
	var kept int
	if err := kb.db.QueryRow(`SELECT COUNT(*) FROM knowledge_atoms WHERE content = 'kept'`).Scan(&kept); err != nil || kept != 1 {
		t.Errorf("knowledge_atoms row lost by the sync: count %d, err %v", kept, err)
	}
}

// The shard manager's spawn hook and boot activation share one synchronizer
// and may call it at once.
func TestSyncAgent_ConcurrentCallsAreSafe(t *testing.T) {
	root := t.TempDir()
	names := []string{"A1", "A2", "A3", "A4"}
	for _, n := range names {
		writeAgentYAML(t, root, n, agentYAML(strings.ToLower(n), 3, "Guidance"))
	}
	ctx := context.Background()
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	var wg gosync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%4 == 0 {
				errs <- syncer.SyncAll(ctx)
				return
			}
			_, err := syncer.SyncAgent(ctx, names[i%len(names)])
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent sync: %v", err)
		}
	}
	for _, n := range names {
		if got := observeKB(t, root, n).atomIDs(t); len(got) != 3 {
			t.Errorf("%s: %d atoms, want 3", n, len(got))
		}
	}
}

func TestSyncAll_LegacyProjectPromptYAMLDoesNotBreakTheSync(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Zeta", agentYAML("zeta", 1, "Zeta"))
	legacy := filepath.Join(root, ".nerd", "prompts")
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "legacy.yaml"), []byte("id: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	if got := observeKB(t, root, "Zeta").atomIDs(t); len(got) != 1 {
		t.Errorf("Zeta atoms = %v", got)
	}
}

// BenchmarkSyncAllUnchanged is the boot cost of the prompt sync on a
// workspace of 21 agents (672 atoms) whose YAML has not changed since the
// last boot: every agent is skipped by hash.
func BenchmarkSyncAllUnchanged(b *testing.B) {
	root := b.TempDir()
	for a := 0; a < 21; a++ {
		agent := fmt.Sprintf("agent%02d", a)
		writeAgentYAML(b, root, agent, agentYAML(agent, 32, strings.Repeat("Guidance text for the expert. ", 40)))
	}
	ctx := context.Background()
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if err := syncer.SyncAll(ctx); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := syncer.SyncAll(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func TestSyncAll_IgnoresStrayFilesInAgentsDir(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Good", agentYAML("good", 1, "Good"))
	if err := os.WriteFile(filepath.Join(root, ".nerd", "agents", "stray.yaml"), []byte("id: stray"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if err := syncer.SyncAll(context.Background()); err != nil {
		t.Fatalf("SyncAll: %v", err)
	}
	got := syncer.GetDiscoveredAgents()
	if len(got) != 1 || got[0].ID != "Good" {
		t.Fatalf("discovered = %v, want [Good]", got)
	}
}

func TestRecordDiscovered_UpdatesExistingIDCaseInsensitively(t *testing.T) {
	root := t.TempDir()
	writeAgentYAML(t, root, "Delta", agentYAML("delta", 1, "Delta"))
	syncer := NewAgentSynchronizer(root, prompt.NewAtomLoader(nil))
	if _, err := syncer.SyncAgent(context.Background(), "Delta"); err != nil {
		t.Fatal(err)
	}
	wantDB := filepath.Join(root, ".nerd", "shards", "delta_knowledge.db")
	syncer.recordDiscovered("DELTA", wantDB)
	got := syncer.GetDiscoveredAgents()
	if len(got) != 1 {
		t.Fatalf("discovered = %v, want one entry after case-insensitive update", got)
	}
	if got[0].DBPath != wantDB {
		t.Errorf("DBPath = %s, want %s", got[0].DBPath, wantDB)
	}
}
