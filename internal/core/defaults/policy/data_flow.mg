# Data Flow Safety
# Section 47 of Cortex Executive Policy


# Guard Derivation - Two Patterns for Go Idioms
#
# Two lines are in the same scope when one function_scope (the world scanner's
# function boundaries) holds both. same_scope(Var, File, L1, L2) was declared and
# never asserted, so a return guard guarded nothing and every use after one was
# reported unsafe.

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
    function_scope(File, _, ScopeStart, ScopeEnd),
    Line >= ScopeStart,
    Line <= ScopeEnd,
    GuardLine >= ScopeStart,
    GuardLine <= ScopeEnd.

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
    function_scope(File, _, ScopeStart, ScopeEnd),
    Line >= ScopeStart,
    Line <= ScopeEnd,
    GuardLine >= ScopeStart,
    GuardLine <= ScopeEnd.

# Unsafe Nil Dereference Detection

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

# Error Handling Verification

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
    function_scope(File, _, ScopeStart, ScopeEnd),
    UseLine >= ScopeStart,
    UseLine <= ScopeEnd,
    GuardLine >= ScopeStart,
    GuardLine <= ScopeEnd.

# Helper: has error check at location
has_error_check_at(ErrVar, File, UseLine) :-
    is_error_checked(ErrVar, File, UseLine).

# UNCHECKED ERROR DETECTION
# Error variable assigned but not checked before use
unchecked_error(File, Func, AssignLine) :-
    assigns(ErrVar, /error, File, AssignLine),
    uses(File, Func, ErrVar, UseLine),
    !has_error_check_at(ErrVar, File, UseLine).
