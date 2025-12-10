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

# directory(Path, Name)
Decl directory(Path, Name).

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

# action_mapping(IntentVerb, ActionType) - maps intent verbs to executable actions
# IntentVerb: /explain, /read, /search, /run, /test, /review, /fix, /refactor, etc.
# ActionType: /analyze_code, /fs_read, /search_files, /exec_cmd, /run_tests, etc.
Decl action_mapping(IntentVerb, ActionType).

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
# Confidence: LLM's confidence in the plan (0-100 integer scale)
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

# shard_result(ShardID, ResultType, ResultData, Timestamp)
# ResultType: /success, /failure, /partial, /knowledge
Decl shard_result(ShardID, ResultType, ResultData, Timestamp).

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

# tool_description(ToolName, Description) - human-readable description of what tool does
Decl tool_description(ToolName, Description).

# tool_binary_path(ToolName, BinaryPath) - path to compiled binary
Decl tool_binary_path(ToolName, BinaryPath).

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

# -----------------------------------------------------------------------------
# 28.2 Ouroboros Derived Predicates (from policy.mg)
# -----------------------------------------------------------------------------

# explicit_tool_request(Capability) - user explicitly requested tool generation
Decl explicit_tool_request(Capability).

# capability_gap_detected(Capability) - repeated failures suggest missing capability
Decl capability_gap_detected(Capability).

# tool_generation_permitted(Capability) - tool generation passes safety checks
Decl tool_generation_permitted(Capability).

# dangerous_capability(Capability) - capabilities that should never be auto-generated
# e.g., /exec_arbitrary, /network_unconstrained, /system_admin, /credential_access
Decl dangerous_capability(Capability).

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

# tool_needs_refinement(ToolName) - derived: tool quality is poor and needs refinement
Decl tool_needs_refinement(ToolName).

# active_refinement(ToolName) - tool is currently being refined
Decl active_refinement(ToolName).

# learning_pattern_detected(ToolName, IssueType) - recurring issue pattern found
Decl learning_pattern_detected(ToolName, IssueType).

# refinement_effective(ToolName) - derived: refinement improved tool quality
Decl refinement_effective(ToolName).

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

# -----------------------------------------------------------------------------
# 32.2 Campaign Derived Predicates (from policy.mg Section 19)
# -----------------------------------------------------------------------------

# failed_campaign_task(CampaignID, TaskID) - derived: task failed during campaign
Decl failed_campaign_task(CampaignID, TaskID).

# priority_higher(PriorityA, PriorityB) - priority ordering helper
# Returns true if PriorityA is higher than PriorityB
Decl priority_higher(PriorityA, PriorityB).

# has_blocking_task_dep(TaskID) - helper: task has incomplete blocking dependencies
Decl has_blocking_task_dep(TaskID).

# task_conflict_active(TaskID) - helper: task conflicts with an in-progress task
Decl task_conflict_active(TaskID).

# has_passed_checkpoint(PhaseID, CheckType) - helper: phase has passed checkpoint
Decl has_passed_checkpoint(PhaseID, CheckType).

# phase_success_pattern(PhaseType) - tracks successful phase types for learning
Decl phase_success_pattern(PhaseType).

# phase_tool_permitted(Tool) - derived: tool is permitted in current phase profile
Decl phase_tool_permitted(Tool).

# tool_advisory_block(Tool, Reason) - advisory: tool not in phase profile
Decl tool_advisory_block(Tool, Reason).

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

# Builtin helper predicates
# Note: time_diff removed - use fn:minus(Now, Timestamp) inline in rules instead.
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

# code_edit_outcome(Ref, EditType, Success, Timestamp) - edit result tracking
Decl code_edit_outcome(Ref, EditType, Success, Timestamp).

# proven_safe_edit(Ref, EditType) - derived: edit pattern is safe
Decl proven_safe_edit(Ref, EditType).

# method_in_scope(Ref, File, Sig) - derived: methods in scope
Decl method_in_scope(Ref, File, Sig).

# scope_refreshed(File) - helper: file scope has been refreshed
Decl scope_refreshed(File).

# successful_edit(Ref, EditType) - derived: edit succeeded
Decl successful_edit(Ref, EditType).

# failed_edit(Ref, EditType) - derived: edit failed
Decl failed_edit(Ref, EditType).

# element_count_high() - helper: many elements in scope (triggers campaign for complex refactors)
Decl element_count_high().

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
# SECTION 37: HOLOGRAPHIC CODE GRAPH (Cartographer)
# =============================================================================
# Rich structural facts extracted by Cartographer (§NextGen-1)

# code_defines(File, Symbol, Type, StartLine, EndLine)
# Type: /function, /struct, /interface, /type
Decl code_defines(File, Symbol, Type, StartLine, EndLine).

# code_calls(Caller, Callee)
# Represents dynamic call graph
Decl code_calls(Caller, Callee).

# code_implements(Struct, Interface)
# Represents structural typing relationships
Decl code_implements(Struct, Interface).

# relevant_context(Content) - derived: content relevant to current intent target
# Used by Holographic Retrieval (Cartographer) for X-Ray Vision
Decl relevant_context(Content).

# =============================================================================
# SECTION 38: SPECULATIVE DREAMER (Precognition Layer)
# =============================================================================
# Projected facts produced by the Dreamer to simulate action effects.

# projected_action(ActionID, ActionType, Target)
Decl projected_action(ActionID, ActionType, Target).

# projected_fact(ActionID, FactType, Value)
# FactType: /file_missing, /file_exists, /modified
Decl projected_fact(ActionID, FactType, Value).

# panic_state(ActionID, Reason) - Derived: future state violates invariant
Decl panic_state(ActionID, Reason).

# dream_block(ActionID, Reason) - Derived: action blocked by Dreamer
Decl dream_block(ActionID, Reason).

# critical_file(Path) - Enumerates files whose deletion is catastrophic
Decl critical_file(Path).

# critical_path_prefix(Prefix) - Paths that should never be removed recursively
Decl critical_path_prefix(Prefix).

# =============================================================================
# SECTION 39: EXTENDED METRICS (Aggregation)
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

# first_attempt_success(TaskID) - derived: task succeeded on first verification attempt
Decl first_attempt_success(TaskID).

# required_retry(TaskID) - derived: task required retries before passing
Decl required_retry(TaskID).

# violation_type_count_high(ViolationType) - derived: violation type occurs frequently (5+)
Decl violation_type_count_high(ViolationType).

# corrective_action_effective(TaskID, ActionType) - derived: corrective action improved result
Decl corrective_action_effective(TaskID, ActionType).

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

# low_quality_trace(TraceID) - derived: trace quality < 50 (on 0-100 scale)
Decl low_quality_trace(TraceID).

# high_quality_trace(TraceID) - derived: trace quality >= 80 (on 0-100 scale)
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

# =============================================================================
# SECTION 41: MISSING DECLARATIONS (Cross-Module Support)
# =============================================================================
# Predicates used by policy.mg rules or Go code that were previously undeclared.

# -----------------------------------------------------------------------------
# 41.1 Policy Helper Predicates
# -----------------------------------------------------------------------------

# has_projection_violation(ActionID) - helper for safe negation in shadow mode
Decl has_projection_violation(ActionID).

# is_mutation_approved(MutationID) - helper for safe negation in diff approval
Decl is_mutation_approved(MutationID).

# has_pending_checkpoint(PhaseID) - helper for checkpoint verification
Decl has_pending_checkpoint(PhaseID).

# action_ready_for_routing(ActionID) - derived: action ready for tactile router
Decl action_ready_for_routing(ActionID).

# -----------------------------------------------------------------------------
# 41.2 Shard Configuration Predicates
# -----------------------------------------------------------------------------

# shard_type(Type, Lifecycle, Characteristic) - shard taxonomy configuration
# Type: /system, /ephemeral, /persistent, /user
# Lifecycle: /permanent, /spawn_die, /long_running, /explicit
# Characteristic: /high_reliability, /speed_optimized, /adaptive, /user_defined
Decl shard_type(Type, Lifecycle, Characteristic).

# shard_model_config(ShardType, ModelType) - model capability mapping for shards
# ShardType: /system, /ephemeral, /persistent, /user
# ModelType: /high_reasoning, /high_speed, /balanced
Decl shard_model_config(ShardType, ModelType).

# -----------------------------------------------------------------------------
# 41.3 Perception / Taxonomy Predicates
# -----------------------------------------------------------------------------

# NOTE: context_token(Token) is declared in inference.mg

# user_input_string(Input) - raw user input string for NL processing
Decl user_input_string(Input).

# is_relevant(Path) - derived: path is relevant to current campaign/intent
Decl is_relevant(Path).

# -----------------------------------------------------------------------------
# 41.4 Reviewer Shard Predicates
# -----------------------------------------------------------------------------

# active_finding(File, Line, Severity, Category, RuleID, Message)
# Filtered findings after Mangle rules suppress noisy or irrelevant entries
Decl active_finding(File, Line, Severity, Category, RuleID, Message).

# raw_finding(File, Line, Severity, Category, RuleID, Message)
# Unfiltered findings from static analysis before Mangle processing
Decl raw_finding(File, Line, Severity, Category, RuleID, Message).

# -----------------------------------------------------------------------------
# 41.5 Tool Generator / Ouroboros Predicates
# -----------------------------------------------------------------------------

# tool_generated(ToolName, Timestamp) - successfully generated tool
Decl tool_generated(ToolName, Timestamp).

# tool_trace(ToolName, TraceID) - reasoning trace for tool generation
Decl tool_trace(ToolName, TraceID).

# tool_generation_failed(ToolName, ErrorMessage) - tool generation failure record
Decl tool_generation_failed(ToolName, ErrorMessage).

# tool_issue_pattern(ToolName, IssueType, Occurrences, Confidence)
# Detected patterns from tool learning (pagination, incomplete, rate_limit, timeout)
Decl tool_issue_pattern(ToolName, IssueType, Occurrences, Confidence).

# -----------------------------------------------------------------------------
# 41.6 Campaign / Requirement Predicates
# -----------------------------------------------------------------------------

# requirement_task_link(RequirementID, TaskID, Strength)
# Links requirements to tasks that fulfill them with strength score
Decl requirement_task_link(RequirementID, TaskID, Strength).

# -----------------------------------------------------------------------------
# 41.7 Git Context Predicates (Chesterton's Fence)
# -----------------------------------------------------------------------------

# git_branch(Branch) - current git branch name
Decl git_branch(Branch).

# recent_commit(Hash, Message, Author, Timestamp)
# Recent commit history for Chesterton's Fence analysis
Decl recent_commit(Hash, Message, Author, Timestamp).

# -----------------------------------------------------------------------------
# 41.8 Test State Predicates
# -----------------------------------------------------------------------------

# failing_test(TestName, ErrorMessage) - details of failing tests
Decl failing_test(TestName, ErrorMessage).

# -----------------------------------------------------------------------------
# 41.9 Constitutional Safety Predicates
# -----------------------------------------------------------------------------

# blocked_action(Action) - action blocked by constitutional rules
Decl blocked_action(Action).

# safety_warning(Warning) - active safety concern/warning message
Decl safety_warning(Warning).

# -----------------------------------------------------------------------------
# 41.10 Execution & Context Predicates
# -----------------------------------------------------------------------------

# execution_result(Success, Output) - result of command execution
# Success: /true, /false
Decl execution_result(Success, Output).

# context_to_inject(Fact) - derived: facts selected for LLM context injection
Decl context_to_inject(Fact).

# final_system_prompt(Prompt) - derived: assembled system prompt for LLM
Decl final_system_prompt(Prompt).

# -----------------------------------------------------------------------------
# 41.11 Recursive Helper Predicates
# -----------------------------------------------------------------------------

# parent(Child, Parent) - direct parent-child relationship (recursive base case)
Decl parent(Child, Parent).

# ancestor(Descendant, Ancestor) - transitive ancestor relationship (recursive closure)
Decl ancestor(Descendant, Ancestor).

# =============================================================================
# SECTION 40: INTELLIGENT TOOL ROUTING (§40)
# =============================================================================
# Predicates for smart tool-to-shard routing based on capabilities, intent,
# domain matching, and usage history. Enables context-window-aware tool injection.

# -----------------------------------------------------------------------------
# 40.1 Tool Capability Categories
# -----------------------------------------------------------------------------
# Categories: /validation, /generation, /inspection, /transformation,
#             /analysis, /execution, /knowledge, /debugging, /general

# tool_domain(ToolName, Domain) - tool's primary domain
# Domains: /go, /python, /mangle, /filesystem, /git, /testing, /build, /web
Decl tool_domain(ToolName, Domain).

# tool_usage_stats(ToolName, ExecuteCount, SuccessCount, LastUsed)
# Tracks tool execution history for learning-based prioritization
Decl tool_usage_stats(ToolName, ExecuteCount, SuccessCount, LastUsed).

# tool_priority_score(ToolName, Score)
# Derived score 0.0-1.0 based on combined relevance factors
Decl tool_priority_score(ToolName, Score).

# -----------------------------------------------------------------------------
# 40.2 Shard-Tool Affinity Mapping
# -----------------------------------------------------------------------------

# shard_capability_affinity(ShardType, CapabilityCategory, AffinityScore)
# Score 0-100 (integer) indicating how relevant a capability category is to a shard type
# NOTE: Must use integers because Mangle comparison operators don't support floats
# ShardType: /coder, /tester, /reviewer, /researcher, /generalist
# CapabilityCategory: /validation, /generation, /inspection, /transformation,
#                     /analysis, /execution, /knowledge, /debugging
Decl shard_capability_affinity(ShardType, CapabilityCategory, AffinityScore).

# current_shard_type(ShardType) - the shard type being configured
# Used for context during tool routing derivation
Decl current_shard_type(ShardType).

# -----------------------------------------------------------------------------
# 40.3 Intent-Capability Mapping
# -----------------------------------------------------------------------------

# intent_requires_capability(IntentVerb, CapabilityCategory, Weight)
# Maps user intent verbs to required tool capabilities with importance weights
# IntentVerb: /implement, /refactor, /fix, /test, /review, /explain, /research, /explore
# Weight: 0.0-1.0 (higher = more important)
Decl intent_requires_capability(IntentVerb, CapabilityCategory, Weight).

# current_intent(IntentID) - the active intent for routing context
Decl current_intent(IntentID).

# -----------------------------------------------------------------------------
# 40.4 Tool Routing Derived Predicates
# -----------------------------------------------------------------------------

# tool_base_relevance(ShardType, ToolName, Score)
# Base relevance from shard-capability affinity
Decl tool_base_relevance(ShardType, ToolName, Score).

# tool_intent_relevance(ToolName, Score)
# Boost from matching current intent's required capabilities
Decl tool_intent_relevance(ToolName, Score).

# tool_domain_relevance(ToolName, Score)
# Boost from matching target file's language/domain
Decl tool_domain_relevance(ToolName, Score).

# tool_success_relevance(ToolName, Score)
# Boost based on historical success rate
Decl tool_success_relevance(ToolName, Score).

# tool_recency_relevance(ToolName, Score)
# Boost for recently used tools (likely still relevant)
Decl tool_recency_relevance(ToolName, Score).

# tool_combined_score(ShardType, ToolName, TotalScore)
# Weighted combination of all relevance factors
Decl tool_combined_score(ShardType, ToolName, TotalScore).

# relevant_tool(ShardType, ToolName)
# Derived: tool is relevant for this shard type (above threshold)
Decl relevant_tool(ShardType, ToolName).

# tool_priority_rank(ShardType, ToolName, Rank)
# Integer rank for ordering (higher = more relevant)
Decl tool_priority_rank(ShardType, ToolName, Rank).

# -----------------------------------------------------------------------------
# 40.5 Tool Execution Tracking (for learning feedback)
# -----------------------------------------------------------------------------

# tool_execution(ToolName, Success, Timestamp)
# Individual execution record for aggregation
# Success: /true, /false
Decl tool_execution(ToolName, Success, Timestamp).

# -----------------------------------------------------------------------------
# 40.6 Helper Predicates for Safe Negation
# -----------------------------------------------------------------------------

# has_current_intent() - true if any current intent exists
Decl has_current_intent().

# has_tool_domain(ToolName) - true if tool has a domain specified
Decl has_tool_domain(ToolName).

# has_tool_usage(ToolName) - true if tool has usage stats
Decl has_tool_usage(ToolName).

# -----------------------------------------------------------------------------
# 36.5 Virtual Predicate for Trace Queries
# -----------------------------------------------------------------------------

# query_traces(ShardType, Limit, TraceID, Success, DurationMs)
# Queries reasoning_traces table via VirtualStore FFI
Decl query_traces(ShardType, Limit, TraceID, Success, DurationMs).

# query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration)
# Retrieves aggregate stats for a shard type
Decl query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration).

# -----------------------------------------------------------------------------
# 41.12 Reviewer Feedback Loop Predicates (Self-Correction)
# -----------------------------------------------------------------------------

# review_finding(ReviewID, File, Line, Severity, Category, Message)
# A finding from a specific review session
Decl review_finding(ReviewID, File, Line, Severity, Category, Message).

# user_rejected_finding(ReviewID, File, Line, Reason, Timestamp)
# User explicitly rejected a finding as incorrect
Decl user_rejected_finding(ReviewID, File, Line, Reason, Timestamp).

# user_accepted_finding(ReviewID, File, Line, Timestamp)
# User explicitly accepted a finding (applied the suggestion)
Decl user_accepted_finding(ReviewID, File, Line, Timestamp).

# review_accuracy(ReviewID, TotalFindings, Accepted, Rejected, Score)
# Computed accuracy score for a review session
Decl review_accuracy(ReviewID, TotalFindings, Accepted, Rejected, Score).

# false_positive_pattern(Pattern, Category, Occurrences, Confidence)
# Learned patterns that cause false positives
Decl false_positive_pattern(Pattern, Category, Occurrences, Confidence).

# review_suspect(ReviewID, Reason)
# Derived: Review flagged as potentially inaccurate
Decl review_suspect(ReviewID, Reason).

# reviewer_needs_validation(ReviewID)
# Derived: This review should be spot-checked by main agent
Decl reviewer_needs_validation(ReviewID).

# =============================================================================
# MULTI-SHARD REVIEW ORCHESTRATION (§20.1)
# =============================================================================
# Schemas for tracking orchestrated multi-shard reviews where multiple
# specialist agents review code in parallel.

# multi_shard_review(ReviewID, Target, Participants, IsComplete, TotalFindings, Timestamp)
# Main record for a multi-shard orchestrated review
Decl multi_shard_review(ReviewID, Target, Participants, IsComplete, TotalFindings, Timestamp).

# multi_review_participant(ReviewID, ShardName, FileCount, FindingCount)
# Tracks which specialists participated in a review
Decl multi_review_participant(ReviewID, ShardName, FileCount, FindingCount).

# multi_review_finding(ReviewID, ShardName, FilePath, Line, Severity, Message)
# Individual findings from a multi-shard review, attributed to source shard
Decl multi_review_finding(ReviewID, ShardName, FilePath, Line, Severity, Message).

# cross_shard_insight(ReviewID, InsightType, Description)
# Holistic insights derived from cross-shard analysis
# InsightType: /hot_spot, /pattern, /critical_attention, /cross_domain
Decl cross_shard_insight(ReviewID, InsightType, Description).

# review_insight(Index, Insight)
# Individual review insights stored for learning/retrieval
Decl review_insight(Index, Insight).

# specialist_match(ReviewID, AgentName, Score, Reason)
# Records which specialists were matched for a review and why
Decl specialist_match(ReviewID, AgentName, Score, Reason).

# symbol_verified_exists(Symbol, File, VerifiedAt)
# Symbol was verified to exist (counters false "undefined" claims)
Decl symbol_verified_exists(Symbol, File, VerifiedAt).

# =============================================================================
# SECTION 42: DYNAMIC PROMPT COMPOSITION (Context Injection)
# =============================================================================
# These predicates enable kernel-driven system prompt assembly.
# The articulation layer queries these to build dynamic prompts for shards.

# -----------------------------------------------------------------------------
# 42.1 Base Prompt Templates
# -----------------------------------------------------------------------------

# shard_prompt_base(ShardType, BaseTemplate)
# Base template for each shard type (Type A, B, S, U)
# ShardType: /system, /ephemeral, /persistent, /user
Decl shard_prompt_base(ShardType, BaseTemplate).

# -----------------------------------------------------------------------------
# 42.2 Context Atom Selection (Spreading Activation Output)
# -----------------------------------------------------------------------------

# shard_context_atom(ShardID, Atom, Relevance)
# Context atoms selected for injection into prompts (spreading activation output)
# Relevance: 0.0-1.0 score indicating how relevant the atom is
# NOTE: Named shard_context_atom to distinguish from existing context_atom(Fact) in Section 12
Decl shard_context_atom(ShardID, Atom, Relevance).

# -----------------------------------------------------------------------------
# 42.3 Specialist Knowledge (Type B Persistent Shards)
# -----------------------------------------------------------------------------

# specialist_knowledge(ShardID, Topic, Content)
# Specialist knowledge for Type B persistent shards
# Topic: domain identifier (e.g., /go_concurrency, /react_hooks, /sql_optimization)
Decl specialist_knowledge(ShardID, Topic, Content).

# -----------------------------------------------------------------------------
# 42.4 Session-Level Customizations
# -----------------------------------------------------------------------------

# prompt_customization(SessionID, Key, Value)
# Session-level prompt customizations (user preferences)
# Key: customization key (e.g., /verbosity, /tone, /detail_level)
Decl prompt_customization(SessionID, Key, Value).

# -----------------------------------------------------------------------------
# 42.5 Campaign-Specific Constraints
# -----------------------------------------------------------------------------

# campaign_prompt_policy(CampaignID, ShardType, Constraint)
# Campaign-specific prompt constraints
# Constraint: rule or limitation to apply (e.g., "no external APIs", "strict typing")
Decl campaign_prompt_policy(CampaignID, ShardType, Constraint).

# -----------------------------------------------------------------------------
# 42.6 Learned Exemplars
# -----------------------------------------------------------------------------

# prompt_exemplar(ShardType, Category, Exemplar)
# Learned exemplars that should influence prompts
# Category: exemplar category (e.g., /code_style, /error_handling, /documentation)
# Exemplar: the learned example pattern or template
Decl prompt_exemplar(ShardType, Category, Exemplar).

# -----------------------------------------------------------------------------
# 42.7 Derived Predicates for Prompt Assembly
# -----------------------------------------------------------------------------

# prompt_ready(ShardID) - derived: all required prompt components are available
Decl prompt_ready(ShardID).

# has_specialist_knowledge(ShardID) - helper: shard has specialist knowledge loaded
Decl has_specialist_knowledge(ShardID).

# has_campaign_constraints(CampaignID, ShardType) - helper: campaign has constraints for shard type
Decl has_campaign_constraints(CampaignID, ShardType).

# active_prompt_customization(Key, Value) - derived: active customization for current session
Decl active_prompt_customization(Key, Value).

# prompt_context_budget(ShardID, TokensUsed, TokensAvailable) - context window tracking
Decl prompt_context_budget(ShardID, TokensUsed, TokensAvailable).

# context_overflow(ShardID) - derived: context exceeds available budget
Decl context_overflow(ShardID).

# -----------------------------------------------------------------------------
# 42.8 Active Shard Tracking
# -----------------------------------------------------------------------------

# active_shard(ShardID, ShardType) - currently active shard being configured
Decl active_shard(ShardID, ShardType).

# shard_family(ShardID, Family) - shard belongs to a family (e.g., /planner, /coder)
Decl shard_family(ShardID, Family).

# campaign_active(CampaignID) - currently active campaign
Decl campaign_active(CampaignID).

# -----------------------------------------------------------------------------
# 42.9 Injectable Context Derivation (Policy.mg Section 41)
# -----------------------------------------------------------------------------

# injectable_context(ShardID, Atom) - atoms selected for prompt injection
Decl injectable_context(ShardID, Atom).

# injectable_context_priority(ShardID, Atom, Priority) - priority-tagged context
# Priority: /high, /medium, /low
Decl injectable_context_priority(ShardID, Atom, Priority).

# final_injectable(ShardID, Atom) - final set after budget filtering
Decl final_injectable(ShardID, Atom).

# -----------------------------------------------------------------------------
# 42.10 Context Budget Management
# -----------------------------------------------------------------------------

# context_budget(ShardID, Budget) - available token budget for shard
Decl context_budget(ShardID, Budget).

# context_budget_constrained(ShardID) - derived: shard has limited context budget
Decl context_budget_constrained(ShardID).

# context_budget_sufficient(ShardID) - derived: shard has adequate context budget
Decl context_budget_sufficient(ShardID).

# has_injectable_context(ShardID) - helper: shard has context to inject
Decl has_injectable_context(ShardID).

# has_high_priority_context(ShardID) - helper: shard has high-priority context
Decl has_high_priority_context(ShardID).

# -----------------------------------------------------------------------------
# 42.11 Context Staleness & Refresh
# -----------------------------------------------------------------------------

# context_stale(ShardID, Atom) - context atom is stale and needs refresh
Decl context_stale(ShardID, Atom).

# has_stale_context(ShardID) - helper: shard has any stale context
Decl has_stale_context(ShardID).

# specialist_knowledge_updated(ShardID) - specialist knowledge was recently updated
Decl specialist_knowledge_updated(ShardID).

# -----------------------------------------------------------------------------
# 42.12 Trace Pattern Integration
# -----------------------------------------------------------------------------

# trace_pattern(TraceID, Pattern) - extracted pattern from a reasoning trace
Decl trace_pattern(TraceID, Pattern).

# -----------------------------------------------------------------------------
# 42.13 Learning from Context Injection
# -----------------------------------------------------------------------------

# context_injection_effective(ShardID, Atom) - context injection led to success
Decl context_injection_effective(ShardID, Atom).

# =============================================================================
# SECTION 43: NORTHSTAR VISION & SPECIFICATION
# =============================================================================
# The Northstar defines the project's grand vision, target users, capabilities,
# risks, requirements, and constraints. Used by /northstar command.

# -----------------------------------------------------------------------------
# 43.1 Core Vision
# -----------------------------------------------------------------------------

# northstar_mission(ID, Statement) - The one-sentence mission
Decl northstar_mission(ID, Statement).

# northstar_problem(ID, Description) - Problem being solved
Decl northstar_problem(ID, Description).

# northstar_vision(ID, Description) - Grand vision of success
Decl northstar_vision(ID, Description).

# -----------------------------------------------------------------------------
# 43.2 Target Users (Personas)
# -----------------------------------------------------------------------------

# northstar_persona(PersonaID, Name) - Target user archetype
Decl northstar_persona(PersonaID, Name).

# northstar_pain_point(PersonaID, PainPoint) - User pain points
Decl northstar_pain_point(PersonaID, PainPoint).

# northstar_need(PersonaID, Need) - User needs
Decl northstar_need(PersonaID, Need).

# -----------------------------------------------------------------------------
# 43.3 Capabilities Roadmap
# -----------------------------------------------------------------------------

# northstar_capability(CapID, Description, Timeline, Priority)
# Timeline: /now, /6mo, /1yr, /3yr, /moonshot
# Priority: /critical, /high, /medium, /low
Decl northstar_capability(CapID, Description, Timeline, Priority).

# northstar_serves(CapID, PersonaID) - Capability serves persona
Decl northstar_serves(CapID, PersonaID).

# -----------------------------------------------------------------------------
# 43.4 Risks & Mitigations (Red Teaming)
# -----------------------------------------------------------------------------

# northstar_risk(RiskID, Description, Likelihood, Impact)
# Likelihood/Impact: /high, /medium, /low
Decl northstar_risk(RiskID, Description, Likelihood, Impact).

# northstar_mitigation(RiskID, Strategy) - Risk mitigation strategy
Decl northstar_mitigation(RiskID, Strategy).

# -----------------------------------------------------------------------------
# 43.5 Requirements
# -----------------------------------------------------------------------------

# northstar_requirement(ReqID, Type, Description, Priority)
# Type: /functional, /non_functional, /constraint
# Priority: /must_have, /should_have, /nice_to_have
Decl northstar_requirement(ReqID, Type, Description, Priority).

# northstar_supports(ReqID, CapID) - Requirement supports capability
Decl northstar_supports(ReqID, CapID).

# northstar_addresses(ReqID, RiskID) - Requirement addresses risk
Decl northstar_addresses(ReqID, RiskID).

# -----------------------------------------------------------------------------
# 43.6 Constraints
# -----------------------------------------------------------------------------

# northstar_constraint(ConstraintID, Description) - Hard project constraints
Decl northstar_constraint(ConstraintID, Description).

# -----------------------------------------------------------------------------
# 43.7 Derived Predicates
# -----------------------------------------------------------------------------

# northstar_defined() - True if northstar has been set
Decl northstar_defined().

# critical_capability(CapID) - Derived: capability is critical priority
Decl critical_capability(CapID).

# high_risk(RiskID) - Derived: risk has high likelihood AND impact
Decl high_risk(RiskID).

# has_mitigation(RiskID) - Helper: risk has at least one mitigation
Decl has_mitigation(RiskID).

# unmitigated_risk(RiskID) - Derived: high risk without mitigation
Decl unmitigated_risk(RiskID).

# capability_addresses_need(CapID, PersonaID, Need) - Capability serves persona need
Decl capability_addresses_need(CapID, PersonaID, Need).

# is_served_persona(PersonaID) - Helper: persona is served by at least one capability
Decl is_served_persona(PersonaID).

# capability_is_linked(CapID) - Helper: capability serves at least one persona
Decl capability_is_linked(CapID).

# unserved_persona(PersonaID, Name) - Persona with needs but no capabilities
Decl unserved_persona(PersonaID, Name).

# orphan_capability(CapID, Desc) - Capability not linked to any persona
Decl orphan_capability(CapID, Desc).

# must_have_requirement(ReqID, Desc) - Requirements with must_have priority
Decl must_have_requirement(ReqID, Desc).

# is_supported_req(ReqID) - Helper: requirement is supported by at least one capability
Decl is_supported_req(ReqID).

# orphan_requirement(ReqID, Desc) - Requirement not linked to any capability
Decl orphan_requirement(ReqID, Desc).

# risk_addressing_requirement(ReqID, RiskID) - Requirement that addresses high risk
Decl risk_addressing_requirement(ReqID, RiskID).

# risk_is_addressed(RiskID) - Helper: risk is addressed by at least one requirement
Decl risk_is_addressed(RiskID).

# unaddressed_high_risk(RiskID, Desc) - High risk with no requirement addressing it
Decl unaddressed_high_risk(RiskID, Desc).

# immediate_capability(CapID, Desc) - Capabilities with /now timeline
Decl immediate_capability(CapID, Desc).

# near_term_capability(CapID, Desc) - Capabilities with /6mo timeline
Decl near_term_capability(CapID, Desc).

# long_term_capability(CapID, Desc) - Capabilities with /1yr or /3yr timeline
Decl long_term_capability(CapID, Desc).

# moonshot_capability(CapID, Desc) - Capabilities with /moonshot timeline
Decl moonshot_capability(CapID, Desc).

# strategic_warning(Type, CapID, RiskID) - Strategic gaps and warnings
Decl strategic_warning(Type, CapID, RiskID).

# =============================================================================
# SECTION 44: SEMANTIC MATCHING (Vector Search Results)
# =============================================================================
# These facts are asserted by the SemanticClassifier after vector search.
# They provide semantic similarity signals to the inference engine.

# semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)
# UserInput: Original user query string
# CanonicalSentence: Matched sentence from intent corpus
# Verb: Associated verb from corpus (name constant like /review)
# Target: Associated target from corpus (string)
# Rank: 1-based position in results (1 = best match)
# Similarity: Cosine similarity * 100 (0-100 scale, integer)
Decl semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity).

# Derived: suggested verb from semantic matching
# Populated by inference rules when semantic matches exist
Decl semantic_suggested_verb(Verb, MaxSimilarity).

# Derived: compound suggestions from multiple semantic matches
Decl compound_suggestion(Verb1, Verb2).

# learned_exemplar(Pattern, Verb, Target, Constraint, Confidence)
# Learned user patterns that influence intent classification
# NOTE: Also declared in schema/learning.mg - this is a backup declaration
Decl learned_exemplar(Pattern, Verb, Target, Constraint, Confidence).

# verb_composition(Verb1, Verb2, ComposedAction, Priority)
# Defines valid verb compositions for compound suggestions
# NOTE: Primary declaration is in taxonomy.mg - removed duplicate here
# Decl verb_composition(Verb1, Verb2, ComposedAction, Priority).

# =============================================================================
# SECTION 45: JIT PROMPT COMPILER SCHEMAS
# =============================================================================
# Universal JIT Prompt Compiler for dynamic prompt assembly.
# Every LLM call gets a dynamically compiled prompt based on full context:
# operational mode, campaign phase, intent verb, test state, world model,
# shard type, init phase, northstar state, ouroboros stage, and more.

# -----------------------------------------------------------------------------
# 45.1 Prompt Atom Registry (EDB - loaded from SQLite databases)
# -----------------------------------------------------------------------------

# prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
# Core atom metadata for selection
# AtomID: Unique identifier for the atom (string)
# Category: /identity, /safety, /hallucination, /methodology, /language,
#           /framework, /domain, /campaign, /init, /northstar, /ouroboros,
#           /context, /exemplar, /protocol
# Priority: Base priority score (0-100)
# TokenCount: Estimated token count for budget management
# IsMandatory: /true if atom must be included, /false otherwise
Decl prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory).

# atom_selector(AtomID, Dimension, Value)
# Multi-value selectors for dimensional filtering
# Dimension: /operational_mode, /campaign_phase, /build_layer, /init_phase,
#            /northstar_phase, /ouroboros_stage, /intent_verb, /shard_type,
#            /language, /framework, /world_state
# Value: Name constant matching the dimension (e.g., /active, /coder, /go)
Decl atom_selector(AtomID, Dimension, Value).

# atom_dependency(AtomID, DependsOnID, DepType)
# DepType: /hard (must have), /soft (prefer), /order_only (just ordering)
Decl atom_dependency(AtomID, DependsOnID, DepType).

# atom_conflict(AtomA, AtomB)
# Mutual exclusion - cannot select both
Decl atom_conflict(AtomA, AtomB).

# atom_exclusion_group(AtomID, GroupID)
# Only one atom per group can be selected
Decl atom_exclusion_group(AtomID, GroupID).

# atom_content(AtomID, Content)
# Actual prompt text (loaded on demand, large strings)
Decl atom_content(AtomID, Content).

# -----------------------------------------------------------------------------
# 45.2 Compilation Context (Set by Go before compilation)
# -----------------------------------------------------------------------------

# compile_context(Dimension, Value)
# Current compilation context asserted by Go runtime
# Dimension matches atom_selector dimensions
Decl compile_context(Dimension, Value).

# compile_budget(TotalTokens)
# Available token budget for this compilation
Decl compile_budget(TotalTokens).

# compile_shard(ShardID, ShardType)
# Target shard for this compilation
Decl compile_shard(ShardID, ShardType).

# compile_query(QueryText)
# Semantic query for vector search boosting
Decl compile_query(QueryText).

# -----------------------------------------------------------------------------
# 45.3 Vector Search Results (Asserted by Go after vector search)
# -----------------------------------------------------------------------------

# vector_recall_result(Query, AtomID, SimilarityScore)
# Results from vector store semantic search
# Query: The search query text
# AtomID: Matched atom identifier
# SimilarityScore: Cosine similarity (0.0-1.0)
Decl vector_recall_result(Query, AtomID, SimilarityScore).

# -----------------------------------------------------------------------------
# 45.4 Derived Selection Predicates (IDB - computed by rules)
# -----------------------------------------------------------------------------

# atom_matches_context(AtomID, Score)
# Computed match score based on context dimensions
Decl atom_matches_context(AtomID, Score).

# atom_selected(AtomID)
# Atom passes all selection criteria
Decl atom_selected(AtomID).

# atom_excluded(AtomID, Reason)
# Atom excluded with reason: /conflict, /exclusion_group, /over_budget, /missing_dependency
Decl atom_excluded(AtomID, Reason).

# atom_dependency_satisfied(AtomID)
# All hard dependencies are satisfied
Decl atom_dependency_satisfied(AtomID).

# atom_meets_threshold(AtomID)
# Helper: atom would meet score threshold (40) for selection
Decl atom_meets_threshold(AtomID).

# has_unsatisfied_hard_dep(AtomID)
# Helper: atom has at least one unsatisfied hard dependency
Decl has_unsatisfied_hard_dep(AtomID).

# is_excluded(AtomID)
# Helper: atom is excluded for any reason (for safe negation)
Decl is_excluded(AtomID).

# atom_candidate(AtomID)
# Helper: atom passes initial selection criteria (score threshold + deps)
Decl atom_candidate(AtomID).

# atom_loses_conflict(AtomID)
# Helper: atom loses due to conflict with higher-scoring atom
Decl atom_loses_conflict(AtomID).

# atom_loses_exclusion(AtomID)
# Helper: atom loses due to exclusion group with higher-scoring atom
Decl atom_loses_exclusion(AtomID).

# final_atom(AtomID, Order)
# Final ordered list for assembly
Decl final_atom(AtomID, Order).

# -----------------------------------------------------------------------------
# 45.5 Compilation Validation
# -----------------------------------------------------------------------------

# compilation_valid()
# True if compilation passes all constraints
Decl compilation_valid().

# compilation_error(ErrorType, Details)
# ErrorType: /missing_mandatory, /circular_dependency, /unsatisfied_dependency, /budget_overflow
Decl compilation_error(ErrorType, Details).

# has_compilation_error()
# Helper: true if any compilation error exists
Decl has_compilation_error().

# has_identity_atom()
# Helper: true if at least one identity atom is selected
Decl has_identity_atom().

# has_protocol_atom()
# Helper: true if at least one protocol atom is selected
Decl has_protocol_atom().

# -----------------------------------------------------------------------------
# 45.6 Category Ordering
# -----------------------------------------------------------------------------

# category_order(Category, OrderNum)
# Determines section order in final prompt
Decl category_order(Category, OrderNum).

# category_budget(Category, Percent)
# Budget allocation percentage per category
Decl category_budget(Category, Percent).

# -----------------------------------------------------------------------------
# 45.7 Additional JIT Compiler Schemas (for jit_compiler.mg compatibility)
# -----------------------------------------------------------------------------

# atom_tag(AtomID, Dimension, Tag)
# Alternative tagging predicate used by jit_compiler.mg
# Functionally equivalent to atom_selector but with /mode, /phase, /layer dimensions
# Dimension: /mode, /phase, /layer, /shard, /lang, /framework, /intent, /state, /tag
# Tag: Context value (e.g., /active, /coder, /go, /debug_only, /dream_only)
Decl atom_tag(AtomID, Dimension, Tag).

# vector_hit(AtomID, Score)
# Vector search results injected by Go runtime before compilation
# AtomID: Matched atom identifier
# Score: Cosine similarity score (0.0-1.0)
Decl vector_hit(AtomID, Score).

# current_context(Dimension, Tag)
# Runtime context state injected by Go (alternative to compile_context)
# Used by jit_compiler.mg for context matching
Decl current_context(Dimension, Tag).

# is_mandatory(AtomID)
# Flag indicating atom must be selected if context matches
Decl is_mandatory(AtomID).

# atom_requires(AtomID, DependencyID)
# Hard dependency: AtomID requires DependencyID to be selected
Decl atom_requires(AtomID, DependencyID).

# atom_conflicts(AtomA, AtomB)
# Mutual exclusion: AtomA and AtomB cannot both be selected
Decl atom_conflicts(AtomA, AtomB).

# atom_priority(AtomID, Priority)
# Base priority score for atom ordering
Decl atom_priority(AtomID, Priority).

# -----------------------------------------------------------------------------
# 45.8 Section 46 Selection Rule Schemas (IDB - computed by policy.mg Section 46)
# -----------------------------------------------------------------------------

# skeleton_category(Category)
# Categories that form the mandatory skeleton of every prompt
Decl skeleton_category(Category).

# mandatory_atom(AtomID)
# Atom must be included in prompt (Skeleton layer)
Decl mandatory_atom(AtomID).

# base_prohibited(AtomID)
# Base prohibition from context rules (Stratum 0, no dependency on mandatory)
Decl base_prohibited(AtomID).

# prohibited_atom(AtomID)
# Atom is blocked by firewall rules
Decl prohibited_atom(AtomID).

# candidate_atom(AtomID)
# Atom is a valid candidate for selection (Flesh layer)
Decl candidate_atom(AtomID).

# conflict_loser(AtomID)
# Helper: atom loses in conflict resolution (lower priority in conflict pair)
Decl conflict_loser(AtomID).

# selected_atom(AtomID)
# Final selection: mandatory OR valid candidate (not a conflict loser)
Decl selected_atom(AtomID).

# atom_context_boost(AtomID, BoostedPriority)
# Priority boost based on context matching
Decl atom_context_boost(AtomID, BoostedPriority).

# has_skeleton_category(Category)
# Helper: true if at least one atom from this skeleton category is selected
Decl has_skeleton_category(Category).

# missing_skeleton_category(Category)
# Helper: skeleton category with no selected atoms (compilation error)
Decl missing_skeleton_category(Category).

# =============================================================================
# SECTION 47: STATIC ANALYSIS - DATA FLOW PREDICATES (ReviewerShard Beyond-SOTA)
# =============================================================================
# Advanced static analysis predicates for differential nil-pointer detection,
# error handling verification, and data flow tracking. These enable the
# ReviewerShard to perform precise diff-aware analysis using guard-based
# reasoning.

# -----------------------------------------------------------------------------
# 47.1 Data Flow Predicates - Variable Tracking
# -----------------------------------------------------------------------------

# assigns(Var, ValueType, File, Line) - Variable assignment tracking
# Var: Variable name (string)
# ValueType: Type of value assigned (e.g., /pointer, /interface, /value, /error)
# File: Source file path
# Line: Line number of assignment
Decl assigns(Var, ValueType, File, Line).

# uses(File, Func, Var, Line) - Variable read sites
# File: Source file path
# Func: Function containing the use
# Var: Variable being read
# Line: Line number of use
Decl uses(File, Func, Var, Line).

# call_arg(CallSite, ArgPos, VarRef, File, Line) - Function call argument tracking
# CallSite: Identifier for the call (e.g., "foo.Bar")
# ArgPos: Argument position (0-indexed integer)
# VarRef: Variable reference passed as argument
# File: Source file path
# Line: Line number
Decl call_arg(CallSite, ArgPos, VarRef, File, Line).

# -----------------------------------------------------------------------------
# 47.2 Guard Predicates - Two Types for Go's Idiomatic Patterns
# -----------------------------------------------------------------------------
# Go uses two distinct guard patterns:
# 1. Block guards: if x != nil { /* x is safe here */ }
# 2. Return guards: if x == nil { return } /* x is safe after */

# guards_block(Var, CheckType, File, ScopeStart, ScopeEnd) - Block-scoped guards
# Var: Variable being guarded
# CheckType: Type of check (/nil_check, /len_check, /type_assert, /ok_check)
# File: Source file path
# ScopeStart: Starting line of guarded scope
# ScopeEnd: Ending line of guarded scope
Decl guards_block(Var, CheckType, File, ScopeStart, ScopeEnd).

# guards_return(Var, CheckType, File, Line) - Return-based guards (dominator guards)
# Var: Variable being guarded
# CheckType: Type of check (/nil_check, /zero_check, /err_check)
# File: Source file path
# Line: Line of the guard check (all lines after are guarded)
Decl guards_return(Var, CheckType, File, Line).

# -----------------------------------------------------------------------------
# 47.3 Error Handling Predicates
# -----------------------------------------------------------------------------

# error_checked_block(Var, File, ScopeStart, ScopeEnd) - Block-scoped error handling
# Var: Error variable being checked
# File: Source file path
# ScopeStart: Start of error handling scope
# ScopeEnd: End of error handling scope
Decl error_checked_block(Var, File, ScopeStart, ScopeEnd).

# error_checked_return(Var, File, Line) - Return-based error handling
# Var: Error variable being checked
# File: Source file path
# Line: Line of error check (typically: if err != nil { return err })
Decl error_checked_return(Var, File, Line).

# -----------------------------------------------------------------------------
# 47.4 Function Metadata
# -----------------------------------------------------------------------------

# nil_returns(File, Func, Line) - Functions that can return nil
# File: Source file path
# Func: Function name
# Line: Line number where nil is returned
Decl nil_returns(File, Func, Line).

# modified_function(Func, File) - Functions changed in the current diff
# Func: Function name
# File: Source file path
Decl modified_function(Func, File).

# modified_interface(Interface, File) - Interfaces changed in the current diff
# Interface: Interface name
# File: Source file path
Decl modified_interface(Interface, File).

# -----------------------------------------------------------------------------
# 47.5 Scope Tracking (for dominator guard analysis)
# -----------------------------------------------------------------------------

# same_scope(Var, File, Line1, Line2) - Lines in same function scope
# Var: Variable name (for context)
# File: Source file path
# Line1: First line number
# Line2: Second line number
# Used to determine if a guard at Line1 protects a use at Line2
Decl same_scope(Var, File, Line1, Line2).

# -----------------------------------------------------------------------------
# 47.6 Suppression Predicates (for Autopoiesis - False Positive Learning)
# -----------------------------------------------------------------------------

# suppressed_rule(RuleType, File, Line, Reason) - Manually suppressed findings
# RuleType: Type of rule suppressed (e.g., /nil_deref, /unchecked_error)
# File: Source file path
# Line: Line number of suppression
# Reason: User-provided reason for suppression
Decl suppressed_rule(RuleType, File, Line, Reason).

# suppression_confidence(RuleType, File, Line, Score) - Learned suppression confidence
# RuleType: Type of rule
# File: Source file path
# Line: Line number
# Score: Confidence score (0-100) that this is a false positive
Decl suppression_confidence(RuleType, File, Line, Score).

# -----------------------------------------------------------------------------
# 47.7 Priority and Risk Predicates
# -----------------------------------------------------------------------------

# type_priority(Type, Priority) - Severity priority by type
# Type: Finding type (e.g., /nil_deref, /unchecked_error, /race_condition)
# Priority: Priority level (1=critical, 2=high, 3=medium, 4=low)
Decl type_priority(Type, Priority).

# bug_history(File, Count) - Historical bug count per file
# File: Source file path
# Count: Number of bugs historically found in this file
# Used for risk-based prioritization
Decl bug_history(File, Count).

# -----------------------------------------------------------------------------
# 47.8 Derived Predicates for Data Flow Analysis
# -----------------------------------------------------------------------------

# guarded_use(Var, File, Line) - Derived: variable use is protected by a guard
Decl guarded_use(Var, File, Line).

# unguarded_use(Var, File, Line) - Derived: variable use lacks guard protection
Decl unguarded_use(Var, File, Line).

# error_ignored(Var, File, Line) - Derived: error variable is not checked
Decl error_ignored(Var, File, Line).

# nil_deref_risk(Var, File, Line, RiskLevel) - Derived: potential nil dereference
# RiskLevel: /high (no guard), /medium (conditional guard), /low (likely safe)
Decl nil_deref_risk(Var, File, Line, RiskLevel).

# in_modified_code(File, Line) - Derived: line is within modified diff hunks
Decl in_modified_code(File, Line).

# diff_introduces_risk(File, Line, RiskType) - Derived: diff introduces new risk
# RiskType: /nil_deref, /unchecked_error, /race_condition
Decl diff_introduces_risk(File, Line, RiskType).

# has_guard(Var, File, Line) - Helper: variable has any guard at this point
Decl has_guard(Var, File, Line).

# is_suppressed(RuleType, File, Line) - Helper: finding is suppressed
Decl is_suppressed(RuleType, File, Line).

# -----------------------------------------------------------------------------
# 47.9 Data Flow Safety Derived Predicates (IDB - from policy.mg Section 47)
# -----------------------------------------------------------------------------

# is_guarded(Var, File, Line) - Derived: variable is protected at this point
# Computed from guards_block and guards_return
Decl is_guarded(Var, File, Line).

# unsafe_deref(File, Var, Line) - Derived: nullable dereference without guard
Decl unsafe_deref(File, Var, Line).

# is_error_checked(Var, File, Line) - Derived: error variable is checked
Decl is_error_checked(Var, File, Line).

# unchecked_error(File, Func, Line) - Derived: error assigned but not checked
Decl unchecked_error(File, Func, Line).

# -----------------------------------------------------------------------------
# 47.10 Impact Analysis Derived Predicates (IDB - from policy.mg Section 48)
# -----------------------------------------------------------------------------

# impact_caller(TargetFunc, CallerFunc) - Direct callers of modified function
Decl impact_caller(TargetFunc, CallerFunc).

# impact_implementer(ImplFile, Struct) - Implementers of modified interface
Decl impact_implementer(ImplFile, Struct).

# impact_graph(Target, Caller, Depth) - Transitive impact with depth (max 3)
Decl impact_graph(Target, Caller, Depth).

# relevant_context_file(File) - Files to fetch for review context
Decl relevant_context_file(File).

# context_priority_file(File, Func, Priority) - Priority-ordered context files
Decl context_priority_file(File, Func, Priority).

# -----------------------------------------------------------------------------
# 47.11 Hypothesis Management (IDB - from policy.mg Sections 49-50)
# -----------------------------------------------------------------------------

# active_hypothesis(Type, File, Line, Var) - Post-suppression hypotheses
Decl active_hypothesis(Type, File, Line, Var).

# priority_boost(File, Boost) - Additional priority for risky files
Decl priority_boost(File, Boost).

# prioritized_hypothesis(Type, File, Line, Var, Priority) - Final prioritized findings
Decl prioritized_hypothesis(Type, File, Line, Var, Priority).

# -----------------------------------------------------------------------------
# 47.12 Helper Predicates for Safe Negation (IDB)
# -----------------------------------------------------------------------------
# These helpers enable safe negation by ensuring variables are bound before
# negation is applied. Required by Mangle's safety constraints.

# has_guard_at(Var, File, Line) - Helper for guarded variable check
Decl has_guard_at(Var, File, Line).

# has_error_check_at(Var, File, Line) - Helper for error check presence
Decl has_error_check_at(Var, File, Line).

# has_suppression_unsafe_deref(File, Line) - Helper for suppression check
Decl has_suppression_unsafe_deref(File, Line).

# has_suppression_unchecked_error(File, Line) - Helper for suppression check
Decl has_suppression_unchecked_error(File, Line).

# has_test_coverage(File) - Helper: file has test coverage
Decl has_test_coverage(File).

# has_bug_history(File) - Helper: file has bug history (count > 0)
Decl has_bug_history(File).

# has_priority_boost(File) - Helper: file has any priority boost
Decl has_priority_boost(File).

# -----------------------------------------------------------------------------
# 47.13 Multi-Language Data Flow Predicates
# -----------------------------------------------------------------------------
# These predicates support data flow analysis across multiple languages
# (Go, Python, TypeScript, JavaScript, Rust) using Tree-sitter parsing.

# function_scope(File, Func, Start, End) - Function scope boundaries
# File: Source file path
# Func: Function name
# Start: Starting line number
# End: Ending line number
# Used to determine scope for guard domination
Decl function_scope(File, Func, Start, End).

# guard_dominates(File, Func, GuardLine, EndLine) - Guard domination for early returns
# File: Source file path
# Func: Function containing the guard
# GuardLine: Line of the guard check (e.g., if x == nil { return })
# EndLine: Last line of the function (guard protects all subsequent lines)
# Early return guards dominate all code after them in the same scope
Decl guard_dominates(File, Func, GuardLine, EndLine).

# safe_access(Var, AccessType, File, Line) - Language-specific safe access patterns
# Var: Variable being accessed
# AccessType: Type of safe access pattern:
#   /optional_chain - JavaScript/TypeScript x?.foo
#   /if_let - Rust if let Some(x) = ...
#   /match_exhaustive - Rust match expression (exhaustive)
#   /walrus - Python x := (assignment expression)
# File: Source file path
# Line: Line number
# These accesses are inherently safe by the language's semantics
Decl safe_access(Var, AccessType, File, Line).



# Prompt Atom Schema & Standard Atoms
# Defines the vocabulary for JIT Prompt Compilation.

# --- 1. CORE SCHEMA ---

# atom(ID)
# Identifies a prompt atom.
# defined by: atom(ID).

# atom_category(ID, Category)
# Categorizes atoms for sorting/grouping.
# Categories: /identity, /protocol, /safety, /methodology, /hallucination, 
#             /language, /framework, /domain, /campaign, /init, /context, /exemplar
# defined by: atom_category(ID, Category).

# atom_description(ID, Text)
# Semantic description for vector embedding/search.
# defined by: atom_description(ID, Text).

# atom_content_type(ID, Type)
# Type of content: /standard, /concise, /min.
# defined by: atom_content_type(ID, Type).

# --- 2. CONTEXT TAGS (Normalized Link Table) ---

# atom_tag(ID, Dimension, Tag)
# Tagging system for context matching.
# Dimensions: /mode, /phase, /layer, /shard, /lang, /framework, /intent, /state
# defined by: atom_tag(ID, Dimension, Tag).

# --- 3. RELATIONS ---

# atom_requires(ID, DependencyID)
# Hard dependency: If ID is selected, DependencyID MUST be selected.
# defined by: atom_requires(ID, DependencyID).

# atom_conflicts(ID, ConflictID)
# Exclusion: ID cannot coexist with ConflictID (unless suppressed).
# defined by: atom_conflicts(ID, ConflictID).

# atom_exclusive(ID, GroupID)
# Mutual Exclusion: Only one atom from GroupID can be selected.
# defined by: atom_exclusive(ID, GroupID).

# --- 4. ATTRIBUTES ---

# atom_priority(ID, Score)
# Sorting priority (Higher = Earlier in prompt).
# defined by: atom_priority(ID, Score).

# is_mandatory(ID)
# Flag: This atom is mandatory if context matches.
# defined by: is_mandatory(ID).

# --- 5. RUNTIME INPUTS (Injected by Go) ---

# vector_hit(ID, Score)
# Atom found by vector search with similarity score.

# current_context(Dimension, Tag)
# The current environment state (e.g., current_context(/mode, /active)).

# token_budget(Limit)
# Available token budget.



# User Extensions
# User Schema Extensions
# Define project-specific predicates here.
# These will be loaded AFTER the core schemas.

# Example:
# Decl project_metadata(Key, Value).
# Decl deploy_target(Env, URL).

# Cortex 1.5.0 Executive Policy (IDB)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# =============================================================================
# SECTION 1: SPREADING ACTIVATION (Context Selection)
# =============================================================================
# Per §8.1: Energy flows from the user's intent through the graph of known facts

# 1. Base Activation (Recency) - High priority for new facts
activation(Fact, 100) :- new_fact(Fact).

# 2. Spreading Activation (Dependency)
# Energy flows from goals to required tools
activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# 3. Intent-driven activation
activation(Target, 90) :-
    user_intent(_, _, _, Target, _).

# 4. File modification spreads to dependents
activation(Dep, 70) :-
    modified(File),
    dependency_link(Dep, File, _).

# 5. Context Pruning - Only high-activation facts enter working memory
context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.

# =============================================================================
# SECTION 2: STRATEGY SELECTION (§3.1)
# =============================================================================
# Different coding tasks require different logical loops

# TDD Repair Loop for bug fixes
active_strategy(/tdd_repair_loop) :-
    user_intent(_, _, /fix, _, _),
    diagnostic(/error, _, _, _, _).

active_strategy(/tdd_repair_loop) :-
    user_intent(_, _, /debug, _, _).

# Exploration for queries
active_strategy(/breadth_first_survey) :-
    user_intent(_, /query, /explore, _, _).

active_strategy(/breadth_first_survey) :-
    user_intent(_, /query, /explain, _, _).

# Code generation for scaffolding
active_strategy(/project_init) :-
    user_intent(_, /mutation, /scaffold, _, _).

active_strategy(/project_init) :-
    user_intent(_, /mutation, /init, _, _).

# Refactor guard for modifications
active_strategy(/refactor_guard) :-
    user_intent(_, /mutation, /refactor, _, _).

# =============================================================================
# SECTION 3: TDD REPAIR LOOP (§3.2)
# =============================================================================
# State machine: Write -> Test -> Analyze -> Fix

# State Transitions
next_action(/read_error_log) :-
    test_state(/failing),
    retry_count(N), N < 3.

next_action(/analyze_root_cause) :-
    test_state(/log_read).

next_action(/generate_patch) :-
    test_state(/cause_found).

next_action(/run_tests) :-
    test_state(/patch_applied).

next_action(/run_tests) :-
    test_state(/unknown),
    user_intent(_, _, /test, _, _).

# Surrender Logic - Escalate after 3 retries
next_action(/escalate_to_user) :-
    test_state(/failing),
    retry_count(N), N >= 3.

# Success state
next_action(/complete) :-
    test_state(/passing).

# =============================================================================
# SECTION 4: FOCUS RESOLUTION & CLARIFICATION (§1.2)
# =============================================================================

# Clarification threshold - block execution if confidence < 85 (on 0-100 scale)
clarification_needed(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 85.

# Block action derivation when clarification is needed
next_action(/interrogative_mode) :-
    clarification_needed(_).

# Ambiguity detection
ambiguity_detected(Param) :-
    ambiguity_flag(Param, _, _).

next_action(/interrogative_mode) :-
    ambiguity_detected(_).

# =============================================================================
# SECTION 5: IMPACT ANALYSIS & REFACTORING GUARD (§3.3)
# =============================================================================

# Direct impact
impacted(X) :-
    dependency_link(X, Y, _),
    modified(Y).

# Transitive closure (recursive impact)
impacted(X) :-
    dependency_link(X, Z, _),
    impacted(Z).

# Unsafe to refactor if impacted code lacks test coverage
unsafe_to_refactor(Target) :-
    impacted(Target),
    !test_coverage(Target).

# Block refactoring when unsafe
block_refactor(Target, "uncovered_dependency") :-
    unsafe_to_refactor(Target).

# =============================================================================
# SECTION 6: COMMIT BARRIER (§2.2)
# =============================================================================

# Cannot commit if there are errors
block_commit("Build Broken") :-
    diagnostic(/error, _, _, _, _).

block_commit("Tests Failing") :-
    test_state(/failing).

# Fix Bug #10: The "Timeout = Permission" Trap
# Require explicit positive confirmation that checks actually ran
checks_passed() :-
    build_result(/true, _),
    test_state(/passing).

# Helper for safe negation
has_block_commit() :-
    block_commit(_).

# Safe to commit ONLY if checks passed AND no blocks exist
safe_to_commit() :-
    checks_passed(),
    !has_block_commit().

# =============================================================================
# SECTION 7: CONSTITUTIONAL LOGIC / SAFETY (§5.0)
# =============================================================================

# Default deny - permitted must be positively derived
permitted(Action) :-
    safe_action(Action).

permitted(Action) :-
    dangerous_action(Action),
    admin_override(User),
    signed_approval(Action).

# Fix Bug #12: The "Silent Join" (Shadow Rules)
# Explain WHY permission was denied to aid debugging/feedback
permission_denied(Action, "Dangerous Action") :-
    dangerous_action(Action),
    !admin_override(_).

permission_denied(Action, "Dangerous Action") :-
    dangerous_action(Action),
    !signed_approval(Action).

# Dangerous action patterns - marked by explicit facts
# dangerous_action is derived from danger_marker facts
# (String matching to be implemented via custom builtins)

# =============================================================================
# SAFE ACTIONS - Permitted by default for all shards
# =============================================================================
# These actions are constitutionally permitted without special approval.
# Dangerous actions (rm -rf, etc.) require admin_override + signed_approval.

# File operations (read-only and basic writes)
safe_action(/read_file).
safe_action(/fs_read).
safe_action(/write_file).
safe_action(/fs_write).
safe_action(/search_files).
safe_action(/glob_files).
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

# Test operations (running tests is safe)
safe_action(/run_tests).
safe_action(/test_single).
safe_action(/coverage).

# Knowledge operations
safe_action(/vector_search).
safe_action(/knowledge_query).
safe_action(/embed_text).

# Browser operations (read-only)
safe_action(/browser_navigate).
safe_action(/browser_screenshot).
safe_action(/browser_read_dom).

# Network policy - allowlist approach
allowed_domain("github.com").
allowed_domain("pypi.org").
allowed_domain("crates.io").
allowed_domain("npmjs.com").
allowed_domain("pkg.go.dev").

# Note: network_permitted and security_violation require string matching
# which will be implemented via custom Go builtins at runtime

# =============================================================================
# SECTION 7B: STRATIFIED TRUST - AUTOPOIESIS SAFETY (Bug #15 Fix)
# =============================================================================
# This implements the "Stratified Trust" architecture to prevent jailbreak.
# Learned logic (from learned.gl) can ONLY propose candidate_action/1.
# The Constitution validates all candidates before they become final_action/1.
#
# SECURITY INVARIANT:
#   final_action(X) ⊆ candidate_action(X) ∩ permitted(X)
#
# This ensures learned rules can suggest, but never execute without approval.

# The Bridge Rule: Learned suggestions must pass constitutional checks
final_action(Action) :-
    candidate_action(Action),
    permitted(Action).

# Safety check predicate for runtime validation
safety_check(Action) :-
    permitted(Action).

# Deny actions that are candidates but not permitted
action_denied(Action, "Not constitutionally permitted") :-
    candidate_action(Action),
    !permitted(Action).

# Track learned rule proposals for auditing
learned_proposal(Action) :-
    candidate_action(Action).

# Metrics: Count how many learned rules are being blocked
# TODO: Re-enable when aggregation functions are fully implemented
blocked_learned_action_count(0) :-
    action_denied(_, _).

# =============================================================================
# SECTION 7C: APPEAL MECHANISM (Constitutional Appeals)
# =============================================================================
# Allows users to appeal blocked actions with justification.
# Appeals can be granted permanently or as temporary overrides.

# Actions that were blocked and have appeal available can be reconsidered
# if the appeal provides valid justification

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
# NOTE: temporary_override now stores ExpirationTimestamp directly (no arithmetic needed)
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    temporary_override(ActionType, Expiration),
    current_time(Now),
    Now < Expiration.

# Permanent override (no expiration) - use helper to avoid unbound variable in negation
has_active_override(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    !has_temporary_override(ActionType).

# Appeal granted should permit the action
permitted(ActionType) :-
    has_active_override(ActionType).

# Alert if too many appeals are being denied
excessive_appeal_denials() :-
    appeal_denied(_, _, _, _),
    appeal_denied(_, _, _, _),
    appeal_denied(_, _, _, _).

# Signal need for policy review if appeals frequently granted
appeal_pattern_detected(ActionType) :-
    appeal_granted(_, ActionType, _, _),
    appeal_granted(_, ActionType, _, _).

# =============================================================================
# SECTION 8: SHARD DELEGATION (§7.0)
# =============================================================================

# Delegate to researcher for init/explore
delegate_task(/researcher, "Initialize codebase analysis", /pending) :-
    user_intent(_, _, /init, _, _).

delegate_task(/researcher, Task, /pending) :-
    user_intent(_, _, /research, Task, _).

delegate_task(/researcher, Task, /pending) :-
    user_intent(_, /query, /explore, Task, _).

# Delegate to coder for coding tasks
delegate_task(/coder, Task, /pending) :-
    user_intent(_, /mutation, /implement, Task, _).

# Note: Negation with unbound variables is unsafe in Datalog
# Delegate refactoring task only when block_refactor facts don't exist
# This is handled at runtime by checking block_refactor before delegation
delegate_task(/coder, Task, /pending) :-
    user_intent(_, /mutation, /refactor, Task, _).

# Delegate to tester for test tasks
delegate_task(/tester, Task, /pending) :-
    user_intent(_, _, /test, Task, _).

delegate_task(/tester, "Generate tests for impacted code", /pending) :-
    impacted(File),
    !test_coverage(File).

# Delegate to reviewer for review tasks
delegate_task(/reviewer, Task, /pending) :-
    user_intent(_, _, /review, Task, _).

# =============================================================================
# SECTION 9: BROWSER PHYSICS (§9.0)
# =============================================================================

# Spatial reasoning - element to the left (constrained to interactable elements to avoid O(N²))
left_of(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, Ax, _, _, _),
    geometry(B, Bx, _, _, _),
    Ax < Bx.

# Element above another (constrained to interactable elements)
above(A, B) :-
    interactable(A, _),
    interactable(B, _),
    geometry(A, _, Ay, _, _),
    geometry(B, _, By, _, _),
    Ay < By.

# Honeypot detection via CSS properties
honeypot_detected(ID) :-
    computed_style(ID, "display", "none").

honeypot_detected(ID) :-
    computed_style(ID, "visibility", "hidden").

honeypot_detected(ID) :-
    computed_style(ID, "opacity", "0").

honeypot_detected(ID) :-
    geometry(ID, _, _, 0, _).

honeypot_detected(ID) :-
    geometry(ID, _, _, _, 0).

# Safe interactive elements (not honeypots)
safe_interactable(ID) :-
    interactable(ID, _),
    !honeypot_detected(ID).

# Target checkbox to the left of label text
target_checkbox(CheckID, LabelText) :-
    dom_node(CheckID, /input, _),
    attr(CheckID, "type", "checkbox"),
    visible_text(TextID, LabelText),
    left_of(CheckID, TextID).

# =============================================================================
# SECTION 10: TOOL CAPABILITY MAPPING & ACTION MAPPING
# =============================================================================

# Tool capabilities for spreading activation
tool_capabilities(/fs_read, /read).
tool_capabilities(/fs_write, /write).
tool_capabilities(/exec_cmd, /execute).
tool_capabilities(/browser, /navigate).
tool_capabilities(/browser, /click).
tool_capabilities(/browser, /type).
tool_capabilities(/code_graph, /analyze).
tool_capabilities(/code_graph, /dependencies).

# Goal capability requirements
goal_requires(Goal, /read) :-
    user_intent(_, /query, _, Goal, _).

goal_requires(Goal, /write) :-
    user_intent(_, /mutation, _, Goal, _).

goal_requires(Goal, /execute) :-
    user_intent(_, _, /run, Goal, _).

goal_requires(Goal, /analyze) :-
    user_intent(_, _, /explain, Goal, _).

# Action Mappings: Map intent verbs to executable actions
# Core actions
action_mapping(/explain, /analyze_code).
action_mapping(/read, /fs_read).
action_mapping(/search, /search_files).
action_mapping(/run, /exec_cmd).
action_mapping(/test, /run_tests).

# Code review & analysis actions (delegate to reviewer shard)
action_mapping(/review, /delegate_reviewer).
action_mapping(/security, /delegate_reviewer).
action_mapping(/analyze, /delegate_reviewer).

# Code mutation actions (delegate to coder shard)
action_mapping(/fix, /delegate_coder).
action_mapping(/refactor, /delegate_coder).
action_mapping(/create, /delegate_coder).
action_mapping(/delete, /delegate_coder).
action_mapping(/write, /fs_write).
action_mapping(/document, /delegate_coder).
action_mapping(/commit, /delegate_coder).

# Debug actions
action_mapping(/debug, /delegate_coder).

# Research actions (delegate to researcher shard)
action_mapping(/research, /delegate_researcher).
action_mapping(/explore, /delegate_researcher).

# Autopoiesis/Tool generation actions (delegate to tool_generator shard)
action_mapping(/generate_tool, /delegate_tool_generator).
action_mapping(/refine_tool, /delegate_tool_generator).
action_mapping(/list_tools, /delegate_tool_generator).
action_mapping(/tool_status, /delegate_tool_generator).

# Diff actions
action_mapping(/diff, /show_diff).

# Derive next_action from intent and mapping
next_action(Action) :-
    user_intent(_, _, Verb, _, _),
    action_mapping(Verb, Action).

# Specific file system actions
next_action(/fs_read) :-
    user_intent(_, _, /read, _, _).

next_action(/fs_write) :-
    user_intent(_, _, /write, _, _).

# Review delegation - high confidence triggers immediate delegation
delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /review, Target, _).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /security, Target, _).

