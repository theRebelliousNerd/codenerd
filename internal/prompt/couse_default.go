package prompt

import (
	"sync"

	"codenerd/internal/jsonl"
)

// coUse is the process-wide recorder. Selection happens in the compiler and
// outcomes are known in the session executor, two packages that have no handle
// on each other and no reason to grow one for a measurement.
var coUse = NewCoUseRecorder()

// CoUse returns the process-wide co-use recorder.
func CoUse() *CoUseRecorder { return coUse }

// ObserveAtoms records a selection from the atoms themselves, capturing each
// atom's category on the way through.
//
// Capturing categories here rather than looking them up at report time is what
// lets a readout run without a corpus handle -- and it records the category the
// atom actually had when it was selected, which is the one the selection was
// made under. A later corpus edit cannot retroactively rewrite the evidence.
func (r *CoUseRecorder) ObserveAtoms(turnID string, atoms []*PromptAtom) {
	if r == nil || len(atoms) == 0 {
		return
	}
	// An empty turnID is deliberately passed through rather than short-circuited
	// here: Observe counts it as unattributed, and swallowing it at this layer
	// would recreate exactly the silent gap that counter exists to expose.

	ids := make([]string, 0, len(atoms))
	for _, a := range atoms {
		if a == nil || a.ID == "" {
			continue
		}
		ids = append(ids, a.ID)
		r.noteCategory(a.ID, string(a.Category))
	}
	r.Observe(turnID, ids)
}

func (r *CoUseRecorder) noteCategory(id, category string) {
	if category == "" {
		return
	}
	r.catMu.Lock()
	defer r.catMu.Unlock()
	if r.categories == nil {
		r.categories = make(map[string]string)
	}
	if _, seen := r.categories[id]; !seen {
		r.categories[id] = category
	}
}

// Categories returns a CategoryLookup over the categories captured during
// recording. Safe to call concurrently with recording.
func (r *CoUseRecorder) Categories() CategoryLookup {
	if r == nil {
		return nil
	}
	return func(id string) string {
		r.catMu.RLock()
		defer r.catMu.RUnlock()
		return r.categories[id]
	}
}

// catState is embedded in CoUseRecorder. It is a separate lock from the tally
// mutex because ObserveAtoms writes it before taking the tally lock, and a
// single lock would have to be re-entered.
//
// Lock order, where both are held: tally mutex first, then catMu -- which is
// what Reset does. Nothing acquires the tally mutex while holding catMu:
// noteCategory releases catMu before Observe takes the tally mutex. Keep it
// that way.
type catState struct {
	catMu      sync.RWMutex
	categories map[string]string
}

// logState is embedded in CoUseRecorder. Its own lock, because the log is read
// on the settle path while the tally mutex is deliberately not held.
type logState struct {
	logMu sync.RWMutex
	log   *jsonl.Appender
}
