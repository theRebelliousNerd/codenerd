package browser

import (
	"fmt"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
)

// defaultMaxEpochEventFacts bounds how many event-stream facts one session may
// assert into the kernel between epoch boundaries.
//
// A tab left open on a busy SPA produces console, network, mutation, and toast
// events indefinitely. Nothing downstream retracts them: the fact store is
// monotonic and the kernel's global fact limit is shared with planning, memory,
// and policy, so an unattended browser session could starve the executive. The
// budget is per epoch rather than per session because a navigation invalidates
// the previous page's facts anyway.
const defaultMaxEpochEventFacts = 20000

// sessionFactBudget tracks one session's event-stream volume for the live epoch.
type sessionFactBudget struct {
	epoch     int64
	asserted  int
	dropped   int
	saturated bool
	// saturatedEpochs counts retired epochs of this session that ended
	// saturated. The browser_stream_saturated notice is retracted with its
	// epoch, so this counter is the bounded receipt that stream observation
	// of the session was incomplete at some point.
	saturatedEpochs int
	// tracked holds the live epoch's facts exactly as the sink stored them
	// (post-sanitization), so they can be retracted when the epoch retires.
	// It includes the browser_epoch watermark and browser_stream_saturated
	// notice, which bypass the event budget but not the lifecycle. It is only
	// populated when a retractor is wired, because the retained copy costs
	// memory proportional to the epoch budget and is pure waste to a sink that
	// cannot remove anything.
	tracked []mangle.Fact
}

// FactRetractor removes facts a previous epoch asserted. The plain EngineSink
// is append-only: the browser has no authority to remove facts from a store it
// does not own, so real collection is opt-in and supplied by the owner.
type FactRetractor interface {
	RetractFacts(facts []mangle.Fact) error
}

// SetFactRetractor enables real garbage collection of retired epochs.
//
// Without it, RollSessionEpoch only publishes the watermark and consumers are
// expected to scope their own queries; with it, the previous page's DOM,
// network, and interaction facts are actually removed when the page goes away.
func (m *SessionManager) SetFactRetractor(retractor FactRetractor) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.retractor = retractor
}

func (m *SessionManager) factRetractor() FactRetractor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.retractor
}

// SessionFactStats reports the live epoch and the event-stream volume the
// manager has asserted and dropped for a session.
type SessionFactStats struct {
	Epoch    int64 `json:"epoch"`
	Asserted int   `json:"asserted"`
	Dropped  int   `json:"dropped"`
	Budget   int   `json:"budget"`
	// Saturated reports that the live epoch dropped stream facts: its event
	// observation is incomplete, so a quiet stream is not evidence of a quiet
	// page. SaturatedEpochs counts earlier epochs of this session that ended
	// that way.
	Saturated       bool `json:"saturated"`
	SaturatedEpochs int  `json:"saturated_epochs"`
}

func (m *SessionManager) budgetFor(sessionID string) *sessionFactBudget {
	if m.budgets == nil {
		m.budgets = make(map[string]*sessionFactBudget)
	}
	budget, ok := m.budgets[sessionID]
	if !ok {
		budget = &sessionFactBudget{epoch: 1}
		m.budgets[sessionID] = budget
	}
	return budget
}

// SessionFactStats returns the current epoch accounting for a session.
func (m *SessionManager) SessionFactStats(sessionID string) SessionFactStats {
	m.budgetMu.Lock()
	defer m.budgetMu.Unlock()
	budget := m.budgetFor(sessionID)
	return SessionFactStats{
		Epoch:    budget.epoch,
		Asserted: budget.asserted,
		Dropped:  budget.dropped,
		Budget:   m.cfg.GetMaxEpochEventFacts(),

		Saturated:       budget.saturated,
		SaturatedEpochs: budget.saturatedEpochs,
	}
}

// PendingRetirements reports how many exact facts a failed retraction left
// behind for retry. Zero means every fact the manager has retired is gone.
func (m *SessionManager) PendingRetirements() int {
	m.budgetMu.Lock()
	defer m.budgetMu.Unlock()
	return len(m.pendingRetire)
}

// PublishSessionFacts asserts facts a caller observed for an open session
// through the manager's own budgeted, sanitized path. The facts are counted
// against the epoch budget and retracted exactly when the epoch rolls or the
// session closes, so a caller-side sensor cannot leave transient evidence in
// the kernel that the manager never learned about.
func (m *SessionManager) PublishSessionFacts(sessionID string, facts []mangle.Fact) error {
	m.mu.RLock()
	_, open := m.sessions[sessionID]
	m.mu.RUnlock()
	if !open {
		return fmt.Errorf("publish session facts: session %q is not open", sessionID)
	}
	return m.addStreamFacts(sessionID, facts)
}

// retireFacts removes exact facts through the owner's retractor. A failed
// retraction keeps the same exact facts pending so the next retirement retries
// them; they are never widened to a predicate or session sweep. Without a
// retractor nothing was tracked, so there is nothing to retire.
func (m *SessionManager) retireFacts(retired []mangle.Fact) error {
	retractor := m.factRetractor()
	if retractor == nil {
		return nil
	}
	m.budgetMu.Lock()
	batch := append(m.pendingRetire, retired...)
	m.pendingRetire = nil
	m.budgetMu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := retractor.RetractFacts(batch); err != nil {
		m.budgetMu.Lock()
		m.pendingRetire = append(batch, m.pendingRetire...)
		m.budgetMu.Unlock()
		return fmt.Errorf("retract %d browser facts: %w", len(batch), err)
	}
	return nil
}

