package browser

import (
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
}

// SessionFactStats reports the live epoch and the event-stream volume the
// manager has asserted and dropped for a session.
type SessionFactStats struct {
	Epoch    int64 `json:"epoch"`
	Asserted int   `json:"asserted"`
	Dropped  int   `json:"dropped"`
	Budget   int   `json:"budget"`
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
	}
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
	m.budgetMu.Lock()
	budget := m.budgetFor(sessionID)
	budget.epoch++
	budget.asserted = 0
	budget.dropped = 0
	budget.saturated = false
	epoch := budget.epoch
	m.budgetMu.Unlock()

	now := time.Now()
	if err := m.addFacts([]mangle.Fact{{
		Predicate: "browser_epoch",
		Args:      []any{sessionID, epoch, now.UnixMilli()},
		Timestamp: now,
	}}); err != nil {
		logging.BrowserDebug("[session:%s] browser_epoch fact error: %v", sessionID, err)
	}
	return epoch
}

// forgetSessionBudget drops accounting for a closed session so the map does not
// grow with the tab count over a long-lived process.
func (m *SessionManager) forgetSessionBudget(sessionID string) {
	m.budgetMu.Lock()
	defer m.budgetMu.Unlock()
	delete(m.budgets, sessionID)
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
			// Asserted through addFacts, not addStreamFacts: the saturation
			// notice is exactly the fact that must survive saturation.
			if err := m.addFacts([]mangle.Fact{{
				Predicate: "browser_stream_saturated",
				Args:      []any{sessionID, epoch, int64(limit), now.UnixMilli()},
				Timestamp: now,
			}}); err != nil {
				logging.BrowserDebug("[session:%s] browser_stream_saturated fact error: %v", sessionID, err)
			}
		}
		return nil
	}
	budget.asserted += len(facts)
	m.budgetMu.Unlock()

	return m.addFacts(facts)
}
