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
	// /revalidate, /reconcile); empty when the change was measured.
	Recovery string `json:"recovery,omitempty"`
}

// ResourceEffect is one resource's identity across one call.
type ResourceEffect struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	// State is changed, unchanged or unknown.
	State string `json:"state"`
}

// Unsettled is the sentence a parent must weigh for a receipt the producer's
// policy left a recovery on, or "" when the change was measured.
func (e EffectReceipt) Unsettled() string {
	if e.Recovery == "" {
		return ""
	}
	var paths []string
	for _, eff := range e.Effects {
		if eff.State != "unchanged" {
			paths = append(paths, eff.Path)
		}
	}
	why := strings.TrimSpace(e.Reason)
	if why == "" {
		why = e.Validation
	}
	if e.HandlerError != "" {
		why = strings.TrimSpace(why + "; handler failed: " + e.HandlerError)
	}
	return fmt.Sprintf("%s (%s) changed %s; validation %s (%s); owed recovery %s",
		e.Tool, e.Call, strings.Join(paths, ", "), e.Validation, why, e.Recovery)
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
