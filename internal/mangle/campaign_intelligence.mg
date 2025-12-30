# Campaign Intelligence Predicates
# =================================
# This file defines Mangle predicates for the intelligence integration system.
# These predicates enable the kernel to reason about:
# - World model facts (file topology, symbols)
# - Git history and churn analysis (Chesterton's Fence)
# - Learning patterns from previous sessions
# - Safety warnings and blocked actions
# - Tool gaps and capabilities
# - MCP tool availability
# - Shard advisory recommendations
# - Strategic knowledge integration

# =============================================================================
# INTELLIGENCE FACTS (EDB - Extensional Database)
# =============================================================================

# World Model Facts
Decl intelligence_world_fact(CampaignID, Path, Predicate, Priority).

# File topology from world scanner
Decl intelligence_file_topology(Path, Hash, Language, LineCount, IsTestFile).

# Symbol graph from AST parsing
Decl intelligence_symbol(Path, Name, Kind, Exported, Line).

# Git Churn Hotspots (Chesterton's Fence)
Decl intelligence_churn_hotspot(Path, ChurnRate, Reason).

# Recent git commits
Decl intelligence_git_commit(Hash, Author, Timestamp, Message).

# Historical Learning Patterns
Decl intelligence_learning_pattern(ShardType, Predicate, Confidence).

# Preference signals from cold storage
Decl intelligence_preference(Category, Signal, Strength).

# Safety Warnings
Decl intelligence_safety_warning(CampaignID, Path, Action, RuleViolated, Severity).

# Blocked Actions
Decl intelligence_blocked_action(CampaignID, Action, Reason).

# Tool Gaps
Decl intelligence_tool_gap(CampaignID, Capability, RequiredBy, Priority, Confidence).

# MCP Tool Availability
Decl intelligence_mcp_tool(ToolID, ServerID, Name, Affinity).

# MCP Server Status
Decl intelligence_mcp_server(ServerID, Endpoint, Status).

# Shard Advisory
Decl intelligence_shard_advice(CampaignID, ShardName, Vote, Confidence, Advice).

# Edge Case Analysis
Decl intelligence_file_action(Path, RecommendedAction, Reasoning, Confidence).

# File Dependencies
Decl intelligence_file_depends(File, DependsOn).

# Test Coverage
Decl intelligence_test_coverage(Path, Coverage).

# Code Patterns
Decl intelligence_code_pattern(Name, Type, File, Confidence).

# Previous Campaigns
Decl intelligence_previous_campaign(CampaignID, Goal, TaskCount, SuccessRate).

# Strategic Knowledge (from init/strategic_knowledge.go)
Decl intelligence_strategic_knowledge(Concept, Category, Content, Priority).

# Knowledge Graph Links
Decl intelligence_knowledge_link(SourceEntity, Relation, TargetEntity, Weight).

# =============================================================================
# EXTERNAL PREDICATE DECLARATIONS (from other .mg files)
# =============================================================================

# Campaign tasks (from campaign.mg or similar)
Decl campaign_task(CampaignID, PhaseID, TaskID, Name, Status, Priority, Order).

# Task artifacts (from campaign.mg or similar)
Decl task_artifact(TaskID, ArtifactType, Path).

# User intent (from schemas or perception)
Decl user_intent(SessionID, Category, Verb, Target, Constraint).

# =============================================================================
# DERIVED PREDICATES (IDB - Intensional Database)
# =============================================================================

# -----------------------------------------------------------------------------
# HIGH PRIORITY FILE DETECTION
# -----------------------------------------------------------------------------

Decl intelligence_high_priority_file(Path).

intelligence_high_priority_file(Path) :-
    intelligence_churn_hotspot(Path, Churn, _), Churn > 10.

intelligence_high_priority_file(Path) :-
    intelligence_file_action(Path, /refactor_first, _, _).

intelligence_high_priority_file(Path) :-
    intelligence_file_action(Path, /modularize, _, _).

