package chat

import (
	"errors"

	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/verification"
)

// delegationFailure is a delegation that failed after its worker had begun.
// It is the failure the caller already got -- Unwrap returns it untouched, so
// errors.Is still finds a cancellation, a provider error or a verification
// sentinel -- together with what the worker had measured before it stopped.
// A worker that fails after tool work has changed a workspace, and the error
// alone leaves its reader to guess which files and which checks.
type delegationFailure struct {
	label string
	err   error
	state string
}

func (f *delegationFailure) Error() string {
	msg := perception.UserFailureText(f.err)
	if f.label != "" {
		msg = f.label + ": " + msg
	}
	if f.state != "" {
		msg += "\n\n" + f.state
	}
	return msg
}

func (f *delegationFailure) Unwrap() error { return f.err }

// failedDelegation is err labelled for the user and carrying what ret measured.
// A return that measured nothing leaves the message as the label and the
// error: it cannot say the worker changed nothing.
func failedDelegation(label string, err error, ret observation.Return) error {
	return &delegationFailure{label: label, err: err, state: ret.PartialState()}
}

// withPartialState is failedDelegation for a caller that words its own label.
func withPartialState(err error, ret observation.Return) error {
	return failedDelegation("", err, ret)
}

// failedVerifiedDelegation is the verified lane's failure. When the worker
// itself failed, the verifier's ExecutionError holds the observation the
// worker produced; any other verification failure has none to carry.
func failedVerifiedDelegation(label string, err error) error {
	var executed *verification.ExecutionError
	if errors.As(err, &executed) {
		return failedDelegation(label, err, executed.Return)
	}
	return &delegationFailure{label: label, err: err}
}
