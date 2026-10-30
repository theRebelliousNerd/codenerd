package prompt_evolution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// EvidenceLedger is the durable evidence account of the learning loop, kept in
// evolution.db beside the execution records it accounts for.
//
// It exists because every number the loop decided with was an in-memory
// association: an atom's usage and success counts were incremented on every
// RecordExecution call, so a replayed record counted twice and a revised
// verdict counted both ways; a failure group was regenerated every cycle from
// the same unchanged failures; and nothing recorded what adaptation itself
// cost. The ledger keeps the separate facts those numbers are derived from:
//
//   - candidate_exposures: a recorded turn carried a candidate version in its
//     prompt (served AND recorded; excluded or zero-token candidates never
//     reach a turn's atom list, so they get no row).
//   - candidate_attributions: the credit one verdict version gives one
//     exposure. Unique on (event, candidate, version, verdict version); a
//     revised verdict supersedes the event's older rows instead of adding to
//     them, so counts are reproduced exactly from rows.
//   - trial_envelopes: operator-approved trials of quarantined candidates.
//   - generation_claims / consumed_evidence: which failure evidence has
//     already bought a generation, fenced against concurrent owners.
//   - adaptation_spend: what judging, generating and refining cost, kept
//     disjoint from task cost.
//
// The ledger shares the collector's *sql.DB and never closes it.
type EvidenceLedger struct {
	db *sql.DB
}

const evidenceLedgerSchema = `
CREATE TABLE IF NOT EXISTS candidate_exposures (
	event_id TEXT NOT NULL,
	candidate_id TEXT NOT NULL,
	candidate_version TEXT NOT NULL,
	exposed_at INTEGER NOT NULL,
	PRIMARY KEY (event_id, candidate_id)
);
CREATE INDEX IF NOT EXISTS idx_exposures_candidate ON candidate_exposures(candidate_id, candidate_version);

CREATE TABLE IF NOT EXISTS candidate_attributions (
	event_id TEXT NOT NULL,
	candidate_id TEXT NOT NULL,
	candidate_version TEXT NOT NULL,
	verdict_version TEXT NOT NULL,
	success INTEGER NOT NULL,
	superseded INTEGER NOT NULL DEFAULT 0,
	credited_at INTEGER NOT NULL,
	PRIMARY KEY (event_id, candidate_id, candidate_version, verdict_version)
);
CREATE INDEX IF NOT EXISTS idx_attributions_candidate ON candidate_attributions(candidate_id, candidate_version, superseded);

CREATE TABLE IF NOT EXISTS trial_envelopes (
	atom_id TEXT PRIMARY KEY,
	candidate_version TEXT NOT NULL,
	approved_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	max_exposures INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS generation_claims (
	claim_key TEXT PRIMARY KEY,
	group_key TEXT NOT NULL,
	generator_version TEXT NOT NULL,
	evidence_count INTEGER NOT NULL,
	status TEXT NOT NULL,
	owner TEXT NOT NULL,
	lease_expires_at INTEGER NOT NULL,
	claimed_at INTEGER NOT NULL,
	completed_at INTEGER,
	atom_ids_json TEXT,
	error TEXT
);

CREATE TABLE IF NOT EXISTS consumed_evidence (
	task_id TEXT NOT NULL,
	verdict_version TEXT NOT NULL,
	claim_key TEXT NOT NULL,
	PRIMARY KEY (task_id, verdict_version)
);

CREATE TABLE IF NOT EXISTS adaptation_spend (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	cycle_id TEXT NOT NULL,
	category TEXT NOT NULL,
	outcome TEXT NOT NULL,
	calls INTEGER NOT NULL,
	units INTEGER NOT NULL,
	prompt_bytes INTEGER NOT NULL,
	response_bytes INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL,
	created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_spend_category ON adaptation_spend(category, outcome);
`

// Claim states. A claim in claimStatusClaimed is owned until its lease
// expires; completed consumes its evidence; released and publish_failed
// return the evidence to the pool for a later cycle.
const (
	claimStatusClaimed       = "claimed"
	claimStatusCompleted     = "completed"
	claimStatusReleased      = "released"
	claimStatusPublishFailed = "publish_failed"
)

