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

Decl log_entry(Time, Category, Level, Message, File, Line) bound [/number, /name, /name, /name, /name, /number].

# =============================================================================
# ERROR DETECTION QUERIES
# =============================================================================

# All errors
Decl error_entry(Time, Category, Message) bound [/number, /name, /name].
error_entry(T, C, M) :- log_entry(T, C, /error, M, _, _).

# All warnings
Decl warning_entry(Time, Category, Message) bound [/number, /name, /name].
warning_entry(T, C, M) :- log_entry(T, C, /warn, M, _, _).

# Panics (message contains "panic")
Decl panic_detected(Time, Category, Message) bound [/number, /name, /name].
panic_detected(T, C, M) :- log_entry(T, C, _, M, _, _), :string:contains(M, "panic").

# Nil pointer errors
Decl nil_pointer_error(Time, Category, Message) bound [/number, /name, /name].
nil_pointer_error(T, C, M) :- log_entry(T, C, /error, M, _, _), :string:contains(M, "nil pointer").

# Out of memory events
Decl oom_event(Time, Category, Message) bound [/number, /name, /name].
oom_event(T, C, M) :- log_entry(T, C, /error, M, _, _), :string:contains(M, "out of memory").

# Timeout events
Decl timeout_event(Time, Category, Message) bound [/number, /name, /name].
timeout_event(T, C, M) :- log_entry(T, C, /error, M, _, _), :string:contains(M, "timeout").

# Deadline exceeded
Decl deadline_exceeded(Time, Category, Message) bound [/number, /name, /name].
deadline_exceeded(T, C, M) :- log_entry(T, C, /error, M, _, _), :string:contains(M, "deadline").

# =============================================================================
# RESOURCE STRESS QUERIES
# =============================================================================

# Memory warnings
Decl memory_warning(Time, Message) bound [/number, /name].
memory_warning(T, M) :- log_entry(T, _, /warn, M, _, _), :string:contains(M, "memory").

# Queue full events
Decl queue_full(Time, Message) bound [/number, /name].
queue_full(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "queue full").

# Limit exceeded events
Decl limit_exceeded(Time, Category, Message) bound [/number, /name, /name].
limit_exceeded(T, C, M) :- log_entry(T, C, /warn, M, _, _), :string:contains(M, "limit").

# Gas limit in Mangle
Decl gas_limit_hit(Time, Message) bound [/number, /name].
gas_limit_hit(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "gas").

# =============================================================================
# SHARD LIFECYCLE QUERIES
# =============================================================================

# Shard spawn events
Decl shard_spawned(Time, Message) bound [/number, /name].
shard_spawned(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "spawn").

# Shard completion events
Decl shard_completed(Time, Message) bound [/number, /name].
shard_completed(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "complet").

# Shard errors
Decl shard_error(Time, Message) bound [/number, /name].
shard_error(T, M) :- log_entry(T, /shards, /error, M, _, _).

# Shard timeout
Decl shard_timeout(Time, Message) bound [/number, /name].
shard_timeout(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "timeout").

# =============================================================================
# API STRESS QUERIES
# =============================================================================

# API call events
Decl api_call(Time, Message) bound [/number, /name].
api_call(T, M) :- log_entry(T, /api, _, M, _, _).

# API errors
Decl api_error(Time, Message) bound [/number, /name].
api_error(T, M) :- log_entry(T, /api, /error, M, _, _).

# Rate limit hits
Decl rate_limit_hit(Time, Message) bound [/number, /name].
rate_limit_hit(T, M) :- log_entry(T, /api, _, M, _, _), :string:contains(M, "rate").

# =============================================================================
# API SCHEDULER STRESS QUERIES
# =============================================================================

# Scheduler initialization
Decl scheduler_initialized(Time, Message) bound [/number, /name].
scheduler_initialized(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "APIScheduler: initialized").

# Shard registration with scheduler
Decl shard_registered(Time, Message) bound [/number, /name].
shard_registered(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "APIScheduler: registered shard").

# Shard unregistration
Decl shard_unregistered(Time, Message) bound [/number, /name].
shard_unregistered(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "APIScheduler: unregistered shard").

# Slot acquisition events
Decl slot_acquired(Time, SHardId, Message) bound [/number, /name, /name].
slot_acquired(T, /unknown, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "acquired slot").

# Slot release events
Decl slot_released(Time, SHardId, Message) bound [/number, /name, /name].
slot_released(T, /unknown, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "released slot").

# Slot wait events (contention detected)
Decl slot_wait(Time, SHardId, Message) bound [/number, /name, /name].
slot_wait(T, /unknown, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "waiting for slot").

# Slot acquisition after wait (shows wait duration)
Decl slot_acquired_after_wait(Time, Message) bound [/number, /name].
slot_acquired_after_wait(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "acquired slot after").

# Scheduler metrics in logs
Decl api_scheduler_metrics(Time, Message) bound [/number, /name].
api_scheduler_metrics(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "api_calls=").
api_scheduler_metrics(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "total_calls=").
api_scheduler_metrics(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "total_wait=").

# Slot contention (5 slots in use)
Decl slot_contention(Time, Message) bound [/number, /name].
slot_contention(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "active=5/5").

# Potential slot leak (release without acquire)
Decl slot_leak_warning(Time, Message) bound [/number, /name].
slot_leak_warning(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "released slot it didn't hold").

# Context cancellation during slot wait
Decl slot_wait_cancelled(Time, Message) bound [/number, /name].
slot_wait_cancelled(T, M) :- log_entry(T, /shards, /warn, M, _, _), :string:contains(M, "cancelled while waiting for slot").

# Scheduler stop events
Decl scheduler_stopped(Time, Message) bound [/number, /name].
scheduler_stopped(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "scheduler stopped").

