# Constitutional Logic / Safety
# Section 7, 7B, 7C of Cortex Executive Policy


# Default deny - permitted must be positively derived
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _).

permitted(Action, Target, Payload) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action),
    pending_action(_, Action, Target, Payload, _).

# Downstream executor bridge:
permitted(Action, Target, Payload) :-
    permitted_action(ActionID, Action, Target, Payload, _),
    permission_check_result(ActionID, /permit, _, _).

# Fix Bug #12: The "Silent Join" (Shadow Rules)
permission_denied(Action, "Dangerous Action") :-
    dangerous_action(Action),
    !admin_override(_).

permission_denied(Action, "Dangerous Action") :-
    dangerous_action(Action),
    !signed_approval(Action).

# SAFE ACTIONS - Permitted by default for all shards
# File operations
safe_action(/read_file).
safe_action(/fs_read).
safe_action(/write_file).
safe_action(/fs_write).
safe_action(/search_files).
safe_action(/glob_files).
safe_action(/analyze_code).

# Code analysis operations
safe_action(/parse_ast).
safe_action(/query_symbols).
safe_action(/check_syntax).
safe_action(/code_graph).

# Review operations
safe_action(/review).
safe_action(/lint).
safe_action(/check_security).

# Test operations
safe_action(/run_tests).
safe_action(/test_single).
safe_action(/coverage).

# Knowledge operations
safe_action(/vector_search).
safe_action(/knowledge_query).
safe_action(/embed_text).

# Browser operations
safe_action(/browser_navigate).
safe_action(/browser_screenshot).
safe_action(/browser_read_dom).

# System lifecycle
safe_action(/initialize).
safe_action(/system_start).
safe_action(/shutdown).
safe_action(/heartbeat).

# Campaign operations
safe_action(/campaign_create_file).
safe_action(/campaign_modify_file).
safe_action(/campaign_write_test).
safe_action(/campaign_run_test).
safe_action(/campaign_research).
safe_action(/campaign_verify).
safe_action(/campaign_document).
safe_action(/campaign_refactor).
safe_action(/campaign_integrate).
safe_action(/campaign_clarify).
safe_action(/campaign_cleanup).
safe_action(/campaign_complete).
safe_action(/campaign_final_verify).
safe_action(/archive_campaign).
safe_action(/ask_campaign_interrupt).
safe_action(/show_campaign_progress).
safe_action(/show_campaign_status).
safe_action(/run_phase_checkpoint).
safe_action(/investigate_systemic).
safe_action(/pause_and_replan).

# TDD repair loop operations
safe_action(/read_error_log).
safe_action(/analyze_root_cause).
safe_action(/generate_patch).
safe_action(/complete).

# Autopoiesis/Ouroboros operations
safe_action(/generate_tool).
safe_action(/refine_tool).
safe_action(/ouroboros_detect).
safe_action(/ouroboros_generate).
safe_action(/ouroboros_compile).
safe_action(/ouroboros_register).

# Strategic/control operations
safe_action(/ask_user).
safe_action(/resume_task).
safe_action(/escalate_to_user).
safe_action(/interrogative_mode).
safe_action(/refresh_shard_context).
safe_action(/update_world_model).

# Context management operations
safe_action(/compress_context).
safe_action(/emergency_compress).
safe_action(/create_checkpoint).

# Code DOM operations
safe_action(/edit_element).
safe_action(/open_file).
safe_action(/query_elements).
safe_action(/refresh_scope).

# Corrective operations
safe_action(/corrective_decompose).
safe_action(/corrective_docs).
safe_action(/corrective_research).

# Execution operations
safe_action(/exec_cmd).

# Investigation operations
safe_action(/investigate_anomaly).

# Extended Code DOM operations
safe_action(/close_scope).
safe_action(/edit_lines).
safe_action(/insert_lines).
safe_action(/delete_lines).
safe_action(/get_elements).
safe_action(/get_element).

# Autopoiesis tool execution
safe_action(/exec_tool).

# Delegate routing patterns
safe_action(/delegate_reviewer).
safe_action(/delegate_coder).
safe_action(/delegate_researcher).
safe_action(/delegate_tool_generator).

# Network policy - allowlist approach
allowed_domain("github.com").
allowed_domain("pypi.org").
allowed_domain("crates.io").
allowed_domain("npmjs.com").
allowed_domain("pkg.go.dev").

# SECTION 7B: STRATIFIED TRUST - AUTOPOIESIS SAFETY
# The Bridge Rule: Learned suggestions must pass constitutional checks
final_action(Action) :-
    candidate_action(Action),
    permitted(Action, _, _).

# Safety check predicate for runtime validation
safety_check(Action) :-
    permitted(Action, _, _).

# Deny actions that are candidates but not permitted
action_denied(Action, "Not constitutionally permitted") :-
    candidate_action(Action),
    !permitted(Action, _, _).

# Expose denied actions for session context
forbidden(Action) :-
    action_denied(Action, _).

forbidden(Action) :-
    security_violation(Action, _, _).

# Track learned rule proposals for auditing
learned_proposal(Action) :-
    candidate_action(Action).

# Metrics: Count how many learned rules are being blocked
blocked_learned_action_count(C) :-
    action_denied(_, _)
    |> do fn:group_by(), let C = fn:count().

# Default to 0 when no blocks exist
blocked_learned_action_count(0) :-
    !action_denied(_, _).

# SECTION 7C: APPEAL MECHANISM
# Suggest appeal for ambiguous blocks (not dangerous patterns)
suggest_appeal(ActionID) :-
    appeal_available(ActionID, ActionType, Target, Reason),
    !dangerous_action(ActionType).

# Track appeals that need user review
appeal_needs_review(ActionID, ActionType, Justification) :-
    appeal_pending(ActionID, ActionType, Justification, _).

# Helper: check if an action type has a temporary override configured
has_temporary_override(ActionType) :-
    temporary_override(ActionType, _).

# Override is currently active for an action type
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    temporary_override(ActionType, Expiration),
    current_time(Now),
    Now < Expiration.

# Permanent override (no expiration)
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    !has_temporary_override(ActionType).

# Appeal granted should permit the action
permitted(ActionType, Target, Payload) :-
    has_active_override(ActionType),
    pending_action(_, ActionType, Target, Payload, _).

# Alert if too many appeals are being denied
excessive_appeal_denials() :-
    appeal_denied(_, _, _, _),
    appeal_denied(_, _, _, _),
    appeal_denied(_, _, _, _).

# Signal need for policy review if appeals frequently granted
appeal_pattern_detected(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    appeal_granted(_, ActionType, _, _).
