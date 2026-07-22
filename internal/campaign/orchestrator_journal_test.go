package campaign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
	"runtime"
)

func TestOrchestratorJournal_Gaps(t *testing.T) {
	t.Run("Scanner Limit Exceeded", func(t *testing.T) {
		tmpDir := t.TempDir()
		o := &Orchestrator{
			nerdDir: tmpDir,
			campaign: &Campaign{
				ID: "limit-test-campaign",
			},
		}

		// payload > 8MB
		payload := bytes.Repeat([]byte("A"), 10*1024*1024)

		err := o.appendJournalEventLocked("large_payload_event", payload, "snapshot_chk_xyz")
		if err != nil {
			t.Fatalf("appendJournalEventLocked failed: %v", err)
		}

		path := o.journalPath("limit-test-campaign")

		fileInfo1, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat journal: %v", err)
		}
		size1 := fileInfo1.Size()

		o.recoverJournalSequence("limit-test-campaign")

		fileInfo2, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat journal after recovery: %v", err)
		}
		size2 := fileInfo2.Size()

		if size1 != size2 {
			t.Errorf("Journal size changed during recovery! Expected %d, got %d. Truncation occurred.", size1, size2)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read journal content: %v", err)
		}
		if !strings.Contains(string(content), "large_payload_event") {
			t.Errorf("Recovered journal missing the large event")
		}
	})
	// TODO: TEST_GAP: Missing/Nil Payload Hash Verification. Verify checksumJournalEvent deterministic hashing when ev.Payload is exactly nil vs []byte("").
	// TODO: TEST_GAP: Sequence Mismatch Truncation Test. Verify that if line N is corrupt, recovery correctly preserves 0..N-1 and safely overwrites the corrupt line.
}

func TestOrchestratorJournal_UnsupportedSyncError_Integration(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("syncDirIfSupported returns nil on windows, skipping test")
	}

	origOsSyncFile := osSyncFile
	defer func() {
		osSyncFile = origOsSyncFile
	}()

	errFuse := &os.PathError{Op: "sync", Path: "/tmp/fuse", Err: syscall.EINVAL}
	osSyncFile = func(f *os.File) error {
		return errFuse
	}

	tmpDir := t.TempDir()
	o := &Orchestrator{
		nerdDir: tmpDir,
		campaign: &Campaign{
			ID: "test-campaign-sync",
		},
	}

	err := o.appendJournalEventLocked("test_event", nil, "")
	if err != nil {
		t.Errorf("expected appendJournalEventLocked to succeed with FUSE/NFS sync error, got %v", err)
	}
}

func TestOrchestratorJournal_UnsupportedSyncError_FUSENFS(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("syncDirIfSupported returns nil on windows, skipping test")
	}

	origOsSyncFile := osSyncFile
	defer func() {
		osSyncFile = origOsSyncFile
	}()

	errFuse := &os.PathError{Op: "sync", Path: "/tmp/fuse", Err: syscall.EINVAL}
	osSyncFile = func(f *os.File) error {
		return errFuse
	}

	tmpDirFuse := t.TempDir()
	err := syncDirIfSupported(tmpDirFuse)
	if err != nil {
		t.Errorf("expected syncDirIfSupported to ignore EINVAL error, got %v", err)
	}

	errNfs := &os.PathError{Op: "sync", Path: "/tmp/nfs", Err: fmt.Errorf("operation not supported")}
	osSyncFile = func(f *os.File) error {
		return errNfs
	}

	tmpDirNfs := t.TempDir()
	err = syncDirIfSupported(tmpDirNfs)
	if err != nil {
		t.Errorf("expected syncDirIfSupported to ignore operation not supported error, got %v", err)
	}

	errPerm := &os.PathError{Op: "sync", Path: "/tmp/bad", Err: os.ErrPermission}
	osSyncFile = func(f *os.File) error {
		return errPerm
	}

	tmpDirBad := t.TempDir()
	err = syncDirIfSupported(tmpDirBad)
	if err == nil {
		t.Errorf("expected syncDirIfSupported to return permission error, got nil")
	} else if err != errPerm {
		t.Errorf("expected syncDirIfSupported to return exact permission error, got %v", err)
	}
}

func TestOrchestratorJournal_UnsupportedSyncError(t *testing.T) {
	// Test that standard sync unsupported errors are ignored (FUSE/NFS scenarios)
	// We use standard errors to avoid build failures on Windows.
	unsupportedErrs := []error{
		syscall.EINVAL,
		&os.PathError{Op: "sync", Path: "/tmp/fuse", Err: syscall.EINVAL},
		&os.PathError{Op: "sync", Path: "/tmp/nfs", Err: fmt.Errorf("operation not supported")},
	}

	for _, err := range unsupportedErrs {
		if got := ignoreUnsupportedSyncError(err); got != nil {
			t.Errorf("expected nil for unsupported error %v, got %v", err, got)
		}
	}

	// Test that other errors are returned as-is (including os.PathError context)
	otherErrs := []error{
		os.ErrPermission,
		os.ErrNotExist,
		&os.PathError{Op: "sync", Path: "/tmp/bad", Err: os.ErrPermission},
	}

	for _, err := range otherErrs {
		got := ignoreUnsupportedSyncError(err)
		if got == nil {
			t.Errorf("expected error for %v, got nil", err)
		} else if got != err {
			t.Errorf("expected exact error %v to be returned, got %v", err, got)
		}
	}
}

func TestOrchestratorJournal_ChecksumNilPayload(t *testing.T) {
	ev1 := campaignJournalEvent{
		Seq:           1,
		TimestampUnix: 1678886400,
		EventType:     "test_event",
		CampaignID:    "test_camp_1",
		Payload:       nil,
	}

	ev2 := campaignJournalEvent{
		Seq:           1,
		TimestampUnix: 1678886400,
		EventType:     "test_event",
		CampaignID:    "test_camp_1",
		Payload:       []byte(""),
	}

	hash1 := checksumJournalEvent(ev1)
	hash2 := checksumJournalEvent(ev2)

	if hash1 != hash2 {
		t.Errorf("expected hash for nil payload and empty byte slice payload to be exactly the same, got %s and %s", hash1, hash2)
	}
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
