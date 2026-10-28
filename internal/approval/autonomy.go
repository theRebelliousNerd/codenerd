package approval

import (
	"fmt"

	"github.com/google/uuid"
)

// AutonomyChannel and AutonomyActor name a decision made by codeNERD itself
// under the operator's standing yolo instruction. They are never a person:
// the journal says who decided, and an audit can tell a yolo approval from a
// human one.
const (
	AutonomyChannel = "autonomy"
	AutonomyActor   = "codenerd (yolo)"
)

// SetAutonomous turns the operator's standing yolo instruction on or off.
// While it is on, Await approves every request itself, as AutonomyActor, and
// needs no attached host: nobody is asked. The chat sets it from the work mode
// (Shift+Tab, /yolo); a headless run from work_mode in config.
func (b *Broker) SetAutonomous(on bool) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.autonomous = on
}

// Autonomous reports whether the standing yolo instruction is on.
func (b *Broker) Autonomous() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.autonomous && !b.closed
}

// autoApprove records an approval of one exact request under the standing
// yolo instruction and returns its grant. The grant is the same as a human one
// from here on — Reserve revalidates the operation, and the kernel still
// derives permission — and the hard safety verdicts a human approval cannot
// lift are not lifted by this one either (dream_reason_liftable, dreamer.mg).
func (b *Broker) autoApprove(intentID string, subject Subject, digest string, preview Preview) (*Grant, Request, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, Request{}, fmt.Errorf("approval broker closed")
	}
	now := b.now()
	eventID := "yolo-" + uuid.NewString()
	r := &Request{
		ID:                      "apr-" + uuid.NewString(),
		Version:                 1,
		IntentID:                intentID,
		Subject:                 subject.canonical(),
		Digest:                  digest,
		CanonicalizationVersion: CanonicalizationVersion,
		Channel:                 AutonomyChannel,
		Preview:                 preview.clone(),
		CreatedAt:               now,
		ExpiresAt:               now.Add(b.opts.TTL),
		Status:                  StatusApproved,
		StatusReason:            string(VerdictApprove) + " by " + AutonomyActor,
	}
	r.Decision = &Decision{
		RequestID: r.ID, RequestVersion: r.Version, RequestDigest: r.Digest,
		Actor: AutonomyActor, Channel: AutonomyChannel, EventID: eventID, Verdict: VerdictApprove,
		DecidedAt: now, Reason: "standing yolo instruction (work_mode yolo)",
	}
	if err := b.journal.append(r, now); err != nil {
		return nil, Request{}, err
	}
	b.requests[r.ID] = r
	b.eventSeen[eventID] = r.ID
	b.notifyLocked(r)
	return &Grant{broker: b, requestID: r.ID, intentID: intentID, digest: digest, eventID: eventID}, r.clone(), nil
}
