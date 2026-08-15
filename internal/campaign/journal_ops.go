package campaign

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Operator tooling for the durable campaign journal.
//
// saveCampaign writes the journal event BEFORE the snapshot and a second event
// after the rename commits, which is what makes a half-written campaign
// recoverable. That guarantee was unobservable: nothing outside the orchestrator
// could read the journal, so an operator staring at a campaign that would not
// resume had no way to find out whether the last snapshot committed, whether
// the tail was truncated by a crash, or whether the JSON on disk is the one the
// journal says it wrote.
//
// These functions are read-only on purpose. recoverJournalSequence already
// repairs a corrupt tail when a campaign loads; a repair tool that runs outside
// that path would be a second, subtly different truncation policy.

// JournalEventKind values that carry the snapshot protocol.
const (
	JournalEventSnapshotRequested = "snapshot_write_requested"
	JournalEventSnapshotCommitted = "snapshot_write_committed"
)

// JournalProblem is one defect found while verifying a journal.
type JournalProblem struct {
	Line   int    `json:"line"`
	Seq    uint64 `json:"seq,omitzero"`
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// JournalVerification reports the health of one campaign's journal.
type JournalVerification struct {
	CampaignID   string `json:"campaign_id"`
	JournalPath  string `json:"journal_path"`
	SnapshotPath string `json:"snapshot_path"`

	TotalLines  int    `json:"total_lines"`
	ValidEvents int    `json:"valid_events"`
	FirstSeq    uint64 `json:"first_seq,omitzero"`
	LastSeq     uint64 `json:"last_seq,omitzero"`

	// UncommittedWrites counts snapshot_write_requested events with no matching
	// commit. Exactly one at the tail is the signature of a crash between the
	// temp write and the rename; more than one means commits are being lost.
	UncommittedWrites int `json:"uncommitted_writes"`

	ExpectedSnapshotChecksum string `json:"expected_snapshot_checksum,omitzero"`
	ActualSnapshotChecksum   string `json:"actual_snapshot_checksum,omitzero"`
	SnapshotMatches          bool   `json:"snapshot_matches"`

	Problems []JournalProblem `json:"problems,omitzero"`
	Healthy  bool             `json:"healthy"`
}

// JournalReplayStep is one reconstructed state transition.
type JournalReplayStep struct {
	Seq            uint64    `json:"seq"`
	At             time.Time `json:"at"`
	EventType      string    `json:"event_type"`
	Status         string    `json:"status,omitzero"`
	CompletedTasks int       `json:"completed_tasks,omitzero"`
	TotalTasks     int       `json:"total_tasks,omitzero"`
	Path           string    `json:"path,omitzero"`
	Committed      bool      `json:"committed"`
}

// JournalReplay is the reconstructed history of a campaign.
type JournalReplay struct {
	CampaignID string              `json:"campaign_id"`
	Steps      []JournalReplayStep `json:"steps"`
	Truncated  bool                `json:"truncated,omitzero"`
	FinalState *JournalReplayStep  `json:"final_state,omitzero"`
}

func campaignJournalPath(workspace, campaignID string) string {
	return filepath.Join(workspace, ".nerd", "campaigns", campaignID+".journal.jsonl")
}

func campaignSnapshotPath(workspace, campaignID string) string {
	return filepath.Join(workspace, ".nerd", "campaigns", campaignID+".json")
}

// ListCampaignJournals returns the campaign IDs that have a journal on disk,
// newest first by journal mtime.
func ListCampaignJournals(workspace string) ([]string, error) {
	dir := filepath.Join(workspace, ".nerd", "campaigns")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read campaigns dir: %w", err)
	}

	type row struct {
		id  string
		mod time.Time
	}
	rows := make([]row, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".journal.jsonl") {
			continue
		}
		id := strings.TrimSuffix(name, ".journal.jsonl")
		mod := time.Time{}
		if info, ierr := e.Info(); ierr == nil {
			mod = info.ModTime()
		}
		rows = append(rows, row{id: id, mod: mod})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].mod.After(rows[j].mod) })

	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.id)
	}
	return ids, nil
}

