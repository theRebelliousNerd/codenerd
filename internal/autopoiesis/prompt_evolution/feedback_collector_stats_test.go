package prompt_evolution

import (
	"path/filepath"
	"testing"
)

// loadStats ran both COUNT queries with the Scan error dropped. A missing or
// unreadable execution_records table left totalRecorded and totalFailures at
// zero — the same numbers a collector that has genuinely recorded nothing
// reports — so the failure-driven prompt-evolution trigger simply never fired
// and nothing said why.

func TestFeedbackCollector_LoadStats_CountsRealRecords(t *testing.T) {
	fc, err := NewFeedbackCollector(filepath.Join(t.TempDir(), "feedback.db"))
	if err != nil {
		t.Fatalf("new collector: %v", err)
	}
	defer fc.Close()

	if err := fc.Record(&ExecutionRecord{
		TaskID:    "task-1",
		SessionID: "sess-1",
		ShardType: "coder",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	fc.loadStats()
	recorded, _ := fc.GetStats()
	if recorded != 1 {
		t.Errorf("expected one recorded execution, got %d", recorded)
	}
}

// A broken table must leave the counters where they were rather than zeroing
// them from a failed Scan, and it must say so.
func TestFeedbackCollector_LoadStats_BrokenTableKeepsStatsStale(t *testing.T) {
	fc, err := NewFeedbackCollector(filepath.Join(t.TempDir(), "feedback.db"))
	if err != nil {
		t.Fatalf("new collector: %v", err)
	}
	defer fc.Close()

	if err := fc.Record(&ExecutionRecord{TaskID: "task-1", SessionID: "sess-1", ShardType: "coder"}); err != nil {
		t.Fatalf("record: %v", err)
	}
	fc.loadStats()

	if _, err := fc.db.Exec("DROP TABLE execution_records"); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	fc.loadStats()

	recorded, _ := fc.GetStats()
	if recorded != 1 {
		t.Errorf("a failed stats reload silently zeroed the counters: got %d", recorded)
	}
}
