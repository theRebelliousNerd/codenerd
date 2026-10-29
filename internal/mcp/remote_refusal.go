package mcp

import "errors"

// A remote call refused before dispatch used to reach the model as "transport
// error" (ControlPlane.Call), the same words as a server that failed after the
// request left: the model could not tell a call it should correct from one it
// must not repeat, nor either from one that never had a host review
// (Docs/audits/mcp-capability-admission.md, section 4C). The authority
// boundary now says which.

// RemoteRefusal is why the authority boundary refused a remote call before
// any request reached the server.
type RemoteRefusal string

const (
	// RefusedNotAdmitted: the subject has no current host review (none
	// installed, or its schema changed since).
	RefusedNotAdmitted RemoteRefusal = "not_admitted"
	// RefusedInvalidRequest: the arguments or the reviewed target broke the
	// subject's contract.
	RefusedInvalidRequest RemoteRefusal = "invalid_request"
	// RefusedPolicyDenied: the kernel derived no permission for this exact
	// call, or a person's approval was not given or cannot be used.
	RefusedPolicyDenied RemoteRefusal = "policy_denied"
	// RefusedConfirmationMissing: the kernel requires a separate risk
	// confirmation the call did not carry.
	RefusedConfirmationMissing RemoteRefusal = "confirmation_missing"
	// RefusedStaleAuthority: the server's connection, registration or
	// catalog is not the one authority was established for.
	RefusedStaleAuthority RemoteRefusal = "stale_authority"
	// RefusedAuthorityUnavailable: the kernel or fact publication the
	// decision needs is unavailable.
	RefusedAuthorityUnavailable RemoteRefusal = "authority_unavailable"
	// RefusedAttemptConsumed: the supplied call identity was already used.
	RefusedAttemptConsumed RemoteRefusal = "attempt_consumed"
)

// RemoteRefusalError is a refusal before dispatch: nothing reached the
// server. Its message is the boundary's own; errors.Is/As reach the cause.
type RemoteRefusalError struct {
	Outcome RemoteRefusal
	Err     error
}

func (e *RemoteRefusalError) Error() string { return e.Err.Error() }
func (e *RemoteRefusalError) Unwrap() error { return e.Err }

func refused(outcome RemoteRefusal, err error) error {
	return &RemoteRefusalError{Outcome: outcome, Err: err}
}

// RefusalOf reports the refusal behind err, if err is one.
func RefusalOf(err error) (RemoteRefusal, bool) {
	var r *RemoteRefusalError
	if errors.As(err, &r) {
		return r.Outcome, true
	}
	return "", false
}