intelligence_high_priority_file(Path) :-
    intelligence_safety_warning(_, Path, _, _, /critical).

# -----------------------------------------------------------------------------
# FILE MODIFICATION RECOMMENDATIONS
# -----------------------------------------------------------------------------

Decl intelligence_requires_modularization(Path).

intelligence_requires_modularization(Path) :-
    intelligence_file_topology(Path, _, _, Lines, _), Lines > 1000.

intelligence_requires_modularization(Path) :-
    intelligence_file_topology(Path, _, _, Lines, _), Lines > 500,
    intelligence_churn_hotspot(Path, Churn, _), Churn > 5.

Decl intelligence_requires_refactor(Path).

intelligence_requires_refactor(Path) :-
    intelligence_file_action(Path, /refactor_first, _, Confidence), Confidence > 0.7.

intelligence_requires_refactor(Path) :-
    intelligence_code_pattern(_, /antipattern, Path, Confidence), Confidence > 0.8.

# -----------------------------------------------------------------------------
# CHESTERTON'S FENCE (Understanding Before Modification)
# -----------------------------------------------------------------------------

Decl intelligence_chestertons_fence(Path, Warning).

intelligence_chestertons_fence(Path, "CAUTION: High churn rate - understand changes before modifying") :-
    intelligence_churn_hotspot(Path, Churn, _), Churn > 10, Churn < 21.

intelligence_chestertons_fence(Path, "WARNING: Very high churn - requires careful analysis before changes") :-
    intelligence_churn_hotspot(Path, Churn, _), Churn > 20.

# Core infrastructure detection - file has 3+ dependents
Decl intelligence_is_core_infrastructure(Path).

intelligence_is_core_infrastructure(Path) :-
    intelligence_file_depends(D1, Path),
    intelligence_file_depends(D2, Path),
    intelligence_file_depends(D3, Path),
    D1 != D2, D2 != D3, D1 != D3.

intelligence_chestertons_fence(Path, "CAUTION: Core infrastructure file - understand dependencies first") :-
    intelligence_is_core_infrastructure(Path).

# -----------------------------------------------------------------------------
# TEST COVERAGE ANALYSIS
# -----------------------------------------------------------------------------

Decl intelligence_missing_tests(Path).

intelligence_missing_tests(Path) :-
    intelligence_file_topology(Path, _, _, _, /false),
    !intelligence_test_coverage(Path, _).

intelligence_missing_tests(Path) :-
    intelligence_test_coverage(Path, Coverage), Coverage < 0.3.

Decl intelligence_well_tested(Path).

intelligence_well_tested(Path) :-
    intelligence_test_coverage(Path, Coverage), Coverage > 0.7.

# -----------------------------------------------------------------------------
# CAMPAIGN SAFETY & BLOCKING
# -----------------------------------------------------------------------------

Decl intelligence_action_blocked(CampaignID, TaskID, Action).

intelligence_action_blocked(CampaignID, TaskID, Action) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_safety_warning(CampaignID, Path, Action, _, /critical).

intelligence_action_blocked(CampaignID, TaskID, /delete) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_high_impact(Path).

# -----------------------------------------------------------------------------
# ADVISORY BOARD CONSENSUS
# -----------------------------------------------------------------------------

Decl intelligence_advisory_approved(CampaignID).

intelligence_advisory_approved(CampaignID) :-
    intelligence_shard_advice(CampaignID, /coder, /approve, Conf1, _),
    intelligence_shard_advice(CampaignID, /tester, Vote2, Conf2, _),
    Vote2 != /reject,
    Conf1 > 0.5,
    Conf2 > 0.5.

Decl intelligence_advisory_concerns(CampaignID, ShardName).

intelligence_advisory_concerns(CampaignID, ShardName) :-
    intelligence_shard_advice(CampaignID, ShardName, /reject, _, _).

intelligence_advisory_concerns(CampaignID, ShardName) :-
    intelligence_shard_advice(CampaignID, ShardName, /conditional, Conf, _), Conf > 0.7.


