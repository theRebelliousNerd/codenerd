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
Decl campaign(CampaignID, Type, Title, SourceMaterial, Status).

# campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence)
# Confidence: LLM's confidence in the plan (0-100 integer scale)
Decl campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence).

# campaign_goal(CampaignID, GoalDescription)
# The high-level goal in natural language
Decl campaign_goal(CampaignID, GoalDescription).

# campaign_config(CampaignID, MaxRetries, ReplanThreshold, AutoReplan, CheckpointOnFail)
# Runtime configuration asserted by the Go Campaign Orchestrator.
Decl campaign_config(CampaignID, MaxRetries, ReplanThreshold, AutoReplan, CheckpointOnFail).

# failed_campaign_task_count_computed(CampaignID, Count)
# Runtime-computed count of fully failed tasks in the current campaign.
Decl failed_campaign_task_count_computed(CampaignID, Count).

# -----------------------------------------------------------------------------
# 27.2 Phase Decomposition (LLM + Mangle Collaboration)
# -----------------------------------------------------------------------------

# campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile)
# Status: /pending, /in_progress, /completed, /failed, /skipped
# ContextProfile: ID referencing what context this phase needs
Decl campaign_phase(PhaseID, CampaignID, Name, Order, Status, ContextProfile).

# phase_objective(PhaseID, ObjectiveType, Description, VerificationMethod)
# ObjectiveType: /create, /modify, /test, /research, /validate, /integrate, /review
# VerificationMethod: /tests_pass, /builds, /manual_review, /shard_validation, /none
Decl phase_objective(PhaseID, ObjectiveType, Description, VerificationMethod).

# phase_category(PhaseID, Category) - architectural layer classification for build topology
Decl phase_category(PhaseID, Category).

# build_phase_type(Category, Priority) - canonical build layers (lower number executes first)
Decl build_phase_type(Category, Priority).

# phase_synonym(Category, Alias) - natural language aliases for categories
Decl phase_synonym(Category, Alias).

# phase_precedence(PhaseID, PriorityScore) - derived precedence per phase
Decl phase_precedence(PhaseID, PriorityScore).

# phase_dependency(PhaseID, DependsOnPhaseID, DependencyType)
# DependencyType: /hard (must complete), /soft (preferred), /artifact (needs output)
Decl phase_dependency(PhaseID, DependsOnPhaseID, DependencyType).

# phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity)
# EstimatedComplexity: /low, /medium, /high, /critical
Decl phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity).

# architectural_violation(DownstreamPhase, UpstreamPhase, Reason) - build order inversion detection
Decl architectural_violation(DownstreamPhase, UpstreamPhase, Reason).

# suspicious_gap(DownstreamPhase, UpstreamPhase) - warns on skipping layers
Decl suspicious_gap(DownstreamPhase, UpstreamPhase).

# -----------------------------------------------------------------------------
# 27.3 Task Granularity (Atomic Work Units)
# -----------------------------------------------------------------------------

# campaign_task(TaskID, PhaseID, Description, Status, TaskType)
# TaskType: /file_create, /file_modify, /test_write, /test_run, /research,
#           /shard_spawn, /tool_create, /verify, /document, /refactor, /integrate
# Status: /pending, /in_progress, /completed, /failed, /skipped, /blocked
Decl campaign_task(TaskID, PhaseID, Description, Status, TaskType).
# eligible_task(TaskID) - derived: runnable tasks (deps met, no conflicts)
Decl eligible_task(TaskID).
# task_conflict(TaskID, OtherTaskID) - optional: tasks that must not run together
Decl task_conflict(TaskID, OtherTaskID).

# task_priority(TaskID, Priority)
# Priority: /critical, /high, /normal, /low
Decl task_priority(TaskID, Priority).

# task_order(TaskID, OrderIndex) - stable ordering for deterministic scheduling
Decl task_order(TaskID, OrderIndex).

# task_dependency(TaskID, DependsOnTaskID)
Decl task_dependency(TaskID, DependsOnTaskID).

# task_remediation_target(RemediationTaskID, OriginalTaskID, ViolationType)
# Tracks which task a remediation fix is targeting and what violation it addresses
Decl task_remediation_target(RemediationTaskID, OriginalTaskID, ViolationType).

# task_artifact(TaskID, ArtifactType, Path, Hash)
# ArtifactType: /source_file, /test_file, /config, /shard_agent, /knowledge_base, /doc
Decl task_artifact(TaskID, ArtifactType, Path, Hash).

