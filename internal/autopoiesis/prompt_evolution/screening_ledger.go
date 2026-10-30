package prompt_evolution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"codenerd/internal/learnjudge"
	"codenerd/internal/logging"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"
)

// screeningVersion is bumped for deliberate reanalysis after a contract change.
const screeningVersion = "shared-cause-cohort-v1"

const screeningSchema = `
CREATE TABLE IF NOT EXISTS evolution_cohorts (
 cohort_id TEXT PRIMARY KEY, state_json TEXT NOT NULL,
 evidence_json TEXT NOT NULL, omitted_count INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS generation_cohorts (
 claim_key TEXT PRIMARY KEY, cohort_id TEXT NOT NULL, screen_key TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS screening_observations (
 purchase_key TEXT PRIMARY KEY, cohort_id TEXT NOT NULL, contract_json TEXT NOT NULL,
 state_json TEXT NOT NULL, status TEXT NOT NULL, owner TEXT NOT NULL,
 lease_expires_at INTEGER NOT NULL, probability REAL, error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS screening_receipts (
 id INTEGER PRIMARY KEY AUTOINCREMENT, purchase_key TEXT NOT NULL,
 cycle_id TEXT NOT NULL, outcome TEXT NOT NULL, probability REAL, model TEXT NOT NULL,
 usage_json TEXT NOT NULL, usage_complete INTEGER NOT NULL, error TEXT NOT NULL,
 duration_ms INTEGER NOT NULL, created_at INTEGER NOT NULL,
 transport_attempts INTEGER, response_bytes INTEGER
);
CREATE INDEX IF NOT EXISTS idx_screen_receipt_key ON screening_receipts(purchase_key);
`

type screeningCapture struct {
	mu      sync.Mutex
	reports []usage.Report
	unknown bool
}

func (c *screeningCapture) Observed(r usage.Report) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reports = append(c.reports, r)
}
func (c *screeningCapture) UsageUnknown() { c.mu.Lock(); defer c.mu.Unlock(); c.unknown = true }
func (c *screeningCapture) snapshot() ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, _ := json.Marshal(c.reports)
	complete := len(c.reports) > 0 && !c.unknown
	for _, r := range c.reports {
		if !r.Usage.Reported() || r.Usage.Partial {
			complete = false
		}
	}
	return b, complete
}

