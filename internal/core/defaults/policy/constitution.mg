# Constitutional Logic / Safety
# Section 7, 7B, 7C of Cortex Executive Policy


# Default deny - permitted must be positively derived
# FIX: Check both Payload AND Target for dangerous content (Bug #Safety-ExecCmd)
permitted(Action, Target, Payload) :-
    safe_action(Action),
    pending_action(_, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target),
    !restricted_tuple(Action, Target, Payload),
    !consent_gated_tuple(Action, Target, Payload).

# A turn restriction (section 7D below) is joined into every arm of
# permitted/3 as a negation. restricted_tuple/3 is a call the user's own
# limit bars for this turn (no changes, read-only, dry run, what-if);
# consent_gated_tuple/3 is a call the user asked to confirm first and has not
# yet decided. Neither adds a permission: a barred call stays barred even if
# another arm would have derived it, and a call owed confirmation is permitted
# again only by the person's decision for that exact call, through the
# human-approval arm below.

# Request-bound human approval: the only human-consent arm.
#
# A person approves one frozen request (internal/approval), not an action
# type. After the execution claim on that request succeeds against the
# operation as it stands now, the host asserts human_approval_granted for the
# exact pending_action it admits -- same ActionID, Action, Target and Payload
# -- and retracts it with that pending_action. A sibling request, a retry, a
# changed payload or another target joins nothing here. dangerous_content
# still applies: consent satisfies the approval obligation, not the content
# checks, so a hard prohibition stays a prohibition.
#
# This replaced `dangerous_action(A), signed_approval(A), pending_action(...),
# admin_override(U)`, which approved every pending action of a type at once
# and whose User was joined to nothing (2026-10-27).
permitted(Action, Target, Payload) :-
    dangerous_action(Action),
    human_approvable(Action),
    pending_action(ActionID, Action, Target, Payload, _),
    human_approval_granted(ActionID, Action, Target, Payload, _),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target),
    !restricted_tuple(Action, Target, Payload).

# Which denials a person may answer. Only these two action types; every other
# requires_permission action (git_push, git_force, system_modify,
# run_regression_battery) has no consent arm and stays denied. /delete_file is
# the registered tool; /run_arbitrary_command is the action a host-reviewed
# MCP /execute effect maps to (policy_mcp.mg).
human_approvable(/delete_file).
human_approvable(/run_arbitrary_command).

# The executor asks this while the pending_action is asserted, and opens an
# approval request only when it holds: denied, approvable, and nothing
# dangerous in the target or payload.
human_approval_required(ActionID) :-
    pending_action(ActionID, Action, Target, Payload, _),
    dangerous_action(Action),
    human_approvable(Action),
    !permitted(Action, Target, Payload),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target),
    !restricted_tuple(Action, Target, Payload).

# A call the user asked to confirm first (section 7D below), on an action
# the constitution otherwise permits, opens the same request. A restricted
# call never does: a person cannot approve what their own request barred.
human_approval_required(ActionID) :-
    pending_action(ActionID, Action, Target, Payload, _),
    confirmation_owed(ActionID),
    safe_action(Action),
    !permitted(Action, Target, Payload),
    !dangerous_content(Action, Payload),
    !dangerous_content(Action, Target),
    !restricted_tuple(Action, Target, Payload).

# A delete the repository can undo.
#
# requires_permission(/delete_file) stays, and so does the refusal for every
# other delete. This rule fires only when the executor has asserted
# file_recoverable(Target), which it does only for a target git tracks with
# nothing staged and nothing modified -- `git checkout -- <path>` restores the
# exact bytes. The premise the permission exists to enforce, that deletion
# cannot be undone, is false for exactly this case.
#
# Untracked files, files with uncommitted work, paths outside the workspace and
# anything the check cannot determine assert no fact and stay denied.
# dangerous_content is still consulted, as on the safe_action path.
permitted(/delete_file, Target, Payload) :-
    pending_action(_, /delete_file, Target, Payload, _),
    file_recoverable(Target),
    !dangerous_content(/delete_file, Payload),
    !dangerous_content(/delete_file, Target),
    !restricted_tuple(/delete_file, Target, Payload),
    !consent_gated_tuple(/delete_file, Target, Payload).

