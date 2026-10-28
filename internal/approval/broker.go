package approval

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Options configures a Broker. TTL, PreviewMaxBytes and JournalRetention come
// from the `approval` section of .nerd/config.json (config.ApprovalPolicy).
type Options struct {
	TTL              time.Duration
	PreviewMaxBytes  int
	JournalRetention time.Duration
	// JournalPath is where transitions are recorded. Empty keeps the
	// lifecycle in memory only (tests, and hosts with no workspace).
	JournalPath string
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Broker owns every approval request in the process. It is the only writer of
// request state; the executor holds Grants and the UI holds a Host, and both
// act only through the broker's checked transitions.
type Broker struct {
	mu        sync.Mutex
	opts      Options
	now       func() time.Time
	journal   *journal
	requests  map[string]*Request
	eventSeen map[string]string // decision event ID → request ID
	host      *Host
	// changed is closed and replaced on every transition, waking every
	// waiter to re-read the record it cares about.
	changed chan struct{}
	closed  bool
	// autonomous is the operator's standing yolo instruction (autonomy.go):
	// Await decides as AutonomyActor instead of asking a person.
	autonomous bool
}

// NewBroker builds a broker, replaying and reconciling its journal when one is
// configured.
func NewBroker(opts Options) (*Broker, error) {
	if opts.TTL <= 0 {
		return nil, fmt.Errorf("approval request TTL must be positive")
	}
	if opts.PreviewMaxBytes <= 0 {
		return nil, fmt.Errorf("approval preview limit must be positive")
	}
	if opts.JournalRetention <= 0 {
		return nil, fmt.Errorf("approval journal retention must be positive")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	b := &Broker{
		opts:      opts,
		now:       now,
		requests:  map[string]*Request{},
		eventSeen: map[string]string{},
		changed:   make(chan struct{}),
	}
	if opts.JournalPath != "" {
		j, records, err := openJournal(opts.JournalPath, now(), opts.JournalRetention)
		if err != nil {
			return nil, err
		}
		b.journal = j
		b.requests = records
		for _, r := range records {
			if r.Decision != nil {
				b.eventSeen[r.Decision.EventID] = r.ID
			}
		}
	}
	return b, nil
}

// PreviewMaxBytes is the bound a caller builds previews with.
func (b *Broker) PreviewMaxBytes() int { return b.opts.PreviewMaxBytes }

// Close releases the journal. Every waiter is woken and every open request is
// canceled, so no call stays blocked on a broker that is gone.
func (b *Broker) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	for _, r := range b.requests {
		if r.Status == StatusAwaitingHuman || r.Status == StatusApproved {
			b.transitionLocked(r, StatusCanceled, "approval broker closed")
		}
	}
	b.closed = true
	b.broadcastLocked()
	return b.journal.close()
}

// HostAttached reports whether a person can currently be asked. Headless runs
// attach no host, and an approvable denial then stays a refusal.
func (b *Broker) HostAttached() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.host != nil && !b.closed
}

// Snapshot returns a copy of every request, oldest first. It is the bounded
// status surface: it exposes no way to change anything.
func (b *Broker) Snapshot() []Request {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Request, 0, len(b.requests))
	for _, r := range sortedRequests(b.requests) {
		out = append(out, r.clone())
	}
	return out
}

// Get returns a copy of one request.
func (b *Broker) Get(id string) (Request, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.requests[id]
	if !ok {
		return Request{}, false
	}
	return r.clone(), true
}

// Await freezes subject and preview into a request offered to the attached
// host, and blocks until the human decides, the request expires, or ctx ends.
// No kernel fact, lock or lease is held while it waits. On approval it returns
// a Grant the caller must Reserve against the operation as it stands then.
func (b *Broker) Await(ctx context.Context, intentID string, subject Subject, preview Preview) (*Grant, Request, error) {
	return b.await(ctx, intentID, subject, preview, true)
}

