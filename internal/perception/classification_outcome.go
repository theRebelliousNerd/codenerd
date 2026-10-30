package perception

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

// ClassificationOutcome is what happened when perception tried to read a turn:
// whether the model produced an admitted interpretation, and when it did not,
// at which stage and for which provider failure class.
//
// It is an observation, never a lane. The compatibility Intent a failed
// classification returns is a zero-confidence /explain with a safe message;
// that projection is not an interpretation of the user, and before this type
// the only machine-readable trace of the failure was one transient bit, so an
// authentication failure, an unreadable answer and a cancellation all looked
// like a request that was merely unclear. The kernel derives what each outcome
// means for the turn (policy/perception_outcome.mg); model confidence stays a
// separate fact about an admitted interpretation.
//
// The zero value is "not classified by a model" -- a delegated task's preset
// intent, a blank input -- and is neither admitted nor failed.
type ClassificationOutcome struct {
	// Disposition is the observation. Empty: no model classification ran.
	Disposition ClassificationDisposition
	// Stage is where a failed classification failed.
	Stage ClassificationStage
	// Failure is the provider failure class of a provider_failed or canceled
	// classification (provider_failure.go).
	Failure ProviderFailureClass
	// Status is the HTTP status the provider reported, when it reported one.
	Status int
	// Method is the model operation that produced the answer, or the last one
	// attempted.
	Method ClassificationMethod
	// Attempts counts the model sends this classification made (the adapters'
	// own transport retries sit inside one send). It is the accounting the
	// bounded method change below is held to.
	Attempts int
	// Code names an invalid_response in a fixed vocabulary of safe codes
	// ("no_json", "envelope_field_type", "missing:action_type"). It never
	// holds model output.
	Code string
}

// ClassificationDisposition is the observation a classification ends in.
type ClassificationDisposition string

const (
	// ClassificationAdmitted: the model's answer was read as a classification.
	// A low-confidence reading is still admitted; whether it is clear enough
	// is the kernel's (clarification.mg).
	ClassificationAdmitted ClassificationDisposition = "admitted"
	// ClassificationInvalidResponse: the model answered, and the answer is not
	// a supported classification shape.
	ClassificationInvalidResponse ClassificationDisposition = "invalid_response"
	// ClassificationProviderFailed: the model call itself failed.
	ClassificationProviderFailed ClassificationDisposition = "provider_failed"
	// ClassificationCanceled: the caller's context ended the classification.
	ClassificationCanceled ClassificationDisposition = "canceled"
)

// ClassificationStage is where a failed classification failed.
type ClassificationStage string

const (
	// StageTransport: the model call returned an error.
	StageTransport ClassificationStage = "transport"
	// StageOutput: the call succeeded and its output holds no JSON object.
	StageOutput ClassificationStage = "output"
	// StageDecode: the JSON is not a supported classification shape, or a
	// field has the wrong wire type.
	StageDecode ClassificationStage = "decode"
	// StageRequiredFields: a supported shape lacks a field the routing
	// decision reads.
	StageRequiredFields ClassificationStage = "required_fields"
)

// ClassificationMethod is the model operation a classification used.
type ClassificationMethod string

const (
	// MethodGeneric: the plain system completion.
	MethodGeneric ClassificationMethod = "generic"
	// MethodSchema: a schema-enforced completion (schemaCapableClient).
	MethodSchema ClassificationMethod = "schema"
	// MethodFreeForm: the thinking-model free-form completion, which parses the
	// last JSON object after the thoughts.
	MethodFreeForm ClassificationMethod = "free_form"
)

// Failed reports whether a model classification ran and did not produce an
// admitted interpretation.
func (o ClassificationOutcome) Failed() bool {
	return o.Disposition != "" && o.Disposition != ClassificationAdmitted
}

// Retryable reports whether the same request may succeed unchanged later.
func (o ClassificationOutcome) Retryable() bool {
	switch o.Disposition {
	case ClassificationInvalidResponse:
		return true
	case ClassificationProviderFailed:
		return o.Failure.Retryable()
	default:
		return false
	}
}

// ClassificationError is a structural fault in a model's classification
// answer: the call returned, and what it returned is not a classification.
type ClassificationError struct {
	Stage ClassificationStage
	// Code is a safe diagnostic code, never model output.
	Code string
	// Cause is the decoder's error, when there is one. It names fields and
	// types, not values.
	Cause error
}

func (e *ClassificationError) Error() string {
	msg := fmt.Sprintf("classification response invalid (%s: %s)", e.Stage, e.Code)
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *ClassificationError) Unwrap() error { return e.Cause }

// FailedOutcome classifies the error a classification attempt ended in.
// Cancellation wins over everything else, as in ClassifyProviderFailure; a
// ClassificationError is an invalid response; anything else is the provider's.
func FailedOutcome(err error, method ClassificationMethod, attempts int) ClassificationOutcome {
	out := ClassificationOutcome{Method: method, Attempts: attempts}
	var structural *ClassificationError
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		out.Disposition = ClassificationCanceled
		out.Stage = StageTransport
		out.Failure = FailureCanceled
	case errors.As(err, &structural):
		out.Disposition = ClassificationInvalidResponse
		out.Stage = structural.Stage
		out.Code = structural.Code
	default:
		out.Disposition = ClassificationProviderFailed
		out.Stage = StageTransport
		out.Failure = ClassifyProviderFailure(err)
		out.Status = providerFailureStatus(err)
	}
	return out
}

// admittedOutcome is the outcome of a classification the model answered.
func admittedOutcome(method ClassificationMethod, attempts int) ClassificationOutcome {
	return ClassificationOutcome{Disposition: ClassificationAdmitted, Method: method, Attempts: attempts}
}

