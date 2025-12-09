# Reviewer Shard Policy - Code Review & Security Logic
# Loaded by ReviewerShard kernel alongside base policy.gl
# Part of Cortex 1.5.0 Architecture

# =============================================================================
# SECTION 1: REVIEWER TASK CLASSIFICATION
# =============================================================================

Decl reviewer_task(ID, Action, Files, Timestamp).

reviewer_action(/review) :-
    reviewer_task(_, /review, _, _).

reviewer_action(/security_scan) :-
    reviewer_task(_, /security_scan, _, _).

reviewer_action(/style_check) :-
    reviewer_task(_, /style_check, _, _).

reviewer_action(/complexity) :-
    reviewer_task(_, /complexity, _, _).

# =============================================================================
# SECTION 2: FINDING SEVERITY CLASSIFICATION
# =============================================================================
# NOTE: review_finding/6 is declared in schemas.mg

# Critical severity patterns
is_critical_finding(Finding) :-
    review_finding(Finding, _, _, /critical, _, _).

is_critical_finding(Finding) :-
    review_finding(Finding, _, _, _, /security, _).

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "sql injection").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "command injection").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "xss").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "hardcoded secret").

# is_critical_finding(Finding) :-
#    review_finding(Finding, _, _, _, _, Msg),
#    fn:string_contains(Msg, "path traversal").

# Error severity
is_error_finding(Finding) :-
    review_finding(Finding, _, _, /error, _, _).

# Warning severity
is_warning_finding(Finding) :-
    review_finding(Finding, _, _, /warning, _, _).

# =============================================================================
# SECTION 3: COMMIT BLOCKING
# =============================================================================

# Block commit on critical findings
block_commit("critical_security_finding") :-
    is_critical_finding(_).

# Block commit on high error count
block_commit("too_many_errors") :-
    finding_count(/error, N),
    N > 10.

# Block commit on security issues
block_commit("security_vulnerabilities") :-
    review_finding(_, _, _, _, /security, _).

# =============================================================================
# SECTION 4: REVIEW PRIORITIZATION
# =============================================================================
# Uses churn_rate from schemas.gl

Decl file_contains(FilePath, Pattern).

# High priority files (recently modified, high churn)
# Note: Rate is integer (churn count), not float
high_priority_review(File) :-
    modified(File),
    churn_rate(File, Rate),
    Rate > 3.

high_priority_review(File) :-
    modified(File),
    file_has_security_sensitive(File).

# Security-sensitive markers
file_has_security_sensitive(File) :-
    file_contains(File, "password").

file_has_security_sensitive(File) :-
    file_contains(File, "api_key").

file_has_security_sensitive(File) :-
    file_contains(File, "secret").

file_has_security_sensitive(File) :-
    file_contains(File, "credential").

file_has_security_sensitive(File) :-
    file_contains(File, "token").

file_has_security_sensitive(File) :-
    file_contains(File, "private_key").

# =============================================================================
# SECTION 5: SECURITY RULE DEFINITIONS
# =============================================================================

Decl security_rule(RuleID, Severity, Pattern, Message).

# SQL Injection
security_rule("SEC001", /critical, "execute.*concat", "SQL injection risk").
security_rule("SEC001", /critical, "raw.*sql.*concat", "SQL injection via raw query").

# Command Injection
security_rule("SEC002", /critical, "exec.Command.*concat", "Command injection risk").
security_rule("SEC002", /critical, "os.system.*concat", "Command injection via os.system").

# Hardcoded Secrets
security_rule("SEC003", /critical, "password.*=.*literal", "Hardcoded password").
security_rule("SEC003", /critical, "api_key.*=.*literal", "Hardcoded API key").

# XSS
security_rule("SEC004", /error, "innerHTML.*=", "XSS via innerHTML").
security_rule("SEC004", /error, "document.write", "XSS via document.write").

# Weak Crypto
security_rule("SEC006", /warning, "md5|sha1", "Weak cryptographic algorithm").

# =============================================================================
# SECTION 6: COMPLEXITY THRESHOLDS
# =============================================================================

Decl code_metrics(TotalLines, CodeLines, CyclomaticAvg, FunctionCount).
Decl cyclomatic_complexity(File, Function, Complexity).
Decl nesting_depth(File, Function, Depth).

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

Decl pattern_count(Pattern, Count).
Decl approval_count(Pattern, Count).
Decl review_approved(ReviewID, Pattern).

# Track patterns that get flagged repeatedly
recurring_issue_pattern(Pattern, Category) :-
    review_finding(_, _, _, _, Category, Pattern),
    pattern_count(Pattern, N),
    N >= 3.

# Learn project-specific anti-patterns
# Note: Category is implicitly tracked via recurring_issue_pattern
promote_to_long_term(/anti_pattern, Pattern) :-
    recurring_issue_pattern(Pattern, _).

# Track patterns that pass review
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
style_rule("STY001", "line_length", 120).
style_rule("STY002", "trailing_whitespace", 0).
style_rule("STY003", "todo_without_issue", "TODO|FIXME").
style_rule("STY005", "max_nesting", 5).

# Style violation from rule
has_style_violation(File) :-
    style_violation(File, _, _, _).

# =============================================================================
# SECTION 10: FINDING FILTERING & SUPPRESSION (Smart Rules)
# =============================================================================