delegate_task(/reviewer, Target, /pending) :-
    user_intent(_, _, /analyze, Target, _).

# Tool generator delegation - autopoiesis operations
delegate_task(/tool_generator, Target, /pending) :-
    user_intent(_, _, /generate_tool, Target, _).

delegate_task(/tool_generator, Target, /pending) :-
    user_intent(_, _, /refine_tool, Target, _).

delegate_task(/tool_generator, "", /pending) :-
    user_intent(_, _, /list_tools, _, _).

delegate_task(/tool_generator, Target, /pending) :-
    user_intent(_, _, /tool_status, Target, _).

# Auto-delegate when missing capability detected (implicit tool generation)
delegate_task(/tool_generator, Cap, /pending) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).

# =============================================================================
# SECTION 11: ABDUCTIVE REASONING (§8.2)
# =============================================================================

# Abductive reasoning: missing hypotheses are symptoms without known causes
# This rule requires all variables to be bound in the negated atom
# Implementation: We use a helper predicate has_known_cause to track which symptoms have causes
# Then negate against that helper

# Mark symptoms that have known causes
has_known_cause(Symptom) :-
    known_cause(Symptom, _).

# Symptoms without causes need investigation
# Note: Using has_known_cause helper to ensure safe negation
missing_hypothesis(Symptom) :-
    symptom(_, Symptom),
    !has_known_cause(Symptom).

# Trigger clarification for missing hypotheses
next_action(/interrogative_mode) :-
    missing_hypothesis(_).

# =============================================================================
# SECTION 12: AUTOPOIESIS / LEARNING (§8.3)
# =============================================================================

# Detect repeated rejection pattern
preference_signal(Pattern) :-
    rejection_count(Pattern, N),
    N >= 3.

# Promote to long-term memory
promote_to_long_term(FactType, FactValue) :-
    preference_signal(Pattern),
    derived_rule(Pattern, FactType, FactValue).

# Autopoiesis: Missing Tool Detection
# Helper: derive when we HAVE a capability (for safe negation)
has_capability(Cap) :-
    tool_capabilities(_, Cap).

# Derive missing_tool_for if user intent requires a capability we don't have
missing_tool_for(IntentID, Cap) :-
    user_intent(IntentID, _, _, _, _),
    goal_requires(_, Cap),
    !has_capability(Cap).

# Trigger tool generation if tool is missing
next_action(/generate_tool) :-
    missing_tool_for(_, _).

# =============================================================================
# SECTION 12B: OUROBOROS LOOP - TOOL SELF-GENERATION
# =============================================================================
# The Ouroboros Loop: Detection → Specification → Safety → Compile → Register → Execute
# Named after the ancient symbol of a serpent eating its own tail.

# Tool exists in registry
tool_exists(ToolName) :-
    tool_registered(ToolName, _).

# Tool is ready for execution (compiled and registered)
tool_ready(ToolName) :-
    tool_exists(ToolName),
    tool_hash(ToolName, _).

# Tool is available (registered and ready)
tool_available(ToolName) :-
    registered_tool(ToolName, _, _).

# Capability is available if any tool provides it
capability_available(Cap) :-
    tool_capability(_, Cap).

# Need new tool when capability missing and user explicitly requests it
explicit_tool_request(Cap) :-
    user_intent(_, /mutation, /generate_tool, Cap, _).

# Need new tool when repeated failures suggest capability gap
capability_gap_detected(Cap) :-
    task_failure_reason(_, "missing_capability", Cap),
    task_failure_count(Cap, N),
    N >= 2.

# Tool generation is permitted (safety gate)
tool_generation_permitted(Cap) :-
    missing_tool_for(_, Cap),
    !tool_generation_blocked(Cap).

# Block tool generation for dangerous capabilities
tool_generation_blocked(Cap) :-
    dangerous_capability(Cap).

# Define dangerous capabilities that should never be auto-generated
dangerous_capability(/exec_arbitrary).
dangerous_capability(/network_unconstrained).
dangerous_capability(/system_admin).
dangerous_capability(/credential_access).

# Ouroboros next actions
next_action(/ouroboros_detect) :-
    capability_gap_detected(_).

next_action(/ouroboros_generate) :-
    tool_generation_permitted(_),
    !has_active_generation().

next_action(/ouroboros_compile) :-
    tool_source_ready(ToolName),
    tool_safety_verified(ToolName),
    !tool_compiled(ToolName).

next_action(/ouroboros_register) :-
    tool_compiled(ToolName),
    !is_tool_registered(ToolName).

# Track active tool generation (prevent parallel generations)
active_generation(ToolName) :-
    generation_state(ToolName, /in_progress).

# Helper for safe negation - true if any generation is in progress
has_active_generation() :-
    active_generation(_).

# Helper for safe negation - true if tool is registered
is_tool_registered(ToolName) :-
    tool_registered(ToolName, _).

# Tool lifecycle states
tool_lifecycle(ToolName, /detected) :-
    missing_tool_for(_, ToolName).

tool_lifecycle(ToolName, /generating) :-
    generation_state(ToolName, /in_progress).

tool_lifecycle(ToolName, /safety_check) :-
    tool_source_ready(ToolName),
    !tool_safety_verified(ToolName).

tool_lifecycle(ToolName, /compiling) :-
    tool_safety_verified(ToolName),
    !tool_compiled(ToolName).

tool_lifecycle(ToolName, /ready) :-
    tool_ready(ToolName).

# =============================================================================
# SECTION 12C: TOOL LEARNING AND OPTIMIZATION
# =============================================================================
# Learning from tool executions to improve future generations.

# Tool quality tracking (quality on 0-100 scale)
tool_quality_poor(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality < 50.

tool_quality_acceptable(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality >= 50,
    AvgQuality < 80.

tool_quality_good(ToolName) :-
    tool_learning(ToolName, Executions, _, AvgQuality),
    Executions >= 3,
    AvgQuality >= 80.

# Trigger refinement for poor quality tools
tool_needs_refinement(ToolName) :-
    tool_quality_poor(ToolName).

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /pagination),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

tool_needs_refinement(ToolName) :-
    tool_known_issue(ToolName, /incomplete),
    tool_learning(ToolName, Executions, _, _),
    Executions >= 2.

# Next action for refinement
next_action(/refine_tool) :-
    tool_needs_refinement(_),
    !has_active_refinement().

# Prevent parallel refinements
active_refinement(ToolName) :-
    refinement_state(ToolName, /in_progress).

# Helper for safe negation - true if any refinement is in progress
has_active_refinement() :-
    active_refinement(_).

# Learning pattern signals
learning_pattern_detected(ToolName, IssueType) :-
    tool_known_issue(ToolName, IssueType),
    issue_occurrence_count(ToolName, IssueType, Count),
    Count >= 3.

# Promote learnings to tool generation hints
tool_generation_hint(Capability, "add_pagination") :-
    learning_pattern_detected(_, /pagination),
    capability_similar_to(Capability, _).

tool_generation_hint(Capability, "increase_limits") :-
    learning_pattern_detected(_, /incomplete),
    capability_similar_to(Capability, _).

tool_generation_hint(Capability, "add_retry") :-
    learning_pattern_detected(_, /rate_limit),
    capability_similar_to(Capability, _).

# Track refinement success
refinement_effective(ToolName) :-
    tool_refined(ToolName, OldVersion, NewVersion),
    version_quality(ToolName, OldVersion, OldQuality),
    version_quality(ToolName, NewVersion, NewQuality),
    NewQuality > OldQuality.

# Escalate if refinement didn't help
escalate_to_user(ToolName, "refinement_ineffective") :-
    tool_refined(ToolName, _, _),
    tool_quality_poor(ToolName),
    refinement_count(ToolName, Count),
    Count >= 2.

# =============================================================================
# SECTION 13: GIT-AWARE SAFETY / CHESTERTON'S FENCE (§21)
# =============================================================================

# Recent change by another author (within 2 days)
recent_change_by_other(File) :-
    git_history(File, _, Author, Age, _),
    current_user(CurrentUser),
    Author != CurrentUser,
    Age < 2.

# Chesterton's Fence warning - warn before deleting recently-changed code
chesterton_fence_warning(File, "recent_change_by_other") :-
    user_intent(_, /mutation, /delete, File, _),
    recent_change_by_other(File).

chesterton_fence_warning(File, "high_churn_file") :-
    user_intent(_, /mutation, /refactor, File, _),
    churn_rate(File, Freq),
    Freq > 5.

# Trigger clarification for Chesterton's Fence
clarification_needed(File) :-
    chesterton_fence_warning(File, _).

# =============================================================================
# SECTION 14: SHADOW MODE / COUNTERFACTUAL REASONING (§22)
# =============================================================================

# Helper for safe negation
has_projection_violation(ActionID) :-
    projection_violation(ActionID, _).

# Safe projection - action passes safety checks in shadow simulation
safe_projection(ActionID) :-
    shadow_state(_, ActionID, /valid),
    !has_projection_violation(ActionID).

# Projection violation detection
projection_violation(ActionID, "test_failure") :-
    simulated_effect(ActionID, "diagnostic", _),
    simulated_effect(ActionID, "diagnostic_severity", /error).

projection_violation(ActionID, "security_violation") :-
    simulated_effect(ActionID, "security_violation", _).

# Block action if projection fails
block_commit("shadow_simulation_failed") :-
    pending_mutation(MutationID, _, _, _),
    !safe_projection(MutationID).

# =============================================================================
# SECTION 15: INTERACTIVE DIFF APPROVAL (§23)
# =============================================================================

# Require approval for dangerous mutations
requires_approval(MutationID) :-
    pending_mutation(MutationID, File, _, _),
    chesterton_fence_warning(File, _).

requires_approval(MutationID) :-
    pending_mutation(MutationID, File, _, _),
    impacted(File).

# Helper for safe negation
is_mutation_approved(MutationID) :-
    mutation_approved(MutationID, _, _).

# Block mutation without approval
next_action(/ask_user) :-
    pending_mutation(MutationID, _, _, _),
    requires_approval(MutationID),
    !is_mutation_approved(MutationID).

# =============================================================================
# SECTION 16: SESSION STATE / CLARIFICATION LOOP (§20)
# =============================================================================

# Resume from clarification
next_action(/resume_task) :-
    session_state(_, /suspended, _),
    focus_clarification(_).

# Clear clarification when answered
# (Handled at runtime - logic marks session as active)

# =============================================================================
# SECTION 17: KNOWLEDGE ATOM INTEGRATION (§24)
# =============================================================================

# When high-confidence knowledge about the domain exists
# Knowledge atoms inform strategy selection (confidence on 0-100 scale)
active_strategy(/domain_expert) :-
    knowledge_atom(_, _, _, Confidence),
    Confidence > 80,
    user_intent(_, _, _, _, _).

# =============================================================================
# SECTION 17B: LEARNED KNOWLEDGE APPLICATION
# =============================================================================
# These rules leverage facts hydrated from knowledge.db by HydrateLearnings().
# The hydration happens during OODA Observe phase.

# 1. User preferences influence tool selection
# If user prefers a language, boost activation for related tools
activation(Tool, 85) :-
    learned_preference(/prefer_language, _),
    tool_capabilities(Tool, /code_generation),
    tool_language(Tool, _).

# 2. Learned constraints become safety checks
# Constraints from knowledge.db feed into constitutional logic
constraint_violation(Action, Reason) :-
    learned_constraint(Predicate, Args),
    action_violates(Action, Predicate, Args),
    Reason = Args.

# 3. User facts inform context
# Facts about the user/project activate relevant context
context_atom(fn:pair(Pred, Args)) :-
    learned_fact(Pred, Args),
    relevant_to_intent(Pred, Intent),
    user_intent(_, _, _, _, Intent).

# 4. Knowledge graph links spread activation
# Entity relationships from knowledge_graph propagate energy
activation(EntityB, 60) :-
    knowledge_link(EntityA, /related_to, EntityB),
    activation(EntityA, Score),
    Score > 50.

activation(EntityB, 70) :-
    knowledge_link(EntityA, /depends_on, EntityB),
    activation(EntityA, Score),
    Score > 40.

# 5. High-activation facts boost related content
# Recent activations from activation_log inform focus
context_priority(FactID, /high) :-
    activation(FactID, Score),
    Score > 70.

# 6. Session continuity - recent turns inform context
# Session history provides conversational context
context_atom(UserInput) :-
    session_turn(_, TurnNum, UserInput, _),
    TurnNum > 0.

# 7. Similar content retrieval for semantic search
# Vector recall results inform related context
related_context(Content) :-
    similar_content(Rank, Content),
    Rank < 5.

# =============================================================================
# SECTION 18: SHARD TYPE CLASSIFICATION (§6.1 Taxonomy)
# =============================================================================

# Type 1: System Level - Always on, high reliability
shard_type(/system, /permanent, /high_reliability).

# Type 2: Ephemeral - Fast spawning, RAM only
shard_type(/ephemeral, /spawn_die, /speed_optimized).

# Type 3: Persistent LLM-Created - Background tasks, SQLite
shard_type(/persistent, /long_running, /adaptive).

# Type 4: User Configured - Deep domain knowledge
shard_type(/user, /explicit, /user_defined).

# Model capability mapping for shards
shard_model_config(/system, /high_reasoning).
shard_model_config(/ephemeral, /high_speed).
shard_model_config(/persistent, /balanced).
shard_model_config(/user, /high_reasoning).

# =============================================================================
# SECTION 19: CAMPAIGN ORCHESTRATION POLICY
# =============================================================================
# Long-running, multi-phase goal execution with context management

# -----------------------------------------------------------------------------
# 19.1 Campaign State Machine
# -----------------------------------------------------------------------------

# Current campaign is the one that's active
current_campaign(CampaignID) :-
    campaign(CampaignID, _, _, _, /active).

# Campaign execution strategy activates when a campaign is active
active_strategy(/campaign_execution) :-
    current_campaign(_).

# -----------------------------------------------------------------------------
# 19.2 Phase Eligibility & Sequencing
# -----------------------------------------------------------------------------

# Helper: check if a phase has incomplete hard dependencies
has_incomplete_hard_dep(PhaseID) :-
    phase_dependency(PhaseID, DepPhaseID, /hard),
    campaign_phase(DepPhaseID, _, _, _, Status, _),
    /completed != Status.

# A phase is eligible when all hard dependencies are complete
phase_eligible(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    !has_incomplete_hard_dep(PhaseID).

# Helper: check if there's an earlier eligible phase
# Note: Order is bound by looking up PhaseID's order within the rule
has_earlier_phase(PhaseID) :-
    campaign_phase(PhaseID, _, _, Order, _, _),
    phase_eligible(OtherPhaseID),
    OtherPhaseID != PhaseID,
    campaign_phase(OtherPhaseID, _, _, OtherOrder, _, _),
    OtherOrder < Order.

# Current phase: lowest order eligible phase, or the one in progress
current_phase(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

current_phase(PhaseID) :-
    phase_eligible(PhaseID),
    !has_earlier_phase(PhaseID),
    !has_in_progress_phase().

# Helper: check if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID).

# Phase is blocked if it has incomplete hard dependencies
phase_blocked(PhaseID, "hard_dependency_incomplete") :-
    campaign_phase(PhaseID, CampaignID, _, _, /pending, _),
    current_campaign(CampaignID),
    has_incomplete_hard_dep(PhaseID).

# -----------------------------------------------------------------------------
# 19.3 Task Selection & Execution
# -----------------------------------------------------------------------------

# Helper: check if task has blocking dependencies
has_blocking_task_dep(TaskID) :-
    task_dependency(TaskID, BlockerID),
    campaign_task(BlockerID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Helper: check if task conflicts with an in-progress task
task_conflict_active(TaskID) :-
    task_conflict(TaskID, OtherTaskID),
    campaign_task(OtherTaskID, _, _, /in_progress, _).

task_conflict_active(TaskID) :-
    task_conflict(OtherTaskID, TaskID),
    campaign_task(OtherTaskID, _, _, /in_progress, _).

# Optional conflict heuristic: same artifact path -> conflict
task_conflict(TaskID, OtherTaskID) :-
    TaskID != OtherTaskID,
    task_artifact(TaskID, _, Path, _),
    task_artifact(OtherTaskID, _, Path, _).

# Helper: check if there's an earlier pending task
has_earlier_task(TaskID, PhaseID) :-
    campaign_task(OtherTaskID, PhaseID, _, /pending, _),
    OtherTaskID != TaskID,
    task_priority(OtherTaskID, OtherPriority),
    task_priority(TaskID, Priority),
    priority_higher(OtherPriority, Priority).

# Priority ordering helper (will be implemented as Go builtin)
# For now, we use simple rules
priority_higher(/critical, /high).
priority_higher(/critical, /normal).
priority_higher(/critical, /low).
priority_higher(/high, /normal).
priority_higher(/high, /low).
priority_higher(/normal, /low).

# Eligible tasks: highest-priority pending tasks in the current phase without blockers or conflicts
eligible_task(TaskID) :-
    current_phase(PhaseID),
    campaign_task(TaskID, PhaseID, _, /pending, _),
    !has_blocking_task_dep(TaskID),
    !has_earlier_task(TaskID, PhaseID),
    !task_conflict_active(TaskID).

# Next task remains available for single-dispatch clients
next_campaign_task(TaskID) :-
    eligible_task(TaskID).

# Derive next_action based on campaign task type
next_action(/campaign_create_file) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /file_create).

next_action(/campaign_modify_file) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /file_modify).

next_action(/campaign_write_test) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /test_write).

next_action(/campaign_run_test) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /test_run).

next_action(/campaign_research) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /research).

next_action(/campaign_verify) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /verify).

next_action(/campaign_document) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /document).

next_action(/campaign_refactor) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /refactor).

next_action(/campaign_integrate) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, /integrate).

# Auto-spawn researcher shard for research tasks
delegate_task(/researcher, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /research).

# Auto-spawn coder shard for file creation/modification
delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_create).

delegate_task(/coder, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /file_modify).

# Auto-spawn tester shard for test tasks
delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_write).

delegate_task(/tester, Description, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Description, _, /test_run).

# -----------------------------------------------------------------------------
# 19.4 Context Paging (Phase-Aware Spreading Activation)
# -----------------------------------------------------------------------------

# Boost activation for current phase context
activation(Fact, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Fact, _).

# Boost files matching current task's target
activation(Target, 140) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, _),
    task_artifact(TaskID, _, Target, _).

# Suppress context from completed phases
activation(Fact, -50) :-
    context_compression(PhaseID, _, _, _),
    phase_context_atom(PhaseID, Fact, _).

# -----------------------------------------------------------------------------
# 19.5 Checkpoint & Verification
# -----------------------------------------------------------------------------

# Helper: check if phase has pending checkpoint
has_pending_checkpoint(PhaseID) :-
    phase_objective(PhaseID, _, _, VerifyMethod),
    /none != VerifyMethod,
    !has_passed_checkpoint(PhaseID, VerifyMethod).

has_passed_checkpoint(PhaseID, CheckType) :-
    phase_checkpoint(PhaseID, CheckType, /true, _, _).

# Helper: check if all phase tasks are complete
has_incomplete_phase_task(PhaseID) :-
    campaign_task(_, PhaseID, _, Status, _),
    /completed != Status,
    /skipped != Status.

all_phase_tasks_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    !has_incomplete_phase_task(PhaseID).

# Trigger checkpoint when all tasks complete but checkpoint pending
next_action(/run_phase_checkpoint) :-
    current_phase(PhaseID),
    all_phase_tasks_complete(PhaseID),
    has_pending_checkpoint(PhaseID).

# Block phase completion if checkpoint failed
phase_blocked(PhaseID, "checkpoint_failed") :-
    phase_checkpoint(PhaseID, _, /false, _, _).

# -----------------------------------------------------------------------------
# 19.6 Replanning Triggers
# -----------------------------------------------------------------------------

# Helper: identify failed tasks (for counting in Go runtime)
failed_campaign_task(CampaignID, TaskID) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, PhaseID, Desc, /failed, TaskType),
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, Status, Profile).

# Trigger replan on repeated failures (threshold checked in Go runtime)
# The Go runtime counts failed_campaign_task facts and triggers replan if > 3
replan_needed(CampaignID, "task_failure_cascade") :-
    current_campaign(CampaignID),
    failed_campaign_task(CampaignID, TaskID1),
    failed_campaign_task(CampaignID, TaskID2),
    failed_campaign_task(CampaignID, TaskID3),
    TaskID1 != TaskID2,
    TaskID2 != TaskID3,
    TaskID1 != TaskID3.

# Trigger replan if user provides new instruction during campaign
replan_needed(CampaignID, "user_instruction") :-
    current_campaign(CampaignID),
    user_intent(_, /instruction, _, _, _).

# Trigger replan if explicit trigger exists
replan_needed(CampaignID, Reason) :-
    replan_trigger(CampaignID, Reason, _).

# Pause and replan action
next_action(/pause_and_replan) :-
    replan_needed(_, _).

# -----------------------------------------------------------------------------
# 19.7 Campaign Helpers for Safe Negation
# -----------------------------------------------------------------------------

# Helper: true if any phase is eligible to start
has_eligible_phase() :-
    phase_eligible(_).

# Helper: true if any phase is in progress
has_in_progress_phase() :-
    campaign_phase(_, _, _, _, /in_progress, _).

# Helper: true if there's a next campaign task available
has_next_campaign_task() :-
    next_campaign_task(_).

