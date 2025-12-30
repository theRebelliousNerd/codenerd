# Stress Test Log Analysis Queries
# Use with log-analyzer skill's logquery tool
#
# Usage:
#   1. Parse logs: python parse_log.py .nerd/logs/* --no-schema > facts.mg
#   2. Load: ./logquery.exe facts.mg -i
#   3. Run queries below
#
# =============================================================================
# QUERY INDEX (899 lines total)
# =============================================================================
#
# SECTION 1: ERROR DETECTION (lines 19-46)
#   - error_entry/3, warning_entry/3, panic_detected/3
#   - nil_pointer_error/3, oom_event/3, timeout_event/3
#   - deadline_exceeded/3
#
# SECTION 2: RESOURCE STRESS (lines 50-66)
#   - memory_warning/2, queue_full/2, limit_exceeded/3
#   - gas_limit_hit/2
#
# SECTION 3: SHARD LIFECYCLE (lines 70-86)
#   - shard_spawned/2, shard_completed/2, shard_error/2
#   - shard_timeout/2
#
# SECTION 4: API STRESS (lines 90-102)
#   - api_call/2, api_error/2, rate_limit_hit/2
#
# SECTION 4.5: API SCHEDULER (lines ~233-290)
#   - slot_acquired/3, slot_released/3, slot_wait/3
#   - scheduler_initialized/2, shard_registered/2, shard_unregistered/2
#   - api_scheduler_metrics/2, slot_contention/2, slot_leak/2
#
# SECTION 5: KERNEL STRESS (lines 106-122)
#   - kernel_event/3, kernel_error/2, derivation_event/2
#   - fact_operation/2
#
# SECTION 6: CAMPAIGN STRESS (lines 126-138)
#   - campaign_event/3, phase_complete/2, checkpoint_event/2
#
# SECTION 7: AUTOPOIESIS (lines 142-158)
#   - tool_generation/2, tool_compilation/2
#   - ouroboros_event/2, thunderdome_event/2
#
# SECTION 8: MANGLE SELF-HEALING (lines 162-359)
#   8.1 Infinite Loop Detection (lines 167-188)
#   8.2 JIT Repair Tracking (lines 192-216)
#   8.3 File Watcher Events (lines 220-236)
#   8.4 Startup Validation (lines 240-265)
#   8.5 Budget Tracking (lines 269-285)
#   8.6 Corpus Usage (lines 289-313)
#   8.7 Existing Self-Healing (lines 317-359)
#
# SECTION 9: 75+ FAILURE MODES (lines 363-768)
#   9.1 Kernel & Core Runtime (6 modes, lines 367-386)
#   9.2 Spawn Queue Failures (4 modes, lines 390-406)
#   9.3 Virtual Store Failures (4 modes, lines 410-426)
#   9.4 Limits Enforcer (3 modes, lines 430-440)
#   9.5 Perception Layer (4 modes, lines 444-460)
#   9.6 LLM Client (4 modes, lines 464-476)
#   9.7 Articulation Layer (4 modes, lines 480-494)
#   9.8 Coder Shard (4 modes, lines 498-514)
#   9.9 Tester Shard (4 modes, lines 518-534)
#   9.10 Reviewer Shard (4 modes, lines 538-554)
#   9.11 Researcher Shard (4 modes, lines 558-574)
#   9.12 Nemesis Shard (3 modes, lines 578-590)
#   9.13 Ouroboros (4 modes, lines 594-610)
#   9.14 Thunderdome (4 modes, lines 614-630)
#   9.15 Campaign (4 modes, lines 634-650)
#   9.16 World Model (4 modes, lines 654-670)
#   9.17 Holographic (3 modes, lines 674-686)
#   9.18 Dream State (3 modes, lines 690-702)
#   9.19 Shadow Mode (2 modes, lines 706-714)
#   9.20 Browser (3 modes, lines 718-730)
#   9.21 All Failure Modes Aggregation (lines 734-768)
#
# SECTION 10: AGGREGATIONS (lines 772-813)
#   - Self-healing specific aggregations
#   - JIT system health, validation pipeline
#   - Healing effectiveness metrics
#
# SECTION 11: FAILURE SUMMARIES (lines 817-857)
#   - critical_failure/3, high_severity_failure/3
#   - medium_severity_failure/3, low_severity_failure/3
#   - failure_by_severity/4
#
# SECTION 12: SUCCESS CRITERIA (lines 861-899)
#   - critical_issue/3, self_healing_health/2
#   - stress_test_health/3
#
# =============================================================================
# KEY PREDICATES FOR SELF-HEALING ANALYSIS
# =============================================================================
#
# INFINITE LOOP DETECTION:
#   - timeout_detected(Time, Category, Msg)
#   - gas_limit_hit(Time, Msg)
#   - recursion_exceeded(Time, Msg)
#   - derivation_explosion(Time, Msg)
#   - cyclic_rule_detected(Time, Msg)
#   - stratification_error(Time, Msg)
#
# JIT REPAIR:
#   - jit_repair_triggered(Time, ErrorType, Msg)  # ErrorType: /undeclared_predicate, /syntax_error, etc.
#   - repair_attempt_count(Time, AttemptNum, Msg)
#   - repair_success(Time, Msg)
#   - repair_failure(Time, Msg)
#   - repair_max_retries_exceeded(Time, Msg)
#
# FILE WATCHER:
#   - file_change_detected(Time, Msg)
#   - validation_triggered(Time, Msg)
#   - repair_on_save(Time, Msg)
#   - file_watcher_error(Time, Msg)
#
# STARTUP VALIDATION:
#   - startup_validation_result(Time, Status, Msg)  # Status: /pass or /fail
#   - invalid_rules_found(Time, Count, Msg)
#   - commented_rules_detected(Time, Msg)
#   - previously_healed_count(Time, Msg)
#   - kernel_boot(Time, Msg)
#   - boot_failure(Time, Msg)
#
# BUDGET TRACKING:
#   - budget_remaining(Time, Msg)
#   - budget_exhausted(Time, Msg)
#   - repair_cost(Time, Msg)
#   - token_usage(Time, Msg)
#
# CORPUS USAGE:
#   - corpus_loaded(Time, Msg)
#   - corpus_predicates_used(Time, Count, Msg)
#   - selector_relevance_score(Time, Msg)
#   - corpus_validation(Time, Msg)
#   - corpus_query_failure(Time, Msg)
#   - corpus_stats(Time, Msg)
#
# HEALTH CHECKS:
#   - self_healing_health(Status, Msg)
#   - jit_system_health(Time, Status, Msg)
#   - validation_pipeline(Time, Stage, Result, Msg)
#   - healing_metric(Time, Metric, Msg)
#
# FAILURE MODE QUERIES (75+ modes):
#   - any_failure_mode(Time, Mode, Category, Msg)
#   - failure_by_severity(Time, Severity, Mode, Msg)
#   - critical_failure(Time, Mode, Msg)
#   - high_severity_failure(Time, Mode, Msg)
#   - medium_severity_failure(Time, Mode, Msg)
#   - low_severity_failure(Time, Mode, Msg)
#
# =============================================================================

