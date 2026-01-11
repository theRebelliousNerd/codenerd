# Coder Policy - Language Detection & Conventions
# Version: 1.0.0
# Extracted from coder.mg Section 3

# =============================================================================
# SECTION 3: LANGUAGE DETECTION & CONVENTIONS
# =============================================================================
# Detect language and apply language-specific rules.

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
