# Coder Policy - Autopoiesis & Learning
# Version: 1.0.0
# Extracted from coder.mg Section 11

# =============================================================================
# SECTION 11: AUTOPOIESIS - LEARNING FROM PATTERNS
# =============================================================================
# Learn from rejections and acceptances.

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
coder_promote_to_long_term(/style_preference, Style) :-
    coder_rejection_pattern(Style).

# Promote error avoidance to long-term memory
coder_promote_to_long_term(/error_avoidance, ErrorType) :-
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
coder_promote_to_long_term(/preferred_pattern, Pattern) :-
    coder_success_pattern(Pattern).

# -----------------------------------------------------------------------------
# 11.3 Learning Signals
# -----------------------------------------------------------------------------

# Signal to avoid patterns that always fail
coder_learning_signal(/avoid, Pattern) :-
    coder_rejection_count(_, Pattern, N),
    N >= 3,
    acceptance_count(Pattern, M),
    M < 1.

# Signal to prefer patterns that always succeed
coder_learning_signal(/prefer, Pattern) :-
    acceptance_count(Pattern, N),
    N >= 5,
    coder_rejection_count(_, Pattern, M),
    M < 1.

# Helper for rejection check
has_rejection(Pattern) :-
    coder_rejection_count(_, Pattern, _).
