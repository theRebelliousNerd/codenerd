# Campaign Intelligence Logic
# Implements reasoning for intelligence, safety, and context prioritization.

# -----------------------------------------------------------------------------
# THE WORLD, READ LIVE
# -----------------------------------------------------------------------------
# These conclusions join the world model the scan already keeps (file_topology,
# file_line_count, dependency_link, churn_rate, test_file_for) instead of a copy
# the decomposer seeds, so they are true of the working tree at the moment of
# planning. Everything keyed by a campaign path is seeded from task_artifact:
# the planner asks about the files its plan touches, not the repository.

intelligence_campaign_path(Path) :-
    task_artifact(_, _, Path, _),
    file_topology(Path, _, _, _, _).

intelligence_file_topology(Path, Hash, Language, Lines, IsTest) :-
    file_topology(Path, Hash, Language, _, IsTest),
    file_line_count(Path, Lines).

intelligence_file_depends(File, DependsOn) :-
    dependency_link(File, DependsOn, _),
    file_topology(DependsOn, _, _, _, _).

# Churn from the repository history; the gatherer's hotspot list adds to it.
Decl intelligence_churn(Path, Churn) bound [/string, /number].
intelligence_churn(Path, Churn) :- churn_rate(Path, Churn).
intelligence_churn(Path, Churn) :- intelligence_churn_hotspot(Path, Churn, _).

# -----------------------------------------------------------------------------
# HIGH PRIORITY FILE DETECTION
# -----------------------------------------------------------------------------

intelligence_high_priority_file(Path) :-
    intelligence_campaign_path(Path),
    intelligence_churn(Path, Churn), Churn > 10.

intelligence_high_priority_file(Path) :-
    intelligence_campaign_path(Path),
    intelligence_file_action(Path, /refactor_first, _, _).

intelligence_high_priority_file(Path) :-
    intelligence_campaign_path(Path),
    intelligence_file_action(Path, /modularize, _, _).

intelligence_high_priority_file(Path) :-
    intelligence_campaign_path(Path),
    intelligence_safety_warning(_, Path, _, _, "critical").

# -----------------------------------------------------------------------------
# FILE MODIFICATION RECOMMENDATIONS
# -----------------------------------------------------------------------------

# What to do to a file before extending it, derived from what the scan measures.
intelligence_file_action(Path, /modularize, "the file is over 1000 lines", 90) :-
    intelligence_file_topology(Path, _, _, Lines, /false), Lines > 1000.

intelligence_file_action(Path, /refactor_first, "complex and churning", 80) :-
    complexity_warning(Path, _),
    intelligence_churn(Path, Churn), Churn > 5.

intelligence_requires_modularization(Path) :-
    intelligence_file_topology(Path, _, _, Lines, _), Lines > 1000.

intelligence_requires_modularization(Path) :-
    intelligence_file_topology(Path, _, _, Lines, _), Lines > 500,
    intelligence_churn(Path, Churn), Churn > 5.

# SCALE: every confidence/weight/coverage slot in schemas_intelligence.mg is
# declared /number (int64) on a 0-100 scale. Do NOT write float thresholds here
# (0.7, 0.5, ...): this Mangle fork's <,<=,>,>= accept int64 only, so a float
# operand makes EvalStratifiedProgram return an error, which aborts the entire
# kernel fixpoint — not just this rule. See TestCorpus_NoFloatLiteralInComparison.
intelligence_requires_refactor(Path) :-
    intelligence_file_action(Path, /refactor_first, _, Confidence), Confidence > 70.

# Type is a /string, and its value is the kernel predicate the pattern was read
# from: gatherCodePatterns sets CodePattern.Type to "anti_pattern", the one
# pattern predicate anything produces
# (internal/campaign/intelligence_gathering_methods.go) and seedIntelligenceFacts
# passes it through unmodified, so no leading slash ever reaches the kernel. The
# old /antipattern literal was wrong in both kind and spelling.
intelligence_requires_refactor(Path) :-
    intelligence_code_pattern(_, "anti_pattern", Path, Confidence), Confidence > 80.

