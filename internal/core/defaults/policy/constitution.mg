# Constitutional Logic / Safety
# Section 7, 7B, 7C of Cortex Executive Policy


# Default deny - permitted must be positively derived
# FIX: Check both Payload AND Target for dangerous content (Bug #Safety-ExecCmd)
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target).

permitted(Action, Target, Payload) :-
    dangerous_action(Action),
    signed_approval(Action),
    pending_action(_, Action, Target, Payload, _),
    admin_override(User).

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
# Aliases for tool-registry names that the LLM actually emits
# (the registry exposes /glob and /list_files; the policy was only
# permitting the /glob_files spelling, which silently blocked every
# directory listing the agent tried to do). See session.log for the
# 2026-05-28 cascade of "tool call blocked by safety gate".
safe_action(/glob).
safe_action(/list_files).
safe_action(/find_files).
# /grep and /search_code are safe, read-only, in-process search tools
# (internal/tools/core/search.go uses filepath.Walk + regexp — no shell). They were
# absent from this allowlist, so the gate denied them and shards fell back to shelling
# out to findstr/rg (findstr mangles '/'-paths, rg not installed) and every audit
# search failed. /grep grants strictly LESS than /read_file (already permitted below):
# it returns matching lines, not whole files. Same bug class as the /glob alias above.
safe_action(/grep).
safe_action(/search_code).
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
safe_action(/run_command).
safe_action(/bash).
safe_action(/run_build).
safe_action(/run_tests).
safe_action(/git_operation).
safe_action(/git_diff).
safe_action(/git_log).

# Blocked patterns for dangerous commands
blocked_pattern("git push --force").
blocked_pattern("git push -f").
blocked_pattern("git push origin --force").
blocked_pattern("git push origin -f").
blocked_pattern("rm -rf /").
blocked_pattern("rm -rf").
blocked_pattern("sudo").
blocked_pattern("> /dev/").
blocked_pattern("mkfs").
blocked_pattern("dd if=").
blocked_pattern("chmod 777").
blocked_pattern("chmod -R 777").
blocked_pattern(":(){").
blocked_pattern("| bash").
blocked_pattern("| sh").
blocked_pattern("nc -e").

# Centralized Permissions (merged from intent_routing.mg)
requires_permission(/delete_file).
requires_permission(/git_push).
requires_permission(/git_force).
requires_permission(/run_arbitrary_command).
requires_permission(/system_modify).

# Wiring: Actions requiring permission are dangerous by default
# Note: dangerous_action takes ActionType (e.g., /delete_file), not ActionID.
dangerous_action(ActionType) :- requires_permission(ActionType).

# Identify dangerous command content
# string_contains requires both args bound in the same rule.
# Pattern: bind Payload/Target from pending_action, then check each constant pattern.

# --- exec_cmd Payload ---
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "git push --force").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "git push -f").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "git push origin --force").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "git push origin -f").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "rm -rf /").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "rm -rf").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "sudo").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "> /dev/").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "mkfs").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "dd if=").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "chmod 777").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "chmod -R 777").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, ":(){").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "| bash").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "| sh").
dangerous_content(/exec_cmd, Payload) :- pending_action(_, /exec_cmd, _, Payload, _), :string:contains(Payload, "nc -e").

# --- exec_cmd Target ---
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "git push --force").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "git push -f").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "git push origin --force").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "git push origin -f").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "rm -rf /").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "rm -rf").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "sudo").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "> /dev/").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "mkfs").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "dd if=").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "chmod 777").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "chmod -R 777").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, ":(){").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "| bash").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "| sh").
dangerous_content(/exec_cmd, Target) :- pending_action(_, /exec_cmd, Target, _, _), :string:contains(Target, "nc -e").

# --- run_command Payload ---
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "git push --force").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "git push -f").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "git push origin --force").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "git push origin -f").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "rm -rf /").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "rm -rf").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "sudo").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "> /dev/").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "mkfs").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "dd if=").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "chmod 777").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "chmod -R 777").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, ":(){").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "| bash").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "| sh").
dangerous_content(/run_command, Payload) :- pending_action(_, /run_command, _, Payload, _), :string:contains(Payload, "nc -e").

# --- run_command Target ---
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "git push --force").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "git push -f").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "git push origin --force").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "git push origin -f").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "rm -rf /").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "rm -rf").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "sudo").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "> /dev/").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "mkfs").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "dd if=").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "chmod 777").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "chmod -R 777").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, ":(){").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "| bash").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "| sh").
dangerous_content(/run_command, Target) :- pending_action(_, /run_command, Target, _, _), :string:contains(Target, "nc -e").

