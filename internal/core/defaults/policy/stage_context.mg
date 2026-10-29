# Per-stage context policy — Mangle tables, not Go switches.
# Producer: task_stage/1 (task_stage.mg, derived from user_intent) +
#   active_shard/2 + context_budget_sufficient/1 and
#   context_budget_constrained/1 (schemas_prompts.mg). Consumer: injectable_context readers
#   (articulation prompt_assembler + JIT compiler collectKernelInjectedAtoms)
#   and future final_injectable/tool-list wiring.
# Value domains: Stage /ideate /design /plan /implement /verify /harden
#   /debug /refactor /review. Family /intent_target /failure_recall
#   /trace_recall /specialist /learning /campaign /tool_desc /exemplar
#   /callers /covering_tests /build_env. Category is an AtomCategory
#   /name (methodology, safety, domain, context, exemplar, knowledge,
#   reviewer, protocol, world_state, campaign). Capability is the
#   tool-routing vocabulary /generation /debugging /transformation
#   /inspection /validation /execution /analysis /knowledge.
#
# Families ground in prompt_context.mg producers: /intent_target is the
# 90-relevance intent match, /failure_recall the 90 failure recall,
# /trace_recall the 85 trace recall, /specialist the 80 specialist
# knowledge, /learning the 75 learning recall, /campaign the 70 campaign
# constraint, /tool_desc the 65 tool description, /exemplar the 60 learned
# exemplar. /callers /covering_tests /build_env are Phase-4 forward-compat
# (caller graph, covering tests, build env via test_framework/build_command);
# they fire only when those producers land and are harmless EDB until then.
#
# Monotonicity (read before extending): Mangle rules only ADD facts, so a
# forbidden row here cannot retract prompt_context.mg's Relevance > 50
# admissions. Forbidden rows derive the stage_forbidden_active/2 veto signal
# for a future final_injectable revision to consume; the injectable_context
# bridge below only ADMITS one stage-guidance atom per active shard.
# Absence is neutral: a family/category/capability listed in neither table
# is neither admitted nor denied here; budget and relevance decide.
#
# Budgets are structure: stage_shard_tool_allowed/2 shapes the per-shard
# tool list — permitted-minus-suppressed when budget is sufficient,
# preferred-only when constrained (mirrors final_injectable's all-vs-high).

Decl stage_required_family(Stage, Family) bound [/name, /name].
Decl stage_optional_family(Stage, Family) bound [/name, /name].
Decl stage_forbidden_family(Stage, Family) bound [/name, /name].
Decl stage_required_category(Stage, Category) bound [/name, /name].
Decl stage_forbidden_category(Stage, Category) bound [/name, /name].
# derived: explicitly admitted family (required, or optional and not vetoed).
Decl stage_permits_family(Stage, Family) bound [/name, /name].
# derived: explicitly admitted category (required; absence is neutral).
Decl stage_permits_category(Stage, Category) bound [/name, /name].
# derived: explicitly admitted capability (preferred; absence is neutral).
Decl stage_permits_capability(Stage, Capability) bound [/name, /name].
# derived: per-turn required/forbidden families for the current stage.
Decl stage_required_active(Stage, Family) bound [/name, /name].
Decl stage_forbidden_active(Stage, Family) bound [/name, /name].
# one guidance string per stage, admitted into injectable_context per shard.
Decl stage_guidance(Stage, Guidance) bound [/name, /string].
Decl stage_tool_preferred(Stage, Capability) bound [/name, /name].
Decl stage_tool_suppressed(Stage, Capability) bound [/name, /name].
# derived: stage-shaped tool views (parallel to relevant_tool; additive).
Decl stage_relevant_tool(ShardType, ToolName) bound [/name, /string].
Decl stage_suppressed_tool(ShardType, ToolName) bound [/name, /string].
# derived: budget-shaped per-shard tool list.
Decl stage_shard_tool_allowed(ShardID, ToolName) bound [/string, /string].

# --- /ideate: discovery, understanding, speculation ---
stage_required_family(/ideate, /trace_recall).
stage_required_family(/ideate, /specialist).
stage_optional_family(/ideate, /learning).
stage_optional_family(/ideate, /campaign).
stage_optional_family(/ideate, /exemplar).
stage_optional_family(/ideate, /intent_target).
stage_forbidden_family(/ideate, /callers).
stage_forbidden_family(/ideate, /covering_tests).
stage_forbidden_family(/ideate, /build_env).
stage_required_category(/ideate, /knowledge).
stage_required_category(/ideate, /domain).
stage_forbidden_category(/ideate, /reviewer).
stage_tool_preferred(/ideate, /knowledge).
stage_tool_preferred(/ideate, /analysis).
stage_tool_preferred(/ideate, /inspection).
stage_tool_suppressed(/ideate, /execution).
stage_tool_suppressed(/ideate, /validation).

