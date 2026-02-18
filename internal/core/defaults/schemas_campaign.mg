# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: CAMPAIGN
# Sections: 27

# =============================================================================
# SECTION 27: CAMPAIGN ORCHESTRATION (Long-Running Goal Execution)
# =============================================================================
# Campaigns are long-running, multi-phase goals that can span sessions.
# Examples: greenfield builds, large features, stability audits, migrations

# -----------------------------------------------------------------------------
# 27.1 Campaign Identity
# -----------------------------------------------------------------------------

# campaign(CampaignID, Type, Title, SourceMaterial, Status)
# Type: /greenfield, /feature, /audit, /migration, /remediation, /custom
# Status: /planning, /decomposing, /validating, /active, /paused, /completed, /failed
Decl campaign(CampaignID, Type, Title, SourceMaterial, Status) bound [/string, /name, /string, /string, /name].

# campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence)
# Confidence: LLM's confidence in the plan (0-100 integer scale)
Decl campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence) bound [/string, /number, /number, /number].

# campaign_goal(CampaignID, GoalDescription)
# The high-level goal in natural language
Decl campaign_goal(CampaignID, GoalDescription) bound [/string, /string].

# campaign_config(CampaignID, MaxRetries, ReplanThreshold, AutoReplan, CheckpointOnFail)
# Runtime configuration asserted by the Go Campaign Orchestrator.
Decl campaign_config(CampaignID, MaxRetries, ReplanThreshold, AutoReplan, CheckpointOnFail) bound [/string, /number, /number, /name, /name].

# failed_campaign_task_count_computed(CampaignID, Count)
# Runtime-computed count of fully failed tasks in the current campaign.
Decl failed_campaign_task_count_computed(CampaignID, Count) bound [/string, /number].

# -----------------------------------------------------------------------------
# 27.2 Phase Decomposition (LLM + Mangle Collaboration)
# -----------------------------------------------------------------------------

# campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile)
# Status: /pending, /in_progress, /completed, /failed, /skipped
# ContextProfile: ID referencing what context this phase needs
Decl campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile) bound [/string, /string, /string, /number, /name, /string].

# phase_objective(PhaseID, ObjectiveType, Description, VerificationMethod)
# ObjectiveType: /create, /modify, /test, /research, /validate, /integrate, /review
# VerificationMethod: /tests_pass, /builds, /manual_review, /shard_validation, /none
Decl phase_objective(PhaseID, ObjectiveType, Description, VerificationMethod) bound [/string, /name, /string, /name].

# phase_category(PhaseID, Category) - architectural layer classification for build topology
Decl phase_category(PhaseID, Category) bound [/string, /name].

# build_phase_type(Category, Priority) - canonical build layers (lower number executes first)
Decl build_phase_type(Category, Priority) bound [/name, /number].

# phase_synonym(Category, Alias) - natural language aliases for categories
Decl phase_synonym(Category, Alias) bound [/name, /string].

# phase_precedence(PhaseID, PriorityScore) - derived precedence per phase
Decl phase_precedence(PhaseID, PriorityScore) bound [/string, /number].

# phase_dependency(PhaseID, DependsOnPhaseID, DependencyType)
# DependencyType: /hard (must complete), /soft (preferred), /artifact (needs output)
Decl phase_dependency(PhaseID, DependsOnPhaseID, DependencyType) bound [/string, /string, /name].

# phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity)
# EstimatedComplexity: /low, /medium, /high, /critical
Decl phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity) bound [/string, /number, /number].

# architectural_violation(DownstreamPhase, UpstreamPhase, Reason) - build order inversion detection
Decl architectural_violation(DownstreamPhase, UpstreamPhase, Reason) bound [/string, /string, /string].

# suspicious_gap(DownstreamPhase, UpstreamPhase) - warns on skipping layers
Decl suspicious_gap(DownstreamPhase, UpstreamPhase) bound [/string, /string].

# -----------------------------------------------------------------------------
# 27.3 Task Granularity (Atomic Work Units)
# -----------------------------------------------------------------------------

# campaign_task(TaskID, PhaseID, Description, Status, TaskType)
# TaskType: /file_create, /file_modify, /test_write, /test_run, /research,
#           /shard_spawn, /tool_create, /verify, /document, /refactor, /integrate
# Status: /pending, /in_progress, /completed, /failed, /skipped, /blocked
Decl campaign_task(TaskID, PhaseID, Description, Status, TaskType) bound [/string, /string, /string, /name, /name].
# eligible_task(TaskID) - derived: runnable tasks (deps met, no conflicts)
Decl eligible_task(TaskID) bound [/string].
# task_conflict(TaskID, OtherTaskID) - optional: tasks that must not run together
Decl task_conflict(TaskID, OtherTaskID) bound [/string, /string].

