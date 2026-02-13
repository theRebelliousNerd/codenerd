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
# finding_count(Severity, N) :-
#     review_finding(_, _, Severity, _, _) |>
#     do fn:group_by(Severity),
#     let N = fn:count().

# =============================================================================
# SECTION 3: COMMIT BLOCKING
# =============================================================================

# Block commit on critical findings
block_commit("critical_security_finding") :-
    is_critical_finding(_, _).

# Block commit on high error count
# block_commit("too_many_errors") :-
#     finding_count(/error, N),
#     N > 10.

# Block commit on security issues
block_commit("security_vulnerabilities") :-
    review_finding(_, _, _, /security, _).

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
# SECTION 5B: FLOW-BASED SECURITY DETECTION (Language-Agnostic)
# =============================================================================
# These rules enable security detection across ANY programming language by
# abstracting sink/source relationships rather than language-specific patterns.
# Flow-based rules take precedence when detected_security_flow facts are available;
# regex-based security_rule/4 facts (above) serve as fallback.

# Abstract security rules keyed by sink type (language-agnostic)
# SinkType: /sql_sink, /command_sink, /dom_sink, /hardcoded_secret, /weak_crypto
flow_security_rule("SEC001", /critical, /sql_sink, "SQL injection: user input flows to SQL execution").
flow_security_rule("SEC002", /critical, /command_sink, "Command injection: user input flows to command execution").
flow_security_rule("SEC003", /critical, /hardcoded_secret, "Hardcoded credential detected in source").
flow_security_rule("SEC004", /error, /dom_sink, "XSS: untrusted data flows to DOM manipulation").
flow_security_rule("SEC006", /warning, /weak_crypto, "Weak cryptographic algorithm detected").

# Derive security findings from flow analysis results
# detected_security_flow/5 facts are emitted by language-specific analyzers
# (Go, Python, JavaScript, Rust, etc.) in internal/world/dataflow_multilang.go
raw_finding(File, Line, Severity, /security, RuleID, Message) :-
    detected_security_flow(File, Line, SinkType, _, Confidence),
    Confidence > 50,
    flow_security_rule(RuleID, Severity, SinkType, Message).

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
# pattern_count(Message, N) :-
#     review_finding(_, _, _, _, Message) |>
#     do fn:group_by(Message),
#     let N = fn:count().

# Aggregation: Count how many times each pattern was approved by users
# Enables approved_pattern learning
# approval_count(Pattern, N) :-
#     review_approved(_, Pattern) |>
#     do fn:group_by(Pattern),
#     let N = fn:count().

# Track patterns that get flagged repeatedly (>= 3 times)
# NOTE: Pattern is extracted from Message (5th arg) in 5-arg schema
# recurring_issue_pattern(Message, Category) :-
#     review_finding(_, _, _, Category, Message),
#     pattern_count(Message, N),
#     N >= 3.

# Learn project-specific anti-patterns
# Note: Category is implicitly tracked via recurring_issue_pattern
# promote_to_long_term(/anti_pattern, Pattern) :-
#     recurring_issue_pattern(Pattern, _).

# Track patterns that pass review (approved >= 3 times)
# approved_pattern(Pattern) :-
#     review_approved(_, Pattern),
#     approval_count(Pattern, N),
#     N >= 3.

# Promote approved styles
# promote_to_long_term(/approved_style, Pattern) :-
#     approved_pattern(Pattern).

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
Decl is_finding_suppressed(File, Line, RuleID).

# Helper: Projection to ignore Reason for safe negation
is_finding_suppressed(File, Line, RuleID) :-
    suppressed_finding(File, Line, RuleID, _).

# Finding is active if not explicitly suppressed
active_finding(File, Line, Severity, Category, RuleID, Message) :-
    raw_finding(File, Line, Severity, Category, RuleID, Message),
    !is_finding_suppressed(File, Line, RuleID).

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

# Bridge rule: Associate 5-arg findings with the active review session
# This derives review_finding_with_id/6 from review_finding/5 when a review is active.
# The active_review(ReviewID) fact is asserted by Go when a review session starts.
Decl active_review(ReviewID).

review_finding_with_id(ReviewID, File, Line, Severity, Category, Message) :-
    active_review(ReviewID),
    review_finding(File, Line, Severity, Category, Message).

# Helper: Check if a review has any rejections
Decl has_rejections(ReviewID).
has_rejections(ReviewID) :-
    user_rejected_finding(ReviewID, _, _, _, _).

# Aggregation: Count rejections per review session
# Renamed from rejection_count to avoid conflict with schemas.mg's rejection_count(Pattern, Count)
# review_rejection_count(ReviewID, N) :-
#     user_rejected_finding(ReviewID, _, _, _, _) |>
#     do fn:group_by(ReviewID),
#     let N = fn:count().

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
review_suspect(ReviewID, "flagged_existing_symbol") :-
    review_finding_with_id(ReviewID, File, Line, _, _, Message),
    symbol_verified_exists(Symbol, File, _),
    :string:contains(Message, "undefined").

