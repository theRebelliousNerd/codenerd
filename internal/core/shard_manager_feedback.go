package core

// =============================================================================
// REVIEWER FEEDBACK INTERFACE (Validation Triggers)
// =============================================================================
// These methods allow the main agent to interact with the reviewer feedback
// system for validating suspect reviews and learning from user feedback.

// ReviewerFeedbackProvider defines the interface for reviewer validation.
// This allows the main agent to check if reviews need validation without
// importing the reviewer package directly.
type ReviewerFeedbackProvider interface {
	NeedsValidation(reviewID string) bool
	GetSuspectReasons(reviewID string) []string
	AcceptFinding(reviewID, file string, line int)
	RejectFinding(reviewID, file string, line int, reason string)
	GetAccuracyReport(reviewID string) string
}

// reviewerFeedbackProvider holds the current reviewer instance if available.
var reviewerFeedbackProvider ReviewerFeedbackProvider

// SetReviewerFeedbackProvider registers a reviewer feedback provider.
// Called by registration.go when creating reviewer shards.
func SetReviewerFeedbackProvider(provider ReviewerFeedbackProvider) {
	reviewerFeedbackProvider = provider
}

// CheckReviewNeedsValidation queries whether a review is suspect.
// Returns true if the review has signs of inaccuracy and should be spot-checked.
func (sm *ShardManager) CheckReviewNeedsValidation(reviewID string) bool {
	if reviewerFeedbackProvider == nil {
		// Fallback: check kernel directly for reviewer_needs_validation
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("reviewer_needs_validation")
			if err == nil {
				for _, fact := range facts {
					if len(fact.Args) > 0 && fact.Args[0] == reviewID {
						return true
					}
				}
			}
		}
		return false
	}
	return reviewerFeedbackProvider.NeedsValidation(reviewID)
}

// GetReviewSuspectReasons returns reasons why a review is flagged as suspect.
func (sm *ShardManager) GetReviewSuspectReasons(reviewID string) []string {
	if reviewerFeedbackProvider == nil {
		// Fallback: check kernel directly
		if sm.kernel != nil {
			facts, err := sm.kernel.Query("review_suspect")
			if err == nil {
				var reasons []string
				for _, fact := range facts {
					if len(fact.Args) >= 2 && fact.Args[0] == reviewID {
						if reason, ok := fact.Args[1].(string); ok {
							reasons = append(reasons, reason)
						}
					}
				}
				return reasons
			}
		}
		return nil
	}
	return reviewerFeedbackProvider.GetSuspectReasons(reviewID)
}

// AcceptReviewFinding marks a finding as accepted by the user.
func (sm *ShardManager) AcceptReviewFinding(reviewID, file string, line int) {
	if reviewerFeedbackProvider != nil {
		reviewerFeedbackProvider.AcceptFinding(reviewID, file, line)
	}
}

// RejectReviewFinding marks a finding as rejected by the user.
// The reason helps the system learn from the rejection.
func (sm *ShardManager) RejectReviewFinding(reviewID, file string, line int, reason string) {
	if reviewerFeedbackProvider != nil {
		reviewerFeedbackProvider.RejectFinding(reviewID, file, line, reason)
	}
}

// GetReviewAccuracyReport returns accuracy statistics for a review session.
func (sm *ShardManager) GetReviewAccuracyReport(reviewID string) string {
	if reviewerFeedbackProvider == nil {
		return "Review feedback provider not available"
	}
	return reviewerFeedbackProvider.GetAccuracyReport(reviewID)
}
