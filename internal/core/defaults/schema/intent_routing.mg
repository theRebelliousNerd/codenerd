# intent_routing.mg
# =============================================================================
# LLM-FIRST INTENT ROUTING
# =============================================================================
# This schema defines how the harness routes AFTER the LLM classifies intent.
# The LLM understands what the user wants; these rules determine how to fulfill it.
#
# Philosophy: LLM describes → Logic determines
#
# The LLM outputs an Understanding with:
#   - semantic_type: what kind of request (causation, mechanism, location, etc.)
#   - action_type: what to do (investigate, implement, verify, etc.)
#   - domain: what area (testing, security, git, etc.)
#   - scope: how much (function, file, module, codebase)
#
# This schema maps those signals to:
#   - Modes (tdd, dream, debug, security_audit, etc.)
#   - Context priorities (what facts to surface)
#   - Shard affinities (which agents to invoke)
#   - Tool affinities (which tools to serve)

# =============================================================================
# SECTION 1: VALIDATION VOCABULARY
# =============================================================================
# These define valid values for LLM output fields.
# The harness validates LLM classification against these.

# Decl valid_semantic_type(Type, Description)
# Decl valid_action_type(Action, Description)
# Decl valid_domain(Domain, Description)
# Decl valid_scope_level(Level, Order)
# Decl valid_mode(Mode, Description)
# Decl valid_urgency(Urgency, Order)

# --- Semantic Types (how is the user asking?) ---
valid_semantic_type(/definition, "What is X? Explain X.").
valid_semantic_type(/causation, "Why does X happen? What causes X?").
valid_semantic_type(/mechanism, "How does X work? How do I do X?").
valid_semantic_type(/location, "Where is X? Find X.").
valid_semantic_type(/temporal, "When did X happen? History of X.").
valid_semantic_type(/attribution, "Who wrote X? Who owns X?").
valid_semantic_type(/selection, "Which X should I use? Compare X and Y.").
valid_semantic_type(/hypothetical, "What if X? Imagine X.").
valid_semantic_type(/recommendation, "Should I do X? What's best practice?").
valid_semantic_type(/existence, "Is there X? Does X exist?").
valid_semantic_type(/quantification, "How many X? How much X?").
valid_semantic_type(/state, "Is X working? Is X secure?").
valid_semantic_type(/instruction, "Always do X. Never do Y. Remember Z.").

# --- Action Types (what does the user want done?) ---
valid_action_type(/investigate, "Analyze, debug, find root cause, understand").
valid_action_type(/implement, "Create, add, write, build new functionality").
valid_action_type(/modify, "Edit, change, update existing code").
valid_action_type(/refactor, "Restructure, clean up, improve without changing behavior").
valid_action_type(/verify, "Test, check, validate, ensure correctness").
valid_action_type(/explain, "Describe, document, teach, clarify").
valid_action_type(/research, "Learn, gather info, find docs, explore").
valid_action_type(/configure, "Setup, initialize, adjust settings").
valid_action_type(/attack, "Adversarial testing, security probing, stress test").
valid_action_type(/revert, "Undo, rollback, restore previous state").
valid_action_type(/review, "Audit, critique, assess quality").
valid_action_type(/remember, "Store preference, learn pattern, save for later").
valid_action_type(/forget, "Remove preference, unlearn pattern").

# --- Domains (what area of concern?) ---
valid_domain(/testing, "Unit tests, integration tests, coverage, test fixtures").
valid_domain(/security, "Vulnerabilities, auth, encryption, input validation").
valid_domain(/performance, "Speed, memory, optimization, profiling").
valid_domain(/documentation, "Comments, READMEs, API docs, examples").
valid_domain(/architecture, "Design patterns, structure, dependencies, coupling").
valid_domain(/git, "Version control, commits, branches, history, blame").
valid_domain(/dependencies, "Packages, imports, versions, updates").
valid_domain(/configuration, "Settings, env vars, build config").
valid_domain(/error_handling, "Exceptions, panics, recovery, logging").
valid_domain(/concurrency, "Goroutines, threads, race conditions, deadlocks").
valid_domain(/general, "No specific domain, general coding task").

