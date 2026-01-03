# TDD Loop Logic
# Implements the TDD Repair Loop (OODA Loop) decision making.
# Extracted from internal/core/tdd_loop.go

# Cortex 1.5.0 §2.2 "The Barrier"
# Blocks commits if there are any error severity diagnostics.
block_commit(/error) :-
    diagnostic(/error, _, _, _, _).
