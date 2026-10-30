# Reviewer Shard Policy - Code Review & Security Logic
# Loaded by ReviewerShard kernel alongside base policy.gl
# Part of Cortex 1.5.0 Architecture

# SECTION 1 (reviewer_task -> reviewer_action) is gone (2026-09-23): nothing
# asserted reviewer_task and nothing read reviewer_action. What kind of review
# a turn is, is its intent verb (/review, /security, /lint, ...), which
# persona_verb and reasoning_intensive_verb already read.

# =============================================================================
# SECTION 2: FINDING SEVERITY CLASSIFICATION
# =============================================================================
# NOTE: review_finding/5 is declared in schemas.mg:
#   review_finding(File, Line, Severity, Category, Message)
# This matches the facts emitted by Go's ReviewerShard.

# Critical severity patterns
is_critical_finding(File, Line) :-
    review_finding(File, Line, /critical, _, _).

is_critical_finding(File, Line) :-
    review_finding(File, Line, _, /security, _).

# Error severity
is_error_finding(File, Line) :-
    review_finding(File, Line, /error, _, _).

# Warning severity
is_warning_finding(File, Line) :-
    review_finding(File, Line, /warning, _, _).

# =============================================================================
# SECTION 2B: FINDING COUNT AGGREGATIONS
# =============================================================================
# These aggregations enable commit blocking and statistics rules.
# Uses the |> do fn:group_by() syntax required by Mangle.

# Count findings by severity level
finding_count(Severity, N) :-
    review_finding(_, _, Severity, _, _) |>
    do fn:group_by(Severity),
    let N = fn:count().

# =============================================================================
# SECTION 3: COMMIT BLOCKING
# =============================================================================

# Block commit on critical findings
block_commit("critical_security_finding") :-
    is_critical_finding(_, _).

# Block commit on high error count
block_commit("too_many_errors") :-
    finding_count(/error, N),
    N > 10.

# Block commit on security issues
block_commit("security_vulnerabilities") :-
    review_finding(_, _, _, /security, _).

# =============================================================================
# SECTION 4: REVIEW PRIORITIZATION
# =============================================================================
# Uses churn_rate from schemas.gl

# High priority files (recently modified, high churn)
# Note: Rate is integer (churn count), not float
high_priority_review(File) :-
    modified(File),
    churn_rate(File, Rate),
    Rate > 3.

# =============================================================================
# SECTION 6: COMPLEXITY THRESHOLDS
# =============================================================================

Decl code_metrics(TotalLines, CodeLines, CyclomaticAvg, FunctionCount).



# High complexity warning
complexity_warning(File, Function) :-
    cyclomatic_complexity(File, Function, C),
    C > 15.

# Deep nesting warning
nesting_warning(File, Function) :-
    nesting_depth(File, Function, D),
    D > 5.

# Long file warning
long_file_warning(File) :-
    file_line_count(File, Lines),
    Lines > 500.

# =============================================================================
# SECTION 7: AUTOPOIESIS - LEARNING FROM REVIEWS
# =============================================================================

Decl review_approved(ReviewID, Pattern).

# Aggregation: Count how many times each message pattern appears in findings
# Enables recurring issue detection for autopoiesis learning
pattern_count(Message, N) :-
    review_finding(_, _, _, _, Message) |>
    do fn:group_by(Message),
    let N = fn:count().

# Aggregation: Count how many times each pattern was approved by users
# Enables approved_pattern learning
approval_count(Pattern, N) :-
    review_approved(_, Pattern) |>
    do fn:group_by(Pattern),
    let N = fn:count().

# Track patterns that get flagged repeatedly (>= 3 times)
# NOTE: Pattern is extracted from Message (5th arg) in 5-arg schema
recurring_issue_pattern(Message, Category) :-
    review_finding(_, _, _, Category, Message),
    pattern_count(Message, N),
    N >= 3.

# Learn project-specific anti-patterns
# Note: Category is implicitly tracked via recurring_issue_pattern
promote_to_long_term(/anti_pattern, Pattern) :-
    recurring_issue_pattern(Pattern, _).