# -----------------------------------------------------------------------------
# TOOL CAPABILITY ANALYSIS
# -----------------------------------------------------------------------------

Decl intelligence_gap_resolved(CampaignID, Capability).

intelligence_gap_resolved(CampaignID, Capability) :-
    intelligence_tool_gap(CampaignID, Capability, _, _, _),
    intelligence_mcp_tool(_, _, Capability, Affinity), Affinity > 0.5.

Decl intelligence_gap_unresolved(CampaignID, Capability, Priority).

intelligence_gap_unresolved(CampaignID, Capability, Priority) :-
    intelligence_tool_gap(CampaignID, Capability, _, Priority, _),
    !intelligence_gap_resolved(CampaignID, Capability).

Decl intelligence_blocking_gap(CampaignID, Capability).

intelligence_blocking_gap(CampaignID, Capability) :-
    intelligence_gap_unresolved(CampaignID, Capability, Priority), Priority > 80.

# -----------------------------------------------------------------------------
# IMPACT ANALYSIS
# -----------------------------------------------------------------------------

Decl intelligence_high_impact(Path).

intelligence_high_impact(Path) :-
    intelligence_file_depends(Dependent, Path),
    intelligence_file_depends(Dependent2, Path),
    Dependent != Dependent2.

Decl intelligence_depends_transitive(File, DependsOn).

intelligence_depends_transitive(File, DependsOn) :-
    intelligence_file_depends(File, DependsOn).

intelligence_depends_transitive(File, TransitiveDep) :-
    intelligence_file_depends(File, Intermediate),
    intelligence_depends_transitive(Intermediate, TransitiveDep).

Decl intelligence_impact_scope(ChangedFile, AffectedFile).

intelligence_impact_scope(ChangedFile, AffectedFile) :-
    intelligence_depends_transitive(AffectedFile, ChangedFile).

# -----------------------------------------------------------------------------
# CAMPAIGN READINESS
# -----------------------------------------------------------------------------

Decl intelligence_campaign_ready(CampaignID).

intelligence_campaign_ready(CampaignID) :-
    intelligence_advisory_approved(CampaignID),
    !intelligence_action_blocked(CampaignID, _, _),
    !intelligence_blocking_gap(CampaignID, _).

Decl intelligence_campaign_blocked(CampaignID, Reason).

intelligence_campaign_blocked(CampaignID, "Advisory board rejected") :-
    intelligence_advisory_concerns(CampaignID, _),
    !intelligence_advisory_approved(CampaignID).

intelligence_campaign_blocked(CampaignID, "Unresolved high-priority tool gap") :-
    intelligence_blocking_gap(CampaignID, _).

intelligence_campaign_blocked(CampaignID, "Safety-blocked actions pending") :-
    intelligence_action_blocked(CampaignID, _, _).

# =============================================================================
# CONTEXT SELECTION PREDICATES
# =============================================================================

# To avoid stratification issues, we define exclusion predicates based on
# SOURCE facts only, not on the derived priority predicate.

Decl intelligence_context_priority(Path, Priority).

# Critical: safety warnings with critical severity
intelligence_context_priority(Path, /critical) :-
    intelligence_safety_warning(_, Path, _, _, /critical).

# Exclusion: has critical-level source fact
Decl intelligence_has_critical_source(Path).
intelligence_has_critical_source(Path) :- intelligence_safety_warning(_, Path, _, _, /critical).

# High: high-priority file, chesterton's fence, or high impact (but not critical)
intelligence_context_priority(Path, /high) :-
    intelligence_high_priority_file(Path),
    !intelligence_has_critical_source(Path).

intelligence_context_priority(Path, /high) :-
    intelligence_chestertons_fence(Path, _),
    !intelligence_has_critical_source(Path).

intelligence_context_priority(Path, /high) :-
    intelligence_high_impact(Path),
    !intelligence_has_critical_source(Path).