// AwaitPerson is Await for a request only a person may decide: the standing
// yolo instruction does not approve it. It is for an obligation the user put
// on this very request ("confirm with me first"), which is more specific than
// the operator's general instruction to proceed without asking and so outranks
// it. With no host attached nobody can decide, and the request is refused as
// any unattended approvable denial is.
func (b *Broker) AwaitPerson(ctx context.Context, intentID string, subject Subject, preview Preview) (*Grant, Request, error) {
	return b.await(ctx, intentID, subject, preview, false)
}

func (b *Broker) await(ctx context.Context, intentID string, subject Subject, preview Preview, mayAutoApprove bool) (*Grant, Request, error) {
	if strings.TrimSpace(intentID) == "" {
		return nil, Request{}, fmt.Errorf("approval request needs an execution intent")
	}
	digest, err := subject.Digest()
	if err != nil {
		return nil, Request{}, err
	}
	if !preview.Verify() {
		return nil, Request{}, fmt.Errorf("approval preview is not sealed with its own digest")
	}
	if mayAutoApprove && b.Autonomous() {
		return b.autoApprove(intentID, subject, digest, preview)
	}
	id, err := b.open(intentID, subject, digest, preview)
	if err != nil {
		return nil, Request{}, err
	}
	for {
		b.mu.Lock()
		current := b.requests[id]
		b.expireLocked(current)
		status := current.Status
		snapshot := current.clone()
		wake := b.changed
		b.mu.Unlock()
		switch status {
		case StatusApproved:
			return &Grant{broker: b, requestID: id, intentID: intentID, digest: digest, eventID: snapshot.Decision.EventID}, snapshot, nil
		case StatusRejected:
			return nil, snapshot, fmt.Errorf("%w: %s", ErrRejected, decisionReason(snapshot))
		case StatusCanceled:
			return nil, snapshot, fmt.Errorf("%w: %s", ErrCanceled, snapshot.StatusReason)
		case StatusExpired:
			return nil, snapshot, ErrExpired
		case StatusSuperseded:
			return nil, snapshot, ErrStale
		case StatusAwaitingHuman:
		default:
			return nil, snapshot, fmt.Errorf("approval request %s is %s: another call with the same operation already claimed it, and it runs once", id, status)
		}
		timer := time.NewTimer(snapshot.ExpiresAt.Sub(b.now()))
		select {
		case <-wake:
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			b.mu.Lock()
			if cur := b.requests[id]; cur.Status == StatusAwaitingHuman || cur.Status == StatusApproved {
				b.transitionLocked(cur, StatusCanceled, "the requesting call ended before it could run: "+ctx.Err().Error())
			}
			snapshot = b.requests[id].clone()
			b.mu.Unlock()
			return nil, snapshot, fmt.Errorf("%w: %v", ErrCanceled, ctx.Err())
		}
		timer.Stop()
	}
}

func decisionReason(r Request) string {
	if r.Decision != nil && r.Decision.Reason != "" {
		return r.Decision.Reason
	}
	return "no reason given"
}

// open creates the request, or returns the live one an identical duplicate
// call (same intent, same digest) already opened.
func (b *Broker) open(intentID string, subject Subject, digest string, preview Preview) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return "", fmt.Errorf("approval broker closed")
	}
	if b.host == nil {
		return "", ErrNoHost
	}
	// The same operation under the same intent joins its request, decided or
	// not: a duplicate resume that arrives after the decision must neither ask
	// the person again nor dispatch twice (Await returns the decision; a
	// claimed or finished request runs for its first caller only). Only a
	// request that ended undecided (expired, canceled, superseded) is asked
	// again.
	for _, r := range b.requests {
		if r.IntentID == intentID && r.Digest == digest && !endedUndecided(r.Status) {
			return r.ID, nil
		}
	}
	// A different operation under the same intent replaces the one on
	// screen: the person must review the revised scope, not approve the old.
	for _, r := range b.requests {
		if r.IntentID == intentID && r.Status == StatusAwaitingHuman {
			b.transitionLocked(r, StatusSuperseded, "the operation was revised before a decision")
		}
	}
	now := b.now()
	r := &Request{
		ID:                      "apr-" + uuid.NewString(),
		Version:                 1,
		IntentID:                intentID,
		Subject:                 subject.canonical(),
		Digest:                  digest,
		CanonicalizationVersion: CanonicalizationVersion,
		Channel:                 b.host.channel,
		Preview:                 preview.clone(),
		CreatedAt:               now,
		ExpiresAt:               now.Add(b.opts.TTL),
		Status:                  StatusAwaitingHuman,
	}
	if err := b.journal.append(r, now); err != nil {
		return "", err
	}
	b.requests[r.ID] = r
	b.notifyLocked(r)
	return r.ID, nil
}

