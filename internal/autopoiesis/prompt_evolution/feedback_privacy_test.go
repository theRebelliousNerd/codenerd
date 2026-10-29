package prompt_evolution

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFeedbackExcludesProviderThoughtSummaries(t *testing.T) {
	collector, err := NewFeedbackCollector(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer collector.Close()
	record := &ExecutionRecord{TaskID: "privacy", ShardType: "/coder", Timestamp: time.Now()}
	// Old serialized events may still contain this field. New records must
	// neither retain nor re-publish it.
	if err := json.Unmarshal([]byte(`{"thought_summary":"private-summary-marker"}`), record); err != nil {
		t.Fatal(err)
	}
	if _, err := collector.Record(record); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := collector.db.QueryRow(`SELECT thought_summary FROM execution_records WHERE task_id = ?`, record.TaskID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != "" {
		t.Fatalf("provider thought text was persisted: %q", stored)
	}
	// Simulate a row written by an older binary. Its retired column must not
	// become learning material when the record is retrieved.
	if _, err := collector.db.Exec(`UPDATE execution_records SET thought_summary = ? WHERE task_id = ?`, "legacy-private-marker", record.TaskID); err != nil {
		t.Fatal(err)
	}
	rows, err := collector.GetRecentByShardType("/coder", 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("read legacy record: %v, %v", rows, err)
	}
	data, err := json.Marshal(rows[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-marker") || strings.Contains(string(data), "thought_summary") {
		t.Fatalf("legacy provider summary entered the learning record: %s", data)
	}
}