# Helper: check if any phase is not complete
has_incomplete_phase(CampaignID) :-
    campaign_phase(_, CampaignID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# Campaign complete when all phases complete
campaign_complete(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID).

next_action(/campaign_complete) :-
    campaign_complete(_).

# -----------------------------------------------------------------------------
# 19.8 Campaign Blocking Conditions
# -----------------------------------------------------------------------------

# Campaign blocked if no eligible phases and none in progress
campaign_blocked(CampaignID, "no_eligible_phases") :-
    current_campaign(CampaignID),
    !has_eligible_phase(),
    !has_in_progress_phase(),
    has_incomplete_phase(CampaignID).

# Campaign blocked if all remaining tasks are blocked
campaign_blocked(CampaignID, "all_tasks_blocked") :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    !has_next_campaign_task(),
    has_incomplete_phase_task(PhaseID).

# -----------------------------------------------------------------------------
# 19.9 Autopoiesis During Campaign
# -----------------------------------------------------------------------------

# Track successful phase types for learning (Go runtime extracts from kernel)
phase_success_pattern(PhaseType) :-
    campaign_phase(PhaseID, CampaignID, PhaseName, Seq, /completed, Profile),
    phase_objective(PhaseID, PhaseType, Desc, Priority),
    phase_checkpoint(PhaseID, CheckpointID, /true, ValidatedAt, ValidatorShard).

# Learn from phase completion - promotes success pattern for phase type
promote_to_long_term(/phase_success, PhaseType) :-
    phase_success_pattern(PhaseType).

# Learn from task failures for future avoidance
campaign_learning(CampaignID, /failure_pattern, TaskType, ErrorMsg, Now) :-
    current_campaign(CampaignID),
    campaign_task(TaskID, _, _, /failed, TaskType),
    task_error(TaskID, _, ErrorMsg),
    current_time(Now).

# -----------------------------------------------------------------------------
# 19.10 Campaign-Aware Tool Permissions
# -----------------------------------------------------------------------------

# During campaigns, only permit tools in the phase's context profile
phase_tool_permitted(Tool) :-
    current_phase(PhaseID),
    campaign_phase(PhaseID, _, _, _, _, ContextProfile),
    context_profile(ContextProfile, _, RequiredTools, _),
    tool_in_list(Tool, RequiredTools).

# Block tools not in phase profile during active campaign
# (This is advisory - Go runtime can override for safety)
# Note: Tool is bound via tool_capabilities before negation check
tool_advisory_block(Tool, "not_in_phase_profile") :-
    current_campaign(_),
    current_phase(_),
    tool_capabilities(Tool, _),
    !phase_tool_permitted(Tool).

# =============================================================================
# SECTION 20: CAMPAIGN START TRIGGER
# =============================================================================

# Trigger campaign mode when user wants to start a campaign
active_strategy(/campaign_planning) :-
    user_intent(_, /mutation, /campaign, _, _).

# Alternative triggers for campaign-like requests
active_strategy(/campaign_planning) :-
    user_intent(_, /mutation, /build, Target, _),
    target_is_large(Target).

active_strategy(/campaign_planning) :-
    user_intent(_, /mutation, /implement, Target, _),
    target_is_complex(Target).

# Heuristics for complexity (implemented in Go builtins)
# target_is_large(Target) - true if target references multiple files/features
# target_is_complex(Target) - true if target requires multiple phases

# =============================================================================
# SECTION 21: SYSTEM SHARD COORDINATION
# =============================================================================
# Coordinates the 6 system shards: perception_firewall, executive_policy,
# constitution_gate, world_model_ingestor, tactile_router, session_planner.
# These are Type 1 (permanent/continuous) shards that form the OODA loop.

# -----------------------------------------------------------------------------
# 21.1 Intent Processing Flow (Perception → Executive)
# -----------------------------------------------------------------------------

# A user_intent is pending if not yet processed by executive
pending_intent(IntentID) :-
    user_intent(IntentID, _, _, _, _),
    !intent_processed(IntentID).

# Helper for safe negation
intent_processed(IntentID) :-
    processed_intent(IntentID).

# Focus needs resolution if confidence is low (score on 0-100 scale)
focus_needs_resolution(Ref) :-
    focus_resolution(Ref, _, _, Score),
    Score < 70.

# Intent ready for executive processing
intent_ready_for_executive(IntentID) :-
    user_intent(IntentID, _, _, Target, _),
    !focus_needs_resolution(Target).

# -----------------------------------------------------------------------------
# 21.2 Action Flow (Executive → Constitution → Router)
# -----------------------------------------------------------------------------

# Action is pending permission check from constitution gate
action_pending_permission(ActionID) :-
    pending_permission_check(ActionID),
    !permission_checked(ActionID).

# Helper for safe negation
permission_checked(ActionID) :-
    permission_check_result(ActionID, _, _).

# Action is permitted by constitution gate
action_permitted(ActionID) :-
    permission_check_result(ActionID, /permit, _).

# Action is blocked by constitution gate
action_blocked(ActionID, Reason) :-
    permission_check_result(ActionID, /deny, Reason).

# Action ready for routing (permitted and not yet routed)
action_ready_for_routing(ActionID) :-
    action_permitted(ActionID),
    !action_routed(ActionID).

# Helper for safe negation
action_routed(ActionID) :-
    ready_for_routing(ActionID),
    routing_result(ActionID, _, _).

# Derive routing result success
routing_succeeded(ActionID) :-
    routing_result(ActionID, /success, _).

# Derive routing result failure
routing_failed(ActionID, Error) :-
    routing_result(ActionID, /failure, Error).

# -----------------------------------------------------------------------------
# 21.3 System Shard Health Monitoring
# -----------------------------------------------------------------------------

# System shard is healthy if heartbeat within threshold (30 seconds)
# Note: Using fn:minus directly since time_diff would need bound variables.
# Assumes Now >= Timestamp (current time always after heartbeat time).
system_shard_healthy(ShardName) :-
    system_heartbeat(ShardName, Timestamp),
    current_time(Now),
    Now >= Timestamp,
    Diff = fn:minus(Now, Timestamp),
    Diff < 30.

# Helper: check if shard has no recent heartbeat
shard_heartbeat_stale(ShardName) :-
    system_shard(ShardName, _),
    !system_shard_healthy(ShardName).

# Escalate if critical system shard is unhealthy
escalation_needed(/system_health, ShardName, "heartbeat_timeout") :-
    shard_heartbeat_stale(ShardName),
    system_startup(ShardName, /auto).

# System shards that must auto-start
system_startup(/perception_firewall, /auto).
system_startup(/executive_policy, /auto).
system_startup(/constitution_gate, /auto).
system_startup(/world_model_ingestor, /on_demand).
system_startup(/tactile_router, /on_demand).
system_startup(/session_planner, /on_demand).

# -----------------------------------------------------------------------------
# 21.4 Safety Violation Handling (Constitution Gate)
# -----------------------------------------------------------------------------

# A safety violation blocks all further actions
block_all_actions("safety_violation") :-
    safety_violation(_, _, _, _).

# Security anomaly triggers investigation
# Need to check if there are uninvestigated anomalies
next_action(/investigate_anomaly) :-
    security_anomaly(AnomalyID, _, _),
    !anomaly_investigated(AnomalyID).

# Helper for safe negation
anomaly_investigated(AnomalyID) :-
    security_anomaly(AnomalyID, _, _),
    investigation_result(AnomalyID, _).

# Pattern recognition for repeated violations
repeated_violation_pattern(Pattern) :-
    safety_violation(_, Pattern, _, _),
    violation_count(Pattern, Count),
    Count >= 3.

# Propose rule when pattern detected (Autopoiesis)
propose_safety_rule(Pattern) :-
    repeated_violation_pattern(Pattern).

# -----------------------------------------------------------------------------
# 21.5 World Model Updates (World Model Ingestor)
# -----------------------------------------------------------------------------

# File change triggers world model update
# Note: Using fn:minus directly for time difference calculation.
world_model_stale(File) :-
    modified(File),
    file_topology(File, _, _, LastUpdate, _),
    current_time(Now),
    Now >= LastUpdate,
    Diff = fn:minus(Now, LastUpdate),
    Diff > 5.

# Trigger ingestor when world model is stale
next_action(/update_world_model) :-
    world_model_stale(_),
    system_shard_healthy(/world_model_ingestor).

# File topology derived from filesystem
file_in_project(File) :-
    file_topology(File, _, _, _, _).

# Symbol graph connectivity (uses dependency_link for edges)
symbol_reachable(From, To) :-
    dependency_link(From, To, _).

symbol_reachable(From, To) :-
    dependency_link(From, Mid, _),
    symbol_reachable(Mid, To).

# -----------------------------------------------------------------------------
# 21.6 Routing Table (Tactile Router)
# -----------------------------------------------------------------------------

# Default routing table entries (can be extended via Autopoiesis)
routing_table(/fs_read, /read_file, /low).
routing_table(/fs_write, /write_file, /medium).
routing_table(/exec_cmd, /execute_command, /high).
routing_table(/browser, /browser_action, /high).
routing_table(/code_graph, /analyze_code, /low).

# Tool is allowed for action type
tool_allowed(Tool, ActionType) :-
    routing_table(ActionType, Tool, _),
    tool_allowlist(Tool, _).

# Route action to appropriate tool
route_action(ActionID, Tool) :-
    action_ready_for_routing(ActionID),
    action_type(ActionID, ActionType),
    tool_allowed(Tool, ActionType).

# Routing blocked if no tool available
routing_blocked(ActionID, "no_tool_available") :-
    action_ready_for_routing(ActionID),
    action_type(ActionID, ActionType),
    !has_tool_for_action(ActionType).

# Helper for safe negation
has_tool_for_action(ActionType) :-
    tool_allowed(_, ActionType).

# -----------------------------------------------------------------------------
# 21.7 Session Planning (Session Planner)
# -----------------------------------------------------------------------------

# Agenda item is ready when dependencies complete
agenda_item_ready(ItemID) :-
    agenda_item(ItemID, _, _, /pending, _),
    !has_incomplete_dependency(ItemID).

# Helper for dependency checking
has_incomplete_dependency(ItemID) :-
    agenda_dependency(ItemID, DepID),
    agenda_item(DepID, _, _, Status, _),
    /completed != Status.

# Next agenda item: highest priority ready item
next_agenda_item(ItemID) :-
    agenda_item_ready(ItemID),
    !has_higher_priority_item(ItemID).

# Helper for priority ordering
has_higher_priority_item(ItemID) :-
    agenda_item(ItemID, _, Priority, _, _),
    agenda_item_ready(OtherID),
    OtherID != ItemID,
    agenda_item(OtherID, _, OtherPriority, _, _),
    OtherPriority > Priority.

# Checkpoint needed based on time or completion (10 minutes = 600 seconds)
checkpoint_due() :-
    last_checkpoint_time(LastTime),
    current_time(Now),
    Now >= LastTime,
    Diff = fn:minus(Now, LastTime),
    Diff > 600.

next_action(/create_checkpoint) :-
    checkpoint_due().

# Blocked item triggers escalation after retries
agenda_item_escalate(ItemID, "max_retries_exceeded") :-
    agenda_item(ItemID, _, _, /blocked, _),
    item_retry_count(ItemID, Count),
    Count >= 3.

escalation_needed(/session_planner, ItemID, Reason) :-
    agenda_item_escalate(ItemID, Reason).

# -----------------------------------------------------------------------------
# 21.8 On-Demand Shard Activation
# -----------------------------------------------------------------------------

# Activate world_model_ingestor when files change
activate_shard(/world_model_ingestor) :-
    modified(_),
    !system_shard_healthy(/world_model_ingestor).

# Activate tactile_router when actions are pending
activate_shard(/tactile_router) :-
    action_ready_for_routing(_),
    !system_shard_healthy(/tactile_router).

# Activate session_planner for campaigns or complex goals
activate_shard(/session_planner) :-
    current_campaign(_),
    !system_shard_healthy(/session_planner).

activate_shard(/session_planner) :-
    user_intent(_, _, /plan, _, _),
    !system_shard_healthy(/session_planner).

# -----------------------------------------------------------------------------
# 21.9 Autopoiesis Integration for System Shards
# -----------------------------------------------------------------------------

# Unhandled case tracking (for rule learning)
unhandled_case_count(ShardName, Count) :-
    system_shard(ShardName, _),
    unhandled_cases(ShardName, Cases),
    list_length(Cases, Count).

# Trigger LLM for rule proposal when threshold reached
propose_new_rule(ShardName) :-
    unhandled_case_count(ShardName, Count),
    Count >= 3.

# Proposed rule needs human approval if low confidence (confidence on 0-100 scale)
rule_needs_approval(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence < 80.

# Auto-apply rule if high confidence (confidence on 0-100 scale)
auto_apply_rule(RuleID) :-
    proposed_rule(RuleID, _, _, Confidence),
    Confidence >= 80,
    !rule_applied(RuleID).

# Helper for safe negation
rule_applied(RuleID) :-
    applied_rule(RuleID, _).

# Learn from successful rule applications
learning_signal(/rule_success, RuleID) :-
    applied_rule(RuleID, Timestamp),
    rule_outcome(RuleID, /success, _).

# -----------------------------------------------------------------------------
# 21.10 OODA Loop Coordination
# -----------------------------------------------------------------------------

# OODA phases: Observe → Orient → Decide → Act
ooda_phase(/observe) :-
    pending_intent(IntentID),
    !intent_ready_for_executive(IntentID).

ooda_phase(/orient) :-
    intent_ready_for_executive(IntentID),
    pending_intent(IntentID),
    !has_next_action().

ooda_phase(/decide) :-
    has_next_action(),
    next_action(ActionID),
    !action_permitted(ActionID).

ooda_phase(/act) :-
    action_ready_for_routing(_).

# Helper for OODA phase detection
has_next_action() :-
    next_action(_).

# Current OODA state for debugging/monitoring
current_ooda_phase(Phase) :-
    ooda_phase(Phase).

# OODA loop stalled detection (30 second threshold)
ooda_stalled(Reason) :-
    pending_intent(_),
    !has_next_action(),
    current_time(Now),
    last_action_time(LastTime),
    Now >= LastTime,
    Diff = fn:minus(Now, LastTime),
    Diff > 30,
    Reason = "no_action_derived".

# Escalate stalled OODA loop
escalation_needed(/ooda_loop, "stalled", Reason) :-
    ooda_stalled(Reason).

# =============================================================================
# SECTION 22: CODE DOM RULES
# =============================================================================
# Rules for semantic code element operations - treating code like a DOM.
# These rules coordinate file scope, element queries, and edit tracking.

# -----------------------------------------------------------------------------
# 22.1 File Scope Rules
# -----------------------------------------------------------------------------

# A file is in scope if it's the active file
in_scope(File) :- active_file(File).

# A file is in scope if the active file imports it
in_scope(File) :-
    active_file(ActiveFile),
    dependency_link(ActiveFile, File, _).

# A file is in scope if it imports the active file
in_scope(File) :-
    active_file(ActiveFile),
    dependency_link(File, ActiveFile, _).

# -----------------------------------------------------------------------------
# 22.2 Element Accessibility Rules
# -----------------------------------------------------------------------------

# An element is editable if its file is in scope and it has replace action
editable(Ref) :-
    code_element(Ref, _, File, _, _),
    in_scope(File),
    code_interactable(Ref, /replace).

# All functions in scope (for querying)
function_in_scope(Ref, File, Sig) :-
    code_element(Ref, /function, File, _, _),
    in_scope(File),
    element_signature(Ref, Sig).

# All methods in scope
method_in_scope(Ref, File, Sig) :-
    code_element(Ref, /method, File, _, _),
    in_scope(File),
    element_signature(Ref, Sig).

# Method belongs to struct
method_of(MethodRef, StructRef) :- element_parent(MethodRef, StructRef).

# -----------------------------------------------------------------------------
# 22.3 Transitive Element Containment
# -----------------------------------------------------------------------------

# Direct containment
code_contains(Parent, Child) :- element_parent(Child, Parent).

# Transitive containment
code_contains(Ancestor, Descendant) :-
    element_parent(Mid, Ancestor),
    code_contains(Mid, Descendant).

# -----------------------------------------------------------------------------
# 22.4 Safety and Complexity Rules
# -----------------------------------------------------------------------------

# Safe to modify: element is in scope (implicitly has context)
safe_to_modify(Ref) :-
    code_element(Ref, _, File, _, _),
    in_scope(File).

# Helper: count elements for complexity analysis (evaluated in Go runtime)
# Note: Mangle doesn't have != operator, so we use virtual predicate for counting
element_count_high() :-
    code_element(Ref1, _, _, _, _),
    code_element(Ref2, _, _, _, _),
    code_element(Ref3, _, _, _, _),
    code_element(Ref4, _, _, _, _),
    code_element(Ref5, _, _, _, _).

# Trigger campaign for complex refactors affecting many elements
requires_campaign(Intent) :-
    user_intent(Intent, /mutation, _, Target, _),
    in_scope(Target),
    element_count_high().

# -----------------------------------------------------------------------------
# 22.5 Next Action Derivation for Code DOM
# -----------------------------------------------------------------------------

# Open file when targeting a file that's not yet in scope
next_action(/open_file) :-
    user_intent(_, _, _, Target, _),
    file_topology(Target, _, /go, _, _),
    !active_file(Target),
    !in_scope(Target).

# Query elements when file is open and we need to find something
next_action(/query_elements) :-
    active_file(_),
    user_intent(_, /query, _, _, _).

# Edit element when mutation targets a known element
next_action(/edit_element) :-
    user_intent(_, /mutation, _, Ref, _),
    code_element(Ref, _, File, _, _),
    in_scope(File).

# Refresh scope after external changes
next_action(/refresh_scope) :-
    active_file(File),
    modified(File),
    !scope_refreshed(File).

# Helper for safe negation
scope_refreshed(File) :-
    file_in_scope(File, _, _, _),
    !modified(File).

# -----------------------------------------------------------------------------
# 22.6 Learning From Code Edits
# -----------------------------------------------------------------------------

# Track edit outcomes for learning
successful_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /true, _).

failed_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /false, _).

# Proven safe: edit pattern has succeeded multiple times (3+)
# Note: Exact counting done in Go runtime; this is a heuristic trigger
proven_safe_edit(Ref, EditType) :-
    code_edit_outcome(Ref, EditType, /true, _),
    code_edit_outcome(Ref2, EditType, /true, _),
    code_edit_outcome(Ref3, EditType, /true, _),
    Ref != Ref2, Ref2 != Ref3, Ref != Ref3.

# Promote to long-term memory when edit pattern is proven
promote_to_long_term(/edit_pattern, EditType) :-
    proven_safe_edit(_, EditType).

# -----------------------------------------------------------------------------
# 22.7 Code DOM Activation Rules
# -----------------------------------------------------------------------------

# Boost activation for elements in the active file
activation(Ref, 100) :-
    code_element(Ref, _, File, _, _),
    active_file(File).

# Boost activation for elements matching current intent target
activation(Ref, 120) :-
    code_element(Ref, _, _, _, _),
    user_intent(_, _, _, Ref, _).

# Suppress activation for elements in files outside scope
activation(Ref, -50) :-
    code_element(Ref, _, File, _, _),
    file_topology(File, _, _, _, _),
    !in_scope(File).

# -----------------------------------------------------------------------------
# 22.8 Edit Safety & Risk Assessment Rules
# -----------------------------------------------------------------------------

# Public function has external callers (exported = can be called from outside)
has_external_callers(Ref) :-
    code_element(Ref, /function, _, _, _),
    element_visibility(Ref, /public).

has_external_callers(Ref) :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public).

# Breaking change risk: HIGH for public API functions
breaking_change_risk(Ref, /high, "public_api") :-
    has_external_callers(Ref),
    api_handler_function(Ref, _, _).

# Breaking change risk: HIGH for public interface methods
breaking_change_risk(Ref, /high, "interface_contract") :-
    code_element(Ref, /method, _, _, _),
    element_visibility(Ref, /public),
    element_parent(Ref, InterfaceRef),
    code_element(InterfaceRef, /interface, _, _, _).

# Breaking change risk: MEDIUM for public functions in libraries
breaking_change_risk(Ref, /medium, "public_function") :-
    has_external_callers(Ref),
    !api_handler_function(Ref, _, _).

# Breaking change risk: LOW for private elements
breaking_change_risk(Ref, /low, "private") :-
    code_element(Ref, _, _, _, _),
    element_visibility(Ref, /private).

# Breaking change risk: CRITICAL for generated code
breaking_change_risk(Ref, /critical, "generated_will_be_overwritten") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: generated code will be overwritten
edit_unsafe(Ref, "generated_code") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Edit unsafe: CGo requires special handling
edit_unsafe(Ref, "cgo_code") :-
    code_element(Ref, _, File, _, _),
    cgo_code(File).

# Edit unsafe: file has hash mismatch (concurrent modification)
edit_unsafe(Ref, "concurrent_modification") :-
    code_element(Ref, _, File, _, _),
    file_hash_mismatch(File, _, _).

# Edit unsafe: element is stale
edit_unsafe(Ref, "stale_reference") :-
    element_stale(Ref, _).

# -----------------------------------------------------------------------------
# 22.9 Mock & Interface Rules
# -----------------------------------------------------------------------------

# Struct implements interface if it has methods matching interface methods
# (Simplified: if struct has methods and interface exists in same package)
interface_impl(StructRef, InterfaceRef) :-
    code_element(StructRef, /struct, File, _, _),
    code_element(InterfaceRef, /interface, File, _, _),
    element_parent(MethodRef, StructRef),
    code_element(MethodRef, /method, _, _, _).

# Test file mocks source file if it's a _test.go in same package
mock_file(TestFile, SourceFile) :-
    file_topology(TestFile, _, /go, _, _),
    file_topology(SourceFile, _, /go, _, _),
    TestFile != SourceFile.
    # Note: Actual mock detection needs content analysis in Go runtime

# Suggest updating mocks when source function signature changes
suggest_update_mocks(Ref) :-
    code_element(Ref, /function, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    mock_file(TestFile, File).

suggest_update_mocks(Ref) :-
    code_element(Ref, /method, File, _, _),
    element_visibility(Ref, /public),
    element_modified(Ref, _, _),
    mock_file(TestFile, File).

# -----------------------------------------------------------------------------
# 22.10 Scope Staleness Detection
# -----------------------------------------------------------------------------

# File modified externally: hash doesn't match what we loaded
file_modified_externally(Path) :-
    file_hash_mismatch(Path, _, _).

# Scope needs refresh when any in-scope file was modified
needs_scope_refresh() :-
    active_file(ActiveFile),
    in_scope(File),
    modified(File).

needs_scope_refresh() :-
    file_modified_externally(_).

# Element edit blocked due to concurrent modification
element_edit_blocked(Ref, "concurrent_modification") :-
    code_element(Ref, _, File, _, _),
    file_modified_externally(File).

# Element edit blocked due to generated code
element_edit_blocked(Ref, "generated_code") :-
    code_element(Ref, _, File, _, _),
    generated_code(File, _, _).

# Element edit blocked due to parse error
element_edit_blocked(Ref, "parse_error") :-
    code_element(Ref, _, File, _, _),
    parse_error(File, _, _).

# -----------------------------------------------------------------------------
# 22.11 API Client/Handler Awareness
# -----------------------------------------------------------------------------

# API client functions should be tested carefully
requires_integration_test(Ref) :-
    api_client_function(Ref, _, _).

# API handlers need contract validation
requires_contract_check(Ref) :-
    api_handler_function(Ref, _, _),
    element_modified(Ref, _, _).

# Warn when editing API client without corresponding test
api_edit_warning(Ref, "no_integration_test") :-
    api_client_function(Ref, _, _),
    element_modified(Ref, _, _).

# =============================================================================
# SECTION 23: VERIFICATION LOOP POLICY (Post-Execution Quality Enforcement)
# =============================================================================
# Implements the quality-enforcing verification loop that ensures tasks are
# completed PROPERLY - no shortcuts, no mock code, no corner-cutting.
# Per plan: Execute → Verify → Corrective Action → Retry (max 3)

# -----------------------------------------------------------------------------
# 23.1 Verification State Management
# -----------------------------------------------------------------------------

# Task has any quality violation
has_quality_violation(TaskID) :-
    quality_violation(TaskID, _).

# Task passed verification (successful with no violations)
verification_succeeded(TaskID) :-
    verification_attempt(TaskID, _, /success),
    !has_quality_violation(TaskID).

# Block further execution after max retries (3 attempts)
verification_blocked(TaskID) :-
    verification_attempt(TaskID, 3, /failure).

# -----------------------------------------------------------------------------
# 23.2 Corrective Action Triggers
# -----------------------------------------------------------------------------

# Task needs corrective action if verification failed and not blocked
needs_corrective_action(TaskID) :-
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum < 3,
    !verification_blocked(TaskID).

# Specific corrective action based on violation type

# Mock code → needs research for real implementation
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /mock_code),
    needs_corrective_action(TaskID).

# Placeholder code → needs research for proper implementation
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /placeholder),
    needs_corrective_action(TaskID).

# Hallucinated API → needs documentation lookup
next_action(/corrective_docs) :-
    current_task(TaskID),
    quality_violation(TaskID, /hallucinated_api),
    needs_corrective_action(TaskID).

# Incomplete implementation → may need decomposition
next_action(/corrective_decompose) :-
    current_task(TaskID),
    quality_violation(TaskID, /incomplete),
    needs_corrective_action(TaskID).

# Missing error handling → needs docs/examples
next_action(/corrective_docs) :-
    current_task(TaskID),
    quality_violation(TaskID, /missing_errors),
    needs_corrective_action(TaskID).

# Fake tests → needs research on testing patterns
next_action(/corrective_research) :-
    current_task(TaskID),
    quality_violation(TaskID, /fake_tests),
    needs_corrective_action(TaskID).

# -----------------------------------------------------------------------------
# 23.3 Escalation Logic
# -----------------------------------------------------------------------------

# Escalation required when max retries exceeded
escalation_required(TaskID, "max_retries_exceeded") :-
    verification_blocked(TaskID),
    current_task(TaskID).

# Escalation triggers next_action
next_action(/escalate_to_user) :-
    escalation_required(_, _).

# Block all other actions during escalation
block_all_actions("verification_escalation") :-
    escalation_required(_, _).

# -----------------------------------------------------------------------------
# 23.4 Learning Signals from Quality Violations
# -----------------------------------------------------------------------------

# Learn to avoid mock code patterns
learning_signal(/avoid_mock_code) :-
    quality_violation(_, /mock_code).

# Learn to avoid placeholder patterns
learning_signal(/avoid_placeholders) :-
    quality_violation(_, /placeholder).

# Learn to avoid hallucinated APIs
learning_signal(/avoid_hallucinated_api) :-
    quality_violation(_, /hallucinated_api).

# Learn to avoid incomplete implementations
learning_signal(/avoid_incomplete) :-
    quality_violation(_, /incomplete).

# Learn to avoid fake tests
learning_signal(/avoid_fake_tests) :-
    quality_violation(_, /fake_tests).

# Learn to include error handling
learning_signal(/require_error_handling) :-
    quality_violation(_, /missing_errors).

# Promote learning signals to long-term memory after repeated violations
promote_to_long_term(/quality_pattern, ViolationType) :-
    quality_violation(Task1, ViolationType),
    quality_violation(Task2, ViolationType),
    quality_violation(Task3, ViolationType),
    Task1 != Task2,
    Task2 != Task3,
    Task1 != Task3.

# -----------------------------------------------------------------------------
# 23.5 Shard Selection for Retries
# -----------------------------------------------------------------------------

# Prefer specialist shards for complex tasks after failure
delegate_task(/specialist, TaskID, /pending) :-
    current_task(TaskID),
    verification_attempt(TaskID, AttemptNum, /failure),
    AttemptNum >= 1,
    shard_profile(_, /specialist, _).

# Delegate to researcher when documentation is needed
delegate_task(/researcher, Query, /pending) :-
    current_task(TaskID),
    corrective_query(TaskID, _, Query),
    corrective_action_taken(TaskID, /research).

# Delegate to researcher when docs lookup needed
delegate_task(/researcher, Query, /pending) :-
    current_task(TaskID),
    corrective_query(TaskID, _, Query),
    corrective_action_taken(TaskID, /docs).

# -----------------------------------------------------------------------------
# 23.6 Verification Strategy Selection
# -----------------------------------------------------------------------------

# Activate verification strategy when task has been executed
active_strategy(/verification_loop) :-
    current_task(TaskID),
    verification_attempt(TaskID, _, _).

# Block normal task execution when in verification failure state
execution_blocked("verification_in_progress") :-
    current_task(TaskID),
    needs_corrective_action(TaskID).

# -----------------------------------------------------------------------------
# 23.7 Quality Gate Integration with Commit Barrier
# -----------------------------------------------------------------------------

# Block commit if any task has unresolved quality violations
block_commit("Quality Violations") :-
    has_quality_violation(_).

# Block commit if verification is blocked (max retries reached)
block_commit("Verification Failed") :-
    verification_blocked(_).

# -----------------------------------------------------------------------------
# 23.8 Context Enrichment Tracking
# -----------------------------------------------------------------------------

# Track successful corrective actions for learning
corrective_action_effective(TaskID, ActionType) :-
    corrective_action_taken(TaskID, ActionType),
    verification_attempt(TaskID, AttemptNum, /failure),
    verification_attempt(TaskID, NextAttempt, /success),
    NextAttempt > AttemptNum.

# Learn from effective corrective actions
learning_signal(/effective_correction, ActionType) :-
    corrective_action_effective(_, ActionType).

# -----------------------------------------------------------------------------
# 23.9 Verification Metrics (for monitoring)
# -----------------------------------------------------------------------------

# Helper: track tasks that succeeded on first attempt
first_attempt_success(TaskID) :-
    verification_attempt(TaskID, 1, /success),
    !has_quality_violation(TaskID).

# Helper: track tasks that required retries
required_retry(TaskID) :-
    verification_attempt(TaskID, AttemptNum, /success),
    AttemptNum > 1.

# Helper: track specific violation types for analytics
violation_type_count_high(ViolationType) :-
    quality_violation(T1, ViolationType),
    quality_violation(T2, ViolationType),
    quality_violation(T3, ViolationType),
    quality_violation(T4, ViolationType),
    quality_violation(T5, ViolationType),
    T1 != T2, T2 != T3, T3 != T4, T4 != T5.

# Trigger rule proposal for high-frequency violations
propose_new_rule(/verification_policy) :-
    violation_type_count_high(_).

# =============================================================================
# SECTION 24: REASONING TRACE POLICY (Shard Learning from Traces)
# =============================================================================
# Rules for analyzing shard LLM traces and deriving learning signals.
# Covers all 4 shard types: system, ephemeral, LLM-created, user-created specialists.

# -----------------------------------------------------------------------------
# 24.1 Trace Quality Tracking
# -----------------------------------------------------------------------------

# Low quality trace (needs review) - score on 0-100 scale
low_quality_trace(TraceID) :-
    trace_quality(TraceID, Score),
    Score < 50.

# High quality trace (good for learning) - score on 0-100 scale
high_quality_trace(TraceID) :-
    trace_quality(TraceID, Score),
    Score >= 80.

# -----------------------------------------------------------------------------
# 24.2 Shard Performance Patterns
# -----------------------------------------------------------------------------