# --- Scope Levels (ordered from smallest to largest) ---
valid_scope_level(/line, 1).
valid_scope_level(/block, 2).
valid_scope_level(/function, 3).
valid_scope_level(/type, 4).
valid_scope_level(/file, 5).
valid_scope_level(/package, 6).
valid_scope_level(/module, 7).
valid_scope_level(/codebase, 8).

# --- Modes (what harness mode to enter?) ---
valid_mode(/normal, "Standard execution mode").
valid_mode(/tdd, "Test-driven development loop: red-green-refactor").
valid_mode(/debug, "Debugging mode with enhanced diagnostics").
valid_mode(/dream, "Hypothetical simulation, no real changes").
valid_mode(/security_audit, "Security-focused analysis mode").
valid_mode(/campaign, "Multi-phase long-running task").
valid_mode(/research, "Information gathering, no code changes").
valid_mode(/assault, "Adversarial testing campaign").

# --- Urgency Levels ---
valid_urgency(/low, 1).
valid_urgency(/normal, 2).
valid_urgency(/high, 3).
valid_urgency(/critical, 4).

# =============================================================================
# SECTION 2: MODE ROUTING
# =============================================================================
# Given semantic signals from LLM, which mode should the harness enter?

# Decl mode_from_semantic(SemanticType, Mode, Priority)
# Decl mode_from_action(ActionType, Mode, Priority)
# Decl mode_from_domain(Domain, Mode, Priority)
# Decl mode_from_signal(Signal, Mode, Priority)

# --- Semantic Type → Mode ---
mode_from_semantic(/causation, /debug, 90).
mode_from_semantic(/hypothetical, /dream, 95).
mode_from_semantic(/temporal, /normal, 70).
mode_from_semantic(/instruction, /normal, 80).

# --- Action Type → Mode ---
mode_from_action(/investigate, /debug, 85).
mode_from_action(/verify, /tdd, 90).
mode_from_action(/attack, /assault, 95).
mode_from_action(/research, /research, 90).
mode_from_action(/revert, /normal, 80).

# --- Domain → Mode ---
mode_from_domain(/testing, /tdd, 85).
mode_from_domain(/security, /security_audit, 88).

# --- Signal Flags → Mode ---
mode_from_signal(/is_hypothetical, /dream, 95).
mode_from_signal(/is_multi_step, /campaign, 80).

# =============================================================================
# SECTION 3: CONTEXT AFFINITY
# =============================================================================
# What context should be prioritized given the intent?
# Higher weight = more important to include in LLM context window.

# Decl context_affinity_semantic(SemanticType, ContextCategory, Weight)
# Decl context_affinity_action(ActionType, ContextCategory, Weight)
# Decl context_affinity_domain(Domain, ContextCategory, Weight)

# --- Semantic Type → Context ---
context_affinity_semantic(/causation, /error_logs, 100).
context_affinity_semantic(/causation, /stack_traces, 95).
context_affinity_semantic(/causation, /recent_changes, 90).
context_affinity_semantic(/causation, /test_output, 88).

context_affinity_semantic(/temporal, /git_history, 100).
context_affinity_semantic(/temporal, /commit_messages, 90).

context_affinity_semantic(/attribution, /git_blame, 100).
context_affinity_semantic(/attribution, /author_info, 85).

context_affinity_semantic(/location, /file_topology, 95).
context_affinity_semantic(/location, /symbol_index, 90).

context_affinity_semantic(/state, /current_diagnostics, 100).
context_affinity_semantic(/state, /test_results, 95).

# --- Action Type → Context ---
context_affinity_action(/investigate, /error_logs, 95).
context_affinity_action(/investigate, /stack_traces, 90).
context_affinity_action(/investigate, /related_code, 85).