# -----------------------------------------------------------------------------
# CHESTERTON'S FENCE (Understanding Before Modification)
# -----------------------------------------------------------------------------

intelligence_chestertons_fence(Path, "CAUTION: High churn rate - understand changes before modifying") :-
    intelligence_churn(Path, Churn), Churn > 10, Churn < 21.

intelligence_chestertons_fence(Path, "WARNING: Very high churn - requires careful analysis before changes") :-
    intelligence_churn(Path, Churn), Churn > 20.

# Core infrastructure detection - file has 3+ dependents
# Uses aggregation instead of O(N³) self-join
Decl intelligence_dependent_count(Path, Count) bound [/string, /number].
intelligence_dependent_count(Path, N) :-
    intelligence_file_depends(_, Path) |>
    do fn:group_by(Path),
    let N = fn:count().

intelligence_is_core_infrastructure(Path) :-
    intelligence_dependent_count(Path, N),
    N >= 5.

intelligence_chestertons_fence(Path, "CAUTION: Core infrastructure file - understand dependencies first") :-
    intelligence_is_core_infrastructure(Path).

# -----------------------------------------------------------------------------
# TEST COVERAGE ANALYSIS
# -----------------------------------------------------------------------------

# A campaign file has a test when the scan paired one with it or the gatherer
# measured real coverage; a test the plan itself writes counts too.
Decl intelligence_has_test(Path) bound [/string].
intelligence_has_test(Path) :- test_file_for(_, Path).
intelligence_has_test(Path) :- intelligence_well_tested(Path).
intelligence_has_test(Path) :- intelligence_test_coverage(Path, Coverage), Coverage >= 30, Coverage <= 70.

Decl intelligence_well_tested(Path) bound [/string].
intelligence_well_tested(Path) :-
    intelligence_test_coverage(Path, Coverage), Coverage > 70.
intelligence_has_test(Path) :-
    test_file_for(TestFile, Path),
    task_artifact(_, _, TestFile, _).

intelligence_missing_tests(Path) :-
    intelligence_campaign_path(Path),
    file_topology(Path, _, _, _, /false),
    !intelligence_has_test(Path).

intelligence_missing_tests(Path) :-
    intelligence_test_coverage(Path, Coverage), Coverage < 30.

# -----------------------------------------------------------------------------
# CAMPAIGN SAFETY & BLOCKING
# -----------------------------------------------------------------------------

intelligence_action_blocked(CampaignID, TaskID, Action) :-
    campaign_task(TaskID, PhaseID, _, _, _),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    task_artifact(TaskID, _, Path, _),
    intelligence_safety_warning(CampaignID, Path, Action, _, "critical").

intelligence_has_blocked_action(CampaignID) :- intelligence_action_blocked(CampaignID, _, _).

# -----------------------------------------------------------------------------
# ADVISORY BOARD CONSENSUS
# -----------------------------------------------------------------------------

intelligence_advisory_approved(CampaignID) :-
    intelligence_shard_advice(CampaignID, /coder, /approve, Conf1, _),
    intelligence_shard_advice(CampaignID, /tester, Vote2, Conf2, _),
    Vote2 != /reject,
    Conf1 > 50,
    Conf2 > 50.

intelligence_advisory_concerns(CampaignID, ShardName) :-
    intelligence_shard_advice(CampaignID, ShardName, /reject, _, _).

intelligence_advisory_concerns(CampaignID, ShardName) :-
    intelligence_shard_advice(CampaignID, ShardName, /conditional, Conf, _), Conf > 70.


# -----------------------------------------------------------------------------
# TOOL CAPABILITY ANALYSIS
# -----------------------------------------------------------------------------

intelligence_gap_resolved(CampaignID, Capability) :-
    intelligence_tool_gap(CampaignID, Capability, _, _, _),
    intelligence_mcp_tool(_, _, Capability, Affinity), Affinity > 50.

