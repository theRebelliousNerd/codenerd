# Cortex 1.5.0 Schemas (EDB Declarations)
# Modular Schema: INTELLIGENCE
# Sections: Campaign Intelligence

# =============================================================================
# CAMPAIGN INTELLIGENCE (Pre-planning context from 12 systems)
# =============================================================================

# intelligence_world_fact(CampaignID, Predicate, Args)
# Generic fact from the world model scanner (wrapped)
Decl intelligence_world_fact(CampaignID, Predicate, Args) bound [/string, /string, /any].

# intelligence_file_topology(Path, Hash, Language, LineCount, IsTestFile)
# File topology from world scanner
Decl intelligence_file_topology(Path, Hash, Language, LineCount, IsTestFile) bound [/string, /string, /string, /number, /name].

# intelligence_symbol(Path, Name, Kind, Exported, Line)
# Symbol graph from AST parsing
Decl intelligence_symbol(Path, Name, Kind, Exported, Line) bound [/string, /string, /string, /name, /number].

# intelligence_churn_hotspot(Path, ChurnRate, Reason)
# Git Churn Hotspots (Chesterton's Fence)
Decl intelligence_churn_hotspot(Path, ChurnRate, Reason) bound [/string, /number, /string].

# intelligence_git_commit(Hash, Author, Timestamp, Message)
# Recent git commits
Decl intelligence_git_commit(Hash, Author, Timestamp, Message) bound [/string, /string, /number, /string].

# intelligence_learning_pattern(ShardType, Predicate, Confidence)
# Historical Learning Patterns
Decl intelligence_learning_pattern(ShardType, Predicate, Confidence) bound [/string, /string, /number].

# intelligence_preference(Category, Signal, Strength)
# Preference signals from cold storage
Decl intelligence_preference(Category, Signal, Strength) bound [/string, /string, /number].

# intelligence_safety_warning(CampaignID, Path, Action, RuleViolated, Severity)
# Safety Warnings from constitutional gate
Decl intelligence_safety_warning(CampaignID, Path, Action, RuleViolated, Severity) bound [/string, /string, /string, /string, /string].

# intelligence_blocked_action(CampaignID, Action, Reason)
# Blocked Actions
Decl intelligence_blocked_action(CampaignID, Action, Reason) bound [/string, /string, /string].

# intelligence_tool_gap(CampaignID, Capability, RequiredBy, Priority, Confidence)
# Tool Gaps
Decl intelligence_tool_gap(CampaignID, Capability, RequiredBy, Priority, Confidence) bound [/string, /string, /string, /number, /number].

# intelligence_mcp_tool(ToolID, ServerID, Name, Affinity)
# MCP Tool Availability
Decl intelligence_mcp_tool(ToolID, ServerID, Name, Affinity) bound [/string, /string, /string, /number].

# intelligence_mcp_server(ServerID, Endpoint, Status)
# MCP Server Status
Decl intelligence_mcp_server(ServerID, Endpoint, Status) bound [/string, /string, /string].

# intelligence_shard_advice(CampaignID, ShardName, Vote, Confidence, Advice)
# Shard Advisory
Decl intelligence_shard_advice(CampaignID, ShardName, Vote, Confidence, Advice) bound [/string, /string, /string, /number, /string].

# intelligence_file_action(Path, RecommendedAction, Reasoning, Confidence)
# Edge Case Analysis
Decl intelligence_file_action(Path, RecommendedAction, Reasoning, Confidence) bound [/string, /name, /string, /number].

# intelligence_file_depends(File, DependsOn)
# File Dependencies
Decl intelligence_file_depends(File, DependsOn) bound [/string, /string].

# intelligence_test_coverage(Path, Coverage)
# Test Coverage
Decl intelligence_test_coverage(Path, Coverage) bound [/string, /number].

# intelligence_code_pattern(Name, Type, File, Confidence)
# Code Patterns
Decl intelligence_code_pattern(Name, Type, File, Confidence) bound [/string, /string, /string, /number].