# Review is suspect if >50% findings were rejected
# NOTE: Mangle can't do percentage math; use rejection_rate_high virtual predicate
review_suspect(ReviewID, "high_rejection_rate") :-
    review_accuracy(ReviewID, Total, _, _, _),
    Total > 2,
    review_rejection_rate_high(ReviewID).

# Trigger validation for suspect reviews
reviewer_needs_validation(ReviewID) :-
    review_suspect(ReviewID, _).

# Trigger validation for reviews with "undefined" findings (common false positive)
reviewer_needs_validation(ReviewID) :-
    review_finding_with_id(ReviewID, _, _, /error, /bug, Message),
    :string:contains(Message, "undefined").

# Trigger validation for reviews with "not found" findings
reviewer_needs_validation(ReviewID) :-
    review_finding_with_id(ReviewID, _, _, /error, /bug, Message),
    :string:contains(Message, "not found").

# --- False Positive Learning ---

# Suppress findings that match learned false positive patterns
# Note: Confidence is integer 0-100, not float 0.0-1.0
suppressed_finding(File, Line, RuleID, "learned_false_positive") :-
    raw_finding(File, Line, _, Category, RuleID, Message),
    false_positive_pattern(Pattern, Category, Occurrences, Confidence),
    Occurrences > 2,
    Confidence > 70,
    string_contains(Message, Pattern).

# --- Self-Correction Signals ---

# Signal to main agent: recent review may be inaccurate
Decl recent_review_unreliable().

# =============================================================================
# SECTION 13: WIRING GAP DETECTION
# =============================================================================

Decl unwired_function(ID, File).
Decl is_called(CalleeID).

# Helper: ID is called by something
is_called(CalleeID) :- code_calls(_, CalleeID).

# Declare entry_point fact (generated by nerd init)
# Decl entry_point/1 is defined in core/defaults/schemas.mg.
# Helper to check if file is an entry point
Decl is_entry_point_file(File).
is_entry_point_file(File) :-
    entry_point(File).

# Identify public functions containing no incoming dependency links
# Exclude test files to avoid false positives on test helpers
# Exclude entry points (main.go, etc.) as they are naturally top-level
unwired_function(ID, File) :-
    symbol_graph(ID, "function", "public", File, _),
    code_defines(File, ID, /function, _, _),
    in_scope(File),
    !is_called(ID),
    file_topology(File, _, _, _, /false),
    !is_entry_point_file(File).

# Emit raw finding with Symbol ID as message to ensure unique findings per symbol
raw_finding(File, 1, /warning, /architecture, "UNWIRED_SYMBOL", Message) :-
    unwired_function(ID, File),
    Message = fn:string:concat("Unwired public function detected: ", ID, " (no callers found)").

# =============================================================================
# SECTION 14: ARCHITECTURAL INSIGHTS
# =============================================================================

# --- 1. Shotgun Surgery (Hidden Temporal Coupling) ---
# Files that change together often but have no static dependency
# OPTIMIZED: Avoids Cartesian explosion on large git histories by:
# 1. Using FileA < FileB to eliminate duplicate pairs
# 2. Aggregating co-commits first to limit the search space
# 3. Only checking dependency for frequently co-committed pairs (>= 3 times)

Decl hidden_coupling(FileA, FileB).

# Redefine helper to work with Symbol IDs -> Files
Decl dependency_link_exists(FileA, FileB).

dependency_link_exists(FileA, FileB) :-
    dependency_link(CallerID, CalleeID, _),
    symbol_graph(CallerID, _, _, FileA, _),
    symbol_graph(CalleeID, _, _, FileB, _).

# Helper: Files committed together in the same commit hash
# Use FileA < FileB to avoid duplicate pairs (A,B) and (B,A)
Decl co_committed_files(FileA, FileB, Hash).

co_committed_files(FileA, FileB, Hash) :-
    git_history(FileA, Hash, _, _, _),
    git_history(FileB, Hash, _, _, _),
    FileA != FileB.

# Aggregation: Count how many times each file pair was co-committed
Decl co_commit_count(FileA, FileB, Count).

# co_commit_count(FileA, FileB, N) :-
#     co_committed_files(FileA, FileB, _) |>
#     do fn:group_by(FileA, FileB),
#     let N = fn:count().

# Hidden coupling: Files that change together frequently (>= 3 times)
# but have no static dependency between them
# hidden_coupling(FileA, FileB) :-
#     co_commit_count(FileA, FileB, N),
#     N >= 3,
#     !dependency_link_exists(FileA, FileB),
#     !dependency_link_exists(FileB, FileA).

