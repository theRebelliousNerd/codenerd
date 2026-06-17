package campaign

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOrchestratorJournal_Gaps(t *testing.T) {
	// TODO: TEST_GAP: Scanner Limit Exceeded (Data Loss). Verify that journal recovery does not inadvertently truncate valid events that exceed the 8MB bufio.Scanner limit. If a payload >8MB is written, recovery must not delete it.
	// TODO: TEST_GAP: Missing/Nil Payload Hash Verification. Verify checksumJournalEvent deterministic hashing when ev.Payload is exactly nil vs []byte("").
	// TODO: TEST_GAP: FUSE/NFS syncDirIfSupported Error. Verify that the system handles Sync() returning an unsupported operation error gracefully without breaking campaign flow.
}

func TestOrchestratorJournal_SequenceMismatchTruncation(t *testing.T) {
	// Setup orchestrator with a temporary workspace
	tmpDir := t.TempDir()

	orch := &Orchestrator{
		nerdDir: tmpDir,
		campaign: &Campaign{
			ID: "test_campaign_seq",
		},
	}

	// Write some valid lines
	err := orch.appendJournalEventLocked("test_event_1", map[string]string{"foo": "bar"}, "hash1")
	if err != nil {
		t.Fatalf("failed to append valid event 1: %v", err)
	}
	err = orch.appendJournalEventLocked("test_event_2", map[string]string{"foo": "baz"}, "hash2")
	if err != nil {
		t.Fatalf("failed to append valid event 2: %v", err)
	}

	// Simulate corrupt line: Write an event with sequence 4 (skipping 3)
	// We'll just read the file, append a bad line, and then try recovery
	path := orch.journalPath(orch.campaign.ID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open journal file to append corrupt line: %v", err)
	}

	// Creating an event with sequence mismatch
	badEv := campaignJournalEvent{
		Seq:              4, // Should have been 3
		TimestampUnix:    time.Now().Unix(),
		EventType:        "test_event_bad",
		CampaignID:       orch.campaign.ID,
		Payload:          []byte(`{"bad":"data"}`),
		SnapshotChecksum: "hash_bad",
	}
	badEv.Checksum = checksumJournalEvent(badEv)

	badLine, _ := json.Marshal(badEv)
	_, err = f.Write(append(badLine, '\n'))
	f.Close()
	if err != nil {
		t.Fatalf("failed to write corrupt line: %v", err)
	}

	// Trigger recovery
	orch.recoverJournalSequence(orch.campaign.ID)

	// Verify the sequence is rolled back to 2
	if seq := orch.journalSeq.Load(); seq != 2 {
		t.Errorf("expected sequence 2 after recovery, got %d", seq)
	}

	// Verify we can safely overwrite by appending a new event
	err = orch.appendJournalEventLocked("test_event_3", map[string]string{"foo": "qux"}, "hash3")
	if err != nil {
		t.Fatalf("failed to append new event 3: %v", err)
	}

	// Read file and verify we have exactly 3 valid events
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read journal file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(contentBytes)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines in journal after recovery and new append, got %d", len(lines))
	}

	// Verify sequences of the 3 lines are 1, 2, 3
	for i, line := range lines {
		var ev campaignJournalEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("failed to parse line %d: %v", i+1, err)
		}
		if ev.Seq != uint64(i+1) {
			t.Errorf("expected sequence %d at line %d, got %d", i+1, i+1, ev.Seq)
		}
	}
}
