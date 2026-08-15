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
Decl permitted(ActionType, Target, Payload) bound [/name, /string, /string].

# forbidden(ActionType) - derived: constitutionally blocked action
Decl forbidden(ActionType) bound [/name].

# dangerous_action(ActionType) - derived predicate
# Priority: 100
# SerializationOrder: 7
Decl dangerous_action(ActionType) bound [/name].

# blocked_pattern(Pattern) - dangerous string patterns
Decl blocked_pattern(Pattern) bound [/string].

# string_contains(Haystack, Needle) - virtual predicate for dynamic substring checks
# External predicate: both args are required inputs
# Decl string_contains(Haystack, Needle) descr [external(), mode('+', '+')] bound [/string, /string].

# dangerous_content(ActionType, Payload) - derived predicate for content-based blocking
Decl dangerous_content(ActionType, Payload) bound [/name, /string].

# admin_override(User)
Decl admin_override(User) bound [/string].

# signed_approval(ActionType)
Decl signed_approval(ActionType) bound [/name].

# allowed_domain(Domain) - network allowlist
Decl allowed_domain(Domain) bound [/string].

# network_permitted(URL) - derived predicate
Decl network_permitted(URL) bound [/string].

# security_violation_type(ViolationType) - derived: simple violation type flag
Decl security_violation_type(ViolationType) bound [/name].

# requires_permission(ActionType) - derived: action type requires explicit permission
Decl requires_permission(ActionType) bound [/name].

# -----------------------------------------------------------------------------
# SECTION 11A: APPEAL MECHANISM (Constitutional Appeals)
# -----------------------------------------------------------------------------

# appeal_available(ActionID, ActionType, Target, Reason) - action can be appealed
Decl appeal_available(ActionID, ActionType, Target, Reason) bound [/string, /name, /string, /name].

# appeal_pending(ActionID, ActionType, Justification, Timestamp) - appeal submitted
Decl appeal_pending(ActionID, ActionType, Justification, Timestamp) bound [/string, /name, /string, /number].

# appeal_granted(ActionID, ActionType, Approver, Timestamp) - appeal approved
Decl appeal_granted(ActionID, ActionType, Approver, Timestamp) bound [/string, /name, /string, /number].

# appeal_denied(ActionID, ActionType, Reason, Timestamp) - appeal rejected
Decl appeal_denied(ActionID, ActionType, Reason, Timestamp) bound [/string, /name, /name, /number].

# temporary_override(ActionType, ExpirationTimestamp) - temporary permission with expiration
# NOTE: Changed from (ActionType, DurationSeconds, OverrideTime) to store computed expiration
# This avoids arithmetic in Mangle rules (+ operator not supported in rule bodies)
Decl temporary_override(ActionType, ExpirationTimestamp) bound [/name, /number].

# has_temporary_override(ActionType) - helper predicate for safe negation
Decl has_temporary_override(ActionType) bound [/name].

# user_requests_appeal(ActionID, Justification, Requester) - user appeal request
Decl user_requests_appeal(ActionID, Justification, Requester) bound [/string, /string, /string].

# active_override(ActionType, Approver, ExpiresAt) - currently active override
Decl active_override(ActionType, Approver, ExpiresAt) bound [/name, /string, /number].

# appeal_history(ActionID, Granted, Approver, Timestamp) - appeal audit trail
Decl appeal_history(ActionID, Granted, Approver, Timestamp) bound [/string, /name, /string, /number].

# suggest_appeal(ActionID) - derived: suggest user can appeal this action
Decl suggest_appeal(ActionID) bound [/string].

# appeal_needs_review(ActionID, ActionType, Justification) - pending appeal requires review
Decl appeal_needs_review(ActionID, ActionType, Justification) bound [/string, /name, /string].

# has_active_override(ActionType) - helper: true if override is active
Decl has_active_override(ActionType) bound [/name].

# appeal_denial_count(Count) - count of denied appeals
Decl appeal_denial_count(Count) bound [/number].

