package broker

import (
	"errors"
	"fmt"
	"time"

	"codenerd/internal/types"
)

// Confidence records where a token count came from. It is carried on every
// Count and every Receipt so that downstream code can tell a measurement from
// a guess — the distinction the pre-broker codebase could not make.
type Confidence string

const (
	// ConfidenceExact means a provider API produced this number for this exact
	// content and model. Anthropic's count_tokens endpoint, or a provider usage
	// report on a completed response.
	ConfidenceExact Confidence = "exact"

	// ConfidenceCalibrated means an estimator produced this number using a
	// chars-per-token ratio that has been corrected against at least one
	// observed provider actual for this model.
	ConfidenceCalibrated Confidence = "calibrated"

	// ConfidenceSeeded means an estimator produced this number from the seed
	// ratio with no observations yet for this model. This is the weakest
	// regime and the only one that resembles the old heuristic — but unlike the
	// old heuristic it is labelled, and it stops being true after the first
	// response comes back.
	ConfidenceSeeded Confidence = "seeded"
)

// Rank orders confidence from weakest to strongest so callers can compare.
func (c Confidence) Rank() int {
	switch c {
	case ConfidenceExact:
		return 3
	case ConfidenceCalibrated:
		return 2
	case ConfidenceSeeded:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether c is at least as strong as min.
func (c Confidence) AtLeast(min Confidence) bool { return c.Rank() >= min.Rank() }

// Segments attributes a request's input tokens to the part of the request that
// produced them. The total is authoritative; the split is proportional when the
// underlying counter reports only a total (which the Anthropic endpoint does),
// so treat segments as attribution for observability rather than as separately
// measured quantities.
type Segments struct {
	System  int `json:"system"`
	History int `json:"history"`
	User    int `json:"user"`
	Tools   int `json:"tools"`
}

// Total returns the sum of all segments.
func (s Segments) Total() int { return s.System + s.History + s.User + s.Tools }

// Count is a token count with provenance.
type Count struct {
	Tokens     int        `json:"tokens"`
	Segments   Segments   `json:"segments"`
	Confidence Confidence `json:"confidence"`
	// Source names the mechanism, e.g. "anthropic.count_tokens" or
	// "estimator:calibrated(3.71)". It is for humans reading receipts.
	Source string `json:"source"`
	Model  string `json:"model"`
}

// Request is the assembled outbound request, as it will actually be sent.
//
// The broker counts this rather than trusting any upstream subsystem's own
// arithmetic, because content is routinely appended after a subsystem counts:
// the chat loop concatenates a persona onto the kernel's final_system_prompt,
// the prompt assembler expands templates after budget fitting, and tool schemas
// are serialized last. Counting here is the only place the number matches the
// wire.
type Request struct {
	Provider string
	Model    string
	// Method names the client method being invoked, for receipts.
	Method string

	System   string
	User     string
	Messages []types.Message
	Tools    []types.ToolDefinition
}

// Purpose identifies the subsystem spending the tokens. It travels on the
// context so that a call made deep inside a tool loop is still attributed to
// the session that started it.
type Purpose string

const (
	PurposePerception   Purpose = "perception"
	PurposeArticulation Purpose = "articulation"
	PurposeSession      Purpose = "session"
	PurposeCompression  Purpose = "compression"
	PurposeCampaign     Purpose = "campaign"
	PurposeCritic       Purpose = "critic"
	PurposeSubagent     Purpose = "subagent"
	PurposeAutopoiesis  Purpose = "autopoiesis"
	PurposeVerification Purpose = "verification"

	// PurposeUnattributed is the fallback when no purpose was set on the
	// context. It is deliberately a real account rather than a silent discard:
	// unattributed spend is a wiring bug, and it should be visible as a number
	// rather than vanish.
	PurposeUnattributed Purpose = "unattributed"
)

// Spend is a set of token counters. Values come from provider usage reports and
// are therefore exact.
type Spend struct {
	InputTokens    int64 `json:"input_tokens"`
	OutputTokens   int64 `json:"output_tokens"`
	CachedTokens   int64 `json:"cached_tokens,omitempty"`
	ThinkingTokens int64 `json:"thinking_tokens,omitempty"`
	Calls          int64 `json:"calls"`
}

// Add accumulates other into s.
func (s *Spend) Add(other Spend) {
	s.InputTokens += other.InputTokens
	s.OutputTokens += other.OutputTokens
	s.CachedTokens += other.CachedTokens
	s.ThinkingTokens += other.ThinkingTokens
	s.Calls += other.Calls
}

// Total returns input plus output. Cached tokens are already counted inside
// input by every provider this codebase talks to, so they are not added again.
func (s Spend) Total() int64 { return s.InputTokens + s.OutputTokens }

// DecisionCode is the machine-readable outcome of an admission check.
type DecisionCode string

const (
	DecisionAdmitted DecisionCode = "admitted"
	// DecisionWindowExceeded means the counted request plus the output reserve
	// does not fit the model's context window. This is a hard provider limit;
	// sending anyway produces an API error and bills for the attempt.
	DecisionWindowExceeded DecisionCode = "window_exceeded"
	// DecisionBudgetExhausted means a configured cumulative spend cap for this
	// purpose is used up.
	DecisionBudgetExhausted DecisionCode = "budget_exhausted"
	// DecisionCountUnavailable means the counter could not produce a number.
	// The broker fails closed here: a budget that cannot be checked is a budget
	// that is not enforced.
	DecisionCountUnavailable DecisionCode = "count_unavailable"
)

// Decision is the result of an admission check.
type Decision struct {
	Allowed  bool         `json:"allowed"`
	Code     DecisionCode `json:"code"`
	Reason   string       `json:"reason,omitempty"`
	Count    Count        `json:"count"`
	Window   int          `json:"window"`
	Headroom int          `json:"headroom"`
}

// Receipt is the record of one inference call. It is produced by the same pass
// that assembles, admits, and dispatches the request, so it cannot drift from
// what was actually sent — an audit artifact generated separately from the
// thing it audits always drifts eventually.
type Receipt struct {
	Purpose  Purpose       `json:"purpose"`
	Provider string        `json:"provider"`
	Model    string        `json:"model"`
	Method   string        `json:"method"`
	Started  time.Time     `json:"started"`
	Duration time.Duration `json:"duration"`

	// Scope is the session this call belonged to, or "" when untagged. Epoch
	// segmentation groups by it so two concurrent sessions are not spliced into
	// one alternating run that reports every call as a singleton.
	Scope string `json:"scope,omitempty"`
	// Prefix fingerprints the cacheable head of the request -- the tool
	// definitions and the system prompt, in wire order. Two consecutive calls
	// with the same Prefix could have shared a provider cache entry; a change
	// means the entry was invalidated. Empty when the request had no cacheable
	// head at all.
	Prefix string `json:"prefix,omitempty"`

	// Estimated is what admission believed before the request went out.
	Estimated Count `json:"estimated"`
	// Actual is what the provider reported. Zero when the provider reported
	// nothing (some CLI-backed engines do not).
	Actual Spend `json:"actual"`
	// EstimateErrorPct is (estimated-actual)/actual as a percentage of the
	// actual input tokens, or 0 when no actual was reported. This is the number
	// that tells you whether the meter can be trusted.
	EstimateErrorPct float64 `json:"estimate_error_pct,omitempty"`

	Decision Decision `json:"decision"`
	Err      string   `json:"error,omitempty"`
}

// AdmissionError is returned when the broker refuses a request.
type AdmissionError struct {
	Decision Decision
	Purpose  Purpose
}

func (e *AdmissionError) Error() string {
	return fmt.Sprintf("broker refused %s request for %s: %s (%s): counted %d tokens, window %d, headroom %d",
		e.Decision.Code, e.Purpose, e.Decision.Reason, e.Decision.Count.Confidence,
		e.Decision.Count.Tokens, e.Decision.Window, e.Decision.Headroom)
}

// Unwrap lets errors.As find an AdmissionError through a wrapping chain.
func (e *AdmissionError) Unwrap() error { return nil }

// IsAdmissionError reports whether err is, or wraps, a broker refusal.
//
// errors.As rather than a type assertion: a refusal raised inside perception
// reaches the caller as "observation failed: %w", and a bare assertion answers
// false for every path a refusal actually travels. The function existed to let
// a caller present a refusal as the specific thing it is, and it could not have
// worked on any real error.
func IsAdmissionError(err error) (*AdmissionError, bool) {
	var ae *AdmissionError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