# task_inference(TaskID, InferredFrom, Confidence, Reasoning)
# Tracks WHERE this task came from (spec section, user intent, LLM inference)
Decl task_inference(TaskID, InferredFrom, Confidence, Reasoning).

# task_attempt(TaskID, AttemptNumber, Outcome, Timestamp)
# Outcome: /success, /failure, /partial
Decl task_attempt(TaskID, AttemptNumber, Outcome, Timestamp).

# task_error(TaskID, ErrorType, ErrorMessage)
Decl task_error(TaskID, ErrorType, ErrorMessage).

# -----------------------------------------------------------------------------
# 27.4 Context Profiles (Phase-Aware Context Paging)
# -----------------------------------------------------------------------------

# context_profile(ProfileID, RequiredSchemas, RequiredTools, FocusPatterns)
# RequiredSchemas: comma-separated schema sections (e.g., "file_topology,symbol_graph")
# RequiredTools: comma-separated tools (e.g., "fs_read,fs_write,exec_cmd")
# FocusPatterns: glob patterns for files to activate (e.g., "internal/core/*")
Decl context_profile(ProfileID, RequiredSchemas, RequiredTools, FocusPatterns).

# tool_in_list(Tool, ToolList) - helper predicate to check if tool is in comma-separated list
Decl tool_in_list(Tool, ToolList).

# phase_context_atom(PhaseID, FactPredicate, ActivationBoost)
# Specific facts that should be boosted for this phase
Decl phase_context_atom(PhaseID, FactPredicate, ActivationBoost).

# context_compression(PhaseID, CompressedSummary, OriginalAtomCount, Timestamp)
# When a phase completes, its verbose context is compressed to a summary
Decl context_compression(PhaseID, CompressedSummary, OriginalAtomCount, Timestamp).

# context_window_state(CampaignID, UsedTokens, TotalBudget, Utilization)
Decl context_window_state(CampaignID, UsedTokens, TotalBudget, Utilization).

# -----------------------------------------------------------------------------
# 27.5 Progress & Verification
# -----------------------------------------------------------------------------

# campaign_progress(CampaignID, CompletedPhases, TotalPhases, CompletedTasks, TotalTasks)
Decl campaign_progress(CampaignID, CompletedPhases, TotalPhases, CompletedTasks, TotalTasks).

# campaign_heartbeat(CampaignID, Timestamp) - last heartbeat from orchestrator
Decl campaign_heartbeat(CampaignID, Timestamp).

# task_retry_at(TaskID, RetryAt) - next allowed retry time for a task
Decl task_retry_at(TaskID, RetryAt).

# task_in_backoff(TaskID) - derived: task not yet eligible to retry
Decl task_in_backoff(TaskID).

# phase_checkpoint(PhaseID, CheckpointType, Passed, Details, Timestamp)
# CheckpointType: /tests, /build, /lint, /coverage, /manual, /integration
Decl phase_checkpoint(PhaseID, CheckpointType, Passed, Details, Timestamp).

# campaign_milestone(CampaignID, MilestoneID, Description, ReachedAt)
Decl campaign_milestone(CampaignID, MilestoneID, Description, ReachedAt).

# campaign_learning(CampaignID, LearningType, Pattern, Fact, AppliedAt)
# Tracks what the system learned during execution (autopoiesis)
# LearningType: /success_pattern, /failure_pattern, /preference, /optimization
Decl campaign_learning(CampaignID, LearningType, Pattern, Fact, AppliedAt).

# -----------------------------------------------------------------------------
# 27.6 Replanning & Adaptation
# -----------------------------------------------------------------------------

# replan_trigger(CampaignID, Reason, TriggeredAt)
# Reason: /task_failed, /new_requirement, /user_feedback, /dependency_change, /blocked
Decl replan_trigger(CampaignID, Reason, TriggeredAt).

# plan_revision(CampaignID, RevisionNumber, ChangeSummary, Timestamp)
Decl plan_revision(CampaignID, RevisionNumber, ChangeSummary, Timestamp).

# plan_validation_issue(CampaignID, IssueType, Description)
# IssueType: /missing_dependency, /circular_dependency, /unreachable_task, /ambiguous_goal
Decl plan_validation_issue(CampaignID, IssueType, Description).

# -----------------------------------------------------------------------------
# 27.7 Campaign Shard Delegation
# -----------------------------------------------------------------------------

