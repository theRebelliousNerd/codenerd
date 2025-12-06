# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# =============================================================================
# SECTION 1: INTENT SCHEMA (§1.1)
# =============================================================================

# user_intent(ID, Category, Verb, Target, Constraint)
# Category: /query, /mutation, /instruction
# Verb: /explain, /refactor, /debug, /generate, /scaffold, /init, /test, /review, /fix, /run, /research, /explore, /implement
Decl user_intent(ID, Category, Verb, Target, Constraint).

# =============================================================================
# SECTION 2: FOCUS RESOLUTION (§1.2)
# =============================================================================

# focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence)
Decl focus_resolution(RawReference, ResolvedPath, SymbolName, Confidence).

# ambiguity_flag(MissingParam, ContextClue, Hypothesis)
Decl ambiguity_flag(MissingParam, ContextClue, Hypothesis).

# =============================================================================
# SECTION 3: FILE TOPOLOGY (§2.1)
# =============================================================================

# file_topology(Path, Hash, Language, LastModified, IsTestFile)
# Language: /go, /python, /ts, /rust, /java, /js
# IsTestFile: /true, /false
Decl file_topology(Path, Hash, Language, LastModified, IsTestFile).

# modified(FilePath) - marks a file as modified
Decl modified(FilePath).

# test_coverage(FilePath) - marks a file as having test coverage
Decl test_coverage(FilePath).

# =============================================================================
# SECTION 4: SYMBOL GRAPH / AST PROJECTION (§2.3)
# =============================================================================

# symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature)
# Type: /function, /class, /interface, /struct, /variable, /constant
# Visibility: /public, /private, /protected
Decl symbol_graph(SymbolID, Type, Visibility, DefinedAt, Signature).

# dependency_link(CallerID, CalleeID, ImportPath)
Decl dependency_link(CallerID, CalleeID, ImportPath).

# =============================================================================
# SECTION 5: DIAGNOSTICS / LINTER-LOGIC BRIDGE (§2.2)
# =============================================================================

# diagnostic(Severity, FilePath, Line, ErrorCode, Message)
# Severity: /panic, /error, /warning, /info
Decl diagnostic(Severity, FilePath, Line, ErrorCode, Message).

# =============================================================================
# SECTION 6: SHARD DELEGATION (§7.0)
# =============================================================================

# delegate_task(ShardType, TaskDescription, Status)
# ShardType: /researcher, /coder, /reviewer, /tester, /generalist, /specialist
# Status: /pending, /in_progress, /completed, /failed
Decl delegate_task(ShardType, TaskDescription, Status).

# shard_profile(AgentName, Type, KnowledgePath)
Decl shard_profile(AgentName, Type, KnowledgePath).

# =============================================================================
# SECTION 7: MEMORY SHARDS (§7.1-7.4)
# =============================================================================

# vector_recall(Query, Content, Score)
Decl vector_recall(Query, Content, Score).

# knowledge_link(EntityA, Relation, EntityB)
Decl knowledge_link(EntityA, Relation, EntityB).

# new_fact(FactID) - marks a fact as newly added (for activation)
Decl new_fact(FactID).

# =============================================================================
# SECTION 7B: VIRTUAL PREDICATES FOR KNOWLEDGE QUERIES (Bound)
# =============================================================================
# These predicates are resolved by VirtualStore FFI to query knowledge.db
# Virtual predicates computed on-demand by the Go runtime (VirtualStore)

# query_learned(Predicate, Args) - Queries cold_storage for learned facts
Decl query_learned(Predicate, Args).

# query_session(SessionID, TurnNumber, UserInput) - Queries session_history
Decl query_session(SessionID, TurnNumber, UserInput).

# recall_similar(Query, TopK, Results) - Semantic search on vectors table
Decl recall_similar(Query, TopK, Results).

# query_knowledge_graph(EntityA, Relation, EntityB) - Entity relationships
Decl query_knowledge_graph(EntityA, Relation, EntityB).

# query_activations(FactID, Score) - Activation log scores
Decl query_activations(FactID, Score).

# has_learned(Predicate) - Check if facts exist in cold_storage
Decl has_learned(Predicate).

# =============================================================================
# SECTION 7C: HYDRATED KNOWLEDGE FACTS (asserted by HydrateLearnings)
# =============================================================================
# These EDB predicates are populated by VirtualStore.HydrateLearnings() during
# the OODA Observe phase. They make learned knowledge available to Mangle rules.

# learned_preference(Predicate, Args) - User preferences from cold_storage
Decl learned_preference(Predicate, Args).

# learned_fact(Predicate, Args) - User facts from cold_storage
Decl learned_fact(Predicate, Args).

# learned_constraint(Predicate, Args) - User constraints from cold_storage
Decl learned_constraint(Predicate, Args).

# activation(FactID, Score) - Recent activation scores
Decl activation(FactID, Score).

# session_turn(SessionID, TurnNumber, UserInput, Response) - Conversation history
Decl session_turn(SessionID, TurnNumber, UserInput, Response).

# similar_content(Rank, Content) - Semantic search results
Decl similar_content(Rank, Content).

# =============================================================================
# SECTION 7D: HELPER PREDICATES FOR LEARNED KNOWLEDGE RULES
# =============================================================================
# These support the IDB rules in policy.gl SECTION 17B

# tool_language(Tool, Language) - Maps tools to programming languages
Decl tool_language(Tool, Language).

# action_violates(Action, Predicate, Args) - Check if action violates a constraint
Decl action_violates(Action, Predicate, Args).

# relevant_to_intent(Predicate, Intent) - Maps predicates to user intents
Decl relevant_to_intent(Predicate, Intent).

# context_priority(FactID, Priority) - Priority level for context inclusion
Decl context_priority(FactID, Priority).

# related_context(Content) - Content related to current context
Decl related_context(Content).

# constraint_violation(Action, Reason) - Detected constraint violations
Decl constraint_violation(Action, Reason).

# =============================================================================
# SECTION 8: BROWSER PHYSICS (§9.0)
# =============================================================================

# dom_node(ID, Tag, Parent)
Decl dom_node(ID, Tag, Parent).

# attr(ID, Key, Val)
Decl attr(ID, Key, Val).

# geometry(ID, X, Y, W, H)
Decl geometry(ID, X, Y, W, H).

# computed_style(ID, Prop, Val)
Decl computed_style(ID, Prop, Val).

# interactable(ID, Type)
# Type: /button, /input, /link, /select, /checkbox
Decl interactable(ID, Type).

# visible_text(ID, Text)
Decl visible_text(ID, Text).

# =============================================================================
# SECTION 9: TDD REPAIR LOOP STATE (§3.2)
# =============================================================================

# test_state(State)
# State: /failing, /log_read, /cause_found, /patch_applied, /passing, /unknown
Decl test_state(State).

# test_type(Type)
# Type: /unit, /integration, /e2e (detected from test file patterns)
Decl test_type(Type).

# retry_count(Count)
Decl retry_count(Count).

# =============================================================================
# SECTION 10: ACTION & EXECUTION (§4.0)
# =============================================================================

# next_action(ActionType)
# ActionType: /read_error_log, /analyze_root_cause, /generate_patch, /run_tests,
#             /escalate_to_user, /complete, /interrogative_mode
Decl next_action(ActionType).

# action_details(ActionType, Payload)
Decl action_details(ActionType, Payload).

# safe_action(ActionType)
Decl safe_action(ActionType).

# =============================================================================
# SECTION 11: CONSTITUTIONAL LOGIC / SAFETY (§5.0)
# =============================================================================

# permitted(ActionType) - derived predicate
Decl permitted(ActionType).

