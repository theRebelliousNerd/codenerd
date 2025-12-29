# Coder Shard - Classification Logic
# Extracted from coder.mg Section 2
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
task_complexity(/simple) :-
    coder_task(ID, _, Target, _),
    !task_has_multiple_targets(ID),
    !task_is_architectural(ID).

# Complex task: multiple files or architectural change
task_complexity(/complex) :-
    coder_task(ID, _, _, _),
    task_has_multiple_targets(ID).

task_complexity(/complex) :-
    coder_task(ID, _, _, _),
    task_is_architectural(ID).

# Critical task: affects core interfaces or has many dependents
task_complexity(/critical) :-
    coder_task(_, _, Target, _),
    is_core_file(Target).

task_complexity(/critical) :-
    coder_task(_, _, Target, _),
    dependent_count(Target, N),
    N > 10.

# Helpers for safe negation
task_has_multiple_targets(ID) :-
    coder_task(ID, _, T1, _),
    coder_task(ID, _, T2, _),
    T1 != T2.

task_is_architectural(ID) :-
    coder_task(ID, _, Target, _),
    is_interface_file(Target).

task_is_architectural(ID) :-
    coder_task(ID, /refactor, _, Instruction),
    instruction_mentions_architecture(Instruction).

# Heuristics for architectural changes
instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "interface").

instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "abstraction").

instruction_mentions_architecture(Instruction) :-
    instruction_contains(Instruction, "architecture").