func newEvidenceLedger(db *sql.DB) (*EvidenceLedger, error) {
	if db == nil {
		return nil, fmt.Errorf("evidence ledger needs a database")
	}
	if _, err := db.Exec(evidenceLedgerSchema + screeningSchema); err != nil {
		return nil, fmt.Errorf("evidence ledger schema: %w", err)
	}
	return &EvidenceLedger{db: db}, nil
}

// newOwnerToken mints a fencing token for one claim attempt.
func newOwnerToken() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("owner-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// versionOf is the content version of a stored JSON payload.
func versionOf(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// =============================================================================
// Exposure and attribution
// =============================================================================

// CandidateRef is one candidate version a recorded turn carried.
type CandidateRef struct {
	ID      string
	Version string
}

// RecordExposures stores that eventID carried each candidate. Replaying the
// same event is a no-op; the version recorded first is the one credited.
func (l *EvidenceLedger) RecordExposures(eventID string, candidates []CandidateRef, at time.Time) error {
	if len(candidates) == 0 {
		return nil
	}
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range candidates {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO candidate_exposures
			(event_id, candidate_id, candidate_version, exposed_at) VALUES (?, ?, ?, ?)`,
			eventID, c.ID, c.Version, at.Unix()); err != nil {
			return fmt.Errorf("record exposure %s/%s: %w", eventID, c.ID, err)
		}
	}
	return tx.Commit()
}

// ApplyCredit attributes verdictVersion of eventID to every candidate the
// event was exposed to. Idempotent: the same (event, verdict version) adds
// nothing on replay, and a different verdict version supersedes the event's
// earlier credit. Returns the candidates whose credit changed.
func (l *EvidenceLedger) ApplyCredit(eventID, verdictVersion string, success bool) ([]CandidateRef, error) {
	if eventID == "" || verdictVersion == "" {
		return nil, nil
	}
	tx, err := l.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	superseded, err := tx.Exec(`UPDATE candidate_attributions SET superseded = 1
		WHERE event_id = ? AND verdict_version <> ? AND superseded = 0`, eventID, verdictVersion)
	if err != nil {
		return nil, fmt.Errorf("supersede credit for %s: %w", eventID, err)
	}
	successInt := 0
	if success {
		successInt = 1
	}
	inserted, err := tx.Exec(`INSERT OR IGNORE INTO candidate_attributions
		(event_id, candidate_id, candidate_version, verdict_version, success, superseded, credited_at)
		SELECT event_id, candidate_id, candidate_version, ?, ?, 0, ?
		FROM candidate_exposures WHERE event_id = ?`,
		verdictVersion, successInt, time.Now().Unix(), eventID)
	if err != nil {
		return nil, fmt.Errorf("credit %s: %w", eventID, err)
	}
	changedSuperseded, _ := superseded.RowsAffected()
	changedInserted, _ := inserted.RowsAffected()
	var changed []CandidateRef
	if changedSuperseded+changedInserted > 0 {
		rows, err := tx.Query(`SELECT candidate_id, candidate_version FROM candidate_exposures WHERE event_id = ?`, eventID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var ref CandidateRef
			if err := rows.Scan(&ref.ID, &ref.Version); err != nil {
				rows.Close()
				return nil, err
			}
			changed = append(changed, ref)
		}
		rows.Close()
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return changed, nil
}

// CandidateCounts reproduces a candidate version's usage and success counts
// from its current (non-superseded) attribution rows.
func (l *EvidenceLedger) CandidateCounts(id, version string) (uses, successes int, err error) {
	err = l.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(success), 0) FROM candidate_attributions
		WHERE candidate_id = ? AND candidate_version = ? AND superseded = 0`, id, version).Scan(&uses, &successes)
	return uses, successes, err
}

// ExposureCount is how many recorded turns carried a candidate version.
func (l *EvidenceLedger) ExposureCount(id, version string) (int, error) {
	var n int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM candidate_exposures
		WHERE candidate_id = ? AND candidate_version = ?`, id, version).Scan(&n)
	return n, err
}

// =============================================================================
// Trial envelopes
// =============================================================================

// TrialEnvelope is one operator-approved trial of one exact candidate version.
type TrialEnvelope struct {
	AtomID       string    `json:"atom_id"`
	Version      string    `json:"version"`
	ApprovedAt   time.Time `json:"approved_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	MaxExposures int       `json:"max_exposures"`
	// Exposures is the ledger's count for the pinned version, filled on read.
	Exposures int `json:"exposures"`
}

// Live reports whether the envelope can still admit at now. The kernel rule
// decides admission; this is the Go-side materialization filter and the
// operator display.
func (e TrialEnvelope) Live(now time.Time) bool {
	return now.Before(e.ExpiresAt) && e.Exposures < e.MaxExposures
}

// PutTrial creates or replaces the trial envelope for an atom.
func (l *EvidenceLedger) PutTrial(e TrialEnvelope) error {
	_, err := l.db.Exec(`INSERT INTO trial_envelopes (atom_id, candidate_version, approved_at, expires_at, max_exposures)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(atom_id) DO UPDATE SET candidate_version = excluded.candidate_version,
			approved_at = excluded.approved_at, expires_at = excluded.expires_at,
			max_exposures = excluded.max_exposures`,
		e.AtomID, e.Version, e.ApprovedAt.Unix(), e.ExpiresAt.Unix(), e.MaxExposures)
	return err
}

// DeleteTrial ends a trial. Reports whether one existed.
func (l *EvidenceLedger) DeleteTrial(atomID string) (bool, error) {
	res, err := l.db.Exec(`DELETE FROM trial_envelopes WHERE atom_id = ?`, atomID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Trials lists every envelope with its current exposure count.
func (l *EvidenceLedger) Trials() ([]TrialEnvelope, error) {
	rows, err := l.db.Query(`SELECT t.atom_id, t.candidate_version, t.approved_at, t.expires_at, t.max_exposures,
		(SELECT COUNT(*) FROM candidate_exposures e WHERE e.candidate_id = t.atom_id AND e.candidate_version = t.candidate_version)
		FROM trial_envelopes t ORDER BY t.atom_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrialEnvelope
	for rows.Next() {
		var e TrialEnvelope
		var approved, expires int64
		if err := rows.Scan(&e.AtomID, &e.Version, &approved, &expires, &e.MaxExposures, &e.Exposures); err != nil {
			return nil, err
		}
		e.ApprovedAt = time.Unix(approved, 0)
		e.ExpiresAt = time.Unix(expires, 0)
		out = append(out, e)
	}
	return out, rows.Err()
}

// =============================================================================
// Generation claims and consumed evidence
// =============================================================================

// EvidenceRef is one failure record at one verdict version.
type EvidenceRef struct {
	TaskID         string
	VerdictVersion string
}

// generationClaimKey identifies one purchase of generation: the group, its
// exact evidence set, and the generator that would consume it. Changing the
// generator version is how deliberate reanalysis of the same evidence is
// requested; it is a new key and is charged as a new purchase.
func generationClaimKey(groupKey, generatorVersion string, evidence []EvidenceRef) string {
	parts := make([]string, 0, len(evidence))
	for _, ref := range evidence {
		parts = append(parts, ref.TaskID+"@"+ref.VerdictVersion)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(groupKey + "\x00" + generatorVersion + "\x00" + strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

// ErrClaimNotOwned means the claim was taken over (its lease expired and a
// later cycle or process re-claimed it) before this owner could publish.
var ErrClaimNotOwned = errors.New("generation claim no longer owned")

// Claim tries to own a generation claim. It succeeds for a new key, a
// released or publish_failed key, or a claimed key whose lease expired.
// A completed key is never re-acquired.
func (l *EvidenceLedger) Claim(key, groupKey, generatorVersion string, evidenceCount int, owner string, lease time.Duration, now time.Time) (bool, error) {
	tx, err := l.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO generation_claims
		(claim_key, group_key, generator_version, evidence_count, status, owner, lease_expires_at, claimed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(claim_key) DO UPDATE SET status = excluded.status, owner = excluded.owner,
			lease_expires_at = excluded.lease_expires_at, claimed_at = excluded.claimed_at, error = NULL
		WHERE generation_claims.status IN (?, ?)
		   OR (generation_claims.status = ? AND generation_claims.lease_expires_at <= ?)`,
		key, groupKey, generatorVersion, evidenceCount, claimStatusClaimed, owner, now.Add(lease).Unix(), now.Unix(),
		claimStatusReleased, claimStatusPublishFailed, claimStatusClaimed, now.Unix())
	if err != nil {
		return false, fmt.Errorf("claim %s: %w", key, err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

// Release returns an owned claim's evidence to the pool (the generation call
// failed, or the cycle ran out of budget before using it).
func (l *EvidenceLedger) Release(key, owner, reason string) error {
	_, err := l.db.Exec(`UPDATE generation_claims SET status = ?, error = ? WHERE claim_key = ? AND owner = ? AND status = ?`,
		claimStatusReleased, reason, key, owner, claimStatusClaimed)
	return err
}

// Complete marks an owned claim completed and consumes its evidence in one
// transaction. The owner check is the fence: a worker whose lease was taken
// over gets ErrClaimNotOwned and must not publish.
func (l *EvidenceLedger) Complete(key, owner string, evidence []EvidenceRef, atomIDs []string, now time.Time) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ids, _ := json.Marshal(atomIDs)
	res, err := tx.Exec(`UPDATE generation_claims SET status = ?, completed_at = ?, atom_ids_json = ?, error = NULL
		WHERE claim_key = ? AND owner = ? AND status = ?`,
		claimStatusCompleted, now.Unix(), string(ids), key, owner, claimStatusClaimed)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return ErrClaimNotOwned
	}
	for _, ref := range evidence {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO consumed_evidence (task_id, verdict_version, claim_key) VALUES (?, ?, ?)`,
			ref.TaskID, ref.VerdictVersion, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PublishFailed records that a completed claim's atoms could not be stored,
// and returns its evidence to the pool: the purchase is recorded as spent,
// but the evidence was not converted into a candidate, so it is not consumed.
func (l *EvidenceLedger) PublishFailed(key, owner, reason string) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE generation_claims SET status = ?, error = ? WHERE claim_key = ? AND owner = ?`,
		claimStatusPublishFailed, reason, key, owner); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM consumed_evidence WHERE claim_key = ?`, key); err != nil {
		return err
	}
	return tx.Commit()
}

// ClaimStatus returns a claim's status ("" when absent).
func (l *EvidenceLedger) ClaimStatus(key string) (string, error) {
	var status string
	err := l.db.QueryRow(`SELECT status FROM generation_claims WHERE claim_key = ?`, key).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return status, err
}

// ClaimCounts summarizes claims by status, for stats.
func (l *EvidenceLedger) ClaimCounts() (map[string]int, error) {
	rows, err := l.db.Query(`SELECT status, COUNT(*) FROM generation_claims GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int)
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// =============================================================================
// Adaptation spend
// =============================================================================

// SpendCategory is one disjoint adaptation cost category. Task execution is
// not here: it is the task's own cost, recorded by the task.
type SpendCategory string

const (
	SpendJudge    SpendCategory = "judge"
	SpendScreen   SpendCategory = "screen"
	SpendGenerate SpendCategory = "generate"
	SpendRefine   SpendCategory = "refine"
)

// SpendOutcome classifies one spend row.
type SpendOutcome string

const (
	// SpendOK is a model call that returned.
	SpendOK SpendOutcome = "ok"
	// SpendError is a model call that failed: spent, nothing returned.
	SpendError SpendOutcome = "error"
	// SpendUnusable is a returned generation that yielded no stored candidate
	// (unparseable, or every atom invalid).
	SpendUnusable SpendOutcome = "unusable"
	// SpendDiscarded counts generated candidates dropped by a per-group or
	// per-cycle cap or refused by the admission check (Units = candidates,
	// no call).
	SpendDiscarded SpendOutcome = "discarded"
	// SpendPublishFailed counts generated candidates whose storage failed.
	SpendPublishFailed SpendOutcome = "publish_failed"
)

// SpendRow is one ledger entry.
type SpendRow struct {
	CycleID       string
	Category      SpendCategory
	Outcome       SpendOutcome
	Calls         int
	Units         int
	PromptBytes   int
	ResponseBytes int
	Duration      time.Duration
}

// RecordSpend appends one row.
func (l *EvidenceLedger) RecordSpend(r SpendRow) error {
	_, err := l.db.Exec(`INSERT INTO adaptation_spend
		(cycle_id, category, outcome, calls, units, prompt_bytes, response_bytes, duration_ms, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.CycleID, string(r.Category), string(r.Outcome), r.Calls, r.Units, r.PromptBytes, r.ResponseBytes,
		r.Duration.Milliseconds(), time.Now().Unix())
	return err
}

// SpendSummary is the per-category total of the spend ledger.
//
// Token counts are absent on purpose: the evolver's LLM interface returns text
// only, so token usage is unknown here rather than estimated. Bytes describe application payloads. Screening records the canonical
// state/question bytes; its raw response bytes and transport attempt count
// are unavailable and remain NULL in screening_receipts. UnknownResponseCalls
// makes that missing measurement visible in this aggregate view.
type SpendSummary struct {
	Category             SpendCategory `json:"category"`
	UnknownResponseCalls int           `json:"unknown_response_calls"`
	Calls                int           `json:"calls"`
	FailedCalls          int           `json:"failed_calls"`
	Unusable             int           `json:"unusable"`
	Discarded            int           `json:"discarded"`
	PublishFailed        int           `json:"publish_failed"`
	PromptBytes          int64         `json:"prompt_bytes"`
	ResponseBytes        int64         `json:"response_bytes"`
	DurationMs           int64         `json:"duration_ms"`
}

// SpendSummaries totals the ledger by category.
func (l *EvidenceLedger) SpendSummaries() ([]SpendSummary, error) {
	rows, err := l.db.Query(`SELECT category, outcome, SUM(calls), SUM(units), SUM(prompt_bytes), SUM(response_bytes), SUM(duration_ms)
		FROM adaptation_spend GROUP BY category, outcome ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byCategory := make(map[SpendCategory]*SpendSummary)
	var order []SpendCategory
	for rows.Next() {
		var category, outcome string
		var calls, units int
		var promptBytes, responseBytes, durationMs int64
		if err := rows.Scan(&category, &outcome, &calls, &units, &promptBytes, &responseBytes, &durationMs); err != nil {
			return nil, err
		}
		s := byCategory[SpendCategory(category)]
		if s == nil {
			s = &SpendSummary{Category: SpendCategory(category)}
			byCategory[s.Category] = s
			order = append(order, s.Category)
		}
		s.Calls += calls
		if s.Category == SpendScreen {
			s.UnknownResponseCalls += calls
		}
		s.PromptBytes += promptBytes
		s.ResponseBytes += responseBytes
		s.DurationMs += durationMs
		switch SpendOutcome(outcome) {
		case SpendError:
			s.FailedCalls += calls
		case SpendUnusable:
			s.Unusable += units
		case SpendDiscarded:
			s.Discarded += units
		case SpendPublishFailed:
			s.PublishFailed += units
		}
	}
	out := make([]SpendSummary, 0, len(order))
	for _, c := range order {
		out = append(out, *byCategory[c])
	}
	return out, rows.Err()
}

// =============================================================================
// Metered model client
// =============================================================================

type spendContextKey struct{}

type spendContext struct {
	cycleID  string
	category SpendCategory
}

// withSpend tags model calls made under ctx with a cycle and category.
func withSpend(ctx context.Context, cycleID string, category SpendCategory) context.Context {
	return context.WithValue(ctx, spendContextKey{}, spendContext{cycleID: cycleID, category: category})
}

// meteredLLM records every adaptation model call in the spend ledger. A call
// made without a spend context (outside a cycle) is recorded under the
// wrapper's default category with an empty cycle id.
type meteredLLM struct {
	inner    LLMClient
	ledger   *EvidenceLedger
	category SpendCategory
}

func (m *meteredLLM) record(ctx context.Context, promptBytes int, response string, err error, started time.Time) {
	if m.ledger == nil {
		return
	}
	row := SpendRow{Category: m.category, Outcome: SpendOK, Calls: 1, PromptBytes: promptBytes,
		ResponseBytes: len(response), Duration: time.Since(started)}
	if sc, ok := ctx.Value(spendContextKey{}).(spendContext); ok {
		row.CycleID = sc.cycleID
		if sc.category != "" {
			row.Category = sc.category
		}
	}
	if err != nil {
		row.Outcome = SpendError
	}
	_ = m.ledger.RecordSpend(row)
}

func (m *meteredLLM) Complete(ctx context.Context, prompt string) (string, error) {
	started := time.Now()
	out, err := m.inner.Complete(ctx, prompt)
	m.record(ctx, len(prompt), out, err, started)
	return out, err
}

func (m *meteredLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	started := time.Now()
	out, err := m.inner.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	m.record(ctx, len(systemPrompt)+len(userPrompt), out, err, started)
	return out, err
}

// GetModel forwards the wrapped client's model name, which the judge records.
func (m *meteredLLM) GetModel() string {
	if named, ok := m.inner.(interface{ GetModel() string }); ok {
		return named.GetModel()
	}
	return ""
}