// expireLocked moves an open request past its deadline to expired.
func (b *Broker) expireLocked(r *Request) {
	if (r.Status == StatusAwaitingHuman || r.Status == StatusApproved) && !b.now().Before(r.ExpiresAt) {
		b.transitionLocked(r, StatusExpired, "no claim before the request's deadline")
	}
}

// transitionLocked applies one checked transition, journals it, and wakes
// every waiter. A journal failure leaves the record unchanged.
func (b *Broker) transitionLocked(r *Request, to Status, reason string) error {
	if !canTransition(r.Status, to) {
		return fmt.Errorf("approval request %s cannot move from %s to %s", r.ID, r.Status, to)
	}
	next := r.clone()
	next.Status = to
	next.StatusReason = reason
	if err := b.journal.append(&next, b.now()); err != nil {
		return err
	}
	*r = next
	b.notifyLocked(r)
	return nil
}

func (b *Broker) notifyLocked(r *Request) {
	b.broadcastLocked()
	if h := b.host; h != nil && h.channel == r.Channel {
		select {
		case h.updates <- r.clone():
		default:
			// The UI re-reads Pending on every update it does receive, so a
			// dropped notification loses no request.
		}
	}
}

func (b *Broker) broadcastLocked() {
	close(b.changed)
	b.changed = make(chan struct{})
}

// Host is the trusted decision channel: the interactive UI attaches one at
// boot. Actor and channel come from the host, never from a decision's caller,
// so a decision cannot claim to be from someone else.
type Host struct {
	broker  *Broker
	channel string
	actor   string
	updates chan Request
}

// AttachHost registers the one channel decisions are accepted from.
func (b *Broker) AttachHost(channel, actor string) (*Host, error) {
	if strings.TrimSpace(channel) == "" || strings.TrimSpace(actor) == "" {
		return nil, fmt.Errorf("approval host needs a channel and an actor")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, fmt.Errorf("approval broker closed")
	}
	if b.host != nil {
		return nil, ErrHostAttached
	}
	h := &Host{broker: b, channel: channel, actor: actor, updates: make(chan Request, 64)}
	b.host = h
	return h, nil
}

// Updates delivers a copy of every request offered to this host each time it
// changes.
func (h *Host) Updates() <-chan Request { return h.updates }

// Pending returns the requests awaiting this host's decision, oldest first.
func (h *Host) Pending() []Request {
	b := h.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []Request
	for _, r := range sortedRequests(b.requests) {
		b.expireLocked(r)
		if r.Channel == h.channel && r.Status == StatusAwaitingHuman {
			out = append(out, r.clone())
		}
	}
	return out
}

// Detach removes the host. Requests still awaiting it are canceled: nobody is
// left who could decide them.
func (h *Host) Detach() {
	b := h.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.host != h {
		return
	}
	for _, r := range b.requests {
		if r.Channel == h.channel && r.Status == StatusAwaitingHuman {
			b.transitionLocked(r, StatusCanceled, "the approval host detached")
		}
	}
	b.host = nil
	b.broadcastLocked()
}

// DecisionInput is what the UI submits: the request exactly as displayed, and
// a fresh event ID per user action.
type DecisionInput struct {
	RequestID      string
	RequestVersion int
	RequestDigest  string
	EventID        string
	Verdict        Verdict
	Reason         string
}

