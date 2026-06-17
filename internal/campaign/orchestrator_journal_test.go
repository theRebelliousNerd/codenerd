package campaign

import (
	"fmt"
	"os"
	"syscall"
	"testing"
)

func TestOrchestratorJournal_Gaps(t *testing.T) {
	// TODO: TEST_GAP: Scanner Limit Exceeded (Data Loss). Verify that journal recovery does not inadvertently truncate valid events that exceed the 8MB bufio.Scanner limit. If a payload >8MB is written, recovery must not delete it.
	// TODO: TEST_GAP: Missing/Nil Payload Hash Verification. Verify checksumJournalEvent deterministic hashing when ev.Payload is exactly nil vs []byte("").
	// TODO: TEST_GAP: Sequence Mismatch Truncation Test. Verify that if line N is corrupt, recovery correctly preserves 0..N-1 and safely overwrites the corrupt line.
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
