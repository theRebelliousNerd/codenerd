package campaign

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type campaignJournalEvent struct {
	Seq              uint64          `json:"seq"`
	TimestampUnix    int64           `json:"timestamp_unix"`
	EventType        string          `json:"event_type"`
	CampaignID       string          `json:"campaign_id"`
	Payload          json.RawMessage `json:"payload,omitzero"`
	SnapshotChecksum string          `json:"snapshot_checksum,omitzero"`
	Checksum         string          `json:"checksum"`
}

func (o *Orchestrator) journalPath(campaignID string) string {
	return filepath.Join(o.nerdDir, "campaigns", campaignID+".journal.jsonl")
}

// appendJournalEventLocked appends one durable journal record. The caller
// holds o.mu.
//
// The sequence advances only once the record is written and synced. An append
// that fails before any byte could land (marshal, open) consumes nothing, so a
// later success cannot leave a gap that makes recovery discard every record
// after it. An append that fails after it may have written (write, file sync,
// directory sync) is not "absent": its bytes may be on disk, torn or whole. It
// marks the journal uncertain, and the next append first reconciles — re-reads
// the journal, preserves and cuts a torn tail, adopts a record that did land,
// and re-syncs — instead of guessing by decrementing or retrying blind.
func (o *Orchestrator) appendJournalEventLocked(eventType string, payload any, snapshotChecksum string) error {
	if o.campaign == nil {
		return nil
	}
	if o.journalUncertain {
		if err := o.reconcileJournalLocked(); err != nil {
			return fmt.Errorf("journal append refused: an earlier append's durability is unresolved: %w", err)
		}
	}
	campaignID := o.campaign.ID
	seq := o.journalSeq.Load() + 1

	var rawPayload json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal journal payload: %w", err)
		}
		rawPayload = b
	}

	ev := campaignJournalEvent{
		Seq:              seq,
		TimestampUnix:    time.Now().Unix(),
		EventType:        eventType,
		CampaignID:       campaignID,
		Payload:          rawPayload,
		SnapshotChecksum: snapshotChecksum,
	}
	ev.Checksum = checksumJournalEvent(ev)

	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal journal event: %w", err)
	}

	path := o.journalPath(campaignID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create journal dir: %w", err)
	}

	f, err := osOpenJournal(path)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}
	defer f.Close()

	// From here a failure may have left bytes behind.
	if _, err := osWriteJournal(f, append(line, '\n')); err != nil {
		o.journalUncertain = true
		return fmt.Errorf("write journal event %d: %w", seq, err)
	}
	if err := f.Sync(); err != nil {
		o.journalUncertain = true
		return fmt.Errorf("sync journal event %d: %w", seq, err)
	}
	if err := syncDirIfSupported(filepath.Dir(path)); err != nil {
		o.journalUncertain = true
		return fmt.Errorf("sync journal dir after event %d: %w", seq, err)
	}
	o.journalSeq.Store(seq)
	return nil
}

// osOpenJournal and osWriteJournal are seams for the uncertain-append window:
// a test can only reach "the write failed after some bytes landed" by making
// the write itself fail.
var (
	osOpenJournal = func(path string) (*os.File, error) {
		return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	}
	osWriteJournal = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
)

// reconcileJournalLocked resolves an uncertain append. It re-reads the
// journal, preserves and removes a damaged tail (a torn record), re-syncs what
// remains, and only then lets the sequence continue from the last valid
// record. A record the failed append did land is adopted, not rewritten. On
// any error the journal stays uncertain and appends stay refused. The caller
// holds o.mu.
func (o *Orchestrator) reconcileJournalLocked() error {
	path := o.journalPath(o.campaign.ID)
	scan, err := scanJournal(path, o.campaign.ID)
	if err != nil {
		return fmt.Errorf("reconcile journal: %w", err)
	}
	if scan.damaged {
		if _, err := o.repairJournalLocked(path, scan); err != nil {
			return fmt.Errorf("reconcile journal: %w", err)
		}
	} else if err := syncPath(path); err != nil {
		return fmt.Errorf("reconcile journal: re-sync: %w", err)
	}
	o.journalSeq.Store(scan.lastSeq())
	o.journalUncertain = false
	return nil
}

func checksumJournalEvent(ev campaignJournalEvent) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strconv.FormatUint(ev.Seq, 10)))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(strconv.FormatInt(ev.TimestampUnix, 10)))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(ev.EventType))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(ev.CampaignID))
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write(ev.Payload)
	_, _ = h.Write([]byte("|"))
	_, _ = h.Write([]byte(ev.SnapshotChecksum))
	return hex.EncodeToString(h.Sum(nil))
}

func checksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (o *Orchestrator) writeCampaignSnapshotAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create snapshot temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTemp := true
	defer func() {
		if cleanupTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write snapshot temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync snapshot temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close snapshot temp file: %w", err)
	}
	// Verify bytes before rename to guard against partial writes on flaky filesystems.
	verifyBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("read snapshot temp file for verification: %w", err)
	}
	if checksumBytes(verifyBytes) != checksumBytes(data) {
		return fmt.Errorf("snapshot temp verification checksum mismatch")
	}

	if err := renameAtomicReplace(tmpPath, path); err != nil {
		return fmt.Errorf("atomic snapshot rename: %w", err)
	}
	cleanupTemp = false

	if err := syncDirIfSupported(dir); err != nil {
		return fmt.Errorf("sync snapshot dir: %w", err)
	}
	return nil
}

// journalScan is one read of a campaign journal: the valid prefix (events and
// their raw lines) and the defect, if any, that ended it.
type journalScan struct {
	events     []campaignJournalEvent
	lines      [][]byte
	problems   []JournalProblem
	totalLines int
	// damaged reports that a defect in the bytes (not an I/O error) ended the
	// valid prefix: an unparsable or torn record, a wrong campaign, a sequence
	// gap or a checksum mismatch. Everything after it is not trusted.
	damaged bool
}

func (s journalScan) lastSeq() uint64 {
	if len(s.events) == 0 {
		return 0
	}
	return s.events[len(s.events)-1].Seq
}

// scanJournal reads the journal at path and validates it record by record. It
// never writes. The error is an I/O failure (os.IsNotExist when there is no
// journal); defects in the content are problems on the scan. A final record
// without its newline is torn: the append that wrote it never completed, so
// its readable bytes are not evidence that it was made durable.
//
// It reads with bufio.Reader, so a record has no length cap, but it holds the
// valid prefix in memory: O(journal bytes) at recovery.
func scanJournal(path, campaignID string) (journalScan, error) {
	var scan journalScan
	f, err := os.Open(path)
	if err != nil {
		return scan, err
	}
	defer f.Close()

	var lastSeq uint64
	reader := bufio.NewReader(f)
	for {
		raw, rerr := reader.ReadBytes('\n')
		if rerr != nil && rerr != io.EOF {
			return scan, fmt.Errorf("read journal: %w", rerr)
		}
		if len(raw) == 0 && rerr == io.EOF {
			break
		}
		scan.totalLines++
		line := bytes.TrimSpace(raw)
		if len(line) == 0 {
			if rerr == io.EOF {
				break
			}
			continue
		}
		fail := func(seq uint64, kind, detail string) {
			scan.problems = append(scan.problems, JournalProblem{Line: scan.totalLines, Seq: seq, Kind: kind, Detail: detail})
			scan.damaged = true
		}
		if rerr == io.EOF {
			fail(0, "torn_record", "the last record has no newline: the append that wrote it never completed")
			break
		}

		var ev campaignJournalEvent
		if uerr := json.Unmarshal(line, &ev); uerr != nil {
			fail(0, "unparsable", "line is not a JSON journal event; everything after it is unrecoverable")
			break
		}
		// The event carries the campaign's atom ("/campaign_x"); the caller
		// names it by its file stem or a trimmed flag ("campaign_x"). Both
		// are one campaign when their filesystem identities agree.
		if campaignID != "" && sanitizeCampaignID(ev.CampaignID) != sanitizeCampaignID(campaignID) {
			fail(ev.Seq, "wrong_campaign", fmt.Sprintf("event belongs to %s, not %s", ev.CampaignID, campaignID))
			break
		}
		if ev.Seq != lastSeq+1 {
			fail(ev.Seq, "sequence_gap", fmt.Sprintf("expected seq %d, found %d; events were lost or reordered", lastSeq+1, ev.Seq))
			break
		}
		expected := checksumJournalEvent(campaignJournalEvent{
			Seq:              ev.Seq,
			TimestampUnix:    ev.TimestampUnix,
			EventType:        ev.EventType,
			CampaignID:       ev.CampaignID,
			Payload:          ev.Payload,
			SnapshotChecksum: ev.SnapshotChecksum,
		})
		if ev.Checksum != expected {
			fail(ev.Seq, "checksum_mismatch", "event content does not match its recorded checksum; the record was altered or truncated mid-write")
			break
		}
		scan.events = append(scan.events, ev)
		scan.lines = append(scan.lines, append([]byte(nil), line...))
		lastSeq = ev.Seq
	}
	return scan, nil
}

