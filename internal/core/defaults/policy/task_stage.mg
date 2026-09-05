# Lifecycle Stage Derivation (Phase 1)
# Campaign: the right context, at the right time, for the right thing
# (references/campaign-lifecycle-2026-09-04.md, Phase 1 bullet 1).
#
# task_stage/1 is derived in policy from the current intent, never from a Go
# switch. This mirrors the README precedent: model-tier routing lives in
# delegation.mg as intent_requires_reasoning_model/1, not as Go branching.
# Per-stage context policy feeding injectable_context is a follow-up task;
# this file owns only the intent -> stage table plus the single derivation.
#
# Stages (9): /ideate, /design, /plan, /implement, /verify, /harden,
#             /debug, /refactor, /review.
#
# Table notes (verb inventory from taxonomy.mg + delegation.mg action_mapping
# + intent_routing_rules.mg persona map):
# - /fix -> /debug (not /implement): the bug-fixing slice measured by
#   SWE-bench. /implement is new-functionality creation (/create, /implement).
# - /security -> /harden: active hardening work. /audit and /lint stay in
#   /review to match current reviewer routing (delegation.mg maps both to
#   /delegate_reviewer); move them only when a golden task shows the seam.
# - /document -> /design: design artifacts (spec, architecture) live in the
#   design stage alongside /design. General prose stays researcher-routed;
#   stage shapes context, shard routing is unchanged.
# - /run -> /verify: executing code to check behaviour belongs to verification.
# - /dream, /shadow -> /ideate: speculative alternatives are ideation.
# - /init -> /ideate: codebase bootstrap is discovery ("Initialize codebase
#   analysis" delegates to /researcher today).
# - /delete -> /implement: destructive mutation needs full implementation
#   context (callers, covering tests, build env), not a narrowed slice.
# - /migrate -> /implement (platform change, new behaviour);
#   /optimize, /format -> /refactor (behaviour-preserving restructuring).
# - Forward-compat rows (/ideate, /architect, /multi_step, /harden, /generate
#   verbs) are harmless EDB: they fire only when those intents land.
#
# Intentionally unmapped (derive NO task_stage): conversational and workflow
# verbs with no lifecycle slice (/help, /greet, /stats, /commit, /git, /push,
# /list_tools, /tool_status, ...). Add a row only when a golden task needs
# the stage; an absent stage is honest, a guessed stage mis-shapes context.
#
# World-state extension point (deliberately omitted for smallest diff):
# a future rule may override by campaign task type (e.g. next_campaign_task
# of type /verify forces /verify). No golden task needs it yet.

Decl intent_stage(IntentVerb, Stage) bound [/name, /name].
Decl task_stage(Stage) bound [/name].

# --- /ideate: discovery, understanding, speculation ---
intent_stage(/brainstorm, /ideate).
intent_stage(/ideate, /ideate).
intent_stage(/explore, /ideate).
intent_stage(/research, /ideate).
intent_stage(/learn, /ideate).
intent_stage(/understand, /ideate).
intent_stage(/find, /ideate).
intent_stage(/search, /ideate).
intent_stage(/explain, /ideate).
intent_stage(/dream, /ideate).
intent_stage(/shadow, /ideate).
intent_stage(/init, /ideate).

# --- /design: architecture and design artifacts ---
intent_stage(/design, /design).
intent_stage(/document, /design).
intent_stage(/architect, /design).

# --- /plan: decomposition and orchestration ---
intent_stage(/plan, /plan).
intent_stage(/campaign, /plan).
intent_stage(/multi_step, /plan).

# --- /implement: new functionality and file-mutating creation ---
intent_stage(/create, /implement).
intent_stage(/implement, /implement).
intent_stage(/write, /implement).
intent_stage(/add, /implement).
intent_stage(/update, /implement).
intent_stage(/modify, /implement).
intent_stage(/scaffold, /implement).
intent_stage(/generate, /implement).
intent_stage(/generate_tool, /implement).
intent_stage(/refine_tool, /implement).
intent_stage(/migrate, /implement).
intent_stage(/configure, /implement).
intent_stage(/delete, /implement).

# --- /verify: tests and measurement ---
intent_stage(/test, /verify).
intent_stage(/cover, /verify).
intent_stage(/verify, /verify).
intent_stage(/validate, /verify).
intent_stage(/benchmark, /verify).
intent_stage(/profile, /verify).
intent_stage(/run, /verify).

# --- /harden: security hardening ---
intent_stage(/harden, /harden).
intent_stage(/security, /harden).

# --- /debug: bug fixing ---
intent_stage(/debug, /debug).
intent_stage(/fix, /debug).

# --- /refactor: behaviour-preserving restructuring ---
intent_stage(/refactor, /refactor).
intent_stage(/optimize, /refactor).
intent_stage(/format, /refactor).

# --- /review: inspection and verdicts ---
intent_stage(/review, /review).
intent_stage(/review_enhance, /review).
intent_stage(/analyze, /review).
intent_stage(/inspect, /review).
intent_stage(/check, /review).
intent_stage(/audit, /review).
intent_stage(/lint, /review).
intent_stage(/diff, /review).

# --- Derivation: any intent id (interactive /current_intent and subagent
# /task_intent_N) joins the table on verb. Category, Target, and Constraint
# are ignored: stage follows what the turn is for, not what it is about. ---
task_stage(Stage) :-
    user_intent(_, _, Verb, _, _),
    intent_stage(Verb, Stage).
