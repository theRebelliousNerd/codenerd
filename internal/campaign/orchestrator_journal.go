package campaign

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type campaignJournalEvent struct {
	Seq              uint64          `json:"seq"`
	TimestampUnix    int64           `json:"timestamp_unix"`
	EventType        string          `json:"event_type"`
	CampaignID       string          `json:"campaign_id"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	SnapshotChecksum string          `json:"snapshot_checksum,omitempty"`
	Checksum         string          `json:"checksum"`
}

func (o *Orchestrator) journalPath(campaignID string) string {
	return filepath.Join(o.nerdDir, "campaigns", campaignID+".journal.jsonl")
}

func (o *Orchestrator) appendJournalEventLocked(eventType string, payload any, snapshotChecksum string) error {
	if o.campaign == nil {
		return nil
	}
	campaignID := o.campaign.ID
	seq := o.journalSeq.Add(1)

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

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write journal event: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync journal event: %w", err)
	}
	if err := syncDirIfSupported(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync journal dir: %w", err)
	}
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

func (o *Orchestrator) recoverJournalSequence(campaignID string) {
	path := o.journalPath(campaignID)
	o.journalSeq.Store(0)

	f, err := os.Open(path)
	if err != nil {
		return
	}

	validLines := make([]string, 0)
	var lastSeq uint64
	needsTruncate := false

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev campaignJournalEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			needsTruncate = true
			break
		}
		if ev.CampaignID != campaignID {
			needsTruncate = true
			break
		}
		if ev.Seq != lastSeq+1 {
			needsTruncate = true
			break
		}
		expectedChecksum := checksumJournalEvent(campaignJournalEvent{
			Seq:              ev.Seq,
			TimestampUnix:    ev.TimestampUnix,
			EventType:        ev.EventType,
			CampaignID:       ev.CampaignID,
			Payload:          ev.Payload,
			SnapshotChecksum: ev.SnapshotChecksum,
		})
		if ev.Checksum != expectedChecksum {
			needsTruncate = true
			break
		}
		validLines = append(validLines, line)
		lastSeq = ev.Seq
	}

	if scanErr := scanner.Err(); scanErr != nil {
		needsTruncate = true
	}
	_ = f.Close()

	if needsTruncate {
		_ = o.writeJournalLinesAtomic(path, validLines)
	}
	o.journalSeq.Store(lastSeq)
}

func (o *Orchestrator) writeJournalLinesAtomic(path string, lines []string) error {
	var data []byte
	if len(lines) > 0 {
		data = append(data, []byte(strings.Join(lines, "\n"))...)
		data = append(data, '\n')
	}
	if err := o.writeCampaignSnapshotAtomic(path, data); err == nil {
		return nil
	}

	// Recovery fallback: rewrite in place if atomic replacement is unavailable.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	return syncDirIfSupported(filepath.Dir(path))
}

func renameAtomicReplace(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else {
		if removeErr := os.Remove(dst); removeErr == nil || os.IsNotExist(removeErr) {
			if retryErr := os.Rename(src, dst); retryErr == nil {
				return nil
			} else {
				return retryErr
			}
		}
		return err
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
	return d.Sync()
}