# Shard has high failure rate (3+ consecutive failures)
shard_struggling(ShardType) :-
    reasoning_trace(T1, ShardType, _, _, /false, _),
    reasoning_trace(T2, ShardType, _, _, /false, _),
    reasoning_trace(T3, ShardType, _, _, /false, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Shard is performing well (5+ consecutive successes)
shard_performing_well(ShardType) :-
    reasoning_trace(T1, ShardType, _, _, /true, _),
    reasoning_trace(T2, ShardType, _, _, /true, _),
    reasoning_trace(T3, ShardType, _, _, /true, _),
    reasoning_trace(T4, ShardType, _, _, /true, _),
    reasoning_trace(T5, ShardType, _, _, /true, _),
    T1 != T2,
    T2 != T3,
    T3 != T4,
    T4 != T5.

# Detect slow reasoning (> 30 seconds)
slow_reasoning_detected(ShardType) :-
    reasoning_trace(_, ShardType, _, _, _, DurationMs),
    DurationMs > 30000.

# -----------------------------------------------------------------------------
# 24.3 Learning Signals from Traces
# -----------------------------------------------------------------------------

# Learn from repeated failures - shard needs help
learning_from_traces(/shard_needs_help, ShardType) :-
    shard_struggling(ShardType).

# Learn from success patterns
learning_from_traces(/success_pattern, ShardType) :-
    shard_performing_well(ShardType).

# Learn from slow traces (performance issue)
learning_from_traces(/slow_reasoning, ShardType) :-
    slow_reasoning_detected(ShardType).

# Promote learning signals to long-term memory
promote_to_long_term(/shard_pattern, ShardType) :-
    learning_from_traces(_, ShardType).

# -----------------------------------------------------------------------------
# 24.4 Cross-Shard Learning (Specialist vs Ephemeral)
# -----------------------------------------------------------------------------

# Specialist outperforms ephemeral for same task type
specialist_outperforms(SpecialistName, TaskType) :-
    reasoning_trace(T1, SpecialistName, /specialist, _, /true, _),
    reasoning_trace(T2, /coder, /ephemeral, _, /false, _),
    trace_task_type(T1, TaskType),
    trace_task_type(T2, TaskType).

# Suggest using specialist instead of ephemeral
suggest_use_specialist(TaskType, SpecialistName) :-
    specialist_outperforms(SpecialistName, TaskType).

# Suggest switching shard when current one struggles
shard_switch_suggestion(TaskType, CurrentShard, AlternateShard) :-
    shard_struggling(CurrentShard),
    shard_performing_well(AlternateShard),
    shard_can_handle(AlternateShard, TaskType).

# -----------------------------------------------------------------------------
# 24.5 Trace-Based Context Enhancement
# -----------------------------------------------------------------------------

# Boost activation for successful trace patterns in current session
activation(TraceID, 80) :-
    high_quality_trace(TraceID),
    reasoning_trace(TraceID, ShardType, _, SessionID, /true, _),
    session_state(SessionID, /active, _).

# Suppress failed trace patterns
activation(TraceID, -30) :-
    low_quality_trace(TraceID),
    reasoning_trace(TraceID, _, _, _, /false, _).

# -----------------------------------------------------------------------------
# 24.6 Corrective Actions Based on Traces
# -----------------------------------------------------------------------------

# Escalate if multiple shards struggling
escalation_needed(/system_health, /shard_performance, "Multiple shards struggling") :-
    shard_struggling(Shard1),
    shard_struggling(Shard2),
    Shard1 != Shard2.

# Suggest spawning researcher for failed traces with unknown errors
delegate_task(/researcher, TaskContext, /pending) :-
    reasoning_trace(TraceID, _, _, _, /false, _),
    trace_error(TraceID, /unknown),
    trace_task_type(TraceID, TaskContext).

# -----------------------------------------------------------------------------
# 24.7 System Shard Trace Monitoring
# -----------------------------------------------------------------------------

# System shard traces get special attention
activation(TraceID, 90) :-
    reasoning_trace(TraceID, _, /system, _, _, _).

# Alert on system shard failures
escalation_needed(/system_health, ShardType, "System shard failure") :-
    reasoning_trace(_, ShardType, /system, _, /false, _).

# -----------------------------------------------------------------------------
# 24.8 Specialist Knowledge Hydration from Traces
# -----------------------------------------------------------------------------

# Specialist with good traces should be preferred for similar tasks
delegate_task(SpecialistName, Task, /pending) :-
    shard_performing_well(SpecialistName),
    shard_profile(SpecialistName, /specialist, _),
    trace_task_type(_, TaskType),
    shard_can_handle(SpecialistName, TaskType),
    user_intent(_, _, _, Task, _).

# Learn which tasks specialists handle well
shard_can_handle(ShardType, TaskType) :-
    reasoning_trace(TraceID, ShardType, /specialist, _, /true, _),
    trace_task_type(TraceID, TaskType),
    high_quality_trace(TraceID).

# =============================================================================
# SECTION 25: HOLOGRAPHIC RETRIEVAL (Cartographer)
# =============================================================================

# "X-Ray Vision": Find context relevant to the target file/symbol

# 1. Callers of the target symbol
relevant_context(File) :-
    user_intent(_, _, _, TargetSymbol, _),
    code_calls(Caller, TargetSymbol),
    code_defines(File, Caller, _, _, _).

# 2. Definitions in the target file
relevant_context(Symbol) :-
    user_intent(_, _, _, TargetFile, _),
    code_defines(TargetFile, Symbol, _, _, _).

# 3. Implementations of target interface
relevant_context(Struct) :-
    user_intent(_, _, _, Interface, _),
    code_implements(Struct, Interface).

# 4. Structs implementing the target interface (if target is interface)
relevant_context(StructFile) :-
    user_intent(_, _, _, Interface, _),
    code_implements(Struct, Interface),
    code_defines(StructFile, Struct, _, _, _).

# Boost activation for holographic matches
activation(Ctx, 85) :-
    relevant_context(Ctx).

# =============================================================================
# SECTION 26: SPECULATIVE DREAMER (Precognition Layer)
# =============================================================================

# Enumerate critical files that must never disappear
critical_file("go.mod").
critical_file("go.sum").

# Panic if a projected action would remove a critical file
panic_state(Action, "critical_file_missing") :-
    projected_fact(Action, /file_missing, File),
    critical_file(File).

# Panic on obviously dangerous exec commands
panic_state(Action, "dangerous_exec") :-
    projected_fact(Action, /exec_danger, _).

# Panic when deleting a file whose symbols are covered by tests
panic_state(Action, "deletes_tested_symbol") :-
    projected_fact(Action, /file_missing, _),
    projected_fact(Action, /impacts_test, _).

# Panic when Dreamer flags critical path hits
panic_state(Action, "critical_path_missing") :-
    projected_fact(Action, /critical_path_hit, _).

# Block actions the Dreamer marks as panic states
dream_block(Action, Reason) :-
    panic_state(Action, Reason).

# =============================================================================
# SECTION 40: INTELLIGENT TOOL ROUTING (§40)
# =============================================================================
# Routes Ouroboros-generated tools to shards based on capabilities, intent,
# domain matching, and usage history. Enables context-window-aware injection.

# -----------------------------------------------------------------------------
# 40.1 Base Shard-Capability Affinities (EDB)
# -----------------------------------------------------------------------------
# Score 0-100 (integer scale) indicating how relevant a capability is to each shard type
# NOTE: Must use integers because Mangle comparison operators don't support floats

# CoderShard affinities
shard_capability_affinity(/coder, /generation, 100).
shard_capability_affinity(/coder, /debugging, 90).
shard_capability_affinity(/coder, /transformation, 80).
shard_capability_affinity(/coder, /inspection, 50).
shard_capability_affinity(/coder, /validation, 40).
shard_capability_affinity(/coder, /execution, 60).

# TesterShard affinities
shard_capability_affinity(/tester, /validation, 100).
shard_capability_affinity(/tester, /execution, 90).
shard_capability_affinity(/tester, /inspection, 70).
shard_capability_affinity(/tester, /debugging, 60).
shard_capability_affinity(/tester, /analysis, 50).

# ReviewerShard affinities
shard_capability_affinity(/reviewer, /inspection, 100).
shard_capability_affinity(/reviewer, /analysis, 90).
shard_capability_affinity(/reviewer, /validation, 60).
shard_capability_affinity(/reviewer, /debugging, 40).

# ResearcherShard affinities
shard_capability_affinity(/researcher, /knowledge, 100).
shard_capability_affinity(/researcher, /analysis, 80).
shard_capability_affinity(/researcher, /inspection, 60).

# Generalist affinities (moderate across all)
shard_capability_affinity(/generalist, /generation, 50).
shard_capability_affinity(/generalist, /validation, 50).
shard_capability_affinity(/generalist, /inspection, 50).
shard_capability_affinity(/generalist, /analysis, 50).
shard_capability_affinity(/generalist, /execution, 50).
shard_capability_affinity(/generalist, /knowledge, 50).
shard_capability_affinity(/generalist, /debugging, 50).
shard_capability_affinity(/generalist, /transformation, 50).

# -----------------------------------------------------------------------------
# 40.2 Intent-Capability Mappings (EDB)
# -----------------------------------------------------------------------------
# Maps user intent verbs to required capabilities with importance weights (0-100 scale)

# Mutation intents
intent_requires_capability(/implement, /generation, 100).
intent_requires_capability(/implement, /validation, 50).
intent_requires_capability(/refactor, /transformation, 100).
intent_requires_capability(/refactor, /analysis, 70).
intent_requires_capability(/fix, /debugging, 100).
intent_requires_capability(/fix, /validation, 80).
intent_requires_capability(/generate, /generation, 100).
intent_requires_capability(/scaffold, /generation, 90).
intent_requires_capability(/init, /generation, 80).

# Query intents
intent_requires_capability(/test, /validation, 100).
intent_requires_capability(/test, /execution, 90).
intent_requires_capability(/review, /inspection, 100).
intent_requires_capability(/review, /analysis, 80).
intent_requires_capability(/explain, /analysis, 100).
intent_requires_capability(/explain, /knowledge, 70).
intent_requires_capability(/debug, /debugging, 100).
intent_requires_capability(/debug, /inspection, 80).

# Research intents
intent_requires_capability(/research, /knowledge, 100).
intent_requires_capability(/research, /analysis, 60).
intent_requires_capability(/explore, /inspection, 90).
intent_requires_capability(/explore, /analysis, 80).
intent_requires_capability(/explore, /knowledge, 50).

# Run intents
intent_requires_capability(/run, /execution, 100).
intent_requires_capability(/run, /validation, 40).

# -----------------------------------------------------------------------------
# 40.3 Tool Relevance Derivation Rules (IDB)
# -----------------------------------------------------------------------------

# 40.3.1 Base Relevance: Tool matches shard's capability affinity
tool_base_relevance(ShardType, ToolName, AffinityScore) :-
    tool_capability(ToolName, Cap),
    shard_capability_affinity(ShardType, Cap, AffinityScore),
    tool_registered(ToolName, _).

# 40.3.2 Intent Boost: Tool matches current intent's required capabilities
tool_intent_relevance(ToolName, Weight) :-
    current_intent(IntentID),
    user_intent(IntentID, _, Verb, _, _),
    intent_requires_capability(Verb, Cap, Weight),
    tool_capability(ToolName, Cap).

# No current intent = no boost (fallback rule)
# Uses helper predicate for safe negation
tool_intent_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_current_intent().

# 40.3.3 Domain Boost: Tool matches target file's language/domain
# Score: 30 (out of 100)
tool_domain_relevance(ToolName, 30) :-
    current_intent(IntentID),
    user_intent(IntentID, _, _, Target, _),
    file_topology(Target, _, Lang, _, _),
    tool_domain(ToolName, Lang).

tool_domain_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_domain(ToolName).

# 40.3.4 Success History Boost: Tool succeeded in similar contexts
# Note: Uses simplified scoring - full implementation would compute rate
# Score: 20 (out of 100)
tool_success_relevance(ToolName, 20) :-
    tool_usage_stats(ToolName, ExecCount, SuccessCount, _),
    ExecCount > 0,
    SuccessCount > 0.

tool_success_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_usage(ToolName).

# 40.3.5 Recency Boost: Recently used tools likely still relevant
# Note: Full implementation would check timestamp difference
# Score: 15 (out of 100)
tool_recency_relevance(ToolName, 15) :-
    tool_usage_stats(ToolName, _, _, LastUsed),
    current_time(Now),
    LastUsed > 0.

tool_recency_relevance(ToolName, 0) :-
    tool_registered(ToolName, _),
    !has_tool_usage(ToolName).

# 40.3.6 Combined Score: Weighted sum of all relevance factors
# Base relevance weighted at 40%, intent at 30%, domain/success/recency fill rest
# Note: Mangle doesn't support arithmetic in rule bodies, so we use approximation
#       The Go implementation will compute exact scores

# Simplified relevance threshold: tool is relevant if it has base affinity >= 30
relevant_tool(ShardType, ToolName) :-
    tool_base_relevance(ShardType, ToolName, BaseScore),
    BaseScore >= 30.

# Also relevant if intent matches strongly (>= 70)
relevant_tool(ShardType, ToolName) :-
    tool_intent_relevance(ToolName, IntentScore),
    IntentScore >= 70,
    tool_registered(ToolName, _),
    current_shard_type(ShardType).

# System shards see all tools (Type S gets full visibility)
relevant_tool(/system, ToolName) :-
    tool_registered(ToolName, _).

# -----------------------------------------------------------------------------
# 40.4 Helper Predicates
# -----------------------------------------------------------------------------

# has_current_intent() - helper for safe negation
has_current_intent() :- current_intent(_).

# has_tool_domain(ToolName) - helper for safe negation
has_tool_domain(ToolName) :- tool_domain(ToolName, _).

# has_tool_usage(ToolName) - helper for safe negation
has_tool_usage(ToolName) :- tool_usage_stats(ToolName, _, _, _).

# =============================================================================
# SECTION 41: DYNAMIC PROMPT COMPOSITION (Spreading Activation Extension)
# =============================================================================
# Rules for selecting context atoms to inject into shard system prompts.
# Implements spreading activation from user_intent to relevant facts.
# Per codeNERD architecture: facts flow through the kernel to shape LLM context.

# -----------------------------------------------------------------------------
# 41.1 Shard-Specific Context Relevance (3-arity extension)
# -----------------------------------------------------------------------------
# shard_context_atom(ShardID, Atom, Relevance) - context relevance per shard
# Relevance is integer 0-100 scale (Mangle doesn't support floats)

# Context relevance based on intent match - HIGH relevance (90)
# When shard type matches intent category, target is highly relevant
shard_context_atom(ShardID, Target, 90) :-
    active_shard(ShardID, ShardType),
    user_intent(_, ShardType, _, Target, _).

# Propagate specialist knowledge to context - HIGH relevance (80)
shard_context_atom(ShardID, Knowledge, 80) :-
    active_shard(ShardID, _),
    specialist_knowledge(ShardID, _, Knowledge).

# Include campaign constraints in context - MEDIUM relevance (70)
shard_context_atom(ShardID, Constraint, 70) :-
    active_shard(ShardID, ShardType),
    campaign_active(CampaignID),
    campaign_prompt_policy(CampaignID, ShardType, Constraint).

# Include learned exemplars - MEDIUM relevance (60)
shard_context_atom(ShardID, Exemplar, 60) :-
    active_shard(ShardID, ShardType),
    user_intent(_, Category, _, _, _),
    prompt_exemplar(ShardType, Category, Exemplar).

# Include relevant tool descriptions - MEDIUM relevance (65)
shard_context_atom(ShardID, ToolDesc, 65) :-
    active_shard(ShardID, ShardType),
    relevant_tool(ShardType, ToolName),
    tool_description(ToolName, ToolDesc).

# Include recent successful trace patterns - LOW relevance (50)
shard_context_atom(ShardID, TracePattern, 50) :-
    active_shard(ShardID, ShardType),
    high_quality_trace(TraceID),
    reasoning_trace(TraceID, ShardType, _, _, /true, _),
    trace_pattern(TraceID, TracePattern).

# -----------------------------------------------------------------------------
# 41.2 Injectable Context Selection (Threshold Filtering)
# -----------------------------------------------------------------------------

# Select injectable context based on relevance threshold (> 50)
injectable_context(ShardID, Atom) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance > 50.

# High-priority injectable context (relevance >= 80)
injectable_context_priority(ShardID, Atom, /high) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance >= 80.

# Medium-priority injectable context (60 <= relevance < 80)
injectable_context_priority(ShardID, Atom, /medium) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance >= 60,
    Relevance < 80.

# Low-priority injectable context (50 < relevance < 60)
injectable_context_priority(ShardID, Atom, /low) :-
    shard_context_atom(ShardID, Atom, Relevance),
    Relevance > 50,
    Relevance < 60.

# -----------------------------------------------------------------------------
# 41.3 Context Budget Awareness (for context window management)
# -----------------------------------------------------------------------------

# Helper: shard has injectable context
has_injectable_context(ShardID) :-
    injectable_context(ShardID, _).

# Helper: shard has high-priority context
has_high_priority_context(ShardID) :-
    injectable_context_priority(ShardID, _, /high).

# When context budget is limited, only inject high-priority items
context_budget_constrained(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget < 5000.

# Full context injection allowed when budget is sufficient
context_budget_sufficient(ShardID) :-
    active_shard(ShardID, _),
    context_budget(ShardID, Budget),
    Budget >= 5000.

# Final injectable set: all items when budget sufficient
final_injectable(ShardID, Atom) :-
    context_budget_sufficient(ShardID),
    injectable_context(ShardID, Atom).

# Final injectable set: only high priority when budget constrained
final_injectable(ShardID, Atom) :-
    context_budget_constrained(ShardID),
    injectable_context_priority(ShardID, Atom, /high).

# -----------------------------------------------------------------------------
# 41.4 Spreading Activation Integration
# -----------------------------------------------------------------------------

# Boost activation for atoms selected as injectable context
activation(Atom, 95) :-
    final_injectable(_, Atom).

# Boost activation for specialist knowledge atoms
activation(Knowledge, 85) :-
    specialist_knowledge(_, _, Knowledge).

# Boost activation for campaign prompt policy atoms
activation(Constraint, 75) :-
    campaign_active(_),
    campaign_prompt_policy(_, _, Constraint).

# Boost activation for learned exemplars
activation(Exemplar, 70) :-
    prompt_exemplar(_, _, Exemplar).

# -----------------------------------------------------------------------------
# 41.5 Context Staleness Detection
# -----------------------------------------------------------------------------

# Context atom is stale if it references a modified file
context_stale(ShardID, Atom) :-
    shard_context_atom(ShardID, Atom, _),
    modified(Atom).

# Context atom is stale if specialist knowledge was updated
context_stale(ShardID, Knowledge) :-
    shard_context_atom(ShardID, Knowledge, _),
    specialist_knowledge(ShardID, _, Knowledge),
    specialist_knowledge_updated(ShardID).

# Helper: shard has stale context
has_stale_context(ShardID) :-
    context_stale(ShardID, _).

# Trigger context refresh when stale atoms detected
next_action(/refresh_shard_context) :-
    active_shard(ShardID, _),
    has_stale_context(ShardID).

# -----------------------------------------------------------------------------
# 41.6 Learning Signals from Context Usage
# -----------------------------------------------------------------------------

# Track when injected context leads to successful task completion
context_injection_effective(ShardID, Atom) :-
    final_injectable(ShardID, Atom),
    shard_executed(ShardID, _, /success, _).

# Learn from effective context injections
learning_signal(/effective_context, Atom) :-
    context_injection_effective(_, Atom).

# Promote frequently effective context to long-term memory
promote_to_long_term(/context_pattern, Atom) :-
    context_injection_effective(S1, Atom),
    context_injection_effective(S2, Atom),
    context_injection_effective(S3, Atom),
    S1 != S2,
    S2 != S3,
    S1 != S3.

# =============================================================================
# 42. NORTHSTAR VISION REASONING
# =============================================================================
# Rules for reasoning over northstar facts defined via /northstar command.
# These rules derive strategic insights from vision, personas, capabilities,
# risks, and requirements.

# -----------------------------------------------------------------------------
# 42.1 Critical Path Derivation
# -----------------------------------------------------------------------------

# Derive critical capabilities (priority = /critical)
critical_capability(CapID) :-
    northstar_capability(CapID, _, _, /critical).

# Derive high-risk items (both likelihood AND impact are high)
high_risk(RiskID) :-
    northstar_risk(RiskID, _, /high, /high).

# Helper: risk has at least one mitigation
has_mitigation(RiskID) :-
    northstar_mitigation(RiskID, _).

# Derive unmitigated risks (high risk without any mitigation)
unmitigated_risk(RiskID) :-
    high_risk(RiskID),
    !has_mitigation(RiskID).

# -----------------------------------------------------------------------------
# 42.2 Alignment Analysis
# -----------------------------------------------------------------------------

# Capability addresses persona need when serves relationship exists
capability_addresses_need(CapID, PersonaID, Need) :-
    northstar_serves(CapID, PersonaID),
    northstar_need(PersonaID, Need).

# Helper: persona is served by at least one capability
is_served_persona(PersonaID) :-
    northstar_serves(_, PersonaID).

# Helper: capability serves at least one persona
capability_is_linked(CapID) :-
    northstar_serves(CapID, _).

# Unserved persona - has needs but no capability serves them
unserved_persona(PersonaID, Name) :-
    northstar_persona(PersonaID, Name),
    northstar_need(PersonaID, _),
    !is_served_persona(PersonaID).

# Orphan capability - not linked to any persona
orphan_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, _, _),
    !capability_is_linked(CapID).

# -----------------------------------------------------------------------------
# 42.3 Requirements Traceability
# -----------------------------------------------------------------------------

# Must-have requirements (priority = /must_have)
must_have_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, /must_have).

# Helper: requirement is supported by at least one capability
is_supported_req(ReqID) :-
    northstar_supports(ReqID, _).

# Orphan requirement - not linked to any capability
orphan_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, _),
    !is_supported_req(ReqID).

# Risk-addressing requirement
risk_addressing_requirement(ReqID, RiskID) :-
    northstar_addresses(ReqID, RiskID),
    high_risk(RiskID).

# Helper: risk is addressed by at least one requirement
risk_is_addressed(RiskID) :-
    northstar_addresses(_, RiskID).

# Unaddressed high risk - no requirement addresses it
unaddressed_high_risk(RiskID, Desc) :-
    high_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _),
    !risk_is_addressed(RiskID).

# -----------------------------------------------------------------------------
# 42.4 Timeline Planning
# -----------------------------------------------------------------------------

# Immediate work (timeline = /now)
immediate_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /now, _).

# Near-term work (timeline = /6mo)
near_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /6mo, _).

# Long-term work (timeline = /1yr or /3yr)
long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /1yr, _).

long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /3yr, _).

# Moonshot capabilities (timeline = /moonshot)
moonshot_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /moonshot, _).

# -----------------------------------------------------------------------------
# 42.5 Strategic Warnings
# -----------------------------------------------------------------------------

# Warning: critical capability with unmitigated high risk
strategic_warning(/critical_unmitigated_risk, CapID, RiskID) :-
    critical_capability(CapID),
    northstar_supports(ReqID, CapID),
    northstar_addresses(ReqID, RiskID),
    unmitigated_risk(RiskID).

# Warning: immediate work depends on unaddressed risk
strategic_warning(/immediate_risk_gap, CapID, RiskID) :-
    immediate_capability(CapID, _),
    unaddressed_high_risk(RiskID, _).

# -----------------------------------------------------------------------------
# 42.6 Context Injection for Northstar
# -----------------------------------------------------------------------------

# Inject mission when planning or deciding actions
injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /coder).

# Inject critical capabilities during planning
injectable_context(/critical_cap, Desc) :-
    northstar_defined(),
    critical_capability(CapID),
    northstar_capability(CapID, Desc, _, _),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

# Inject unmitigated risks as warnings
injectable_context(/unmitigated_risk_warning, Desc) :-
    northstar_defined(),
    unmitigated_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _).

# Inject constraints always
injectable_context(/constraint, Desc) :-
    northstar_defined(),
    northstar_constraint(_, Desc).

# =============================================================================
# SECTION 45: JIT PROMPT COMPILER POLICY
# =============================================================================
# Universal JIT Prompt Compiler for dynamic prompt assembly.
# Selects and orders prompt atoms based on contextual dimensions.
# Implements spreading activation from compile_context to atom selection.

# -----------------------------------------------------------------------------
# 45.1 Category Ordering (Static Facts)
# -----------------------------------------------------------------------------
# Determines section order in final prompt.
# Lower numbers appear first in the assembled prompt.

category_order(/identity, 1).
category_order(/safety, 2).
category_order(/hallucination, 3).
category_order(/methodology, 4).
category_order(/language, 5).
category_order(/framework, 6).
category_order(/domain, 7).
category_order(/campaign, 8).
category_order(/init, 8).
category_order(/northstar, 8).
category_order(/ouroboros, 8).
category_order(/context, 9).
category_order(/exemplar, 10).
category_order(/protocol, 11).

# -----------------------------------------------------------------------------
# 45.2 Category Budget Allocation
# -----------------------------------------------------------------------------
# Percentage of total token budget allocated to each category.
# These are targets; actual allocation may vary based on selection.

category_budget(/identity, 5).
category_budget(/protocol, 12).
category_budget(/safety, 5).
category_budget(/hallucination, 8).
category_budget(/methodology, 15).
category_budget(/language, 8).
category_budget(/framework, 8).
category_budget(/domain, 15).
category_budget(/context, 12).
category_budget(/exemplar, 7).
category_budget(/campaign, 5).
category_budget(/init, 5).
category_budget(/northstar, 5).
category_budget(/ouroboros, 5).

# -----------------------------------------------------------------------------
# 45.3 Contextual Matching Rules
# -----------------------------------------------------------------------------
# Compute match scores for atoms based on context dimensions.
# Higher scores indicate better match to current compilation context.
# Scores are additive when multiple dimensions match.

# Base score from atom priority (all atoms start with their priority)
atom_matches_context(AtomID, Priority) :-
    prompt_atom(AtomID, _, Priority, _, _).

# Boost for shard type match (+30)
# Atoms designed for this shard type get significant boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /shard_type, ShardType),
    compile_shard(_, ShardType),
    Boosted = fn:plus(Priority, 30).

# Boost for operational mode match (+20)
# Mode-specific atoms (e.g., /debugging, /tdd_repair) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /operational_mode, Mode),
    compile_context(/operational_mode, Mode),
    Boosted = fn:plus(Priority, 20).

# Boost for campaign phase match (+15)
# Phase-specific atoms (e.g., /planning, /validating) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /campaign_phase, Phase),
    compile_context(/campaign_phase, Phase),
    Boosted = fn:plus(Priority, 15).

# Boost for intent verb match (+25)
# Verb-specific atoms (e.g., /fix, /debug, /refactor) get strong boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /intent_verb, Verb),
    compile_context(/intent_verb, Verb),
    Boosted = fn:plus(Priority, 25).

# Boost for language match (+10)
# Language-specific atoms (e.g., /go, /python) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /language, Lang),
    compile_context(/language, Lang),
    Boosted = fn:plus(Priority, 10).

# Boost for framework match (+15)
# Framework-specific atoms (e.g., /bubbletea, /gin, /rod) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /framework, Framework),
    compile_context(/framework, Framework),
    Boosted = fn:plus(Priority, 15).

# Boost for world state match (+20)
# World-state atoms (e.g., failing_tests, diagnostics) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /world_state, State),
    compile_context(/world_state, State),
    Boosted = fn:plus(Priority, 20).

# Boost for init phase match (+15)
# Init-phase atoms (e.g., /analysis, /kb_agent) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /init_phase, Phase),
    compile_context(/init_phase, Phase),
    Boosted = fn:plus(Priority, 15).

# Boost for ouroboros stage match (+15)
# Ouroboros-stage atoms (e.g., /specification, /refinement) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /ouroboros_stage, Stage),
    compile_context(/ouroboros_stage, Stage),
    Boosted = fn:plus(Priority, 15).

# Boost for northstar phase match (+15)
# Northstar-phase atoms (e.g., /doc_ingestion, /requirements) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /northstar_phase, Phase),
    compile_context(/northstar_phase, Phase),
    Boosted = fn:plus(Priority, 15).

# Boost for build layer match (+10)
# Build-layer atoms (e.g., /scaffold, /service) get boost
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /build_layer, Layer),
    compile_context(/build_layer, Layer),
    Boosted = fn:plus(Priority, 10).

# Mandatory atoms always get max score (100)
# These must be included regardless of context
atom_matches_context(AtomID, 100) :-
    prompt_atom(AtomID, _, _, _, /true).

# Vector similarity boost (scaled 0-30)
# Semantic similarity from vector search adds to score
atom_matches_context(AtomID, VecBoosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    compile_query(Query),
    vector_recall_result(Query, AtomID, Similarity),
    VecBoost = fn:mult(Similarity, 30),
    VecBoosted = fn:plus(Priority, VecBoost).

# -----------------------------------------------------------------------------
# 45.4 Dependency Resolution (Stratified)
# -----------------------------------------------------------------------------
# Ensure atoms with hard dependencies only select if dependencies are satisfiable.
# Uses a score-based approach to avoid cycles with atom_selected.
#
# Key insight: A dependency is "satisfiable" if the dependent atom would meet
# the minimum score threshold (40), not if it's actually selected. This allows
# dependency checking to happen before selection.

# Helper: atom would meet score threshold (potential candidate)
atom_meets_threshold(AtomID) :-
    atom_matches_context(AtomID, Score),
    Score > 40.

# Helper: atom is mandatory (always meets threshold)
atom_meets_threshold(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true).

# Helper: atom has at least one unsatisfiable hard dependency
# A dependency is unsatisfiable if the target atom exists but wouldn't meet threshold
has_unsatisfied_hard_dep(AtomID) :-
    atom_dependency(AtomID, DepID, /hard),
    prompt_atom(DepID, _, _, _, _),
    !atom_meets_threshold(DepID).

# Atom dependencies are satisfied if no unsatisfiable hard deps exist
atom_dependency_satisfied(AtomID) :-
    prompt_atom(AtomID, _, _, _, _),
    !has_unsatisfied_hard_dep(AtomID).

# -----------------------------------------------------------------------------
# 45.5 Selection Algorithm (Stratified)
# -----------------------------------------------------------------------------
# Uses a two-phase approach to avoid stratification issues:
# Phase 1 (Stratum 0): Identify candidate atoms based on scores
# Phase 2 (Stratum 1): Detect conflicts among candidates
# Phase 3 (Stratum 2): Final selection excludes conflicted atoms

# Phase 1: Candidate atoms pass score threshold and have satisfied dependencies
# This is computed first without any negation on selection predicates
atom_candidate(AtomID) :-
    atom_matches_context(AtomID, Score),
    Score > 40,
    atom_dependency_satisfied(AtomID).

# Mandatory atoms are always candidates
atom_candidate(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true).

# Phase 2: Detect conflicts among candidates
# An atom loses to a conflicting atom with higher score
atom_loses_conflict(AtomID) :-
    atom_candidate(AtomID),
    atom_conflict(AtomID, OtherID),
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

atom_loses_conflict(AtomID) :-
    atom_candidate(AtomID),
    atom_conflict(OtherID, AtomID),
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# An atom loses in exclusion group to higher-scoring atom
atom_loses_exclusion(AtomID) :-
    atom_candidate(AtomID),
    atom_exclusion_group(AtomID, GroupID),
    atom_exclusion_group(OtherID, GroupID),
    AtomID != OtherID,
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# Helper: atom is excluded for any reason
is_excluded(AtomID) :-
    atom_loses_conflict(AtomID).

is_excluded(AtomID) :-
    atom_loses_exclusion(AtomID).

# Exclude if dependency not satisfied (computed early, no cycle)
is_excluded(AtomID) :-
    prompt_atom(AtomID, _, _, _, _),
    !atom_dependency_satisfied(AtomID).

# Phase 3: Final selection - candidates that are not excluded
atom_selected(AtomID) :-
    atom_candidate(AtomID),
    !is_excluded(AtomID).

# -----------------------------------------------------------------------------
# 45.6 Final Ordering
# -----------------------------------------------------------------------------
# Order selected atoms by category first, then by match score within category.
# Order value = (CategoryOrder * 1000) + Score

final_atom(AtomID, Order) :-
    atom_selected(AtomID),
    prompt_atom(AtomID, Category, _, _, _),
    category_order(Category, CatOrder),
    atom_matches_context(AtomID, Score),
    Order = fn:plus(fn:mult(CatOrder, 1000), Score).

# -----------------------------------------------------------------------------
# 45.7 Compilation Validation
# -----------------------------------------------------------------------------
# Validate compilation meets minimum requirements.

# Helper: at least one identity atom is selected
has_identity_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /identity, _, _, _).

# Helper: at least one protocol atom is selected
has_protocol_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /protocol, _, _, _).

# Helper: at least one compilation error exists
has_compilation_error() :-
    compilation_error(_, _).

# Compilation is valid if: has identity, has protocol, no errors
compilation_valid() :-
    has_identity_atom(),
    has_protocol_atom(),
    !has_compilation_error().

# Error: missing mandatory atom (mandatory atom not selected)
compilation_error(/missing_mandatory, AtomID) :-
    prompt_atom(AtomID, _, _, _, /true),
    !atom_selected(AtomID).

# Error: circular dependency (simplified - full detection in Go)
# Direct cycle detection: A depends on B and B depends on A
compilation_error(/circular_dependency, AtomID) :-
    atom_dependency(AtomID, DepID, /hard),
    atom_dependency(DepID, AtomID, /hard).

# -----------------------------------------------------------------------------
# 45.8 Integration with Spreading Activation
# -----------------------------------------------------------------------------
# Selected atoms boost activation for related facts.

# High activation for selected atoms
activation(AtomID, 95) :-
    atom_selected(AtomID).

# Medium activation for atoms matching context but not selected
activation(AtomID, 60) :-
    atom_matches_context(AtomID, Score),
    Score > 30,
    !atom_selected(AtomID).

# -----------------------------------------------------------------------------
# 45.9 Learning Signals from Prompt Compilation
# -----------------------------------------------------------------------------
# Track effective prompt patterns for autopoiesis learning.

# Signal: atom was selected and shard execution succeeded
effective_prompt_atom(AtomID) :-
    atom_selected(AtomID),
    compile_shard(ShardID, _),
    shard_executed(ShardID, _, /success, _).

# Learning signal: promote effective atoms to higher priority
learning_signal(/effective_prompt_atom, AtomID) :-
    effective_prompt_atom(AtomID).

# =============================================================================
# SECTION 46: JIT PROMPT COMPILER SELECTION RULES
# =============================================================================
# Implements the Skeleton/Firewall/Flesh selection model for JIT prompt compilation.
# This provides a simplified, layered approach complementing Section 45's score-based model.
#
# Architecture:
# - SKELETON: Mandatory atoms that MUST be included (identity, protocol, safety, methodology)
# - FIREWALL: Prohibited atoms blocked by operational mode constraints
# - FLESH: Vector search candidates filtered by context and safety rules
#
# Stratum Order:
# - Stratum 0: skeleton_category (facts), prohibited_atom (base rules)
# - Stratum 1: mandatory_atom, candidate_atom (depends on prohibited_atom)
# - Stratum 2: selected_atom (depends on mandatory_atom, candidate_atom)

# -----------------------------------------------------------------------------
# 46.1 SKELETON (Mandatory - Fail if missing)
# -----------------------------------------------------------------------------
# These categories MUST be included in every prompt for their shard type.
# Skeleton atoms form the structural foundation of every compiled prompt.

# Define skeleton categories - these are non-negotiable prompt sections
skeleton_category(/identity).
skeleton_category(/protocol).
skeleton_category(/safety).
skeleton_category(/methodology).

# An atom is mandatory if:
# 1. It belongs to a skeleton category
# 2. It matches the current shard type (if tagged)
# 3. It is not explicitly prohibited
mandatory_atom(AtomID) :-
    prompt_atom(AtomID, Category, _, _, _),
    skeleton_category(Category),
    compile_shard(_, ShardType),
    atom_tag(AtomID, /shard_type, ShardType),
    !prohibited_atom(AtomID).

# Atoms explicitly marked as mandatory are always mandatory
mandatory_atom(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true),
    !prohibited_atom(AtomID).

# Atoms with is_mandatory flag (from jit_compiler.mg schema)
mandatory_atom(AtomID) :-
    is_mandatory(AtomID),
    !prohibited_atom(AtomID).