intelligence_gap_unresolved(CampaignID, Capability, Priority) :-
    intelligence_tool_gap(CampaignID, Capability, _, Priority, _),
    !intelligence_gap_resolved(CampaignID, Capability).

intelligence_blocking_gap(CampaignID, Capability) :-
    intelligence_gap_unresolved(CampaignID, Capability, Priority), Priority > 80.

intelligence_has_blocking_gap(CampaignID) :- intelligence_blocking_gap(CampaignID, _).

# -----------------------------------------------------------------------------
# IMPACT ANALYSIS
# -----------------------------------------------------------------------------

# A file others import is costly to change. The count is an aggregate over the
# live import edges, not a self-join.
intelligence_high_impact(Path) :-
    intelligence_dependent_count(Path, N),
    N >= 3.

# Who a change reaches, demand-seeded from the files the plan touches: the
# closure is computed for those files only, never for the whole import graph.
intelligence_impact_scope(Changed, Affected) :-
    intelligence_campaign_path(Changed),
    intelligence_file_depends(Affected, Changed).

intelligence_impact_scope(Changed, Affected) :-
    intelligence_impact_scope(Changed, Mid),
    intelligence_file_depends(Affected, Mid),
    Changed != Affected.

# -----------------------------------------------------------------------------
# CAMPAIGN READINESS
# -----------------------------------------------------------------------------

intelligence_campaign_ready(CampaignID) :-
    intelligence_advisory_approved(CampaignID),
    !intelligence_has_blocked_action(CampaignID),
    !intelligence_has_blocking_gap(CampaignID).

intelligence_campaign_blocked(CampaignID, "Advisory board rejected") :-
    intelligence_advisory_concerns(CampaignID, _),
    !intelligence_advisory_approved(CampaignID).

intelligence_campaign_blocked(CampaignID, "Unresolved high-priority tool gap") :-
    intelligence_has_blocking_gap(CampaignID).

intelligence_campaign_blocked(CampaignID, "Safety-blocked actions pending") :-
    intelligence_has_blocked_action(CampaignID).

# =============================================================================
# CONTEXT SELECTION PREDICATES
# =============================================================================
# What the context compiler includes while a campaign is planned or run: the
# files the plan touches, ranked by how dangerous they are to touch. The ranks
# reach the compiler through context_relevant (context_compilation.mg), which
# resolves each path to every fact that names it.

Decl intelligence_context_rank(Path, Rank) bound [/string, /number].
intelligence_context_rank(Path, 95) :-
    intelligence_campaign_path(Path),
    intelligence_safety_warning(_, Path, _, _, "critical").
intelligence_context_rank(Path, 85) :- intelligence_campaign_path(Path), intelligence_high_priority_file(Path).
intelligence_context_rank(Path, 85) :- intelligence_campaign_path(Path), intelligence_chestertons_fence(Path, _).
intelligence_context_rank(Path, 85) :- intelligence_campaign_path(Path), intelligence_high_impact(Path).

intelligence_context_priority(Path, /critical) :-
    intelligence_context_rank(Path, 95).
intelligence_context_priority(Path, /high) :-
    intelligence_context_rank(Path, 85).

# =============================================================================
# PLANNING CONSTRAINTS AND ADVISORIES
# =============================================================================
# The planner reads these in two ways. validation_error is the channel its
# refinement loop already reads: a plan that edits a volatile, widely imported
# file nothing tests is sent back to be fixed. intelligence_plan_advisory is
# what the user is shown about the plan: no verdict, but the warnings the
# planner's own context did not carry.
#
# These rules run where the world model lives, and the world shard holds only
# the plan's task_artifact rows, not the campaign's tasks and phases, so they
# speak about the plan the kernel currently holds (a task id, a path) and not
# about a campaign id.

Decl intelligence_task_path(TaskID, Path) bound [/string, /string].
intelligence_task_path(TaskID, Path) :-
    intelligence_campaign_path(Path),
    task_artifact(TaskID, _, Path, _).