// readJournalEvents decodes the journal, reporting defects instead of stopping
// at the first one. Unlike recoverJournalSequence it never writes.
func readJournalEvents(path, campaignID string) ([]campaignJournalEvent, []JournalProblem, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("open journal: %w", err)
	}
	defer f.Close()

	var (
		events   []campaignJournalEvent
		problems []JournalProblem
		lastSeq  uint64
		lineNo   int
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var ev campaignJournalEvent
		if uerr := json.Unmarshal([]byte(line), &ev); uerr != nil {
			problems = append(problems, JournalProblem{
				Line: lineNo, Kind: "unparsable",
				Detail: "line is not a JSON journal event; everything after it is unrecoverable",
			})
			break
		}
		if campaignID != "" && ev.CampaignID != campaignID {
			problems = append(problems, JournalProblem{
				Line: lineNo, Seq: ev.Seq, Kind: "wrong_campaign",
				Detail: fmt.Sprintf("event belongs to %s, not %s", ev.CampaignID, campaignID),
			})
			break
		}
		if ev.Seq != lastSeq+1 {
			problems = append(problems, JournalProblem{
				Line: lineNo, Seq: ev.Seq, Kind: "sequence_gap",
				Detail: fmt.Sprintf("expected seq %d, found %d; events were lost or reordered", lastSeq+1, ev.Seq),
			})
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
			problems = append(problems, JournalProblem{
				Line: lineNo, Seq: ev.Seq, Kind: "checksum_mismatch",
				Detail: "event content does not match its recorded checksum; the record was altered or truncated mid-write",
			})
			break
		}

		events = append(events, ev)
		lastSeq = ev.Seq
	}
	if serr := scanner.Err(); serr != nil {
		problems = append(problems, JournalProblem{
			Line: lineNo, Kind: "read_error", Detail: serr.Error(),
		})
	}
	return events, problems, lineNo, nil
}

// VerifyCampaignJournal checks a campaign's journal and the snapshot it claims
// to have written.
func VerifyCampaignJournal(workspace, campaignID string) (*JournalVerification, error) {
	if strings.TrimSpace(campaignID) == "" {
		return nil, fmt.Errorf("campaign id is required")
	}

	journalPath := campaignJournalPath(workspace, campaignID)
	snapshotPath := campaignSnapshotPath(workspace, campaignID)

	v := &JournalVerification{
		CampaignID:   campaignID,
		JournalPath:  journalPath,
		SnapshotPath: snapshotPath,
	}

	events, problems, lines, err := readJournalEvents(journalPath, campaignID)
	if err != nil {
		return nil, err
	}
	v.TotalLines = lines
	v.ValidEvents = len(events)
	v.Problems = problems
	if len(events) > 0 {
		v.FirstSeq = events[0].Seq
		v.LastSeq = events[len(events)-1].Seq
	}

	// Pair snapshot_write_requested with its commit. An unpaired request at the
	// tail is exactly what a crash between temp-write and rename leaves behind.
	pending := 0
	for _, ev := range events {
		switch ev.EventType {
		case JournalEventSnapshotRequested:
			pending++
		case JournalEventSnapshotCommitted:
			if pending > 0 {
				pending--
			} else {
				v.Problems = append(v.Problems, JournalProblem{
					Seq: ev.Seq, Kind: "orphan_commit",
					Detail: "a snapshot commit with no preceding write request; event-before-ack ordering was violated",
				})
			}
			v.ExpectedSnapshotChecksum = ev.SnapshotChecksum
		}
	}
	v.UncommittedWrites = pending

	if data, rerr := os.ReadFile(snapshotPath); rerr == nil {
		v.ActualSnapshotChecksum = checksumBytes(data)
		v.SnapshotMatches = v.ExpectedSnapshotChecksum != "" &&
			v.ActualSnapshotChecksum == v.ExpectedSnapshotChecksum
		if v.ExpectedSnapshotChecksum != "" && !v.SnapshotMatches {
			v.Problems = append(v.Problems, JournalProblem{
				Kind: "snapshot_mismatch",
				Detail: fmt.Sprintf("snapshot on disk hashes to %s but the last committed event recorded %s; "+
					"the file was modified outside the orchestrator or a commit was lost",
					shortChecksum(v.ActualSnapshotChecksum), shortChecksum(v.ExpectedSnapshotChecksum)),
			})
		}
	} else if !os.IsNotExist(rerr) {
		v.Problems = append(v.Problems, JournalProblem{
			Kind: "snapshot_unreadable", Detail: rerr.Error(),
		})
	} else if len(events) > 0 {
		v.Problems = append(v.Problems, JournalProblem{
			Kind:   "snapshot_missing",
			Detail: "the journal records writes but no campaign snapshot exists; the campaign cannot be resumed",
		})
	}

	// An uncommitted tail write is survivable — the previous snapshot is still
	// the valid one — so it is reported without failing the verification.
	v.Healthy = len(v.Problems) == 0
	return v, nil
}

func shortChecksum(sum string) string {
	if len(sum) <= 12 {
		return sum
	}
	return sum[:12]
}

