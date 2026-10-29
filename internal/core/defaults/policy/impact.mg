# Impact Analysis & Refactoring Guard
# Section 5 of Cortex Executive Policy


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

# Section 48: Impact Analysis Rules (ReviewerShard Beyond-SOTA)

# Direct Impact Detection

# Direct callers of modified functions (Distance 1)
impact_caller(TargetFunc, CallerFunc) :-
    modified_function(TargetFunc, _),
    code_calls(CallerFunc, TargetFunc).

# Interface implementations affected by interface changes
impact_implementer(ImplFile, Struct) :-
    modified_interface(Interface, _),
    code_implements(Struct, Interface),
    code_defines(ImplFile, Struct, /struct, _, _).

# Bounded Transitive Impact (Max Depth 3)

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

# Context File Selection

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