# --- /design: architecture and design artifacts ---
stage_required_family(/design, /campaign).
stage_required_family(/design, /specialist).
stage_optional_family(/design, /trace_recall).
stage_optional_family(/design, /learning).
stage_optional_family(/design, /exemplar).
stage_optional_family(/design, /intent_target).
stage_forbidden_family(/design, /callers).
stage_forbidden_family(/design, /covering_tests).
stage_forbidden_family(/design, /build_env).
stage_required_category(/design, /domain).
stage_required_category(/design, /context).
stage_forbidden_category(/design, /reviewer).
stage_tool_preferred(/design, /analysis).
stage_tool_preferred(/design, /knowledge).
stage_tool_suppressed(/design, /execution).
stage_tool_suppressed(/design, /debugging).

# --- /plan: decomposition and orchestration ---
stage_required_family(/plan, /campaign).
stage_required_family(/plan, /trace_recall).
stage_optional_family(/plan, /learning).
stage_optional_family(/plan, /exemplar).
stage_optional_family(/plan, /intent_target).
stage_optional_family(/plan, /specialist).
stage_forbidden_family(/plan, /callers).
stage_forbidden_family(/plan, /covering_tests).
stage_forbidden_family(/plan, /build_env).
stage_required_category(/plan, /methodology).
stage_required_category(/plan, /context).
stage_forbidden_category(/plan, /reviewer).
stage_tool_preferred(/plan, /analysis).
stage_tool_suppressed(/plan, /execution).
stage_tool_suppressed(/plan, /generation).

# --- /implement: new functionality and file-mutating creation ---
# No forbidden rows: implementation admits the full slice.
stage_required_family(/implement, /intent_target).
stage_required_family(/implement, /callers).
stage_required_family(/implement, /covering_tests).
stage_required_family(/implement, /build_env).
stage_optional_family(/implement, /specialist).
stage_optional_family(/implement, /learning).
stage_optional_family(/implement, /exemplar).
stage_optional_family(/implement, /tool_desc).
stage_optional_family(/implement, /trace_recall).
stage_optional_family(/implement, /failure_recall).
stage_optional_family(/implement, /campaign).
stage_required_category(/implement, /methodology).
stage_required_category(/implement, /context).
stage_required_category(/implement, /domain).
stage_forbidden_category(/implement, /reviewer).
stage_tool_preferred(/implement, /generation).
stage_tool_preferred(/implement, /validation).
stage_tool_preferred(/implement, /execution).

# --- /verify: tests and measurement ---
# No forbidden rows: verification admits recall alongside test slices.
stage_required_family(/verify, /intent_target).
stage_required_family(/verify, /covering_tests).
stage_required_family(/verify, /build_env).
stage_optional_family(/verify, /failure_recall).
stage_optional_family(/verify, /trace_recall).
stage_optional_family(/verify, /tool_desc).
stage_optional_family(/verify, /specialist).
stage_optional_family(/verify, /learning).
stage_required_category(/verify, /context).
stage_required_category(/verify, /world_state).
stage_tool_preferred(/verify, /validation).
stage_tool_preferred(/verify, /execution).
stage_tool_preferred(/verify, /inspection).
stage_tool_suppressed(/verify, /generation).

# --- /harden: security hardening ---
# No forbidden rows: hardening admits code slices with recall.
stage_required_family(/harden, /intent_target).
stage_required_family(/harden, /specialist).
stage_required_family(/harden, /failure_recall).
stage_optional_family(/harden, /trace_recall).
stage_optional_family(/harden, /learning).
stage_optional_family(/harden, /campaign).
stage_optional_family(/harden, /callers).
stage_optional_family(/harden, /covering_tests).
stage_optional_family(/harden, /build_env).
stage_optional_family(/harden, /tool_desc).
stage_optional_family(/harden, /exemplar).
stage_required_category(/harden, /safety).
stage_required_category(/harden, /domain).
stage_required_category(/harden, /context).
stage_tool_preferred(/harden, /inspection).
stage_tool_preferred(/harden, /analysis).
stage_tool_preferred(/harden, /validation).
stage_tool_suppressed(/harden, /generation).

# --- /debug: bug fixing (the SWE-bench slice) ---
# No forbidden rows: debugging admits recall, code, and test slices.
stage_required_family(/debug, /intent_target).
stage_required_family(/debug, /failure_recall).
stage_required_family(/debug, /covering_tests).
stage_required_family(/debug, /callers).
stage_optional_family(/debug, /trace_recall).
stage_optional_family(/debug, /specialist).
stage_optional_family(/debug, /learning).
stage_optional_family(/debug, /build_env).
stage_optional_family(/debug, /tool_desc).
stage_optional_family(/debug, /exemplar).
stage_optional_family(/debug, /campaign).
stage_required_category(/debug, /methodology).
stage_required_category(/debug, /world_state).
stage_required_category(/debug, /context).
stage_tool_preferred(/debug, /debugging).
stage_tool_preferred(/debug, /inspection).
stage_tool_preferred(/debug, /validation).
stage_tool_preferred(/debug, /execution).