# Downstream executor bridge:
permitted(Action, Target, Payload) :-
    permitted_action(ActionID, Action, Target, Payload, _),
    permission_check_result(ActionID, /permit, _, _),
    !restricted_tuple(Action, Target, Payload),
    !consent_gated_tuple(Action, Target, Payload).

# Bound-negation helpers. See SECTION 11C in schemas_safety.mg: a negated
# literal containing an anonymous wildcard excludes nothing in this Mangle
# build, so each of the three negations below projects the wildcard away first.

action_is_permitted(Action) :-
    permitted(Action, Target, Payload).

any_action_denied(/yes) :-
    action_denied(Action, Reason).

# Fix Bug #12: The "Silent Join" (Shadow Rules). A dangerous action is denied
# for each pending request of it the constitution did not permit. Every
# negated argument is bound by pending_action, and both live in the policy
# shard, so the negation excludes exactly the admitted (e.g. human-approved)
# requests -- an action-type-wide "override present" flag no longer exists.
permission_denied(Action, "Dangerous Action") :-
    pending_action(ActionID, Action, Target, Payload, _),
    dangerous_action(Action),
    !permitted(Action, Target, Payload).

# SAFE ACTIONS - Permitted by default for all shards
# File operations
safe_action(/read_file).
safe_action(/recall_context).
safe_action(/fs_read).
safe_action(/write_file).
safe_action(/fs_write).
# /edit_file is a targeted search/replace mutation — strictly LESS powerful than
# /write_file (wholesale overwrite), which is already safe above. It was absent
# from this allowlist while being treated identically to /write_file everywhere
# else (requires_paranoid_validation, interactive_side_effect_type,
# specialist_allowed_tools), so the constitutional gate silently denied every
# edit the agent tried and failed the task ("tool call blocked by safety gate:
# edit_file"). Confirmed live: `nerd run` on a mutation intent reaches
# /delegate_coder, then the coder's edit_file is gated. Same bug class as the
# /glob alias and /grep additions above; paranoid validation still applies.
safe_action(/edit_file).
safe_action(/fs_edit).
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

# /search_expand reads back the matching lines a /search_code call already
# retained in this process. It runs no search and touches no file, so it grants
# strictly less than /search_code did a moment earlier. Denying it would leave
# every search_code result advertising a handle nothing could redeem, which is
# worse than never eliding the lines at all.
safe_action(/search_expand).

# /subagent_expand reads back the transcript a delegation already retained in
# this process. It spawns nothing, runs nothing and touches no file, so it
# grants strictly less than the /delegate that minted the handle — which is the
# whole reason it is a verb of its own rather than an argument on /delegate. A
# depth or budget cap that denies further delegation must not also take away the
# transcript of the delegation that already happened.
safe_action(/subagent_expand).

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

# Build operations
# /build_project executes the project build command through the same guarded
# pipeline as /run_tests (command allowlist, binary policy, Dreamer
# simulation, exact pending_action payload). It was absent from this
# allowlist, so the constitutional gate silently denied every build the TDD
# loop routed and every repair cycle ended in escalation. Same bug class as
# the /edit_file and /grep additions above; capabilities.mg already derives
# next_action(/build_project) for /build intents, so the system intends it
# routable.
safe_action(/build_project).

# Knowledge operations
safe_action(/vector_search).
safe_action(/knowledge_query).
safe_action(/embed_text).