// publishMarker asserts a lifecycle marker that bypasses the event budget
// (browser_epoch, browser_stream_saturated) and tracks it with the live epoch
// so it is retired with the epoch instead of outliving the page it describes.
func (m *SessionManager) publishMarker(sessionID string, fact mangle.Fact) error {
	safe := m.sanitizeFacts([]mangle.Fact{fact})
	collecting := m.factRetractor() != nil
	if collecting {
		m.budgetMu.Lock()
		budget := m.budgetFor(sessionID)
		budget.tracked = append(budget.tracked, safe...)
		m.budgetMu.Unlock()
	}
	return m.publishSanitized(safe)
}

// RollSessionEpoch retires the current epoch and starts a new one.
//
// The epoch number is the garbage-collection watermark the kernel reasons
// with: facts asserted under a retired epoch describe a page that no longer
// exists, so a consumer may scope queries to browser_epoch's latest value and
// collect anything older. The manager cannot retract them itself - EngineSink
// is append-only by design, since the browser has no authority to remove facts
// another subsystem may have derived from.
func (m *SessionManager) RollSessionEpoch(sessionID string) int64 {
	if sessionID == "" {
		return 0
	}
	// Serialize navigation retirement with progressive Observe replacements so
	// an old-page snapshot cannot be asserted after the epoch is advanced.
	m.progressiveMu.Lock()
	m.budgetMu.Lock()
	budget := m.budgetFor(sessionID)
	budget.epoch++
	if budget.saturated {
		budget.saturatedEpochs++
	}
	budget.asserted = 0
	budget.dropped = 0
	budget.saturated = false
	retired := budget.tracked
	budget.tracked = nil
	epoch := budget.epoch
	m.budgetMu.Unlock()
	if snapshot, ok := m.progressiveSnapshots[sessionID]; ok {
		retired = append(retired, snapshot.facts...)
		delete(m.progressiveSnapshots, sessionID)
	}

	if len(retired) > 0 || m.PendingRetirements() > 0 {
		if err := m.retireFacts(retired); err != nil {
			logging.BrowserWarn("[session:%s] retiring epoch %d left facts behind: %v", sessionID, epoch-1, err)
		} else {
			logging.BrowserDebug("[session:%s] collected %d facts from epoch %d", sessionID, len(retired), epoch-1)
		}
	}

	now := time.Now()
	if err := m.publishMarker(sessionID, mangle.Fact{
		Predicate: "browser_epoch",
		Args:      []any{sessionID, epoch, now.UnixMilli()},
		Timestamp: now,
	}); err != nil {
		logging.BrowserDebug("[session:%s] browser_epoch fact error: %v", sessionID, err)
	}
	m.progressiveMu.Unlock()
	return epoch
}

// forgetSessionBudget drops accounting for a closed session so the map does not
// grow with the tab count over a long-lived process, and retires the session's
// tracked facts. A retraction failure is returned so the closer can report the
// cleanup as pending; the exact facts stay queued for the next retirement.
func (m *SessionManager) forgetSessionBudget(sessionID string) error {
	m.progressiveMu.Lock()
	defer m.progressiveMu.Unlock()
	m.budgetMu.Lock()
	var retired []mangle.Fact
	if budget := m.budgets[sessionID]; budget != nil {
		retired = append(retired, budget.tracked...)
	}
	delete(m.budgets, sessionID)
	if snapshot, ok := m.progressiveSnapshots[sessionID]; ok {
		retired = append(retired, snapshot.facts...)
	}
	delete(m.progressiveSnapshots, sessionID)
	m.budgetMu.Unlock()
	if err := m.retireFacts(retired); err != nil {
		logging.BrowserWarn("[session:%s] closing session left facts behind: %v", sessionID, err)
		return fmt.Errorf("retire session %s facts: %w", sessionID, err)
	}
	return nil
}

// addStreamFacts is the budgeted path for continuous event-stream assertions.
// One-off captures (SnapshotDOM, ReifyReact, explicit actions) go through
// addFacts directly: they are caller-initiated and already bounded.
func (m *SessionManager) addStreamFacts(sessionID string, facts []mangle.Fact) error {
	if len(facts) == 0 {
		return nil
	}
	limit := m.cfg.GetMaxEpochEventFacts()
	if limit < 0 || sessionID == "" {
		return m.addFacts(facts)
	}
	// Read before taking budgetMu: nothing may acquire the manager lock while
	// holding the budget lock, or a concurrent SetFactQuerier would deadlock.
	collecting := m.factRetractor() != nil
	// Sanitize before tracking: the sink stores the redacted spelling, and an
	// exact retraction keyed by the raw spelling would not match it.
	safe := m.sanitizeFacts(facts)

	m.budgetMu.Lock()
	budget := m.budgetFor(sessionID)
	if budget.asserted >= limit {
		budget.dropped += len(facts)
		firstSaturation := !budget.saturated
		budget.saturated = true
		epoch := budget.epoch
		m.budgetMu.Unlock()

		if firstSaturation {
			logging.BrowserWarn("[session:%s] event fact budget exhausted at epoch %d (%d facts); dropping stream facts until the next navigation", sessionID, epoch, limit)
			now := time.Now()
			// Asserted outside the event budget, not through it: the
			// saturation notice is exactly the fact that must survive
			// saturation. It is tracked so it retires with its epoch.
			if err := m.publishMarker(sessionID, mangle.Fact{
				Predicate: "browser_stream_saturated",
				Args:      []any{sessionID, epoch, int64(limit), now.UnixMilli()},
				Timestamp: now,
			}); err != nil {
				logging.BrowserDebug("[session:%s] browser_stream_saturated fact error: %v", sessionID, err)
			}
		}
		return nil
	}
	budget.asserted += len(facts)
	if collecting {
		budget.tracked = append(budget.tracked, safe...)
	}
	m.budgetMu.Unlock()

	return m.publishSanitized(safe)
}