# -----------------------------------------------------------------------------
# 46.2 FIREWALL (Prohibited in certain contexts)
# -----------------------------------------------------------------------------
# These atoms are BLOCKED in certain operational modes.
# The firewall ensures safety and context-appropriateness.
#
# Stratification Note:
# - base_prohibited is computed first (Stratum 0) - no dependencies on mandatory
# - prohibited_atom extends base_prohibited (Stratum 0)
# - mandatory_atom negates prohibited_atom (Stratum 1)
# - Conflict resolution happens at selection time, not prohibition time

# Base prohibitions: context-based blocking (no dependency on mandatory_atom)
base_prohibited(AtomID) :-
    compile_context(/operational_mode, /production),
    atom_tag(AtomID, /tag, /debug_only).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /dream),
    atom_tag(AtomID, /category, /ouroboros).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /init),
    atom_tag(AtomID, /category, /campaign).

base_prohibited(AtomID) :-
    compile_context(/operational_mode, /active),
    atom_tag(AtomID, /tag, /dream_only).

# Dependency-based prohibition: if a required atom is base_prohibited, prohibit the dependent
base_prohibited(AtomID) :-
    atom_requires(AtomID, DepID),
    base_prohibited(DepID).

# prohibited_atom = base_prohibited (for now, conflict handling moved to selection)
prohibited_atom(AtomID) :- base_prohibited(AtomID).

# -----------------------------------------------------------------------------
# 46.3 FLESH (Vector candidates filtered by Mangle)
# -----------------------------------------------------------------------------
# These atoms are CANDIDATES for inclusion based on vector similarity.
# Vector search provides semantic relevance; Mangle provides safety filtering.

# Candidate atoms must:
# 1. Have a vector hit with sufficient similarity (> 0.3)
# 2. Not be prohibited by firewall rules
candidate_atom(AtomID) :-
    vector_hit(AtomID, Score),
    Score > 0.3,
    !prohibited_atom(AtomID).

# Also consider atoms matching context dimensions even without vector hit
candidate_atom(AtomID) :-
    prompt_atom(AtomID, _, Priority, _, _),
    Priority > 50,
    atom_tag(AtomID, /shard_type, ShardType),
    compile_shard(_, ShardType),
    !prohibited_atom(AtomID),
    !mandatory_atom(AtomID).

# -----------------------------------------------------------------------------
# 46.4 Final Selection (with Conflict Resolution)
# -----------------------------------------------------------------------------
# An atom is selected if it's mandatory OR a valid candidate.
# Mandatory atoms take precedence (included regardless of vector score).
# Conflicts are resolved by excluding the lower-priority atom.
#
# Stratification:
# - Stratum 0: base_prohibited, prohibited_atom
# - Stratum 1: mandatory_atom (negates prohibited_atom)
# - Stratum 2: candidate_atom (negates prohibited_atom, mandatory_atom)
# - Stratum 3: conflict_loser (depends on mandatory_atom, candidate_atom)
# - Stratum 4: selected_atom (negates conflict_loser)

# Helper: An atom loses a conflict to a mandatory atom
conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    atom_conflicts(AtomID, MandatoryID),
    mandatory_atom(MandatoryID).

conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    atom_conflicts(MandatoryID, AtomID),
    mandatory_atom(MandatoryID).

# Helper: Two candidates conflict, lower priority loses
conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    candidate_atom(OtherID),
    atom_conflicts(AtomID, OtherID),
    prompt_atom(AtomID, _, PriorityA, _, _),
    prompt_atom(OtherID, _, PriorityB, _, _),
    PriorityA < PriorityB.

conflict_loser(AtomID) :-
    candidate_atom(AtomID),
    candidate_atom(OtherID),
    atom_conflicts(OtherID, AtomID),
    prompt_atom(AtomID, _, PriorityA, _, _),
    prompt_atom(OtherID, _, PriorityB, _, _),
    PriorityA < PriorityB.

# Final selection: mandatory atoms always selected
selected_atom(AtomID) :- mandatory_atom(AtomID).

# Candidates selected if not a conflict loser
selected_atom(AtomID) :-
    candidate_atom(AtomID),
    !mandatory_atom(AtomID),
    !conflict_loser(AtomID).

# -----------------------------------------------------------------------------
# 46.5 Context Matching Boost
# -----------------------------------------------------------------------------
# Atoms matching current context get priority boost for ordering.
# Boosts are additive when multiple dimensions match.

# Boost for operational mode match (+30)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /operational_mode, Mode),
    compile_context(/operational_mode, Mode),
    Boost = fn:plus(Priority, 30).

# Boost for shard type match (+25)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /shard_type, Type),
    compile_context(/shard_type, Type),
    Boost = fn:plus(Priority, 25).

# Boost for language match (+20)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /language, Lang),
    compile_context(/language, Lang),
    Boost = fn:plus(Priority, 20).

# Boost for framework match (+15)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /framework, Framework),
    compile_context(/framework, Framework),
    Boost = fn:plus(Priority, 15).

# Boost for intent verb match (+25)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /intent_verb, Verb),
    compile_context(/intent_verb, Verb),
    Boost = fn:plus(Priority, 25).

# Boost for campaign phase match (+15)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /campaign_phase, Phase),
    compile_context(/campaign_phase, Phase),
    Boost = fn:plus(Priority, 15).

# Boost for world state match (+20)
atom_context_boost(AtomID, Boost) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_tag(AtomID, /world_state, State),
    compile_context(/world_state, State),
    Boost = fn:plus(Priority, 20).

# -----------------------------------------------------------------------------
# 46.6 Section 46 Validation
# -----------------------------------------------------------------------------
# Ensure skeleton categories have at least one selected atom.

has_skeleton_category(Category) :-
    selected_atom(AtomID),
    prompt_atom(AtomID, Category, _, _, _),
    skeleton_category(Category).

missing_skeleton_category(Category) :-
    skeleton_category(Category),
    !has_skeleton_category(Category).

# Report missing skeleton as compilation error
compilation_error(/missing_skeleton, Category) :-
    missing_skeleton_category(Category).

# =============================================================================
# SECTION 47: DATA FLOW SAFETY RULES (ReviewerShard Beyond-SOTA)
# =============================================================================
# These rules implement guard-based reasoning for nil-pointer safety and
# error handling verification. They use Mangle's stratified negation to
# identify unsafe dereferences and unchecked errors.
#
# Stratification:
# - Stratum 0: guards_block, guards_return, assigns, uses (EDB)
# - Stratum 1: is_guarded, is_error_checked (derived from guards)
# - Stratum 2: unsafe_deref, unchecked_error (negates is_guarded/is_error_checked)

# -----------------------------------------------------------------------------
# 47.1 Guard Derivation - Two Patterns for Go Idioms
# -----------------------------------------------------------------------------
# These rules derive guarded use sites by joining guards with actual uses.
# This ensures the Line variable is always bound before comparison.

# Pattern 1: Block-scoped guards (if x != nil { ... })
# A use site is guarded if inside a nil_check block's scope
is_guarded(Var, File, Line) :-
    uses(File, _, Var, Line),
    guards_block(Var, /nil_check, File, Start, End),
    Line >= Start,
    Line <= End.

# Pattern 2: Return-based guards (if x == nil { return })
# A use site is guarded after a guard clause that forces a return (Go idiom)
is_guarded(Var, File, Line) :-
    uses(File, _, Var, Line),
    guards_return(Var, /nil_check, File, GuardLine),
    Line > GuardLine,
    same_scope(Var, File, Line, GuardLine).

# Additional guard types: ok checks from type assertions and map lookups
is_guarded(Var, File, Line) :-
    uses(File, _, Var, Line),
    guards_block(Var, /ok_check, File, Start, End),
    Line >= Start,
    Line <= End.

is_guarded(Var, File, Line) :-
    uses(File, _, Var, Line),
    guards_return(Var, /ok_check, File, GuardLine),
    Line > GuardLine,
    same_scope(Var, File, Line, GuardLine).

# -----------------------------------------------------------------------------
# 47.2 Unsafe Nil Dereference Detection
# -----------------------------------------------------------------------------
# UNSAFE: nullable assigned but used without guard check
# This is a HYPOTHESIS - filtered by suppression before LLM sees it

# Helper: has any guard for variable at location
has_guard_at(Var, File, Line) :-
    is_guarded(Var, File, Line).

unsafe_deref(File, Var, Line) :-
    assigns(Var, /nullable, File, _),
    uses(File, _, Var, Line),
    !has_guard_at(Var, File, Line).

# Also detect pointer types that may be nil
unsafe_deref(File, Var, Line) :-
    assigns(Var, /pointer, File, _),
    uses(File, _, Var, Line),
    !has_guard_at(Var, File, Line).

# Interface types can also be nil
unsafe_deref(File, Var, Line) :-
    assigns(Var, /interface, File, _),
    uses(File, _, Var, Line),
    !has_guard_at(Var, File, Line).

# -----------------------------------------------------------------------------
# 47.3 Error Handling Verification
# -----------------------------------------------------------------------------
# These rules derive error-checked use sites by joining error checks with uses.

# Error use site is checked if inside error handling block
is_error_checked(ErrVar, File, UseLine) :-
    uses(File, _, ErrVar, UseLine),
    error_checked_block(ErrVar, File, Start, End),
    UseLine >= Start,
    UseLine <= End.

# Error use site is checked if after an error return guard
is_error_checked(ErrVar, File, UseLine) :-
    uses(File, _, ErrVar, UseLine),
    error_checked_return(ErrVar, File, GuardLine),
    UseLine > GuardLine,
    same_scope(ErrVar, File, UseLine, GuardLine).

# Helper: has error check at location
has_error_check_at(ErrVar, File, UseLine) :-
    is_error_checked(ErrVar, File, UseLine).

# UNCHECKED ERROR DETECTION
# Error variable assigned but not checked before use
unchecked_error(File, Func, AssignLine) :-
    assigns(ErrVar, /error, File, AssignLine),
    uses(File, Func, ErrVar, UseLine),
    !has_error_check_at(ErrVar, File, UseLine).

# =============================================================================
# SECTION 48: IMPACT ANALYSIS RULES (ReviewerShard Beyond-SOTA)
# =============================================================================
# These rules compute the impact graph for modified functions and interfaces.
# Used to fetch relevant context files for review and prioritize attention.
#
# Stratification:
# - Stratum 0: modified_function, modified_interface, code_calls, code_implements (EDB)
# - Stratum 1: impact_caller, impact_implementer (direct impacts)
# - Stratum 2: impact_graph (transitive closure with depth limit)
# - Stratum 3: relevant_context_file, context_priority_file (derived from graph)

# -----------------------------------------------------------------------------
# 48.1 Direct Impact Detection
# -----------------------------------------------------------------------------

# Direct callers of modified functions (Distance 1)
impact_caller(TargetFunc, CallerFunc) :-
    modified_function(TargetFunc, _),
    code_calls(CallerFunc, TargetFunc).

# Interface implementations affected by interface changes
impact_implementer(ImplFile, Struct) :-
    modified_interface(Interface, _),
    code_implements(Struct, Interface),
    code_defines(ImplFile, Struct, /struct, _, _).

# -----------------------------------------------------------------------------
# 48.2 Bounded Transitive Impact (Max Depth 3)
# -----------------------------------------------------------------------------
# We limit depth to 3 to:
# 1. Prevent infinite recursion on cyclic call graphs
# 2. Focus on most relevant context (direct and near-callers)
# 3. Keep context window manageable

# Base case: direct callers are at depth 1
impact_graph(Target, Caller, 1) :-
    impact_caller(Target, Caller).

# Recursive case: grandcallers at depth 2
impact_graph(Target, GrandCaller, 2) :-
    impact_graph(Target, Caller, 1),
    code_calls(GrandCaller, Caller).

# Recursive case: great-grandcallers at depth 3
impact_graph(Target, GreatGrandCaller, 3) :-
    impact_graph(Target, Caller, 2),
    code_calls(GreatGrandCaller, Caller).

# -----------------------------------------------------------------------------
# 48.3 Context File Selection
# -----------------------------------------------------------------------------

# Files to fetch for review context
relevant_context_file(File) :-
    impact_graph(_, Func, _),
    code_defines(File, Func, /function, _, _).

# Also include files containing interface implementations
relevant_context_file(File) :-
    impact_implementer(File, _).

# Prioritized context: closer callers get higher priority
# Priority = 4 - Depth (so depth 1 = priority 3, depth 2 = priority 2, etc.)
context_priority_file(File, Func, 3) :-
    impact_graph(_, Func, 1),
    code_defines(File, Func, /function, _, _).

context_priority_file(File, Func, 2) :-
    impact_graph(_, Func, 2),
    code_defines(File, Func, /function, _, _).

context_priority_file(File, Func, 1) :-
    impact_graph(_, Func, 3),
    code_defines(File, Func, /function, _, _).

# =============================================================================
# SECTION 49: SUPPRESSION RULES / AUTOPOIESIS (ReviewerShard Beyond-SOTA)
# =============================================================================
# These rules filter hypotheses through learned suppressions before presenting
# to the LLM. This implements the "autopoiesis" learning loop where false
# positives are remembered and filtered out.
#
# Stratification:
# - Stratum 0: suppressed_rule (EDB - learned facts)
# - Stratum 1: is_suppressed (derived)
# - Stratum 2: active_hypothesis (negates is_suppressed)

# -----------------------------------------------------------------------------
# 49.1 Suppression Check
# -----------------------------------------------------------------------------

# A finding is suppressed if there's a suppression rule for it
is_suppressed(Type, File, Line) :-
    suppressed_rule(Type, File, Line, _).

# Also suppress based on confidence score (learned from user feedback)
is_suppressed(Type, File, Line) :-
    suppression_confidence(Type, File, Line, Score),
    Score >= 80.

# -----------------------------------------------------------------------------
# 49.2 Active Hypothesis Filtering
# -----------------------------------------------------------------------------
# Only hypotheses that pass suppression filter are presented to LLM

# Helper predicates for safe negation
has_suppression_unsafe_deref(File, Line) :-
    is_suppressed(/unsafe_deref, File, Line).

has_suppression_unchecked_error(File, Line) :-
    is_suppressed(/unchecked_error, File, Line).

# Active unsafe dereference hypotheses
active_hypothesis(/unsafe_deref, File, Line, Var) :-
    unsafe_deref(File, Var, Line),
    !has_suppression_unsafe_deref(File, Line).

# Active unchecked error hypotheses
active_hypothesis(/unchecked_error, File, Line, Var) :-
    unchecked_error(File, Var, Line),
    !has_suppression_unchecked_error(File, Line).

# =============================================================================
# SECTION 50: HYPOTHESIS PRIORITIZATION (ReviewerShard Beyond-SOTA)
# =============================================================================
# These rules assign priority scores to hypotheses based on:
# 1. Finding type severity (e.g., SQL injection > nil deref > unchecked error)
# 2. File risk factors (no test coverage, bug history)
#
# This allows the LLM to focus on highest-priority issues first.

# -----------------------------------------------------------------------------
# 50.1 Base Type Priorities
# -----------------------------------------------------------------------------
# Higher numbers = higher priority (examined first)

type_priority(/sql_injection, 95).
type_priority(/command_injection, 95).
type_priority(/path_traversal, 90).
type_priority(/unsafe_deref, 85).
type_priority(/unchecked_error, 75).
type_priority(/race_condition, 70).
type_priority(/resource_leak, 65).

# -----------------------------------------------------------------------------
# 50.2 File-Based Priority Boosts
# -----------------------------------------------------------------------------

# Helper: file has test coverage
has_test_coverage(File) :-
    test_coverage(File).

# Helper: file has bug history
has_bug_history(File) :-
    bug_history(File, Count),
    Count > 0.

# Boost for files without test coverage (+20)
priority_boost(File, 20) :-
    active_hypothesis(_, File, _, _),
    !has_test_coverage(File).

# Boost for files with historical bugs (+15)
priority_boost(File, 15) :-
    active_hypothesis(_, File, _, _),
    has_bug_history(File).

# -----------------------------------------------------------------------------
# 50.3 Final Priority Calculation
# -----------------------------------------------------------------------------

# Helper: file has any boost
has_priority_boost(File) :-
    priority_boost(File, _).

# With boost: BasePriority + Boost
# Note: Mangle doesn't support inline arithmetic, so we enumerate common cases
prioritized_hypothesis(Type, File, Line, Var, Priority) :-
    active_hypothesis(Type, File, Line, Var),
    type_priority(Type, BasePriority),
    priority_boost(File, Boost),
    Priority = fn:plus(BasePriority, Boost).

# Without boost: just use base priority
prioritized_hypothesis(Type, File, Line, Var, Priority) :-
    active_hypothesis(Type, File, Line, Var),
    type_priority(Type, Priority),
    !has_priority_boost(File).


# internal/mangle/doc_taxonomy.mg
# =========================================================
# DOCUMENT LAYER TAXONOMY
# =========================================================

# 1. Architectural Layers & Priorities (Lower runs first)
# Config, Env, Setup
layer_priority(/scaffold, 10).
# Types, Interfaces, Entities
layer_priority(/domain_core, 20).
# Schemas, Repositories, Migrations
layer_priority(/data_layer, 30).
# Business Logic, Use Cases
layer_priority(/service, 40).
# HTTP, gRPC, CLI, API
layer_priority(/transport, 50).
# Wiring, Main, E2E
layer_priority(/integration, 60).

# 2. Logic for Layer Distance (Used for conflict detection)
layer_distance(L1, L2, Dist) :-
    layer_priority(L1, P1),
    layer_priority(L2, P2),
    P1 >= P2,
    Dist = fn:minus(P1, P2).

layer_distance(L1, L2, Dist) :-
    layer_priority(L1, P1),
    layer_priority(L2, P2),
    P2 > P1,
    Dist = fn:minus(P2, P1).

# 3. Validation: Detect "God Documents"
# If a doc maps to layers that are too far apart (e.g., Scaffold AND Integration),
# it suggests the doc is too broad and might confuse the planner.
doc_conflict(Doc, L1, L2) :-
    doc_layer(Doc, L1, _),
    doc_layer(Doc, L2, _),
    L1 != L2,
    layer_distance(L1, L2, Dist),
    Dist > 30.


# internal/mangle/topology_planner.mg
# =========================================================
# TOPOLOGY PLANNER
# =========================================================

# 1. Identify "Active" Layers
# A layer is active if we have high-confidence docs for it.
# Note: Confidence is integer 0-100, not float
active_layer(Layer) :-
    doc_layer(_, Layer, Confidence),
    Confidence > 65.

# 2. Generate Phase Skeletons
# Every active layer becomes a proposed phase in the campaign.
proposed_phase(Layer) :-
    active_layer(Layer).

# 3. Generate Hard Dependencies
# If Layer A has lower priority number than Layer B, A must finish before B starts.
phase_dependency_generated(PhaseA, PhaseB) :-
    active_layer(LayerA),
    active_layer(LayerB),
    layer_priority(LayerA, ScoreA),
    layer_priority(LayerB, ScoreB),
    ScoreA < ScoreB,
    PhaseA = LayerA,
    PhaseB = LayerB.

# 4. Context Scoping (The "Pollution" Fix)
# Defines exactly which files are allowed in the context window for a phase.
phase_context_scope(Phase, DocPath) :-
    active_layer(Layer),
    doc_layer(DocPath, Layer, _),
    Phase = Layer.

# Also allow "scaffold" docs (config/env) to be visible to ALL phases
phase_context_scope(Phase, DocPath) :-
    active_layer(Phase),
    doc_layer(DocPath, /scaffold, _).


# internal/mangle/build_topology.mg
# =========================================================
# BUILD TOPOLOGY ENFORCEMENT
# Enforces architectural ordering between phases using explicit categories.
# =========================================================

# ----------------------------------------------------------------------------- 
# 1. Canonical Build Layers
# -----------------------------------------------------------------------------

build_phase_type(/scaffold, 10).     # Config, env, bootstrapping
build_phase_type(/domain_core, 20).  # Interfaces, types, constants
build_phase_type(/data_layer, 30).   # Schemas, repositories, migrations
build_phase_type(/service, 40).      # Business logic, state machines
build_phase_type(/transport, 50).    # HTTP, gRPC, CLI, UI endpoints
build_phase_type(/integration, 60).  # Wiring, main, E2E, deploy

# Natural language aliases to improve LLM classification resilience
phase_synonym(/scaffold, "setup").
phase_synonym(/scaffold, "config").
phase_synonym(/scaffold, "bootstrap").
phase_synonym(/domain_core, "types").
phase_synonym(/domain_core, "interfaces").
phase_synonym(/domain_core, "entities").
phase_synonym(/data_layer, "database").
phase_synonym(/data_layer, "storage").
phase_synonym(/service, "logic").
phase_synonym(/service, "processor").
phase_synonym(/transport, "api").
phase_synonym(/transport, "frontend").
phase_synonym(/integration, "wiring").
phase_synonym(/integration, "main").

# ----------------------------------------------------------------------------- 
# 2. Phase Precedence
# -----------------------------------------------------------------------------

# Derive precedence score from explicit category
# Derive precedence score from explicit category
phase_precedence(PhaseID, Score) :-
    phase_category(PhaseID, Category),
    build_phase_type(Category, Score).

# If category provided via synonym, map it
phase_precedence(PhaseID, Score) :-
    phase_category(PhaseID, Alias),
    phase_synonym(Category, Alias),
    build_phase_type(Category, Score).

# ----------------------------------------------------------------------------- 
# 3. Violations & Warnings
# -----------------------------------------------------------------------------

# Architectural inversion: downstream depends on upstream with higher precedence score
architectural_violation(Downstream, Upstream, "inverted_dependency") :-
    phase_dependency(Downstream, Upstream, _),
    phase_precedence(Downstream, ScoreDown),
    phase_precedence(Upstream, ScoreUp),
    ScoreUp > ScoreDown.

# Gap warning: phases skip more than one layer
suspicious_gap(Downstream, Upstream) :-
    phase_dependency(Downstream, Upstream, _),
    phase_precedence(Downstream, ScoreDown),
    phase_precedence(Upstream, ScoreUp),
    Gap = fn:minus(ScoreDown, ScoreUp),
    Gap > 20.

# Helper to check if a phase has any precedence derived
has_phase_category(PhaseID) :-
    phase_precedence(PhaseID, _).

# Validation surface for the decomposer/validator
validation_error(PhaseID, /topology, "inverted_dependency") :-
    architectural_violation(PhaseID, _, _).

validation_error(PhaseID, /topology, "inverted_dependency") :-
    architectural_violation(_, PhaseID, _).

validation_error(PhaseID, /topology, "missing_category") :-
    campaign_phase(PhaseID, _, _, _, _, _),
    !has_phase_category(PhaseID).


# Campaign Rules - Advanced Campaign Orchestration Logic
# Version: 2.0.0
# =============================================================================
# CAMPAIGN INTELLIGENCE LAYER
# This file contains advanced campaign execution rules that complement the
# base policy (policy.mg Section 19) and topology (build_topology.mg).
#
# Architecture:
#   policy.mg        → Base campaign state machine, task selection
#   build_topology.mg → Architectural layer enforcement
#   campaign_rules.mg → THIS FILE: Advanced scheduling, quality, learning
# =============================================================================

# =============================================================================
# SECTION 1: CAMPAIGN PLANNING INTELLIGENCE
# =============================================================================
# Rules for intelligent campaign creation and phase decomposition.

# -----------------------------------------------------------------------------
# 1.1 Goal Complexity Analysis
# -----------------------------------------------------------------------------

# Goal requires campaign (not single-shot) if it mentions multiple components
goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, Topic1),
    goal_topic(CampaignID, Topic2),
    Topic1 != Topic2.

# Goal requires campaign if it involves known complex patterns
goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, /migration).

goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, /refactor).

goal_requires_campaign(Goal) :-
    campaign_goal(CampaignID, Goal),
    goal_topic(CampaignID, /greenfield).

# Simple goals that DON'T need campaigns
simple_goal(Goal) :-
    campaign_goal(CampaignID, Goal),
    !goal_requires_campaign(Goal).

# Recommend downgrade to single-shot for simple goals
recommend_downgrade(CampaignID) :-
    campaign(CampaignID, _, _, _, /planning),
    campaign_goal(CampaignID, Goal),
    simple_goal(Goal).

# -----------------------------------------------------------------------------
# 1.2 Phase Count Heuristics
# -----------------------------------------------------------------------------

# Campaign seems too ambitious (> 6 phases)
campaign_too_ambitious(CampaignID) :-
    campaign_metadata(CampaignID, _, EstPhases, _),
    EstPhases > 6.

# Campaign seems trivial (< 2 phases for complex goal)
campaign_too_trivial(CampaignID) :-
    campaign_metadata(CampaignID, _, EstPhases, _),
    EstPhases < 2,
    goal_requires_campaign(_).

# Warning for LLM to reconsider decomposition
decomposition_warning(CampaignID, "too_many_phases") :-
    campaign_too_ambitious(CampaignID).

decomposition_warning(CampaignID, "too_few_phases") :-
    campaign_too_trivial(CampaignID).

# -----------------------------------------------------------------------------
# 1.3 Confidence-Based Planning
# -----------------------------------------------------------------------------

# Low confidence plan needs user review
plan_needs_review(CampaignID) :-
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence < 70.

# High confidence allows auto-start
plan_can_autostart(CampaignID) :-
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence >= 85,
    !plan_needs_review(CampaignID).

# Trigger user clarification for low-confidence plans
next_action(/campaign_clarify) :-
    campaign(CampaignID, _, _, _, /planning),
    plan_needs_review(CampaignID).

# =============================================================================
# SECTION 2: INTELLIGENT TASK SCHEDULING
# =============================================================================
# Advanced task scheduling beyond basic priority ordering.

# -----------------------------------------------------------------------------
# 2.1 Parallel Task Detection
# -----------------------------------------------------------------------------

# Tasks are parallelizable if they don't share artifacts
tasks_parallelizable(TaskA, TaskB) :-
    campaign_task(TaskA, PhaseID, _, /pending, _),
    campaign_task(TaskB, PhaseID, _, /pending, _),
    TaskA != TaskB,
    !task_dependency(TaskA, TaskB),
    !task_dependency(TaskB, TaskA),
    !tasks_share_artifact(TaskA, TaskB).

# Helper: check if tasks touch the same file
tasks_share_artifact(TaskA, TaskB) :-
    task_artifact(TaskA, _, Path, _),
    task_artifact(TaskB, _, Path, _).

# Batch of parallelizable tasks (first 3)
parallel_batch_task(TaskID) :-
    eligible_task(TaskID),
    tasks_parallelizable(TaskID, _).

# Count parallelizable tasks (heuristic via multiple bindings)
has_parallel_opportunity(PhaseID) :-
    campaign_task(T1, PhaseID, _, /pending, _),
    campaign_task(T2, PhaseID, _, /pending, _),
    T1 != T2,
    tasks_parallelizable(T1, T2).

# -----------------------------------------------------------------------------
# 2.2 Task Complexity Estimation
# -----------------------------------------------------------------------------

# High complexity tasks get extra context
task_is_complex(TaskID) :-
    phase_estimate(PhaseID, _, /high),
    campaign_task(TaskID, PhaseID, _, _, _).

task_is_complex(TaskID) :-
    phase_estimate(PhaseID, _, /critical),
    campaign_task(TaskID, PhaseID, _, _, _).

# Simple tasks can run with minimal context
task_is_simple(TaskID) :-
    phase_estimate(PhaseID, _, /low),
    campaign_task(TaskID, PhaseID, _, _, _).

# Complex tasks should spawn specialist shards if available
prefer_specialist_for_task(TaskID, SpecialistName) :-
    task_is_complex(TaskID),
    campaign_task(TaskID, _, Description, _, TaskType),
    shard_profile(SpecialistName, /specialist, _),
    shard_can_handle(SpecialistName, TaskType).

# -----------------------------------------------------------------------------
# 2.3 Task Retry Strategy
# -----------------------------------------------------------------------------

# Task has exhausted basic retries
task_retry_exhausted(TaskID) :-
    task_attempt(TaskID, 3, /failure, _).

# Task should try with enriched context
task_needs_enrichment(TaskID) :-
    task_attempt(TaskID, AttemptNum, /failure, _),
    AttemptNum >= 1,
    AttemptNum < 3,
    !task_retry_exhausted(TaskID).

# Specific enrichment strategies based on failure type (computed in stratum 0)
# Note: /research for /unknown_api is a specific strategy, not the default
specific_enrichment(TaskID, /research) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /unknown_api, _).

specific_enrichment(TaskID, /documentation) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /missing_context, _).

specific_enrichment(TaskID, /decompose) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /too_complex, _).

specific_enrichment(TaskID, /specialist) :-
    task_needs_enrichment(TaskID),
    task_error(TaskID, /domain_specific, _).

# Helper: task has a specific enrichment strategy (for safe negation in stratum 1)
has_specific_enrichment(TaskID) :-
    specific_enrichment(TaskID, _).

# Final enrichment_strategy: either specific or default to /research (stratum 1)
enrichment_strategy(TaskID, Strategy) :-
    specific_enrichment(TaskID, Strategy).

enrichment_strategy(TaskID, /research) :-
    task_needs_enrichment(TaskID),
    !has_specific_enrichment(TaskID).

# =============================================================================
# SECTION 3: QUALITY ENFORCEMENT
# =============================================================================
# Rules to ensure campaign outputs meet quality standards.

# -----------------------------------------------------------------------------
# 3.1 Task Output Verification
# -----------------------------------------------------------------------------

# Task output needs verification if it produces code
task_needs_verification(TaskID) :-
    campaign_task(TaskID, _, _, /completed, /file_create).

task_needs_verification(TaskID) :-
    campaign_task(TaskID, _, _, /completed, /file_modify).

task_needs_verification(TaskID) :-
    campaign_task(TaskID, _, _, /completed, /test_write).

# Task passes verification if checkpoint succeeded
task_verified(TaskID) :-
    task_needs_verification(TaskID),
    phase_checkpoint(PhaseID, _, /true, _, _),
    campaign_task(TaskID, PhaseID, _, _, _).

# Unverified completed task (quality risk)
task_unverified(TaskID) :-
    task_needs_verification(TaskID),
    !task_verified(TaskID).

# Block phase completion if unverified tasks exist
has_unverified_task(PhaseID) :-
    campaign_task(TaskID, PhaseID, _, /completed, _),
    task_unverified(TaskID).

phase_blocked(PhaseID, "unverified_tasks") :-
    has_unverified_task(PhaseID).

# -----------------------------------------------------------------------------
# 3.2 Code Quality Gates
# -----------------------------------------------------------------------------

# Quality violation patterns (detected by shards)
quality_violation_detected(TaskID, /no_error_handling) :-
    campaign_task(TaskID, _, _, /completed, /file_create),
    task_artifact(TaskID, /source_file, Path, _),
    file_topology(Path, _, /go, _, _),
    quality_violation(TaskID, /missing_errors).

quality_violation_detected(TaskID, /no_tests) :-
    campaign_task(TaskID, _, _, /completed, /file_create),
    task_artifact(TaskID, /source_file, Path, _),
    !test_coverage(Path).

# Task with quality violations needs remediation
task_needs_remediation(TaskID) :-
    quality_violation_detected(TaskID, _).

# Auto-spawn remediation task
remediation_task_needed(TaskID, ViolationType) :-
    quality_violation_detected(TaskID, ViolationType),
    !remediation_task_exists(TaskID, ViolationType).

# Helper to check if remediation already spawned
# Note: ViolationType bound via task_remediation_target which tracks what we're fixing
remediation_task_exists(OriginalTaskID, ViolationType) :-
    campaign_task(RemediationID, _, _, _, /fix),
    task_dependency(RemediationID, OriginalTaskID),
    task_remediation_target(RemediationID, OriginalTaskID, ViolationType).

