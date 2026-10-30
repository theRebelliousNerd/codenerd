package observation

import (
	"fmt"
	"strings"
)

// EffectReceipt is one of a run's tool-effect receipts as the parent receives
// it: which call changed which files, what its post-action validation
// established, and the recovery the producer's policy derived for it
// (Docs/audits/tool-effect-receipts-and-validation-completeness.md). Without
// it a parent saw a changed path and could not tell a write validation
// rejected from one validation passed.
//
// Ruling on redaction and retention: a receipt carries content identities
// ("sha256:<hex>", "absent", or "" when unreadable), never content, so it is
// safe to serialize and retain with the return; every receipt the run
// produced is carried.
type EffectReceipt struct {
	Call string `json:"call"`
	Tool string `json:"tool"`
	// Seq is the call's place on the producer turn's write clock.
	Seq     int              `json:"seq,omitempty"`
	Effects []ResourceEffect `json:"effects,omitempty"`
	// Validation is the validation state (verified, rejected, incomplete,
	// unvalidated, not_run) and Reason why.
	Validation string `json:"validation,omitempty"`
	Reason     string `json:"reason,omitempty"`
	// HandlerError is the handler's own failure when it failed after changing
	// a file.
	HandlerError string `json:"handler_error,omitempty"`
	// Recovery is the producer policy's move for the call (/repair, /inspect,
	// /revalidate, /reconcile) when the call returned; empty when the change
	// was measured. It is history: a later call can retire it.
	Recovery string `json:"recovery,omitempty"`
	// Closure is the producer policy's reading of the call when its turn
	// closed; nil when the turn ended before its closure (an error return, a
	// partial state), and then the call-time Recovery is the latest reading.
	Closure *EffectClosure `json:"closure,omitempty"`
}

// EffectClosure is what a call still owed when its turn closed, read from the
// producer's policy (tool_effect_recovery and turn_effect_current) while the
// turn's facts still held, so the parent receives the obligation the owning
// kernel derived rather than the one cached when the call returned
// (Docs/audits/effect-recovery-closure.md).
type EffectClosure struct {
	// State is ClosureOwed, ClosureSettled, ClosureSuperseded or
	// ClosureUnknown.
	State string `json:"state"`
	// Recovery is the move still owed; set only when State is owed.
	Recovery string `json:"recovery,omitempty"`
	// Current lists the paths whose current effect is this call: a path a
	// later call replaced is not here, so a partly repaired call owes only
	// for the rest.
	Current []string `json:"current,omitempty"`
}

// Closure states.
const (
	ClosureOwed       = "owed"       // current on some path, and a recovery is owed
	ClosureSettled    = "settled"    // current, and nothing is owed
	ClosureSuperseded = "superseded" // a later effect replaced every path it changed
	ClosureUnknown    = "unknown"    // the closure could not be read
)

// ResourceEffect is one resource's identity across one call.
type ResourceEffect struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	// State is changed, unchanged or unknown.
	State string `json:"state"`
}

// Unsettled is the sentence a parent must weigh for a receipt that still owes
// a recovery, or "" when nothing is owed. With a closure, the closure decides
// and names only the paths still current; a closure that could not be read is
// said, never taken for settled. Without one, the call-time recovery stands.
func (e EffectReceipt) Unsettled() string {
	recovery := e.Recovery
	var paths []string
	for _, eff := range e.Effects {
		if eff.State != "unchanged" {
			paths = append(paths, eff.Path)
		}
	}
	if c := e.Closure; c != nil {
		switch c.State {
		case ClosureOwed:
			recovery, paths = c.Recovery, c.Current
		case ClosureUnknown:
			return fmt.Sprintf("%s (%s) changed %s; validation %s; what it owes at the turn's close could not be read (owed %q when it returned)",
				e.Tool, e.Call, strings.Join(paths, ", "), e.Validation, e.Recovery)
		default:
			return ""
		}
	}
	if recovery == "" {
		return ""
	}
	why := strings.TrimSpace(e.Reason)
	if why == "" {
		why = e.Validation
	}
	if e.HandlerError != "" {
		why = strings.TrimSpace(why + "; handler failed: " + e.HandlerError)
	}
	return fmt.Sprintf("%s (%s) changed %s; validation %s (%s); owed recovery %s",
		e.Tool, e.Call, strings.Join(paths, ", "), e.Validation, why, recovery)
}

// unsettledEffects lists the receipts a parent cannot treat as measured.
func unsettledEffects(effects []EffectReceipt) []string {
	var out []string
	for _, e := range effects {
		if s := e.Unsettled(); s != "" {
			out = append(out, "tool effect not settled: "+s)
		}
	}
	return out
}