# Track patterns that pass review (approved >= 3 times)
approved_pattern(Pattern) :-
    review_approved(_, Pattern),
    approval_count(Pattern, N),
    N >= 3.

# Promote approved styles
promote_to_long_term(/approved_style, Pattern) :-
    approved_pattern(Pattern).

# =============================================================================
# SECTION 8: REVIEW STATUS
# =============================================================================

Decl review_complete(Files, Severity).
Decl security_issue(File, Line, RuleID, Message).

# Helper for safe negation - true if any block_commit exists
has_block_commit() :-
    block_commit(_).

# Overall review status
review_passed(Files) :-
    review_complete(Files, /clean).

review_passed(Files) :-
    review_complete(Files, /info).

review_passed(Files) :-
    review_complete(Files, /warning),
    !has_block_commit().

review_failed(Files) :-
    review_complete(Files, /error).

review_failed(Files) :-
    review_complete(Files, /critical).

review_blocked(Files) :-
    review_complete(Files, _),
    has_block_commit().

# =============================================================================
# SECTION 9: STYLE RULES
# =============================================================================

Decl style_violation(File, Line, Rule, Message).

# Common style rules
# Threshold is the numeric budget the rule permits. STY003 previously carried
# the regex "TODO|FIXME" in this slot, which is a string in a /number slot: it
# would abort whole-program evaluation the first time any rule compared it.
# The threshold is 0 (no unreferenced TODOs permitted) and the pattern moved to
# style_rule_pattern/2.
style_rule("STY001", "line_length", 120).
style_rule("STY002", "trailing_whitespace", 0).
style_rule("STY003", "todo_without_issue", 0).
style_rule("STY005", "max_nesting", 5).

# Regex patterns for the pattern-shaped style rules.
style_rule_pattern("STY003", "TODO|FIXME").

# Style violation from rule
has_style_violation(File) :-
    style_violation(File, _, _, _).

# =============================================================================
# SECTION 11: REVIEWER FEEDBACK LOOP (Self-Correction)
# =============================================================================
# These rules enable the reviewer to learn from mistakes and self-correct.

# Helper: Check if a review has any rejections
Decl has_rejections(ReviewID).
has_rejections(ReviewID) :-
    user_rejected_finding(ReviewID, _, _, _, _).

# Aggregation: Count rejections per review session
# Renamed from rejection_count to avoid conflict with schemas.mg's rejection_count(Pattern, Count)
review_rejection_count(ReviewID, N) :-
    user_rejected_finding(ReviewID, _, _, _, _) |>
    do fn:group_by(ReviewID),
    let N = fn:count().

# Review is suspect if user rejected multiple findings
review_suspect(ReviewID, "multiple_rejections") :-
    user_rejected_finding(ReviewID, File1, Line1, _, _),
    user_rejected_finding(ReviewID, File2, Line2, _, _),
    File1 != File2.

review_suspect(ReviewID, "multiple_rejections") :-
    user_rejected_finding(ReviewID, File, Line1, _, _),
    user_rejected_finding(ReviewID, File, Line2, _, _),
    Line1 != Line2.

# Review is suspect if >50% findings were rejected
# NOTE: Mangle can't do percentage math; use rejection_rate_high virtual predicate
review_suspect(ReviewID, "high_rejection_rate") :-
    review_accuracy(ReviewID, Total, _, _, _),
    Total > 2,
    review_rejection_rate_high(ReviewID).

# Trigger validation for suspect reviews
reviewer_needs_validation(ReviewID) :-
    review_suspect(ReviewID, _).

# --- Self-Correction Signals ---

# Signal to main agent: recent review may be inaccurate
Decl recent_review_unreliable() bound [].

# REMOVED: the raw_finding family (static-analysis findings: security flows,
# unwired symbols, shotgun surgery, hero risk, layer leakage, zombie tests,
# circular dependencies) with its suppression rules and every helper that fed
# only it. Nothing read raw_finding or active_finding: no Go query, no rule, no
# context or commit gate. review_finding, the findings the reviewer reports, is
# what the gates read.
