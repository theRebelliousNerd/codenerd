# Coder Shard Policy - Quality, Autopoiesis & Patterns
# Description: Rules for maintaining code quality, learning from patterns, and specialized generation.

# =============================================================================
# SECTION 10: CODE QUALITY RULES
# =============================================================================

# -----------------------------------------------------------------------------
# 10.1 Go-Specific Quality Rules
# -----------------------------------------------------------------------------

# Go file needs error handling check
go_needs_error_check(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_contains_operation(File, /return),
    !edit_handles_errors(File).

# Go file needs context parameter
go_needs_context(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_is_public_function(File),
    edit_does_io(File),
    !edit_has_context(File).

# Go file leaks goroutine
go_goroutine_leak_risk(File) :-
    pending_edit(File, _),
    detected_language(File, /go),
    edit_spawns_goroutine(File),
    !edit_has_waitgroup(File),
    !edit_has_context_cancel(File).

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

# =============================================================================
# SECTION 11: AUTOPOIESIS - LEARNING FROM PATTERNS
# =============================================================================

# -----------------------------------------------------------------------------
# 11.1 Rejection Pattern Detection
# -----------------------------------------------------------------------------

# Track rejection patterns (style issues rejected 2+ times)
coder_rejection_pattern(Style) :-
    rejection(_, /style, Style),
    coder_rejection_count(/style, Style, N),
    N >= 2.

# Track error patterns (errors repeated 2+ times)
coder_error_pattern(ErrorType) :-
    rejection(_, /error, ErrorType),
    coder_rejection_count(/error, ErrorType, N),
    N >= 2.

# Promote style preference to long-term memory
promote_to_long_term(/style_preference, Style) :-
    coder_rejection_pattern(Style).

# Promote error avoidance to long-term memory
promote_to_long_term(/error_avoidance, ErrorType) :-
    coder_error_pattern(ErrorType).

# -----------------------------------------------------------------------------
# 11.2 Success Pattern Detection
# -----------------------------------------------------------------------------

# Track successful patterns (accepted 3+ times)
coder_success_pattern(Pattern) :-
    code_accepted(_, Pattern),
    acceptance_count(Pattern, N),
    N >= 3.

# Promote preferred patterns
promote_to_long_term(/preferred_pattern, Pattern) :-
    coder_success_pattern(Pattern).

# -----------------------------------------------------------------------------
# 11.3 Learning Signals
# -----------------------------------------------------------------------------

# Signal to avoid patterns that always fail
learning_signal(/avoid, Pattern) :-
    coder_rejection_count(_, Pattern, N),
    N >= 3,
    acceptance_count(Pattern, M),
    M < 1.

# Signal to prefer patterns that always succeed
learning_signal(/prefer, Pattern) :-
    acceptance_count(Pattern, N),
    N >= 5,
    coder_rejection_count(_, Pattern, M),
    M < 1.

# Helper for rejection check
has_rejection(Pattern) :-
    coder_rejection_count(_, Pattern, _).

# =============================================================================
# SECTION 14: SPECIALIZED CODE PATTERNS
# =============================================================================

# -----------------------------------------------------------------------------
# 14.1 API Endpoint Generation
# -----------------------------------------------------------------------------

# API endpoint needs specific patterns
api_endpoint_pattern(File) :-
    coder_task(_, /create, File, Instruction),
    instruction_contains(Instruction, "endpoint").

api_endpoint_pattern(File) :-
    coder_task(_, /create, File, Instruction),
    instruction_contains(Instruction, "handler").

# API requires validation
api_needs_validation(File) :-
    api_endpoint_pattern(File).

# API requires error responses
api_needs_error_handling(File) :-
    api_endpoint_pattern(File).

# -----------------------------------------------------------------------------
# 14.2 Database Operation Patterns
# -----------------------------------------------------------------------------

# Database operation pattern
database_operation_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "database").

database_operation_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "query").

# Database needs transaction handling
db_needs_transaction(File) :-
    database_operation_pattern(File),
    instruction_contains_write(File).

# Database needs connection pooling awareness
db_needs_pooling(File) :-
    database_operation_pattern(File).

# -----------------------------------------------------------------------------
# 14.3 Concurrency Patterns
# -----------------------------------------------------------------------------

# Concurrency pattern detected
concurrency_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "concurrent").

concurrency_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "parallel").

concurrency_pattern(File) :-
    coder_task(_, _, File, Instruction),
    instruction_contains(Instruction, "goroutine").

# Concurrency needs synchronization
needs_synchronization(File) :-
    concurrency_pattern(File).

# Concurrency needs context propagation
needs_context_propagation(File) :-
    concurrency_pattern(File).