# NOTE: raw_finding, active_finding declared in schemas.mg
Decl suppressed_finding(File, Line, RuleID, Reason).
Decl is_suppressed(File, Line, RuleID).

# Helper: Projection to ignore Reason for safe negation
is_suppressed(File, Line, RuleID) :-
    suppressed_finding(File, Line, RuleID, _).

# Finding is active if not explicitly suppressed
active_finding(File, Line, Severity, Category, RuleID, Message) :-
    raw_finding(File, Line, Severity, Category, RuleID, Message),
    !is_suppressed(File, Line, RuleID).

# --- Suppression Rules ---

# Suppress TODOs (STY003) in test files
suppressed_finding(File, Line, "STY003", "todo_allowed_in_tests") :-
    raw_finding(File, Line, _, _, "STY003", _),
    file_topology(File, _, _, _, /true).

# Suppress Magic Numbers (STY004) in test files
suppressed_finding(File, Line, "STY004", "magic_numbers_allowed_in_tests") :-
    raw_finding(File, Line, _, _, "STY004", _),
    file_topology(File, _, _, _, /true).

# Suppress Complexity Warnings in test files
suppressed_finding(File, Line, "COMPLEXITY", "complexity_allowed_in_tests") :-
    raw_finding(File, Line, _, /maintainability, "COMPLEXITY", _),
    file_topology(File, _, _, _, /true).

# Suppress Long File Warnings in test files
suppressed_finding(File, Line, "LONG_FILE", "long_files_allowed_in_tests") :-
    raw_finding(File, Line, _, /maintainability, "LONG_FILE", _),
    file_topology(File, _, _, _, /true).

# Suppress Hardcoded Secrets (SEC003) in test files (usually mocks keys)
suppressed_finding(File, Line, "SEC003", "secrets_allowed_in_tests") :-
    raw_finding(File, Line, _, /security, "SEC003", _),
    file_topology(File, _, _, _, /true).

# Suppress Generated Code (common pattern)
suppressed_finding(File, Line, RuleID, "generated_code") :-
    raw_finding(File, Line, _, _, RuleID, _),
    file_contains(File, "Code generated by").

# =============================================================================
# SECTION 11: REVIEWER FEEDBACK LOOP (Self-Correction)
# =============================================================================
# These rules enable the reviewer to learn from mistakes and self-correct.

# Helper: Check if a review has any rejections
Decl has_rejections(ReviewID).
has_rejections(ReviewID) :-
    user_rejected_finding(ReviewID, _, _, _, _).

# Helper: Count rejections for a review (aggregation)
# Note: Renamed from rejection_count to avoid conflict with schemas.mg's rejection_count(Pattern, Count)
Decl review_rejection_count(ReviewID, Count).

# Review is suspect if user rejected multiple findings
review_suspect(ReviewID, "multiple_rejections") :-
    user_rejected_finding(ReviewID, File1, Line1, _, _),
    user_rejected_finding(ReviewID, File2, Line2, _, _),
    File1 != File2.

review_suspect(ReviewID, "multiple_rejections") :-
    user_rejected_finding(ReviewID, File, Line1, _, _),
    user_rejected_finding(ReviewID, File, Line2, _, _),
    Line1 != Line2.

# Review is suspect if it flagged a symbol that was verified to exist
# TODO: Re-enable when string_contains virtual predicate is implemented
# review_suspect(ReviewID, "flagged_existing_symbol") :-
#     review_finding(ReviewID, File, Line, _, _, Message),
#     symbol_verified_exists(Symbol, File, _),
#     string_contains(Message, "undefined").

# Review is suspect if >50% findings were rejected
review_suspect(ReviewID, "high_rejection_rate") :-
    review_accuracy(ReviewID, Total, _, Rejected, _),
    Total > 2,
    DoubleRejected = fn:mult(Rejected, 2),
    DoubleRejected > Total.

# Trigger validation for suspect reviews
reviewer_needs_validation(ReviewID) :-
    review_suspect(ReviewID, _).

# Trigger validation for reviews with "undefined" findings (common false positive)
# TODO: Re-enable when string_contains virtual predicate is implemented
# reviewer_needs_validation(ReviewID) :-
#     review_finding(ReviewID, _, _, /error, /bug, Message),
#     string_contains(Message, "undefined").

# Trigger validation for reviews with "not found" findings
# TODO: Re-enable when string_contains virtual predicate is implemented
# reviewer_needs_validation(ReviewID) :-
#     review_finding(ReviewID, _, _, /error, /bug, Message),
#     string_contains(Message, "not found").

# --- False Positive Learning ---

# Suppress findings that match learned false positive patterns
# Note: Confidence is integer 0-100, not float 0.0-1.0
# TODO: Re-enable when string_contains virtual predicate is implemented
# suppressed_finding(File, Line, RuleID, "learned_false_positive") :-
#     raw_finding(File, Line, _, Category, RuleID, Message),
#     false_positive_pattern(Pattern, Category, Occurrences, Confidence),
#     Occurrences > 2,
#     Confidence > 70,
#     string_contains(Message, Pattern).

# --- Self-Correction Signals ---

# Signal to main agent: recent review may be inaccurate
Decl recent_review_unreliable().
recent_review_unreliable() :-
    review_suspect(_, _).