# dangerous_action(ActionType) - derived predicate
Decl dangerous_action(ActionType).

# admin_override(User)
Decl admin_override(User).

# signed_approval(ActionType)
Decl signed_approval(ActionType).

# allowed_domain(Domain) - network allowlist
Decl allowed_domain(Domain).

# network_permitted(URL) - derived predicate
Decl network_permitted(URL).

# security_violation(ViolationType) - derived predicate
Decl security_violation(ViolationType).

# -----------------------------------------------------------------------------
# SECTION 11A: APPEAL MECHANISM (Constitutional Appeals)
# -----------------------------------------------------------------------------

# appeal_available(ActionID, ActionType, Target, Reason) - action can be appealed
Decl appeal_available(ActionID, ActionType, Target, Reason).

# appeal_pending(ActionID, ActionType, Justification, Timestamp) - appeal submitted
Decl appeal_pending(ActionID, ActionType, Justification, Timestamp).

# appeal_granted(ActionID, ActionType, Approver, Timestamp) - appeal approved
Decl appeal_granted(ActionID, ActionType, Approver, Timestamp).

# appeal_denied(ActionID, ActionType, Reason, Timestamp) - appeal rejected
Decl appeal_denied(ActionID, ActionType, Reason, Timestamp).

# temporary_override(ActionType, ExpirationTimestamp) - temporary permission with expiration
# NOTE: Changed from (ActionType, DurationSeconds, OverrideTime) to store computed expiration
# This avoids arithmetic in Mangle rules (+ operator not supported in rule bodies)
Decl temporary_override(ActionType, ExpirationTimestamp).

# has_temporary_override(ActionType) - helper predicate for safe negation
Decl has_temporary_override(ActionType).

# user_requests_appeal(ActionID, Justification, Requester) - user appeal request
Decl user_requests_appeal(ActionID, Justification, Requester).

# active_override(ActionType, Approver, ExpiresAt) - currently active override
Decl active_override(ActionType, Approver, ExpiresAt).

# appeal_history(ActionID, Granted, Approver, Timestamp) - appeal audit trail
Decl appeal_history(ActionID, Granted, Approver, Timestamp).

# suggest_appeal(ActionID) - derived: suggest user can appeal this action
Decl suggest_appeal(ActionID).

# appeal_needs_review(ActionID, ActionType, Justification) - pending appeal requires review
Decl appeal_needs_review(ActionID, ActionType, Justification).

# has_active_override(ActionType) - helper: true if override is active
Decl has_active_override(ActionType).

# excessive_appeal_denials() - helper: true if too many denials (autopoiesis signal)
Decl excessive_appeal_denials().

# appeal_pattern_detected(ActionType) - pattern detected for learning
Decl appeal_pattern_detected(ActionType).

# =============================================================================
# SECTION 11B: STRATIFIED TRUST - AUTOPOIESIS (Bug #15 Fix)
# =============================================================================

# candidate_action(ActionType) - Learned logic proposals (from learned.gl)
Decl candidate_action(ActionType).

# final_action(ActionType) - Validated actions (Constitution approved)
Decl final_action(ActionType).

# safety_check(ActionType) - Runtime validation predicate
Decl safety_check(ActionType).

# action_denied(ActionType, Reason) - Blocked learned actions
Decl action_denied(ActionType, Reason).

# learned_proposal(ActionType) - Audit trail for learned suggestions
Decl learned_proposal(ActionType).

# blocked_learned_action_count(Count) - Metrics
Decl blocked_learned_action_count(Count).

# =============================================================================
# SECTION 12: SPREADING ACTIVATION (§8.1)
# =============================================================================

# activation(FactID, Score) - declared in Section 7C

# active_goal(Goal)
Decl active_goal(Goal).

# tool_capabilities(Tool, Cap)
# Tool: /fs_read, /fs_write, /exec_cmd, /browser, /code_graph
# Cap: /read, /write, /execute, /navigate, /click, /type, /analyze, /dependencies
Decl tool_capabilities(Tool, Cap).

# has_capability(Cap) - helper for safe negation in missing tool detection
Decl has_capability(Cap).

# goal_requires(Goal, Cap)
Decl goal_requires(Goal, Cap).

# context_atom(Fact) - derived predicate
Decl context_atom(Fact).

# =============================================================================
# SECTION 13: STRATEGY SELECTION (§3.1)
# =============================================================================

# active_strategy(Strategy)
# Strategy: /tdd_repair_loop, /breadth_first_survey, /project_init, /refactor_guard
Decl active_strategy(Strategy).

# target_is_large(Target) - true if target references multiple files/features (Go builtin)
Decl target_is_large(Target).

# target_is_complex(Target) - true if target requires multiple phases (Go builtin)
Decl target_is_complex(Target).

# =============================================================================
# SECTION 14: IMPACT ANALYSIS (§3.3)
# =============================================================================

# impacted(FilePath) - derived predicate
Decl impacted(FilePath).

# unsafe_to_refactor(Target) - derived predicate
Decl unsafe_to_refactor(Target).

# block_refactor(Target, Reason) - derived predicate
Decl block_refactor(Target, Reason).

# block_commit(Reason) - derived predicate
Decl block_commit(Reason).

# =============================================================================
# SECTION 15: ABDUCTIVE REASONING (§8.2)
# =============================================================================

# missing_hypothesis(RootCause)
Decl missing_hypothesis(RootCause).

# clarification_needed(Ref) - derived predicate
Decl clarification_needed(Ref).

# ambiguity_detected(Param) - derived predicate
Decl ambiguity_detected(Param).

# symptom(Context, SymptomType)
Decl symptom(Context, SymptomType).

# known_cause(SymptomType, Cause)
Decl known_cause(SymptomType, Cause).

# has_known_cause(SymptomType) - helper for safe negation
Decl has_known_cause(SymptomType).

# =============================================================================
# SECTION 16: AUTOPOIESIS / LEARNING (§8.3)
# =============================================================================

# rejection_count(Pattern, Count)
Decl rejection_count(Pattern, Count).

# preference_signal(Pattern) - derived predicate
Decl preference_signal(Pattern).

# derived_rule(Pattern, FactType, FactValue) - maps rejection patterns to facts for promotion
Decl derived_rule(Pattern, FactType, FactValue).

# promote_to_long_term(FactType, FactValue) - derived predicate for Autopoiesis (§8.3)
# FactType is a name constant (e.g., /style_preference, /avoid_pattern)
# FactValue is the specific value to learn
Decl promote_to_long_term(FactType, FactValue).

# =============================================================================
# SECTION 17: BROWSER SPATIAL REASONING (§9.0)
# =============================================================================

# left_of(A, B) - derived predicate
Decl left_of(A, B).

# above(A, B) - derived predicate
Decl above(A, B).

# honeypot_detected(ID) - derived predicate
Decl honeypot_detected(ID).

# safe_interactable(ID) - derived predicate
Decl safe_interactable(ID).

# target_checkbox(CheckID, LabelText) - derived predicate
Decl target_checkbox(CheckID, LabelText).

# =============================================================================
# SECTION 18: PROJECT PROFILE (nerd init)
# =============================================================================

# project_profile(ProjectID, Name, Description)
Decl project_profile(ProjectID, Name, Description).

# project_language(Language)
# Language: /go, /rust, /python, /javascript, /typescript, /java, etc.
Decl project_language(Language).

# project_framework(Framework)
# Framework: /gin, /echo, /nextjs, /react, /django, etc.
Decl project_framework(Framework).

# project_architecture(Architecture)
# Architecture: /monolith, /microservices, /clean_architecture, /serverless
Decl project_architecture(Architecture).