// recoverJournalLocked loads the journal sequence of campaignID for an
// orchestrator about to publish it, repairing a damaged tail without losing
// it. No journal is sequence 0. It returns the scan and, when it repaired,
// where the damaged original was preserved. The caller holds o.mu.
func (o *Orchestrator) recoverJournalLocked(campaignID string) (journalScan, string, error) {
	path := o.journalPath(campaignID)
	o.journalSeq.Store(0)
	o.journalUncertain = false
	scan, err := scanJournal(path, campaignID)
	if err != nil {
		if os.IsNotExist(err) {
			return scan, "", nil
		}
		return scan, "", fmt.Errorf("read campaign journal: %w", err)
	}
	preserved := ""
	if scan.damaged {
		if preserved, err = o.repairJournalLocked(path, scan); err != nil {
			return scan, "", err
		}
	}
	o.journalSeq.Store(scan.lastSeq())
	return scan, preserved, nil
}

// repairJournalLocked cuts a damaged journal back to its valid prefix. The
// original bytes are first copied, durably, to <journal>.damaged-<nanos>: the
// damaged suffix is the only record of what the failed writes were, and a
// repair that cannot keep it does not run. The rewrite is the atomic
// temp-and-rename replacement; there is no in-place O_TRUNC fallback, because
// an interrupted truncate-and-rewrite can destroy the valid prefix too. It
// returns the preserved copy's path. Preserved copies are kept until an
// operator removes them; a full disk refuses the repair instead of dropping
// the evidence.
func (o *Orchestrator) repairJournalLocked(path string, scan journalScan) (string, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("journal repair: read original: %w", err)
	}
	preserved := path + ".damaged-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := o.writeCampaignSnapshotAtomic(preserved, original); err != nil {
		return "", fmt.Errorf("journal repair refused: could not preserve the damaged journal at %s: %w", preserved, err)
	}
	var data []byte
	for _, line := range scan.lines {
		data = append(data, line...)
		data = append(data, '\n')
	}
	if err := o.writeCampaignSnapshotAtomic(path, data); err != nil {
		return preserved, fmt.Errorf("journal repair: rewrite valid prefix (damaged original kept at %s): %w", preserved, err)
	}
	return preserved, nil
}

// syncPath fsyncs an existing file and its directory.
func syncPath(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return syncDirIfSupported(filepath.Dir(path))
}

// osRenameFile is a seam, like osSyncFile above. The snapshot rename is the one
// instant where a crash can leave a campaign with a written temp file, a
// journalled write request and no commit, and that window is only reachable in
// a test by making the rename itself fail.
var osRenameFile = os.Rename

func renameAtomicReplace(src, dst string) error {
	if err := osRenameFile(src, dst); err == nil {
		return nil
	} else {
		// Windows and some network filesystems refuse to rename onto an
		// existing file, so a fallback is needed. The fallback used to be
		// `os.Remove(dst)` followed by a retry — which DELETED the last
		// committed snapshot before knowing whether the replacement would land.
		// When the retry also failed (a kill at exactly this instant, a full
		// disk, a revoked handle) the campaign was left with no snapshot at all
		// and could not be resumed: the precise loss this whole atomic-write
		// protocol exists to prevent. Found by
		// TestSaveCampaign_WhenKilledDuringSnapshotRename.
		//
		// Move the current snapshot aside instead, and put it back if the
		// replacement does not land.
		backup := dst + ".prev-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		if mvErr := osRenameFile(dst, backup); mvErr != nil {
			// Either dst does not exist (nothing to displace, and the original
			// rename failed for another reason) or the filesystem will not move
			// it. Report the original failure with dst untouched.
			return err
		}
		if retryErr := osRenameFile(src, dst); retryErr != nil {
			if restoreErr := osRenameFile(backup, dst); restoreErr != nil {
				return fmt.Errorf("snapshot replace failed (%w) and the previous snapshot could not be restored from %s: %v",
					retryErr, backup, restoreErr)
			}
			return retryErr
		}
		_ = os.Remove(backup)
		return nil
	}
}

func syncDirIfSupported(dir string) error {
	if runtime.GOOS == "windows" {
		// Windows does not reliably support directory fsync semantics.
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return ignoreUnsupportedSyncError(osSyncFile(d))
}

var osSyncFile = func(f *os.File) error {
	return f.Sync()
}

func ignoreUnsupportedSyncError(err error) error {
	if err == nil {
		return nil
	}

	// Check for standard ErrUnsupported added in Go 1.21
	if errors.Is(err, errors.ErrUnsupported) {
		return nil
	}

	// errors.Is automatically unwraps os.PathError to check the inner error.
	// We do not reassign err to keep the original os.PathError context for other errors.
	if errors.Is(err, syscall.EINVAL) {
		return nil
	}

	// ENOTSUP and EOPNOTSUPP are not defined on Windows.
	// We can check by string match if ErrUnsupported/EINVAL don't catch it.
	errStr := err.Error()
	if strings.Contains(errStr, "operation not supported") || strings.Contains(errStr, "not supported") {
		return nil
	}

	return err
}