intelligence_needs_tests_first(TaskID) :-
    intelligence_task_path(TaskID, Path),
    intelligence_missing_tests(Path),
    intelligence_high_impact(Path),
    intelligence_churn(Path, Churn), Churn > 15.

validation_error(TaskID, /needs_tests_first, Message) :-
    intelligence_needs_tests_first(TaskID),
    intelligence_task_path(TaskID, Path),
    intelligence_missing_tests(Path),
    Message = fn:string:concat("task ", TaskID, " edits ", Path, ", which many files import and nothing tests; add a test task for it first").

validation_error(PhaseID, /safety_blocked, Message) :-
    intelligence_action_blocked(_, TaskID, Action),
    campaign_task(TaskID, PhaseID, _, _, _),
    Message = fn:string:concat("task ", TaskID, " would ", Action, " a path under a critical safety warning").

Decl intelligence_plan_advisory(Kind, Path, Message) bound [/name, /string, /string].
intelligence_plan_advisory(/modularize_first, Path, "the file is large and volatile: split it before adding to it") :-
    intelligence_campaign_path(Path),
    intelligence_requires_modularization(Path).
intelligence_plan_advisory(/fence, Path, Warning) :-
    intelligence_campaign_path(Path),
    intelligence_chestertons_fence(Path, Warning).
intelligence_plan_advisory(/high_impact, Path, "many files import it: a change here reaches them") :-
    intelligence_campaign_path(Path),
    intelligence_high_impact(Path).

# =============================================================================
# STRATEGIC KNOWLEDGE INTEGRATION
# =============================================================================

# Helper: extract active campaign IDs from task/phase chain (avoids cross-product)
Decl active_campaign_id(CampaignID).
active_campaign_id(CampaignID) :-
    campaign_task(_, PhaseID, _, _, _),
    campaign_phase(PhaseID, CampaignID, _, _, _, _).

# Only select high-priority patterns
# Accepted bounded cross-product: CampaignID, Concept, Content all appear in head.
# The knowledge × campaigns join is semantically required. active_campaign_id helper
# pre-computes the campaign_task->campaign_phase chain to avoid deeper cross-products.
intelligence_relevant_strategy(CampaignID, Concept, Content) :-
    intelligence_strategic_knowledge(Concept, /pattern, Content, Priority),
    Priority > 50,
    active_campaign_id(CampaignID).

# Always include vision for active campaigns (same accepted bounded cross-product)
intelligence_relevant_strategy(CampaignID, Concept, Content) :-
    intelligence_strategic_knowledge(Concept, /vision, Content, _),
    active_campaign_id(CampaignID).

intelligence_knowledge_path(Start, End, Relation) :-
    intelligence_knowledge_link(Start, Relation, End, Weight), Weight > 50.

intelligence_knowledge_path(Start, End, "transitive") :-
    intelligence_knowledge_link(Start, _, Intermediate, W1),
    intelligence_knowledge_link(Intermediate, _, End, W2),
    Start != End,
    W1 > 50,
    W2 > 50.

# What the knowledge graph links to the plan's files: the plan's artifacts are
# the seeds, so the closure is never computed for the whole graph.
Decl intelligence_plan_related(Path, Relation, Entity) bound [/string, /string, /string].
intelligence_plan_related(Path, Relation, Entity) :-
    task_artifact(_, _, Path, _),
    intelligence_knowledge_path(Path, Entity, Relation).

# =============================================================================
# LEARNING PATTERN APPLICATION
# =============================================================================

intelligence_applicable_pattern(ShardType, Pattern) :-
    intelligence_learning_pattern(ShardType, Pattern, Confidence),
    Confidence > 70.

intelligence_active_preference(Category, Signal) :-
    intelligence_preference(Category, Signal, Strength),
    Strength > 60.

# =============================================================================
# REPORTING PREDICATES (Aggregation with correct syntax)
# =============================================================================