# =============================================================================
# SCHEMA (matches log-analyzer schema)
# =============================================================================

Decl log_entry(time: num, category: name, level: name, message: name, file: name, line: num).

# =============================================================================
# ERROR DETECTION QUERIES
# =============================================================================

# All errors
Decl error_entry(time: num, category: name, message: name).
error_entry(T, C, M) :- log_entry(T, C, /error, M, _, _).

# All warnings
Decl warning_entry(time: num, category: name, message: name).
warning_entry(T, C, M) :- log_entry(T, C, /warn, M, _, _).

# Panics (message contains "panic")
Decl panic_detected(time: num, category: name, message: name).
panic_detected(T, C, M) :- log_entry(T, C, _, M, _, _), fn:contains(M, "panic").

# Nil pointer errors
Decl nil_pointer_error(time: num, category: name, message: name).
nil_pointer_error(T, C, M) :- log_entry(T, C, /error, M, _, _), fn:contains(M, "nil pointer").

# Out of memory events
Decl oom_event(time: num, category: name, message: name).
oom_event(T, C, M) :- log_entry(T, C, /error, M, _, _), fn:contains(M, "out of memory").

# Timeout events
Decl timeout_event(time: num, category: name, message: name).
timeout_event(T, C, M) :- log_entry(T, C, /error, M, _, _), fn:contains(M, "timeout").

# Deadline exceeded
Decl deadline_exceeded(time: num, category: name, message: name).
deadline_exceeded(T, C, M) :- log_entry(T, C, /error, M, _, _), fn:contains(M, "deadline").

# =============================================================================
# RESOURCE STRESS QUERIES
# =============================================================================

# Memory warnings
Decl memory_warning(time: num, message: name).
memory_warning(T, M) :- log_entry(T, _, /warn, M, _, _), fn:contains(M, "memory").

# Queue full events
Decl queue_full(time: num, message: name).
queue_full(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "queue full").

# Limit exceeded events
Decl limit_exceeded(time: num, category: name, message: name).
limit_exceeded(T, C, M) :- log_entry(T, C, /warn, M, _, _), fn:contains(M, "limit").

# Gas limit in Mangle
Decl gas_limit_hit(time: num, message: name).
gas_limit_hit(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "gas").

# =============================================================================
# SHARD LIFECYCLE QUERIES
# =============================================================================

# Shard spawn events
Decl shard_spawned(time: num, message: name).
shard_spawned(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "spawn").

# Shard completion events
Decl shard_completed(time: num, message: name).
shard_completed(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "complet").

# Shard errors
Decl shard_error(time: num, message: name).
shard_error(T, M) :- log_entry(T, /shards, /error, M, _, _).

# Shard timeout
Decl shard_timeout(time: num, message: name).
shard_timeout(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "timeout").

# =============================================================================
# API STRESS QUERIES
# =============================================================================

# API call events
Decl api_call(time: num, message: name).
api_call(T, M) :- log_entry(T, /api, _, M, _, _).

# API errors
Decl api_error(time: num, message: name).
api_error(T, M) :- log_entry(T, /api, /error, M, _, _).

# Rate limit hits
Decl rate_limit_hit(time: num, message: name).
rate_limit_hit(T, M) :- log_entry(T, /api, _, M, _, _), fn:contains(M, "rate").

# =============================================================================
# API SCHEDULER STRESS QUERIES
# =============================================================================

# Scheduler initialization
Decl scheduler_initialized(time: num, message: name).
scheduler_initialized(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "APIScheduler: initialized").

# Shard registration with scheduler
Decl shard_registered(time: num, message: name).
shard_registered(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "APIScheduler: registered shard").

# Shard unregistration
Decl shard_unregistered(time: num, message: name).
shard_unregistered(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "APIScheduler: unregistered shard").

# Slot acquisition events
Decl slot_acquired(time: num, shard_id: name, message: name).
slot_acquired(T, /unknown, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "acquired slot").

# Slot release events
Decl slot_released(time: num, shard_id: name, message: name).
slot_released(T, /unknown, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "released slot").

# Slot wait events (contention detected)
Decl slot_wait(time: num, shard_id: name, message: name).
slot_wait(T, /unknown, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "waiting for slot").

# Slot acquisition after wait (shows wait duration)
Decl slot_acquired_after_wait(time: num, message: name).
slot_acquired_after_wait(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "acquired slot after").

# Scheduler metrics in logs
Decl api_scheduler_metrics(time: num, message: name).
api_scheduler_metrics(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "api_calls=").
api_scheduler_metrics(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "total_calls=").
api_scheduler_metrics(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "total_wait=").

# Slot contention (5 slots in use)
Decl slot_contention(time: num, message: name).
slot_contention(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "active=5/5").

# Potential slot leak (release without acquire)
Decl slot_leak_warning(time: num, message: name).
slot_leak_warning(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "released slot it didn't hold").

# Context cancellation during slot wait
Decl slot_wait_cancelled(time: num, message: name).
slot_wait_cancelled(T, M) :- log_entry(T, /shards, /warn, M, _, _), fn:contains(M, "cancelled while waiting for slot").

# Scheduler stop events
Decl scheduler_stopped(time: num, message: name).
scheduler_stopped(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "scheduler stopped").