# appeal_grant_count(ActionType, Count) - count of granted appeals by action type
Decl appeal_grant_count(ActionType, Count) bound [/name, /number].

# excessive_appeal_denials() - helper: true if too many denials (autopoiesis signal)
Decl excessive_appeal_denials() bound [].

# appeal_pattern_detected(ActionType) - pattern detected for learning
Decl appeal_pattern_detected(ActionType) bound [/name].

# =============================================================================
# SECTION 11B: STRATIFIED TRUST - AUTOPOIESIS (Bug #15 Fix)
# =============================================================================

# candidate_action(ActionType) - Learned logic proposals (from learned.gl)
Decl candidate_action(ActionType) bound [/name].

# final_action(ActionType) - Validated actions (Constitution approved)
Decl final_action(ActionType) bound [/name].

# safety_check(ActionType) - Runtime validation predicate
Decl safety_check(ActionType) bound [/name].

# action_denied(ActionType, Reason) - Blocked learned actions
Decl action_denied(ActionType, Reason) bound [/name, /name].

# learned_proposal(ActionType) - Audit trail for learned suggestions
Decl learned_proposal(ActionType) bound [/name].

# blocked_learned_action_count(Count) - Metrics
Decl blocked_learned_action_count(Count) bound [/number].

# =============================================================================
# SECTION 11C: BOUND NEGATION HELPERS
# =============================================================================
#
# A negated literal containing ANY anonymous wildcard is a no-op in this Mangle
# build: the wildcard leaves the literal unbound rather than existentially
# quantified, so the negation excludes nothing at all. Verified on a booted
# kernel — `!p(X, _)`, `!p(X, _, _)`, `!p(_)` and `!p(_, _)` all derive every
# row, while `!p(X)` and `!helper(X)` exclude correctly.
#
# Several .mg files already carried comments warning about this (shards.mg,
# validation.mg, codedom_safety.mg, prompt_northstar.mg), but the workaround
# was never applied to the constitution, so three safety rules silently did
# nothing: admin_override could not suppress a denial, every candidate action
# was denied whether or not it was permitted, and the blocked-action counter
# reported zero while blocks existed.
#
# The pattern: project the wildcard away into a helper whose every argument is
# bound at the negation site, then negate the helper.

# admin_override_present(/yes) - true when ANY admin override exists.
Decl admin_override_present(Flag) bound [/name].

# action_is_permitted(ActionType) - true when an action is permitted for any
# target/payload pair. Projects permitted/3 down to its action.
Decl action_is_permitted(ActionType) bound [/name].

# any_action_denied(/yes) - true when ANY action has been denied.
Decl any_action_denied(Flag) bound [/name].

# =============================================================================
# SECTION 21: GIT-AWARE SAFETY (Chesterton's Fence)
# =============================================================================

# git_history(FilePath, CommitHash, Author, AgeDays, Message)
Decl git_history(FilePath, CommitHash, Author, AgeDays, Message) bound [/string, /string, /string, /number, /string].

# git_state(Attribute, Value) - summarized git context for session injection
Decl git_state(Attribute, Value) bound [/name, /string].

# churn_rate(FilePath, ChangeFrequency)
Decl churn_rate(FilePath, ChangeFrequency) bound [/string, /number].

# current_user(UserName)
Decl current_user(UserName) bound [/string].

# current_time(Timestamp) - current system time for learning timestamps
Decl current_time(Timestamp) bound [/number].

# recent_change_by_other(FilePath) - derived predicate
# True if file was changed < 2 days ago by a different author
Decl recent_change_by_other(FilePath) bound [/string].

# chesterton_fence_warning(FilePath, Reason) - derived predicate
# Warns before deleting recently-changed code
Decl chesterton_fence_warning(FilePath, Reason) bound [/string, /string].

# =============================================================================
# SECTION 22: SHADOW MODE / COUNTERFACTUAL REASONING
# =============================================================================

# hypothetical(Change) - counterfactual input
Decl hypothetical(Change) bound [/string].