intelligence_summary_churn_count(Count) :-
    intelligence_churn_hotspot(_, _, _) |>
    do fn:group_by(),
    let Count = fn:count().

intelligence_summary_gap_count(Count) :-
    intelligence_tool_gap(_, _, _, _, _) |>
    do fn:group_by(),
    let Count = fn:count().

intelligence_summary_warning_count(Count) :-
    intelligence_safety_warning(_, _, _, _, _) |>
    do fn:group_by(),
    let Count = fn:count().

intelligence_warnings_by_severity(Severity, Count) :-
    intelligence_safety_warning(_, _, _, _, Severity) |>
    do fn:group_by(Severity),
    let Count = fn:count().

intelligence_high_priority_count(Count) :-
    intelligence_high_priority_file(_) |>
    do fn:group_by(),
    let Count = fn:count().

intelligence_modularization_count(Count) :-
    intelligence_requires_modularization(_) |>
    do fn:group_by(),
    let Count = fn:count().

# =============================================================================
# PLAN METRICS
# =============================================================================
# Complexity is the blast radius: how many files the plan's changes touch or
# reach through the import graph (the demand-seeded closure above). Risk is the
# worst hazard on any file the plan touches. Each is ONE aggregation (count,
# max) and a threshold, not a stack of exclusion helpers saying "no higher level
# exists".

Decl intelligence_blast_member(Path) bound [/string].
intelligence_blast_member(Path) :- intelligence_campaign_path(Path).
intelligence_blast_member(Affected) :- intelligence_impact_scope(_, Affected).

Decl intelligence_blast_radius(Files) bound [/number].
intelligence_blast_radius(Files) :-
    intelligence_blast_member(Path)
    |> do fn:group_by(), let Files = fn:count().

intelligence_plan_complexity(/high) :- intelligence_blast_radius(Files), Files > 25.
intelligence_plan_complexity(/medium) :- intelligence_blast_radius(Files), Files > 5, Files <= 25.
intelligence_plan_complexity(/low) :- intelligence_blast_radius(Files), Files <= 5.

# One weight per hazard; the plan's risk is the heaviest hazard on any file it
# touches. The base row says a file the world model knows has hazard 0 at worst,
# so "low" is derived from a measurement (the heaviest hazard is light), never
# from the absence of rows the kernel may not have been able to compute.
Decl intelligence_hazard(Path, Weight, Reason) bound [/string, /number, /string].
intelligence_hazard(Path, 0, "no hazard found") :-
    intelligence_campaign_path(Path).
intelligence_hazard(Path, 100, "a critical safety warning applies") :-
    intelligence_campaign_path(Path),
    intelligence_safety_warning(_, Path, _, _, "critical").
intelligence_hazard(Path, 60, "modifying high-churn core infrastructure") :-
    intelligence_campaign_path(Path),
    intelligence_churn(Path, Churn), Churn > 15,
    intelligence_high_impact(Path).
intelligence_hazard(Path, 30, "a file that should be modularized first") :-
    intelligence_campaign_path(Path),
    intelligence_requires_modularization(Path).
intelligence_hazard(Path, 20, "low test coverage on a modified file") :-
    intelligence_campaign_path(Path),
    intelligence_missing_tests(Path).

Decl intelligence_plan_hazard(Weight) bound [/number].
intelligence_plan_hazard(Weight) :-
    intelligence_hazard(Path, W, Reason)
    |> do fn:group_by(), let Weight = fn:max(W).

intelligence_plan_risk(/high, Reason) :-
    intelligence_plan_hazard(Weight), Weight >= 60,
    intelligence_hazard(_, W, Reason), W >= 60.
intelligence_plan_risk(/medium, Reason) :-
    intelligence_plan_hazard(Weight), Weight >= 20, Weight < 60,
    intelligence_hazard(_, W, Reason), W >= 20.
intelligence_plan_risk(/low, "no hazard of weight") :-
    intelligence_plan_hazard(Weight), Weight < 20.
