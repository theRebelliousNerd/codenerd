package approval

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"codenerd/internal/atomicfile"
)

// journal is the durable record of every transition: one JSON line holding
// the request's full snapshot after the change. Replay keeps the last snapshot
// per request. Each append is fsynced before the transition is visible, so a
// dispatch is never started on a claim the disk does not hold.
type journal struct {
	path string
	file *os.File
}

type journalLine struct {
	At      time.Time `json:"at"`
	Request Request   `json:"request"`
}

// openJournal replays path, reconciles every record a restart left open, and
// compacts the file to one line per retained request. Records older than
// retention whose status is terminal are dropped: their continuation is long
// gone and no transition leads out of a terminal status.
func openJournal(path string, now time.Time, retention time.Duration) (*journal, map[string]*Request, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("approval journal directory: %w", err)
	}
	records, err := replayJournal(path)
	if err != nil {
		return nil, nil, err
	}
	kept := make(map[string]*Request, len(records))
	for id, r := range records {
		reconcileAfterRestart(r, now)
		if r.Status.Terminal() && now.Sub(r.CreatedAt) > retention {
			continue
		}
		kept[id] = r
	}
	var buf bytes.Buffer
	for _, r := range sortedRequests(kept) {
		line, err := json.Marshal(journalLine{At: now, Request: *r})
		if err != nil {
			return nil, nil, fmt.Errorf("encode approval journal: %w", err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := atomicfile.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return nil, nil, fmt.Errorf("compact approval journal: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("open approval journal: %w", err)
	}
	return &journal{path: path, file: file}, kept, nil
}

func replayJournal(path string) (map[string]*Request, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]*Request{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read approval journal: %w", err)
	}
	records := make(map[string]*Request)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var line journalLine
		// A torn last line (a crash mid-append) is the only damage an append
		// can leave; it carries no committed transition, so it is skipped.
		if err := json.Unmarshal(raw, &line); err != nil || line.Request.ID == "" || !line.Request.Status.valid() {
			continue
		}
		r := line.Request
		records[r.ID] = &r
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan approval journal: %w", err)
	}
	return records, nil
}

// reconcileAfterRestart closes what a restart left open without inventing a
// decision. The executor's continuation is the goroutine that was waiting, so
// after a restart there is nothing to resume: a request still awaiting a human
// or approved but unclaimed expires; a claim that never started dispatch is a
// known no-dispatch failure; a dispatch that started has an unknown outcome,
// and is never replayed.
func reconcileAfterRestart(r *Request, now time.Time) {
	switch r.Status {
	case StatusAwaitingHuman, StatusApproved:
		r.Status = StatusExpired
		r.StatusReason = "codeNERD restarted before the approved call ran; the waiting call no longer exists"
	case StatusReserved:
		r.Status = StatusAdmissionFailed
		r.StatusReason = "codeNERD restarted after the claim and before dispatch started; nothing was dispatched"
		if r.Claim != nil {
			r.Claim.FinishedAt = now
			r.Claim.Outcome = "not_started"
		}
	case StatusDispatchStarted:
		r.Status = StatusOutcomeUnknown
		r.StatusReason = "codeNERD restarted after dispatch started and before the outcome was recorded; the effect is unknown and the call is not replayed"
		if r.Claim != nil {
			r.Claim.FinishedAt = now
			r.Claim.Outcome = "unknown"
		}
	}
}

func (j *journal) append(r *Request, at time.Time) error {
	if j == nil {
		return nil
	}
	line, err := json.Marshal(journalLine{At: at, Request: *r})
	if err != nil {
		return fmt.Errorf("encode approval transition: %w", err)
	}
	line = append(line, '\n')
	if _, err := j.file.Write(line); err != nil {
		return fmt.Errorf("write approval journal: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		return fmt.Errorf("sync approval journal: %w", err)
	}
	return nil
}

func (j *journal) close() error {
	if j == nil || j.file == nil {
		return nil
	}
	return j.file.Close()
}

func sortedRequests(m map[string]*Request) []*Request {
	out := make([]*Request, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}
