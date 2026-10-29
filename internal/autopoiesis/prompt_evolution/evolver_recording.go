package prompt_evolution

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/prompt"
)

// RecordAdmission is what SubmitExecution did with a record. Only
// RecordAccepted promises the record will be persisted (unless Close abandons
// it at its drain deadline, which is counted); the two rejections are final.
type RecordAdmission string

const (
	RecordAccepted       RecordAdmission = "accepted"
	RecordRejectedFull   RecordAdmission = "rejected_full"
	RecordRejectedClosed RecordAdmission = "rejected_closed"
)

// RecordingStats counts the recording queue's outcomes.
type RecordingStats struct {
	Accepted       int64 `json:"accepted"`
	RejectedFull   int64 `json:"rejected_full"`
	RejectedClosed int64 `json:"rejected_closed"`
	Persisted      int64 `json:"persisted"`
	Duplicates     int64 `json:"duplicates"`
	Failed         int64 `json:"failed"`
	Abandoned      int64 `json:"abandoned"`
	Queued         int   `json:"queued"`
	Capacity       int   `json:"capacity"`
}

type queuedRecord struct {
	record              *ExecutionRecord
	attributeStrategies bool
}

// recordQueue is the owned, bounded recording path. It replaces a goroutine
// per finished turn: those were unbounded while a cycle held the evolver
// mutex through model calls, and nothing joined them, so one could reach a
// store after Close had shut it.
type recordQueue struct {
	pe      *PromptEvolver
	mu      sync.RWMutex // guards closed and the send on items
	closed  bool
	items   chan queuedRecord
	done    chan struct{}
	abandon atomic.Bool

	accepted, rejectedFull, rejectedClosed atomic.Int64
	persisted, duplicates, failed          atomic.Int64
	abandoned                              atomic.Int64
}

func newRecordQueue(pe *PromptEvolver, capacity int) *recordQueue {
	q := &recordQueue{
		pe:    pe,
		items: make(chan queuedRecord, capacity),
		done:  make(chan struct{}),
	}
	go q.run()
	return q
}

func (q *recordQueue) submit(item queuedRecord) RecordAdmission {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		q.rejectedClosed.Add(1)
		return RecordRejectedClosed
	}
	select {
	case q.items <- item:
		q.accepted.Add(1)
		return RecordAccepted
	default:
		q.rejectedFull.Add(1)
		return RecordRejectedFull
	}
}

func (q *recordQueue) run() {
	defer close(q.done)
	for item := range q.items {
		if q.abandon.Load() {
			q.abandoned.Add(1)
			continue
		}
		outcome, err := q.pe.recordExecution(item.record)
		switch {
		case err != nil:
			q.failed.Add(1)
			logging.Get(logging.CategoryAutopoiesis).Warn(
				"Failed to record execution %s for evolution: %v", item.record.TaskID, err)
			continue
		case outcome == RecordDuplicate:
			q.duplicates.Add(1)
			continue
		}
		q.persisted.Add(1)
		if item.attributeStrategies {
			// Attribute the outcome to any learned strategy that was in this
			// turn's prompt. Only on first persistence: a replayed event must
			// not count its strategy outcome twice.
			success := item.record.Verdict != nil && item.record.Verdict.IsPass()
			q.pe.RecordStrategyOutcome(item.record.TaskID, item.record.AtomIDs, success)
		}
	}
}

// close stops admission and drains. Records still queued at the deadline are
// abandoned (counted) and close returns an error; it always waits for the
// worker to finish its in-flight record before returning.
func (q *recordQueue) close(drainTimeout time.Duration) error {
	q.mu.Lock()
	if !q.closed {
		q.closed = true
		close(q.items)
	}
	q.mu.Unlock()

	timer := time.NewTimer(drainTimeout)
	defer timer.Stop()
	select {
	case <-q.done:
		return nil
	case <-timer.C:
	}
	q.abandon.Store(true)
	<-q.done
	return fmt.Errorf("incomplete shutdown: recording queue did not drain within %v; %d accepted record(s) abandoned",
		drainTimeout, q.abandoned.Load())
}

func (q *recordQueue) stats() RecordingStats {
	return RecordingStats{
		Accepted:       q.accepted.Load(),
		RejectedFull:   q.rejectedFull.Load(),
		RejectedClosed: q.rejectedClosed.Load(),
		Persisted:      q.persisted.Load(),
		Duplicates:     q.duplicates.Load(),
		Failed:         q.failed.Load(),
		Abandoned:      q.abandoned.Load(),
		Queued:         len(q.items),
		Capacity:       cap(q.items),
	}
}

// SubmitExecution hands a finished turn to the owned recording worker and
// returns at once with the admission outcome. attributeStrategies credits the
// learned strategies in the record's atom list once the record persists.
//
// This is the path for callers on a turn's own goroutine. An acknowledgment
// is not durability: RecordAccepted means queued, and GetStats().Recording
// says how many were persisted, rejected, failed or abandoned.
func (pe *PromptEvolver) SubmitExecution(exec *ExecutionRecord, attributeStrategies bool) RecordAdmission {
	if pe == nil || pe.recorder == nil || exec == nil {
		return RecordRejectedClosed
	}
	admission := pe.recorder.submit(queuedRecord{record: exec, attributeStrategies: attributeStrategies})
	if admission == RecordRejectedFull {
		logging.Get(logging.CategoryAutopoiesis).Warn(
			"Evolution recording queue full (capacity %d); turn %s not recorded",
			cap(pe.recorder.items), exec.TaskID)
	}
	return admission
}