// Decide commits one human decision. A repeated event ID returns the decision
// it already committed (a double click is one decision). Everything else that
// does not name the displayed request exactly, arrives after the deadline, or
// targets a request no longer awaiting a decision is refused.
func (h *Host) Decide(in DecisionInput) (Request, error) {
	b := h.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.host != h {
		return Request{}, ErrWrongChannel
	}
	if strings.TrimSpace(in.EventID) == "" {
		return Request{}, fmt.Errorf("a decision needs an event ID")
	}
	to, ok := in.Verdict.status()
	if !ok {
		return Request{}, fmt.Errorf("unknown verdict %q", in.Verdict)
	}
	r, ok := b.requests[in.RequestID]
	if !ok {
		return Request{}, ErrUnknownRequest
	}
	if prior, seen := b.eventSeen[in.EventID]; seen {
		if prior == r.ID && r.Decision != nil && r.Decision.EventID == in.EventID {
			return r.clone(), nil
		}
		return Request{}, fmt.Errorf("%w: event %s already decided another request", ErrDecisionMismatch, in.EventID)
	}
	if r.Channel != h.channel {
		return Request{}, ErrWrongChannel
	}
	if in.RequestVersion != r.Version || in.RequestDigest != r.Digest {
		return Request{}, ErrDecisionMismatch
	}
	b.expireLocked(r)
	if r.Status == StatusExpired {
		return r.clone(), ErrExpired
	}
	if r.Status != StatusAwaitingHuman {
		return r.clone(), fmt.Errorf("%w: it is %s", ErrNotDecidable, r.Status)
	}
	next := r.clone()
	next.Status = to
	next.StatusReason = string(in.Verdict) + " by " + h.actor
	next.Decision = &Decision{
		RequestID: r.ID, RequestVersion: r.Version, RequestDigest: r.Digest,
		Actor: h.actor, Channel: h.channel, EventID: in.EventID, Verdict: in.Verdict,
		DecidedAt: b.now(), Reason: in.Reason,
	}
	// Commit before announcing: the journal holds the decision before any
	// waiter can act on it.
	if err := b.journal.append(&next, b.now()); err != nil {
		return Request{}, err
	}
	*r = next
	b.eventSeen[in.EventID] = r.ID
	b.notifyLocked(r)
	return r.clone(), nil
}

// Grant is an approval in the hands of the call that requested it. It carries
// no authority by itself: Reserve has to succeed against the operation as it
// stands now, and the kernel still has to derive permitted/3.
type Grant struct {
	broker    *Broker
	requestID string
	intentID  string
	digest    string
	eventID   string
}

// RequestID is the approved request.
func (g *Grant) RequestID() string { return g.requestID }

// Digest is the reviewed request's digest, which the host asserts into the
// kernel beside the exact pending_action it admits.
func (g *Grant) Digest() string { return g.digest }

// Reserve claims the approval for intentID against the operation as it is
// now. A different digest — arguments, target, tool registration, endpoint or
// any source revision changed since the person looked — supersedes the
// request. A second Reserve of the same approval fails: at most one execution
// consumes it.
func (g *Grant) Reserve(intentID string, current Subject) error {
	b := g.broker
	digest, digestErr := current.Digest()
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.requests[g.requestID]
	if !ok {
		return ErrUnknownRequest
	}
	b.expireLocked(r)
	switch r.Status {
	case StatusApproved:
	case StatusExpired:
		return ErrExpired
	case StatusSuperseded:
		return ErrStale
	case StatusCanceled:
		return ErrCanceled
	default:
		return fmt.Errorf("%w (%s)", ErrAlreadyClaimed, r.Status)
	}
	if intentID != r.IntentID {
		return fmt.Errorf("%w: the approval belongs to intent %s", ErrAlreadyClaimed, r.IntentID)
	}
	if digestErr != nil || digest != r.Digest || digest != g.digest {
		if err := b.transitionLocked(r, StatusSuperseded, "the operation changed between review and execution"); err != nil {
			return err
		}
		return ErrStale
	}
	next := r.clone()
	next.Status = StatusReserved
	next.StatusReason = "claimed by the requesting call"
	next.Claim = &Claim{IntentID: intentID, RequestDigest: r.Digest, DecisionEventID: g.eventID, ReservedAt: b.now()}
	if err := b.journal.append(&next, b.now()); err != nil {
		return err
	}
	*r = next
	b.notifyLocked(r)
	return nil
}

