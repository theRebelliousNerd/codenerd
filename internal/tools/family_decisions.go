package tools

import (
	"context"
	"sync"
)

// FamilyDecision is the model's recorded judgment that one member of a
// declared code family needs no change after a peer changed: the family, the
// member's element ref and the reason. The host turns it into
// turn_family_no_change (policy/code_families.mg); the reason is required so
// the decision is reviewable, not a way to wave an obligation away.
type FamilyDecision struct {
	Family string
	Member string
	Reason string
}

type familyDecisionLogKey struct{}

type familyDecisionLog struct {
	mu        sync.Mutex
	decisions []FamilyDecision
}

// WithFamilyDecisionLog returns a context in which RecordFamilyDecision
// records, and a function that returns what was recorded.
func WithFamilyDecisionLog(ctx context.Context) (context.Context, func() []FamilyDecision) {
	log := &familyDecisionLog{}
	return context.WithValue(ctx, familyDecisionLogKey{}, log), func() []FamilyDecision {
		log.mu.Lock()
		defer log.mu.Unlock()
		return append([]FamilyDecision(nil), log.decisions...)
	}
}

// RecordFamilyDecision records one decision. It reports false when the
// context carries no log (no turn is open to receive it).
func RecordFamilyDecision(ctx context.Context, d FamilyDecision) bool {
	log, ok := ctx.Value(familyDecisionLogKey{}).(*familyDecisionLog)
	if !ok {
		return false
	}
	log.mu.Lock()
	log.decisions = append(log.decisions, d)
	log.mu.Unlock()
	return true
}