# task_priority(TaskID, Priority)
# Priority: /critical, /high, /normal, /low
Decl task_priority(TaskID, Priority) bound [/string, /number].

# task_order(TaskID, OrderIndex) - stable ordering for deterministic scheduling
Decl task_order(TaskID, OrderIndex) bound [/string, /number].

# task_dependency(TaskID, DependsOnTaskID)
Decl task_dependency(TaskID, DependsOnTaskID) bound [/string, /string].

# task_remediation_target(RemediationTaskID, OriginalTaskID, ViolationType)
# Tracks which task a remediation fix is targeting and what violation it addresses
Decl task_remediation_target(RemediationTaskID, OriginalTaskID, ViolationType) bound [/string, /string, /name].

# task_artifact(TaskID, ArtifactType, Path, Hash)
# ArtifactType: /source_file, /test_file, /config, /shard_agent, /knowledge_base, /doc
Decl task_artifact(TaskID, ArtifactType, Path, Hash) bound [/string, /name, /string, /string].

# task_inference(TaskID, InferredFrom, Confidence, Reasoning)
# Tracks WHERE this task came from (spec section, user intent, LLM inference)
Decl task_inference(TaskID, InferredFrom, Confidence, Reasoning) bound [/string, /string, /number, /string].

# task_attempt(TaskID, AttemptNumber, Outcome, Timestamp)
# Outcome: /success, /failure, /partial
Decl task_attempt(TaskID, AttemptNumber, Outcome, Timestamp) bound [/string, /number, /name, /number].

# task_error(TaskID, ErrorType, ErrorMessage)
Decl task_error(TaskID, ErrorType, ErrorMessage) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 27.4 Context Profiles (Phase-Aware Context Paging)
# -----------------------------------------------------------------------------

# context_profile(ProfileID, RequiredSchemas, RequiredTools, FocusPatterns)
# RequiredSchemas: comma-separated schema sections (e.g., "file_topology,symbol_graph")
# RequiredTools: comma-separated tools (e.g., "fs_read,fs_write,exec_cmd")
# FocusPatterns: glob patterns for files to activate (e.g., "internal/core/*")
Decl context_profile(ProfileID, RequiredSchemas, RequiredTools, FocusPatterns) bound [/string, /string, /string, /string].

# tool_in_list(Tool, ToolList) - helper predicate to check if tool is in comma-separated list
Decl tool_in_list(Tool, ToolList) bound [/name, /string].

# phase_context_atom(PhaseID, FactPredicate, ActivationBoost)
# Specific facts that should be boosted for this phase
Decl phase_context_atom(PhaseID, FactPredicate, ActivationBoost) bound [/string, /string, /number].

# context_compression(PhaseID, CompressedSummary, OriginalAtomCount, Timestamp)
# When a phase completes, its verbose context is compressed to a summary
Decl context_compression(PhaseID, CompressedSummary, OriginalAtomCount, Timestamp) bound [/string, /string, /number, /number].

# context_window_state(CampaignID, UsedTokens, TotalBudget, Utilization)
Decl context_window_state(CampaignID, UsedTokens, TotalBudget, Utilization) bound [/string, /number, /number, /number].

# -----------------------------------------------------------------------------
# 27.5 Progress & Verification
# -----------------------------------------------------------------------------

# campaign_progress(CampaignID, CompletedPhases, TotalPhases, CompletedTasks, TotalTasks)
Decl campaign_progress(CampaignID, CompletedPhases, TotalPhases, CompletedTasks, TotalTasks) bound [/string, /number, /number, /number, /number].

# campaign_completed(CampaignID, Summary)
# Emitted when a campaign reaches a terminal completed state.
Decl campaign_completed(CampaignID, Summary) bound [/string, /string].

# campaign_heartbeat(CampaignID, Timestamp) - last heartbeat from orchestrator
Decl campaign_heartbeat(CampaignID, Timestamp) bound [/string, /number].

# task_retry_at(TaskID, RetryAt) - next allowed retry time for a task
Decl task_retry_at(TaskID, RetryAt) bound [/string, /number].

# task_in_backoff(TaskID) - derived: task not yet eligible to retry
Decl task_in_backoff(TaskID) bound [/string].

