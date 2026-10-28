package approval

import (
	"fmt"
	"strings"

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

// AutoApprove records an approval of one exact request under the operator's
// standing yolo instruction and returns its grant. It needs no attached host:
// nobody is asked. The grant is the same as a human one from here on — Reserve
// revalidates the operation, and the kernel still derives permission — and
// the hard safety verdicts a human approval cannot lift are not lifted by
// this one either (dream_reason_liftable, dreamer.mg).
func (b *Broker) AutoApprove(intentID string, subject Subject, preview Preview, reason string) (*Grant, Request, error) {
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
		DecidedAt: now, Reason: reason,
	}
	if err := b.journal.append(r, now); err != nil {
		return nil, Request{}, err
	}
	b.requests[r.ID] = r
	b.eventSeen[eventID] = r.ID
	b.notifyLocked(r)
	return &Grant{broker: b, requestID: r.ID, intentID: intentID, digest: digest, eventID: eventID}, r.clone(), nil
}