# intelligence_previous_campaign(CampaignID, Goal, TaskCount, SuccessRate)
# Previous Campaigns
Decl intelligence_previous_campaign(CampaignID, Goal, TaskCount, SuccessRate) bound [/string, /string, /number, /number].

# intelligence_strategic_knowledge(Concept, Category, Content, Priority)
# Strategic Knowledge
Decl intelligence_strategic_knowledge(Concept, Category, Content, Priority) bound [/string, /name, /string, /number].

# intelligence_knowledge_link(SourceEntity, Relation, TargetEntity, Weight)
# Knowledge Graph Links
Decl intelligence_knowledge_link(SourceEntity, Relation, TargetEntity, Weight) bound [/string, /string, /string, /number].

# =============================================================================
# DERIVED PREDICATES (Declarations only)
# =============================================================================

Decl intelligence_high_priority_file(Path) bound [/string].
Decl intelligence_requires_modularization(Path) bound [/string].
Decl intelligence_requires_refactor(Path) bound [/string].
Decl intelligence_chestertons_fence(Path, Warning) bound [/string, /string].
Decl intelligence_is_core_infrastructure(Path) bound [/string].
Decl intelligence_has_coverage(Path) bound [/string].
Decl intelligence_missing_tests(Path) bound [/string].
Decl intelligence_well_tested(Path) bound [/string].
Decl intelligence_action_blocked(CampaignID, TaskID, Action) bound [/string, /string, /name].
Decl intelligence_has_blocked_action(CampaignID) bound [/string].
Decl intelligence_advisory_approved(CampaignID) bound [/string].
Decl intelligence_advisory_concerns(CampaignID, ShardName) bound [/string, /string].
Decl intelligence_gap_resolved(CampaignID, Capability) bound [/string, /string].
Decl intelligence_gap_unresolved(CampaignID, Capability, Priority) bound [/string, /string, /number].
Decl intelligence_blocking_gap(CampaignID, Capability) bound [/string, /string].
Decl intelligence_has_blocking_gap(CampaignID) bound [/string].
Decl intelligence_high_impact(Path) bound [/string].
Decl intelligence_depends_transitive(File, DependsOn) bound [/string, /string].
Decl intelligence_impact_scope(ChangedFile, AffectedFile) bound [/string, /string].
Decl intelligence_campaign_ready(CampaignID) bound [/string].
Decl intelligence_campaign_blocked(CampaignID, Reason) bound [/string, /string].
Decl intelligence_context_priority(Path, Priority) bound [/string, /name].
Decl intelligence_has_critical_source(Path) bound [/string].
Decl intelligence_has_high_source(Path) bound [/string].
Decl intelligence_has_medium_source(Path) bound [/string].
Decl intelligence_task_prerequisite(PreTask, PostTask) bound [/string, /string].
Decl intelligence_needs_tests_first(TaskID) bound [/string].
Decl intelligence_relevant_strategy(CampaignID, Concept, Content) bound [/string, /string, /string].
Decl intelligence_knowledge_path(Start, End, Relation) bound [/string, /string, /name].
Decl intelligence_applicable_pattern(ShardType, Pattern) bound [/string, /string].
Decl intelligence_active_preference(Category, Signal) bound [/string, /string].
Decl intelligence_summary_churn_count(Count) bound [/number].
Decl intelligence_summary_gap_count(Count) bound [/number].
Decl intelligence_summary_warning_count(Count) bound [/number].
Decl intelligence_warnings_by_severity(Severity, Count) bound [/string, /number].
Decl intelligence_high_priority_count(Count) bound [/number].
Decl intelligence_modularization_count(Count) bound [/number].
Decl intelligence_campaign_complexity(CampaignID, Score) bound [/string, /name].
Decl intelligence_has_high_complexity_source(CampaignID) bound [/string].
Decl intelligence_has_medium_complexity_source(CampaignID) bound [/string].
Decl intelligence_campaign_risk(CampaignID, RiskLevel, Reason) bound [/string, /name, /string].
Decl intelligence_has_high_risk_source(CampaignID) bound [/string].
Decl intelligence_has_medium_risk_source(CampaignID) bound [/string].
