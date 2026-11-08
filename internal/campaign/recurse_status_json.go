package campaign

import (
	"encoding/json"
	"strings"
	"time"
)

// JSON report for `nerd campaign recurse status`: machine-readable parity with
// RecurseStatus.String plus RecurseBaseStatus.render.
//
// Names carry the RecurseStatus prefix so they cannot collide with the
// ledger types in recurse_status.go and recurse_journal.go.

type RecurseStatusAttemptJSON struct {
	Cycle           int      `json:"cycle"`
	Outcome         string   `json:"outcome,omitempty"`
	Node            string   `json:"node,omitempty"`
	Commit          string   `json:"commit,omitempty"`
	ShortCommit     string   `json:"short_commit,omitempty"`
	Signature       string   `json:"signature,omitempty"`
	Finding         string   `json:"finding,omitempty"`
	Why             string   `json:"why,omitempty"`
	Detail          string   `json:"detail,omitempty"`
	MissingEvidence []string `json:"missing_evidence,omitempty"`
}

type RecurseStatusConflictJSON struct {
	Time        time.Time `json:"time"`
	BaseCommit  string    `json:"base_commit,omitempty"`
	ShortCommit string    `json:"short_commit,omitempty"`
	Detail      string    `json:"detail,omitempty"`
}

type RecurseStatusGenerationJSON struct {
	Time        time.Time `json:"time"`
	Commit      string    `json:"commit,omitempty"`
	ShortCommit string    `json:"short_commit,omitempty"`
	Binary      string    `json:"binary,omitempty"`
	Failure     string    `json:"failure,omitempty"`
}

type RecurseStatusBaseJSON struct {
	Branch            string                        `json:"branch,omitempty"`
	Tip               string                        `json:"tip,omitempty"`
	TipShort          string                        `json:"tip_short,omitempty"`
	LastAbsorbed      string                        `json:"last_absorbed,omitempty"`
	LastAbsorbedShort string                        `json:"last_absorbed_short,omitempty"`
	LastAbsorbedAt    time.Time                     `json:"last_absorbed_at,omitempty"`
	Absorbs           int                           `json:"absorbs"`
	Conflicts         []RecurseStatusConflictJSON   `json:"conflicts,omitempty"`
	Generations       []RecurseStatusGenerationJSON `json:"generations,omitempty"`
}

type RecurseStatusJSON struct {
	Running       bool                       `json:"running"`
	StopRequested bool                       `json:"stop_requested"`
	State         string                     `json:"state"`
	Pass          int                        `json:"pass"`
	Node          string                     `json:"node,omitempty"`
	Cycles        int                        `json:"cycles"`
	Kept          int                        `json:"kept"`
	Reverted      int                        `json:"reverted"`
	Refused       int                        `json:"refused"`
	Unverified    int                        `json:"unverified"`
	OpenByPass    []int                      `json:"open_by_pass,omitempty"`
	Base          RecurseStatusBaseJSON      `json:"base"`
	Attempts      []RecurseStatusAttemptJSON `json:"attempts,omitempty"`
}

// recurseStatusMissingEvidence surfaces the unverified-evidence vocabulary
// that text reports only imply via the unverified outcome: the atoms named in
// policy/coder_safety.mg turn_missing_evidence and carried on turns as
// MissingEvidence. It scans the ratchet's Why and Detail, so an empty Why and
// Detail yields no evidence rather than a guess.
func recurseStatusMissingEvidence(why string, detail string) []string {
	hay := why + "\n" + detail
	var out []string
	if strings.Contains(hay, "/changed_code_unexecuted") {
		out = append(out, "/changed_code_unexecuted")
	}
	if strings.Contains(hay, "/check_not_green") {
		out = append(out, "/check_not_green")
	}
	if strings.Contains(hay, "/tests_not_green") {
		out = append(out, "/tests_not_green")
	}
	return out
}

// ToRecurseStatusJSON converts a ledger-derived RecurseStatus into its JSON
// report. Every commit hash also carries its shortCommit form; shortCommit is
// nil-safe for empty hashes (empty stays empty, never panics).
func (s RecurseStatus) ToRecurseStatusJSON() RecurseStatusJSON {
	state := "not running"
	if s.Running {
		state = "running"
	}
	if s.StopRequested {
		state += ", stop requested"
	}
	out := RecurseStatusJSON{
		Running:       s.Running,
		StopRequested: s.StopRequested,
		State:         state,
		Pass:          s.Pass,
		Node:          s.Node,
		Cycles:        s.Cycles,
		Kept:          s.Kept,
		Reverted:      s.Reverted,
		Refused:       s.Refused,
		Unverified:    s.Unverified,
		OpenByPass:    append([]int(nil), s.OpenByPass...),
	}
	out.Base = RecurseStatusBaseJSON{
		Branch:            s.Base.Branch,
		Tip:               s.Base.TipAtStart,
		TipShort:          shortCommit(s.Base.TipAtStart),
		LastAbsorbed:      s.Base.LastAbsorbed,
		LastAbsorbedShort: shortCommit(s.Base.LastAbsorbed),
		LastAbsorbedAt:    s.Base.LastAbsorbedAt,
		Absorbs:           s.Base.Absorbs,
	}
	for _, c := range s.Base.Conflicts {
		out.Base.Conflicts = append(out.Base.Conflicts, RecurseStatusConflictJSON{
			Time:        c.Time,
			BaseCommit:  c.BaseCommit,
			ShortCommit: shortCommit(c.BaseCommit),
			Detail:      c.Detail,
		})
	}
	for _, g := range s.Base.Generations {
		out.Base.Generations = append(out.Base.Generations, RecurseStatusGenerationJSON{
			Time:        g.Time,
			Commit:      g.Commit,
			ShortCommit: shortCommit(g.Commit),
			Binary:      g.Binary,
			Failure:     g.Failure,
		})
	}
	for _, r := range s.ThisPass {
		out.Attempts = append(out.Attempts, RecurseStatusAttemptJSON{
			Cycle:           r.Cycle,
			Outcome:         r.Outcome,
			Node:            r.Node,
			Commit:          r.Commit,
			ShortCommit:     shortCommit(r.Commit),
			Signature:       r.Signature,
			Finding:         r.Finding,
			Why:             r.Why,
			Detail:          r.Detail,
			MissingEvidence: recurseStatusMissingEvidence(r.Why, r.Detail),
		})
	}
	return out
}

// MarshalRecurseStatusJSON marshals the status report for --json output.
//
// Baseline pin: DTOs cover text-report parity (running state, pass, attempt
// counts by outcome, base/last absorbed commit, conflicts, generations, each
// attempt cycle/outcome/node/commit with shortCommit nil-safety). Full-suite
// failures in Northstar adapter, RelevanceInFlight, and ExecutiveLiteralBudget
// are pre-existing and out of scope; verify via narrowed
// go test ./internal/campaign -run RecurseStatus|JSON.
func (s RecurseStatus) MarshalRecurseStatusJSON() ([]byte, error) {
	return json.Marshal(s.ToRecurseStatusJSON())
}
