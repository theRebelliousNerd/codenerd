# Coder Policy - Quality Rules
# Version: 1.0.0
# Extracted from coder.mg Section 10

# =============================================================================
# SECTION 10: CODE QUALITY RULES
# =============================================================================
# Rules for maintaining code quality.

# -----------------------------------------------------------------------------
# 10.1 Go-Specific Quality Rules
# -----------------------------------------------------------------------------

# Go file needs error handling check
go_needs_error_check(File) :-
    pending_edit(File, _),
    !edit_handles_errors(File),
    detected_language(File, /go),
    edit_contains_operation(File, /return).

# Go file needs context parameter
go_needs_context(File) :-
    pending_edit(File, _),
    !edit_has_context(File),
    detected_language(File, /go),
    edit_is_public_function(File),
    edit_does_io(File).

# Go file leaks goroutine
go_goroutine_leak_risk(File) :-
    pending_edit(File, _),
    !edit_has_waitgroup(File),
    !edit_has_context_cancel(File),
    detected_language(File, /go),
    edit_spawns_goroutine(File).

# Go interface quality
go_interface_too_large(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_defines_interface(File, _, MethodCount),
    MethodCount > 5.

# Helpers (populated by Go runtime analysis)
edit_handles_errors(File) :- edit_analysis(File, /handles_errors).
edit_has_context(File) :- edit_analysis(File, /has_context).
edit_has_waitgroup(File) :- edit_analysis(File, /has_waitgroup).
edit_has_context_cancel(File) :- edit_analysis(File, /has_context_cancel).
edit_spawns_goroutine(File) :- edit_analysis(File, /spawns_goroutine).
edit_is_public_function(File) :- edit_analysis(File, /public_function).
edit_does_io(File) :- edit_analysis(File, /does_io).
edit_defines_interface(File, Name, Count) :- interface_definition(File, Name, Count).
edit_contains_operation(File, Op) :- edit_operation(File, Op).

# -----------------------------------------------------------------------------
# 10.2 General Quality Rules
# -----------------------------------------------------------------------------

# Function too long (> 100 lines)
function_too_long(File, FuncName) :-
    function_metrics(File, FuncName, Lines, _),
    Lines > 100.

# Cyclomatic complexity too high (> 15)
complexity_too_high(File, FuncName) :-
    function_metrics(File, FuncName, _, Complexity),
    Complexity > 15.

# Too many parameters (> 5)
too_many_params(File, FuncName) :-
    function_params(File, FuncName, ParamCount),
    ParamCount > 5.

# Deep nesting (> 4 levels)
deep_nesting(File, FuncName) :-
    function_nesting(File, FuncName, Depth),
    Depth > 4.

# -----------------------------------------------------------------------------
# 10.3 Quality Recommendations
# -----------------------------------------------------------------------------

# Recommend extraction for long functions
recommend_extraction(File, FuncName) :-
    function_too_long(File, FuncName).

# Recommend simplification for complex functions
recommend_simplify(File, FuncName) :-
    complexity_too_high(File, FuncName).

# Recommend parameter object for many params
recommend_param_object(File, FuncName) :-
    too_many_params(File, FuncName).
