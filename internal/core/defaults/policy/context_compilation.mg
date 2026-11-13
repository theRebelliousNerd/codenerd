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

# Test-failure relevance (Go tests). These read the turn's own measured run
# (turn_test_failure_at / turn_test_build_failure, coder_safety.mg), the only
# producer of test failures; a repair round compiles its context while those
# rows stand. The file a test fails in, the file a build diagnostic names, and
# the source file that failing test file covers (test_file_for, the world
# scanner's pairing). Paths on both sides are workspace-canonical.
context_relevant(File, /p95) :-
    turn_test_failure_at(Turn, Pkg, Test, File, Line, Msg, Count).
context_relevant(File, /p95) :-
    turn_test_build_failure(Turn, Pkg, File, Line, Msg).
context_relevant(SourceFile, /p90) :-
    turn_test_failure_at(Turn, Pkg, Test, TestFile, Line, Msg, Count),
    test_file_for(TestFile, SourceFile).

# Session-note relevance: what the model noted for itself this session with the
# memory operation "note" (session_note/2). Below the intent and the focus it
# was noted about, above modification relevance.
context_relevant(Key, /p90) :-
    session_note(Key, _).

# Campaign danger relevance: the files a campaign plan touches that are risky to
# touch (critical safety warning, high impact, high churn, a refactor first), by
# the rank policy/intelligence.mg gives them. Rank 95 and 85 are /p95 and /p85.
context_relevant(Path, /p95) :-
    intelligence_context_rank(Path, 95).
context_relevant(Path, /p85) :-
    intelligence_context_rank(Path, 85).

# Modification relevance: recently changed files (high relevance, must be in context)
context_relevant(File, /p85) :-
    modified(File).

# Dependency relevance: direct dependencies of focal files
# Focal file (from focus_resolution) -> its direct dependencies via dependency_link
context_relevant(Dep, /p70) :-
    focus_resolution(_, File, _, _),
    import_unit(File, Unit),
    context_small_unit(Unit),
    file_unit(Dep, Unit).

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
# CC.2: Retention -- what the window always carries
# Every fact of a retained predicate is in every context block, relevant or not:
# a model that cannot see what is forbidden will propose it. This list lived in
# Go (getCoreFacts) until 2026-09-25; the compressor now asks, and keeps its
# own copy only as the floor under a policy that failed to load.
# A rule may retain more (a predicate while some state holds); nothing here may
# retain less than these five -- TestRetentionPolicy_KeepsTheConstitutionalFloor.
# =============================================================================

context_must_retain(/permitted).
context_must_retain(/dangerous_action).
context_must_retain(/human_approval_granted).
context_must_retain(/security_violation).
context_must_retain(/block_commit).

# =============================================================================
# CC.3: Entity views -- what the window shows of a selected entity
# Selection above names entities. Which facts about an entity enter the window
# is decided here, not by which facts happen to mention it in one shard: Go
# reads context_view and context_focal and fetches exactly these rows, from
# whichever shard holds or derives them.
# =============================================================================

# The entities the turn is working on.
context_focal(Target) :-
    user_intent(/current_intent, _, _, Target, _).
context_focal(File) :-
    focus_resolution(_, File, _, _).
context_focal(File) :-
    modified(File).
context_focal(File) :-
    turn_test_failure_at(Turn, Pkg, Test, File, Line, Msg, Count).
context_focal(File) :-
    turn_test_build_failure(Turn, Pkg, File, Line, Msg).

# Rank 1: what is wrong with it. Shown of every selected entity.
context_view(/turn_test_failure_at, 7, 3, 1, /any).
context_view(/turn_test_build_failure, 5, 2, 1, /any).
context_view(/diagnostic, 5, 1, 1, /any).
context_view(/intelligence_chestertons_fence, 2, 0, 1, /any).

# Rank 2: where it sits. One or two rows an entity.
context_view(/modified, 1, 0, 2, /any).
context_view(/import_cycle_member, 2, 0, 2, /any).
context_view(/intelligence_dependent_count, 2, 0, 2, /any).
context_view(/file_unit, 2, 0, 2, /any).
context_view(/test_file_for, 2, 0, 2, /any).
context_view(/test_file_for, 2, 1, 2, /any).
context_view(/session_note, 2, 0, 2, /any).
# Of a unit selected as itself: how many files it has and how many import it.
context_view(/context_unit_size, 2, 0, 2, /any).
context_view(/intelligence_unit_importers, 2, 0, 2, /any).

# Rank 3: what it reaches. Shown of the entities being worked on.
context_view(/import_unit, 2, 0, 3, /focal).

# Rank 4: its surface, the bulky view. Shown of the entities being worked on,
# after every compact view of every selected entity has had its turn.
context_view(/symbol_graph, 5, 3, 4, /focal).

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

# A neighbourhood is selected file by file only where it is small enough to be
# a selection. Measured on this repository (2026-11-13,
# TestRealWorldProfile): one modified file of a hub package selected 2,051
# files -- every importer of the package, then every file of every package
# those import -- which is 29% of the workspace named "relevant", cut to the
# budget in alphabetical order. A wide unit is selected as itself: one entity
# whose views say how large it is and how many files import it.
context_neighbour_limit(8).

context_unit_size(Unit, N) :-
    file_unit(_, Unit) |>
    do fn:group_by(Unit),
    let N = fn:count().

context_small_unit(Unit) :-
    context_unit_size(Unit, N),
    context_neighbour_limit(Limit),
    N <= Limit.

context_few_importers(Unit) :-
    intelligence_unit_importers(Unit, N),
    context_neighbour_limit(Limit),
    N <= Limit.

# Hop 1: direct dependencies of focal files (what focal files import), file by
# file when the imported unit is small...
context_reachable(Dep, /hop1) :-
    context_reachable(File, /hop0),
    import_unit(File, Unit),
    context_small_unit(Unit),
    file_unit(Dep, Unit).

# ...and as the unit when it is not.
context_wide_unit(Unit) :-
    context_reachable(File, /hop0),
    import_unit(File, Unit),
    context_unit_size(Unit, _),
    !context_small_unit(Unit).

# Hop 1: reverse dependencies (files that import focal files), when they are
# few. A widely imported file's importers are a number, shown by the
# intelligence_dependent_count view.
context_reachable(Importer, /hop1) :-
    context_reachable(File, /hop0),
    file_unit(File, Unit),
    context_few_importers(Unit),
    import_unit(Importer, Unit).

# Hop 2: dependencies of hop-1 files (2-hop reach), under the same limit.
context_reachable(Dep, /hop2) :-
    context_reachable(Mid, /hop1),
    import_unit(Mid, Unit),
    context_small_unit(Unit),
    file_unit(Dep, Unit).

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
    import_unit(RootFile, Unit),
    context_small_unit(Unit),
    file_unit(Dep, Unit).

# Wire C4 file priorities into C1's context_relevant chain
context_relevant(File, Priority) :-
    context_file_priority(File, Priority).
context_relevant(Unit, /p80) :-
    context_wide_unit(Unit).

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