// SafeMessage describes a failed classification for the user or a log line
// without the provider's response body or the model's output.
func (o ClassificationOutcome) SafeMessage() string {
	switch o.Disposition {
	case ClassificationProviderFailed, ClassificationCanceled:
		return safeFailureMessage(o.Failure, o.Status)
	case ClassificationInvalidResponse:
		return fmt.Sprintf("the model's answer could not be read as a classification (%s: %s)", o.Stage, o.Code)
	default:
		return ""
	}
}

// outcomePredicate is the base predicate policy/perception_outcome.mg reads.
const outcomePredicate = "classification_outcome"

// Fact projects the outcome as classification_outcome(IntentID, Disposition,
// Stage, Class, Method). An outcome with no disposition projects nothing: a
// preset or blank input was not read by a model, and "not read" must not
// become a row the policy could mistake for an observation.
func (o ClassificationOutcome) Fact(intentID string) (types.Fact, bool) {
	if o.Disposition == "" {
		return types.Fact{}, false
	}
	atom := func(v string) types.MangleAtom {
		if v == "" {
			v = "none"
		}
		return types.MangleAtom("/" + strings.TrimPrefix(v, "/"))
	}
	return types.Fact{
		Predicate: outcomePredicate,
		Args: []any{
			types.MangleAtom(intentID),
			atom(string(o.Disposition)),
			atom(string(o.Stage)),
			atom(string(o.Failure)),
			atom(string(o.Method)),
		},
	}, true
}

// PublishClassificationOutcome replaces the intent's classification_outcome
// row with this outcome's. It touches only that intent's row: delegated tasks
// publish under task-scoped ids while the interactive turn's row stands.
func PublishClassificationOutcome(kernel types.Kernel, intentID string, outcome ClassificationOutcome) error {
	if kernel == nil {
		return nil
	}
	existing, err := kernel.Query(outcomePredicate)
	if err != nil {
		return fmt.Errorf("read %s: %w", outcomePredicate, err)
	}
	for _, f := range existing {
		if len(f.Args) > 0 && types.ExtractString(f.Args[0]) == intentID {
			if err := kernel.RetractFact(f); err != nil {
				return fmt.Errorf("retract %s: %w", outcomePredicate, err)
			}
		}
	}
	fact, ok := outcome.Fact(intentID)
	if !ok {
		return nil
	}
	if err := kernel.Assert(fact); err != nil {
		return fmt.Errorf("assert %s: %w", outcomePredicate, err)
	}
	return nil
}

// ClassificationStop is the kernel's verdict that a turn's classification
// failed and the turn ends with Notice (classification_stop,
// policy/perception_outcome.mg). It is an error so the callers that return
// errors (a session turn, a CLI command) carry the typed cause, not text.
type ClassificationStop struct {
	Outcome ClassificationOutcome
	// Action is the kernel's disposition: "/retry", "/blocker" or "/canceled".
	Action string
	// Notice is the user-facing wording the kernel derived.
	Notice string
}

func (e *ClassificationStop) Error() string {
	return fmt.Sprintf("classification %s (%s): %s", e.Outcome.Disposition, e.Action, e.Notice)
}

// ClassificationStopFor asks the kernel whether the intent's classification
// ends the turn. It returns nil when the outcome was admitted, was never
// published, or yields to held work's go-ahead.
//
// A kernel that cannot be asked cannot clear a failed classification: the
// failure stands with the safe message, because a failure nobody derived a
// disposition for must not continue as an interpretation.
func ClassificationStopFor(kernel types.Kernel, intentID string, intent Intent) (*ClassificationStop, error) {
	if !intent.Outcome.Failed() {
		return nil, nil
	}
	if kernel == nil {
		return &ClassificationStop{Outcome: intent.Outcome, Action: "/blocker", Notice: failedFallbackNotice(intent)}, nil
	}
	rows, err := kernel.Query("classification_stop")
	if err != nil {
		return &ClassificationStop{Outcome: intent.Outcome, Action: "/blocker", Notice: failedFallbackNotice(intent)},
			fmt.Errorf("ask classification_stop: %w", err)
	}
	for _, f := range rows {
		if len(f.Args) != 3 || types.ExtractString(f.Args[0]) != intentID {
			continue
		}
		return &ClassificationStop{
			Outcome: intent.Outcome,
			Action:  types.ExtractString(f.Args[1]),
			Notice:  types.ExtractString(f.Args[2]),
		}, nil
	}
	return nil, nil
}

func failedFallbackNotice(intent Intent) string {
	if msg := strings.TrimSpace(intent.Response); msg != "" {
		return msg
	}
	return "I could not read your request, so I have not acted on it."
}

// AdmitClassification publishes the intent's outcome under intentID and asks
// the kernel whether the turn may proceed: nil when it may, a
// *ClassificationStop when the classification failed. It is the one call a
// caller without its own routing needs (a session turn, a CLI command); the
// chat publishes through the firewall and asks after it has asserted the rest
// of the turn's routing facts.
func AdmitClassification(kernel types.Kernel, intentID string, intent Intent) error {
	if err := PublishClassificationOutcome(kernel, intentID, intent.Outcome); err != nil {
		if intent.Outcome.Failed() {
			return &ClassificationStop{Outcome: intent.Outcome, Action: "/blocker", Notice: failedFallbackNotice(intent)}
		}
		return err
	}
	stop, err := ClassificationStopFor(kernel, intentID, intent)
	if stop != nil {
		return stop
	}
	return err
}
