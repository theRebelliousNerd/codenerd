package types

import "strings"

// ToolValidationState is what a tool call's post-action validation
// established about the result it measured. It is not a verdict on the task:
// a verified write can still be the wrong write, and acceptance stays a
// separate decision.
//
// Until 2026-10-27 the interactive bridge returned an error or nil, and nil
// meant "verified or no opinion": a canceled validator, a validator with
// nothing to check and an affirmative pass all crossed into the session as the
// same nil. Unknown must not become passed, so the four states cross typed.
type ToolValidationState string

const (
	// ToolValidationVerified: every validator that ran completed, at least
	// one affirmed the result, and none failed definitively.
	ToolValidationVerified ToolValidationState = "verified"
	// ToolValidationRejected: a validator failed definitively (at or above
	// DefinitiveValidationConfidence). The side effect may still have
	// landed: a failed postcondition is not proof of no mutation.
	ToolValidationRejected ToolValidationState = "rejected"
	// ToolValidationIncomplete: a validator the result owed could not
	// complete (canceled, timed out), so nothing affirmative was measured.
	ToolValidationIncomplete ToolValidationState = "incomplete"
	// ToolValidationUnvalidated: no validator holds an opinion about this
	// tool (none registered, or each one deferred). Visible, not a defect.
	ToolValidationUnvalidated ToolValidationState = "unvalidated"
	// ToolValidationNotRun: validation never ran on the result, because the
	// handler failed before reporting one. Set by the caller, never by a
	// validator registry.
	ToolValidationNotRun ToolValidationState = "not_run"
)

// Atom is the state as the Mangle name constant the policy corpus joins on.
func (s ToolValidationState) Atom() MangleAtom {
	if s == "" {
		return MangleAtom("/" + string(ToolValidationNotRun))
	}
	return MangleAtom("/" + string(s))
}

// Affirmative reports whether the state is an affirmative measurement.
func (s ToolValidationState) Affirmative() bool { return s == ToolValidationVerified }

// ValidatorObservation is one validator's reading of one resource.
type ValidatorObservation struct {
	// Resource is what was measured (a path for file tools).
	Resource string
	// Validator names the validator; Method how it measured.
	Validator string
	Method    string
	// State is pass, fail, skipped or unavailable (core.ValidationState).
	State      string
	Confidence float64
	Detail     string
}

// ToolValidation is the typed outcome of one tool call's post-action
// validation: the aggregate state and every validator reading behind it, so a
// low-confidence failure ahead of a definitive one cannot hide it.
type ToolValidation struct {
	State        ToolValidationState
	Reason       string
	Observations []ValidatorObservation
}

// Merge folds another resource's validation into this one, for a call that
// changed several resources. The worse state wins: rejected over incomplete
// over not_run over verified over unvalidated.
func (v ToolValidation) Merge(o ToolValidation) ToolValidation {
	out := ToolValidation{Observations: append(append([]ValidatorObservation(nil), v.Observations...), o.Observations...)}
	if toolValidationRank(o.State) > toolValidationRank(v.State) {
		out.State, out.Reason = o.State, o.Reason
	} else {
		out.State, out.Reason = v.State, v.Reason
	}
	if out.State == "" {
		out.State = ToolValidationUnvalidated
	}
	return out
}

func toolValidationRank(s ToolValidationState) int {
	switch s {
	case ToolValidationRejected:
		return 5
	case ToolValidationIncomplete:
		return 4
	case ToolValidationNotRun:
		return 3
	case ToolValidationVerified:
		return 2
	case ToolValidationUnvalidated:
		return 1
	}
	return 0
}

// Summary is the validation in one line for a model-facing tool result.
func (v ToolValidation) Summary() string {
	reason := strings.TrimSpace(v.Reason)
	if reason == "" {
		return string(v.State)
	}
	return string(v.State) + ": " + reason
}