# Browser operations
safe_action(/browser_navigate).
safe_action(/browser_screenshot).
safe_action(/browser_read_dom).
# Registered modular spellings. Availability still comes from the effective
# JIT allowlist and execution still requires an exact pending_action payload;
# these facts only let the constitutional rule positively derive permitted/3.
safe_action(/browser_extract).
safe_action(/browser_click).
safe_action(/browser_type).
safe_action(/browser_close).
safe_action(/browser_observe).
safe_action(/browser_act).
safe_action(/browser_mangle).
safe_action(/browser_wait).
safe_action(/browser_reason).
safe_action(/browser_evidence).
safe_action(/browser_specs).
safe_action(/browser_test).
safe_action(/browser_audit).
# MCP control plane. These five verbs front every connected MCP server; they
# are safe as VERBS, and that is the correct granularity here. The constitution
# cannot know the blast radius of a remote tool it has never seen, so the risk
# decision is made one level down, per remote tool, by mcp_tool_gated in
# policy_mcp.mg against a class derived at discovery. Marking mcp_call blanket
# dangerous would prompt on every read through MCP and teach the operator to
# wave the prompts through, which is worse than no prompt at all.
safe_action(/mcp_map).
safe_action(/mcp_probe).
safe_action(/mcp_call).
safe_action(/mcp_expand).
safe_action(/mcp_context).
# Provider-mediated, read-only research; JIT routing still limits availability.
safe_action(/grounded_web_search).
# Registered and granted by the JIT ConfigFactory, but absent here until
# 2026-09-04, so every call was hard-denied ("action not permitted") while
# the model was still offered the tool — the drift class documented above
# for /edit_file. Added by hand during the subsystem pass; the catalog
# golden test in internal/tools pins registry↔policy parity from now on.
safe_action(/web_search).
safe_action(/web_fetch).
safe_action(/context7_fetch).
safe_action(/get_impacted_tests).
safe_action(/run_impacted_tests).
safe_action(/apply_edits).
# The research cache is an in-process map with no external side effect;
# get/set/stats are granted to /research verbs by the ConfigFactory and were
# likewise hard-denied here. research_cache_clear is deliberately NOT listed:
# it discards work every agent in the process shares, and the registry-to-
# policy parity test (internal/tools/catalog_policy_parity_test.go) records
# that exclusion with its reason.
safe_action(/research_cache_get).
safe_action(/research_cache_set).
safe_action(/research_cache_stats).

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
safe_action(/campaign_final_verify).
safe_action(/archive_campaign).
safe_action(/ask_campaign_interrupt).
safe_action(/show_campaign_progress).
safe_action(/show_campaign_status).
safe_action(/investigate_systemic).

# TDD repair loop operations
safe_action(/read_error_log).
safe_action(/analyze_root_cause).
safe_action(/generate_patch).
safe_action(/complete).

# Autopoiesis/Ouroboros operations
safe_action(/generate_tool).
safe_action(/refine_tool).
safe_action(/ouroboros_generate).

# Strategic/control operations
safe_action(/ask_user).
safe_action(/resume_task).
safe_action(/escalate_to_user).
safe_action(/interrogative_mode).
safe_action(/refresh_shard_context).

# Context management operations
safe_action(/create_checkpoint).

# Code DOM operations
safe_action(/edit_element).
safe_action(/open_file).
safe_action(/query_elements).
safe_action(/refresh_scope).

# Execution operations
safe_action(/exec_cmd).
safe_action(/run_command).
safe_action(/bash).
safe_action(/run_build).
safe_action(/run_tests).
# /run_check runs the campaign's own acceptance command with no model
# arguments; the argv is the campaign's declaration, so there is no payload
# for dangerous_content to judge.
safe_action(/run_check).
# /run_lint runs go vet, staticcheck or golangci-lint with the repository's
# argv; the model chooses the gate from a closed set and workspace-relative
# packages, never a command or a flag.
safe_action(/run_lint).

# /regenerate runs one generator the workspace's nerd.md declares and nothing
# else; its writes are the generator's declared outputs, guarded and undoable
# like any write (internal/tools/shell/regenerate.go). It is how an agent
# repairs a generated file its change left stale without free-form shell.
safe_action(/regenerate).
safe_action(/git_operation).
safe_action(/git_diff).
safe_action(/git_log).