# API slot timeout
Decl slot_acquire_timeout(Time, Message) bound [/number, /name].
slot_acquire_timeout(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "failed to acquire API slot").

# Checkpoint events in scheduler
Decl scheduler_checkpoint(Time, Message) bound [/number, /name].
scheduler_checkpoint(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "checkpoint").

# Retry with slot release
Decl retry_with_slot_release(Time, Message) bound [/number, /name].
retry_with_slot_release(T, M) :- log_entry(T, /shards, _, M, _, _), :string:contains(M, "retrying after error").

# =============================================================================
# API SCHEDULER FAILURE MODES
# =============================================================================

# 76. Slot leak (ActiveSlots > 0 when idle)
Decl api_scheduler_slot_leak(Time, Message) bound [/number, /name].
api_scheduler_slot_leak(T, M) :- slot_leak_warning(T, M).

# 77. Double release
Decl api_scheduler_double_release(Time, Message) bound [/number, /name].
api_scheduler_double_release(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "released slot it didn't hold").

# 78. Wait queue leak
Decl api_scheduler_wait_queue_leak(Time, Message) bound [/number, /name].
api_scheduler_wait_queue_leak(T, M) :- log_entry(T, /shards, /warn, M, _, _), :string:contains(M, "wait queue").

# 79. Slot starvation (long waits)
Decl api_scheduler_starvation(Time, Message) bound [/number, /name].
api_scheduler_starvation(T, M) :- slot_acquired_after_wait(T, M), :string:contains(M, "after 30").
api_scheduler_starvation(T, M) :- slot_acquired_after_wait(T, M), :string:contains(M, "after 60").

# 80. Deadlock (scheduler stopped while shards waiting)
Decl api_scheduler_deadlock(Time, Message) bound [/number, /name].
api_scheduler_deadlock(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "scheduler stopped").

# Add API scheduler failures to aggregation
any_failure_mode(T, /SlotLeak, /ApiScheduler, M) :- api_scheduler_slot_leak(T, M).
any_failure_mode(T, /DoubleRelease, /ApiScheduler, M) :- api_scheduler_double_release(T, M).
any_failure_mode(T, /WaitQueueLeak, /ApiScheduler, M) :- api_scheduler_wait_queue_leak(T, M).
any_failure_mode(T, /SlotStarvation, /ApiScheduler, M) :- api_scheduler_starvation(T, M).
any_failure_mode(T, /SchedulerDeadlock, /ApiScheduler, M) :- api_scheduler_deadlock(T, M).

# API scheduler health check
Decl api_scheduler_health(Status, Message) bound [/name, /name].
api_scheduler_health(/initialized, "OK") :- scheduler_initialized(_, _).
api_scheduler_health(/SlotsWorking, "OK") :- slot_acquired(_, _, _).
api_scheduler_health(/SlotsReleasing, "OK") :- slot_released(_, _, _).
api_scheduler_health(/NoLeaks, "OK") :- scheduler_initialized(_, _), !slot_leak_warning(_, _).

# =============================================================================
# KERNEL STRESS QUERIES
# =============================================================================

# Kernel events
Decl kernel_event(Time, Level, Message) bound [/number, /name, /name].
kernel_event(T, L, M) :- log_entry(T, /kernel, L, M, _, _).

# Kernel errors
Decl kernel_error(Time, Message) bound [/number, /name].
kernel_error(T, M) :- log_entry(T, /kernel, /error, M, _, _).

# Derivation events
Decl derivation_event(Time, Message) bound [/number, /name].
derivation_event(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "deriv").

# Fact operations
Decl fact_operation(Time, Message) bound [/number, /name].
fact_operation(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "fact").

# =============================================================================
# CAMPAIGN STRESS QUERIES
# =============================================================================

# Campaign events
Decl campaign_event(Time, Level, Message) bound [/number, /name, /name].
campaign_event(T, L, M) :- log_entry(T, /campaign, L, M, _, _).

# Phase completion
Decl phase_complete(Time, Message) bound [/number, /name].
phase_complete(T, M) :- log_entry(T, /campaign, _, M, _, _), :string:contains(M, "phase").

# Checkpoint events
Decl checkpoint_event(Time, Message) bound [/number, /name].
checkpoint_event(T, M) :- log_entry(T, /campaign, _, M, _, _), :string:contains(M, "checkpoint").

# =============================================================================
# AUTOPOIESIS QUERIES
# =============================================================================

# Tool generation events
Decl tool_generation(Time, Message) bound [/number, /name].
tool_generation(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), :string:contains(M, "generat").

# Tool compilation events
Decl tool_compilation(Time, Message) bound [/number, /name].
tool_compilation(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), :string:contains(M, "compil").

# Ouroboros events
Decl ouroboros_event(Time, Message) bound [/number, /name].
ouroboros_event(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), :string:contains(M, "ouroboros").

# Thunderdome events
Decl thunderdome_event(Time, Message) bound [/number, /name].
thunderdome_event(T, M) :- log_entry(T, /autopoiesis, _, M, _, _), :string:contains(M, "thunderdome").

# =============================================================================
# MANGLE SELF-HEALING QUERIES
# =============================================================================

# -----------------------------------------------------------------------------
# 1. INFINITE LOOP DETECTION
# -----------------------------------------------------------------------------

# Timeout detected events
Decl timeout_detected(Time, Category, Message) bound [/number, /name, /name].
timeout_detected(T, C, M) :- log_entry(T, C, _, M, _, _), :string:contains(M, "timeout").

# Gas limit hit (already defined above, moving here for organization)
# gas_limit_hit(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "gas").

