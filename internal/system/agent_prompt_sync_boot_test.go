package system

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/prompt"
)

func syncBootAgentYAML(marker string) string {
	return fmt.Sprintf(`- id: "SyncBoot/identity"
  category: "identity"
  subcategory: "SyncBoot"
  priority: 100
  is_mandatory: true
  description: "Identity for SyncBoot"
  content_concise: |
    %[1]s concise
  content_min: |
    %[1]s min
  content: |
    %[1]s
    You are SyncBoot, an authority on widget calibration.
`, marker)
}

// compiledAgentPrompt compiles a prompt for the agent through the booted
// session's own JIT compiler, the way a delegated task does.
func compiledAgentPrompt(t *testing.T, cortex *Cortex, agent string) string {
	t.Helper()
	result, err := cortex.JITCompiler.Compile(context.Background(), &prompt.CompilationContext{
		IntentVerb:      "/consult/" + agent,
		OperationalMode: "/active",
		TokenBudget:     32000,
		ShardID:         agent,
		ShardType:       "/" + agent,
	})
	if err != nil {
		t.Fatalf("Compile(%s): %v", agent, err)
	}
	return result.Prompt
}

// agentAtomRowIDs reads the agent's atom rows (AUTOINCREMENT ids change when
// the atom set is rewritten) from a Cortex that has been closed.
func agentAtomRowIDs(t *testing.T, ws, agent string) string {
	t.Helper()
	path := filepath.Join(ws, ".nerd", "shards", strings.ToLower(agent)+"_knowledge.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT id, atom_id FROM prompt_atoms ORDER BY id")
	if err != nil {
		t.Fatalf("read prompt_atoms: %v", err)
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

// An agent's prompts.yaml edited between boots reaches the compiler in the
// boot that follows, and a boot over unchanged YAML rewrites nothing. Boot's
// only path from YAML to knowledge DB is AgentSynchronizer.SyncAll
// (initExecutionLayer), which runs before the JIT compiler exists and before
// the agent DB is registered with it; there is no later reload.
func TestBootSyncsEditedAgentPromptsIntoTheCompilerInTheSameBoot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "SyncBoot")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeYAML := func(marker string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(syncBootAgentYAML(marker)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	boot := func() *Cortex {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		t.Cleanup(cancel)
		cortex, err := BootCortex(ctx, ws, "", nil)
		if err != nil {
			t.Fatalf("BootCortex: %v", err)
		}
		return cortex
	}

	writeYAML("SYNCBOOT_MARKER_ONE")
	first := boot()
	if got := compiledAgentPrompt(t, first, "SyncBoot"); !strings.Contains(got, "SYNCBOOT_MARKER_ONE") {
		t.Fatalf("first boot: the agent's atom is missing from the compiled prompt: %.400s", got)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	idsAfterFirst := agentAtomRowIDs(t, ws, "SyncBoot")

	writeYAML("SYNCBOOT_MARKER_TWO")
	second := boot()
	got := compiledAgentPrompt(t, second, "SyncBoot")
	if !strings.Contains(got, "SYNCBOOT_MARKER_TWO") {
		t.Errorf("second boot: the edited atom is not in the compiled prompt: %.400s", got)
	}
	if strings.Contains(got, "SYNCBOOT_MARKER_ONE") {
		t.Error("second boot: the compiler still serves the pre-edit atom")
	}

	// The shard manager's spawn hook goes through the same synchronizer: an
	// edit made mid-session reaches the KB when the agent spawns. The shard's
	// own run needs a model and is not under test, only the sync before it.
	writeYAML("SYNCBOOT_MARKER_THREE")
	spawnCtx, cancelSpawn := context.WithTimeout(context.Background(), 20*time.Second)
	_, _ = second.ShardManager.Spawn(spawnCtx, "SyncBoot", "calibrate the widget")
	cancelSpawn()
	if got := agentAtomContents(t, ws, "SyncBoot"); !strings.Contains(got, "SYNCBOOT_MARKER_THREE") || strings.Contains(got, "SYNCBOOT_MARKER_TWO") {
		t.Errorf("spawn did not sync the edited prompts.yaml into the KB: %.300s", got)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	idsAfterEdit := agentAtomRowIDs(t, ws, "SyncBoot")
	if idsAfterEdit == idsAfterFirst {
		t.Errorf("the edited YAML did not replace the atom rows: %s", idsAfterEdit)
	}

	// Count every write to the atom tables during the next boot. An upsert
	// keeps row ids, so ids alone would not show a rewrite.
	auditAtomWrites(t, ws, "SyncBoot")
	third := boot()
	if err := third.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := agentAtomRowIDs(t, ws, "SyncBoot"); got != idsAfterEdit {
		t.Errorf("a boot over unchanged YAML rewrote the agent's atoms:\nbefore %s\nafter  %s", idsAfterEdit, got)
	}
	if n := auditedAtomWrites(t, ws, "SyncBoot"); n != 0 {
		t.Errorf("a boot over unchanged YAML wrote the agent's prompt atoms %d time(s); it must be skipped by hash", n)
	}
	if err := os.RemoveAll(ws); err != nil {
		t.Logf("workspace cleanup: %v", err)
	}
}