# SWE-bench operations — benchmark equivalents of already-permitted operations
# (/write_file, /edit_file and /run_tests are all safe_action today). Handlers
# live in internal/core/virtual_store_python.go and the router already marks
# them RequiresSafe, so the ConstitutionGate can derive permitted/3.
safe_action(/swebench_setup).
safe_action(/swebench_apply_patch).
safe_action(/swebench_run_tests).
safe_action(/swebench_snapshot).
safe_action(/swebench_restore).
safe_action(/swebench_evaluate).
safe_action(/swebench_teardown).

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

# A secret file's contents never reach a model: whatever a tool returns goes to
# the provider. The executor measures the target against execution.secret_paths
# and asserts touches_secret_path; this makes it dangerous content for every
# action, so the permitted rules above (safe_action, the recoverable delete)
# cannot derive for it. Hand-built 2026-09-22: a model must not widen the rule
# that constrains it.
dangerous_content(Action, Target) :-
    pending_action(_, Action, Target, _, _),
    touches_secret_path(Target).

# What the host acts on. A Go site dispatches next_action and the executive's
# next-action variants, spawns delegate_task, and gates pending_action through
# permitted. Hand-built 2026-09-22 with the prose_only rows below.
exec_sink(/next_action).
exec_sink(/tdd_next_action).
exec_sink(/campaign_next_action).
exec_sink(/repair_next_action).
exec_sink(/delegate_task).
exec_sink(/pending_action).
exec_sink(/permitted).

# Model-written facts whose strings are prose: a reviewer's reason and a
# model's observation are read back and printed, and a readable reason has
# semicolons in it. Observed 2026-09-21 (campaign aab9612b) and 2026-09-22
# (7b853890): a /fail verdict and an observation were dropped for their
# punctuation, and the checkpoint reported that no verdict existed.
prose_only(/checkpoint_verdict).
prose_only(/observation).

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
# Structural queries read a parsed index of the workspace and nothing else (hand-built, 2026-09-21).
safe_action(/find_symbol).
safe_action(/package_outline).
safe_action(/callers_of).
safe_action(/callees_of).
safe_action(/unreferenced_symbols).
safe_action(/get_element).
# Records a reasoned no-change decision for a code family peer; writes nothing
# (code_families.mg, 2026-10-28).
safe_action(/family_no_change).
# Structural reads and element-addressed edits (CodeDOM, R8, 2026-09-23). The
# edits are the same class as edit_lines and write_file: every one names its
# file(s) in path/paths, so the secret-path and write gates see its target.
safe_action(/importers_of).
safe_action(/find_text).
safe_action(/predicate_outline).
safe_action(/replace_element).
safe_action(/insert_element).
safe_action(/delete_element).
safe_action(/create_file).
safe_action(/repoint).

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

# Deny actions that are candidates but not permitted.
# `!permitted(Action, _, _)` excluded nothing, so this fired for every candidate
# action including the ones the constitution had just permitted.
action_denied(Action, "Not constitutionally permitted") :-
    candidate_action(Action),
    !action_is_permitted(Action).

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

# Default to 0 when no blocks exist.
# `!action_denied(_, _)` excluded nothing, so this reported zero blocked actions
# even while denials existed — a safety counter that always read clean.
blocked_learned_action_count(0) :-
    !any_action_denied(/yes).

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
    pending_action(_, ActionType, Target, Payload, _),
    !restricted_tuple(ActionType, Target, Payload),
    !consent_gated_tuple(ActionType, Target, Payload).

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