# build_system(BuildSystem)
# BuildSystem: /go, /npm, /cargo, /pip, /maven, /gradle
Decl build_system(BuildSystem).

# architectural_pattern(Pattern)
# Pattern: /standard_go_layout, /repository_pattern, /service_layer, /domain_driven
Decl architectural_pattern(Pattern).

# entry_point(FilePath)
Decl entry_point(FilePath).

# =============================================================================
# SECTION 19: USER PREFERENCES (Autopoiesis / Learning)
# =============================================================================

# user_preference(Key, Value)
# Keys: /test_style, /error_handling, /commit_style, /verbosity, /explanation_level
Decl user_preference(Key, Value).

# preference_learned(Key, Value, Timestamp, Confidence)
Decl preference_learned(Key, Value, Timestamp, Confidence).

# =============================================================================
# SECTION 20: SESSION STATE (Pause/Resume Protocol)
# =============================================================================

# session_state(SessionID, State, SerializedContext)
# State: /active, /suspended, /completed
Decl session_state(SessionID, State, SerializedContext).

# pending_clarification(Question, Options, DefaultValue)
Decl pending_clarification(Question, Options, DefaultValue).

# focus_clarification(Response) - user's clarification response
Decl focus_clarification(Response).

# turn_context(TurnNumber, IntentID, ActionsTaken)
Decl turn_context(TurnNumber, IntentID, ActionsTaken).

# =============================================================================
# SECTION 21: GIT-AWARE SAFETY (Chesterton's Fence)
# =============================================================================

# git_history(FilePath, CommitHash, Author, AgeDays, Message)
Decl git_history(FilePath, CommitHash, Author, AgeDays, Message).

# churn_rate(FilePath, ChangeFrequency)
Decl churn_rate(FilePath, ChangeFrequency).

# current_user(UserName)
Decl current_user(UserName).

# current_time(Timestamp) - current system time for learning timestamps
Decl current_time(Timestamp).

# recent_change_by_other(FilePath) - derived predicate
# True if file was changed < 2 days ago by a different author
Decl recent_change_by_other(FilePath).

# chesterton_fence_warning(FilePath, Reason) - derived predicate
# Warns before deleting recently-changed code
Decl chesterton_fence_warning(FilePath, Reason).

# =============================================================================
# SECTION 22: SHADOW MODE / COUNTERFACTUAL REASONING
# =============================================================================

# shadow_state(StateID, ActionID, IsValid)
Decl shadow_state(StateID, ActionID, IsValid).

# simulated_effect(ActionID, FactPredicate, FactArgs)
Decl simulated_effect(ActionID, FactPredicate, FactArgs).

# safe_projection(ActionID) - derived predicate
# True if the action passes safety checks in shadow simulation
Decl safe_projection(ActionID).

# projection_violation(ActionID, ViolationType) - derived predicate
Decl projection_violation(ActionID, ViolationType).

# =============================================================================
# SECTION 23: INTERACTIVE DIFF APPROVAL
# =============================================================================

# pending_mutation(MutationID, FilePath, OldContent, NewContent)
Decl pending_mutation(MutationID, FilePath, OldContent, NewContent).

# mutation_approved(MutationID, ApprovedBy, Timestamp)
Decl mutation_approved(MutationID, ApprovedBy, Timestamp).

# mutation_rejected(MutationID, RejectedBy, Reason)
Decl mutation_rejected(MutationID, RejectedBy, Reason).

# requires_approval(MutationID) - derived predicate
# True if the mutation requires user approval before execution
Decl requires_approval(MutationID).

# =============================================================================
# SECTION 24: KNOWLEDGE ATOMS (Research Results)
# =============================================================================

# knowledge_atom(SourceURL, Concept, Title, Confidence)
Decl knowledge_atom(SourceURL, Concept, Title, Confidence).

# code_pattern(Concept, PatternCode)
Decl code_pattern(Concept, PatternCode).

# anti_pattern(Concept, PatternCode, Reason)
Decl anti_pattern(Concept, PatternCode, Reason).

# research_complete(Query, AtomCount, DurationSeconds)
Decl research_complete(Query, AtomCount, DurationSeconds).

# =============================================================================
# SECTION 25: LSP INTEGRATION (Language Server Protocol)
# =============================================================================

# lsp_definition(Symbol, FilePath, Line, Column)
Decl lsp_definition(Symbol, FilePath, Line, Column).

# lsp_reference(Symbol, RefFile, RefLine)
Decl lsp_reference(Symbol, RefFile, RefLine).

# lsp_hover(Symbol, Documentation)
Decl lsp_hover(Symbol, Documentation).

# lsp_diagnostic(FilePath, Line, Severity, Message)
Decl lsp_diagnostic(FilePath, Line, Severity, Message).

# =============================================================================
# SECTION 26: DERIVATION TRACE (Glass Box Interface)
# =============================================================================

# derivation_trace(Conclusion, RuleApplied, Premises)
Decl derivation_trace(Conclusion, RuleApplied, Premises).

# proof_tree_node(NodeID, ParentID, Fact, RuleName)
Decl proof_tree_node(NodeID, ParentID, Fact, RuleName).

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
# Confidence: LLM's confidence in the plan (0.0-1.0)
Decl campaign_metadata(CampaignID, CreatedAt, EstimatedPhases, Confidence).

# campaign_goal(CampaignID, GoalDescription)
# The high-level goal in natural language
Decl campaign_goal(CampaignID, GoalDescription).

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

# phase_dependency(PhaseID, DependsOnPhaseID, DependencyType)
# DependencyType: /hard (must complete), /soft (preferred), /artifact (needs output)
Decl phase_dependency(PhaseID, DependsOnPhaseID, DependencyType).

# phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity)
# EstimatedComplexity: /low, /medium, /high, /critical
Decl phase_estimate(PhaseID, EstimatedTasks, EstimatedComplexity).

# -----------------------------------------------------------------------------
# 27.3 Task Granularity (Atomic Work Units)
# -----------------------------------------------------------------------------

# campaign_task(TaskID, PhaseID, Description, Status, TaskType)
# TaskType: /file_create, /file_modify, /test_write, /test_run, /research,
#           /shard_spawn, /tool_create, /verify, /document, /refactor, /integrate
# Status: /pending, /in_progress, /completed, /failed, /skipped, /blocked
Decl campaign_task(TaskID, PhaseID, Description, Status, TaskType).

# task_priority(TaskID, Priority)
# Priority: /critical, /high, /normal, /low
Decl task_priority(TaskID, Priority).

# task_dependency(TaskID, DependsOnTaskID)
Decl task_dependency(TaskID, DependsOnTaskID).

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

# shard_result(ShardID, ResultType, ResultData, Timestamp)
# ResultType: /success, /failure, /partial, /knowledge
Decl shard_result(ShardID, ResultType, ResultData, Timestamp).

# -----------------------------------------------------------------------------
# 27.8 Source Material Ingestion
# -----------------------------------------------------------------------------

# source_document(CampaignID, DocPath, DocType, ParsedAt)
# DocType: /spec, /requirements, /design, /readme, /api_doc, /tutorial
Decl source_document(CampaignID, DocPath, DocType, ParsedAt).

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

# replan_needed(CampaignID, Reason) - derived: campaign needs replanning
Decl replan_needed(CampaignID, Reason).

# =============================================================================
# SECTION 28: OUROBOROS / TOOL SELF-GENERATION (§8.3)
# =============================================================================
# Tool registry and lifecycle for self-generating tools

# tool_registered(ToolName, RegisteredAt) - tracks registered tools
Decl tool_registered(ToolName, RegisteredAt).

