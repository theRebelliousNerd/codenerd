package broker

import (
	"fmt"
	"sync"
)

// Ledger is the one place token spend is authorized and recorded.
//
// It enforces two different kinds of limit, and the distinction matters:
//
//   - The context window is a hard provider limit. A request that exceeds it
//     does not degrade — it is rejected by the API, after being transmitted,
//     and the attempt is billed. Refusing locally is strictly cheaper.
//   - A purpose budget is local policy: a cumulative cap on what a subsystem
//     may spend. Exceeding it is a business decision, not a protocol error.
//
// Before this type there were at least seven budget authorities that could not
// see each other, two of which each believed they owned the entire configured
// window. Nothing reconciled them; the conservative hard-coded constants
// scattered through the codebase were the premium being paid for that.
//
// Safe for concurrent use.
type Ledger struct {
	mu sync.Mutex

	window        int
	outputReserve int

	budgets  map[Purpose]int64
	accounts map[Purpose]*Spend
	total    Spend
}

// LedgerConfig configures a Ledger.
type LedgerConfig struct {
	// Window is the model's total context window in tokens. A non-positive
	// value disables window enforcement, which should only happen when the
	// window genuinely is not known — every admission then reports
	// DecisionAdmitted with zero headroom, and that shows up in receipts.
	Window int
	// OutputReserve is held back from the window for the response. A request
	// that fits the window exactly still fails if the model has nowhere to
	// write its answer.
	OutputReserve int
	// Budgets are optional cumulative token caps per purpose. Absent means
	// uncapped.
	Budgets map[Purpose]int64
}

// NewLedger builds a ledger from cfg.
func NewLedger(cfg LedgerConfig) *Ledger {
	budgets := make(map[Purpose]int64, len(cfg.Budgets))
	for p, b := range cfg.Budgets {
		if b > 0 {
			budgets[p] = b
		}
	}
	return &Ledger{
		window:        cfg.Window,
		outputReserve: cfg.OutputReserve,
		budgets:       budgets,
		accounts:      make(map[Purpose]*Spend),
	}
}

// Available returns the tokens a request may occupy: the window minus the
// output reserve. Zero when no window is configured.
func (l *Ledger) Available() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.availableLocked()
}

func (l *Ledger) availableLocked() int {
	if l.window <= 0 {
		return 0
	}
	avail := l.window - l.outputReserve
	if avail < 0 {
		return 0
	}
	return avail
}

// Admit decides whether a counted request may proceed.
//
// Fail-closed is the rule: a request whose size is unknown is refused, because
// a budget that cannot be checked is a budget that is not enforced. The one
// exception is an unconfigured window, which is an explicit "we do not know the
// limit" rather than a failure to measure — that case admits and reports zero
// headroom so it is visible in every receipt rather than mistaken for a pass.
func (l *Ledger) Admit(p Purpose, count Count) Decision {
	l.mu.Lock()
	defer l.mu.Unlock()

	decision := Decision{Count: count, Window: l.window}

	if count.Tokens <= 0 {
		decision.Code = DecisionCountUnavailable
		decision.Reason = "counter produced no token count"
		return decision
	}

	avail := l.availableLocked()
	if avail > 0 {
		decision.Headroom = avail - count.Tokens
		if count.Tokens > avail {
			decision.Code = DecisionWindowExceeded
			decision.Reason = fmt.Sprintf("request of %d tokens exceeds %d available (window %d, output reserve %d)",
				count.Tokens, avail, l.window, l.outputReserve)
			return decision
		}
	}

	if limit, capped := l.budgets[p]; capped {
		spent := int64(0)
		if acct, ok := l.accounts[p]; ok {
			spent = acct.Total()
		}
		if spent+int64(count.Tokens) > limit {
			decision.Code = DecisionBudgetExhausted
			decision.Reason = fmt.Sprintf("purpose %q has spent %d of %d tokens; this request needs %d",
				p, spent, limit, count.Tokens)
			return decision
		}
	}

	decision.Allowed = true
	decision.Code = DecisionAdmitted
	return decision
}

// Record folds a completed call's actual spend into the ledger.
//
// Only provider-reported numbers reach here. Estimates are never recorded as
// spend: the ledger's balances are a statement about money, and a statement
// about money assembled from guesses is worse than no statement at all.
func (l *Ledger) Record(p Purpose, actual Spend) {
	if actual.Total() == 0 && actual.Calls == 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	acct, ok := l.accounts[p]
	if !ok {
		acct = &Spend{}
		l.accounts[p] = acct
	}
	acct.Add(actual)
	l.total.Add(actual)
}

// Total returns cumulative recorded spend across all purposes.
func (l *Ledger) Total() Spend {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total
}

// Account returns cumulative recorded spend for one purpose.
func (l *Ledger) Account(p Purpose) Spend {
	l.mu.Lock()
	defer l.mu.Unlock()
	if acct, ok := l.accounts[p]; ok {
		return *acct
	}
	return Spend{}
}

// Accounts returns a copy of every account, for observability.
func (l *Ledger) Accounts() map[Purpose]Spend {
	l.mu.Lock()
	defer l.mu.Unlock()

	out := make(map[Purpose]Spend, len(l.accounts))
	for p, acct := range l.accounts {
		out[p] = *acct
	}
	return out
}

// SetWindow updates the context window, for callers that learn the real window
// after construction (a model switch mid-session, or a capability lookup).
func (l *Ledger) SetWindow(window, outputReserve int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if window > 0 {
		l.window = window
	}
	if outputReserve >= 0 {
		l.outputReserve = outputReserve
	}
}

// Reset clears recorded spend but keeps limits. Used when a session restarts.
func (l *Ledger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.accounts = make(map[Purpose]*Spend)
	l.total = Spend{}
}