// AdmissionFailed records that a reserved claim was refused before dispatch
// (a gate rerun after approval denied it). Nothing ran; the approval is spent.
func (g *Grant) AdmissionFailed(reason string) {
	g.finish(StatusReserved, StatusAdmissionFailed, reason, "not_started")
}

// Abandon cancels an approval its call will not claim (the call was refused
// before it could reserve it), so it does not linger approved until expiry.
func (g *Grant) Abandon(reason string) {
	g.finish(StatusApproved, StatusCanceled, reason, "")
}

// DispatchStarted durably records that the backend is about to run. If the
// record cannot be written the caller must not dispatch.
func (g *Grant) DispatchStarted() error {
	b := g.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.requests[g.requestID]
	if !ok {
		return ErrUnknownRequest
	}
	if r.Status != StatusReserved {
		return fmt.Errorf("approval request %s is %s, not reserved", r.ID, r.Status)
	}
	next := r.clone()
	next.Status = StatusDispatchStarted
	next.StatusReason = "dispatched through the ordinary backend"
	next.Claim.DispatchedAt = b.now()
	if err := b.journal.append(&next, b.now()); err != nil {
		return err
	}
	*r = next
	b.notifyLocked(r)
	return nil
}

// Finish records the dispatch's outcome. An error is an unknown outcome, not
// "no effect": a failed tool or validator may still have changed the world,
// and the approval is never a reason to run it again.
func (g *Grant) Finish(err error) {
	if err == nil {
		g.finish(StatusDispatchStarted, StatusCompleted, "the approved call completed", "completed")
		return
	}
	g.finish(StatusDispatchStarted, StatusOutcomeUnknown, "the approved call returned an error after dispatch: "+err.Error(), "unknown")
}

func (g *Grant) finish(from, to Status, reason, outcome string) {
	b := g.broker
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.requests[g.requestID]
	if !ok || r.Status != from {
		return
	}
	next := r.clone()
	next.Status = to
	next.StatusReason = reason
	if next.Claim != nil {
		next.Claim.FinishedAt = b.now()
		next.Claim.Outcome = outcome
	}
	if err := b.journal.append(&next, b.now()); err != nil {
		// The outcome could not be made durable; the in-memory record still
		// moves so this process never treats the claim as reusable.
		next.StatusReason += " (journal write failed: " + err.Error() + ")"
	}
	*r = next
	b.notifyLocked(r)
}

// shared is the process's brokers by journal path. Several Cortex instances
// can be live for one workspace (one per provider); two brokers appending to
// and compacting one journal would lose transitions, so they share one.
var shared = struct {
	sync.Mutex
	byPath map[string]*sharedBroker
}{byPath: map[string]*sharedBroker{}}

type sharedBroker struct {
	broker *Broker
	refs   int
}

// OpenShared returns the process's broker for opts.JournalPath, opening it on
// first use. The release func drops this holder's reference; the last one
// closes the broker.
func OpenShared(opts Options) (*Broker, func() error, error) {
	if opts.JournalPath == "" {
		return nil, nil, fmt.Errorf("a shared approval broker needs a journal path")
	}
	shared.Lock()
	defer shared.Unlock()
	entry := shared.byPath[opts.JournalPath]
	if entry == nil {
		b, err := NewBroker(opts)
		if err != nil {
			return nil, nil, err
		}
		entry = &sharedBroker{broker: b}
		shared.byPath[opts.JournalPath] = entry
	}
	entry.refs++
	var once sync.Once
	release := func() error {
		var err error
		once.Do(func() {
			shared.Lock()
			defer shared.Unlock()
			entry.refs--
			if entry.refs == 0 {
				delete(shared.byPath, opts.JournalPath)
				err = entry.broker.Close()
			}
		})
		return err
	}
	return entry.broker, release, nil
}

// endedUndecided reports a request that closed without a person's decision
// being acted on, so the same operation may be asked again.
func endedUndecided(s Status) bool {
	return s == StatusExpired || s == StatusCanceled || s == StatusSuperseded
}