# -----------------------------------------------------------------------------
# 3.3 Build & Test Gates
# -----------------------------------------------------------------------------

# Phase cannot complete without passing build
phase_requires_build_pass(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    phase_objective(PhaseID, /create, _, /builds).

phase_requires_build_pass(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, _, _),
    phase_objective(PhaseID, /modify, _, /builds).

# Phase build check failed
phase_build_failed(PhaseID) :-
    phase_requires_build_pass(PhaseID),
    phase_checkpoint(PhaseID, /build, /false, _, _).

# Block phase on build failure
phase_blocked(PhaseID, "build_failed") :-
    phase_build_failed(PhaseID).

# Phase test check failed
phase_tests_failed(PhaseID) :-
    phase_checkpoint(PhaseID, /tests, /false, _, _).

phase_blocked(PhaseID, "tests_failed") :-
    phase_tests_failed(PhaseID).

# =============================================================================
# SECTION 4: CONTEXT BUDGET MANAGEMENT
# =============================================================================
# Rules for managing LLM context window during campaigns.

# -----------------------------------------------------------------------------
# 4.1 Context Pressure Detection
# -----------------------------------------------------------------------------

# Context is under pressure (> 80% utilized)
context_pressure_high(CampaignID) :-
    context_window_state(CampaignID, Used, Total, _),
    Utilization = fn:mult(100, fn:div(Used, Total)),
    Utilization > 80.

# Context is critical (> 95% utilized)
context_pressure_critical(CampaignID) :-
    context_window_state(CampaignID, Used, Total, _),
    Utilization = fn:mult(100, fn:div(Used, Total)),
    Utilization > 95.

# Trigger compression when pressure is high
next_action(/compress_context) :-
    current_campaign(CampaignID),
    context_pressure_high(CampaignID),
    !context_pressure_critical(CampaignID).

# Emergency compression when critical
next_action(/emergency_compress) :-
    current_campaign(CampaignID),
    context_pressure_critical(CampaignID).

# -----------------------------------------------------------------------------
# 4.2 Phase Context Isolation
# -----------------------------------------------------------------------------

# Helper to identify phases that already have compression
has_compression(PhaseID) :-
    context_compression(PhaseID, _, _, _).

# Context atoms from completed phases should be compressed
phase_context_stale(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    !has_compression(PhaseID).

# Trigger compression for stale phases
should_compress_phase(PhaseID) :-
    phase_context_stale(PhaseID).

# Priority: compress oldest completed phases first
compress_priority(PhaseID, Order) :-
    phase_context_stale(PhaseID),
    campaign_phase(PhaseID, _, _, Order, /completed, _).

# -----------------------------------------------------------------------------
# 4.3 Focus Amplification
# -----------------------------------------------------------------------------

# Current task artifacts get maximum activation
activation(Path, 200) :-
    next_campaign_task(TaskID),
    task_artifact(TaskID, _, Path, _).

# Current phase context atoms get high activation
activation(Atom, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Atom, _).

# Previous phase summaries get moderate activation
activation(Summary, 80) :-
    campaign_phase(PhaseID, CampaignID, _, Order, /completed, _),
    current_phase(CurrentPhaseID),
    campaign_phase(CurrentPhaseID, CampaignID, _, CurrentOrder, _, _),
    PrevOrder = fn:minus(CurrentOrder, 1),
    Order = PrevOrder,
    context_compression(PhaseID, Summary, _, _).

# =============================================================================
# SECTION 5: CROSS-PHASE LEARNING
# =============================================================================
# Rules for learning patterns across campaign execution.

# -----------------------------------------------------------------------------
# 5.1 Success Pattern Detection
# -----------------------------------------------------------------------------

# Phase completed quickly (success pattern)
phase_completed_fast(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    campaign_milestone(_, PhaseID, _, CompletedAt),
    phase_estimate(PhaseID, EstTasks, _),
    EstTasks > 0.

# Phase had no failures (success pattern)
phase_zero_failures(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /completed, _),
    !phase_had_failure(PhaseID).

# Helper: check if phase had any task failures
phase_had_failure(PhaseID) :-
    campaign_task(_, PhaseID, _, /failed, _).

# Learn from zero-failure phases
promote_to_long_term(/phase_pattern, PhaseType) :-
    phase_zero_failures(PhaseID),
    phase_objective(PhaseID, PhaseType, _, _).

# -----------------------------------------------------------------------------
# 5.2 Failure Pattern Detection
# -----------------------------------------------------------------------------

# Same error type across multiple tasks (systemic issue)
systemic_error_detected(ErrorType) :-
    task_error(T1, ErrorType, _),
    task_error(T2, ErrorType, _),
    task_error(T3, ErrorType, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Systemic error should trigger investigation
next_action(/investigate_systemic) :-
    current_campaign(_),
    systemic_error_detected(ErrorType),
    !systemic_error_investigated(ErrorType).

# Helper for safe negation
systemic_error_investigated(ErrorType) :-
    campaign_learning(_, /failure_pattern, ErrorType, _, _).

# Learn to avoid systemic errors
learning_signal(/avoid_pattern, ErrorType) :-
    systemic_error_detected(ErrorType).

# -----------------------------------------------------------------------------
# 5.3 Shard Performance in Campaigns
# -----------------------------------------------------------------------------

# Track which shard types perform well in campaigns
shard_campaign_success(ShardType) :-
    campaign_shard(_, ShardID, ShardType, _, /completed),
    shard_success(ShardID).

# Track shard failures in campaigns
shard_campaign_failure(ShardType) :-
    campaign_shard(_, ShardID, ShardType, _, /failed),
    shard_error(ShardID, _).

# Shard is reliable for campaigns (3+ successes, < 2 failures)
shard_campaign_reliable(ShardType) :-
    shard_campaign_success(ShardType),
    !shard_has_many_failures(ShardType).

# Helper: shard has 2+ failures
shard_has_many_failures(ShardType) :-
    campaign_shard(_, S1, ShardType, _, /failed),
    campaign_shard(_, S2, ShardType, _, /failed),
    S1 != S2.

# Prefer reliable shards for campaign tasks
delegate_task(ShardType, Task, /pending) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, Task, _, TaskType),
    shard_profile(ShardType, _, _),
    shard_campaign_reliable(ShardType),
    shard_can_handle(ShardType, TaskType).

# =============================================================================
# SECTION 6: RESILIENCE & RECOVERY
# =============================================================================
# Rules for handling failures and recovering campaigns.

# -----------------------------------------------------------------------------
# 6.1 Cascading Failure Detection
# -----------------------------------------------------------------------------

# Multiple consecutive task failures in same phase
phase_failure_cascade(PhaseID) :-
    campaign_task(T1, PhaseID, _, /failed, _),
    campaign_task(T2, PhaseID, _, /failed, _),
    campaign_task(T3, PhaseID, _, /failed, _),
    T1 != T2,
    T2 != T3,
    T1 != T3.

# Cascade triggers phase pause
phase_blocked(PhaseID, "failure_cascade") :-
    phase_failure_cascade(PhaseID).

# Cascade triggers replan consideration
replan_needed(CampaignID, "phase_failure_cascade") :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    phase_failure_cascade(PhaseID).

# -----------------------------------------------------------------------------
# 6.2 Stuck Detection
# -----------------------------------------------------------------------------

# Phase is stuck: in-progress but no runnable tasks
phase_stuck(PhaseID) :-
    campaign_phase(PhaseID, CampaignID, _, _, /in_progress, _),
    current_campaign(CampaignID),
    !has_runnable_task(PhaseID),
    has_pending_tasks(PhaseID).

# Helper: phase has runnable tasks
has_runnable_task(PhaseID) :-
    eligible_task(TaskID),
    campaign_task(TaskID, PhaseID, _, _, _).

# Helper: phase has pending tasks
has_pending_tasks(PhaseID) :-
    campaign_task(_, PhaseID, _, /pending, _).

# Helper: phase has running tasks
has_running_tasks(PhaseID) :-
    campaign_task(_, PhaseID, _, /in_progress, _).

# Stuck phase needs intervention
escalation_needed(/campaign, PhaseID, "phase_stuck") :-
    phase_stuck(PhaseID).

# Debug helper: why is task blocked?
debug_why_blocked(TaskID, DepID) :-
    task_dependency(TaskID, DepID),
    campaign_task(DepID, _, _, Status, _),
    /completed != Status,
    /skipped != Status.

# -----------------------------------------------------------------------------
# 6.3 Recovery Actions
# -----------------------------------------------------------------------------

# Skip non-critical failed task after 3 retries
task_can_skip(TaskID) :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /low).

task_can_skip(TaskID) :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /normal),
    !task_blocks_others(TaskID).

# Helper: task blocks other tasks
task_blocks_others(TaskID) :-
    task_dependency(_, TaskID).

# Critical task failure requires escalation
escalation_required(TaskID, "critical_task_failed") :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /critical).

escalation_required(TaskID, "high_priority_failed") :-
    task_retry_exhausted(TaskID),
    task_priority(TaskID, /high),
    task_blocks_others(TaskID).

# Auto-skip suggestion for non-blocking failed tasks
suggest_skip_task(TaskID) :-
    task_can_skip(TaskID),
    !task_blocks_others(TaskID).

# =============================================================================
# SECTION 7: MILESTONE & PROGRESS TRACKING
# =============================================================================
# Rules for tracking campaign progress and milestones.

# -----------------------------------------------------------------------------
# 7.1 Progress Calculation Helpers
# -----------------------------------------------------------------------------

# Phase is partially complete
phase_partial_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /in_progress, _),
    campaign_task(_, PhaseID, _, /completed, _).

# Phase has high completion rate (75%+)
phase_nearly_complete(PhaseID) :-
    campaign_phase(PhaseID, _, _, _, /in_progress, _),
    all_phase_tasks_complete(PhaseID).

# Campaign is past halfway point
campaign_past_halfway(CampaignID) :-
    campaign_progress(CampaignID, Completed, Total, _, _),
    Total > 0,
    Progress = fn:mult(100, fn:div(Completed, Total)),
    Progress >= 50.

# -----------------------------------------------------------------------------
# 7.2 Milestone Detection
# -----------------------------------------------------------------------------

# First phase complete = "foundation" milestone
milestone_reached(CampaignID, /foundation) :-
    campaign_phase(PhaseID, CampaignID, _, 1, /completed, _).

# All phases complete = "completion" milestone
milestone_reached(CampaignID, /completion) :-
    campaign_complete(CampaignID).

# Halfway milestone
milestone_reached(CampaignID, /halfway) :-
    campaign_past_halfway(CampaignID).

# Integration phase complete = "integrated" milestone
milestone_reached(CampaignID, /integrated) :-
    campaign_phase(PhaseID, CampaignID, _, _, /completed, _),
    phase_category(PhaseID, /integration).

# -----------------------------------------------------------------------------
# 7.3 Progress Events
# -----------------------------------------------------------------------------

# Trigger progress update on task completion
progress_changed(CampaignID) :-
    current_campaign(CampaignID),
    campaign_task(_, _, _, /completed, _).

progress_changed(CampaignID) :-
    current_campaign(CampaignID),
    campaign_phase(_, CampaignID, _, _, /completed, _).

# =============================================================================
# SECTION 8: SHARD SELECTION FOR CAMPAIGNS
# =============================================================================
# Rules for choosing the right shard for campaign tasks.

# -----------------------------------------------------------------------------
# 8.1 Task Type to Shard Mapping
# -----------------------------------------------------------------------------

# File creation → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /file_create).

# File modification → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /file_modify).

# Test writing → Tester
campaign_task_shard(TaskID, /tester) :-
    campaign_task(TaskID, _, _, /pending, /test_write).

# Test running → Tester
campaign_task_shard(TaskID, /tester) :-
    campaign_task(TaskID, _, _, /pending, /test_run).

# Research → Researcher
campaign_task_shard(TaskID, /researcher) :-
    campaign_task(TaskID, _, _, /pending, /research).

# Verification → Reviewer
campaign_task_shard(TaskID, /reviewer) :-
    campaign_task(TaskID, _, _, /pending, /verify).

# Refactoring → Coder (with reviewer support)
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /refactor).

# Integration → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /integrate).

# Documentation → Coder
campaign_task_shard(TaskID, /coder) :-
    campaign_task(TaskID, _, _, /pending, /document).

# -----------------------------------------------------------------------------
# 8.2 Specialist Override
# -----------------------------------------------------------------------------

# If specialist available and performs well, prefer it
campaign_task_shard_override(TaskID, SpecialistName) :-
    campaign_task(TaskID, _, _, /pending, TaskType),
    shard_profile(SpecialistName, /specialist, _),
    shard_campaign_reliable(SpecialistName),
    shard_can_handle(SpecialistName, TaskType).

# Final shard selection (specialist override or default)
final_shard_for_task(TaskID, SpecialistName) :-
    campaign_task_shard_override(TaskID, SpecialistName).

final_shard_for_task(TaskID, ShardType) :-
    campaign_task_shard(TaskID, ShardType),
    !campaign_task_shard_override(TaskID, _).

# =============================================================================
# SECTION 9: CAMPAIGN INTENT HANDLING
# =============================================================================
# Rules for handling user intents during active campaigns.

# -----------------------------------------------------------------------------
# 9.1 Intent Interruption
# -----------------------------------------------------------------------------

# User mutation intent during campaign needs routing decision
campaign_intent_conflict(IntentID) :-
    current_campaign(_),
    user_intent(IntentID, /mutation, _, _, _),
    !intent_is_campaign_related(IntentID).

# Intent is campaign-related if it matches current phase
intent_is_campaign_related(IntentID) :-
    current_campaign(CampaignID),
    current_phase(PhaseID),
    user_intent(IntentID, _, _, Target, _),
    phase_objective(PhaseID, _, Description, _).

# Route conflict to user for decision
next_action(/ask_campaign_interrupt) :-
    campaign_intent_conflict(_).

# -----------------------------------------------------------------------------
# 9.2 Campaign Queries
# -----------------------------------------------------------------------------

# Status query during campaign
next_action(/show_campaign_status) :-
    current_campaign(_),
    user_intent(_, /query, _, /status, _).

# Progress query during campaign
next_action(/show_campaign_progress) :-
    current_campaign(_),
    user_intent(_, /query, _, /progress, _).

# =============================================================================
# SECTION 10: CAMPAIGN COMPLETION & CLEANUP
# =============================================================================
# Rules for finalizing and learning from completed campaigns.

# -----------------------------------------------------------------------------
# 10.1 Completion Verification
# -----------------------------------------------------------------------------

# Campaign ready for completion verification
campaign_completion_ready(CampaignID) :-
    current_campaign(CampaignID),
    !has_incomplete_phase(CampaignID),
    !has_unverified_task_in_campaign(CampaignID).

# Helper: any unverified task in campaign
has_unverified_task_in_campaign(CampaignID) :-
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    has_unverified_task(PhaseID).

# Final verification action
next_action(/campaign_final_verify) :-
    campaign_completion_ready(_).

# -----------------------------------------------------------------------------
# 10.2 Post-Campaign Learning
# -----------------------------------------------------------------------------

# Extract success patterns from completed campaign
campaign_success_pattern(CampaignID, Pattern) :-
    campaign(CampaignID, Type, _, _, /completed),
    campaign_metadata(CampaignID, _, _, Confidence),
    Confidence > 80,
    Pattern = Type.

# Promote campaign success to long-term memory
promote_to_long_term(/campaign_success, Pattern) :-
    campaign_success_pattern(_, Pattern).

# Track campaign type effectiveness
campaign_type_effective(Type) :-
    campaign(C1, Type, _, _, /completed),
    campaign(C2, Type, _, _, /completed),
    C1 != C2.

# Learn from campaign failures
campaign_failure_pattern(CampaignID, Reason) :-
    campaign(CampaignID, _, _, _, /failed),
    campaign_blocked(CampaignID, Reason).

learning_signal(/campaign_failure, Reason) :-
    campaign_failure_pattern(_, Reason).

# -----------------------------------------------------------------------------
# 10.3 Cleanup Actions
# -----------------------------------------------------------------------------

# Compress all context after campaign completion
next_action(/campaign_cleanup) :-
    campaign(CampaignID, _, _, _, /completed),
    !campaign_cleaned(CampaignID).

# Helper for safe negation
campaign_cleaned(CampaignID) :-
    campaign_learning(CampaignID, /cleanup, _, _, _).

# Archive campaign artifacts
next_action(/archive_campaign) :-
    campaign(CampaignID, _, _, _, /completed),
    campaign_cleaned(CampaignID).

# =============================================================================
# SECTION 11: OBSERVABILITY & DEBUGGING
# =============================================================================
# Rules for campaign monitoring and debugging.

# -----------------------------------------------------------------------------
# 11.1 Health Indicators
# -----------------------------------------------------------------------------

# Campaign is healthy (making progress)
campaign_healthy(CampaignID) :-
    current_campaign(CampaignID),
    !campaign_blocked(CampaignID, _),
    !phase_stuck(_),
    !context_pressure_critical(CampaignID).

# Campaign has issues
campaign_has_issues(CampaignID) :-
    current_campaign(CampaignID),
    !campaign_healthy(CampaignID).

# -----------------------------------------------------------------------------
# 11.2 Diagnostic Queries
# -----------------------------------------------------------------------------

# Why is campaign blocked? (for debugging)
campaign_block_reason(CampaignID, Reason) :-
    campaign_blocked(CampaignID, Reason).

# Why is phase blocked? (for debugging)
phase_block_reason(PhaseID, Reason) :-
    phase_blocked(PhaseID, Reason).

# Task failure summary (for debugging)
task_failure_summary(TaskID, ErrorType, ErrorMsg) :-
    campaign_task(TaskID, _, _, /failed, _),
    task_error(TaskID, ErrorType, ErrorMsg).

# -----------------------------------------------------------------------------
# 11.3 Metrics Derivation
# -----------------------------------------------------------------------------

# Helper for task counts (simplified - exact counting in Go)
campaign_has_tasks(CampaignID) :-
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    campaign_task(_, PhaseID, _, _, _).

# Phases per campaign (simplified heuristic)
campaign_has_multiple_phases(CampaignID) :-
    campaign_phase(P1, CampaignID, _, _, _, _),
    campaign_phase(P2, CampaignID, _, _, _, _),
    P1 != P2.

# =============================================================================
# SECTION 12: INTEGRATION WITH BUILD TOPOLOGY
# =============================================================================
# Rules that connect campaign execution with build topology validation.

# -----------------------------------------------------------------------------
# 12.1 Topology Violation Handling
# -----------------------------------------------------------------------------

# Block task if it would create architectural violation
task_blocked_by_topology(TaskID) :-
    campaign_task(TaskID, PhaseID, _, /pending, _),
    architectural_violation(PhaseID, _, _).

# Task warning for suspicious gaps
task_topology_warning(TaskID, "skips_layer") :-
    campaign_task(TaskID, PhaseID, _, /pending, _),
    suspicious_gap(PhaseID, _).

# Block campaign start if topology violations exist
campaign_blocked(CampaignID, "topology_violations") :-
    campaign(CampaignID, _, _, _, /validating),
    campaign_phase(PhaseID, CampaignID, _, _, _, _),
    architectural_violation(PhaseID, _, _).

# -----------------------------------------------------------------------------
# 12.2 Layer-Aware Scheduling
# -----------------------------------------------------------------------------

# Prefer lower-layer phases first (enforces build order)
phase_layer_priority(PhaseID, Priority) :-
    phase_precedence(PhaseID, Priority).

# Earlier layers should complete before later layers start
layer_sequencing_correct(PhaseA, PhaseB) :-
    phase_layer_priority(PhaseA, PriorityA),
    phase_layer_priority(PhaseB, PriorityB),
    phase_dependency(PhaseB, PhaseA, _),
    PriorityA < PriorityB.

# Warning if layer sequencing is wrong
layer_sequencing_warning(PhaseA, PhaseB) :-
    phase_dependency(PhaseB, PhaseA, _),
    phase_layer_priority(PhaseA, PriorityA),
    phase_layer_priority(PhaseB, PriorityB),
    PriorityA >= PriorityB.

# =============================================================================
# END OF CAMPAIGN RULES
# =============================================================================


# Document selection policy for large-spec ingestion
# Leverages doc_metadata and campaign_goal to prune irrelevant sources

# Exclude experimental docs unless explicitly requested
is_irrelevant(Path) :-
    doc_tag(Path, /experimental).

# Base relevance: matches goal topics
is_relevant(Path) :-
    doc_metadata(CampID, Path, _, _, _),
    doc_tag(Path, Topic),
    goal_topic(CampID, Topic),
    !is_irrelevant(Path).

# Hard dependencies: propagate relevance through references
is_relevant(Path) :-
    doc_metadata(CampID, Path, _, _, _),
    doc_metadata(CampID, Parent, _, _, _),
    doc_reference(Parent, Path),
    is_relevant(Parent),
    !is_irrelevant(Path).

# Pull in referenced documents transitively
include_in_context(Path) :- is_relevant(Path).
include_in_context(Dep) :-
    include_in_context(Parent),
    doc_reference(Parent, Dep).


Decl verb_def(Verb, Category, Shard, Priority).

Decl verb_synonym(Verb, Synonym).

Decl verb_pattern(Verb, Regex).

# =========================================================================
# CODE REVIEW & ANALYSIS (Reviewer)
# =========================================================================

# /review
verb_def(/review, /query, /reviewer, 100).
verb_synonym(/review, "review").
verb_synonym(/review, "code review").
verb_synonym(/review, "pr review").
verb_synonym(/review, "check code").
verb_synonym(/review, "audit").
verb_synonym(/review, "evaluate").
verb_synonym(/review, "critique").
# Regexes simplified to avoid backslash hell - using dot for wildcard
verb_pattern(/review, "(?i)review.*code").
verb_pattern(/review, "(?i)can.*you.*review").
verb_pattern(/review, "(?i)check.*(this|the).*file").

# /security
verb_def(/security, /query, /reviewer, 105).
verb_synonym(/security, "security").
verb_synonym(/security, "security scan").
verb_synonym(/security, "vulnerability").
verb_synonym(/security, "injection").
verb_synonym(/security, "xss").
verb_pattern(/security, "(?i)security.*scan").
verb_pattern(/security, "(?i)check.*for.*vuln").
verb_pattern(/security, "(?i)find.*vulnerabilities").

# /analyze
verb_def(/analyze, /query, /reviewer, 95).
verb_synonym(/analyze, "analyze").
verb_synonym(/analyze, "complexity").
verb_synonym(/analyze, "metrics").
verb_synonym(/analyze, "lint").
verb_synonym(/analyze, "code smell").
verb_pattern(/analyze, "(?i)analyze.*code").
verb_pattern(/analyze, "(?i)static.*analysis").

# =========================================================================
# UNDERSTANDING (Researcher/None)
# =========================================================================

# /explain
verb_def(/explain, /query, /none, 80).
verb_synonym(/explain, "explain").
verb_synonym(/explain, "describe").
verb_synonym(/explain, "what is").
verb_synonym(/explain, "how does").
verb_synonym(/explain, "help me understand").
verb_pattern(/explain, "(?i)explain.*this").
verb_pattern(/explain, "(?i)tell.*me.*about").
verb_pattern(/explain, "(?i)help.*understand").

# /explore
verb_def(/explore, /query, /researcher, 75).
verb_synonym(/explore, "explore").
verb_synonym(/explore, "browse").
verb_synonym(/explore, "show structure").
verb_synonym(/explore, "list files").
verb_pattern(/explore, "(?i)show.*structure").
verb_pattern(/explore, "(?i)explore.*codebase").

# /search
verb_def(/search, /query, /researcher, 85).
verb_synonym(/search, "search").
verb_synonym(/search, "find").
verb_synonym(/search, "grep").
verb_synonym(/search, "occurrences").
verb_pattern(/search, "(?i)search.*for").
verb_pattern(/search, "(?i)find.*all").
verb_pattern(/search, "(?i)grep").

# =========================================================================
# MUTATION (Coder)
# =========================================================================

# /fix
verb_def(/fix, /mutation, /coder, 90).
verb_synonym(/fix, "fix").
verb_synonym(/fix, "repair").
verb_synonym(/fix, "patch").
verb_synonym(/fix, "resolve").
verb_synonym(/fix, "bug fix").
verb_pattern(/fix, "(?i)fix.*bug").
verb_pattern(/fix, "(?i)repair.*this").
verb_pattern(/fix, "(?i)resolve.*issue").

# /refactor
verb_def(/refactor, /mutation, /coder, 88).
verb_synonym(/refactor, "refactor").
verb_synonym(/refactor, "clean up").
verb_synonym(/refactor, "improve").
verb_synonym(/refactor, "optimize").
verb_synonym(/refactor, "simplify").
verb_pattern(/refactor, "(?i)refactor").
verb_pattern(/refactor, "(?i)clean.*up").
verb_pattern(/refactor, "(?i)improve.*code").

# /create
verb_def(/create, /mutation, /coder, 85).
verb_synonym(/create, "create").
verb_synonym(/create, "new").
verb_synonym(/create, "add").
verb_synonym(/create, "implement").
verb_synonym(/create, "generate").
verb_pattern(/create, "(?i)create.*new").
verb_pattern(/create, "(?i)add.*new").
verb_pattern(/create, "(?i)implement").

# /write
verb_def(/write, /mutation, /coder, 70).
verb_synonym(/write, "write").
verb_synonym(/write, "save").
verb_synonym(/write, "export").
verb_pattern(/write, "(?i)write.*to").
verb_pattern(/write, "(?i)save.*to").

# /delete
verb_def(/delete, /mutation, /coder, 85).
verb_synonym(/delete, "delete").
verb_synonym(/delete, "remove").
verb_synonym(/delete, "drop").
verb_pattern(/delete, "(?i)delete").
verb_pattern(/delete, "(?i)remove").

# =========================================================================
# DEBUGGING (Coder)
# =========================================================================

# /debug
verb_def(/debug, /query, /coder, 92).
verb_synonym(/debug, "debug").
verb_synonym(/debug, "troubleshoot").
verb_synonym(/debug, "diagnose").
verb_synonym(/debug, "root cause").
verb_pattern(/debug, "(?i)debug").
verb_pattern(/debug, "(?i)troubleshoot").
verb_pattern(/debug, "(?i)why.*fail").

# =========================================================================
# TESTING (Tester)
# =========================================================================

# /test
verb_def(/test, /mutation, /tester, 88).
verb_synonym(/test, "test").
verb_synonym(/test, "unit test").
verb_synonym(/test, "run tests").
verb_synonym(/test, "coverage").
verb_pattern(/test, "(?i)write.*test").
verb_pattern(/test, "(?i)run.*test").
verb_pattern(/test, "(?i)test.*coverage").

# =========================================================================
# RESEARCH (Researcher)
# =========================================================================

# /research
verb_def(/research, /query, /researcher, 75).
verb_synonym(/research, "research").
verb_synonym(/research, "learn").
verb_synonym(/research, "docs").
verb_synonym(/research, "documentation").
verb_pattern(/research, "(?i)research").
verb_pattern(/research, "(?i)learn.*about").
verb_pattern(/research, "(?i)find.*docs").

# =========================================================================
# SETUP & CONFIG
# =========================================================================

# /init
verb_def(/init, /mutation, /researcher, 70).
verb_synonym(/init, "init").
verb_synonym(/init, "setup").
verb_synonym(/init, "bootstrap").
verb_pattern(/init, "(?i)^init").
verb_pattern(/init, "(?i)set.*up").

# /configure
verb_def(/configure, /instruction, /none, 65).
verb_synonym(/configure, "configure").
verb_synonym(/configure, "config").
verb_synonym(/configure, "settings").
verb_pattern(/configure, "(?i)configure").
verb_pattern(/configure, "(?i)change.*setting").

# =========================================================================
# CAMPAIGN
# =========================================================================

# /campaign
verb_def(/campaign, /mutation, /coder, 95).
verb_synonym(/campaign, "campaign").
verb_synonym(/campaign, "epic").
verb_synonym(/campaign, "feature").
verb_pattern(/campaign, "(?i)start.*campaign").
verb_pattern(/campaign, "(?i)implement.*feature").

# =========================================================================
# AUTOPOIESIS (Tool Generation)
# =========================================================================

# /generate_tool
verb_def(/generate_tool, /mutation, /tool_generator, 95).
verb_synonym(/generate_tool, "generate tool").
verb_synonym(/generate_tool, "create tool").
verb_synonym(/generate_tool, "need a tool").
verb_pattern(/generate_tool, "(?i)create.*tool").
verb_pattern(/generate_tool, "(?i)need.*tool").

# =========================================================================
# MULTI-STEP VERB TAXONOMY
# Encyclopedic definitions for multi-step task detection and decomposition
# =========================================================================

# --- Multi-Step Declarations ---
Decl verb_composition(Verb1, Verb2, Relation, Priority).
Decl step_connector(Connector, ConnectorType, StepBoundary).
Decl completion_marker(Marker, MarkerType).
Decl pronoun_ref(Pronoun, Resolution).
Decl constraint_marker(Marker, ConstraintType).

# --- Step Relations ---
# sequential: Verb2 depends on Verb1 completing
# parallel: Verb1 and Verb2 can run concurrently
# conditional: Verb2 runs only if Verb1 succeeds
# fallback: Verb2 runs only if Verb1 fails
# iterative: Verb1 repeats over a collection

# =========================================================================
# VERB COMPOSITIONS - Which verbs naturally follow each other
# =========================================================================

# --- Review-Then-Fix Compositions (highest priority) ---
verb_composition(/review, /fix, "sequential", 95).
verb_composition(/analyze, /fix, "sequential", 93).
verb_composition(/security, /fix, "sequential", 97).
verb_composition(/debug, /fix, "sequential", 94).
verb_composition(/review, /refactor, "sequential", 90).
verb_composition(/analyze, /refactor, "sequential", 88).

# --- Create-Then-Validate Compositions ---
verb_composition(/create, /test, "sequential", 92).
verb_composition(/fix, /test, "sequential", 94).
verb_composition(/refactor, /test, "sequential", 91).
verb_composition(/create, /review, "sequential", 85).
verb_composition(/fix, /review, "sequential", 86).

# --- Research-Then-Act Compositions ---
verb_composition(/research, /create, "sequential", 88).
verb_composition(/research, /fix, "sequential", 87).
verb_composition(/research, /refactor, "sequential", 86).
verb_composition(/explore, /create, "sequential", 85).
verb_composition(/explore, /refactor, "sequential", 84).

# --- Analysis-Then-Optimize Compositions ---
verb_composition(/analyze, /refactor, "sequential", 89).
verb_composition(/analyze, /fix, "sequential", 88).
verb_composition(/review, /optimize, "sequential", 87).

# --- Documentation Compositions ---
verb_composition(/create, /document, "sequential", 80).
verb_composition(/refactor, /document, "sequential", 79).
verb_composition(/fix, /document, "sequential", 78).

# --- Git Workflow Compositions ---
verb_composition(/fix, /commit, "sequential", 85).
verb_composition(/create, /commit, "sequential", 84).
verb_composition(/refactor, /commit, "sequential", 83).
verb_composition(/commit, /push, "sequential", 90).
verb_composition(/fix, /push, "sequential", 82).

# --- Parallel Analysis Compositions ---
verb_composition(/review, /security, "parallel", 75).
verb_composition(/review, /analyze, "parallel", 74).
verb_composition(/test, /lint, "parallel", 76).