# SECTION 7D: Turn restriction. The limit a user put on the request being served, enforced
# where a tool is offered and where it is admitted
# (Docs/audits/perception-restriction-continuity.md).
#
# Perception recognises the limit (read-only, no changes, dry run, a what-if,
# "confirm with me first") and the session executor carries it on the turn's
# context (internal/perception/restriction.go). Go measures: it asserts the
# turn's restriction kinds and each tool's reviewed effect (tools.LookupEffect,
# /unknown when the tool declares none), and nothing more. This section derives
# what a restriction forbids and what it owes. A restriction narrows: it is
# joined into every permitted/3 arm (above) as a negation, so no
# independently derived permission outranks it, and it never grants anything.
# The user's own words are not an approval; a person's decision arrives only
# as human_approval_granted, through the request-bound approval broker.
#
# EDB, per turn (retracted with the turn's other facts):
#   turn_restriction(Turn, Kind)                 each recognised kind
#   restriction_tool_effect(Turn, Tool, Effect)  the catalog's tools, measured
# EDB, per admitted call (retracted with its pending_action):
#   pending_action_turn(ActionID, Turn)          the turn whose restriction binds it
#   pending_action_effect(ActionID, Effect)      its tool's declared effect
#
# Only turns that carry a restriction assert anything here, so an unrestricted
# turn pays nothing and joins nothing.

Decl turn_restriction(Turn, Kind) bound [/name, /name].
Decl restriction_tool_effect(Turn, Tool, Effect) bound [/name, /name, /name].
Decl pending_action_turn(ActionID, Turn) bound [/string, /name].
Decl pending_action_effect(ActionID, Effect) bound [/string, /name].

Decl effect_barring_kind(Kind) bound [/name].
Decl barred_effect(Effect) bound [/name].
Decl consequential_effect(Effect) bound [/name].
Decl turn_bars_effects(Turn) bound [/name].
Decl turn_owes_confirmation(Turn) bound [/name].
Decl pending_action_effect_known(ActionID) bound [/string].
Decl restriction_withholds_tool(Turn, Tool) bound [/name, /name].
Decl restricted_action(ActionID) bound [/string].
Decl restricted_tuple(Action, Target, Payload) bound [/name, /string, /string].
Decl confirmation_owed(ActionID) bound [/string].
Decl consent_given(ActionID) bound [/string].
Decl consent_gated_tuple(Action, Target, Payload) bound [/name, /string, /string].

# --- What a restriction forbids ------------------------------------------------

# These kinds all say the workspace stays as it is. They differ in what the
# user said, not in what they forbid; the receipt names the user's word.
effect_barring_kind(/read_only).
effect_barring_kind(/no_changes).
effect_barring_kind(/dry_run).
effect_barring_kind(/hypothetical).

# A tool that writes, runs a command, or declares no effect is barred. An
# unknown effect is never read-only. /external (web, browser, MCP) is not
# barred: the manifest does not separate a lookup from a remote mutation, so
# the restriction is a statement about the workspace, and an external call is
# outside what it can honestly promise.
barred_effect(/write).
barred_effect(/execute).
barred_effect(/unknown).

turn_bars_effects(Turn) :-
    turn_restriction(Turn, Kind),
    effect_barring_kind(Kind).

# The catalog: a barred tool is not offered to the turn, so its schema and the
# atoms that teach it never reach the model. Admission below is the second
# line for a call made anyway.
restriction_withholds_tool(Turn, Tool) :-
    turn_bars_effects(Turn),
    restriction_tool_effect(Turn, Tool, Effect),
    barred_effect(Effect).

# Admission. A call whose effect was not measured is barred too: a measurement
# that failed is a guard that did not run.
pending_action_effect_known(ActionID) :-
    pending_action_effect(ActionID, _).

restricted_action(ActionID) :-
    pending_action_turn(ActionID, Turn),
    turn_bars_effects(Turn),
    pending_action_effect(ActionID, Effect),
    barred_effect(Effect).
restricted_action(ActionID) :-
    pending_action_turn(ActionID, Turn),
    turn_bars_effects(Turn),
    !pending_action_effect_known(ActionID).

