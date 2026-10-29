// Package consultation is codeNERD's one specialist-consultation engine. Every
// production door (chat boot, `nerd campaign`, the system campaign runner)
// constructs a Manager over its own Spawner; campaign intelligence gathering
// and the advisory board consume it through campaign.ConsultationProvider.
//
// Three contracts live here:
//
//   - Reusable content is separate from request identity. An Artifact is what a
//     specialist computed; a Response is one consumer's receipt for it, with its
//     own request ID, requester, wait time and deep-copied fields. Two consumers
//     never share a slice.
//   - Identical concurrent work is coalesced without coupling cancellation. A
//     follower that gives up detaches; it does not cancel its siblings, and the
//     first caller's deadline does not own every follower's lifetime.
//   - Partial outcomes are lossless. A Batch has one Entry per requested
//     specialist, and a failed entry never erases a successful one.
//
// Completed advice is NOT cached across calls: no evidence-revision source
// exists to prove a stored answer still describes the workspace, and a TTL
// bounds retention, not freshness. Reuse is in-flight coalescing only.
package consultation

import (
	"context"
	"errors"
	"time"
)

// Spawner runs one consultation to completion and returns the specialist's
// reply text.
type Spawner interface {
	SpawnConsultation(ctx context.Context, specialistName, task string) (string, error)
}

// ErrNoSpawner is a configuration failure: no consultation can run, and
// retrying will not change that.
var ErrNoSpawner = errors.New("no consultation spawner configured")

// DefaultRequester names the system as the requester of a batch consultation.
const DefaultRequester = "system"

// Request is one consultation of one specialist.
type Request struct {
	RequestID  string // per-consumer identity; generated when empty, never part of the reuse key
	Requester  string // who asks; rendered into the prompt, so part of the reuse key
	Specialist string // who is asked
	Question   string
	Context    string
	// ResponseContract, when set, replaces the default ADVICE/CONFIDENCE
	// answer format. A caller with its own reply grammar (the advisory board's
	// versioned VOTE contract) supplies it here instead of embedding a second,
	// conflicting format in the question.
	ResponseContract string
}

// ParseStatus says whether the reply followed the default section format.
type ParseStatus string

const (
	ParseStructured   ParseStatus = "structured"   // an ADVICE: section was found
	ParseUnstructured ParseStatus = "unstructured" // the whole reply is the advice
)

// ReuseKind says how a consumer's response was produced.
type ReuseKind string

const (
	ReuseComputed  ReuseKind = "computed"  // this consumer started the specialist
	ReuseCoalesced ReuseKind = "coalesced" // joined an identical in-flight computation
)

// Artifact is the reusable content of one specialist computation. It is
// immutable once published; consumers receive copies through Response.
type Artifact struct {
	ContentKey      string
	Specialist      string
	Raw             string
	Advice          string
	Confidence      float64 // the model's self-reported claim, 0-1; not calibration
	References      []string
	Caveats         []string
	ParseStatus     ParseStatus
	ComputeDuration time.Duration
	CreatedAt       time.Time
}

// Response is one consumer's receipt for an Artifact.
type Response struct {
	RequestID   string
	Requester   string
	Specialist  string
	ContentKey  string
	Raw         string
	Advice      string
	Confidence  float64
	References  []string
	Caveats     []string
	ParseStatus ParseStatus
	Reuse       ReuseKind
	// ComputeDuration is the original computation's duration. A coalesced
	// consumer did not pay it again; WaitDuration is what it waited.
	ComputeDuration time.Duration
	WaitDuration    time.Duration
	DeliveredAt     time.Time
}

func (a *Artifact) receipt(req Request, reuse ReuseKind, wait time.Duration) *Response {
	return &Response{
		RequestID:       req.RequestID,
		Requester:       req.Requester,
		Specialist:      a.Specialist,
		ContentKey:      a.ContentKey,
		Raw:             a.Raw,
		Advice:          a.Advice,
		Confidence:      a.Confidence,
		References:      append([]string(nil), a.References...),
		Caveats:         append([]string(nil), a.Caveats...),
		ParseStatus:     a.ParseStatus,
		Reuse:           reuse,
		ComputeDuration: a.ComputeDuration,
		WaitDuration:    wait,
		DeliveredAt:     time.Now(),
	}
}

// BatchRequest consults several specialists on one question.
type BatchRequest struct {
	Topic            string
	Question         string
	Context          string
	Specialists      []string
	Requester        string // DefaultRequester when empty
	ResponseContract string
}

// EntryStatus is the terminal state of one specialist in a batch.
type EntryStatus string

const (
	EntrySucceeded EntryStatus = "succeeded"
	EntryFailed    EntryStatus = "failed"
	EntryCancelled EntryStatus = "cancelled"
)

// Entry is one requested specialist's outcome.
type Entry struct {
	Specialist string
	Status     EntryStatus
	Response   *Response // set only when Status is EntrySucceeded
	Err        error
	Retryable  bool
}

// Batch has one Entry per distinct requested specialist, in request order.
// RequestErr is set when the request itself was invalid; then Entries may be
// empty and the batch is not complete.
type Batch struct {
	Entries    []Entry
	RequestErr error
}

// Succeeded returns the successful responses in request order.
func (b Batch) Succeeded() []Response {
	out := make([]Response, 0, len(b.Entries))
	for _, e := range b.Entries {
		if e.Status == EntrySucceeded && e.Response != nil {
			out = append(out, *e.Response)
		}
	}
	return out
}

// Failed returns every entry that did not succeed.
func (b Batch) Failed() []Entry {
	var out []Entry
	for _, e := range b.Entries {
		if e.Status != EntrySucceeded {
			out = append(out, e)
		}
	}
	return out
}

// Complete reports whether every requested specialist answered.
func (b Batch) Complete() bool {
	return b.RequestErr == nil && len(b.Failed()) == 0
}

// Err joins the request error and every entry error; nil for a complete batch.
func (b Batch) Err() error {
	errs := []error{b.RequestErr}
	for _, e := range b.Entries {
		if e.Status != EntrySucceeded {
			errs = append(errs, e.Err)
		}
	}
	return errors.Join(errs...)
}