# --- Conditional Compositions ---
verb_composition(/test, /commit, "conditional", 88).
verb_composition(/test, /push, "conditional", 87).
verb_composition(/test, /deploy, "conditional", 90).
verb_composition(/fix, /deploy, "conditional", 85).

# --- Fallback Compositions ---
verb_composition(/migrate, /rollback, "fallback", 85).
verb_composition(/deploy, /rollback, "fallback", 88).
verb_composition(/refactor, /revert, "fallback", 80).

# =========================================================================
# STEP CONNECTORS - Words that signal step boundaries
# =========================================================================

# --- Sequential Connectors (explicit ordering) ---
step_connector("first", "sequential_start", /true).
step_connector("then", "sequential_continue", /true).
step_connector("next", "sequential_continue", /true).
step_connector("after that", "sequential_continue", /true).
step_connector("afterwards", "sequential_continue", /true).
step_connector("afterward", "sequential_continue", /true).
step_connector("following that", "sequential_continue", /true).
step_connector("subsequently", "sequential_continue", /true).
step_connector("finally", "sequential_end", /true).
step_connector("lastly", "sequential_end", /true).
step_connector("start by", "sequential_start", /true).
step_connector("begin with", "sequential_start", /true).
step_connector("once done", "sequential_continue", /true).
step_connector("when done", "sequential_continue", /true).
step_connector("when finished", "sequential_continue", /true).
step_connector("after done", "sequential_continue", /true).
step_connector("after you're done", "sequential_continue", /true).
step_connector("when complete", "sequential_continue", /true).
step_connector("once complete", "sequential_continue", /true).

# --- Numbered Step Connectors ---
step_connector("step 1", "numbered", /true).
step_connector("step 2", "numbered", /true).
step_connector("step 3", "numbered", /true).
step_connector("step 4", "numbered", /true).
step_connector("step 5", "numbered", /true).
step_connector("1.", "numbered", /true).
step_connector("2.", "numbered", /true).
step_connector("3.", "numbered", /true).
step_connector("4.", "numbered", /true).
step_connector("5.", "numbered", /true).
step_connector("1)", "numbered", /true).
step_connector("2)", "numbered", /true).
step_connector("3)", "numbered", /true).
step_connector("first,", "numbered", /true).
step_connector("second,", "numbered", /true).
step_connector("third,", "numbered", /true).

# --- Implicit Sequential Connectors ---
step_connector("and", "implicit_sequential", /false).
step_connector("and then", "sequential_continue", /true).
step_connector("then also", "sequential_continue", /true).

# --- Parallel Connectors ---
step_connector("also", "parallel", /true).
step_connector("additionally", "parallel", /true).
step_connector("at the same time", "parallel", /true).
step_connector("simultaneously", "parallel", /true).
step_connector("in parallel", "parallel", /true).
step_connector("as well as", "parallel", /false).
step_connector("along with", "parallel", /false).
step_connector("together with", "parallel", /false).
step_connector("plus", "parallel", /false).

# --- Conditional Success Connectors ---
step_connector("if it works", "conditional_success", /true).
step_connector("if successful", "conditional_success", /true).
step_connector("if it passes", "conditional_success", /true).
step_connector("if tests pass", "conditional_success", /true).
step_connector("when tests pass", "conditional_success", /true).
step_connector("once tests pass", "conditional_success", /true).
step_connector("on success", "conditional_success", /true).
step_connector("assuming it works", "conditional_success", /true).
step_connector("provided it works", "conditional_success", /true).
step_connector("if no errors", "conditional_success", /true).
step_connector("if it compiles", "conditional_success", /true).
step_connector("if build succeeds", "conditional_success", /true).
step_connector("once tests are green", "conditional_success", /true).
step_connector("when green", "conditional_success", /true).

# --- Conditional Failure / Fallback Connectors ---
step_connector("if it fails", "conditional_failure", /true).
step_connector("if it doesn't work", "conditional_failure", /true).
step_connector("if it breaks", "conditional_failure", /true).
step_connector("otherwise", "conditional_failure", /true).
step_connector("or else", "conditional_failure", /true).
step_connector("on failure", "conditional_failure", /true).
step_connector("on error", "conditional_failure", /true).
step_connector("if errors occur", "conditional_failure", /true).
step_connector("if tests fail", "conditional_failure", /true).
step_connector("revert if fails", "conditional_failure", /true).
step_connector("rollback if fails", "conditional_failure", /true).
step_connector("undo if fails", "conditional_failure", /true).
step_connector("revert if needed", "conditional_failure", /true).
step_connector("if something goes wrong", "conditional_failure", /true).

# --- Pipeline Connectors ---
step_connector("pass the results to", "pipeline", /true).
step_connector("feed output to", "pipeline", /true).
step_connector("use the results to", "pipeline", /true).
step_connector("pipe to", "pipeline", /true).
step_connector("based on the results", "pipeline", /true).
step_connector("according to findings", "pipeline", /true).
step_connector("based on issues", "pipeline", /true).
step_connector("using the output", "pipeline", /true).

# =========================================================================
# COMPLETION MARKERS - Words that indicate task boundaries
# =========================================================================

completion_marker("done", "completion").
completion_marker("finished", "completion").
completion_marker("complete", "completion").
completion_marker("completed", "completion").
completion_marker("all done", "completion").
completion_marker("that's it", "completion").
completion_marker("verified", "verification").
completion_marker("confirmed", "verification").
completion_marker("tested", "verification").
completion_marker("working", "verification").
completion_marker("passes", "verification").
completion_marker("green", "verification").
completion_marker("ready", "readiness").
completion_marker("ready to", "readiness").
completion_marker("ready for", "readiness").

# =========================================================================
# PRONOUN REFERENCES - How pronouns resolve to targets
# =========================================================================

pronoun_ref("it", "previous_target").
pronoun_ref("them", "previous_targets").
pronoun_ref("this", "context_target").
pronoun_ref("that", "previous_target").
pronoun_ref("these", "previous_targets").
pronoun_ref("those", "previous_targets").
pronoun_ref("the file", "previous_file").
pronoun_ref("the files", "previous_files").
pronoun_ref("the code", "previous_code").
pronoun_ref("the function", "previous_function").
pronoun_ref("the changes", "previous_changes").
pronoun_ref("the results", "previous_output").
pronoun_ref("the output", "previous_output").
pronoun_ref("the findings", "previous_output").
pronoun_ref("the issues", "previous_issues").
pronoun_ref("the bugs", "previous_bugs").
pronoun_ref("the errors", "previous_errors").

# =========================================================================
# CONSTRAINT MARKERS - Words that modify scope
# =========================================================================

constraint_marker("but not", "exclusion").
constraint_marker("but skip", "exclusion").
constraint_marker("except", "exclusion").
constraint_marker("except for", "exclusion").
constraint_marker("excluding", "exclusion").
constraint_marker("without", "exclusion").
constraint_marker("don't touch", "exclusion").
constraint_marker("leave alone", "exclusion").
constraint_marker("skip", "exclusion").
constraint_marker("ignore", "exclusion").
constraint_marker("while keeping", "preservation").
constraint_marker("while preserving", "preservation").
constraint_marker("while maintaining", "preservation").
constraint_marker("preserving", "preservation").
constraint_marker("keeping", "preservation").
constraint_marker("maintaining", "preservation").
constraint_marker("without breaking", "preservation").
constraint_marker("without changing", "preservation").
constraint_marker("only", "inclusion").
constraint_marker("just", "inclusion").
constraint_marker("only the", "inclusion").
constraint_marker("just the", "inclusion").
constraint_marker("specifically", "inclusion").
constraint_marker("in particular", "inclusion").

# =========================================================================
# ITERATIVE MARKERS - Words that signal repetition
# =========================================================================

Decl iterative_marker(Marker, IterationType).

iterative_marker("each", "collection").
iterative_marker("every", "collection").
iterative_marker("all", "collection").
iterative_marker("all the", "collection").
iterative_marker("for each", "loop").
iterative_marker("for every", "loop").
iterative_marker("for all", "loop").
iterative_marker("one by one", "sequential_iteration").
iterative_marker("across all", "collection").
iterative_marker("throughout", "scope").
iterative_marker("everywhere", "scope").
iterative_marker("in all files", "file_scope").
iterative_marker("in all functions", "function_scope").
iterative_marker("in the entire", "full_scope").
iterative_marker("the whole", "full_scope").
iterative_marker("the entire", "full_scope").

# =========================================================================
# URGENCY/PRIORITY MARKERS
# =========================================================================

Decl priority_marker(Marker, PriorityLevel).

priority_marker("urgent", "high").
priority_marker("urgently", "high").
priority_marker("asap", "high").
priority_marker("immediately", "high").
priority_marker("right now", "high").
priority_marker("quickly", "medium").
priority_marker("soon", "medium").
priority_marker("when you can", "low").
priority_marker("eventually", "low").
priority_marker("at some point", "low").
priority_marker("critical", "critical").
priority_marker("blocking", "critical").
priority_marker("blocker", "critical").

# =========================================================================
# VERIFICATION MARKERS - Words that trigger verification steps
# =========================================================================

Decl verification_marker(Marker, VerificationType).

verification_marker("make sure", "verification_required").
verification_marker("ensure", "verification_required").
verification_marker("verify", "verification_required").
verification_marker("confirm", "verification_required").
verification_marker("check that", "verification_required").
verification_marker("validate", "verification_required").
verification_marker("test that", "test_verification").
verification_marker("run tests", "test_verification").
verification_marker("run the tests", "test_verification").
verification_marker("and test", "test_verification").
verification_marker("it works", "functional_verification").
verification_marker("it compiles", "build_verification").
verification_marker("it builds", "build_verification").
verification_marker("no errors", "error_verification").
verification_marker("no warnings", "warning_verification").

# =========================================================================
# INFERENCE RULES FOR MULTI-STEP DETECTION
# =========================================================================

# Check if two verbs can be composed
Decl can_compose(Verb1, Verb2).
can_compose(Verb1, Verb2) :-
    verb_composition(Verb1, Verb2, _, _).

# Get the default relation between two verbs
Decl verb_pair_relation(Verb1, Verb2, Relation).
verb_pair_relation(Verb1, Verb2, Relation) :-
    verb_composition(Verb1, Verb2, Relation, _).

# Check if a connector indicates a step boundary
Decl is_step_boundary(Connector).
is_step_boundary(Connector) :-
    step_connector(Connector, _, /true).

# Get connector type
Decl connector_type(Connector, Type).
connector_type(Connector, Type) :-
    step_connector(Connector, Type, _).

# Check if a word is an iterative marker
Decl is_iterative(Word).
is_iterative(Word) :-
    iterative_marker(Word, _).

# Check if a phrase needs verification
Decl needs_verification(Marker).
needs_verification(Marker) :-
    verification_marker(Marker, _).

# Resolve pronoun to reference type
Decl resolve_pronoun(Pronoun, RefType).
resolve_pronoun(Pronoun, RefType) :-
    pronoun_ref(Pronoun, RefType).


# Inference Logic for Intent Refinement (Simplified)
# This module takes raw intent candidates (from regex/LLM) and refines them
# using contextual logic and safety constraints.

Decl candidate_intent(Verb, RawScore).
Decl context_token(Token).

# Decl system_state(Key, Value).

# Declare Boost and Penalty predicates
Decl boost(Verb, Amount).
Decl penalty(Verb, Amount).

# Output: Refined Score
Decl refined_score(Verb, Score).

# Base score from candidate
refined_score(Verb, Score) :-
    candidate_intent(Verb, Score).

# -----------------------------------------------------------------------------
# CONTEXTUAL BOOSTING
# -----------------------------------------------------------------------------
# NOTE: Boost/penalty values are integers (0-100 scale) because fn:plus/fn:minus
# only support integers, not floats.

# Security Boost: If "security" or "vuln" appears, boost /security
boost(Verb, 30) :-
    candidate_intent(Verb, _),
    Verb = /security,
    context_token("security").

boost(Verb, 30) :-
    candidate_intent(Verb, _),
    Verb = /security,
    context_token("vulnerability").

# Testing Boost: If "coverage" appears, prefer /test over /review
boost(Verb, 20) :-
    candidate_intent(Verb, _),
    Verb = /test,
    context_token("coverage").

# Debugging Boost: If "error" or "panic" appears, prefer /debug over /fix
# fixing is the goal, but debugging is the immediate action.
boost(Verb, 15) :-
    candidate_intent(Verb, _),
    Verb = /debug,
    context_token("panic").

boost(Verb, 15) :-
    candidate_intent(Verb, _),
    Verb = /debug,
    context_token("stacktrace").

# -----------------------------------------------------------------------------
# SAFETY CONSTRAINTS (Penalties)
# -----------------------------------------------------------------------------

# Safety: Don't /delete if we are in a "learning" mode or context implies "safe"
penalty(Verb, 50) :-
    candidate_intent(Verb, _),
    Verb = /delete,
    context_token("safe").

# Ambiguity: If "fix" and "test" both appear, "fix" usually dominates,
# but if "verify" is present, "test" should win.
boost(Verb, 20) :-
    candidate_intent(Verb, _),
    Verb = /test,
    context_token("verify").

# -----------------------------------------------------------------------------
# FINAL SCORE CALCULATION (Relational Max, No Pipes)
# -----------------------------------------------------------------------------

# Generate potential scores by applying single boosts.
Decl potential_score(Verb, Score).

# 1. Base Score is a potential score
potential_score(Verb, Score) :- candidate_intent(Verb, Score).

# 2. Boosted Scores (Apply Boost)
# S = Base + Amount
potential_score(Verb, S) :-
    candidate_intent(Verb, Base),
    boost(Verb, Amount),
    S = fn:plus(Base, Amount).

# 3. Penalized Scores (Apply Penalty)
# S = Base - Amount (using minus 0, Amount)
potential_score(Verb, S) :-
    candidate_intent(Verb, Base),
    penalty(Verb, Amount),
    Neg = fn:minus(0, Amount),
    S = fn:plus(Base, Neg).

# 4. Relational Max Logic
# Find scores that are NOT max
Decl has_greater_score(Score).
has_greater_score(S) :-
    potential_score(_, S),
    potential_score(_, Other),
    Other > S.

# Define max score as one that has no greater score
Decl best_score(MaxScore).
best_score(S) :-
    potential_score(_, S),
    !has_greater_score(S).

# Select verb matching the max score
Decl selected_verb(Verb).
selected_verb(Verb) :-
    potential_score(Verb, S),
    best_score(Max),
    S = Max.

# JIT Compiler Logic (The "Gatekeeper")
# Determines which atoms are selected for the final prompt.

# --- 1. SKELETON (Deterministic Selection) ---

# Context Matching Helper
# An atom matches context if ALL its tag dimensions align with current_context.
# (Logic: For every tag dimension D required by Atom, current_context must have a matching tag).
# This is tricky in Datalog without "forall".
# Simplified Approach: An atom is "mismatched" if it has a tag that CONTRADICTS current context.
# Assuming atom_tag implies "Required".

# Helper: Atom has a tag in Dimension D, but context has a DIFFERENT tag in Dimension D.
# (Implicitly assuming single-value per dimension in context, identifying mismatch).
# tag_mismatch(Atom) :-
#     atom_tag(Atom, Dim, Tag),
#     current_context(Dim, CtxTag),
#     Tag != CtxTag.
    
# Better Approach: Positive Matching
# An atom matches if it is NOT mismatched.
# matches_context(Atom) :-
#     atom(Atom),
#     !tag_mismatch(Atom).

# Wait, tags can be multi-valued (e.g., supports /go AND /python).
# So: Mismatch is if Atom defines a set of tags for Dim, and Context has a tag for Dim, 
# but Context's tag is NOT in Atom's set.
# This requires knowing if Atom HAS a constraint on Dim.

has_constraint(Atom, Dim) :- atom_tag(Atom, Dim, _).

satisfied_constraint(Atom, Dim) :-
    atom_tag(Atom, Dim, Tag),
    current_context(Dim, Tag).
    
# An atom is blocked if it has a constraint on Dim, but constraint is not satisfied.
blocked_by_context(Atom) :-
    has_constraint(Atom, Dim),
    !satisfied_constraint(Atom, Dim).

# Safe Skeleton: Mandatory atoms that are NOT blocked.
mandatory_selection(Atom) :-
    is_mandatory(Atom),
    !blocked_by_context(Atom).

# --- 2. EXCLUSION (The Firewall) ---

# Explicit prohibitions (e.g., safety rules)
prohibited(Atom) :-
    atom_tag(Atom, /mode, /active),
    atom_tag(Atom, /tag, /dream_only).
    
# Dependency-based prohibition
prohibited(Atom) :-
    atom_requires(Atom, Dep),
    prohibited(Dep).

# Conflict-based suppression
# If A and B conflict, and A is mandatory, prohibited B.
prohibited(B) :-
    atom_conflicts(A, B),
    mandatory_selection(A).
    
# --- 3. FLESH (Probabilistic Selection) ---

# Candidates from Vector Search
# Must match context, not be prohibited, and score high enough.
candidate_selection(Atom, Score) :-
    vector_hit(Atom, Score),
    !blocked_by_context(Atom),
    !prohibited(Atom).

# --- 4. CONFLICT RESOLUTION (Score-Based) ---

# Conflict: A beats B if they conflict and A has higher score.
# If scores equal, break tie using atom ID (lexicographical).
beats(A, B) :-
    atom_conflicts(A, B),
    candidate_selection(A, ScoreA),
    candidate_selection(B, ScoreB),
    ScoreA > ScoreB.

beats(A, B) :-
    atom_conflicts(A, B),
    candidate_selection(A, Score),
    candidate_selection(B, Score),
    A < B. # Lexicographical tie-breaker

# Atom is suppressed if something beats it.
suppressed(Atom) :- beats(_, Atom).

# --- 5. DEPENDENCY RESOLUTION (Recursive) ---

# Tentative Selection: Mandatory OR Candidate (if not suppressed)
tentative(Atom) :- mandatory_selection(Atom).
tentative(Atom) :- candidate_selection(Atom, _), !suppressed(Atom).

# Recursive dependency inclusion: If A is selected, Dep must be selected.
# This expands the set to include dependencies.
# Note: This might pull in atoms that were NOT in candidates.
# We must ensure pulled-in deps are not prohibited.
tentative(Dep) :-
    tentative(Atom),
    atom_requires(Atom, Dep),
    !prohibited(Dep).

# Missing Dependency Check:
# An atom has a missing dependency if it requires Dep, 
# but Dep is NOT in the tentative set (perhaps prohibited or filtered).
missing_dep(Atom) :-
    tentative(Atom),
    atom_requires(Atom, Dep),
    !tentative(Dep).

# Iterate validity: An atom is invalid if it has a missing dep.
# This handles chains: A->B->C. If C missing, B invalid, then A invalid.
invalid(Atom) :- missing_dep(Atom).

# A parent is invalid if it requires an invalid child.
invalid(Atom) :-
    tentative(Atom),
    atom_requires(Atom, Dep),
    invalid(Dep).

# --- 6. FINAL OUTPUT ---

# Valid Selection: Tentative AND NOT Invalid
final_valid(Atom) :-
    tentative(Atom),
    !invalid(Atom).

# Report selected atoms for Go Assembly
# selected_result(Atom, Priority, Source)
selected_result(Atom, Prio, /skeleton) :-
    final_valid(Atom),
    atom_priority(Atom, Prio),
    mandatory_selection(Atom).

selected_result(Atom, Prio, /flesh) :-
    final_valid(Atom),
    atom_priority(Atom, Prio),
    !mandatory_selection(Atom).


# User Policy Overrides
# User Policy Overrides
# Define project-specific rules here.
# These can extend or override core behavior.

# Example: Allow deleting .tmp files even if modified
# permitted(Action) :- 
#     action_type(Action, /delete_file),
#     target_path(Action, Path),
#     fn:string_suffix(Path, ".tmp").


# Appended Policy
# Reviewer Shard Policy - Code Review & Security Logic
# Loaded by ReviewerShard kernel alongside base policy.gl
# Part of Cortex 1.5.0 Architecture

# =============================================================================
# SECTION 1: REVIEWER TASK CLASSIFICATION
# =============================================================================

Decl reviewer_task(ID, Action, Files, Timestamp).

reviewer_action(/review) :-
    reviewer_task(_, /review, _, _).

reviewer_action(/security_scan) :-
    reviewer_task(_, /security_scan, _, _).

reviewer_action(/style_check) :-
    reviewer_task(_, /style_check, _, _).

reviewer_action(/complexity) :-
    reviewer_task(_, /complexity, _, _).

# =============================================================================
# SECTION 2: FINDING SEVERITY CLASSIFICATION
# =============================================================================
# NOTE: review_finding/6 is declared in schemas.mg

# Critical severity patterns
is_critical_finding(Finding) :-
    review_finding(Finding, _, _, /critical, _, _).

is_critical_finding(Finding) :-
    review_finding(Finding, _, _, _, /security, _).

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "sql injection").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "command injection").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "xss").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "hardcoded secret").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "path traversal").

# Error severity
is_error_finding(Finding) :-
    review_finding(Finding, _, _, /error, _, _).

# Warning severity
is_warning_finding(Finding) :-
    review_finding(Finding, _, _, /warning, _, _).

# =============================================================================
# SECTION 3: COMMIT BLOCKING
# =============================================================================

# Block commit on critical findings
block_commit("critical_security_finding") :-
    is_critical_finding(_).

# Block commit on high error count
block_commit("too_many_errors") :-
    finding_count(/error, N),
    N > 10.

# Block commit on security issues
block_commit("security_vulnerabilities") :-
    review_finding(_, _, _, _, /security, _).

# =============================================================================
# SECTION 4: REVIEW PRIORITIZATION
# =============================================================================
# Uses churn_rate from schemas.gl

Decl file_contains(FilePath, Pattern).

# High priority files (recently modified, high churn)
# Note: Rate is integer (churn count), not float
high_priority_review(File) :-
    modified(File),
    churn_rate(File, Rate),
    Rate > 3.

high_priority_review(File) :-
    modified(File),
    file_has_security_sensitive(File).

# Security-sensitive markers
file_has_security_sensitive(File) :-
    file_contains(File, "password").

file_has_security_sensitive(File) :-
    file_contains(File, "api_key").

file_has_security_sensitive(File) :-
    file_contains(File, "secret").

file_has_security_sensitive(File) :-
    file_contains(File, "credential").

file_has_security_sensitive(File) :-
    file_contains(File, "token").

file_has_security_sensitive(File) :-
    file_contains(File, "private_key").

# =============================================================================
# SECTION 5: SECURITY RULE DEFINITIONS
# =============================================================================

Decl security_rule(RuleID, Severity, Pattern, Message).

# SQL Injection
security_rule("SEC001", /critical, "execute.*concat", "SQL injection risk").
security_rule("SEC001", /critical, "raw.*sql.*concat", "SQL injection via raw query").

# Command Injection
security_rule("SEC002", /critical, "exec.Command.*concat", "Command injection risk").
security_rule("SEC002", /critical, "os.system.*concat", "Command injection via os.system").

# Hardcoded Secrets
security_rule("SEC003", /critical, "password.*=.*literal", "Hardcoded password").
security_rule("SEC003", /critical, "api_key.*=.*literal", "Hardcoded API key").

# XSS
security_rule("SEC004", /error, "innerHTML.*=", "XSS via innerHTML").
security_rule("SEC004", /error, "document.write", "XSS via document.write").

# Weak Crypto
security_rule("SEC006", /warning, "md5|sha1", "Weak cryptographic algorithm").

# =============================================================================
# SECTION 6: COMPLEXITY THRESHOLDS
# =============================================================================

Decl code_metrics(TotalLines, CodeLines, CyclomaticAvg, FunctionCount).
Decl cyclomatic_complexity(File, Function, Complexity).
Decl nesting_depth(File, Function, Depth).

# High complexity warning
complexity_warning(File, Function) :-
    cyclomatic_complexity(File, Function, C),
    C > 15.

# Deep nesting warning
nesting_warning(File, Function) :-
    nesting_depth(File, Function, D),
    D > 5.

# Long file warning
long_file_warning(File) :-
    file_line_count(File, Lines),
    Lines > 500.

# =============================================================================
# SECTION 7: AUTOPOIESIS - LEARNING FROM REVIEWS
# =============================================================================

Decl pattern_count(Pattern, Count).
Decl approval_count(Pattern, Count).
Decl review_approved(ReviewID, Pattern).

# Track patterns that get flagged repeatedly
recurring_issue_pattern(Pattern, Category) :-
    review_finding(_, _, _, _, Category, Pattern),
    pattern_count(Pattern, N),
    N >= 3.

# Learn project-specific anti-patterns
# Note: Category is implicitly tracked via recurring_issue_pattern
promote_to_long_term(/anti_pattern, Pattern) :-
    recurring_issue_pattern(Pattern, _).

# Track patterns that pass review
approved_pattern(Pattern) :-
    review_approved(_, Pattern),
    approval_count(Pattern, N),
    N >= 3.

# Promote approved styles
promote_to_long_term(/approved_style, Pattern) :-
    approved_pattern(Pattern).

# =============================================================================
# SECTION 8: REVIEW STATUS
# =============================================================================

Decl review_complete(Files, Severity).
Decl security_issue(File, Line, RuleID, Message).

# Helper for safe negation - true if any block_commit exists
has_block_commit() :-
    block_commit(_).

# Overall review status
review_passed(Files) :-
    review_complete(Files, /clean).

review_passed(Files) :-
    review_complete(Files, /info).

review_passed(Files) :-
    review_complete(Files, /warning),
    !has_block_commit().

review_failed(Files) :-
    review_complete(Files, /error).

review_failed(Files) :-
    review_complete(Files, /critical).

review_blocked(Files) :-
    review_complete(Files, _),
    has_block_commit().

# =============================================================================
# SECTION 9: STYLE RULES
# =============================================================================

Decl style_violation(File, Line, Rule, Message).

# Common style rules
style_rule("STY001", "line_length", 120).
style_rule("STY002", "trailing_whitespace", 0).
style_rule("STY003", "todo_without_issue", "TODO|FIXME").
style_rule("STY005", "max_nesting", 5).

# Style violation from rule
has_style_violation(File) :-
    style_violation(File, _, _, _).

# =============================================================================
# SECTION 10: FINDING FILTERING & SUPPRESSION (Smart Rules)
# =============================================================================

# NOTE: raw_finding, active_finding declared in schemas.mg
Decl suppressed_finding(File, Line, RuleID, Reason).
Decl is_suppressed(File, Line, RuleID).

# Helper: Projection to ignore Reason for safe negation
is_suppressed(File, Line, RuleID) :-
    suppressed_finding(File, Line, RuleID, _).

# Finding is active if not explicitly suppressed
active_finding(File, Line, Severity, Category, RuleID, Message) :-
    raw_finding(File, Line, Severity, Category, RuleID, Message),
    !is_suppressed(File, Line, RuleID).

# --- Suppression Rules ---

# Suppress TODOs (STY003) in test files
suppressed_finding(File, Line, "STY003", "todo_allowed_in_tests") :-
    raw_finding(File, Line, _, _, "STY003", _),
    file_topology(File, _, _, _, /true).

# Suppress Magic Numbers (STY004) in test files
suppressed_finding(File, Line, "STY004", "magic_numbers_allowed_in_tests") :-
    raw_finding(File, Line, _, _, "STY004", _),
    file_topology(File, _, _, _, /true).

# Suppress Complexity Warnings in test files
suppressed_finding(File, Line, "COMPLEXITY", "complexity_allowed_in_tests") :-
    raw_finding(File, Line, _, /maintainability, "COMPLEXITY", _),
    file_topology(File, _, _, _, /true).

# Suppress Long File Warnings in test files
suppressed_finding(File, Line, "LONG_FILE", "long_files_allowed_in_tests") :-
    raw_finding(File, Line, _, /maintainability, "LONG_FILE", _),
    file_topology(File, _, _, _, /true).

# Suppress Hardcoded Secrets (SEC003) in test files (usually mocks keys)
suppressed_finding(File, Line, "SEC003", "secrets_allowed_in_tests") :-
    raw_finding(File, Line, _, /security, "SEC003", _),
    file_topology(File, _, _, _, /true).

# Suppress Generated Code (common pattern)
suppressed_finding(File, Line, RuleID, "generated_code") :-
    raw_finding(File, Line, _, _, RuleID, _),
    file_contains(File, "Code generated by").

# =============================================================================
# SECTION 11: REVIEWER FEEDBACK LOOP (Self-Correction)
# =============================================================================
# These rules enable the reviewer to learn from mistakes and self-correct.

# Helper: Check if a review has any rejections
Decl has_rejections(ReviewID).
has_rejections(ReviewID) :-
    user_rejected_finding(ReviewID, _, _, _, _).

# Helper: Count rejections for a review (aggregation)
# Note: Renamed from rejection_count to avoid conflict with schemas.mg's rejection_count(Pattern, Count)
Decl review_rejection_count(ReviewID, Count).

# Review is suspect if user rejected multiple findings
review_suspect(ReviewID, "multiple_rejections") :-
    user_rejected_finding(ReviewID, File1, Line1, _, _),
    user_rejected_finding(ReviewID, File2, Line2, _, _),
    File1 != File2.

review_suspect(ReviewID, "multiple_rejections") :-
    user_rejected_finding(ReviewID, File, Line1, _, _),
    user_rejected_finding(ReviewID, File, Line2, _, _),
    Line1 != Line2.

# Review is suspect if it flagged a symbol that was verified to exist
review_suspect(ReviewID, "flagged_existing_symbol") :-
    review_finding(ReviewID, File, Line, _, _, Message),
    symbol_verified_exists(Symbol, File, _),
    :string:contains(Message, "undefined").

# Review is suspect if >50% findings were rejected
review_suspect(ReviewID, "high_rejection_rate") :-
    review_accuracy(ReviewID, Total, _, Rejected, _),
    Total > 2,
    DoubleRejected = fn:mult(Rejected, 2),
    DoubleRejected > Total.

# Trigger validation for suspect reviews
reviewer_needs_validation(ReviewID) :-
    review_suspect(ReviewID, _).

# Trigger validation for reviews with "undefined" findings (common false positive)
reviewer_needs_validation(ReviewID) :-
    review_finding(ReviewID, _, _, /error, /bug, Message),
    :string:contains(Message, "undefined").

# Trigger validation for reviews with "not found" findings
reviewer_needs_validation(ReviewID) :-
    review_finding(ReviewID, _, _, /error, /bug, Message),
    :string:contains(Message, "not found").

# --- False Positive Learning ---

# Suppress findings that match learned false positive patterns
# Note: Confidence is integer 0-100, not float 0.0-1.0
suppressed_finding(File, Line, RuleID, "learned_false_positive") :-
    raw_finding(File, Line, _, Category, RuleID, Message),
    false_positive_pattern(Pattern, Category, Occurrences, Confidence),
    Occurrences > 2,
    Confidence > 70,
    :string:contains(Message, Pattern).

# --- Self-Correction Signals ---

# Signal to main agent: recent review may be inaccurate
Decl recent_review_unreliable().
recent_review_unreliable() :-
    review_suspect(_, _).


# Learned Rules (Autopoiesis Layer - Stratified Trust)
# Learned Taxonomy Rules (Autopoiesis)
# This file is automatically appended to by the system when it learns new synonyms.

# User Learned Rules

# Autopoiesis-learned rule (added 2025-12-09 15:38:31)
permitted(Action) :- Action = "system_start".

# Autopoiesis-learned rule (added 2025-12-10 10:35:56)
system_shard_state(/boot,/initializing).


# Autopoiesis-learned rule (added 2025-12-10 11:45:05)
entry_point(/system_start).