# Exclusion: has high-level source facts (for medium/low exclusion)
Decl intelligence_has_high_source(Path).
intelligence_has_high_source(Path) :- intelligence_has_critical_source(Path).
intelligence_has_high_source(Path) :- intelligence_high_priority_file(Path).
intelligence_has_high_source(Path) :- intelligence_chestertons_fence(Path, _).
intelligence_has_high_source(Path) :- intelligence_high_impact(Path).

# Medium: extend action (but not high or critical)
intelligence_context_priority(Path, /medium) :-
    intelligence_file_action(Path, /extend, _, _),
    !intelligence_has_high_source(Path).

# Exclusion: has medium-level source facts (for low exclusion)
Decl intelligence_has_medium_source(Path).
intelligence_has_medium_source(Path) :- intelligence_has_high_source(Path).
intelligence_has_medium_source(Path) :- intelligence_file_action(Path, /extend, _, _).

# Low: has file topology but no higher priority
intelligence_context_priority(Path, /low) :-
    intelligence_file_topology(Path, _, _, _, _),
    !intelligence_has_medium_source(Path).

# =============================================================================
# PLANNING CONSTRAINTS
# =============================================================================

Decl intelligence_task_prerequisite(PreTask, PostTask).

intelligence_task_prerequisite(PreTask, PostTask) :-
    task_artifact(PreTask, _, Path),
    task_artifact(PostTask, _, Path),
    PreTask != PostTask,
    intelligence_file_action(Path, /modularize, _, _).

intelligence_task_prerequisite(ModularizeTask, ExtendTask) :-
    task_artifact(ModularizeTask, /modularize, Path),
    task_artifact(ExtendTask, /extend, Path).

Decl intelligence_needs_tests_first(TaskID).

intelligence_needs_tests_first(TaskID) :-
    task_artifact(TaskID, _, Path),
    intelligence_missing_tests(Path),
    intelligence_high_impact(Path).

# =============================================================================
# STRATEGIC KNOWLEDGE INTEGRATION
# =============================================================================

Decl intelligence_relevant_strategy(CampaignID, Concept, Content).

# Only select high-priority patterns
intelligence_relevant_strategy(CampaignID, Concept, Content) :-
    intelligence_strategic_knowledge(Concept, /pattern, Content, Priority),
    Priority > 50,
    campaign_task(CampaignID, _, _, _, _, _, _).

# Always include vision for active campaigns
intelligence_relevant_strategy(CampaignID, Concept, Content) :-
    intelligence_strategic_knowledge(Concept, /vision, Content, _),
    campaign_task(CampaignID, _, _, _, _, _, _).

Decl intelligence_knowledge_path(Start, End, Relation).

intelligence_knowledge_path(Start, End, Relation) :-
    intelligence_knowledge_link(Start, Relation, End, Weight), Weight > 0.5.

intelligence_knowledge_path(Start, End, /transitive) :-
    intelligence_knowledge_link(Start, _, Intermediate, W1),
    intelligence_knowledge_link(Intermediate, _, End, W2),
    Start != End,
    W1 > 0.5,
    W2 > 0.5.

# =============================================================================
# LEARNING PATTERN APPLICATION
# =============================================================================

Decl intelligence_applicable_pattern(ShardType, Pattern).

intelligence_applicable_pattern(ShardType, Pattern) :-
    intelligence_learning_pattern(ShardType, Pattern, Confidence),
    Confidence > 0.7.

Decl intelligence_active_preference(Category, Signal).

intelligence_active_preference(Category, Signal) :-
    intelligence_preference(Category, Signal, Strength),
    Strength > 0.6.

# =============================================================================
# REPORTING PREDICATES (Aggregation with correct syntax)
# =============================================================================

Decl intelligence_summary_churn_count(Count).

intelligence_summary_churn_count(Count) :-
    intelligence_churn_hotspot(_, _, _) |>
    do fn:group_by(),
    let Count = fn:count().