// ReplayCampaignJournal reconstructs the campaign's progress history from the
// journal. limit <= 0 replays everything.
func ReplayCampaignJournal(workspace, campaignID string, limit int) (*JournalReplay, error) {
	if strings.TrimSpace(campaignID) == "" {
		return nil, fmt.Errorf("campaign id is required")
	}

	events, problems, _, err := readJournalEvents(campaignJournalPath(workspace, campaignID), campaignID)
	if err != nil {
		return nil, err
	}

	replay := &JournalReplay{CampaignID: campaignID, Truncated: len(problems) > 0}

	committed := make(map[uint64]bool, len(events))
	var lastRequestSeq uint64
	for _, ev := range events {
		switch ev.EventType {
		case JournalEventSnapshotRequested:
			lastRequestSeq = ev.Seq
		case JournalEventSnapshotCommitted:
			if lastRequestSeq != 0 {
				committed[lastRequestSeq] = true
				lastRequestSeq = 0
			}
		}
	}

	for _, ev := range events {
		if ev.EventType != JournalEventSnapshotRequested {
			continue
		}
		var payload struct {
			Status         string `json:"status"`
			CompletedTasks int    `json:"completed_tasks"`
			TotalTasks     int    `json:"total_tasks"`
		}
		if len(ev.Payload) > 0 {
			_ = json.Unmarshal(ev.Payload, &payload)
		}
		replay.Steps = append(replay.Steps, JournalReplayStep{
			Seq:            ev.Seq,
			At:             time.Unix(ev.TimestampUnix, 0).UTC(),
			EventType:      ev.EventType,
			Status:         payload.Status,
			CompletedTasks: payload.CompletedTasks,
			TotalTasks:     payload.TotalTasks,
			Committed:      committed[ev.Seq],
		})
	}

	if limit > 0 && len(replay.Steps) > limit {
		replay.Steps = replay.Steps[len(replay.Steps)-limit:]
	}
	if len(replay.Steps) > 0 {
		final := replay.Steps[len(replay.Steps)-1]
		replay.FinalState = &final
	}
	return replay, nil
}

// RenderJournalVerification formats a verification for a terminal.
func RenderJournalVerification(v *JournalVerification) string {
	if v == nil {
		return "no verification result\n"
	}
	var sb strings.Builder
	status := "HEALTHY"
	if !v.Healthy {
		status = "DEFECTS FOUND"
	}
	sb.WriteString(fmt.Sprintf("Campaign journal: %s  [%s]\n", v.CampaignID, status))
	sb.WriteString(fmt.Sprintf("  journal        : %s\n", v.JournalPath))
	sb.WriteString(fmt.Sprintf("  snapshot       : %s\n", v.SnapshotPath))
	sb.WriteString(fmt.Sprintf("  events         : %d valid of %d lines (seq %d..%d)\n",
		v.ValidEvents, v.TotalLines, v.FirstSeq, v.LastSeq))
	sb.WriteString(fmt.Sprintf("  snapshot match : %v\n", v.SnapshotMatches))
	if v.UncommittedWrites > 0 {
		sb.WriteString(fmt.Sprintf("  uncommitted    : %d write(s) requested but never committed\n", v.UncommittedWrites))
		sb.WriteString("                   (one at the tail is a crash between temp write and rename; the previous snapshot is still valid)\n")
	}
	if len(v.Problems) == 0 {
		return sb.String()
	}
	sb.WriteString("\n  Problems:\n")
	for _, p := range v.Problems {
		loc := ""
		if p.Line > 0 {
			loc = fmt.Sprintf(" line %d", p.Line)
		}
		if p.Seq > 0 {
			loc += fmt.Sprintf(" seq %d", p.Seq)
		}
		sb.WriteString(fmt.Sprintf("    [%s]%s %s\n", p.Kind, loc, p.Detail))
	}
	return sb.String()
}

// RenderJournalReplay formats a replay for a terminal.
func RenderJournalReplay(r *JournalReplay) string {
	if r == nil {
		return "no replay result\n"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Campaign journal replay: %s (%d snapshot points)\n", r.CampaignID, len(r.Steps)))
	if r.Truncated {
		sb.WriteString("  NOTE: the journal has a damaged tail; history beyond it is unavailable.\n")
	}
	sb.WriteString("\n  SEQ  WHEN                 STATUS        TASKS      COMMITTED\n")
	for _, s := range r.Steps {
		sb.WriteString(fmt.Sprintf("  %-4d %-20s %-13s %4d/%-5d %v\n",
			s.Seq, s.At.Format("2006-01-02 15:04:05"), s.Status,
			s.CompletedTasks, s.TotalTasks, s.Committed))
	}
	if r.FinalState != nil && !r.FinalState.Committed {
		sb.WriteString("\n  The last recorded write never committed. Resume will fall back to the previous snapshot.\n")
	}
	return sb.String()
}