# Recursion exceeded events
Decl recursion_exceeded(Time, Message) bound [/number, /name].
recursion_exceeded(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "recursion").

# Derivation explosion detection
Decl derivation_explosion(Time, Message) bound [/number, /name].
derivation_explosion(T, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "derivation").

# Cyclic rule detection
Decl cyclic_rule_detected(Time, Message) bound [/number, /name].
cyclic_rule_detected(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "cycle").

# Stratification violation
Decl stratification_error(Time, Message) bound [/number, /name].
stratification_error(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "stratif").

# -----------------------------------------------------------------------------
# 2. JIT REPAIR TRACKING
# -----------------------------------------------------------------------------

# JIT repair triggered by error type
Decl jit_repair_triggered(Time, ERrorType, Message) bound [/number, /name, /name].
jit_repair_triggered(T, /UndeclaredPredicate, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair"), :string:contains(M, "undeclared").
jit_repair_triggered(T, /SyntaxError, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair"), :string:contains(M, "syntax").
jit_repair_triggered(T, /SafetyViolation, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair"), :string:contains(M, "safety").
jit_repair_triggered(T, /stratification, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair"), :string:contains(M, "stratif").
jit_repair_triggered(T, /TypeError, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair"), :string:contains(M, "type").

# Repair attempt count tracking
Decl repair_attempt_count(Time, ATtemptNum, Message) bound [/number, /number, /name].
repair_attempt_count(T, 1, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair attempt 1").
repair_attempt_count(T, 2, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair attempt 2").
repair_attempt_count(T, 3, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair attempt 3").

# Repair success rate tracking
Decl repair_success(Time, Message) bound [/number, /name].
repair_success(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair successful").

Decl repair_failure(Time, Message) bound [/number, /name].
repair_failure(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair failed").

Decl repair_max_retries_exceeded(Time, Message) bound [/number, /name].
repair_max_retries_exceeded(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "max retries").

# -----------------------------------------------------------------------------
# 3. FILE WATCHER EVENTS
# -----------------------------------------------------------------------------

# File change detected
Decl file_change_detected(Time, Message) bound [/number, /name].
file_change_detected(T, M) :- log_entry(T, /world, _, M, _, _), :string:contains(M, "file change").

# Validation triggered by file watcher
Decl validation_triggered(Time, Message) bound [/number, /name].
validation_triggered(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "validation triggered").

# Repair on save
Decl repair_on_save(Time, Message) bound [/number, /name].
repair_on_save(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair on save").

# File watcher errors
Decl file_watcher_error(Time, Message) bound [/number, /name].
file_watcher_error(T, M) :- log_entry(T, /world, /error, M, _, _), :string:contains(M, "watcher").

# -----------------------------------------------------------------------------
# 4. STARTUP VALIDATION
# -----------------------------------------------------------------------------

# Startup validation result
Decl startup_validation_result(Time, Status, Message) bound [/number, /name, /name].
startup_validation_result(T, /pass, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "startup validation passed").
startup_validation_result(T, /fail, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "startup validation failed").

# Invalid rules found at startup
Decl invalid_rules_found(Time, Count, Message) bound [/number, /number, /name].
invalid_rules_found(T, 1, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "invalid rule").

# Commented rules detected
Decl commented_rules_detected(Time, Message) bound [/number, /name].
commented_rules_detected(T, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "commented").

# Previously healed count
Decl previously_healed_count(Time, Message) bound [/number, /name].
previously_healed_count(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "previously healed").

# Startup kernel boot
Decl kernel_boot(Time, Message) bound [/number, /name].
kernel_boot(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "kernel initialized").

# Startup boot failure
Decl boot_failure(Time, Message) bound [/number, /name].
boot_failure(T, M) :- log_entry(T, /boot, /error, M, _, _).

# -----------------------------------------------------------------------------
# 5. BUDGET TRACKING
# -----------------------------------------------------------------------------

# Budget remaining
Decl budget_remaining(Time, Message) bound [/number, /name].
budget_remaining(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "budget remaining").

# Budget exhausted
Decl budget_exhausted(Time, Message) bound [/number, /name].
budget_exhausted(T, M) :- log_entry(T, /SystemShards, /warn, M, _, _), :string:contains(M, "budget exhausted").

# Repair cost per rule
Decl repair_cost(Time, Message) bound [/number, /name].
repair_cost(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair cost").

# Token usage tracking
Decl token_usage(Time, Message) bound [/number, /name].
token_usage(T, M) :- log_entry(T, /api, _, M, _, _), :string:contains(M, "tokens").

# -----------------------------------------------------------------------------
# 6. CORPUS USAGE
# -----------------------------------------------------------------------------

# PredicateCorpus loading events
Decl corpus_loaded(Time, Message) bound [/number, /name].
corpus_loaded(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "corpus loaded").

# Corpus predicates used
Decl corpus_predicates_used(Time, Count, Message) bound [/number, /number, /name].
corpus_predicates_used(T, 1, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "predicates used").

# Selector relevance score
Decl selector_relevance_score(Time, Message) bound [/number, /name].
selector_relevance_score(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "relevance score").

# Corpus validation events
Decl corpus_validation(Time, Message) bound [/number, /name].
corpus_validation(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "check-mangle").

# Corpus query failure
Decl corpus_query_failure(Time, Message) bound [/number, /name].
corpus_query_failure(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "corpus query").

# Corpus statistics
Decl corpus_stats(Time, Message) bound [/number, /name].
corpus_stats(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "corpus stats").

# -----------------------------------------------------------------------------
# 7. EXISTING SELF-HEALING QUERIES (preserved)
# -----------------------------------------------------------------------------

# MangleRepairShard activity
Decl repair_shard_event(Time, Message) bound [/number, /name].
repair_shard_event(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "MangleRepair").

# Repair attempts
Decl repair_attempt(Time, Message) bound [/number, /name].
repair_attempt(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repair attempt").

# Rule validation errors
Decl validation_error(Time, Message) bound [/number, /name].
validation_error(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "undeclared").

# Undefined predicate errors
Decl undefined_predicate(Time, Message) bound [/number, /name].
undefined_predicate(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "undefined predicate").

# JIT predicate selection events
Decl jit_selection(Time, Message) bound [/number, /name].
jit_selection(T, M) :- log_entry(T, /kernel, _, M, _, _), :string:contains(M, "JIT selected").

# Predicate selection fallback
Decl selection_fallback(Time, Message) bound [/number, /name].
selection_fallback(T, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "JIT selector failed").

# Rule rejection events
Decl rule_rejected(Time, Message) bound [/number, /name].
rule_rejected(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "rejected").

# Schema drift detected
Decl schema_drift(Time, Message) bound [/number, /name].
schema_drift(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "schema drift").

# Self-healing success
Decl healing_success(Time, Message) bound [/number, /name].
healing_success(T, M) :- log_entry(T, /SystemShards, _, M, _, _), :string:contains(M, "repaired successfully").

# Self-healing critical issues
Decl healing_critical(Time, Type, Message) bound [/number, /name, /name].
healing_critical(T, /CorpusMissing, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "corpus not available").
healing_critical(T, /ValidationFailed, M) :- validation_error(T, M).
healing_critical(T, /RuleRejected, M) :- rule_rejected(T, M).

# =============================================================================
# 69+ FAILURE MODE QUERIES
# =============================================================================

# -----------------------------------------------------------------------------
# KERNEL & CORE RUNTIME FAILURES (6 modes)
# -----------------------------------------------------------------------------

# 1. Panic on boot
Decl kernel_panic_on_boot(Time, Message) bound [/number, /name].
kernel_panic_on_boot(T, M) :- log_entry(T, /boot, /error, M, _, _), :string:contains(M, "panic").
kernel_panic_on_boot(T, M) :- log_entry(T, /kernel, /error, M, _, _), :string:contains(M, "CRITICAL"), :string:contains(M, "boot").

# 2. Derivation explosion (already covered above as derivation_explosion)

# 3. Gas limit exceeded (already covered as gas_limit_hit)

# 4. Undeclared predicate (already covered as validation_error)

# 5. Queue overflow
Decl queue_overflow(Time, Message) bound [/number, /name].
queue_overflow(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "ErrQueueFull").

# 6. Shard limit hit
Decl shard_limit_hit(Time, Message) bound [/number, /name].
shard_limit_hit(T, M) :- log_entry(T, /shards, /warn, M, _, _), :string:contains(M, "max concurrent shards").

# -----------------------------------------------------------------------------
# SPAWN QUEUE FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 7. Fast fail on queue full
Decl spawn_fast_fail(Time, Message) bound [/number, /name].
spawn_fast_fail(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "immediate rejection").

# 8. Priority inversion
Decl priority_inversion(Time, Message) bound [/number, /name].
priority_inversion(T, M) :- log_entry(T, /shards, /warn, M, _, _), :string:contains(M, "priority").

# 9. Deadline expiration
Decl spawn_deadline_expired(Time, Message) bound [/number, /name].
spawn_deadline_expired(T, M) :- log_entry(T, /shards, /error, M, _, _), :string:contains(M, "deadline").

# 10. Worker contention
Decl worker_contention(Time, Message) bound [/number, /name].
worker_contention(T, M) :- log_entry(T, /shards, /warn, M, _, _), :string:contains(M, "worker").

# -----------------------------------------------------------------------------
# VIRTUAL STORE FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 11. Permission denied
Decl action_permission_denied(Time, Message) bound [/number, /name].
action_permission_denied(T, M) :- log_entry(T, /VirtualStore, /error, M, _, _), :string:contains(M, "permission denied").

# 12. Action timeout
Decl action_timeout(Time, Message) bound [/number, /name].
action_timeout(T, M) :- log_entry(T, /VirtualStore, /error, M, _, _), :string:contains(M, "timeout").

# 13. Tool not found
Decl tool_not_found(Time, Message) bound [/number, /name].
tool_not_found(T, M) :- log_entry(T, /VirtualStore, /error, M, _, _), :string:contains(M, "tool not found").

# 14. Action crash
Decl action_crash(Time, Message) bound [/number, /name].
action_crash(T, M) :- log_entry(T, /VirtualStore, /error, M, _, _), :string:contains(M, "crash").

# -----------------------------------------------------------------------------
# LIMITS ENFORCER FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 15. Memory exceeded
Decl memory_exceeded(Time, Message) bound [/number, /name].
memory_exceeded(T, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "memory exceeded").

# 16. Session timeout
Decl session_timeout(Time, Message) bound [/number, /name].
session_timeout(T, M) :- log_entry(T, /session, /warn, M, _, _), :string:contains(M, "session timeout").

# 17. Shard limit (already covered as shard_limit_hit)

# -----------------------------------------------------------------------------
# PERCEPTION LAYER FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 18. Parse failure
Decl intent_parse_failure(Time, Message) bound [/number, /name].
intent_parse_failure(T, M) :- log_entry(T, /perception, /error, M, _, _), :string:contains(M, "parse").

# 19. Piggyback corruption
Decl piggyback_corruption(Time, Message) bound [/number, /name].
piggyback_corruption(T, M) :- log_entry(T, /articulation, /error, M, _, _), :string:contains(M, "truncated").

# 20. Taxonomy miss
Decl taxonomy_miss(Time, Message) bound [/number, /name].
taxonomy_miss(T, M) :- log_entry(T, /perception, /warn, M, _, _), :string:contains(M, "unknown verb").

# 21. LLM timeout
Decl llm_timeout(Time, Message) bound [/number, /name].
llm_timeout(T, M) :- log_entry(T, /api, /error, M, _, _), :string:contains(M, "timeout").

# -----------------------------------------------------------------------------
# LLM CLIENT FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 22. API key missing
Decl api_key_missing(Time, Message) bound [/number, /name].
api_key_missing(T, M) :- log_entry(T, /api, /error, M, _, _), :string:contains(M, "API key").

# 23. Rate limit (already covered as rate_limit_hit)

# 24. Provider mismatch
Decl provider_mismatch(Time, Message) bound [/number, /name].
provider_mismatch(T, M) :- log_entry(T, /api, /error, M, _, _), :string:contains(M, "auth").

# 25. API timeout (same as llm_timeout)

# -----------------------------------------------------------------------------
# ARTICULATION LAYER FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 26. Premature articulation
Decl premature_articulation(Time, Message) bound [/number, /name].
premature_articulation(T, M) :- log_entry(T, /articulation, /warn, M, _, _), :string:contains(M, "premature").

# 27. Truncated JSON (same as piggyback_corruption)

# 28. Invalid ControlPacket
Decl invalid_control_packet(Time, Message) bound [/number, /name].
invalid_control_packet(T, M) :- log_entry(T, /articulation, /error, M, _, _), :string:contains(M, "invalid").

# 29. Memory exhaustion from huge response
Decl response_memory_exhaustion(Time, Message) bound [/number, /name].
response_memory_exhaustion(T, M) :- log_entry(T, /articulation, /error, M, _, _), :string:contains(M, "out of memory").

# -----------------------------------------------------------------------------
# CODER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 30. Edit atomicity failure
Decl edit_atomicity_failure(Time, Message) bound [/number, /name].
edit_atomicity_failure(T, M) :- log_entry(T, /coder, /error, M, _, _), :string:contains(M, "atomicity").

# 31. Build timeout
Decl build_timeout(Time, Message) bound [/number, /name].
build_timeout(T, M) :- log_entry(T, /coder, /error, M, _, _), :string:contains(M, "build timeout").

# 32. Language detection failure
Decl language_detection_failure(Time, Message) bound [/number, /name].
language_detection_failure(T, M) :- log_entry(T, /coder, /warn, M, _, _), :string:contains(M, "language").

# 33. Parallel edit race
Decl parallel_edit_race(Time, Message) bound [/number, /name].
parallel_edit_race(T, M) :- log_entry(T, /coder, /error, M, _, _), :string:contains(M, "race").

# -----------------------------------------------------------------------------
# TESTER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 34. Framework detection failure
Decl framework_detection_failure(Time, Message) bound [/number, /name].
framework_detection_failure(T, M) :- log_entry(T, /tester, /warn, M, _, _), :string:contains(M, "framework").

# 35. Test timeout
Decl test_timeout(Time, Message) bound [/number, /name].
test_timeout(T, M) :- log_entry(T, /tester, /error, M, _, _), :string:contains(M, "timeout").

# 36. Coverage parsing failure
Decl coverage_parsing_failure(Time, Message) bound [/number, /name].
coverage_parsing_failure(T, M) :- log_entry(T, /tester, /error, M, _, _), :string:contains(M, "coverage").

# 37. TDD infinite loop
Decl tdd_infinite_loop(Time, Message) bound [/number, /name].
tdd_infinite_loop(T, M) :- log_entry(T, /tester, /error, M, _, _), :string:contains(M, "infinite").

# -----------------------------------------------------------------------------
# REVIEWER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 38. Finding explosion
Decl finding_explosion(Time, Message) bound [/number, /name].
finding_explosion(T, M) :- log_entry(T, /reviewer, /warn, M, _, _), :string:contains(M, "finding").

# 39. Custom rules error
Decl custom_rules_error(Time, Message) bound [/number, /name].
custom_rules_error(T, M) :- log_entry(T, /reviewer, /error, M, _, _), :string:contains(M, "custom rules").

# 40. Specialist cascade
Decl specialist_cascade(Time, Message) bound [/number, /name].
specialist_cascade(T, M) :- log_entry(T, /reviewer, /warn, M, _, _), :string:contains(M, "cascade").

# 41. Division by zero in complexity
Decl complexity_division_by_zero(Time, Message) bound [/number, /name].
complexity_division_by_zero(T, M) :- log_entry(T, /reviewer, /error, M, _, _), :string:contains(M, "division").

# -----------------------------------------------------------------------------
# RESEARCHER SHARD FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 42. HTML parsing bomb
Decl html_parsing_bomb(Time, Message) bound [/number, /name].
html_parsing_bomb(T, M) :- log_entry(T, /researcher, /error, M, _, _), :string:contains(M, "parse").

# 43. Connection exhaustion
Decl connection_exhaustion(Time, Message) bound [/number, /name].
connection_exhaustion(T, M) :- log_entry(T, /researcher, /error, M, _, _), :string:contains(M, "connection").

# 44. Context7 rate limit
Decl context7_rate_limit(Time, Message) bound [/number, /name].
context7_rate_limit(T, M) :- log_entry(T, /researcher, /error, M, _, _), :string:contains(M, "429").

# 45. Domain filter bypass
Decl domain_filter_bypass(Time, Message) bound [/number, /name].
domain_filter_bypass(T, M) :- log_entry(T, /researcher, /warn, M, _, _), :string:contains(M, "domain").

# -----------------------------------------------------------------------------
# NEMESIS SHARD FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 46. Attack explosion
Decl attack_explosion(Time, Message) bound [/number, /name].
attack_explosion(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), :string:contains(M, "attack").

# 47. Tool nesting
Decl tool_nesting(Time, Message) bound [/number, /name].
tool_nesting(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), :string:contains(M, "nesting").

# 48. Vulnerability DB growth
Decl vulnerability_db_growth(Time, Message) bound [/number, /name].
vulnerability_db_growth(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), :string:contains(M, "vulnerability").

# -----------------------------------------------------------------------------
# OUROBOROS FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 49. Generation timeout
Decl tool_generation_timeout(Time, Message) bound [/number, /name].
tool_generation_timeout(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), :string:contains(M, "generation timeout").

# 50. Safety bypass
Decl safety_bypass(Time, Message) bound [/number, /name].
safety_bypass(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), :string:contains(M, "safety").

# 51. Compile failure
Decl tool_compile_failure(Time, Message) bound [/number, /name].
tool_compile_failure(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), :string:contains(M, "compile").

# 52. Infinite nesting
Decl tool_infinite_nesting(Time, Message) bound [/number, /name].
tool_infinite_nesting(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), :string:contains(M, "infinite").

# -----------------------------------------------------------------------------
# THUNDERDOME FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 53. Attack parallelization issue
Decl attack_parallelization(Time, Message) bound [/number, /name].
attack_parallelization(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), :string:contains(M, "parallel").

# 54. Sandbox escape
Decl sandbox_escape(Time, Message) bound [/number, /name].
sandbox_escape(T, M) :- log_entry(T, /autopoiesis, /error, M, _, _), :string:contains(M, "sandbox").

# 55. Artifact growth
Decl artifact_growth(Time, Message) bound [/number, /name].
artifact_growth(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), :string:contains(M, "artifact").

# 56. Incomplete execution
Decl thunderdome_incomplete(Time, Message) bound [/number, /name].
thunderdome_incomplete(T, M) :- log_entry(T, /autopoiesis, /warn, M, _, _), :string:contains(M, "TODO").

# -----------------------------------------------------------------------------
# CAMPAIGN FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 57. Decomposition explosion
Decl decomposition_explosion(Time, Message) bound [/number, /name].
decomposition_explosion(T, M) :- log_entry(T, /campaign, /warn, M, _, _), :string:contains(M, "1000").

# 58. Phase timeout
Decl phase_timeout(Time, Message) bound [/number, /name].
phase_timeout(T, M) :- log_entry(T, /campaign, /error, M, _, _), :string:contains(M, "phase timeout").

# 59. Checkpoint corruption
Decl checkpoint_corruption(Time, Message) bound [/number, /name].
checkpoint_corruption(T, M) :- log_entry(T, /campaign, /error, M, _, _), :string:contains(M, "checkpoint").

# 60. Context overflow
Decl campaign_context_overflow(Time, Message) bound [/number, /name].
campaign_context_overflow(T, M) :- log_entry(T, /campaign, /error, M, _, _), :string:contains(M, "context overflow").

# -----------------------------------------------------------------------------
# WORLD MODEL FAILURES (4 modes)
# -----------------------------------------------------------------------------

# 61. Symlink loop
Decl symlink_loop(Time, Message) bound [/number, /name].
symlink_loop(T, M) :- log_entry(T, /world, /error, M, _, _), :string:contains(M, "symlink").

# 62. Permission denied during scan
Decl scan_permission_denied(Time, Message) bound [/number, /name].
scan_permission_denied(T, M) :- log_entry(T, /world, /error, M, _, _), :string:contains(M, "permission").

# 63. Deep nesting
Decl deep_nesting(Time, Message) bound [/number, /name].
deep_nesting(T, M) :- log_entry(T, /world, /warn, M, _, _), :string:contains(M, "depth").

# 64. Large file
Decl large_file_error(Time, Message) bound [/number, /name].
large_file_error(T, M) :- log_entry(T, /world, /error, M, _, _), :string:contains(M, "large").

# -----------------------------------------------------------------------------
# HOLOGRAPHIC FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 65. Impact explosion
Decl impact_explosion(Time, Message) bound [/number, /name].
impact_explosion(T, M) :- log_entry(T, /context, /warn, M, _, _), :string:contains(M, "impact").

# 66. Cycle in graph
Decl graph_cycle(Time, Message) bound [/number, /name].
graph_cycle(T, M) :- log_entry(T, /context, /error, M, _, _), :string:contains(M, "cycle").

# 67. Holographic timeout
Decl holographic_timeout(Time, Message) bound [/number, /name].
holographic_timeout(T, M) :- log_entry(T, /context, /error, M, _, _), :string:contains(M, "timeout").

# -----------------------------------------------------------------------------
# DREAM STATE FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 68. Consultant overload
Decl consultant_overload(Time, Message) bound [/number, /name].
consultant_overload(T, M) :- log_entry(T, /dream, /error, M, _, _), :string:contains(M, "overload").

# 69. Dream queue overflow
Decl dream_queue_overflow(Time, Message) bound [/number, /name].
dream_queue_overflow(T, M) :- log_entry(T, /dream, /error, M, _, _), :string:contains(M, "queue").

# 70. Learning cascade
Decl learning_cascade(Time, Message) bound [/number, /name].
learning_cascade(T, M) :- log_entry(T, /dream, /warn, M, _, _), :string:contains(M, "cascade").

# -----------------------------------------------------------------------------
# SHADOW MODE FAILURES (2 modes)
# -----------------------------------------------------------------------------

# 71. Simulation timeout
Decl simulation_timeout(Time, Message) bound [/number, /name].
simulation_timeout(T, M) :- log_entry(T, /shadow, /error, M, _, _), :string:contains(M, "timeout").

# 72. Shadow kernel crash
Decl shadow_kernel_crash(Time, Message) bound [/number, /name].
shadow_kernel_crash(T, M) :- log_entry(T, /shadow, /error, M, _, _), :string:contains(M, "crash").

# -----------------------------------------------------------------------------
# BROWSER FAILURES (3 modes)
# -----------------------------------------------------------------------------

# 73. Browser process crash
Decl browser_process_crash(Time, Message) bound [/number, /name].
browser_process_crash(T, M) :- log_entry(T, /browser, /error, M, _, _), :string:contains(M, "crash").

# 74. Browser connection timeout
Decl browser_connection_timeout(Time, Message) bound [/number, /name].
browser_connection_timeout(T, M) :- log_entry(T, /browser, /error, M, _, _), :string:contains(M, "timeout").

# 75. DOM explosion
Decl dom_explosion(Time, Message) bound [/number, /name].
dom_explosion(T, M) :- log_entry(T, /browser, /warn, M, _, _), :string:contains(M, "DOM").

# -----------------------------------------------------------------------------
# ALL FAILURE MODES AGGREGATION
# -----------------------------------------------------------------------------

# Comprehensive failure mode detection
Decl any_failure_mode(Time, Mode, Category, Message) bound [/number, /name, /name, /name].

# Kernel failures
any_failure_mode(T, /KernelPanicOnBoot, /kernel, M) :- kernel_panic_on_boot(T, M).
any_failure_mode(T, /DerivationExplosion, /kernel, M) :- derivation_explosion(T, M).
any_failure_mode(T, /GasLimitHit, /kernel, M) :- gas_limit_hit(T, M).
any_failure_mode(T, /UndeclaredPredicate, /kernel, M) :- validation_error(T, M).
any_failure_mode(T, /QueueOverflow, /shards, M) :- queue_overflow(T, M).
any_failure_mode(T, /ShardLimitHit, /shards, M) :- shard_limit_hit(T, M).

# Spawn queue failures
any_failure_mode(T, /SpawnFastFail, /shards, M) :- spawn_fast_fail(T, M).
any_failure_mode(T, /PriorityInversion, /shards, M) :- priority_inversion(T, M).
any_failure_mode(T, /SpawnDeadlineExpired, /shards, M) :- spawn_deadline_expired(T, M).
any_failure_mode(T, /WorkerContention, /shards, M) :- worker_contention(T, M).

# Virtual store failures
any_failure_mode(T, /ActionPermissionDenied, /VirtualStore, M) :- action_permission_denied(T, M).
any_failure_mode(T, /ActionTimeout, /VirtualStore, M) :- action_timeout(T, M).
any_failure_mode(T, /ToolNotFound, /VirtualStore, M) :- tool_not_found(T, M).
any_failure_mode(T, /ActionCrash, /VirtualStore, M) :- action_crash(T, M).

# Perception failures
any_failure_mode(T, /IntentParseFailure, /perception, M) :- intent_parse_failure(T, M).
any_failure_mode(T, /PiggybackCorruption, /articulation, M) :- piggyback_corruption(T, M).
any_failure_mode(T, /TaxonomyMiss, /perception, M) :- taxonomy_miss(T, M).
any_failure_mode(T, /LlmTimeout, /api, M) :- llm_timeout(T, M).

# Add all other failure modes to the aggregation...
any_failure_mode(T, /BrowserCrash, /browser, M) :- browser_process_crash(T, M).
any_failure_mode(T, /DomExplosion, /browser, M) :- dom_explosion(T, M).
any_failure_mode(T, /ShadowCrash, /shadow, M) :- shadow_kernel_crash(T, M).

# =============================================================================
# AGGREGATION QUERIES (require fn:count)
# =============================================================================

# Error count by category
# Decl error_count_by_category(Category, Count) bound [/name, /number].
# error_count_by_category(C, N) :- error_entry(_, C, _) |> do fn:group_by(C), let N = fn:count().

# =============================================================================
# SELF-HEALING SPECIFIC AGGREGATIONS
# =============================================================================

# All repair events (success + failure)
Decl all_repair_events(Time, Status, Message) bound [/number, /name, /name].
all_repair_events(T, /success, M) :- repair_success(T, M).
all_repair_events(T, /failure, M) :- repair_failure(T, M).
all_repair_events(T, /MaxRetries, M) :- repair_max_retries_exceeded(T, M).

# JIT system health
Decl jit_system_health(Time, Status, Message) bound [/number, /name, /name].
jit_system_health(T, /CorpusLoaded, M) :- corpus_loaded(T, M).
jit_system_health(T, /SelectionOk, M) :- jit_selection(T, M).
jit_system_health(T, /SelectionFallback, M) :- selection_fallback(T, M).
jit_system_health(T, /CorpusMissing, M) :- log_entry(T, /kernel, /warn, M, _, _), :string:contains(M, "corpus not available").

# Validation pipeline events
Decl validation_pipeline(Time, Stage, Result, Message) bound [/number, /name, /name, /name].
validation_pipeline(T, /CorpusValidation, /pass, M) :- corpus_validation(T, M), :string:contains(M, "OK").
validation_pipeline(T, /CorpusValidation, /fail, M) :- corpus_validation(T, M), :string:contains(M, "error").
validation_pipeline(T, /StartupValidation, /pass, M) :- startup_validation_result(T, /pass, M).
validation_pipeline(T, /StartupValidation, /fail, M) :- startup_validation_result(T, /fail, M).
validation_pipeline(T, /RuntimeValidation, /fail, M) :- validation_error(T, M).

# Repair error type breakdown
Decl repair_by_error_type(Time, ERrorType, Message) bound [/number, /name, /name].
repair_by_error_type(T, ET, M) :- jit_repair_triggered(T, ET, M).

# Self-healing effectiveness metrics
Decl healing_metric(Time, Metric, Message) bound [/number, /name, /name].
healing_metric(T, /RepairTriggered, M) :- repair_attempt(T, M).
healing_metric(T, /RepairSucceeded, M) :- repair_success(T, M).
healing_metric(T, /RepairFailed, M) :- repair_failure(T, M).
healing_metric(T, /ValidationError, M) :- validation_error(T, M).
healing_metric(T, /RuleRejected, M) :- rule_rejected(T, M).

# =============================================================================
# FAILURE MODE SUMMARY QUERIES
# =============================================================================

# Critical failures (system-breaking)
Decl critical_failure(Time, Mode, Message) bound [/number, /name, /name].
critical_failure(T, M, Msg) :- kernel_panic_on_boot(T, Msg), M = /KernelPanic .
critical_failure(T, M, Msg) :- boot_failure(T, Msg), M = /BootFailure .
critical_failure(T, M, Msg) :- oom_event(T, _, Msg), M = /oom .
critical_failure(T, M, Msg) :- nil_pointer_error(T, _, Msg), M = /NilPointer .
critical_failure(T, M, Msg) :- shadow_kernel_crash(T, Msg), M = /ShadowCrash .
critical_failure(T, M, Msg) :- browser_process_crash(T, Msg), M = /BrowserCrash .

# High-severity failures (feature-breaking, recoverable)
Decl high_severity_failure(Time, Mode, Message) bound [/number, /name, /name].
high_severity_failure(T, M, Msg) :- queue_overflow(T, Msg), M = /QueueOverflow .
high_severity_failure(T, M, Msg) :- gas_limit_hit(T, Msg), M = /GasLimit .
high_severity_failure(T, M, Msg) :- derivation_explosion(T, Msg), M = /DerivationExplosion .
high_severity_failure(T, M, Msg) :- action_crash(T, Msg), M = /ActionCrash .
high_severity_failure(T, M, Msg) :- tool_compile_failure(T, Msg), M = /ToolCompileFailure .
high_severity_failure(T, M, Msg) :- checkpoint_corruption(T, Msg), M = /CheckpointCorruption .

# Medium-severity failures (degraded performance)
Decl medium_severity_failure(Time, Mode, Message) bound [/number, /name, /name].
medium_severity_failure(T, M, Msg) :- shard_limit_hit(T, Msg), M = /ShardLimit .
medium_severity_failure(T, M, Msg) :- timeout_detected(T, _, Msg), M = /timeout .
medium_severity_failure(T, M, Msg) :- memory_exceeded(T, Msg), M = /MemoryExceeded .
medium_severity_failure(T, M, Msg) :- budget_exhausted(T, Msg), M = /BudgetExhausted .
medium_severity_failure(T, M, Msg) :- rate_limit_hit(T, Msg), M = /RateLimit .

# Low-severity failures (warnings, fallbacks work)
Decl low_severity_failure(Time, Mode, Message) bound [/number, /name, /name].
low_severity_failure(T, M, Msg) :- taxonomy_miss(T, Msg), M = /TaxonomyMiss .
low_severity_failure(T, M, Msg) :- selection_fallback(T, Msg), M = /JitFallback .
low_severity_failure(T, M, Msg) :- language_detection_failure(T, Msg), M = /LanguageDetection .
low_severity_failure(T, M, Msg) :- framework_detection_failure(T, Msg), M = /FrameworkDetection .

# All failures by severity
Decl failure_by_severity(Time, Severity, Mode, Message) bound [/number, /name, /name, /name].
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
Decl critical_issue(Time, Type, Message) bound [/number, /name, /name].
critical_issue(T, /panic, M) :- panic_detected(T, _, M).
critical_issue(T, /oom, M) :- oom_event(T, _, M).
critical_issue(T, /NilPointer, M) :- nil_pointer_error(T, _, M).
critical_issue(T, /deadlock, M) :- deadline_exceeded(T, _, M).
critical_issue(T, /BootFailure, M) :- boot_failure(T, M).
critical_issue(T, /KernelPanic, M) :- kernel_panic_on_boot(T, M).
critical_issue(T, /ShadowCrash, M) :- shadow_kernel_crash(T, M).
critical_issue(T, /CorpusMissing, M) :- healing_critical(T, /CorpusMissing, M).

# Self-healing system health check
Decl self_healing_health(Status, Message) bound [/name, /name].
self_healing_health(/CorpusLoaded, "OK") :- corpus_loaded(_, _).
self_healing_health(/RepairShardActive, "OK") :- repair_shard_event(_, _).
self_healing_health(/JitSelectionWorking, "OK") :- jit_selection(_, _).
self_healing_health(/ValidationActive, "OK") :- corpus_validation(_, _).

# Overall stress test health
Decl stress_test_health(Category, Status, Count) bound [/name, /name, /number].
# Note: These would need aggregation support to count properly
# stress_test_health(/critical_issues, /fail, Count) :- critical_issue(_, _, _) |> do fn:count(), let Count = ...
# For now, existence checks:
stress_test_health(/HasCriticalIssues, /fail, 1) :- critical_issue(_, _, _).
stress_test_health(/HasPanics, /fail, 1) :- panic_detected(_, _, _).
stress_test_health(/HasOom, /fail, 1) :- oom_event(_, _, _).
stress_test_health(/CorpusAvailable, /pass, 1) :- corpus_loaded(_, _).
stress_test_health(/RepairSystemActive, /pass, 1) :- repair_shard_event(_, _).