# permitted/3 is keyed by the tuple, so the arms negate the tuple.
restricted_tuple(Action, Target, Payload) :-
    pending_action(ActionID, Action, Target, Payload, _),
    restricted_action(ActionID).

# --- What a restriction owes ---------------------------------------------------

# "Confirm first" owes the user's decision before a consequential effect: a
# write, a command, or an effect nobody declared. Reading owes nothing.
consequential_effect(/write).
consequential_effect(/execute).
consequential_effect(/unknown).

turn_owes_confirmation(Turn) :-
    turn_restriction(Turn, /confirm_first).

confirmation_owed(ActionID) :-
    pending_action_turn(ActionID, Turn),
    turn_owes_confirmation(Turn),
    pending_action_effect(ActionID, Effect),
    consequential_effect(Effect).
confirmation_owed(ActionID) :-
    pending_action_turn(ActionID, Turn),
    turn_owes_confirmation(Turn),
    !pending_action_effect_known(ActionID).

# The person's decision on this exact pending action (the request-bound
# broker, constitution.mg): same ActionID, Action, Target and Payload.
consent_given(ActionID) :-
    pending_action(ActionID, Action, Target, Payload, _),
    human_approval_granted(ActionID, Action, Target, Payload, _).

# A call that owes confirmation and has none is not permitted by any arm
# except the human-approval arm. Once the person has decided for this exact
# call, the gate no longer holds it back, and the ordinary arms and every other
# check run as they always did.
consent_gated_tuple(Action, Target, Payload) :-
    pending_action(ActionID, Action, Target, Payload, _),
    confirmation_owed(ActionID),
    !consent_given(ActionID).


# SECTION 7E: Task authorization. What the user's own request allowed the
# turn's tasks to do beyond editing, enforced where a shell mutation is
# admitted (internal/projectdoc/shell_authority.go, called by the session
# executor's shell-effect gate and the tool-layer write guard).
#
# The shell-effect gate refuses every mutating command by default: command
# text is evidence about effect and never authorizes itself. A request that
# names a step ("fix X, then commit it") does authorize that step, and the
# refusal made a coder finish its edits and then be unable to commit them
# (2026-10-28). The words are read by Jev (policy/jev_evidence.mg:
# jev_authorization), carried on the turn's context (perception.Authorization)
# and asserted per turn; Go measures the call and this section decides.
#
# EDB, per turn (retracted with the turn's other facts):
#   turn_authorization(Turn, Kind)    a kind the request allowed
# EDB, per gated shell call (asserted by the gate, retracted with it):
#   shell_call(CallID, Turn, Step)    the call is exactly one simple step of a
#                                     kind Go recognises; no fact for any other
#                                     command shape (compound, extra flags, a
#                                     different subcommand), so those stay
#                                     refused
#   shell_call_in_write_set(CallID)   every path the step touches (the paths it
#                                     stages, or the paths already staged for a
#                                     commit) is a file this turn wrote
#
# Narrow by construction: a kind covers only the steps authorization_covers
# lists. "commit" covers staging and committing, and no row covers a push, a
# force, a reset, a branch change or an amend, so those stay refused whatever
# the user said. A restriction on the turn wins (a read-only request is never
# authorized to commit), and so does a standing "confirm first".

Decl turn_authorization(Turn, Kind) bound [/name, /name].
Decl authorization_covers(Kind, Step) bound [/name, /name].
Decl shell_call(CallID, Turn, Step) bound [/string, /name, /name].
Decl shell_call_in_write_set(CallID) bound [/string].
Decl shell_call_authorized(CallID) bound [/string].

authorization_covers(/commit, /git_add).
authorization_covers(/commit, /git_commit).

shell_call_authorized(CallID) :-
    shell_call(CallID, Turn, Step),
    shell_call_in_write_set(CallID),
    turn_authorization(Turn, Kind),
    authorization_covers(Kind, Step),
    !turn_bars_effects(Turn),
    !turn_owes_confirmation(Turn).
