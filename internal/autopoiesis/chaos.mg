# MANDATE FOR AI CODING TOOLS:
# This file contains critical product requirements and architectural mandates.
# DO NOT REMOVE OR SIGNIFICANTLY ALTER THESE COMMENTS.
# They serve as a source of truth for the Symbiogen Agentic Intelligence Platform.
# YOU MUST READ THIS ENTIRE HEADER BEFORE AND AFTER EVERY INTERACTION WITH THIS FILE.

# Symbiogen Product Requirements Document (PRD) for internal/autopoiesis/chaos.mg
#
# File: internal/autopoiesis/chaos.mg
# Author: The Nemesis System
# Date: 2025-12-10
#
# Overview:
# Defines the Mangle schema and rules for Adversarial Co-Evolution.
# This governs the PanicMaker (tool-level) and Nemesis (system-level) adversarial testing.
#
# Key Features & Business Value:
# - Battle-hardened tool promotion rules
# - Nemesis victory/defeat tracking
# - Lazy pattern detection (anti-autopoiesis)
# - System invariant monitoring
#
# Architectural Context:
# - Component Type: Mangle Schema
# - Used By: Thunderdome (thunderdome.go), Nemesis (nemesis.go), Ouroboros (ouroboros.go)
#
# --- END OF PRD HEADER ---

# =============================================================================
# PANICMAKER: TOOL-LEVEL ADVERSARIAL TESTING
# =============================================================================

# Attack vector tracking
Decl attack_vector(AttackID, Name, Category, ToolName).
Decl attack_executed(AttackID, ToolName, Timestamp).
Decl attack_survived(AttackID, ToolName, DurationMS).
Decl attack_killed(AttackID, ToolName, FailureType, StackDump).

# PanicMaker verdict
Decl panic_maker_verdict(ToolName, Verdict, Timestamp).  # Verdict: /survived, /defeated

# Tool survived PanicMaker if no kills recorded
panic_maker_survived(ToolName) :-
    panic_maker_verdict(ToolName, /survived, _).

# Tool was killed by PanicMaker
panic_maker_killed(ToolName) :-
    panic_maker_verdict(ToolName, /defeated, _).

# A step is only valid if it survives the panic maker
step_survived_panic_maker(StepID) :-
    state(StepID, _, _),
    !panic_maker_killed(StepID).

# =============================================================================
# THUNDERDOME: BATTLE ARENA TRACKING
# =============================================================================

# Battle record
Decl thunderdome_battle(BattleID, ToolName, StartTime, EndTime, Verdict).
Decl thunderdome_stats(TotalBattles, Survived, Defeated).

# Tool is battle-hardened if it survived all attacks
Decl battle_hardened(ToolName, Timestamp).

battle_hardened(ToolName, T) :-
    thunderdome_battle(_, ToolName, _, T, /survived).

# Tool is fragile if it was defeated
Decl fragile(ToolName, AttackCategory).

fragile(ToolName, Category) :-
    attack_killed(_, ToolName, _, _),
    attack_vector(AttackID, _, Category, ToolName),
    attack_killed(AttackID, _, _, _).

# =============================================================================
# NEMESIS: SYSTEM-LEVEL ADVERSARIAL TESTING
# =============================================================================

# Patch tracking
Decl patch(PatchID, CommitHash, Description, Timestamp).
Decl patch_tested(PatchID, TestType, Timestamp).
Decl patch_status(PatchID, Status).  # Status: /pending, /testing, /accepted, /rejected

# Nemesis attack tools
Decl nemesis_attack_tool(ToolID, Name, TargetPatch, Category).
Decl nemesis_attack_run(ToolID, PatchID, Timestamp, Verdict).

# A patch is only "battle_hardened" if Nemesis fails to break it
patch_accepted(PatchID) :-
    patch_tested(PatchID, /nemesis, _),
    !nemesis_victory(PatchID).

# Nemesis wins if it triggers a system invariant violation
nemesis_victory(PatchID) :-
    nemesis_attack_run(AttackTool, PatchID, _, /success),
    system_invariant_violated(_, _).

# Patch rejected due to Nemesis
patch_rejected(PatchID) :-
    nemesis_victory(PatchID).

# =============================================================================
# SYSTEM INVARIANTS
# =============================================================================

# Invariant declarations
Decl system_invariant(InvariantID, Name, Threshold).
Decl invariant_value(InvariantID, Value, Timestamp).
Decl system_invariant_violated(InvariantID, Timestamp).

# HTTP 500 rate exceeded
system_invariant_violated(/http_500_rate, T) :-
    invariant_value(/http_500_rate, Rate, T),
    system_invariant(/http_500_rate, _, Threshold),
    Rate > Threshold.

# Deadlock detected
system_invariant_violated(/deadlock_detected, T) :-
    invariant_value(/deadlock_detected, 1, T).

# Memory usage exceeded
system_invariant_violated(/memory_exceeded, T) :-
    invariant_value(/memory_mb, MemMB, T),
    system_invariant(/memory_limit_mb, _, Limit),
    MemMB > Limit.