# registered_tool(ToolName, Command, ShardAffinity) - tool registration details
# ShardAffinity: /coder, /tester, /reviewer, /researcher, /generalist, /all
Decl registered_tool(ToolName, Command, ShardAffinity).

# tool_available(ToolName) - derived: tool is registered and ready
Decl tool_available(ToolName).

# tool_exists(ToolName) - derived: tool is in registry
Decl tool_exists(ToolName).

# tool_ready(ToolName) - derived: tool is compiled and ready
Decl tool_ready(ToolName).

# tool_hash(ToolName, Hash) - content hash for change detection
Decl tool_hash(ToolName, Hash).

# tool_capability(ToolName, Capability) - capabilities provided by a tool
Decl tool_capability(ToolName, Capability).

# capability_available(Capability) - derived: capability exists
Decl capability_available(Capability).

# Tool generation lifecycle
# tool_source_ready(ToolName) - source code has been generated
Decl tool_source_ready(ToolName).

# tool_safety_verified(ToolName) - passed safety checks
Decl tool_safety_verified(ToolName).

# tool_compiled(ToolName) - successfully compiled
Decl tool_compiled(ToolName).

# generation_state(ToolName, State) - current generation state
# State: /pending, /in_progress, /completed, /failed
Decl generation_state(ToolName, State).

# has_active_generation - helper for safe negation (true if any generation in progress)
Decl has_active_generation().

# is_tool_registered(ToolName) - helper for safe negation in tool registration check
Decl is_tool_registered(ToolName).

# missing_tool_for(Intent, Capability) - detected capability gap
Decl missing_tool_for(Intent, Capability).

# task_failure_reason(TaskID, ReasonType, Detail)
Decl task_failure_reason(TaskID, ReasonType, Detail).

# task_failure_count(Capability, Count) - tracks repeated failures
Decl task_failure_count(Capability, Count).

# tool_generation_blocked(Capability) - capability blocked from generation
Decl tool_generation_blocked(Capability).

# tool_lifecycle(ToolName, State) - tool lifecycle tracking
# State: /detected, /generating, /compiled, /deployed, /deprecated
Decl tool_lifecycle(ToolName, State).

# =============================================================================
# SECTION 29: TOOL LEARNING / REFINEMENT (Autopoiesis)
# =============================================================================
# Predicates for tool quality tracking and refinement

# refinement_state(ToolName, State) - tracks refinement lifecycle
# State: /idle, /in_progress, /completed, /failed
Decl refinement_state(ToolName, State).

# tool_known_issue(ToolName, IssueType) - known issues with a tool
# IssueType: /pagination, /incomplete, /rate_limit, /timeout
Decl tool_known_issue(ToolName, IssueType).

# issue_occurrence_count(ToolName, IssueType, Count) - how often issue occurs
Decl issue_occurrence_count(ToolName, IssueType, Count).

# capability_similar_to(Capability, SimilarCapability) - capability relationships
Decl capability_similar_to(Capability, SimilarCapability).

# tool_refined(ToolName, OldVersion, NewVersion) - refinement history
Decl tool_refined(ToolName, OldVersion, NewVersion).

# version_quality(ToolName, Version, QualityScore) - quality per version
Decl version_quality(ToolName, Version, QualityScore).

# tool_quality_poor(ToolName) - derived: tool has low quality
Decl tool_quality_poor(ToolName).

# refinement_count(ToolName, Count) - number of refinements attempted
Decl refinement_count(ToolName, Count).

# tool_learning(ToolName, Executions, SuccessRate, AvgQuality) - learning metrics
Decl tool_learning(ToolName, Executions, SuccessRate, AvgQuality).

# active_generation(ToolName) - tool is being generated
Decl active_generation(ToolName).

# Tool Execution Tracking (for VirtualStore integration)
# tool_executed(ToolName, Output) - tool was executed successfully
Decl tool_executed(ToolName, Output).

# tool_exec_success(ToolName) - marks successful tool execution
Decl tool_exec_success(ToolName).

# tool_exec_failed(ToolName, Reason) - marks failed tool execution
Decl tool_exec_failed(ToolName, Reason).

# tool_not_found(ToolName) - tool was requested but not in registry
Decl tool_not_found(ToolName).

# tool_execution_count(ToolName, Count) - total executions per tool
Decl tool_execution_count(ToolName, Count).

# tool_last_execution(ToolName, Timestamp) - last execution time
Decl tool_last_execution(ToolName, Timestamp).

# tool_quality_acceptable(ToolName) - derived: tool has acceptable quality
Decl tool_quality_acceptable(ToolName).

# tool_quality_good(ToolName) - derived: tool has good quality
Decl tool_quality_good(ToolName).

# tool_generation_hint(ToolName, Hint) - hints for tool generation
Decl tool_generation_hint(ToolName, Hint).

# escalate_to_user(Subject, Reason) - escalation needed for user decision
Decl escalate_to_user(Subject, Reason).

# =============================================================================
# SECTION 30: CODER SHARD HELPERS
# =============================================================================

# file_content(FilePath, Content) - cached file content
Decl file_content(FilePath, Content).

# coder_state(State) - current coder state
# State: /idle, /context_ready, /code_generated, /edit_applied, /build_passed, /build_failed
Decl coder_state(State).

# pending_edit(FilePath, Content) - pending edits
Decl pending_edit(FilePath, Content).

# coder_block_write(FilePath, Reason) - derived: write is blocked
Decl coder_block_write(FilePath, Reason).

# coder_safe_to_write(FilePath) - derived: safe to write
Decl coder_safe_to_write(FilePath).

# is_binary_file(FilePath) - file is binary (cannot edit)
Decl is_binary_file(FilePath).

# build_state(State) - current build state
# State: /passing, /failing, /unknown
Decl build_state(State).

# build_result(Success, Output) - result of build action (added for Bug 10)
Decl build_result(Success, Output).

# =============================================================================
# SECTION 31: REVIEWER SHARD HELPERS
# =============================================================================

# file_line_count(FilePath, LineCount) - line count per file
Decl file_line_count(FilePath, LineCount).

# finding_count(Severity, Count) - count of findings by severity
Decl finding_count(Severity, Count).

# style_rule(RuleID, RuleName, Threshold) - style rule definitions
Decl style_rule(RuleID, RuleName, Threshold).

# permission_denied(Action, Reason) - derived: why permission was denied (Bug 12)
Decl permission_denied(Action, Reason).

# checks_passed() - derived: positive confirmation of checks (Bug 10)
Decl checks_passed().

# safe_to_commit() - derived: all checks pass, safe to commit
Decl safe_to_commit().

# file_truncated(Path, MaxSize) - file content truncated (Bug 6)
Decl file_truncated(Path, MaxSize).

# =============================================================================
# SECTION 32: SAFE NEGATION HELPERS
# =============================================================================
# These helpers support safe negation patterns in Mangle rules

# has_block_commit() - helper: true if any block_commit exists
Decl has_block_commit().

# has_active_refinement() - helper: true if any refinement in progress
Decl has_active_refinement().

# has_eligible_phase() - helper: true if any phase is eligible
Decl has_eligible_phase().

# has_next_campaign_task() - helper: true if there's a next task
Decl has_next_campaign_task().

# has_in_progress_phase() - helper: true if any phase in progress
Decl has_in_progress_phase().

# has_incomplete_phase(CampaignID) - helper: campaign has incomplete phases
Decl has_incomplete_phase(CampaignID).

# has_incomplete_phase_task(PhaseID) - helper: phase has incomplete tasks
Decl has_incomplete_phase_task(PhaseID).