// RecordExecution records a task execution synchronously.
//
// The observation is persisted before any credit is applied, credit is keyed
// so a replay adds nothing, and a conflicting payload under an existing event
// identity is refused. Counts are reproduced from the ledger, never
// incremented in memory.
func (pe *PromptEvolver) RecordExecution(exec *ExecutionRecord) error {
	release, err := pe.enter()
	if err != nil {
		return err
	}
	defer release()
	_, err = pe.recordExecution(exec)
	return err
}

func (pe *PromptEvolver) recordExecution(exec *ExecutionRecord) (RecordOutcome, error) {
	if exec == nil {
		return "", fmt.Errorf("execution record is nil")
	}
	if exec.ProblemType == "" {
		problemType, _ := pe.classifier.ClassifyWithContext(context.Background(), exec.TaskRequest, exec.ShardType)
		exec.ProblemType = string(problemType)
	}

	// 1. The observation, durably, first.
	outcome, err := pe.feedbackCollector.Record(exec)
	if err != nil {
		return outcome, err
	}

	// 2. Exposure: which evolved candidate versions this turn carried. Only
	// candidates the evolver knows are evolved; everything else in the atom
	// list is the ordinary corpus.
	refs := pe.candidateRefs(exec.AtomIDs)
	if err := pe.ledger.RecordExposures(exec.TaskID, refs, exec.Timestamp); err != nil {
		return outcome, fmt.Errorf("record exposures for %s: %w", exec.TaskID, err)
	}

	// 3. Credit, if the record carries a verdict.
	var changed []CandidateRef
	if exec.Verdict != nil && exec.VerdictVersion != "" {
		changed, err = pe.ledger.ApplyCredit(exec.TaskID, exec.VerdictVersion, exec.Verdict.IsPass())
		if err != nil {
			return outcome, fmt.Errorf("apply credit for %s: %w", exec.TaskID, err)
		}
	}
	pe.afterEvidence(refs, changed)
	return outcome, nil
}

// applyVerdict persists a judged verdict and then re-credits the event.
func (pe *PromptEvolver) applyVerdict(exec *ExecutionRecord, verdict *JudgeVerdict) error {
	version, _, err := pe.feedbackCollector.UpdateVerdict(exec.TaskID, verdict)
	if err != nil {
		return err
	}
	exec.Verdict = verdict
	exec.VerdictVersion = version
	changed, err := pe.ledger.ApplyCredit(exec.TaskID, version, verdict.IsPass())
	if err != nil {
		return err
	}
	pe.afterEvidence(nil, changed)
	return nil
}

// afterEvidence refreshes the counts of candidates whose evidence changed,
// runs the opt-in auto-promotion gate, and republishes trial facts when a
// trial candidate's exposure moved.
func (pe *PromptEvolver) afterEvidence(exposed, credited []CandidateRef) {
	if len(exposed) == 0 && len(credited) == 0 {
		return
	}
	var pending []promotionEvent
	trialMoved := len(exposed) > 0
	// With the promotion check on, promotion waits for the cycle, which
	// checks before it promotes (jev_promotion.go).
	promoteNow := !pe.promotionCheckOn()
	pe.mu.Lock()
	for _, ref := range credited {
		ga := pe.evolvedAtoms[ref.ID]
		if ga == nil {
			continue
		}
		pe.refreshCountsLocked(ga)
		if err := pe.persistAtomLocked(ga); err != nil {
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to persist atom usage for %s: %v", ref.ID, err)
		}
		if promoteNow && pe.config.AutoPromote && ga.PromotedAt.IsZero() && ga.ShouldPromote(pe.config.ConfidenceThreshold) {
			cb, promotedAt, err := pe.promoteAtomLocked(ref.ID)
			if err != nil {
				logging.Get(logging.CategoryAutopoiesis).Warn("Failed to auto-promote atom %s: %v", ref.ID, err)
				continue
			}
			trialMoved = true
			if cb != nil {
				pending = append(pending, promotionEvent{atomID: ref.ID, promotedAt: promotedAt, cb: cb})
			}
		}
	}
	pe.mu.Unlock()
	firePromotions(pending)
	if trialMoved {
		pe.publishTrials()
	}
}

// refreshCountsLocked reproduces a candidate's counts from the ledger for its
// current version. A candidate whose content or selectors change starts a
// new version and a new count; the old version's evidence is not its.
func (pe *PromptEvolver) refreshCountsLocked(ga *GeneratedAtom) {
	if ga == nil || ga.Atom == nil || pe.ledger == nil {
		return
	}
	uses, successes, err := pe.ledger.CandidateCounts(ga.Atom.ID, prompt.EvolvedCandidateHash(ga.Atom))
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to read evidence for %s: %v", ga.Atom.ID, err)
		return
	}
	ga.UsageCount, ga.SuccessCount = uses, successes
	ga.Confidence = ga.SuccessRate()
}

// candidateRefs resolves atom IDs to the evolved candidate versions they are.
func (pe *PromptEvolver) candidateRefs(atomIDs []string) []CandidateRef {
	if len(atomIDs) == 0 {
		return nil
	}
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	var refs []CandidateRef
	seen := make(map[string]bool, len(atomIDs))
	for _, id := range atomIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		if ga := pe.evolvedAtoms[id]; ga != nil && ga.Atom != nil {
			refs = append(refs, CandidateRef{ID: id, Version: prompt.EvolvedCandidateHash(ga.Atom)})
		}
	}
	return refs
}

func firePromotions(events []promotionEvent) {
	for _, ev := range events {
		if ev.cb != nil {
			ev.cb(ev.atomID, ev.promotedAt)
		}
	}
}