# API slot timeout
Decl slot_acquire_timeout(time: num, message: name).
slot_acquire_timeout(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "failed to acquire API slot").

# Checkpoint events in scheduler
Decl scheduler_checkpoint(time: num, message: name).
scheduler_checkpoint(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "checkpoint").

# Retry with slot release
Decl retry_with_slot_release(time: num, message: name).
retry_with_slot_release(T, M) :- log_entry(T, /shards, _, M, _, _), fn:contains(M, "retrying after error").

# =============================================================================
# API SCHEDULER FAILURE MODES
# =============================================================================

# 76. Slot leak (ActiveSlots > 0 when idle)
Decl api_scheduler_slot_leak(time: num, message: name).
api_scheduler_slot_leak(T, M) :- slot_leak_warning(T, M).

# 77. Double release
Decl api_scheduler_double_release(time: num, message: name).
api_scheduler_double_release(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "released slot it didn't hold").

# 78. Wait queue leak
Decl api_scheduler_wait_queue_leak(time: num, message: name).
api_scheduler_wait_queue_leak(T, M) :- log_entry(T, /shards, /warn, M, _, _), fn:contains(M, "wait queue").

# 79. Slot starvation (long waits)
Decl api_scheduler_starvation(time: num, message: name).
api_scheduler_starvation(T, M) :- slot_acquired_after_wait(T, M), fn:contains(M, "after 30").
api_scheduler_starvation(T, M) :- slot_acquired_after_wait(T, M), fn:contains(M, "after 60").

# 80. Deadlock (scheduler stopped while shards waiting)
Decl api_scheduler_deadlock(time: num, message: name).
api_scheduler_deadlock(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "scheduler stopped").

# Add API scheduler failures to aggregation
any_failure_mode(T, /slot_leak, /api_scheduler, M) :- api_scheduler_slot_leak(T, M).
any_failure_mode(T, /double_release, /api_scheduler, M) :- api_scheduler_double_release(T, M).
any_failure_mode(T, /wait_queue_leak, /api_scheduler, M) :- api_scheduler_wait_queue_leak(T, M).
any_failure_mode(T, /slot_starvation, /api_scheduler, M) :- api_scheduler_starvation(T, M).
any_failure_mode(T, /scheduler_deadlock, /api_scheduler, M) :- api_scheduler_deadlock(T, M).

# API scheduler health check
Decl api_scheduler_health(status: name, message: name).
api_scheduler_health(/initialized, "OK") :- scheduler_initialized(_, _).
api_scheduler_health(/slots_working, "OK") :- slot_acquired(_, _, _).
api_scheduler_health(/slots_releasing, "OK") :- slot_released(_, _, _).
api_scheduler_health(/no_leaks, "OK") :- scheduler_initialized(_, _), !slot_leak_warning(_, _).

# =============================================================================
# KERNEL STRESS QUERIES
# =============================================================================

# Kernel events
Decl kernel_event(time: num, level: name, message: name).
kernel_event(T, L, M) :- log_entry(T, /kernel, L, M, _, _).

# Kernel errors
Decl kernel_error(time: num, message: name).
kernel_error(T, M) :- log_entry(T, /kernel, /error, M, _, _).

# Derivation events
Decl derivation_event(time: num, message: name).
derivation_event(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "deriv").

# Fact operations
Decl fact_operation(time: num, message: name).
fact_operation(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "fact").

# =============================================================================
# CAMPAIGN STRESS QUERIES
# =============================================================================

# Campaign events
Decl campaign_event(time: num, level: name, message: name).
campaign_event(T, L, M) :- log_entry(T, /campaign, L, M, _, _).

# Phase completion
Decl phase_complete(time: num, message: name).
phase_complete(T, M) :- log_entry(T, /campaign, _, M, _, _), fn:contains(M, "phase").

# Checkpoint events
Decl checkpoint_event(time: num, message: name).
checkpoint_event(T, M) :- log_entry(T, /campaign, _, M, _, _), fn:contains(M, "checkpoint").

# =============================================================================
# AUTOPOIESIS QUERIES
# =============================================================================

# Tool generation events
Decl tool_generation(time: num, message: name).
tool_generation(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), fn:contains(M, "generat").

# Tool compilation events
Decl tool_compilation(time: num, message: name).
tool_compilation(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), fn:contains(M, "compil").

# Ouroboros events
Decl ouroboros_event(time: num, message: name).
ouroboros_event(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), fn:contains(M, "ouroboros").

# Thunderdome events
Decl thunderdome_event(time: num, message: name).
thunderdome_event(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), fn:contains(M, "thunderdome").

# =============================================================================
# MANGLE SELF-HEALING QUERIES
# =============================================================================

# -----------------------------------------------------------------------------
# 1. INFINITE LOOP DETECTION
# -----------------------------------------------------------------------------

# Timeout detected events
Decl timeout_detected(time: num, category: name, message: name).
timeout_detected(T, C, M) :- log_entry(T, C, _, M, _, _), fn:contains(M, "timeout").

# Gas limit hit (already defined above, moving here for organization)
# gas_limit_hit(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "gas").

# Recursion exceeded events
Decl recursion_exceeded(time: num, message: name).
recursion_exceeded(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "recursion").

# Derivation explosion detection
Decl derivation_explosion(time: num, message: name).
derivation_explosion(T, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "derivation").

# Cyclic rule detection
Decl cyclic_rule_detected(time: num, message: name).
cyclic_rule_detected(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "cycle").

# Stratification violation
Decl stratification_error(time: num, message: name).
stratification_error(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "stratif").

# -----------------------------------------------------------------------------
# 2. JIT REPAIR TRACKING
# -----------------------------------------------------------------------------