# phase_checkpoint(PhaseID, CheckpointType, Passed, Details, Timestamp)
# CheckpointType: /tests, /build, /lint, /coverage, /manual, /integration
Decl phase_checkpoint(PhaseID, CheckpointType, Passed, Details, Timestamp) bound [/string, /name, /name, /string, /number].

# campaign_milestone(CampaignID, MilestoneID, Description, ReachedAt)
Decl campaign_milestone(CampaignID, MilestoneID, Description, ReachedAt) bound [/string, /string, /string, /number].

# campaign_learning(CampaignID, LearningType, Pattern, Fact, AppliedAt)
# Tracks what the system learned during execution (autopoiesis)
# LearningType: /success_pattern, /failure_pattern, /preference, /optimization
Decl campaign_learning(CampaignID, LearningType, Pattern, Fact, AppliedAt) bound [/string, /name, /string, /string, /number].

# -----------------------------------------------------------------------------
# 27.6 Replanning & Adaptation
# -----------------------------------------------------------------------------

# replan_trigger(CampaignID, Reason, TriggeredAt)
# Reason: /task_failed, /new_requirement, /user_feedback, /dependency_change, /blocked
Decl replan_trigger(CampaignID, Reason, TriggeredAt) bound [/string, /name, /number].

# plan_revision(CampaignID, RevisionNumber, ChangeSummary, Timestamp)
Decl plan_revision(CampaignID, RevisionNumber, ChangeSummary, Timestamp) bound [/string, /number, /string, /number].

# plan_validation_issue(CampaignID, IssueType, Description)
# IssueType: /missing_dependency, /circular_dependency, /unreachable_task, /ambiguous_goal
Decl plan_validation_issue(CampaignID, IssueType, Description) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 27.7 Campaign Shard Delegation
# -----------------------------------------------------------------------------

# campaign_shard(CampaignID, ShardID, ShardType, Task, Status)
# Tracks shards spawned as part of campaign execution
Decl campaign_shard(CampaignID, ShardID, ShardType, Task, Status) bound [/string, /string, /name, /string, /name].

# campaign_intent_capture(CampaignID, Goal, ClarifierAnswers, AutonomyLevel, Constraints)
# Captures the raw goal and clarifier responses used to launch a campaign
Decl campaign_intent_capture(CampaignID, Goal, ClarifierAnswers, AutonomyLevel, Constraints) bound [/string, /string, /string, /name, /string].

# shard_result_event(ShardID, ResultType, ResultData, Timestamp)
# ResultType: /success, /failure, /partial, /knowledge
Decl shard_result_event(ShardID, ResultType, ResultData, Timestamp) bound [/string, /name, /string, /number].

# -----------------------------------------------------------------------------
# 27.8 Source Material Ingestion
# -----------------------------------------------------------------------------

# source_document(CampaignID, DocPath, DocType, ParsedAt)
# DocType: /spec, /requirements, /design, /readme, /api_doc, /tutorial
Decl source_document(CampaignID, DocPath, DocType, ParsedAt) bound [/string, /string, /name, /number].

# doc_metadata(CampaignID, Path, DocType, SizeBytes, ModifiedAt)
Decl doc_metadata(CampaignID, Path, DocType, SizeBytes, ModifiedAt) bound [/string, /string, /name, /number, /number].
# goal_topic(CampaignID, Topic) - topics extracted from goal text for selection
Decl goal_topic(CampaignID, Topic) bound [/string, /string].

# doc_tag(Path, Tag)
Decl doc_tag(Path, Tag) bound [/string, /string].

# doc_reference(FromPath, ToPath)
Decl doc_reference(FromPath, ToPath) bound [/string, /string].

# doc_layer(Path, Layer, Confidence) - architectural layer classification
Decl doc_layer(Path, Layer, Confidence) bound [/string, /name, /number].

# layer_priority(Layer, Priority) - execution ordering for layers (lower runs first)
Decl layer_priority(Layer, Priority) bound [/name, /number].

# layer_distance(LayerA, LayerB, Distance) - computed layer separation
Decl layer_distance(LayerA, LayerB, Distance) bound [/name, /name, /number].

# doc_conflict(DocPath, LayerA, LayerB) - detects docs spanning distant layers
Decl doc_conflict(DocPath, LayerA, LayerB) bound [/string, /name, /name].

# active_layer(Layer) - derived: layer has confident docs
Decl active_layer(Layer) bound [/name].

