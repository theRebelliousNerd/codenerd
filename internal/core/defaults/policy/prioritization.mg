# Hypothesis Prioritization
# Section 50 of Cortex Executive Policy


# Base Type Priorities
# Higher numbers = higher priority (examined first)

type_priority(/sql_injection, 95).
type_priority(/command_injection, 95).
type_priority(/path_traversal, 90).
type_priority(/unsafe_deref, 85).
type_priority(/unchecked_error, 75).
type_priority(/race_condition, 70).
type_priority(/resource_leak, 65).

# File-Based Priority Boosts

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

# Final Priority Calculation

# Helper: file has any boost
has_priority_boost(File) :-
    priority_boost(File, _).

# With boost: BasePriority + Boost
# NOTE: Uses transform pipeline for arithmetic per Mangle spec
prioritized_hypothesis(Type, File, Line, Var, Priority) :-
    active_hypothesis(Type, File, Line, Var),
    type_priority(Type, BasePriority),
    priority_boost(File, Boost)
    |> let Priority = fn:plus(BasePriority, Boost).

# Without boost: just use base priority
prioritized_hypothesis(Type, File, Line, Var, Priority) :-
    active_hypothesis(Type, File, Line, Var),
    type_priority(Type, Priority),
    !has_priority_boost(File).
