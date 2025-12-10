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

# =============================================================================
# SECTION 48: CROSS-MODULE SUPPORT PREDICATES
# =============================================================================
# Predicates used by policy.mg rules or Go code across multiple modules.

# -----------------------------------------------------------------------------
# 48.1 JIT Prompt Compiler Support (policy.mg Section 41)
# -----------------------------------------------------------------------------

# effective_prompt_atom(AtomID) - Derived: atom is effective (selected and led to success)
# Used for learning signals to improve prompt compilation over time
Decl effective_prompt_atom(AtomID).

# -----------------------------------------------------------------------------
# 48.2 Nemesis / Chaos Engineering Support (nemesis.go, chaos.mg)
# -----------------------------------------------------------------------------

# system_invariant_violated(InvariantID, Timestamp) - System invariant violation detected
# InvariantID: Identifier for the invariant (/http_500_rate, /deadlock_detected, etc.)
# Timestamp: When the violation was detected
# Used by NemesisShard and Thunderdome for chaos engineering
Decl system_invariant_violated(InvariantID, Timestamp).

# patch_diff(PatchID, DiffContent) - Stores patch diffs for analysis
# PatchID: Identifier for the patch
# DiffContent: The actual diff content as a string
# Used by NemesisShard for adversarial patch analysis
Decl patch_diff(PatchID, DiffContent).

# -----------------------------------------------------------------------------
# 48.3 Verification Support (verification.go)
# -----------------------------------------------------------------------------

# verification_summary(Timestamp, Total, Confirmed, Dismissed, DurationMs)
# Timestamp: When verification completed
# Total: Total number of hypotheses verified
# Confirmed: Number confirmed by LLM
# Dismissed: Number dismissed
# DurationMs: Duration in milliseconds
# Used by ReviewerShard hypothesis verification loop
Decl verification_summary(Timestamp, Total, Confirmed, Dismissed, DurationMs).