# proposed_phase(Layer) - derived: active layer becomes a phase candidate
Decl proposed_phase(Layer) bound [/name].

# phase_dependency_generated(PhaseA, PhaseB) - derived ordering from taxonomy
Decl phase_dependency_generated(PhaseA, PhaseB) bound [/name, /name].

# phase_context_scope(Phase, DocPath) - derived: docs allowed for a phase
Decl phase_context_scope(Phase, DocPath) bound [/name, /string].

# source_requirement(CampaignID, ReqID, Description, Priority, Source)
# Requirements extracted from source documents
Decl source_requirement(CampaignID, ReqID, Description, Priority, Source) bound [/string, /string, /string, /number, /string].

# requirement_coverage(ReqID, TaskID)
# Maps requirements to tasks that fulfill them
Decl requirement_coverage(ReqID, TaskID) bound [/string, /string].

# -----------------------------------------------------------------------------
# 27.9 Campaign Derived Predicates (Helpers)
# -----------------------------------------------------------------------------

# current_campaign(CampaignID) - derived: the active campaign
Decl current_campaign(CampaignID) bound [/string].

# current_phase(PhaseID) - derived: the current phase being executed
Decl current_phase(PhaseID) bound [/string].

# next_campaign_task(TaskID) - derived: the next task to execute
Decl next_campaign_task(TaskID) bound [/string].

# pending_task_priority(TaskID, PhaseID, Priority) - derived: pending task priority
Decl pending_task_priority(TaskID, PhaseID, Priority) bound [/string, /string, /number].

# phase_eligible(PhaseID) - derived: phase ready to start
Decl phase_eligible(PhaseID) bound [/string].

# phase_eligible_in_campaign(PhaseID, CampaignID) - derived: eligibility scoped to campaign
Decl phase_eligible_in_campaign(PhaseID, CampaignID) bound [/string, /string].

# has_incomplete_hard_dep_in_campaign(PhaseID, CampaignID) - helper: dependency check scoped to campaign
Decl has_incomplete_hard_dep_in_campaign(PhaseID, CampaignID) bound [/string, /string].

# goal_topic_count(CampaignID, Count) - derived: topic count for campaign goal
Decl goal_topic_count(CampaignID, Count) bound [/string, /number].

# campaign_type_count(Type, Count) - derived: completed campaign count per type
Decl campaign_type_count(Type, Count) bound [/name, /number].

# campaign_phase_count(CampaignID, Count) - derived: phase count per campaign
Decl campaign_phase_count(CampaignID, Count) bound [/string, /number].

# campaign_task_error(CampaignID, ErrorType) - derived: failed task errors within campaign
Decl campaign_task_error(CampaignID, ErrorType) bound [/string, /name].

# campaign_task_error_count(CampaignID, ErrorType, Count) - derived: error counts per campaign
Decl campaign_task_error_count(CampaignID, ErrorType, Count) bound [/string, /name, /number].

# systemic_error_detected(CampaignID, ErrorType) - derived: repeated errors within campaign
Decl systemic_error_detected(CampaignID, ErrorType) bound [/string, /name].

# phase_failed_task_count(PhaseID, Count) - derived: failed task count per phase
Decl phase_failed_task_count(PhaseID, Count) bound [/string, /number].

# shard_failure_count(ShardType, Count) - derived: shard failure count
Decl shard_failure_count(ShardType, Count) bound [/name, /number].

# phase_blocked(PhaseID, Reason) - derived: phase cannot proceed
Decl phase_blocked(PhaseID, Reason) bound [/string, /string].

# campaign_blocked(CampaignID, Reason) - derived: campaign cannot proceed
Decl campaign_blocked(CampaignID, Reason) bound [/string, /string].

# validation_error(EntityID, IssueType, Message) - derived: validation issues found
Decl validation_error(EntityID, IssueType, Message) bound [/string, /name, /string].

# replan_needed(CampaignID, Reason) - derived: campaign needs replanning
Decl replan_needed(CampaignID, Reason) bound [/string, /name].

# phase_stuck(PhaseID) - derived: in-progress phase with no runnable work
Decl phase_stuck(PhaseID) bound [/string].

# has_pending_tasks(PhaseID) - helper for safe negation
Decl has_pending_tasks(PhaseID) bound [/string].

# has_running_tasks(PhaseID) - helper for safe negation
Decl has_running_tasks(PhaseID) bound [/string].

# debug_why_blocked(TaskID, Dependency) - helper to explain blocking
Decl debug_why_blocked(TaskID, Dependency) bound [/string, /string].
