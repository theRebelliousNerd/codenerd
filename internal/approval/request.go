// Package approval is the host-owned human-approval lifecycle: a request that
// freezes exactly the operation a person reviews, a decision bound to that
// request's digest, and a one-shot execution claim that the executor takes
// before dispatching through its ordinary gates.
//
// Human consent satisfies one obligation and nothing else. It never bypasses
// the JIT allowlist, the constitution's content checks, project write
// protection or the Dreamer preflight: the executor reruns all of them after
// approval, and the kernel derives permitted/3 from the exact tuple the host
// asserts (human_approval_granted/5) only while that admission runs. See
// Docs/audits/request-bound-human-approval.md.
//
// Nothing here is reachable from a model. A decision is created only through a
// Host, which the interactive UI attaches at boot; a tool, a prompt, a file or
// MCP's confirm_risk flag cannot reach it.
package approval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// CanonicalizationVersion names the encoding the request digest is computed
// over. A request persisted under another version is never reused: replay
// marks it superseded rather than silently comparing hashes made by different
// rules.
const CanonicalizationVersion = "approval-request/v1"

// digestDomain separates request digests from every other sha256 in the
// process, so a digest of some other structure can never be presented as one.
const digestDomain = "codenerd.approval.request"

// Status is a request's lifecycle state. Transitions are atomic under the
// broker's lock and checked against allowedTransitions.
type Status string

const (
	StatusAwaitingHuman   Status = "awaiting_human"
	StatusApproved        Status = "approved"
	StatusRejected        Status = "rejected"
	StatusCanceled        Status = "canceled"
	StatusExpired         Status = "expired"
	StatusSuperseded      Status = "superseded"
	StatusReserved        Status = "reserved"
	StatusAdmissionFailed Status = "admission_failed"
	StatusDispatchStarted Status = "dispatch_started"
	StatusCompleted       Status = "completed"
	StatusOutcomeUnknown  Status = "outcome_unknown"
)

// allowedTransitions is the lifecycle in the doc's §4, with the execution
// claim folded into the request record: approved → reserved is the claim.
// Nothing leads back to approved, so a rejected, canceled, expired or
// superseded request cannot become eligible again by any later event.
var allowedTransitions = map[Status][]Status{
	StatusAwaitingHuman:   {StatusApproved, StatusRejected, StatusCanceled, StatusExpired, StatusSuperseded},
	StatusApproved:        {StatusReserved, StatusExpired, StatusSuperseded, StatusCanceled},
	StatusReserved:        {StatusDispatchStarted, StatusAdmissionFailed},
	StatusDispatchStarted: {StatusCompleted, StatusOutcomeUnknown},
}

// Terminal reports whether no further transition is possible.
func (s Status) Terminal() bool {
	_, open := allowedTransitions[s]
	return !open
}

func (s Status) valid() bool {
	switch s {
	case StatusAwaitingHuman, StatusApproved, StatusRejected, StatusCanceled, StatusExpired, StatusSuperseded,
		StatusReserved, StatusAdmissionFailed, StatusDispatchStarted, StatusCompleted, StatusOutcomeUnknown:
		return true
	}
	return false
}

