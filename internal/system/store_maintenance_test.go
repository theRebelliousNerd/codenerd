package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/store"
)

// fixedEngine embeds everything as one 4-dimension vector.
type fixedEngine struct{}

func (fixedEngine) Embed(context.Context, string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}
func (fixedEngine) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return out, nil
}
func (fixedEngine) Dimensions() int { return 4 }
func (fixedEngine) Name() string    { return "fixed" }

// The maintenance cycle heals ANN drift: a vectors row the index lost is
// indexed again by runMaintenance, not only by the next engine attach.
func TestRunMaintenance_HealsVecIndexDrift(t *testing.T) {
	ls, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	ls.SetEmbeddingEngine(fixedEngine{})
	// Let the attach-time backfill settle: GetStats reports the drift only
	// once the index exists.
	deadline := time.Now().Add(30 * time.Second)
	for {
		stats, err := ls.GetStats()
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := stats[store.StatVecIndexMissing]; ok {
			break
		}
		if time.Now().After(deadline) {
			t.Skip("sqlite-vec not available in this build")
		}
		time.Sleep(10 * time.Millisecond)
	}
	for i := 0; i < 2; i++ {
		if err := ls.StoreVectorWithEmbedding(context.Background(), fmt.Sprintf("probe-%d", i), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ls.GetDB().Exec("DELETE FROM vec_index"); err != nil {
		t.Fatal(err)
	}
	stats, _ := ls.GetStats()
	if stats[store.StatVecIndexMissing] != 2 {
		t.Fatalf("fixture: drift %d, want 2", stats[store.StatVecIndexMissing])
	}

	(&Cortex{LocalDB: ls}).runMaintenance()

	stats, _ = ls.GetStats()
	if n := stats[store.StatVecIndexMissing]; n != 0 {
		t.Fatalf("a maintenance cycle left %d rows out of the ANN index", n)
	}
}

// Every boot keeps tools.db inside the cleanup budget: ToolStore.AutoCleanup
// had no caller, so the execution journal grew without bound for anyone who
// never typed /cleanup-tools.
func TestInitFactoryToolStore_AutoCleansOverBudgetJournal(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(ws, ".nerd", "tools.db")
	ts, err := store.NewToolStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Three sessions of 150 runtime hours each: 450 against a 336-hour budget.
	hour := int64(time.Hour / time.Millisecond)
	for _, session := range []string{"first", "second", "third"} {
		if err := ts.Store(store.ToolExecution{
			CallID:           "call-" + session,
			SessionID:        session,
			ToolName:         "read_file",
			Result:           "ok",
			Success:          true,
			SessionRuntimeMs: 150 * hour,
		}); err != nil {
			t.Fatal(err)
		}
	}
	_ = ts.Close()

	bctx := &bootContext{workspace: ws}
	initFactoryToolStore(bctx)
	if bctx.toolStore == nil {
		t.Fatal("tools.db did not open")
	}
	defer bctx.toolStore.Close()

	stats, err := bctx.toolStore.GetStats()
	if err != nil {
		t.Fatal(err)
	}
	budget := store.DefaultCleanupConfig().MaxRuntimeHours
	if stats.TotalRuntimeHours > budget {
		t.Fatalf("boot left %.0f runtime hours in a journal budgeted for %.0f", stats.TotalRuntimeHours, budget)
	}
	if stats.TotalExecutions != 2 {
		t.Fatalf("boot cleanup left %d executions, want two sessions' worth", stats.TotalExecutions)
	}
}