# --- /refactor: behaviour-preserving restructuring ---
# No forbidden rows: restructuring admits code and test slices with recall.
stage_required_family(/refactor, /intent_target).
stage_required_family(/refactor, /callers).
stage_required_family(/refactor, /covering_tests).
stage_optional_family(/refactor, /specialist).
stage_optional_family(/refactor, /trace_recall).
stage_optional_family(/refactor, /learning).
stage_optional_family(/refactor, /build_env).
stage_optional_family(/refactor, /tool_desc).
stage_optional_family(/refactor, /failure_recall).
stage_required_category(/refactor, /methodology).
stage_required_category(/refactor, /context).
stage_tool_preferred(/refactor, /transformation).
stage_tool_preferred(/refactor, /analysis).
stage_tool_preferred(/refactor, /validation).
stage_tool_suppressed(/refactor, /generation).

# --- /review: inspection and verdicts ---
# No forbidden rows: review admits code, test, and recall slices.
stage_required_family(/review, /intent_target).
stage_required_family(/review, /callers).
stage_required_family(/review, /covering_tests).
stage_optional_family(/review, /failure_recall).
stage_optional_family(/review, /trace_recall).
stage_optional_family(/review, /specialist).
stage_optional_family(/review, /learning).
stage_optional_family(/review, /campaign).
stage_required_category(/review, /reviewer).
stage_required_category(/review, /safety).
stage_tool_preferred(/review, /inspection).
stage_tool_preferred(/review, /analysis).
stage_tool_suppressed(/review, /generation).
stage_tool_suppressed(/review, /execution).

# --- Stage guidance: one atom per stage, admitted per active shard ---
stage_guidance(/ideate, "stage /ideate: prefer trace recall and specialist knowledge; code slices out of scope; inspect, do not execute").
stage_guidance(/design, "stage /design: prefer campaign constraints and specialist knowledge; code slices out of scope; analyze, do not execute").
stage_guidance(/plan, "stage /plan: prefer campaign constraints and trace recall; code slices out of scope; analyze, do not generate or execute").
stage_guidance(/implement, "stage /implement: require intent target, callers, covering tests, build env; generate and validate").
stage_guidance(/verify, "stage /verify: require covering tests and build env; validate and execute, do not generate").
stage_guidance(/harden, "stage /harden: require intent target, specialist knowledge, failure recall; inspect and validate, do not generate").
stage_guidance(/debug, "stage /debug: require intent target, failure recall, covering tests, callers; debug with world-state diagnostics").
stage_guidance(/refactor, "stage /refactor: require intent target, callers, covering tests; transform behavior-preserving, validate").
stage_guidance(/review, "stage /review: require intent target, callers, covering tests; inspect, do not generate or execute").

# --- Derivation: explicitly admitted families, categories, capabilities ---
stage_permits_family(Stage, Family) :-
    stage_required_family(Stage, Family).
stage_permits_family(Stage, Family) :-
    stage_optional_family(Stage, Family),
    !stage_forbidden_family(Stage, Family).
stage_permits_category(Stage, Category) :-
    stage_required_category(Stage, Category).
stage_permits_capability(Stage, Capability) :-
    stage_tool_preferred(Stage, Capability).

# --- Derivation: per-turn active policy for the current stage ---
stage_required_active(Stage, Family) :-
    task_stage(Stage),
    stage_required_family(Stage, Family).
stage_forbidden_active(Stage, Family) :-
    task_stage(Stage),
    stage_forbidden_family(Stage, Family).

# --- Bridge: stage guidance feeds the existing injectable_context ---
# Slot 1 is the concrete shard id (prompt_context style); both readers
# accept it alongside the "*" and "/_all" wildcards.
injectable_context(ShardID, Guidance) :-
    active_shard(ShardID, _),
    task_stage(Stage),
    stage_guidance(Stage, Guidance).

# --- Tool shaping: stage views over tool_capability (additive) ---
stage_relevant_tool(ShardType, ToolName) :-
    active_shard(_, ShardType),
    task_stage(Stage),
    tool_capability(ToolName, Capability),
    stage_permits_capability(Stage, Capability),
    tool_registered(ToolName, _).
stage_suppressed_tool(ShardType, ToolName) :-
    active_shard(_, ShardType),
    task_stage(Stage),
    tool_capability(ToolName, Capability),
    stage_tool_suppressed(Stage, Capability),
    tool_registered(ToolName, _).

# --- Tool shaping: budgets are structure (mirrors final_injectable) ---
# Sufficient budget: permitted capabilities minus suppressed vetoes.
# Negation is ground (Stage, Capability bound by stage_permits_capability).
stage_shard_tool_allowed(ShardID, ToolName) :-
    active_shard(ShardID, ShardType),
    context_budget_sufficient(ShardID),
    task_stage(Stage),
    tool_capability(ToolName, Capability),
    stage_permits_capability(Stage, Capability),
    tool_registered(ToolName, _),
    !stage_tool_suppressed(Stage, Capability).
# Constrained budget: preferred capabilities only.
stage_shard_tool_allowed(ShardID, ToolName) :-
    active_shard(ShardID, ShardType),
    context_budget_constrained(ShardID),
    task_stage(Stage),
    tool_capability(ToolName, Capability),
    stage_tool_preferred(Stage, Capability),
    tool_registered(ToolName, _).