context_affinity_action(/implement, /existing_patterns, 90).
context_affinity_action(/implement, /similar_code, 85).
context_affinity_action(/implement, /api_signatures, 80).

context_affinity_action(/modify, /target_source, 100).
context_affinity_action(/modify, /callers, 90).
context_affinity_action(/modify, /callees, 85).

context_affinity_action(/refactor, /target_source, 100).
context_affinity_action(/refactor, /test_coverage, 90).
context_affinity_action(/refactor, /dependencies, 85).

context_affinity_action(/verify, /test_output, 100).
context_affinity_action(/verify, /coverage_data, 90).
context_affinity_action(/verify, /test_fixtures, 80).

context_affinity_action(/review, /target_source, 100).
context_affinity_action(/review, /style_guide, 85).
context_affinity_action(/review, /similar_code, 80).

# --- Domain → Context ---
context_affinity_domain(/testing, /test_output, 100).
context_affinity_domain(/testing, /coverage_data, 90).
context_affinity_domain(/testing, /test_fixtures, 85).
context_affinity_domain(/testing, /mocks, 80).

context_affinity_domain(/security, /vulnerability_patterns, 100).
context_affinity_domain(/security, /auth_flows, 95).
context_affinity_domain(/security, /input_validation, 90).
context_affinity_domain(/security, /dependency_audit, 85).

context_affinity_domain(/performance, /profiling_data, 100).
context_affinity_domain(/performance, /benchmarks, 90).
context_affinity_domain(/performance, /hotspots, 85).

context_affinity_domain(/git, /git_status, 100).
context_affinity_domain(/git, /git_diff, 95).
context_affinity_domain(/git, /git_history, 90).
context_affinity_domain(/git, /branch_info, 85).

context_affinity_domain(/architecture, /dependency_graph, 100).
context_affinity_domain(/architecture, /module_structure, 95).
context_affinity_domain(/architecture, /interface_definitions, 90).

context_affinity_domain(/error_handling, /error_patterns, 95).
context_affinity_domain(/error_handling, /panic_handlers, 90).
context_affinity_domain(/error_handling, /logging_config, 85).

context_affinity_domain(/concurrency, /goroutine_patterns, 95).
context_affinity_domain(/concurrency, /sync_primitives, 90).
context_affinity_domain(/concurrency, /channel_usage, 85).

# =============================================================================
# SECTION 4: SHARD AFFINITY
# =============================================================================
# Which shards are appropriate for which intents?
# Higher weight = more suitable for this intent.

# Decl shard_affinity_action(ActionType, ShardType, Weight)
# Decl shard_affinity_domain(Domain, ShardType, Weight)

# --- Action Type → Shard ---
shard_affinity_action(/investigate, /reviewer, 90).
shard_affinity_action(/investigate, /coder, 75).

shard_affinity_action(/implement, /coder, 100).

shard_affinity_action(/modify, /coder, 100).

shard_affinity_action(/refactor, /coder, 95).
shard_affinity_action(/refactor, /reviewer, 70).

shard_affinity_action(/verify, /tester, 100).
shard_affinity_action(/verify, /reviewer, 70).

shard_affinity_action(/explain, /reviewer, 85).
shard_affinity_action(/explain, /researcher, 80).

shard_affinity_action(/research, /researcher, 100).

shard_affinity_action(/attack, /nemesis, 100).
shard_affinity_action(/attack, /tester, 70).

shard_affinity_action(/review, /reviewer, 100).

shard_affinity_action(/revert, /coder, 90).

# --- Domain → Shard ---
shard_affinity_domain(/testing, /tester, 95).
shard_affinity_domain(/testing, /coder, 70).

shard_affinity_domain(/security, /reviewer, 90).
shard_affinity_domain(/security, /nemesis, 85).

shard_affinity_domain(/documentation, /researcher, 90).
shard_affinity_domain(/documentation, /coder, 70).