# has_incomplete_hard_dep(TaskID) - helper: task has incomplete hard dependencies
Decl has_incomplete_hard_dep(TaskID).

# has_earlier_phase(PhaseID) - helper: there are earlier phases to complete
Decl has_earlier_phase(PhaseID).

# has_earlier_task(PhaseID, TaskID) - helper: there are earlier tasks in phase
Decl has_earlier_task(PhaseID, TaskID).

# all_phase_tasks_complete(PhaseID) - derived: all tasks in phase are complete
Decl all_phase_tasks_complete(PhaseID).

# campaign_complete(CampaignID) - derived: entire campaign is complete
Decl campaign_complete(CampaignID).

# =============================================================================
# SECTION 33: SYSTEM SHARD COORDINATION
# =============================================================================
# Predicates for coordinating the 6 Type 1 system shards:
# - perception_firewall: NL → atoms transduction
# - world_model_ingestor: file_topology, symbol_graph maintenance
# - executive_policy: next_action derivation
# - constitution_gate: safety enforcement
# - tactile_router: action → tool routing
# - session_planner: agenda/campaign orchestration

# -----------------------------------------------------------------------------
# 33.1 System Shard Registry
# -----------------------------------------------------------------------------

# system_shard(ShardName, Type) - registered system shards with type
Decl system_shard(ShardName, Type).

# system_shard_state(ShardName, State) - current shard state
# State: /idle, /starting, /running, /stopping, /stopped, /error
Decl system_shard_state(ShardName, State).

# system_heartbeat(ShardName, Timestamp) - last heartbeat from shard
Decl system_heartbeat(ShardName, Timestamp).

# -----------------------------------------------------------------------------
# 33.2 Intent Processing Flow
# -----------------------------------------------------------------------------

# processed_intent(IntentID) - intent has been processed by perception
Decl processed_intent(IntentID).

# pending_intent(IntentID) - derived: intent waiting to be processed
Decl pending_intent(IntentID).

# -----------------------------------------------------------------------------
# 33.3 Action Flow (Executive → Constitution → Router)
# -----------------------------------------------------------------------------

# pending_action(ActionType, Target, Timestamp) - action awaiting permission check
Decl pending_action(ActionType, Target, Timestamp).

# pending_permission_check(ActionID) - derived: action needs constitution check
Decl pending_permission_check(ActionID).

# action_permitted(ActionID) - action passed constitution gate (derived from permission_check_result)
Decl action_permitted(ActionID).

# ready_for_routing(ActionID) - derived: action ready for router
Decl ready_for_routing(ActionID).

# exec_request(ToolName, Target, Timeout, CallID, Timestamp) - router output
Decl exec_request(ToolName, Target, Timeout, CallID, Timestamp).

# -----------------------------------------------------------------------------
# 33.4 Safety & Violations
# -----------------------------------------------------------------------------

# security_violation(ActionType, Reason, Timestamp) - blocked by constitution
Decl security_violation(ActionType, Reason, Timestamp).

# escalation_needed(Target, Subject, Reason) - needs human intervention
# Target: system component (e.g., /system_health, /session_planner, /ooda_loop)
# Subject: entity being escalated (e.g., ShardName, ItemID)
# Reason: why escalation is needed
Decl escalation_needed(Target, Subject, Reason).

# rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp)
Decl rule_proposal_pending(ShardName, MangleCode, Rationale, Confidence, Timestamp).

# -----------------------------------------------------------------------------
# 33.5 System Health
# -----------------------------------------------------------------------------

# system_shard_healthy(ShardName) - derived: shard heartbeat recent
Decl system_shard_healthy(ShardName).

# system_shard_unhealthy(ShardName) - derived: shard heartbeat stale
Decl system_shard_unhealthy(ShardName).

# world_model_heartbeat(ShardID, FileCount, Timestamp) - world model status
Decl world_model_heartbeat(ShardID, FileCount, Timestamp).

# session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp)
Decl session_planner_status(Total, Pending, InProgress, Completed, Blocked, Timestamp).

# plan_task(TaskID, Description, Status, ProgressPct) - individual task state
# Status: /pending, /in_progress, /completed, /blocked
Decl plan_task(TaskID, Description, Status, ProgressPct).

# plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct) - overall plan progress
Decl plan_progress(CampaignID, TotalTasks, CompletedTasks, ProgressPct).

# -----------------------------------------------------------------------------
# 33.6 Routing & Tool Management
# -----------------------------------------------------------------------------

# routing_error(ActionType, Reason, Timestamp) - router couldn't find handler
Decl routing_error(ActionType, Reason, Timestamp).

# route_added(ActionPattern, ToolName, Timestamp) - new route added via autopoiesis
Decl route_added(ActionPattern, ToolName, Timestamp).

# -----------------------------------------------------------------------------
# 33.7 Agenda & Planning
# -----------------------------------------------------------------------------

# agenda_item(ItemID, Description, Priority, Status, Timestamp)
Decl agenda_item(ItemID, Description, Priority, Status, Timestamp).

# session_checkpoint(CheckpointID, ItemsRemaining, Timestamp)
Decl session_checkpoint(CheckpointID, ItemsRemaining, Timestamp).

# task_completed(TaskID) - task marked complete
Decl task_completed(TaskID).

# task_blocked(TaskID) - task is blocked
Decl task_blocked(TaskID).

# -----------------------------------------------------------------------------
# 33.8 Perception Errors & Stats
# -----------------------------------------------------------------------------

# perception_error(Message, Timestamp) - perception shard error
Decl perception_error(Message, Timestamp).

# world_model_error(Message, Timestamp) - world model shard error
Decl world_model_error(Message, Timestamp).

# executive_error(Message, Timestamp) - executive shard error
Decl executive_error(Message, Timestamp).

# executive_trace(Action, FromRule, Rationale, Timestamp) - debug trace
Decl executive_trace(Action, FromRule, Rationale, Timestamp).

# strategy_activated(StrategyName, Timestamp) - strategy change
Decl strategy_activated(StrategyName, Timestamp).

# execution_blocked(Reason, Timestamp) - executive blocked
Decl execution_blocked(Reason, Timestamp).
Decl execution_blocked(Reason).  # 1-arg variant for simple blocks

# -----------------------------------------------------------------------------
# 33.8b Tactile Execution Audit Facts
# -----------------------------------------------------------------------------
# Facts generated by tactile/audit.go for execution event tracking

# execution_started(SessionID, RequestID, Binary, Timestamp) - command started
Decl execution_started(SessionID, RequestID, Binary, Timestamp).

# execution_command(RequestID, CommandString) - full command string
Decl execution_command(RequestID, CommandString).

# execution_working_dir(RequestID, WorkingDir) - working directory
Decl execution_working_dir(RequestID, WorkingDir).

# execution_completed(RequestID, ExitCode, DurationMs, Timestamp) - command finished
Decl execution_completed(RequestID, ExitCode, DurationMs, Timestamp).

# execution_output(RequestID, StdoutLen, StderrLen) - output lengths
Decl execution_output(RequestID, StdoutLen, StderrLen).

# execution_success(RequestID) - successful execution (exit code 0)
Decl execution_success(RequestID).

# execution_nonzero(RequestID, ExitCode) - non-zero exit code
Decl execution_nonzero(RequestID, ExitCode).

# execution_failure(RequestID, Error) - infrastructure failure
Decl execution_failure(RequestID, Error).

# execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes) - resource metrics
Decl execution_resource_usage(RequestID, CPUTimeMs, MemoryBytes).

# execution_io(RequestID, ReadBytes, WriteBytes) - I/O metrics
Decl execution_io(RequestID, ReadBytes, WriteBytes).