# derives_from_hypothetical(Implication) - derived implications for a hypothetical
Decl derives_from_hypothetical(Implication) bound [/string].

# shadow_state(StateID, ActionID, IsValid)
Decl shadow_state(StateID, ActionID, IsValid) bound [/string, /string, /name].

# simulated_effect(ActionID, FactPredicate, FactArgs)
Decl simulated_effect(ActionID, FactPredicate, FactArgs) bound [/string, /string, /string].

# safe_projection(ActionID) - derived predicate
# True if the action passes safety checks in shadow simulation
Decl safe_projection(ActionID) bound [/string].

# projection_violation(ActionID, ViolationType) - derived predicate
Decl projection_violation(ActionID, ViolationType) bound [/string, /name].

# =============================================================================
# SECTION 23: INTERACTIVE DIFF APPROVAL
# =============================================================================

# pending_mutation(MutationID, FilePath, OldContent, NewContent)
Decl pending_mutation(MutationID, FilePath, OldContent, NewContent) bound [/string, /string, /string, /string].

# mutation_approved(MutationID, ApprovedBy, Timestamp)
Decl mutation_approved(MutationID, ApprovedBy, Timestamp) bound [/string, /string, /number].

# mutation_rejected(MutationID, RejectedBy, Reason)
Decl mutation_rejected(MutationID, RejectedBy, Reason) bound [/string, /string, /string].

# requires_approval(MutationID) - derived predicate
# True if the mutation requires user approval before execution
Decl requires_approval(MutationID) bound [/string].

# =============================================================================
# SECTION 24: REGRESSION BATTERY GATING (internal/regression)
# =============================================================================
#
# A regression battery is a YAML file of shell commands. Running one is
# strictly more powerful than /exec_cmd, because the commands live in a file
# the constitution cannot see: dangerous_content/2 inspects a pending_action's
# Target and Payload, and a battery's Target is only a path. An agent that can
# write a file (safe_action(/write_file)) could therefore author a battery
# containing any blocked_pattern and launder it past the entire block list.
#
# These predicates exist so the battery's CONTENTS are visible to the kernel
# before it runs: the host projects one regression_battery_task fact per task
# (internal/regression.PolicyFacts) and the rules in policy/regression_battery.mg
# decide. See that file for the trust argument.

# regression_battery_declared(BatteryPath) - EDB: a host has submitted this
# battery for a permission decision. Asserted by the host, never derived.
Decl regression_battery_declared(BatteryPath) bound [/string].

# regression_battery_task(BatteryPath, TaskID, Command) - EDB: one shell task
# of a declared battery, with the command text the shell would receive.
Decl regression_battery_task(BatteryPath, TaskID, Command) bound [/string, /string, /string].

# regression_task_forbidden(TaskID, Pattern) - derived: this task's command
# contains a constitutionally blocked pattern.
Decl regression_task_forbidden(TaskID, Pattern) bound [/string, /string].

# regression_battery_has_task(BatteryPath) - derived: bound-negation helper
# (SECTION 11C) projecting regression_battery_task down to its battery.
Decl regression_battery_has_task(BatteryPath) bound [/string].

# regression_battery_has_forbidden_task(BatteryPath) - derived: bound-negation
# helper. A negated literal with a wildcard excludes nothing, so the wildcards
# are projected away here rather than at the negation site.
Decl regression_battery_has_forbidden_task(BatteryPath) bound [/string].

# regression_battery_permitted(BatteryPath) - derived: every task in the
# declared battery is free of blocked patterns AND the battery has at least one
# task. This is a NECESSARY condition for a host to run a battery, never a
# sufficient one — permitted/3 still has to derive for the action itself.
Decl regression_battery_permitted(BatteryPath) bound [/string].

# regression_battery_refused(BatteryPath, Reason) - derived: why a declared
# battery may not run. Exists so a refusal is reportable, not just absent.
Decl regression_battery_refused(BatteryPath, Reason) bound [/string, /string].