# JIT repair triggered by error type
Decl jit_repair_triggered(time: num, error_type: name, message: name).
jit_repair_triggered(T, /undeclared_predicate, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair"), fn:contains(M, "undeclared").
jit_repair_triggered(T, /syntax_error, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair"), fn:contains(M, "syntax").
jit_repair_triggered(T, /safety_violation, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair"), fn:contains(M, "safety").
jit_repair_triggered(T, /stratification, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair"), fn:contains(M, "stratif").
jit_repair_triggered(T, /type_error, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair"), fn:contains(M, "type").

# Repair attempt count tracking
Decl repair_attempt_count(time: num, attempt_num: num, message: name).
repair_attempt_count(T, 1, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair attempt 1").
repair_attempt_count(T, 2, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair attempt 2").
repair_attempt_count(T, 3, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair attempt 3").

# Repair success rate tracking
Decl repair_success(time: num, message: name).
repair_success(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair successful").

Decl repair_failure(time: num, message: name).
repair_failure(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair failed").

Decl repair_max_retries_exceeded(time: num, message: name).
repair_max_retries_exceeded(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "max retries").

# -----------------------------------------------------------------------------
# 3. FILE WATCHER EVENTS
# -----------------------------------------------------------------------------

# File change detected
Decl file_change_detected(time: num, message: name).
file_change_detected(T, M) :- log_entry(T, /world, _, M, _, _), fn:contains(M, "file change").

# Validation triggered by file watcher
Decl validation_triggered(time: num, message: name).
validation_triggered(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "validation triggered").

# Repair on save
Decl repair_on_save(time: num, message: name).
repair_on_save(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair on save").

# File watcher errors
Decl file_watcher_error(time: num, message: name).
file_watcher_error(T, M) :- log_entry(T, /world, /error, M, _, _), fn:contains(M, "watcher").

# -----------------------------------------------------------------------------
# 4. STARTUP VALIDATION
# -----------------------------------------------------------------------------

# Startup validation result
Decl startup_validation_result(time: num, status: name, message: name).
startup_validation_result(T, /pass, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "startup validation passed").
startup_validation_result(T, /fail, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "startup validation failed").

# Invalid rules found at startup
Decl invalid_rules_found(time: num, count: num, message: name).
invalid_rules_found(T, 1, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "invalid rule").

# Commented rules detected
Decl commented_rules_detected(time: num, message: name).
commented_rules_detected(T, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "commented").

# Previously healed count
Decl previously_healed_count(time: num, message: name).
previously_healed_count(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "previously healed").

# Startup kernel boot
Decl kernel_boot(time: num, message: name).
kernel_boot(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "kernel initialized").

# Startup boot failure
Decl boot_failure(time: num, message: name).
boot_failure(T, M) :- log_entry(T, /boot, /error, M, _, _).

# -----------------------------------------------------------------------------
# 5. BUDGET TRACKING
# -----------------------------------------------------------------------------

# Budget remaining
Decl budget_remaining(time: num, message: name).
budget_remaining(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "budget remaining").

# Budget exhausted
Decl budget_exhausted(time: num, message: name).
budget_exhausted(T, M) :- log_entry(T, /system_shards, /warn, M, _, _), fn:contains(M, "budget exhausted").

# Repair cost per rule
Decl repair_cost(time: num, message: name).
repair_cost(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair cost").

# Token usage tracking
Decl token_usage(time: num, message: name).
token_usage(T, M) :- log_entry(T, /api, _, M, _, _), fn:contains(M, "tokens").

# -----------------------------------------------------------------------------
# 6. CORPUS USAGE
# -----------------------------------------------------------------------------

# PredicateCorpus loading events
Decl corpus_loaded(time: num, message: name).
corpus_loaded(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "corpus loaded").

# Corpus predicates used
Decl corpus_predicates_used(time: num, count: num, message: name).
corpus_predicates_used(T, 1, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "predicates used").

# Selector relevance score
Decl selector_relevance_score(time: num, message: name).
selector_relevance_score(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "relevance score").

# Corpus validation events
Decl corpus_validation(time: num, message: name).
corpus_validation(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "check-mangle").

# Corpus query failure
Decl corpus_query_failure(time: num, message: name).
corpus_query_failure(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "corpus query").

# Corpus statistics
Decl corpus_stats(time: num, message: name).
corpus_stats(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "corpus stats").

# -----------------------------------------------------------------------------
# 7. EXISTING SELF-HEALING QUERIES (preserved)
# -----------------------------------------------------------------------------

# MangleRepairShard activity
Decl repair_shard_event(time: num, message: name).
repair_shard_event(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "MangleRepair").

# Repair attempts
Decl repair_attempt(time: num, message: name).
repair_attempt(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repair attempt").

# Rule validation errors
Decl validation_error(time: num, message: name).
validation_error(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "undeclared").

# Undefined predicate errors
Decl undefined_predicate(time: num, message: name).
undefined_predicate(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "undefined predicate").

# JIT predicate selection events
Decl jit_selection(time: num, message: name).
jit_selection(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "JIT selected").

# Predicate selection fallback
Decl selection_fallback(time: num, message: name).
selection_fallback(T, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "JIT selector failed").

# Rule rejection events
Decl rule_rejected(time: num, message: name).
rule_rejected(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "rejected").

# Schema drift detected
Decl schema_drift(time: num, message: name).
schema_drift(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "schema drift").

# Self-healing success
Decl healing_success(time: num, message: name).
healing_success(T, M) :- log_entry(T, /system_shards, _, M, _, _), fn:contains(M, "repaired successfully").

# Self-healing critical issues
Decl healing_critical(time: num, type: name, message: name).
healing_critical(T, /corpus_missing, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "corpus not available").
healing_critical(T, /validation_failed, M) :- validation_error(T, M).
healing_critical(T, /rule_rejected, M) :- rule_rejected(T, M).

# =============================================================================
# 69+ FAILURE MODE QUERIES
# =============================================================================

# -----------------------------------------------------------------------------
# KERNEL & CORE RUNTIME FAILURES (6 modes)
# -----------------------------------------------------------------------------

# 1. Panic on boot
Decl kernel_panic_on_boot(time: num, message: name).
kernel_panic_on_boot(T, M) :- log_entry(T, /boot, /error, M, _, _), fn:contains(M, "panic").
kernel_panic_on_boot(T, M) :- log_entry(T, /kernel, /error, M, _, _), fn:contains(M, "CRITICAL"), fn:contains(M, "boot").

# 2. Derivation explosion (already covered above as derivation_explosion)

# 3. Gas limit exceeded (already covered as gas_limit_hit)

# 4. Undeclared predicate (already covered as validation_error)

# 5. Queue overflow
Decl queue_overflow(time: num, message: name).
queue_overflow(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "ErrQueueFull").

# 6. Shard limit hit
Decl shard_limit_hit(time: num, message: name).
shard_limit_hit(T, M) :- log_entry(T, /shards, /warn, M, _, _), fn:contains(M, "max concurrent shards").

# -----------------------------------------------------------------------------
# SPAWN QUEUE FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 7. Fast fail on queue full
Decl spawn_fast_fail(time: num, message: name).
spawn_fast_fail(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "immediate rejection").

# 8. Priority inversion
Decl priority_inversion(time: num, message: name).
priority_inversion(T, M) :- log_entry(T, /shards, /warn, M, _, _), fn:contains(M, "priority").

# 9. Deadline expiration
Decl spawn_deadline_expired(time: num, message: name).
spawn_deadline_expired(T, M) :- log_entry(T, /shards, /error, M, _, _), fn:contains(M, "deadline").

# 10. Worker contention
Decl worker_contention(time: num, message: name).
worker_contention(T, M) :- log_entry(T, /shards, /warn, M, _, _), fn:contains(M, "worker").

# -----------------------------------------------------------------------------
# VIRTUAL STORE FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 11. Permission denied
Decl action_permission_denied(time: num, message: name).
action_permission_denied(T, M) :- log_entry(T, /virtual_store, /error, M, _, _), fn:contains(M, "permission denied").

# 12. Action timeout
Decl action_timeout(time: num, message: name).
action_timeout(T, M) :- log_entry(T, /virtual_store, /error, M, _, _), fn:contains(M, "timeout").

# 13. Tool not found
Decl tool_not_found(time: num, message: name).
tool_not_found(T, M) :- log_entry(T, /virtual_store, /error, M, _, _), fn:contains(M, "tool not found").

# 14. Action crash
Decl action_crash(time: num, message: name).
action_crash(T, M) :- log_entry(T, /virtual_store, /error, M, _, _), fn:contains(M, "crash").

# -----------------------------------------------------------------------------
# LIMITS ENFORCER FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 15. Memory exceeded
Decl memory_exceeded(time: num, message: name).
memory_exceeded(T, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "memory exceeded").

# 16. Session timeout
Decl session_timeout(time: num, message: name).
session_timeout(T, M) :- log_entry(T, /session, /warn, M, _, _), fn:contains(M, "session timeout").

# 17. Shard limit (already covered as shard_limit_hit)

# -----------------------------------------------------------------------------
# PERCEPTION LAYER FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 18. Parse failure
Decl intent_parse_failure(time: num, message: name).
intent_parse_failure(T, M) :- log_entry(T, /perception, /error, M, _, _), fn:contains(M, "parse").

# 19. Piggyback corruption
Decl piggyback_corruption(time: num, message: name).
piggyback_corruption(T, M) :- log_entry(T, /articulation, /error, M, _, _), fn:contains(M, "truncated").

# 20. Taxonomy miss
Decl taxonomy_miss(time: num, message: name).
taxonomy_miss(T, M) :- log_entry(T, /perception, /warn, M, _, _), fn:contains(M, "unknown verb").

# 21. LLM timeout
Decl llm_timeout(time: num, message: name).
llm_timeout(T, M) :- log_entry(T, /api, /error, M, _, _), fn:contains(M, "timeout").

# -----------------------------------------------------------------------------
# LLM CLIENT FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 22. API key missing
Decl api_key_missing(time: num, message: name).
api_key_missing(T, M) :- log_entry(T, /api, /error, M, _, _), fn:contains(M, "API key").

# 23. Rate limit (already covered as rate_limit_hit)

# 24. Provider mismatch
Decl provider_mismatch(time: num, message: name).
provider_mismatch(T, M) :- log_entry(T, /api, /error, M, _, _), fn:contains(M, "auth").

# 25. API timeout (same as llm_timeout)

# -----------------------------------------------------------------------------
# ARTICULATION LAYER FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 26. Premature articulation
Decl premature_articulation(time: num, message: name).
premature_articulation(T, M) :- log_entry(T, /articulation, /warn, M, _, _), fn:contains(M, "premature").

# 27. Truncated JSON (same as piggyback_corruption)

# 28. Invalid ControlPacket
Decl invalid_control_packet(time: num, message: name).
invalid_control_packet(T, M) :- log_entry(T, /articulation, /error, M, _, _), fn:contains(M, "invalid").

# 29. Memory exhaustion from huge response
Decl response_memory_exhaustion(time: num, message: name).
response_memory_exhaustion(T, M) :- log_entry(T, /articulation, /error, M, _, _), fn:contains(M, "out of memory").

# -----------------------------------------------------------------------------
# CODER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 30. Edit atomicity failure
Decl edit_atomicity_failure(time: num, message: name).
edit_atomicity_failure(T, M) :- log_entry(T, /coder, /error, M, _, _), fn:contains(M, "atomicity").

# 31. Build timeout
Decl build_timeout(time: num, message: name).
build_timeout(T, M) :- log_entry(T, /coder, /error, M, _, _), fn:contains(M, "build timeout").

# 32. Language detection failure
Decl language_detection_failure(time: num, message: name).
language_detection_failure(T, M) :- log_entry(T, /coder, /warn, M, _, _), fn:contains(M, "language").

# 33. Parallel edit race
Decl parallel_edit_race(time: num, message: name).
parallel_edit_race(T, M) :- log_entry(T, /coder, /error, M, _, _), fn:contains(M, "race").

# -----------------------------------------------------------------------------
# TESTER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 34. Framework detection failure
Decl framework_detection_failure(time: num, message: name).
framework_detection_failure(T, M) :- log_entry(T, /tester, /warn, M, _, _), fn:contains(M, "framework").

# 35. Test timeout
Decl test_timeout(time: num, message: name).
test_timeout(T, M) :- log_entry(T, /tester, /error, M, _, _), fn:contains(M, "timeout").

# 36. Coverage parsing failure
Decl coverage_parsing_failure(time: num, message: name).
coverage_parsing_failure(T, M) :- log_entry(T, /tester, /error, M, _, _), fn:contains(M, "coverage").

# 37. TDD infinite loop
Decl tdd_infinite_loop(time: num, message: name).
tdd_infinite_loop(T, M) :- log_entry(T, /tester, /error, M, _, _), fn:contains(M, "infinite").

# -----------------------------------------------------------------------------
# REVIEWER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 38. Finding explosion
Decl finding_explosion(time: num, message: name).
finding_explosion(T, M) :- log_entry(T, /reviewer, /warn, M, _, _), fn:contains(M, "finding").

# 39. Custom rules error
Decl custom_rules_error(time: num, message: name).
custom_rules_error(T, M) :- log_entry(T, /reviewer, /error, M, _, _), fn:contains(M, "custom rules").

# 40. Specialist cascade
Decl specialist_cascade(time: num, message: name).
specialist_cascade(T, M) :- log_entry(T, /reviewer, /warn, M, _, _), fn:contains(M, "cascade").

# 41. Division by zero in complexity
Decl complexity_division_by_zero(time: num, message: name).
complexity_division_by_zero(T, M) :- log_entry(T, /reviewer, /error, M, _, _), fn:contains(M, "division").

# -----------------------------------------------------------------------------
# RESEARCHER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 42. HTML parsing bomb
Decl html_parsing_bomb(time: num, message: name).
html_parsing_bomb(T, M) :- log_entry(T, /researcher, /error, M, _, _), fn:contains(M, "parse").

# 43. Connection exhaustion
Decl connection_exhaustion(time: num, message: name).
connection_exhaustion(T, M) :- log_entry(T, /researcher, /error, M, _, _), fn:contains(M, "connection").

# 44. Context7 rate limit
Decl context7_rate_limit(time: num, message: name).
context7_rate_limit(T, M) :- log_entry(T, /researcher, /error, M, _, _), fn:contains(M, "429").

# 45. Domain filter bypass
Decl domain_filter_bypass(time: num, message: name).
domain_filter_bypass(T, M) :- log_entry(T, /researcher, /warn, M, _, _), fn:contains(M, "domain").

# -----------------------------------------------------------------------------
# NEMESIS SHARD FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 46. Attack explosion
Decl attack_explosion(time: num, message: name).
attack_explosion(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), fn:contains(M, "attack").

# 47. Tool nesting
Decl tool_nesting(time: num, message: name).
tool_nesting(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), fn:contains(M, "nesting").

# 48. Vulnerability DB growth
Decl vulnerability_db_growth(time: num, message: name).
vulnerability_db_growth(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), fn:contains(M, "vulnerability").

# -----------------------------------------------------------------------------
# OUROBOROS FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 49. Generation timeout
Decl tool_generation_timeout(time: num, message: name).
tool_generation_timeout(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), fn:contains(M, "generation timeout").

# 50. Safety bypass
Decl safety_bypass(time: num, message: name).
safety_bypass(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), fn:contains(M, "safety").

# 51. Compile failure
Decl tool_compile_failure(time: num, message: name).
tool_compile_failure(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), fn:contains(M, "compile").

# 52. Infinite nesting
Decl tool_infinite_nesting(time: num, message: name).
tool_infinite_nesting(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), fn:contains(M, "infinite").

# -----------------------------------------------------------------------------
# THUNDERDOME FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 53. Attack parallelization issue
Decl attack_parallelization(time: num, message: name).
attack_parallelization(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), fn:contains(M, "parallel").

# 54. Sandbox escape
Decl sandbox_escape(time: num, message: name).
sandbox_escape(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), fn:contains(M, "sandbox").

# 55. Artifact growth
Decl artifact_growth(time: num, message: name).
artifact_growth(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), fn:contains(M, "artifact").

# 56. Incomplete execution
Decl thunderdome_incomplete(time: num, message: name).
thunderdome_incomplete(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), fn:contains(M, "TODO").

# -----------------------------------------------------------------------------
# CAMPAIGN FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 57. Decomposition explosion
Decl decomposition_explosion(time: num, message: name).
decomposition_explosion(T, M) :- log_entry(T, /campaign, /warn, M, _, _), fn:contains(M, "1000").

# 58. Phase timeout
Decl phase_timeout(time: num, message: name).
phase_timeout(T, M) :- log_entry(T, /campaign, /error, M, _, _), fn:contains(M, "phase timeout").

# 59. Checkpoint corruption
Decl checkpoint_corruption(time: num, message: name).
checkpoint_corruption(T, M) :- log_entry(T, /campaign, /error, M, _, _), fn:contains(M, "checkpoint").

# 60. Context overflow
Decl campaign_context_overflow(time: num, message: name).
campaign_context_overflow(T, M) :- log_entry(T, /campaign, /error, M, _, _), fn:contains(M, "context overflow").

# -----------------------------------------------------------------------------
# WORLD MODEL FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 61. Symlink loop
Decl symlink_loop(time: num, message: name).
symlink_loop(T, M) :- log_entry(T, /world, /error, M, _, _), fn:contains(M, "symlink").

# 62. Permission denied during scan
Decl scan_permission_denied(time: num, message: name).
scan_permission_denied(T, M) :- log_entry(T, /world, /error, M, _, _), fn:contains(M, "permission").

# 63. Deep nesting
Decl deep_nesting(time: num, message: name).
deep_nesting(T, M) :- log_entry(T, /world, /warn, M, _, _), fn:contains(M, "depth").

# 64. Large file
Decl large_file_error(time: num, message: name).
large_file_error(T, M) :- log_entry(T, /world, /error, M, _, _), fn:contains(M, "large").

# -----------------------------------------------------------------------------
# HOLOGRAPHIC FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 65. Impact explosion
Decl impact_explosion(time: num, message: name).
impact_explosion(T, M) :- log_entry(T, /context, /warn, M, _, _), fn:contains(M, "impact").

# 66. Cycle in graph
Decl graph_cycle(time: num, message: name).
graph_cycle(T, M) :- log_entry(T, /context, /error, M, _, _), fn:contains(M, "cycle").

# 67. Holographic timeout
Decl holographic_timeout(time: num, message: name).
holographic_timeout(T, M) :- log_entry(T, /context, /error, M, _, _), fn:contains(M, "timeout").

# -----------------------------------------------------------------------------
# DREAM STATE FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 68. Consultant overload
Decl consultant_overload(time: num, message: name).
consultant_overload(T, M) :- log_entry(T, /dream, /error, M, _, _), fn:contains(M, "overload").

# 69. Dream queue overflow
Decl dream_queue_overflow(time: num, message: name).
dream_queue_overflow(T, M) :- log_entry(T, /dream, /error, M, _, _), fn:contains(M, "queue").

# 70. Learning cascade
Decl learning_cascade(time: num, message: name).
learning_cascade(T, M) :- log_entry(T, /dream, /warn, M, _, _), fn:contains(M, "cascade").

# -----------------------------------------------------------------------------
# SHADOW MODE FAILURES (2 modes)
# -----------------------------------------------------------------------------

# 71. Simulation timeout
Decl simulation_timeout(time: num, message: name).
simulation_timeout(T, M) :- log_entry(T, /shadow, /error, M, _, _), fn:contains(M, "timeout").

# 72. Shadow kernel crash
Decl shadow_kernel_crash(time: num, message: name).
shadow_kernel_crash(T, M) :- log_entry(T, /shadow, /error, M, _, _), fn:contains(M, "crash").

# -----------------------------------------------------------------------------
# BROWSER FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 73. Browser process crash
Decl browser_process_crash(time: num, message: name).
browser_process_crash(T, M) :- log_entry(T, /browser, /error, M, _, _), fn:contains(M, "crash").

# 74. Browser connection timeout
Decl browser_connection_timeout(time: num, message: name).
browser_connection_timeout(T, M) :- log_entry(T, /browser, /error, M, _, _), fn:contains(M, "timeout").

# 75. DOM explosion
Decl dom_explosion(time: num, message: name).
dom_explosion(T, M) :- log_entry(T, /browser, /warn, M, _, _), fn:contains(M, "DOM").

# -----------------------------------------------------------------------------
# ALL FAILURE MODES AGGREGATION
# -----------------------------------------------------------------------------

# Comprehensive failure mode detection
Decl any_failure_mode(time: num, mode: name, category: name, message: name).

# Kernel failures
any_failure_mode(T, /kernel_panic_on_boot, /kernel, M) :- kernel_panic_on_boot(T, M).
any_failure_mode(T, /derivation_explosion, /kernel, M) :- derivation_explosion(T, M).
any_failure_mode(T, /gas_limit_hit, /kernel, M) :- gas_limit_hit(T, M).
any_failure_mode(T, /undeclared_predicate, /kernel, M) :- validation_error(T, M).
any_failure_mode(T, /queue_overflow, /shards, M) :- queue_overflow(T, M).
any_failure_mode(T, /shard_limit_hit, /shards, M) :- shard_limit_hit(T, M).

# Spawn queue failures
any_failure_mode(T, /spawn_fast_fail, /shards, M) :- spawn_fast_fail(T, M).
any_failure_mode(T, /priority_inversion, /shards, M) :- priority_inversion(T, M).
any_failure_mode(T, /spawn_deadline_expired, /shards, M) :- spawn_deadline_expired(T, M).
any_failure_mode(T, /worker_contention, /shards, M) :- worker_contention(T, M).

# Virtual store failures
any_failure_mode(T, /action_permission_denied, /virtual_store, M) :- action_permission_denied(T, M).
any_failure_mode(T, /action_timeout, /virtual_store, M) :- action_timeout(T, M).
any_failure_mode(T, /tool_not_found, /virtual_store, M) :- tool_not_found(T, M).
any_failure_mode(T, /action_crash, /virtual_store, M) :- action_crash(T, M).

# Perception failures
any_failure_mode(T, /intent_parse_failure, /perception, M) :- intent_parse_failure(T, M).
any_failure_mode(T, /piggyback_corruption, /articulation, M) :- piggyback_corruption(T, M).
any_failure_mode(T, /taxonomy_miss, /perception, M) :- taxonomy_miss(T, M).
any_failure_mode(T, /llm_timeout, /api, M) :- llm_timeout(T, M).

# Add all other failure modes to the aggregation...
any_failure_mode(T, /browser_crash, /browser, M) :- browser_process_crash(T, M).
any_failure_mode(T, /dom_explosion, /browser, M) :- dom_explosion(T, M).
any_failure_mode(T, /shadow_crash, /shadow, M) :- shadow_kernel_crash(T, M).

# =============================================================================
# AGGREGATION QUERIES (require fn:count)
# =============================================================================

# Error count by category
# Decl error_count_by_category(category: name, count: num).
# error_count_by_category(C, N) :- error_entry(_, C, _) |> do fn:group_by(C), let N = fn:count().

# =============================================================================
# SELF-HEALING SPECIFIC AGGREGATIONS
# =============================================================================

# All repair events (success + failure)
Decl all_repair_events(time: num, status: name, message: name).
all_repair_events(T, /success, M) :- repair_success(T, M).
all_repair_events(T, /failure, M) :- repair_failure(T, M).
all_repair_events(T, /max_retries, M) :- repair_max_retries_exceeded(T, M).

# JIT system health
Decl jit_system_health(time: num, status: name, message: name).
jit_system_health(T, /corpus_loaded, M) :- corpus_loaded(T, M).
jit_system_health(T, /selection_ok, M) :- jit_selection(T, M).
jit_system_health(T, /selection_fallback, M) :- selection_fallback(T, M).
jit_system_health(T, /corpus_missing, M) :- log_entry(T, /kernel, /warn, M, _, _), fn:contains(M, "corpus not available").

# Validation pipeline events
Decl validation_pipeline(time: num, stage: name, result: name, message: name).
validation_pipeline(T, /corpus_validation, /pass, M) :- corpus_validation(T, M), fn:contains(M, "OK").
validation_pipeline(T, /corpus_validation, /fail, M) :- corpus_validation(T, M), fn:contains(M, "error").
validation_pipeline(T, /startup_validation, /pass, M) :- startup_validation_result(T, /pass, M).
validation_pipeline(T, /startup_validation, /fail, M) :- startup_validation_result(T, /fail, M).
validation_pipeline(T, /runtime_validation, /fail, M) :- validation_error(T, M).

# Repair error type breakdown
Decl repair_by_error_type(time: num, error_type: name, message: name).
repair_by_error_type(T, ET, M) :- jit_repair_triggered(T, ET, M).

# Self-healing effectiveness metrics
Decl healing_metric(time: num, metric: name, message: name).
healing_metric(T, /repair_triggered, M) :- repair_attempt(T, M).
healing_metric(T, /repair_succeeded, M) :- repair_success(T, M).
healing_metric(T, /repair_failed, M) :- repair_failure(T, M).
healing_metric(T, /validation_error, M) :- validation_error(T, M).
healing_metric(T, /rule_rejected, M) :- rule_rejected(T, M).

# =============================================================================
# FAILURE MODE SUMMARY QUERIES
# =============================================================================

# Critical failures (system-breaking)
Decl critical_failure(time: num, mode: name, message: name).
critical_failure(T, M, Msg) :- kernel_panic_on_boot(T, Msg), M = /kernel_panic.
critical_failure(T, M, Msg) :- boot_failure(T, Msg), M = /boot_failure.
critical_failure(T, M, Msg) :- oom_event(T, _, Msg), M = /oom.
critical_failure(T, M, Msg) :- nil_pointer_error(T, _, Msg), M = /nil_pointer.
critical_failure(T, M, Msg) :- shadow_kernel_crash(T, Msg), M = /shadow_crash.
critical_failure(T, M, Msg) :- browser_process_crash(T, Msg), M = /browser_crash.

# High-severity failures (feature-breaking, recoverable)
Decl high_severity_failure(time: num, mode: name, message: name).
high_severity_failure(T, M, Msg) :- queue_overflow(T, Msg), M = /queue_overflow.
high_severity_failure(T, M, Msg) :- gas_limit_hit(T, Msg), M = /gas_limit.
high_severity_failure(T, M, Msg) :- derivation_explosion(T, Msg), M = /derivation_explosion.
high_severity_failure(T, M, Msg) :- action_crash(T, Msg), M = /action_crash.
high_severity_failure(T, M, Msg) :- tool_compile_failure(T, Msg), M = /tool_compile_failure.
high_severity_failure(T, M, Msg) :- checkpoint_corruption(T, Msg), M = /checkpoint_corruption.

# Medium-severity failures (degraded performance)
Decl medium_severity_failure(time: num, mode: name, message: name).
medium_severity_failure(T, M, Msg) :- shard_limit_hit(T, Msg), M = /shard_limit.
medium_severity_failure(T, M, Msg) :- timeout_detected(T, _, Msg), M = /timeout.
medium_severity_failure(T, M, Msg) :- memory_exceeded(T, Msg), M = /memory_exceeded.
medium_severity_failure(T, M, Msg) :- budget_exhausted(T, Msg), M = /budget_exhausted.
medium_severity_failure(T, M, Msg) :- rate_limit_hit(T, Msg), M = /rate_limit.

# Low-severity failures (warnings, fallbacks work)
Decl low_severity_failure(time: num, mode: name, message: name).
low_severity_failure(T, M, Msg) :- taxonomy_miss(T, Msg), M = /taxonomy_miss.
low_severity_failure(T, M, Msg) :- selection_fallback(T, Msg), M = /jit_fallback.
low_severity_failure(T, M, Msg) :- language_detection_failure(T, Msg), M = /language_detection.
low_severity_failure(T, M, Msg) :- framework_detection_failure(T, Msg), M = /framework_detection.

# All failures by severity
Decl failure_by_severity(time: num, severity: name, mode: name, message: name).
failure_by_severity(T, /critical, M, Msg) :- critical_failure(T, M, Msg).
failure_by_severity(T, /high, M, Msg) :- high_severity_failure(T, M, Msg).
failure_by_severity(T, /medium, M, Msg) :- medium_severity_failure(T, M, Msg).
failure_by_severity(T, /low, M, Msg) :- low_severity_failure(T, M, Msg).

# =============================================================================
# STRESS TEST SUCCESS CRITERIA
# =============================================================================

# A stress test passes if:
# 1. No panics: panic_detected should return empty
# 2. No OOM: oom_event should return empty
# 3. Limited errors: error_entry count within threshold
# 4. No deadlocks: timeout_event count within threshold
# 5. Shards complete: shard_spawned count == shard_completed count (approx)
# 6. Self-healing active: corpus_loaded and repair_shard_event present
# 7. No critical failures: critical_failure should return empty

# Query all critical issues (expanded):
Decl critical_issue(time: num, type: name, message: name).
critical_issue(T, /panic, M) :- panic_detected(T, _, M).
critical_issue(T, /oom, M) :- oom_event(T, _, M).
critical_issue(T, /nil_pointer, M) :- nil_pointer_error(T, _, M).
critical_issue(T, /deadlock, M) :- deadline_exceeded(T, _, M).
critical_issue(T, /boot_failure, M) :- boot_failure(T, M).
critical_issue(T, /kernel_panic, M) :- kernel_panic_on_boot(T, M).
critical_issue(T, /shadow_crash, M) :- shadow_kernel_crash(T, M).
critical_issue(T, /corpus_missing, M) :- healing_critical(T, /corpus_missing, M).

# Self-healing system health check
Decl self_healing_health(status: name, message: name).
self_healing_health(/corpus_loaded, "OK") :- corpus_loaded(_, _).
self_healing_health(/repair_shard_active, "OK") :- repair_shard_event(_, _).
self_healing_health(/jit_selection_working, "OK") :- jit_selection(_, _).
self_healing_health(/validation_active, "OK") :- corpus_validation(_, _).

# Overall stress test health
Decl stress_test_health(category: name, status: name, count: num).
# Note: These would need aggregation support to count properly
# stress_test_health(/critical_issues, /fail, Count) :- critical_issue(_, _, _) |> do fn:count(), let Count = ...
# For now, existence checks:
stress_test_health(/has_critical_issues, /fail, 1) :- critical_issue(_, _, _).
stress_test_health(/has_panics, /fail, 1) :- panic_detected(_, _, _).
stress_test_health(/has_oom, /fail, 1) :- oom_event(_, _, _).
stress_test_health(/corpus_available, /pass, 1) :- corpus_loaded(_, _).
stress_test_health(/repair_system_active, /pass, 1) :- repair_shard_event(_, _).
