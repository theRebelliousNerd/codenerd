# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: SAFETY
# Sections: 11, 21, 22, 23

# =============================================================================
# SECTION 11: CONSTITUTIONAL LOGIC / SAFETY (§5.0)
# =============================================================================

# permitted(ActionType, Target, Payload) - derived predicate
# Priority: 100
# SerializationOrder: 6
Decl permitted(ActionType, Target, Payload).

# forbidden(ActionType) - derived: constitutionally blocked action
Decl forbidden(ActionType).

# dangerous_action(ActionType) - derived predicate
# Priority: 100
# SerializationOrder: 7
Decl dangerous_action(ActionType).

# blocked_pattern(Pattern) - dangerous string patterns
Decl blocked_pattern(Pattern).

# dangerous_content(ActionType, Payload) - derived predicate for content-based blocking
Decl dangerous_content(ActionType, Payload).

# admin_override(User)
Decl admin_override(User).

# signed_approval(ActionType)
Decl signed_approval(ActionType).

# allowed_domain(Domain) - network allowlist
Decl allowed_domain(Domain).

# network_permitted(URL) - derived predicate
Decl network_permitted(URL).

# security_violation_type(ViolationType) - derived: simple violation type flag
Decl security_violation_type(ViolationType).

# -----------------------------------------------------------------------------
# SECTION 11A: APPEAL MECHANISM (Constitutional Appeals)
# -----------------------------------------------------------------------------

# appeal_available(ActionID, ActionType, Target, Reason) - action can be appealed
Decl appeal_available(ActionID, ActionType, Target, Reason).

# appeal_pending(ActionID, ActionType, Justification, Timestamp) - appeal submitted
Decl appeal_pending(ActionID, ActionType, Justification, Timestamp).

# appeal_granted(ActionID, ActionType, Approver, Timestamp) - appeal approved
Decl appeal_granted(ActionID, ActionType, Approver, Timestamp).

# appeal_denied(ActionID, ActionType, Reason, Timestamp) - appeal rejected
Decl appeal_denied(ActionID, ActionType, Reason, Timestamp).

# temporary_override(ActionType, ExpirationTimestamp) - temporary permission with expiration
# NOTE: Changed from (ActionType, DurationSeconds, OverrideTime) to store computed expiration
# This avoids arithmetic in Mangle rules (+ operator not supported in rule bodies)
Decl temporary_override(ActionType, ExpirationTimestamp).

# has_temporary_override(ActionType) - helper predicate for safe negation
Decl has_temporary_override(ActionType).

# user_requests_appeal(ActionID, Justification, Requester) - user appeal request
Decl user_requests_appeal(ActionID, Justification, Requester).

# active_override(ActionType, Approver, ExpiresAt) - currently active override
Decl active_override(ActionType, Approver, ExpiresAt).

# appeal_history(ActionID, Granted, Approver, Timestamp) - appeal audit trail
Decl appeal_history(ActionID, Granted, Approver, Timestamp).

# suggest_appeal(ActionID) - derived: suggest user can appeal this action
Decl suggest_appeal(ActionID).

# appeal_needs_review(ActionID, ActionType, Justification) - pending appeal requires review
Decl appeal_needs_review(ActionID, ActionType, Justification).

# has_active_override(ActionType) - helper: true if override is active
Decl has_active_override(ActionType).

# appeal_denial_count(Count) - count of denied appeals
Decl appeal_denial_count(Count).

# appeal_grant_count(ActionType, Count) - count of granted appeals by action type
Decl appeal_grant_count(ActionType, Count).

# excessive_appeal_denials() - helper: true if too many denials (autopoiesis signal)
Decl excessive_appeal_denials().

# appeal_pattern_detected(ActionType) - pattern detected for learning
Decl appeal_pattern_detected(ActionType).

# =============================================================================
# SECTION 11B: STRATIFIED TRUST - AUTOPOIESIS (Bug #15 Fix)
# =============================================================================

# candidate_action(ActionType) - Learned logic proposals (from learned.gl)
Decl candidate_action(ActionType).

# final_action(ActionType) - Validated actions (Constitution approved)
Decl final_action(ActionType).

# safety_check(ActionType) - Runtime validation predicate
Decl safety_check(ActionType).

# action_denied(ActionType, Reason) - Blocked learned actions
Decl action_denied(ActionType, Reason).

# learned_proposal(ActionType) - Audit trail for learned suggestions
Decl learned_proposal(ActionType).

# blocked_learned_action_count(Count) - Metrics
Decl blocked_learned_action_count(Count).

# =============================================================================
# SECTION 21: GIT-AWARE SAFETY (Chesterton's Fence)
# =============================================================================

# git_history(FilePath, CommitHash, Author, AgeDays, Message)
Decl git_history(FilePath, CommitHash, Author, AgeDays, Message).

# git_state(Attribute, Value) - summarized git context for session injection
Decl git_state(Attribute, Value).

# churn_rate(FilePath, ChangeFrequency)
Decl churn_rate(FilePath, ChangeFrequency).

# current_user(UserName)
Decl current_user(UserName).

# current_time(Timestamp) - current system time for learning timestamps
Decl current_time(Timestamp).

# recent_change_by_other(FilePath) - derived predicate
# True if file was changed < 2 days ago by a different author
Decl recent_change_by_other(FilePath).

# chesterton_fence_warning(FilePath, Reason) - derived predicate
# Warns before deleting recently-changed code
Decl chesterton_fence_warning(FilePath, Reason).

# =============================================================================
# SECTION 22: SHADOW MODE / COUNTERFACTUAL REASONING
# =============================================================================

# hypothetical(Change) - counterfactual input
Decl hypothetical(Change).

# derives_from_hypothetical(Implication) - derived implications for a hypothetical
Decl derives_from_hypothetical(Implication).

# shadow_state(StateID, ActionID, IsValid)
Decl shadow_state(StateID, ActionID, IsValid).

# simulated_effect(ActionID, FactPredicate, FactArgs)
Decl simulated_effect(ActionID, FactPredicate, FactArgs).

# safe_projection(ActionID) - derived predicate
# True if the action passes safety checks in shadow simulation
Decl safe_projection(ActionID).

# projection_violation(ActionID, ViolationType) - derived predicate
Decl projection_violation(ActionID, ViolationType).

# =============================================================================
# SECTION 23: INTERACTIVE DIFF APPROVAL
# =============================================================================

# pending_mutation(MutationID, FilePath, OldContent, NewContent)
Decl pending_mutation(MutationID, FilePath, OldContent, NewContent).

# mutation_approved(MutationID, ApprovedBy, Timestamp)
Decl mutation_approved(MutationID, ApprovedBy, Timestamp).

# mutation_rejected(MutationID, RejectedBy, Reason)
Decl mutation_rejected(MutationID, RejectedBy, Reason).

# requires_approval(MutationID) - derived predicate
# True if the mutation requires user approval before execution
Decl requires_approval(MutationID).
