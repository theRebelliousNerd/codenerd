# Post-Action Validation Policy Rules
# Version: 1.0.0
# Philosophy: Trust but verify - every action must prove it succeeded.
#
# These rules derive self-healing strategies and action blocking
# based on validation outcomes.

# =============================================================================
# SECTION 1: VALIDATION SUCCESS DERIVATION
# =============================================================================

# An action is validated if verification succeeded with sufficient confidence
action_validated(ActionID) :-
    action_verified(ActionID, _, _, Confidence, _),
    Confidence >= 80.

# Paranoid validation (maximum confidence, zero false positives)
action_paranoid_validated(ActionID) :-
    action_verified(ActionID, _, /paranoid_validation, Confidence, _),
    Confidence = 100.

# Enhanced edit validation (surgical verification)
action_enhanced_validated(ActionID) :-
    action_verified(ActionID, _, /enhanced_edit_validation, Confidence, _),
    Confidence = 100.

# Critical actions REQUIRE paranoid validation (file writes, edits)
requires_paranoid_validation(/write_file).
requires_paranoid_validation(/fs_write).
requires_paranoid_validation(/edit_file).

# Critical action is only validated if paranoid validator passed
critical_action_validated(ActionID) :-
    action_verified(ActionID, ActionType, /paranoid_validation, 100, _),
    requires_paranoid_validation(ActionType).

# Weak validation (lower confidence, might need confirmation)
action_weakly_validated(ActionID) :-
    action_verified(ActionID, _, _, Confidence, _),
    Confidence >= 50,
    Confidence < 80.

# =============================================================================
# SECTION 2: VALIDATION FAILURE DERIVATION
# =============================================================================

# An action failed validation if any validator reported failure
action_failed_validation(ActionID) :-
    action_validation_failed(ActionID, _, _, _, _).

# Hash mismatch indicates content wasn't written correctly
validation_hash_mismatch(ActionID) :-
    action_validation_failed(ActionID, _, "content hash mismatch", _, _).

# Syntax error indicates code corruption
validation_syntax_error(ActionID) :-
    action_validation_failed(ActionID, _, Reason, _, _),
    Reason = "syntax validation failed".

# Element disappeared indicates structural damage
validation_element_lost(ActionID) :-
    action_validation_failed(ActionID, _, Reason, _, _),
    Reason = "target element no longer exists after edit".

# =============================================================================
# SECTION 3: SELF-HEALING STRATEGY SELECTION
# =============================================================================

# Retry strategy for transient failures (hash mismatch, file access)
needs_self_healing(ActionID, /retry) :-
    validation_hash_mismatch(ActionID),
    !validation_max_retries_reached(ActionID).

needs_self_healing(ActionID, /retry) :-
    action_validation_failed(ActionID, _, "cannot read back file", _, _),
    !validation_max_retries_reached(ActionID).

# Rollback strategy for syntax errors (code corruption)
needs_self_healing(ActionID, /rollback) :-
    validation_syntax_error(ActionID).

# Rollback for element loss (structural damage)
needs_self_healing(ActionID, /rollback) :-
    validation_element_lost(ActionID).

# Escalate when retries exhausted
needs_self_healing(ActionID, /escalate) :-
    validation_max_retries_reached(ActionID).

# Escalate for unknown failure types
needs_self_healing(ActionID, /escalate) :-
    action_failed_validation(ActionID),
    !validation_hash_mismatch(ActionID),
    !validation_syntax_error(ActionID),
    !validation_element_lost(ActionID).

# =============================================================================
# SECTION 4: ACTION BLOCKING
# =============================================================================

# Block subsequent actions while validation failure is unresolved
block_action(/validation_pending) :-
    action_failed_validation(ActionID),
    !action_validated(ActionID),
    !needs_self_healing(ActionID, _).

# Block actions if previous action awaiting self-healing
block_action(/awaiting_healing) :-
    needs_self_healing(ActionID, HealingType),
    /escalate != HealingType.

# =============================================================================
# SECTION 5: VALIDATION METRICS
# =============================================================================

# Count validated actions
validation_success_count(N) :-
    action_validated(_) |>
    let N = fn:count().

# Count failed validations
validation_failure_count(N) :-
    action_failed_validation(_) |>
    let N = fn:count().

# Validation by method
validation_by_method(Method, N) :-
    validation_method_used(_, Method) |>
    do fn:group_by(Method),
    let N = fn:count().

# =============================================================================
# SECTION 6: CONFIDENCE THRESHOLDS
# =============================================================================

# Define confidence thresholds for different validation methods
# These can be queried to determine acceptable confidence levels

validation_threshold(/hash, 95).
validation_threshold(/syntax, 90).
validation_threshold(/existence, 70).
validation_threshold(/content_check, 85).
validation_threshold(/output_scan, 75).
validation_threshold(/codedom_refresh, 90).
validation_threshold(/skipped, 0).

# Paranoid and enhanced validations require perfect confidence
validation_threshold(/paranoid_validation, 100).
validation_threshold(/enhanced_edit_validation, 100).

# Check if validation meets threshold for its method
validation_meets_threshold(ActionID) :-
    action_verified(ActionID, _, Method, Confidence, _),
    validation_threshold(Method, Threshold),
    Confidence >= Threshold.

# =============================================================================
# SECTION 7: HEALING OUTCOMES
# =============================================================================

# An action has been healed if there's a successful healing attempt
action_healed(ActionID) :-
    healing_attempt(ActionID, _, /true, _, _).

# Count healing attempts by type
healing_by_type(HealingType, N) :-
    healing_attempt(_, HealingType, _, _, _) |>
    do fn:group_by(HealingType),
    let N = fn:count().

# An action requires user intervention if escalated
requires_user_intervention(ActionID) :-
    action_escalated(ActionID, _, _).

# An action is fully resolved if either validated or healed
action_resolved(ActionID) :-
    action_validated(ActionID).

action_resolved(ActionID) :-
    action_healed(ActionID).

# Critical actions are only resolved if paranoid validation passed
critical_action_resolved(ActionID) :-
    critical_action_validated(ActionID).

# Unresolved failures for monitoring
unresolved_failure(ActionID) :-
    action_failed_validation(ActionID),
    !action_resolved(ActionID),
    !action_escalated(ActionID, _, _).
