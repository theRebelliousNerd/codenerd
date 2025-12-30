# Coder Shard Policy - Classification
# Description: Classify coding tasks to determine strategy and detect language.

# =============================================================================
# SECTION 2: TASK CLASSIFICATION
# =============================================================================

# -----------------------------------------------------------------------------
# 2.1 Primary Strategy Selection
# -----------------------------------------------------------------------------

coder_strategy(/generate) :-
    coder_task(_, /create, _, _).

coder_strategy(/modify) :-
    coder_task(_, /modify, _, _).

coder_strategy(/refactor) :-
    coder_task(_, /refactor, _, _).

coder_strategy(/fix) :-
    coder_task(_, /fix, _, _).

coder_strategy(/integrate) :-
    coder_task(_, /integrate, _, _).

coder_strategy(/document) :-
    coder_task(_, /document, _, _).

# -----------------------------------------------------------------------------
# 2.2 Task Complexity Classification
# -----------------------------------------------------------------------------

# Simple task: single file, clear instruction
coder_task_complexity(/simple) :-
    coder_task(ID, _, Target, _),
    !task_has_multiple_targets(ID),
    !task_is_architectural(ID).

# Complex task: multiple files or architectural change
coder_task_complexity(/complex) :-
    coder_task(ID, _, _, _),
    task_has_multiple_targets(ID).

coder_task_complexity(/complex) :-
    coder_task(ID, _, _, _),
    task_is_architectural(ID).

# Critical task: affects core interfaces or has many dependents
coder_task_complexity(/critical) :-
    coder_task(_, _, Target, _),
    is_core_file(Target).

coder_task_complexity(/critical) :-
    coder_task(_, _, Target, _),
    dependent_count(Target, N),
    N > 10.

# Helpers for safe negation
# Note: These are local helpers, not exported in schema
Decl task_has_multiple_targets(ID).
task_has_multiple_targets(ID) :-
    coder_task(ID, _, T1, _),
    coder_task(ID, _, T2, _),
    T1 != T2.

Decl task_is_architectural(ID).
task_is_architectural(ID) :-
    coder_task(ID, _, Target, _),
    is_interface_file(Target).

task_is_architectural(ID) :-
    coder_task(ID, /refactor, _, Instruction),
    instruction_mentions_architecture(Instruction).

# Heuristics for architectural changes
Decl instruction_mentions_architecture(Instruction).
instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "interface").

instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "abstraction").

instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "architecture").

# =============================================================================
# SECTION 3: LANGUAGE DETECTION & CONVENTIONS
# =============================================================================

# -----------------------------------------------------------------------------
# 3.1 Language Detection
# -----------------------------------------------------------------------------

detected_language(File, /go) :-
    file_extension(File, ".go").

detected_language(File, /python) :-
    file_extension(File, ".py").

detected_language(File, /typescript) :-
    file_extension(File, ".ts").

detected_language(File, /typescript) :-
    file_extension(File, ".tsx").

detected_language(File, /javascript) :-
    file_extension(File, ".js").

detected_language(File, /javascript) :-
    file_extension(File, ".jsx").

detected_language(File, /rust) :-
    file_extension(File, ".rs").

detected_language(File, /java) :-
    file_extension(File, ".java").

detected_language(File, /csharp) :-
    file_extension(File, ".cs").

detected_language(File, /ruby) :-
    file_extension(File, ".rb").

detected_language(File, /php) :-
    file_extension(File, ".php").

detected_language(File, /cpp) :-
    file_extension(File, ".cpp").

detected_language(File, /cpp) :-
    file_extension(File, ".cc").

detected_language(File, /c) :-
    file_extension(File, ".c").

detected_language(File, /kotlin) :-
    file_extension(File, ".kt").

detected_language(File, /swift) :-
    file_extension(File, ".swift").

detected_language(File, /mangle) :-
    file_extension(File, ".mg").

detected_language(File, /mangle) :-
    file_extension(File, ".gl").

detected_language(File, /sql) :-
    file_extension(File, ".sql").

detected_language(File, /yaml) :-
    file_extension(File, ".yaml").

detected_language(File, /yaml) :-
    file_extension(File, ".yml").

detected_language(File, /json) :-
    file_extension(File, ".json").

detected_language(File, /markdown) :-
    file_extension(File, ".md").

detected_language(File, /shell) :-
    file_extension(File, ".sh").

detected_language(File, /powershell) :-
    file_extension(File, ".ps1").

# -----------------------------------------------------------------------------
# 3.2 Language-Specific Conventions
# -----------------------------------------------------------------------------

# Go conventions
language_convention(/go, /error_handling, "return errors, don't panic").
language_convention(/go, /naming, "camelCase for private, PascalCase for exported").
language_convention(/go, /interfaces, "accept interfaces, return concrete types").
language_convention(/go, /context, "first parameter should be ctx context.Context").
language_convention(/go, /defer, "use defer for cleanup").
language_convention(/go, /channels, "close channels from sender side").

# Python conventions
language_convention(/python, /naming, "snake_case for functions and variables").
language_convention(/python, /docstrings, "use docstrings for public functions").
language_convention(/python, /typing, "use type hints for function signatures").
language_convention(/python, /exceptions, "use specific exception types").

# TypeScript conventions
language_convention(/typescript, /typing, "prefer explicit types over any").
language_convention(/typescript, /null, "use strict null checks").
language_convention(/typescript, /interfaces, "prefer interfaces over type aliases for objects").

# Rust conventions
language_convention(/rust, /errors, "use Result<T, E> for fallible operations").
language_convention(/rust, /ownership, "prefer borrowing over cloning").
language_convention(/rust, /lifetimes, "explicit lifetimes only when necessary").

# Java conventions
language_convention(/java, /naming, "PascalCase for classes, camelCase for methods").
language_convention(/java, /exceptions, "use checked exceptions for recoverable conditions").
language_convention(/java, /interfaces, "program to interfaces").

# -----------------------------------------------------------------------------
# 3.3 Convention Application
# -----------------------------------------------------------------------------

# Conventions to apply for current task
apply_convention(Convention, Rule) :-
    coder_task(_, _, Target, _),
    detected_language(Target, Lang),
    language_convention(Lang, Convention, Rule).

# Language requires specific patterns
requires_error_handling(Target) :-
    detected_language(Target, /go).

requires_error_handling(Target) :-
    detected_language(Target, /rust).

requires_type_annotations(Target) :-
    detected_language(Target, /typescript).

requires_type_annotations(Target) :-
    detected_language(Target, /python).