shard_affinity_domain(/architecture, /reviewer, 90).

shard_affinity_domain(/git, /coder, 85).

# =============================================================================
# SECTION 5: TOOL AFFINITY
# =============================================================================
# Which tools are relevant for which intents?

# Decl tool_affinity_action(ActionType, Tool, Weight)
# Decl tool_affinity_domain(Domain, Tool, Weight)

# --- Action Type → Tools ---
tool_affinity_action(/investigate, /grep, 90).
tool_affinity_action(/investigate, /ast_query, 85).
tool_affinity_action(/investigate, /read_file, 100).

tool_affinity_action(/implement, /write_file, 100).
tool_affinity_action(/implement, /edit_file, 95).
tool_affinity_action(/implement, /ast_query, 80).

tool_affinity_action(/modify, /edit_file, 100).
tool_affinity_action(/modify, /read_file, 95).
tool_affinity_action(/modify, /ast_query, 85).

tool_affinity_action(/verify, /run_tests, 100).
tool_affinity_action(/verify, /coverage, 85).
tool_affinity_action(/verify, /lint, 80).

tool_affinity_action(/research, /web_search, 95).
tool_affinity_action(/research, /read_file, 90).
tool_affinity_action(/research, /grep, 85).

tool_affinity_action(/revert, /git_restore, 100).
tool_affinity_action(/revert, /git_checkout, 90).

# --- Domain → Tools ---
tool_affinity_domain(/testing, /run_tests, 100).
tool_affinity_domain(/testing, /coverage, 90).
tool_affinity_domain(/testing, /test_debug, 85).

tool_affinity_domain(/git, /git_status, 100).
tool_affinity_domain(/git, /git_diff, 95).
tool_affinity_domain(/git, /git_log, 90).
tool_affinity_domain(/git, /git_commit, 85).
tool_affinity_domain(/git, /git_push, 80).
tool_affinity_domain(/git, /git_blame, 85).

tool_affinity_domain(/security, /security_scan, 95).
tool_affinity_domain(/security, /dependency_check, 90).

tool_affinity_domain(/performance, /profiler, 95).
tool_affinity_domain(/performance, /benchmark, 90).

# =============================================================================
# SECTION 6: ROUTING INFERENCE RULES
# =============================================================================
# Derive final routing decisions from combined signals.

# Decl best_mode(Mode, Score)
# Decl best_shard(Shard, Score)
# Decl context_category_priority(ContextCategory, Score)
# Decl tool_priority(Tool, Score)

# Aggregate mode scores from all signals
# (In practice, this would be computed in Go by querying all mode_from_* facts)

# Aggregate shard scores
# (Computed in Go from shard_affinity_* facts)

# Aggregate context priorities
# (Computed in Go from context_affinity_* facts)

# Aggregate tool priorities
# (Computed in Go from tool_affinity_* facts)

# =============================================================================
# SECTION 7: CONSTRAINT HANDLING
# =============================================================================
# User constraints that affect routing.

# Decl constraint_type(Constraint, Effect)

constraint_type(/no_changes, /read_only).
constraint_type(/preserve_tests, /must_pass_tests).
constraint_type(/preserve_behavior, /no_functional_change).
constraint_type(/dry_run, /simulation_only).
constraint_type(/confirm_first, /require_approval).
constraint_type(/quick, /fast_path).
constraint_type(/thorough, /deep_analysis).

# Constraint → Mode override
# Decl constraint_forces_mode(Constraint, Mode)
constraint_forces_mode(/no_changes, /research).
constraint_forces_mode(/dry_run, /dream).
constraint_forces_mode(/confirm_first, /normal).

# Constraint → Tool restriction
# Decl constraint_blocks_tool(Constraint, Tool)
constraint_blocks_tool(/no_changes, /write_file).
constraint_blocks_tool(/no_changes, /edit_file).
constraint_blocks_tool(/no_changes, /git_commit).
constraint_blocks_tool(/no_changes, /git_push).
