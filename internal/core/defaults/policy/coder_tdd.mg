# Coder Policy - TDD Integration
# Version: 1.0.0
# Extracted from coder.mg Section 9

# =============================================================================
# SECTION 9: TDD INTEGRATION
# =============================================================================
# Rules for Test-Driven Development loop integration.

# -----------------------------------------------------------------------------
# 9.1 TDD State Awareness
# -----------------------------------------------------------------------------

# TDD is active
tdd_active() :-
    tdd_state(_).

# In red phase: tests should fail
tdd_red_phase() :-
    tdd_state(/red).

# In green phase: make tests pass
tdd_green_phase() :-
    tdd_state(/green).

# In refactor phase: improve code
tdd_refactor_phase() :-
    tdd_state(/refactor).

# -----------------------------------------------------------------------------
# 9.2 TDD-Aware Code Generation
# -----------------------------------------------------------------------------

# During green phase, focus on minimal implementation
minimal_implementation_mode() :-
    tdd_green_phase().

# During refactor phase, allow optimization
refactor_mode() :-
    tdd_refactor_phase().

# TDD retry awareness (don't repeat same fix)
tdd_different_approach_needed() :-
    tdd_state(/green),
    tdd_retry_count(N),
    N >= 2.

# -----------------------------------------------------------------------------
# 9.3 TDD Validation
# -----------------------------------------------------------------------------

# Edit is implementation (not test)
edit_is_implementation(File) :-
    pending_edit(File, _),
    !is_test_file(File).

# Edit is test code
edit_is_test(File) :-
    pending_edit(File, _),
    is_test_file(File).

# TDD violation: writing implementation in red phase
tdd_violation(/red_phase_impl) :-
    tdd_red_phase(),
    edit_is_implementation(_).

# TDD violation: writing tests in green phase
tdd_violation(/green_phase_test) :-
    tdd_green_phase(),
    edit_is_test(_).