# execution_sandbox(RequestID, SandboxMode) - sandbox mode used
Decl execution_sandbox(RequestID, SandboxMode).

# execution_killed(RequestID, Reason, DurationMs) - command was killed
Decl execution_killed(RequestID, Reason, DurationMs).

# execution_error(RequestID, ErrorMessage) - execution error
Decl execution_error(RequestID, ErrorMessage).

# -----------------------------------------------------------------------------
# 33.9 Policy Derived Predicates (Section 21 Support)
# -----------------------------------------------------------------------------

# Intent processing derived predicates
Decl intent_processed(IntentID).
Decl focus_needs_resolution(Ref).
Decl intent_ready_for_executive(IntentID).

# Action flow derived predicates
Decl action_pending_permission(ActionID).
Decl permission_checked(ActionID).
Decl permission_check_result(ActionID, Result, Reason).
Decl action_blocked(ActionID, Reason).
Decl action_routed(ActionID).
Decl routing_result(ActionID, Result, Details).
Decl routing_succeeded(ActionID).
Decl routing_failed(ActionID, Error).

# Health monitoring derived predicates
Decl shard_heartbeat_stale(ShardName).

# Safety derived predicates
Decl block_all_actions(Reason).
Decl security_anomaly(AnomalyID, Type, Details).
Decl anomaly_investigated(AnomalyID).
Decl investigation_result(AnomalyID, Result).
Decl violation_count(Pattern, Count).
Decl propose_safety_rule(Pattern).
Decl repeated_violation_pattern(Pattern).
Decl safety_violation(ViolationID, Pattern, ActionType, Timestamp).

# World model derived predicates
Decl world_model_stale(File).
Decl file_in_project(File).
Decl symbol_reachable(From, To).

# Routing derived predicates
Decl routing_table(ActionType, Tool, RiskLevel).
Decl tool_allowlist(Tool, Timestamp).
Decl tool_allowed(Tool, ActionType).
Decl route_action(ActionID, Tool).
Decl action_type(ActionID, ActionType).
Decl routing_blocked(ActionID, Reason).
Decl has_tool_for_action(ActionType).

# Agenda/Planning derived predicates
Decl agenda_item_ready(ItemID).
Decl has_incomplete_dependency(ItemID).
Decl agenda_dependency(ItemID, DepID).
Decl next_agenda_item(ItemID).
Decl has_higher_priority_item(ItemID).
Decl checkpoint_due().
Decl last_checkpoint_time(Timestamp).
Decl agenda_item_escalate(ItemID, Reason).
Decl item_retry_count(ItemID, Count).

# Shard activation derived predicates
Decl activate_shard(ShardName).
Decl system_startup(ShardName, Mode).

# Autopoiesis derived predicates
Decl unhandled_case_count(ShardName, Count).
Decl unhandled_cases(ShardName, Cases).
Decl propose_new_rule(ShardName).
Decl proposed_rule(RuleID, ShardName, MangleCode, Confidence).
Decl rule_needs_approval(RuleID).
Decl auto_apply_rule(RuleID).
Decl rule_applied(RuleID).
Decl applied_rule(RuleID, Timestamp).
Decl learning_signal(SignalType, RuleID).
Decl learning_signal(SignalType).  # 1-arg variant for quality signals
Decl rule_outcome(RuleID, Outcome, Details).

# OODA loop derived predicates
Decl ooda_phase(Phase).
Decl has_next_action().
Decl current_ooda_phase(Phase).
Decl ooda_stalled(Reason).
Decl last_action_time(Timestamp).

# Builtin helper predicates (implemented in Go runtime)
Decl time_diff(Time1, Time2, Diff).
Decl list_length(List, Length).

# =============================================================================
# SECTION 34: CODE DOM (Interactive Code Elements)
# =============================================================================
# Analogous to Browser DOM, Code DOM projects code into semantic chunks
# (functions, structs, interfaces) with stable refs for querying and editing.
# Uses 1-hop dependency scope: active file + imports + files that import it.

# -----------------------------------------------------------------------------
# 34.1 File Scope Management
# -----------------------------------------------------------------------------

# active_file(Path) - the primary file being worked on
Decl active_file(Path).

# file_in_scope(Path, Hash, Language, LineCount) - files in current scope
# Language: /go, /python, /ts, /rust
Decl file_in_scope(Path, Hash, Language, LineCount).

# -----------------------------------------------------------------------------
# 34.2 Code Elements (Semantic Chunks)
# -----------------------------------------------------------------------------

# code_element(Ref, ElemType, File, StartLine, EndLine)
# Ref: stable reference like "fn:context.Compressor.Compress"
# ElemType: /function, /method, /struct, /interface, /type, /const, /var
Decl code_element(Ref, ElemType, File, StartLine, EndLine).

# element_signature(Ref, Signature) - declaration line
Decl element_signature(Ref, Signature).

# element_body(Ref, BodyText) - full text for display/editing
Decl element_body(Ref, BodyText).

# element_parent(Ref, ParentRef) - containment (method -> struct)
Decl element_parent(Ref, ParentRef).

# element_visibility(Ref, Visibility) - /public, /private
Decl element_visibility(Ref, Visibility).

# code_interactable(Ref, ActionType) - available actions per element
# ActionType: /view, /replace, /insert_before, /insert_after, /delete
Decl code_interactable(Ref, ActionType).

# -----------------------------------------------------------------------------
# 34.3 Edit Tracking
# -----------------------------------------------------------------------------

# element_modified(Ref, SessionID, Timestamp) - tracks element changes
Decl element_modified(Ref, SessionID, Timestamp).

# lines_edited(File, StartLine, EndLine, SessionID) - line-level tracking
Decl lines_edited(File, StartLine, EndLine, SessionID).

# lines_inserted(File, AfterLine, LineCount, SessionID) - insertions
Decl lines_inserted(File, AfterLine, LineCount, SessionID).

# lines_deleted(File, StartLine, EndLine, SessionID) - deletions
Decl lines_deleted(File, StartLine, EndLine, SessionID).

# file_read(Path, SessionID, Timestamp) - file access tracking
Decl file_read(Path, SessionID, Timestamp).

# file_written(Path, Hash, SessionID, Timestamp) - file write tracking
Decl file_written(Path, Hash, SessionID, Timestamp).

# -----------------------------------------------------------------------------
# 34.4 Code DOM Derived Predicates
# -----------------------------------------------------------------------------

# in_scope(File) - derived: file is in current scope
Decl in_scope(File).

# editable(Ref) - derived: element can be edited
Decl editable(Ref).

# function_in_scope(Ref, File, Sig) - derived: functions in scope
Decl function_in_scope(Ref, File, Sig).

# method_of(MethodRef, StructRef) - derived: method belongs to struct
Decl method_of(MethodRef, StructRef).

# code_contains(Parent, Child) - derived: transitive containment
Decl code_contains(Parent, Child).

# safe_to_modify(Ref) - derived: has tests, builds pass
Decl safe_to_modify(Ref).

# requires_campaign(Intent) - derived: complex refactor needs campaign
Decl requires_campaign(Intent).

# code_edit_outcome(Ref, EditType, Success) - edit result tracking
Decl code_edit_outcome(Ref, EditType, Success).

# proven_safe_edit(Ref, EditType) - derived: edit pattern is safe
Decl proven_safe_edit(Ref, EditType).

# -----------------------------------------------------------------------------
# 34.5 Error Handling & Edge Cases
# -----------------------------------------------------------------------------

# scope_open_failed(Path, Error) - file scope open failed
Decl scope_open_failed(Path, Error).

# scope_closed() - current scope was closed
Decl scope_closed().