# campaign_shard(CampaignID, ShardID, ShardType, Task, Status)
# Tracks shards spawned as part of campaign execution
Decl campaign_shard(CampaignID, ShardID, ShardType, Task, Status).

# campaign_intent_capture(CampaignID, Goal, ClarifierAnswers, AutonomyLevel, Constraints)
# Captures the raw goal and clarifier responses used to launch a campaign
Decl campaign_intent_capture(CampaignID, Goal, ClarifierAnswers, AutonomyLevel, Constraints).

# shard_result_event(ShardID, ResultType, ResultData, Timestamp)
# ResultType: /success, /failure, /partial, /knowledge
Decl shard_result_event(ShardID, ResultType, ResultData, Timestamp).

# -----------------------------------------------------------------------------
# 27.8 Source Material Ingestion
# -----------------------------------------------------------------------------

# source_document(CampaignID, DocPath, DocType, ParsedAt)
# DocType: /spec, /requirements, /design, /readme, /api_doc, /tutorial
Decl source_document(CampaignID, DocPath, DocType, ParsedAt).

# doc_metadata(CampaignID, Path, DocType, SizeBytes, ModifiedAt)
Decl doc_metadata(CampaignID, Path, DocType, SizeBytes, ModifiedAt).
# goal_topic(CampaignID, Topic) - topics extracted from goal text for selection
Decl goal_topic(CampaignID, Topic).

# doc_tag(Path, Tag)
Decl doc_tag(Path, Tag).

# doc_reference(FromPath, ToPath)
Decl doc_reference(FromPath, ToPath).

# doc_layer(Path, Layer, Confidence) - architectural layer classification
Decl doc_layer(Path, Layer, Confidence).

# layer_priority(Layer, Priority) - execution ordering for layers (lower runs first)
Decl layer_priority(Layer, Priority).

# layer_distance(LayerA, LayerB, Distance) - computed layer separation
Decl layer_distance(LayerA, LayerB, Distance).

# doc_conflict(DocPath, LayerA, LayerB) - detects docs spanning distant layers
Decl doc_conflict(DocPath, LayerA, LayerB).

# active_layer(Layer) - derived: layer has confident docs
Decl active_layer(Layer).

# proposed_phase(Layer) - derived: active layer becomes a phase candidate
Decl proposed_phase(Layer).

# phase_dependency_generated(PhaseA, PhaseB) - derived ordering from taxonomy
Decl phase_dependency_generated(PhaseA, PhaseB).

# phase_context_scope(Phase, DocPath) - derived: docs allowed for a phase
Decl phase_context_scope(Phase, DocPath).

# source_requirement(CampaignID, ReqID, Description, Priority, Source)
# Requirements extracted from source documents
Decl source_requirement(CampaignID, ReqID, Description, Priority, Source).

# requirement_coverage(ReqID, TaskID)
# Maps requirements to tasks that fulfill them
Decl requirement_coverage(ReqID, TaskID).

# -----------------------------------------------------------------------------
# 27.9 Campaign Derived Predicates (Helpers)
# -----------------------------------------------------------------------------

# current_campaign(CampaignID) - derived: the active campaign
Decl current_campaign(CampaignID).

# current_phase(PhaseID) - derived: the current phase being executed
Decl current_phase(PhaseID).

# next_campaign_task(TaskID) - derived: the next task to execute
Decl next_campaign_task(TaskID).

# phase_eligible(PhaseID) - derived: phase ready to start
Decl phase_eligible(PhaseID).

# phase_blocked(PhaseID, Reason) - derived: phase cannot proceed
Decl phase_blocked(PhaseID, Reason).

# campaign_blocked(CampaignID, Reason) - derived: campaign cannot proceed
Decl campaign_blocked(CampaignID, Reason).

# validation_error(EntityID, IssueType, Message) - derived: validation issues found
Decl validation_error(EntityID, IssueType, Message).

# replan_needed(CampaignID, Reason) - derived: campaign needs replanning
Decl replan_needed(CampaignID, Reason).

# phase_stuck(PhaseID) - derived: in-progress phase with no runnable work
Decl phase_stuck(PhaseID).

# has_pending_tasks(PhaseID) - helper for safe negation
Decl has_pending_tasks(PhaseID).

# has_running_tasks(PhaseID) - helper for safe negation
Decl has_running_tasks(PhaseID).

# debug_why_blocked(TaskID, Dependency) - helper to explain blocking
Decl debug_why_blocked(TaskID, Dependency).

