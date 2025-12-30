# MANDATE FOR AI CODING TOOLS:
# This file contains critical product requirements and architectural mandates.
# DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
# File: internal/core/defaults/state.mg
# Author: Gemini
# Date: 2025-12-08
#
# Recommended Model: 2.5 Pro
#
# Overview:
# Defines the Mangle schema and rules for the Ouroboros Loop "Transactional State Machine".
# It governs self-modifiction stability and termination.
#
# Key Features & Business Value:
# - Stability transition rules (monotonic improvement).
# - Stagnation detection (loop prevention).
#
# Architectural Context:
# - Component Type: Mangle Schema
# - Used By: OuroborosLoop (ouroboros.go) via Mangle Engine.
#
# --- END OF PRD HEADER ---

# =============================================================================
# CORE STATE DECLARATIONS (Original)
# =============================================================================
Decl state(StepID, Stability, Loc).
Decl proposed(StepID).
Decl history(StepID, Hash).
Decl error_event(Type).

# =============================================================================
# ITERATION TRACKING
# =============================================================================
Decl iteration(StepID, IterNum).
Decl max_iterations(Limit).
Decl state_at_iteration(StepID, IterNum, Stability).

# =============================================================================
# RETRY TRACKING
# =============================================================================
Decl retry_attempt(StepID, AttemptNum, Reason).
Decl max_retries(Limit).

# =============================================================================
# ERROR HISTORY
# =============================================================================
Decl error_history(StepID, ErrorType, Timestamp).

# =============================================================================
# STABILITY PENALTIES
# =============================================================================
Decl stability_penalty(StepID, Amount).
Decl cumulative_penalty(StepID, Total).
Decl base_stability(StepID, Score).
Decl effective_stability(StepID, Score).
Decl has_penalty(StepID).
Decl has_panic_penalty(StepID).
Decl has_retry_penalty(StepID).
Decl has_effective_stability(StepID).

# =============================================================================
# HOT-RELOAD TRACKING
# =============================================================================
Decl tool_hot_loaded(ToolName, Timestamp).
Decl tool_version(ToolName, Version).

# =============================================================================
# TERMINATION CONDITIONS
# =============================================================================
Decl should_halt(StepID).
Decl should_complete(StepID).
Decl max_iterations_exceeded(StepID).
Decl max_retries_exceeded(StepID).
Decl stability_degrading(StepID).
Decl converged(StepID).

# =============================================================================
# HALTING ORACLE (Original)
# =============================================================================
# Detects stagnation if two history entries refer to the same hash but have different step IDs.
# This implies the agent is circling back to a previous code state.
stagnation_detected() :-
  history(StepA, Hash),
  history(StepB, Hash),
  StepA != StepB.

# =============================================================================
# PENALTY CALCULATION RULES
# =============================================================================

# Track if step has panic penalty
has_panic_penalty(StepID) :-
    error_history(StepID, /panic, _).

# Track if step has retry penalty
has_retry_penalty(StepID) :-
    retry_attempt(StepID, N, _),
    N >= 2.

# Track if step has any penalty
has_penalty(StepID) :- has_panic_penalty(StepID).
has_penalty(StepID) :- has_retry_penalty(StepID).

# Cumulative penalty calculation (no aggregation - fixed penalty tiers)
# Both penalties: 0.3
cumulative_penalty(StepID, 0.3) :-
    has_panic_penalty(StepID),
    has_retry_penalty(StepID).

# Only panic penalty: 0.2
cumulative_penalty(StepID, 0.2) :-
    has_panic_penalty(StepID),
    !has_retry_penalty(StepID).

# Only retry penalty: 0.1
cumulative_penalty(StepID, 0.1) :-
    has_retry_penalty(StepID),
    !has_panic_penalty(StepID).

# Default zero penalty if no penalties exist
cumulative_penalty(StepID, 0.0) :-
    state(StepID, _, _),
    !has_penalty(StepID).

# =============================================================================
# EFFECTIVE STABILITY CALCULATION
# =============================================================================

# Effective stability = base stability - cumulative penalties
effective_stability(StepID, Effective) :-
    base_stability(StepID, Base),
    cumulative_penalty(StepID, Penalty),
    Effective = fn:minus(Base, Penalty).

# Helper: track which steps have effective stability computed
has_effective_stability(StepID) :- effective_stability(StepID, _).

# =============================================================================
# TRANSITION VALIDATION (Enhanced)
# =============================================================================

# A transition is valid if the proposed next state has equal or greater effective stability.
# This accounts for penalties accumulated during the loop.
valid_transition(Next) :-
    state(Curr, _, _),
    proposed(Next),
    effective_stability(Curr, CurrEff),
    effective_stability(Next, NextEff),
    NextEff >= CurrEff.

# Fallback: Original transition rule if no effective_stability facts exist
# Note: Use helper predicate has_effective_stability for safe negation
valid_transition(Next) :-
    state(Curr, CurrStability, _),
    proposed(Next),
    state(Next, NextStability, _),
    !has_effective_stability(Curr),
    NextStability >= CurrStability.

# =============================================================================
# TERMINATION CONDITION RULES
# =============================================================================

# Max iterations exceeded
max_iterations_exceeded(StepID) :-
    iteration(StepID, N),
    max_iterations(Limit),
    N >= Limit.

# Max retries exceeded
max_retries_exceeded(StepID) :-
    retry_attempt(StepID, N, _),
    max_retries(Limit),
    N >= Limit.

# Stability degradation (3 consecutive drops)
stability_degrading(StepID) :-
    state_at_iteration(StepID, N1, S1),
    state_at_iteration(StepID, N2, S2),
    state_at_iteration(StepID, N3, S3),
    N2 = fn:plus(N1, 1),
    N3 = fn:plus(N2, 1),
    S2 < S1,
    S3 < S2.

# Convergence (stability stable for 2 consecutive iterations)
converged(StepID) :-
    state_at_iteration(StepID, N1, S1),
    state_at_iteration(StepID, N2, S2),
    N2 = fn:plus(N1, 1),
    S1 = S2,
    N2 >= 2.

# =============================================================================
# SHOULD_HALT DERIVATION
# =============================================================================

# Halt on max iterations
should_halt(StepID) :-
    iteration(StepID, _),
    max_iterations_exceeded(StepID).

# Halt on max retries
should_halt(StepID) :-
    retry_attempt(StepID, _, _),
    max_retries_exceeded(StepID).

# Halt on stagnation (code hash repeat)
should_halt(StepID) :-
    state(StepID, _, _),
    stagnation_detected().

# Halt on stability degradation
should_halt(StepID) :-
    state_at_iteration(StepID, _, _),
    stability_degrading(StepID).

# =============================================================================
# SHOULD_COMPLETE DERIVATION
# =============================================================================

# Complete when converged
should_complete(StepID) :-
    state_at_iteration(StepID, _, _),
    converged(StepID).
