# Coder Policy - Context Gathering
# Version: 1.0.0
# Extracted from coder.mg Section 8

# =============================================================================
# SECTION 8: CONTEXT GATHERING INTELLIGENCE
# =============================================================================
# Rules for intelligent context gathering.

# -----------------------------------------------------------------------------
# 8.1 Context Priority
# -----------------------------------------------------------------------------

# Current target has highest priority
coder_context_priority(File, 100) :-
    coder_target(File).

# Direct dependencies have high priority
coder_context_priority(File, 80) :-
    coder_target(Target),
    dependency_link(Target, File, _).

# Files that import target have medium priority
coder_context_priority(File, 60) :-
    coder_target(Target),
    dependency_link(File, Target, _).

# Test files for target have high priority
coder_context_priority(File, 75) :-
    coder_target(Target),
    test_file_for(File, Target).

# Interface definitions have high priority
coder_context_priority(File, 70) :-
    coder_target(Target),
    same_package(File, Target),
    is_interface_file(File).

# -----------------------------------------------------------------------------
# 8.2 Context Selection
# -----------------------------------------------------------------------------

# Include file in context if priority is high enough
include_in_context(File) :-
    coder_context_priority(File, P),
    file_in_project(File),
    P >= 50.

# Include related test files
include_in_context(File) :-
    coder_target(Target),
    test_file_for(File, Target).

# Include type definitions
include_in_context(File) :-
    coder_target(Target),
    same_package(File, Target),
    type_definition_file(File).

# -----------------------------------------------------------------------------
# 8.3 Context Exclusion
# -----------------------------------------------------------------------------

# Exclude generated files from context
exclude_from_context(File) :-
    is_generated_file(File).

# Exclude vendor files
exclude_from_context(File) :-
    is_vendor_file(File).

# Exclude binary files
exclude_from_context(File) :-
    is_binary_file(File).

# Final context decision
final_context_include(File) :-
    include_in_context(File),
    !exclude_from_context(File).