# --- bash Payload ---
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "git push --force").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "git push -f").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "git push origin --force").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "git push origin -f").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "rm -rf /").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "rm -rf").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "sudo").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "> /dev/").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "mkfs").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "dd if=").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "chmod 777").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "chmod -R 777").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, ":(){").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "| bash").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "| sh").
dangerous_content(/bash, Payload) :- pending_action(_, /bash, _, Payload, _), :string:contains(Payload, "nc -e").

# --- bash Target ---
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "git push --force").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "git push -f").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "git push origin --force").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "git push origin -f").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "rm -rf /").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "rm -rf").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "sudo").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "> /dev/").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "mkfs").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "dd if=").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "chmod 777").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "chmod -R 777").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, ":(){").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "| bash").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "| sh").
dangerous_content(/bash, Target) :- pending_action(_, /bash, Target, _, _), :string:contains(Target, "nc -e").

# --- exec_cmd: Catch --force/-f at any position in git push commands ---
dangerous_content(/exec_cmd, Payload) :-
    pending_action(_, /exec_cmd, _, Payload, _),
    :string:contains(Payload, "git push"),
    :string:contains(Payload, "--force").

dangerous_content(/exec_cmd, Payload) :-
    pending_action(_, /exec_cmd, _, Payload, _),
    :string:contains(Payload, "git push"),
    :string:contains(Payload, "-f").

dangerous_content(/exec_cmd, Target) :-
    pending_action(_, /exec_cmd, Target, _, _),
    :string:contains(Target, "git push"),
    :string:contains(Target, "--force").

dangerous_content(/exec_cmd, Target) :-
    pending_action(_, /exec_cmd, Target, _, _),
    :string:contains(Target, "git push"),
    :string:contains(Target, "-f").

dangerous_content(/run_command, Payload) :-

    pending_action(_, /run_command, _, Payload, _),
    :string:contains(Payload, "git push"),
    :string:contains(Payload, "--force").

dangerous_content(/run_command, Payload) :-
    pending_action(_, /run_command, _, Payload, _),
    :string:contains(Payload, "git push"),
    :string:contains(Payload, "-f").

dangerous_content(/bash, Payload) :-
    pending_action(_, /bash, _, Payload, _),
    :string:contains(Payload, "git push"),
    :string:contains(Payload, "--force").

dangerous_content(/bash, Payload) :-
    pending_action(_, /bash, _, Payload, _),
    :string:contains(Payload, "git push"),
    :string:contains(Payload, "-f").

# Safety checks for git_operation
dangerous_content(/git_operation, Payload) :-
    pending_action(_, /git_operation, _, Payload, _),
    :string:contains(Payload, "\"operation\":\"push\""),
    :string:contains(Payload, "--force").

dangerous_content(/git_operation, Payload) :-
    pending_action(_, /git_operation, _, Payload, _),
    :string:contains(Payload, "\"operation\":\"push\""),
    :string:contains(Payload, "-f").

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
safe_action(/delegate_tester).
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
# blocked_learned_action_count(C) :-
#     action_denied(_, _)
#     |> do fn:group_by(), let C = fn:count().

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
# Fixed: current_time (singleton) comes first to avoid cross-product with appeal_granted
has_active_override(ActionType) :-
    current_time(Now),
    appeal_granted(_, ActionType, _, _),
    temporary_override(ActionType, Expiration),
    Now < Expiration.

# Permanent override (no expiration)
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    !has_temporary_override(ActionType).

# Appeal granted should permit the action
permitted(ActionType, Target, Payload) :-
    has_active_override(ActionType),
    pending_action(_, ActionType, Target, Payload, _).

# Count denied appeals for thresholding
# appeal_denial_count(Count) :-
#     appeal_denied(_, _, _, _)
#     |> do fn:group_by(), let Count = fn:count().

# Alert if too many appeals are being denied
excessive_appeal_denials() :-
    appeal_denial_count(Count),
    Count >= 3.

# Count granted appeals by action type for pattern detection
# appeal_grant_count(ActionType, Count) :-
#     appeal_granted(_, ActionType, _, _)
#     |> do fn:group_by(ActionType), let Count = fn:count().

# Signal need for policy review if appeals frequently granted
appeal_pattern_detected(ActionType) :-
    appeal_grant_count(ActionType, Count),
    Count >= 2.
