# Context Compilation Rules
# Derives context relevance from kernel state instead of Go-side heuristics.
#
# This file replaces the 9-component activation scoring in internal/context/activation.go
# with deductive rules that the kernel can evaluate directly.

# NERD-EVOLVE-START: context_compilation_rules
# Hypothesis: C1+C4 (Wire kernel-derived context selection + dependency-graph relevance)
# Predicates declared in schemas_context.mg (CC.1, CC.4).
# Predicates consumed: user_intent, focus_resolution, modified, dependency_link,
#   test_state, pytest_failure, failing_test, test_file_for, context_atom.

# =============================================================================
# C1: Context Relevance Rules
# Derives context_relevant(Fact, Priority) from kernel state.
# Priorities are atom-encoded: /p100 > /p95 > /p90 > /p85 > /p80 > /p70 > /p60
# The Go layer parses /pN atoms by stripping the /p prefix: "/p100" -> 100
# =============================================================================

# Intent-driven relevance: the thing the user directly asked about (highest priority)
context_relevant(Target, /p100) :-
    user_intent(/current_intent, _, _, Target, _).

# Focus-driven relevance: files/symbols currently in focus resolution
context_relevant(File, /p95) :-
    focus_resolution(_, File, _, _).

# Test-failure relevance (Python/pytest): root-cause files when tests are failing
# test_state(/failing) is arity-1 (schemas_execution.mg:16); gates the rule.
# pytest_failure/5 binds RootFile (schemas_testing.mg:210).
context_relevant(RootFile, /p95) :-
    test_state(/failing),
    pytest_failure(_, _, RootFile, _, _).

# Test-failure relevance (Go tests): source files associated with failing tests
# failing_test/2 (schemas_reviewer.mg:123), test_file_for/2 (schemas_shards.mg:185).
context_relevant(SourceFile, /p90) :-
    test_state(/failing),
    failing_test(_, _),
    test_file_for(_, SourceFile).

# Modification relevance: recently changed files (high relevance, must be in context)
context_relevant(File, /p85) :-
    modified(File).

# Dependency relevance: direct dependencies of focal files
# Focal file (from focus_resolution) -> its direct dependencies via dependency_link
context_relevant(Dep, /p70) :-
    focus_resolution(_, File, _, _),
    dependency_link(File, Dep, _).

# Context-atom fallback: existing activation.mg derives context_atom(Fact) at Score > 30
# Feed those into our priority chain at lower priority
context_relevant(Fact, /p60) :-
    context_atom(Fact).

# =============================================================================
# Inclusion gate: should_include_context(Fact, Priority)
# Pass-through from context_relevant; carries priority for sorted selection.
# Multiple rules for the same head form a UNION (Mangle semantics).
# The Go layer queries this predicate and budget-limits selection by priority.
# Constitutional facts (permitted, dangerous_action, etc.) bypass this gate
# entirely via getCoreFacts() in compressor.go.
# =============================================================================

should_include_context(Fact, Priority) :-
    context_relevant(Fact, Priority).

# =============================================================================
# C4: Dependency-Graph-Driven Reachability Rules
# Derives context_reachable(File, HopLevel) and context_file_priority(File, Priority)
# via bounded hop traversal from focal files through dependency_link.
#
# Stratification: context_reachable uses bounded hop atoms (/hop0 -> /hop1 -> /hop2).
# Each level derives from the previous via positive atoms only. No negation. No cycles.
# =============================================================================

# Hop 0: focal files from focus_resolution (current working files)
context_reachable(File, /hop0) :-
    focus_resolution(_, File, _, _).

# Hop 0: modified files are also focal
context_reachable(File, /hop0) :-
    modified(File).

# Hop 1: direct dependencies of focal files (what focal files import)
context_reachable(Dep, /hop1) :-
    context_reachable(File, /hop0),
    dependency_link(File, Dep, _).

# Hop 1: reverse dependencies (files that import focal files)
context_reachable(Importer, /hop1) :-
    context_reachable(File, /hop0),
    dependency_link(Importer, File, _).

# Hop 2: dependencies of hop-1 files (2-hop reach)
context_reachable(Dep, /hop2) :-
    context_reachable(Mid, /hop1),
    dependency_link(Mid, Dep, _).

# Priority by hop distance
context_file_priority(File, /p100) :- context_reachable(File, /hop0).
context_file_priority(File, /p80)  :- context_reachable(File, /hop1).
context_file_priority(File, /p60)  :- context_reachable(File, /hop2).

# Test-failure override (pytest): root-cause files always high priority
context_file_priority(RootFile, /p95) :-
    test_state(/failing),
    pytest_failure(_, _, RootFile, _, _).

# Test-failure dependency override (pytest)
context_file_priority(Dep, /p85) :-
    test_state(/failing),
    pytest_failure(_, _, RootFile, _, _),
    dependency_link(RootFile, Dep, _).

# Test-failure override (Go tests)
context_file_priority(SourceFile, /p90) :-
    test_state(/failing),
    failing_test(_, _),
    test_file_for(_, SourceFile).

# Wire C4 file priorities into C1's context_relevant chain
context_relevant(File, Priority) :-
    context_file_priority(File, Priority).

# =============================================================================
# C3: Observation Masking Rules
# Replaces LLM-based summarization with kernel-derived masking decisions.
# Old/ancient turns have their observation content masked; reasoning is always preserved.
# Predicates declared in schemas_context.mg (CC.3).
# =============================================================================

# Mask old turns: these are stale enough that surface text is noise
should_mask_observation(TurnID) :-
    turn_age_category(TurnID, /old).

# Mask ancient turns: definitely stale
should_mask_observation(TurnID) :-
    turn_age_category(TurnID, /ancient).

# Preserve reasoning chain invariant: ALL turns keep their reasoning/intent/action atoms
# This is a safety net — the Go layer uses this to verify no reasoning is ever dropped.
should_preserve_reasoning(TurnID) :-
    turn_age_category(TurnID, _).

# NERD-EVOLVE-END: context_compilation_rules
