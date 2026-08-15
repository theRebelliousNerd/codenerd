package browser

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"

	"github.com/go-rod/rod"
)

// SetFactQuerier binds the read side of the fact substrate.
//
// Detection is only half a safety feature: the manager can assert element
// evidence through any sink, but without a way to read back what the kernel
// derived it cannot act on the verdict. Sinks that are themselves queryable
// (engineAdapter) are wired at construction; a write-only sink needs this.
func (m *SessionManager) SetFactQuerier(querier FactQuerier) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.querier = querier
}

func (m *SessionManager) factQuerier() FactQuerier {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.querier
}

// managerFactStore routes honeypot evidence through the manager's own
// redacting fact path and reads verdicts back from the bound querier.
type managerFactStore struct {
	manager *SessionManager
	querier FactQuerier
}

func (s managerFactStore) PushFact(predicate string, args ...any) error {
	return s.manager.addFacts([]mangle.Fact{{
		Predicate: predicate,
		Args:      args,
		Timestamp: time.Now(),
	}})
}

func (s managerFactStore) QueryFacts(predicate string, args ...string) []mangle.Fact {
	return s.querier.QueryFacts(predicate, args...)
}

// HoneypotDetector returns a detector bound to this manager's fact substrate,
// or nil when the manager has no way to read derived facts back.
func (m *SessionManager) HoneypotDetector() *HoneypotDetector {
	querier := m.factQuerier()
	if querier == nil {
		return nil
	}
	return NewHoneypotDetector(managerFactStore{manager: m, querier: querier})
}

// ErrHoneypotBlocked is returned when the guard refuses an interaction.
var ErrHoneypotBlocked = fmt.Errorf("interaction refused: element is a honeypot")

// guardElement consults the kernel before an interaction touches el.
//
// Failure modes are deliberately asymmetric. The guard fails OPEN when there is
// no query path or the evidence pass errors: absence of evidence is not
// evidence of a trap, and a deployment with no honeypot rules loaded must still
// be able to drive a browser. It fails CLOSED only on an affirmative
// is_honeypot verdict.
//
// Every verdict - blocked or merely warned - is asserted as interaction_blocked
// so the refusal is visible to planners and the glass box rather than being a
// Go-local error string.
func (m *SessionManager) guardElement(sessionID, action string, el *rod.Element) error {
	mode := m.cfg.GetHoneypotGuard()
	if mode == HoneypotGuardOff || el == nil {
		return nil
	}
	detector := m.HoneypotDetector()
	if detector == nil {
		return nil
	}

	isHoneypot, reasons, err := detector.checkElement(el)
	if err != nil {
		logging.BrowserDebug("honeypot guard skipped for %s: %v", action, err)
		return nil
	}
	if !isHoneypot {
		return nil
	}

	summary := strings.Join(reasons, "; ")
	if summary == "" {
		summary = "is_honeypot"
	}
	reason := fmt.Sprintf("honeypot %s: %s", action, summary)
	now := time.Now()
	if factErr := m.addFacts(blockedInteractionFacts(sessionID, reason, now)); factErr != nil {
		logging.BrowserDebug("interaction_blocked fact error: %v", factErr)
	}

	if mode == HoneypotGuardWarn {
		logging.BrowserWarn("[session:%s] %s (honeypot_guard=warn, proceeding)", sessionID, reason)
		return nil
	}
	logging.BrowserWarn("[session:%s] %s", sessionID, reason)
	return fmt.Errorf("%w (%s)", ErrHoneypotBlocked, summary)
}

// blockedInteractionFacts is the kernel-visible record of a refusal. Split out
// so the schema contract test can type-check it without a live page.
func blockedInteractionFacts(sessionID, reason string, now time.Time) []mangle.Fact {
	return []mangle.Fact{
		{Predicate: "interaction_blocked", Args: []any{sessionID, reason}, Timestamp: now},
		{Predicate: "interaction_blocked_at", Args: []any{sessionID, reason, now.UnixMilli()}, Timestamp: now},
	}
}