func canTransition(from, to Status) bool {
	for _, next := range allowedTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// SourceRevision is one input the operation depends on, as the executor
// resolved it when the request was frozen. Digest is "sha256:<hex>" of the
// file's bytes, "absent" when nothing exists there, or "dir" for a directory.
type SourceRevision struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// Subject is the authority-bearing part of a request: every field the digest
// covers. Two subjects with different bytes in any field are different
// requests, and an approval of one never admits the other.
type Subject struct {
	Workspace string `json:"workspace"`
	Session   string `json:"session"`
	// Agent is the requesting executor's identity (persona or "session").
	Agent string `json:"agent"`
	// Backend is the dispatch path: "modular" for the tool registry, "mcp"
	// for a remote MCP call.
	Backend string `json:"backend"`
	Tool    string `json:"tool"`
	// ToolVersion identifies the registration the call resolves to: the
	// schema digest of a modular tool, the reviewed schema hash of an MCP one.
	ToolVersion string `json:"tool_version"`
	// Action, Target and Payload are exactly the pending_action/5 tuple the
	// constitution is asked about.
	Action  string `json:"action"`
	Target  string `json:"target"`
	Payload string `json:"payload"`
	// Effect is the tool's declared effect class.
	Effect string `json:"effect"`
	// Endpoint is the remote server identity for an MCP call (endpoint and
	// protocol); empty for a local tool.
	Endpoint        string           `json:"endpoint"`
	SourceRevisions []SourceRevision `json:"source_revisions"`
}

// Validate refuses representations the canonical encoding cannot carry
// faithfully. encoding/json replaces invalid UTF-8 with U+FFFD, so two
// different byte strings could otherwise encode, and hash, identically.
func (s Subject) Validate() error {
	fields := []struct{ name, value string }{
		{"workspace", s.Workspace}, {"session", s.Session}, {"agent", s.Agent},
		{"backend", s.Backend}, {"tool", s.Tool}, {"tool_version", s.ToolVersion},
		{"action", s.Action}, {"target", s.Target}, {"payload", s.Payload},
		{"effect", s.Effect}, {"endpoint", s.Endpoint},
	}
	for _, f := range fields {
		if !utf8.ValidString(f.value) {
			return fmt.Errorf("approval subject %s is not valid UTF-8", f.name)
		}
	}
	for _, required := range []struct{ name, value string }{
		{"backend", s.Backend}, {"tool", s.Tool}, {"action", s.Action},
	} {
		if strings.TrimSpace(required.value) == "" {
			return fmt.Errorf("approval subject %s is empty", required.name)
		}
	}
	seen := make(map[string]bool, len(s.SourceRevisions))
	for _, rev := range s.SourceRevisions {
		if !utf8.ValidString(rev.Path) || !utf8.ValidString(rev.Digest) {
			return fmt.Errorf("approval source revision is not valid UTF-8")
		}
		if rev.Path == "" || rev.Digest == "" {
			return fmt.Errorf("approval source revision has an empty path or digest")
		}
		if seen[rev.Path] {
			return fmt.Errorf("approval source revision %q appears twice", rev.Path)
		}
		seen[rev.Path] = true
	}
	return nil
}

// canonical is the subject with its one unordered field sorted. Array order is
// otherwise meaningful and kept as given.
func (s Subject) canonical() Subject {
	revs := append([]SourceRevision(nil), s.SourceRevisions...)
	sort.Slice(revs, func(i, j int) bool { return revs[i].Path < revs[j].Path })
	if revs == nil {
		revs = []SourceRevision{}
	}
	s.SourceRevisions = revs
	return s
}

// Digest is RequestDigest = H(domain || version || Encode(subject)). The
// encoding is encoding/json over a struct, so field order is the declaration
// order, strings are escaped deterministically, and there are no numbers or
// maps whose representation could vary.
func (s Subject) Digest() (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	encoded, err := json.Marshal(s.canonical())
	if err != nil {
		return "", fmt.Errorf("encode approval subject: %w", err)
	}
	h := sha256.New()
	h.Write([]byte(digestDomain))
	h.Write([]byte{0})
	h.Write([]byte(CanonicalizationVersion))
	h.Write([]byte{0})
	h.Write(encoded)
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// Verdict is the human's answer to one request.
type Verdict string

const (
	VerdictApprove Verdict = "approve"
	VerdictReject  Verdict = "reject"
	VerdictCancel  Verdict = "cancel"
)

func (v Verdict) status() (Status, bool) {
	switch v {
	case VerdictApprove:
		return StatusApproved, true
	case VerdictReject:
		return StatusRejected, true
	case VerdictCancel:
		return StatusCanceled, true
	}
	return "", false
}

// Decision is a committed human decision. It names the request by ID, version
// and digest, so a decision taken on one displayed request can never be
// applied to a revised one.
type Decision struct {
	RequestID      string    `json:"request_id"`
	RequestVersion int       `json:"request_version"`
	RequestDigest  string    `json:"request_digest"`
	Actor          string    `json:"actor"`
	Channel        string    `json:"channel"`
	EventID        string    `json:"event_id"`
	Verdict        Verdict   `json:"verdict"`
	DecidedAt      time.Time `json:"decided_at"`
	Reason         string    `json:"reason,omitempty"`
}

// Claim is the execution claim: at most one intent consumes an approval.
type Claim struct {
	IntentID        string    `json:"intent_id"`
	RequestDigest   string    `json:"request_digest"`
	DecisionEventID string    `json:"decision_event_id"`
	ReservedAt      time.Time `json:"reserved_at"`
	DispatchedAt    time.Time `json:"dispatched_at,omitempty"`
	FinishedAt      time.Time `json:"finished_at,omitempty"`
	Outcome         string    `json:"outcome,omitempty"`
}

// Request is one frozen operation awaiting, or past, a human decision.
// Values handed out by the broker are copies; nothing a holder does to one
// changes the broker's record.
type Request struct {
	ID       string  `json:"id"`
	Version  int     `json:"version"`
	IntentID string  `json:"intent_id"`
	Subject  Subject `json:"subject"`
	Digest   string  `json:"digest"`
	// CanonicalizationVersion is the encoding Digest was computed with.
	CanonicalizationVersion string `json:"canonicalization_version"`
	// Channel is the host channel the request was offered to; only a
	// decision from that channel is accepted.
	Channel      string    `json:"channel"`
	Preview      Preview   `json:"preview"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Status       Status    `json:"status"`
	StatusReason string    `json:"status_reason,omitempty"`
	Decision     *Decision `json:"decision,omitempty"`
	Claim        *Claim    `json:"claim,omitempty"`
}

func (r *Request) clone() Request {
	out := *r
	out.Subject.SourceRevisions = append([]SourceRevision(nil), r.Subject.SourceRevisions...)
	out.Preview = r.Preview.clone()
	if r.Decision != nil {
		d := *r.Decision
		out.Decision = &d
	}
	if r.Claim != nil {
		c := *r.Claim
		out.Claim = &c
	}
	return out
}

// Errors a caller can distinguish. Every one is a refusal: nothing is
// dispatched on any of them.
var (
	ErrNoHost           = errors.New("no human-approval host is attached")
	ErrHostAttached     = errors.New("a human-approval host is already attached")
	ErrUnknownRequest   = errors.New("unknown approval request")
	ErrRejected         = errors.New("the human rejected this request")
	ErrCanceled         = errors.New("the approval request was canceled")
	ErrExpired          = errors.New("the approval request expired before a decision")
	ErrStale            = errors.New("the operation changed after it was reviewed")
	ErrAlreadyClaimed   = errors.New("the approval was already claimed by an execution")
	ErrDecisionMismatch = errors.New("the decision does not name the displayed request")
	ErrNotDecidable     = errors.New("the request is no longer awaiting a decision")
	ErrWrongChannel     = errors.New("the decision arrived on a channel the request was not offered to")
)
