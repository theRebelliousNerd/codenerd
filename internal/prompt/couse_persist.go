package prompt

import (
	"time"

	"codenerd/internal/jsonl"
)

// Co-use asks which atoms are selected together across real sessions, so the
// evidence has to outlive the process that produced it. A settled selection is
// appended to a rotating JSONL log; the readout replays the log into a fresh
// recorder and analyses that.
//
// Selections are written at Settle rather than at Observe because an unsettled
// selection is not evidence: the whole question is about turns that succeeded.

// DefaultSelectionLogName is where selections land under the workspace .nerd dir.
const DefaultSelectionLogName = "meter/atom-selections.jsonl"

// RecordedAtom is one atom as it appeared in a selection.
type RecordedAtom struct {
	ID string `json:"id"`
	// Category is the atom's category at selection time. It is written per
	// record rather than kept in a sidecar so a log stays self-describing: a
	// later corpus edit cannot retroactively recategorize evidence that was
	// already gathered, and a half-copied pair of files cannot silently
	// mismatch.
	Category string `json:"category,omitempty"`
}

// SelectionRecord is one settled prompt compilation.
type SelectionRecord struct {
	At      time.Time      `json:"at"`
	Outcome Outcome        `json:"outcome"`
	Atoms   []RecordedAtom `json:"atoms"`
}

// SetLog installs a persistence log. Passing nil detaches it.
func (r *CoUseRecorder) SetLog(log *jsonl.Appender) {
	if r == nil {
		return
	}
	r.logMu.Lock()
	defer r.logMu.Unlock()
	r.log = log
}

// appendToLog writes one settled selection. Called without the tally mutex
// held: writing to disk under the lock that every Observe contends on would
// put file I/O on the compilation path.
func (r *CoUseRecorder) appendToLog(atoms []string, outcome Outcome) {
	r.logMu.RLock()
	log := r.log
	r.logMu.RUnlock()
	if log == nil {
		return
	}

	cats := r.Categories()
	rec := SelectionRecord{At: time.Now().UTC(), Outcome: outcome, Atoms: make([]RecordedAtom, 0, len(atoms))}
	for _, id := range atoms {
		rec.Atoms = append(rec.Atoms, RecordedAtom{ID: id, Category: cats(id)})
	}
	log.Append(rec)
}

// LoadSelections replays a selection log into a fresh recorder.
//
// The replayed recorder is settled by construction: every record in the log was
// written at Settle, so there is nothing pending and nothing to wait for. It
// reports the number of generations that ended on a truncated line so a reader
// can tell a short sample from a damaged one.
func LoadSelections(path string) (*CoUseRecorder, int, error) {
	records, truncated, err := jsonl.Read[SelectionRecord](path)
	if err != nil {
		return nil, truncated, err
	}

	rec := NewCoUseRecorder()
	for i := range records {
		record := &records[i]
		if len(record.Atoms) == 0 {
			continue
		}
		ids := make([]string, 0, len(record.Atoms))
		for _, a := range record.Atoms {
			if a.ID == "" {
				continue
			}
			ids = append(ids, a.ID)
			rec.noteCategory(a.ID, a.Category)
		}
		if len(ids) == 0 {
			continue
		}
		// Replay through the normal path so the loaded tallies are produced by
		// the same code that produces live ones. A separate ingest path is how
		// a readout starts disagreeing with the process it is reading.
		turnID := "replay"
		rec.Observe(turnID, ids)
		rec.Settle(turnID, record.Outcome)
	}
	return rec, truncated, nil
}