Decl intelligence_summary_gap_count(Count).

intelligence_summary_gap_count(Count) :-
    intelligence_tool_gap(_, _, _, _, _) |>
    do fn:group_by(),
    let Count = fn:count().

Decl intelligence_summary_warning_count(Count).

intelligence_summary_warning_count(Count) :-
    intelligence_safety_warning(_, _, _, _, _) |>
    do fn:group_by(),
    let Count = fn:count().

Decl intelligence_warnings_by_severity(Severity, Count).

intelligence_warnings_by_severity(Severity, Count) :-
    intelligence_safety_warning(_, _, _, _, Severity) |>
    do fn:group_by(Severity),
    let Count = fn:count().

Decl intelligence_high_priority_count(Count).

intelligence_high_priority_count(Count) :-
    intelligence_high_priority_file(_) |>
    do fn:group_by(),
    let Count = fn:count().

Decl intelligence_modularization_count(Count).

intelligence_modularization_count(Count) :-
    intelligence_requires_modularization(_) |>
    do fn:group_by(),
    let Count = fn:count().

# =============================================================================
# CAMPAIGN METRICS
# =============================================================================

# To avoid stratification issues, we define source-based exclusion predicates.

Decl intelligence_campaign_complexity(CampaignID, Score).

# High complexity if high-priority AND high-impact
intelligence_campaign_complexity(CampaignID, /high) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_high_priority_file(Path),
    intelligence_high_impact(Path).

# Exclusion: campaign has high-complexity source conditions
Decl intelligence_has_high_complexity_source(CampaignID).
intelligence_has_high_complexity_source(CampaignID) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_high_priority_file(Path),
    intelligence_high_impact(Path).

# Medium complexity if churn > 5 (but not high)
intelligence_campaign_complexity(CampaignID, /medium) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_churn_hotspot(Path, Churn, _), Churn > 5,
    !intelligence_has_high_complexity_source(CampaignID).

# Exclusion: campaign has medium-or-above complexity source
Decl intelligence_has_medium_complexity_source(CampaignID).
intelligence_has_medium_complexity_source(CampaignID) :- intelligence_has_high_complexity_source(CampaignID).
intelligence_has_medium_complexity_source(CampaignID) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_churn_hotspot(Path, Churn, _), Churn > 5.

# Low complexity for everything else
intelligence_campaign_complexity(CampaignID, /low) :-
    campaign_task(CampaignID, _, _, _, _, _, _),
    !intelligence_has_medium_complexity_source(CampaignID).

# Risk assessment
Decl intelligence_campaign_risk(CampaignID, RiskLevel, Reason).

intelligence_campaign_risk(CampaignID, /high, "Modifying high-churn core infrastructure") :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_churn_hotspot(Path, Churn, _), Churn > 15,
    intelligence_high_impact(Path).

# Exclusion: campaign has high-risk source conditions
Decl intelligence_has_high_risk_source(CampaignID).
intelligence_has_high_risk_source(CampaignID) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_churn_hotspot(Path, Churn, _), Churn > 15,
    intelligence_high_impact(Path).

intelligence_campaign_risk(CampaignID, /medium, "Low test coverage on modified files") :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_missing_tests(Path),
    !intelligence_has_high_risk_source(CampaignID).

# Exclusion: campaign has medium-or-above risk source
Decl intelligence_has_medium_risk_source(CampaignID).
intelligence_has_medium_risk_source(CampaignID) :- intelligence_has_high_risk_source(CampaignID).
intelligence_has_medium_risk_source(CampaignID) :-
    campaign_task(CampaignID, _, TaskID, _, _, _, _),
    task_artifact(TaskID, _, Path),
    intelligence_missing_tests(Path).

intelligence_campaign_risk(CampaignID, /low, "Standard modifications with good coverage") :-
    campaign_task(CampaignID, _, _, _, _, _, _),
    !intelligence_has_medium_risk_source(CampaignID).