# parse_error(File, Error, Timestamp) - Go AST parsing failed
Decl parse_error(File, Error, Timestamp).

# file_not_found(Path, Timestamp) - requested file doesn't exist
Decl file_not_found(Path, Timestamp).

# file_hash_mismatch(Path, ExpectedHash, ActualHash) - concurrent modification detected
Decl file_hash_mismatch(Path, ExpectedHash, ActualHash).

# element_stale(Ref, Reason) - element ref may be outdated
Decl element_stale(Ref, Reason).

# scope_refresh_failed(Path, Error) - re-parsing failed after edit
Decl scope_refresh_failed(Path, Error).

# encoding_issue(Path, IssueType) - file encoding problem detected
# IssueType: /bom_detected, /crlf_inconsistent, /non_utf8
Decl encoding_issue(Path, IssueType).

# large_file_warning(Path, LineCount, ByteSize) - file exceeds size thresholds
Decl large_file_warning(Path, LineCount, ByteSize).

# -----------------------------------------------------------------------------
# 34.6 Operation Tracking
# -----------------------------------------------------------------------------

# scope_operation(OpType, Path, Success, Timestamp) - scope operation audit
# OpType: /open, /refresh, /close
Decl scope_operation(OpType, Path, Success, Timestamp).

# edit_operation(OpType, Path, StartLine, EndLine, Success, Timestamp)
# OpType: /edit_lines, /insert_lines, /delete_lines, /replace_element
Decl edit_operation(OpType, Path, StartLine, EndLine, Success, Timestamp).

# undo_available(Path, OperationID) - undo is available for an operation
Decl undo_available(Path, OperationID).

# -----------------------------------------------------------------------------
# 34.7 Derived Predicates for Edge Cases
# -----------------------------------------------------------------------------

# file_modified_externally(Path) - derived: file changed outside of scope
Decl file_modified_externally(Path).

# needs_scope_refresh() - derived: scope is stale and needs refresh
Decl needs_scope_refresh().

# element_edit_blocked(Ref, Reason) - derived: edit is blocked
Decl element_edit_blocked(Ref, Reason).

# -----------------------------------------------------------------------------
# 34.8 Code Pattern Detection
# -----------------------------------------------------------------------------

# generated_code(File, Generator, Marker) - file is auto-generated
# Generator: /protobuf, /openapi, /swagger, /grpc, /wire, /ent, /sqlc, /gqlgen
# Marker: the comment/directive that indicates generation
Decl generated_code(File, Generator, Marker).

# api_client_function(Ref, Endpoint, Method) - function makes HTTP calls
# Method: /GET, /POST, /PUT, /DELETE, /PATCH
Decl api_client_function(Ref, Endpoint, Method).

# api_handler_function(Ref, Route, Method) - function handles HTTP requests
Decl api_handler_function(Ref, Route, Method).

# has_external_callers(Ref) - derived: function is called from outside package
Decl has_external_callers(Ref).

# breaking_change_risk(Ref, RiskLevel, Reason) - edit may break callers
# RiskLevel: /low, /medium, /high, /critical
Decl breaking_change_risk(Ref, RiskLevel, Reason).

# mock_file(TestFile, SourceFile) - test file mocks source file
Decl mock_file(TestFile, SourceFile).

# interface_impl(StructRef, InterfaceRef) - struct implements interface
Decl interface_impl(StructRef, InterfaceRef).

# cgo_code(File) - file contains CGo directives
Decl cgo_code(File).

# build_tag(File, Tag) - file has build constraints
Decl build_tag(File, Tag).

# embed_directive(File, EmbedPath) - file has go:embed
Decl embed_directive(File, EmbedPath).

# -----------------------------------------------------------------------------
# 34.9 Edit Safety Derived Predicates
# -----------------------------------------------------------------------------

# edit_unsafe(Ref, Reason) - derived: editing this element is risky
Decl edit_unsafe(Ref, Reason).

# suggest_update_mocks(Ref) - derived: mocks may need updating after edit
Decl suggest_update_mocks(Ref).

# signature_change_detected(Ref, OldSig, NewSig) - function signature changed
Decl signature_change_detected(Ref, OldSig, NewSig).

# requires_integration_test(Ref) - derived: API client needs integration test
Decl requires_integration_test(Ref).

# requires_contract_check(Ref) - derived: API handler contract validation needed
Decl requires_contract_check(Ref).

# api_edit_warning(Ref, Reason) - derived: warning when editing API code
Decl api_edit_warning(Ref, Reason).

# =============================================================================
# SECTION 34B: SHARD EXECUTION CONTEXT (Cross-Turn Propagation)
# =============================================================================
# These facts capture shard execution results and make them available to the
# main agent's context in subsequent turns. Solves the "lost context" problem
# where shard outputs were displayed but not persisted for later reference.

# -----------------------------------------------------------------------------
# 34B.1 Shard Execution Facts
# -----------------------------------------------------------------------------

# shard_executed(ShardID, ShardType, Task, Timestamp)
# Records that a shard was executed with a specific task
Decl shard_executed(ShardID, ShardType, Task, Timestamp).

# shard_output(ShardID, Output)
# The raw output from shard execution (may be truncated)
Decl shard_output(ShardID, Output).

# shard_success(ShardID)
# Marks successful shard execution
Decl shard_success(ShardID).

# shard_error(ShardID, ErrorMessage)
# Records shard execution failure
Decl shard_error(ShardID, ErrorMessage).

# -----------------------------------------------------------------------------
# 34B.2 Review Findings (from ReviewerShard)
# -----------------------------------------------------------------------------

# review_finding(ShardID, Severity, FilePath, Line, Message)
# Individual findings from code review
# Severity: /critical, /error, /warning, /info
Decl review_finding(ShardID, Severity, FilePath, Line, Message).

# review_summary(ShardID, Critical, Errors, Warnings, Info)
# Summary counts from a review execution
Decl review_summary(ShardID, Critical, Errors, Warnings, Info).

# review_metrics(ShardID, TotalLines, CodeLines, CommentLines, FunctionCount)
# Code metrics from review
Decl review_metrics(ShardID, TotalLines, CodeLines, CommentLines, FunctionCount).

# security_finding(ShardID, Severity, FilePath, Line, RuleID, Message)
# Security-specific findings
Decl security_finding(ShardID, Severity, FilePath, Line, RuleID, Message).

# -----------------------------------------------------------------------------
# 34B.3 Test Results (from TesterShard)
# -----------------------------------------------------------------------------

# test_result(ShardID, TestName, Passed, Duration)
# Individual test results
Decl test_result(ShardID, TestName, Passed, Duration).

# test_summary(ShardID, Total, Passed, Failed, Skipped)
# Summary of test execution
Decl test_summary(ShardID, Total, Passed, Failed, Skipped).

# -----------------------------------------------------------------------------
# 34B.4 Recent Shard Context (for LLM injection)
# -----------------------------------------------------------------------------

# recent_shard_context(ShardType, Task, Summary, Timestamp)
# Compressed context from recent shard executions for LLM injection
Decl recent_shard_context(ShardType, Task, Summary, Timestamp).

# last_shard_execution(ShardID, ShardType, Task)
# The most recent shard execution (for quick reference)
Decl last_shard_execution(ShardID, ShardType, Task).

# -----------------------------------------------------------------------------
# 34B.5 Derived Predicates
# -----------------------------------------------------------------------------

# has_recent_shard_output(ShardType) - derived: there's recent output from this shard type
Decl has_recent_shard_output(ShardType).

# shard_findings_available() - derived: there are findings to reference
Decl shard_findings_available().

