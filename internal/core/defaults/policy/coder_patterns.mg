# Coder Policy - Specialized Patterns
# Version: 1.0.0
# Extracted from coder.mg Section 14

# =============================================================================
# SECTION 14: SPECIALIZED CODE PATTERNS
# =============================================================================
# Rules for specific code generation patterns.

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