# Goroutine count exceeded
system_invariant_violated(/goroutine_leak, T) :-
    invariant_value(/goroutine_count, Count, T),
    system_invariant(/goroutine_limit, _, Limit),
    Count > Limit.

# Response latency exceeded
system_invariant_violated(/latency_exceeded, T) :-
    invariant_value(/p99_latency_ms, Latency, T),
    system_invariant(/latency_limit_ms, _, Limit),
    Latency > Limit.

# =============================================================================
# ARMORY: PERSISTED ATTACK TOOLS
# =============================================================================

# Armory entries (successful attacks become regression tests)
Decl armory_tool(ToolID, Name, Category, TargetVulnerability, CreatedAt).
Decl armory_run(ToolID, BuildID, Timestamp, Verdict).

# A tool should be added to armory if it broke the system
should_add_to_armory(ToolID) :-
    nemesis_attack_run(ToolID, _, _, /success).

# A tool should be retired from armory if it hasn't found bugs recently
Decl armory_tool_stale(ToolID).

armory_tool_stale(ToolID) :-
    armory_tool(ToolID, _, _, _, _),
    !recent_armory_success(ToolID).

recent_armory_success(ToolID) :-
    armory_run(ToolID, _, T, /success),
    current_time(Now),
    fn:minus(Now, T) < 2592000.  # 30 days in seconds

# =============================================================================
# ANTI-AUTOPOIESIS: LAZY PATTERN DETECTION
# =============================================================================

# Track fix patterns
Decl fix_pattern(PatternID, FixType, Count, LastSeen).
Decl lazy_pattern_detected(PatternID, FixType).

# Common lazy fix types
# /timeout_increase - Just increasing timeout instead of fixing root cause
# /retry_addition - Adding retries without addressing underlying issue
# /error_swallow - Catching and ignoring errors
# /mutex_wrap - Wrapping everything in mutex without understanding concurrency

# Detect lazy timeout increases (3+ in recent history)
lazy_pattern_detected(/timeout_lazy, /timeout_increase) :-
    fix_pattern(_, /timeout_increase, Count, _),
    Count >= 3.

# Detect lazy retry additions (3+ in recent history)
lazy_pattern_detected(/retry_lazy, /retry_addition) :-
    fix_pattern(_, /retry_addition, Count, _),
    Count >= 3.

# Nemesis should generate targeted attack for lazy patterns
Decl should_target_lazy_pattern(PatternID, AttackStrategy).

should_target_lazy_pattern(/timeout_lazy, /latency_injection) :-
    lazy_pattern_detected(/timeout_lazy, _).

should_target_lazy_pattern(/retry_lazy, /partial_failure_storm) :-
    lazy_pattern_detected(/retry_lazy, _).

# =============================================================================
# GAUNTLET: CAMPAIGN PHASE RULES
# =============================================================================

# A patch can only enter production if it survives The Gauntlet
Decl gauntlet_result(PatchID, Phase, Verdict, Timestamp).
Decl gauntlet_required(PatchID).

# All code changes require Gauntlet by default
gauntlet_required(PatchID) :-
    patch(PatchID, _, _, _).

# Gauntlet phases: /unit, /integration, /nemesis, /regression
gauntlet_passed(PatchID) :-
    gauntlet_result(PatchID, /unit, /passed, _),
    gauntlet_result(PatchID, /integration, /passed, _),
    gauntlet_result(PatchID, /nemesis, /passed, _),
    gauntlet_result(PatchID, /regression, /passed, _).

# Patch can be merged if Gauntlet passed
merge_allowed(PatchID) :-
    gauntlet_required(PatchID),
    gauntlet_passed(PatchID).

# =============================================================================
# OUROBOROS INTEGRATION: ENHANCED VALIDATION
# =============================================================================

# Enhanced transition validation that includes adversarial testing
# A tool transition is only valid if:
# 1. Stability improved (from state.mg)
# 2. PanicMaker survived
# 3. No critical safety violations

valid_adversarial_transition(Next) :-
    proposed(Next),
    effective_stability(Next, Score),
    Score > 0.8,
    step_survived_panic_maker(Next),
    !safety_violation(Next, /critical).

# Decl for safety violations from checker.go
Decl safety_violation(StepID, Severity).

# =============================================================================
# METRICS AND REPORTING
# =============================================================================

# Track overall adversarial effectiveness
Decl adversarial_effectiveness(Period, Bugs Found, Total Tests).

# A successful adversarial program finds bugs before production
adversarial_value_demonstrated() :-
    nemesis_victory(_).

adversarial_value_demonstrated() :-
    panic_maker_killed(_).

# =============================================================================
# HELPER PREDICATES
# =============================================================================

Decl current_time(Timestamp).

# These will be asserted by Go code:
# - attack_vector/4, attack_executed/3, attack_survived/3, attack_killed/4
# - panic_maker_verdict/3
# - thunderdome_battle/5
# - patch/4, patch_tested/3
# - nemesis_attack_tool/4, nemesis_attack_run/4
# - invariant_value/3
# - armory_tool/5, armory_run/4
# - fix_pattern/4
# - gauntlet_result/4
# - current_time/1
