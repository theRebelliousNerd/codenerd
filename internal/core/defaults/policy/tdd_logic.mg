# TDD Loop Logic
# Implements the TDD Repair Loop (OODA Loop) decision making.
# Extracted from internal/core/tdd_loop.go

# Cortex 1.5.0 §2.2 "The Barrier"
# Blocks commits if there are any error severity diagnostics.
# The BODY matches the severity atom /error for strict type safety (/error vs
# "error"). The HEAD slot is Reason, declared /string in schemas_analysis.mg and
# carrying a free-form reason in all 15 other block_commit rules; this rule was
# the lone outlier leaking the severity atom into it. It now emits the same
# reason as the two rules with an identical body (coder_diagnostics.mg,
# coder_build.mg), so the barrier reads as one relation.
block_commit("build_errors") :-
    diagnostic(/error, _, _, _, _).