# =============================================================================
# SECTION 35: VERIFICATION LOOP (Post-Execution Quality Enforcement)
# =============================================================================
# Tracks task verification attempts, quality violations, and corrective actions.
# Enables the agent to retry with context enrichment until success or escalation.

# -----------------------------------------------------------------------------
# 35.1 Verification State Tracking
# -----------------------------------------------------------------------------

# verification_attempt(TaskID, AttemptNum, Success)
# Tracks each verification attempt for a task
# Success: /success, /failure
Decl verification_attempt(TaskID, AttemptNum, Success).

# current_task(TaskID) - the task currently being executed
Decl current_task(TaskID).

# verification_result(TaskID, AttemptNum, Confidence, Reason)
# Detailed verification result per attempt
Decl verification_result(TaskID, AttemptNum, Confidence, Reason).

# -----------------------------------------------------------------------------
# 35.2 Quality Violation Detection
# -----------------------------------------------------------------------------

# quality_violation(TaskID, ViolationType)
# ViolationType: /mock_code, /placeholder, /hallucinated_api, /incomplete,
#                /hardcoded, /empty_function, /missing_errors, /fake_tests
Decl quality_violation(TaskID, ViolationType).

# quality_violation_evidence(TaskID, ViolationType, Evidence)
# Specific evidence of the violation (e.g., line number, code snippet)
Decl quality_violation_evidence(TaskID, ViolationType, Evidence).

# quality_score(TaskID, AttemptNum, Score)
# Overall quality score (0.0-1.0) for the attempt
Decl quality_score(TaskID, AttemptNum, Score).

# -----------------------------------------------------------------------------
# 35.3 Corrective Action Tracking
# -----------------------------------------------------------------------------

# corrective_action_taken(TaskID, ActionType)
# ActionType: /research, /docs, /tool, /decompose
Decl corrective_action_taken(TaskID, ActionType).

# corrective_context(TaskID, AttemptNum, ContextType, Context)
# Additional context gathered through corrective action
# ContextType: /research_result, /documentation, /tool_output, /decomposition
Decl corrective_context(TaskID, AttemptNum, ContextType, Context).

# corrective_query(TaskID, AttemptNum, Query)
# The query used for corrective action (e.g., research query)
Decl corrective_query(TaskID, AttemptNum, Query).

# -----------------------------------------------------------------------------
# 35.4 Shard Selection Tracking
# -----------------------------------------------------------------------------

# shard_selected(TaskID, AttemptNum, ShardType, SelectionReason)
# Tracks which shard was selected for each attempt
Decl shard_selected(TaskID, AttemptNum, ShardType, SelectionReason).

# shard_selection_confidence(TaskID, AttemptNum, ShardType, Confidence)
# Confidence score for shard selection
Decl shard_selection_confidence(TaskID, AttemptNum, ShardType, Confidence).

# -----------------------------------------------------------------------------
# 35.5 Verification Derived Predicates
# -----------------------------------------------------------------------------

# verification_blocked(TaskID) - derived: max retries reached
Decl verification_blocked(TaskID).

# verification_succeeded(TaskID) - derived: task passed verification
Decl verification_succeeded(TaskID).

# has_quality_violation(TaskID) - derived: task has any quality violation
Decl has_quality_violation(TaskID).

# needs_corrective_action(TaskID) - derived: task needs correction
Decl needs_corrective_action(TaskID).

# escalation_required(TaskID, Reason) - derived: must escalate to user
Decl escalation_required(TaskID, Reason).

# =============================================================================
# SECTION 36: REASONING TRACES (Shard LLM Interaction History)
# =============================================================================
# Captures LLM interactions from all 4 shard types for self-learning,
# main agent oversight, and cross-shard learning via Mangle rules.

# -----------------------------------------------------------------------------
# 36.1 Core Trace Facts
# -----------------------------------------------------------------------------

# reasoning_trace(TraceID, ShardType, ShardCategory, SessionID, Success, DurationMs)
# Summary of a reasoning trace for policy decisions
# ShardCategory: /system, /ephemeral, /specialist
Decl reasoning_trace(TraceID, ShardType, ShardCategory, SessionID, Success, DurationMs).

# trace_quality(TraceID, Score)
# Quality score assigned after analysis (0.0-1.0)
Decl trace_quality(TraceID, Score).

# trace_error(TraceID, ErrorType)
# Error categorization for learning
Decl trace_error(TraceID, ErrorType).

# trace_task_type(TraceID, TaskType)
# Task type classification for pattern matching
Decl trace_task_type(TraceID, TaskType).

# -----------------------------------------------------------------------------
# 36.2 Shard Performance Patterns
# -----------------------------------------------------------------------------

# shard_reasoning_pattern(ShardType, PatternType, Frequency)
# Detected patterns in shard reasoning (for learning)
# PatternType: /success_pattern, /failure_pattern, /slow_reasoning, /quality_issue
Decl shard_reasoning_pattern(ShardType, PatternType, Frequency).

# trace_insight(TraceID, InsightType, Insight)
# Extracted insights from trace analysis
# InsightType: /approach, /error_pattern, /optimization, /quality_note
Decl trace_insight(TraceID, InsightType, Insight).

# shard_performance(ShardType, SuccessRate, AvgDurationMs, TraceCount)
# Aggregate performance metrics per shard type
Decl shard_performance(ShardType, SuccessRate, AvgDurationMs, TraceCount).

# -----------------------------------------------------------------------------
# 36.3 Cross-Shard Learning
# -----------------------------------------------------------------------------

# specialist_outperforms(SpecialistName, TaskType)
# Tracks when specialists outperform ephemeral shards
Decl specialist_outperforms(SpecialistName, TaskType).

# shard_can_handle(ShardType, TaskType)
# Capability mapping based on trace history
Decl shard_can_handle(ShardType, TaskType).

# shard_switch_suggestion(TaskType, FromShard, ToShard)
# Suggested shard switches based on performance data
Decl shard_switch_suggestion(TaskType, FromShard, ToShard).

# -----------------------------------------------------------------------------
# 36.4 Derived Predicates for Trace Analysis
# -----------------------------------------------------------------------------

# low_quality_trace(TraceID) - derived: trace quality < 0.5
Decl low_quality_trace(TraceID).

# high_quality_trace(TraceID) - derived: trace quality >= 0.8
Decl high_quality_trace(TraceID).

# shard_struggling(ShardType) - derived: shard has high failure rate
Decl shard_struggling(ShardType).

# shard_performing_well(ShardType) - derived: shard has high success rate
Decl shard_performing_well(ShardType).

# slow_reasoning_detected(ShardType) - derived: average duration > threshold
Decl slow_reasoning_detected(ShardType).

# learning_from_traces(SignalType, ShardType) - derived: learning signals
# SignalType: /avoid_pattern, /success_pattern, /shard_needs_help
Decl learning_from_traces(SignalType, ShardType).

# suggest_use_specialist(TaskType, SpecialistName) - derived: use specialist
Decl suggest_use_specialist(TaskType, SpecialistName).

# specialist_recommended(ShardName, FilePath, Confidence) - reviewer output
# Emitted when reviewer detects technology patterns matching a specialist shard
Decl specialist_recommended(ShardName, FilePath, Confidence).

# -----------------------------------------------------------------------------
# 36.5 Virtual Predicate for Trace Queries
# -----------------------------------------------------------------------------

# query_traces(ShardType, Limit, TraceID, Success, DurationMs)
# Queries reasoning_traces table via VirtualStore FFI
Decl query_traces(ShardType, Limit, TraceID, Success, DurationMs).

# query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration)
# Retrieves aggregate stats for a shard type
Decl query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration).