# Finding
raw_finding(FileA, 1, /warning, /architecture, "SHOTGUN_SURGERY", FileB) :-
    hidden_coupling(FileA, FileB).


# --- 2. The "Hero" Risk (Bus Factor) ---
# High churn + High complexity + Single Author


Decl hero_risk(File, Author).
Decl has_other_author(File, Author).

has_other_author(File, Author) :-
    git_history(File, _, Author, _, _),     # Bind Author to File history
    git_history(File, _, Other, _, _),      # Check for Other author
    Author != Other.

hero_risk(File, Author) :-
    churn_rate(File, Rate), Rate > 5,
    complexity_warning(File, _),
    git_history(File, _, Author, _, _),
    !has_other_author(File, Author).

# Finding
raw_finding(File, 1, /warning, /risk, "HERO_RISK", Author) :-
    hero_risk(File, Author).


# --- 3. Architectural Layer Leakage ---
# Library Code (Low level) should not import Application Entrypoints (High level)
# Uses configurable patterns for adaptability.

Decl layer(File, LayerName).
Decl architecture_violation(Caller, Callee).

# Configuration Point: Users can extend this in extensions.mg
# Decl configured_layer_pattern(Pattern, Layer).
Decl configured_layer_pattern(Pattern, Layer).

# Default Standard Conventions
configured_layer_pattern("/internal/", /library).
configured_layer_pattern("internal/", /library).
configured_layer_pattern("/pkg/", /library).
configured_layer_pattern("pkg/", /library).
configured_layer_pattern("/lib/", /library).
configured_layer_pattern("lib/", /library).
configured_layer_pattern("/core/", /library).
configured_layer_pattern("core/", /library).

configured_layer_pattern("/cmd/", /entrypoint).
configured_layer_pattern("cmd/", /entrypoint).
configured_layer_pattern("/app/", /entrypoint).
configured_layer_pattern("app/", /entrypoint).
configured_layer_pattern("/cli/", /entrypoint).
configured_layer_pattern("cli/", /entrypoint).
configured_layer_pattern("/bin/", /entrypoint).
configured_layer_pattern("bin/", /entrypoint).

# Derive Layer from Configuration
layer(File, Layer) :-
    symbol_graph(_, _, _, File, _),
    configured_layer_pattern(Pattern, Layer),
    string_contains(File, Pattern).

architecture_violation(CallerFile, CalleeFile) :-
    dependency_link(CallerID, CalleeID, _),
    symbol_graph(CallerID, _, _, CallerFile, _),
    symbol_graph(CalleeID, _, _, CalleeFile, _),
    layer(CallerFile, /library),
    layer(CalleeFile, /entrypoint).

# Finding
raw_finding(CallerFile, 1, /error, /architecture, "LAYER_LEAKAGE", "Library imports Entrypoint") :-
    architecture_violation(CallerFile, CalleeFile).


# --- 4. "Zombie" Tests ---
# Tests that exist but import nothing internal

Decl zombie_test(TestFile).
Decl test_imports_internal(TestFile).

test_imports_internal(TestFile) :-
    symbol_graph(TestID, _, _, TestFile, _),
    dependency_link(TestID, CalleeID, _),
    symbol_graph(CalleeID, _, _, CalleeFile, _),
    TestFile != CalleeFile, # Ensure it's external import (or at least different file)
    # Check if Callee is internal (e.g., same project prefix or not stdlib)
    # Simple heuristic: CalleeFile is in the file_topology (known project file)
    file_topology(CalleeFile, _, _, _, _).


zombie_test(TestFile) :-
    file_topology(TestFile, _, _, _, /true), # Is Test
    symbol_graph(_, _, _, TestFile, _),      # Generator/Safety
    !test_imports_internal(TestFile).

# Finding
raw_finding(TestFile, 1, /warning, /maintenance, "ZOMBIE_TEST", "Test imports no internal code") :-
    zombie_test(TestFile).

# --- 5. Circular Dependencies (File Level) ---
# A -> B -> ... -> A (Structural Cycle)

Decl file_dependency(CallerFile, CalleeFile).
Decl file_reachable(CallerFile, CalleeFile).
Decl circular_dependency(FileA, FileB).

# Direct dependency (using helper from Rule 1)
file_dependency(A, B) :-
    dependency_link_exists(A, B),
    A != B.

# Transitive closure (Reachability)
file_reachable(A, B) :-
    file_dependency(A, B).

file_reachable(A, C) :-
    file_dependency(A, B),
    file_reachable(B, C).

# Cycle detection
circular_dependency(A, B) :-
    file_dependency(A, B),
    file_reachable(B, A).

# Finding
# Note: This will flag both A->B and B->A.
raw_finding(A, 1, /error, /architecture, "CIRCULAR_DEPENDENCY", B) :-
    circular_dependency(A, B).