// screenCohort returns a purchase identity and permission to continue. Cache
// misses at an exhausted allowance and active leases defer, never become a
// negative observation or invoke the unavailable-judgment fallback.
func (pe *PromptEvolver) screenCohort(ctx context.Context, cycleID string, cfg EvolverConfig, group failureGroup, c *evidenceCohort, budget *cycleBudget, result *EvolutionResult) (string, bool) {
	u := pe.jevFor()
	if u == nil || !u.uses.AtomChecks {
		if u != nil {
			logging.Get(logging.CategoryAutopoiesis).Info("Evolution trigger for %s: atom_checks is off; the failure count decides", group.key)
		}
		allowed, err := c.generationAllowed(ctx, u, "unavailable", 0)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return "count-fallback", false
		}
		return "count-fallback", allowed
	}
	contract, err := json.Marshal(struct {
		Version, Epoch, Model string
		Yes, No               float64
		Questions             map[string]typesafe.Question
	}{screeningVersion, u.screenEpoch, u.screenModel, u.yes, u.no, c.questions})
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return "", false
	}
	request, err := json.Marshal(struct {
		State     map[string]any
		Questions map[string]typesafe.Question
	}{c.state, c.questions})
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return "", false
	}
	key := versionOf(append(append([]byte(c.id), 0), contract...))
	var status, detail string
	var p sql.NullFloat64
	err = pe.ledger.db.QueryRow(`SELECT status, probability, error FROM screening_observations WHERE purchase_key = ?`, key).Scan(&status, &p, &detail)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		result.Errors = append(result.Errors, err.Error())
		return key, false
	}
	if err == nil && status != "pending" {
		result.ScreeningReused++
		return key, screeningDisposition(ctx, c, u, status, p.Float64, result)
	}
	allowed, err := c.screenAllowed(ctx, cfg.MaxCycleScreeningCalls-budget.screeningCalls)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return key, false
	}
	if !allowed || ctx.Err() != nil {
		result.GroupsDeferred++
		return key, false
	}
	owner := newOwnerToken()
	now := time.Now()
	res, err := pe.ledger.db.Exec(`INSERT INTO screening_observations
	 (purchase_key, cohort_id, contract_json, state_json, status, owner, lease_expires_at)
	 VALUES (?, ?, ?, ?, 'pending', ?, ?)
	 ON CONFLICT(purchase_key) DO UPDATE SET owner = excluded.owner, lease_expires_at = excluded.lease_expires_at
	 WHERE screening_observations.status = 'pending' AND screening_observations.lease_expires_at <= ?`,
		key, c.id, string(contract), c.json, owner, now.Add(cfg.ClaimLease).Unix(), now.Unix())
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return key, false
	}
	if n, _ := res.RowsAffected(); n != 1 {
		result.GroupsDeferred++
		return key, false
	}
	budget.screeningCalls--
	result.ScreeningCalls++
	capture := &screeningCapture{}
	started := time.Now()
	resp, callErr := learnjudge.JudgeSharedCause(usage.WithObserver(ctx, capture), u.asker, c.state, c.questions)
	probability, model := 0.0, ""
	if callErr == nil {
		probability, model = resp.Probability, resp.Model
	}
	_, complete := capture.snapshot()
	if !complete {
		usage.NoteUnknownUsage(ctx)
	}
	status, detail = "answered", ""
	if callErr != nil {
		status, detail = "unavailable", callErr.Error()
		logging.Get(logging.CategoryAutopoiesis).Warn("Evolution trigger for %s: no Jev answer, the failure count decides: %v", group.key, callErr)
	}
	if err := pe.ledger.completeScreen(key, owner, cycleID, status, probability, detail, model, capture, len(request), time.Since(started)); err != nil {
		result.Errors = append(result.Errors, "screening receipt: "+err.Error())
		return key, false
	}
	if ctx.Err() != nil {
		result.GroupsDeferred++
		return key, false
	}
	return key, screeningDisposition(ctx, c, u, status, probability, result)
}

func screeningDisposition(ctx context.Context, c *evidenceCohort, u *jevUse, status string, p float64, result *EvolutionResult) bool {
	allowed, err := c.generationAllowed(ctx, u, status, p)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return false
	}
	if status == "unavailable" {
		result.ScreeningUnavailable++
	} else if !allowed {
		result.GroupsWithoutSharedCause++
	}
	return allowed
}

// completeScreen always records the purchased attempt, including stale-owner
// spend. Only the lease owner may publish the reusable observation. No failure
// evidence is consumed by this transaction.
func (l *EvidenceLedger) completeScreen(key, owner, cycleID, status string, p float64, detail, model string, capture *screeningCapture, promptBytes int, elapsed time.Duration) error {
	b, complete := capture.snapshot()
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var observed any = p
	if status == "unavailable" {
		observed = nil
	}
	outcome := SpendOK
	if status == "unavailable" {
		outcome = SpendError
	}
	if _, err = tx.Exec(`INSERT INTO screening_receipts
	 (purchase_key, cycle_id, outcome, probability, model, usage_json, usage_complete, error, duration_ms, created_at)
	 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, key, cycleID, status, observed, model, string(b), complete, detail, elapsed.Milliseconds(), time.Now().Unix()); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO adaptation_spend
	 (cycle_id, category, outcome, calls, units, prompt_bytes, response_bytes, duration_ms, created_at)
	 VALUES (?, ?, ?, 1, 0, ?, 0, ?, ?)`, cycleID, SpendScreen, outcome, promptBytes, elapsed.Milliseconds(), time.Now().Unix()); err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE screening_observations SET status = ?, probability = ?, error = ?
	 WHERE purchase_key = ? AND owner = ? AND status = 'pending'`, status, observed, detail, key, owner)
	if err != nil {
		return err
	}
	owned, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if owned != 1 {
		return ErrClaimNotOwned
	}
	return nil
}
