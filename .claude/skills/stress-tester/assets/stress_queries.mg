# Stress Test Log Analysis Queries
# Use with log-analyzer skill's logquery tool
#
# Usage:
#   1. Parse logs: python parse_log.py .nerd/logs/* --no-schema > facts.mg
#   2. Load: ./logquery.exe facts.mg -i
#   3. Run queries below

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

# PredicateCorpus loading events
Decl corpus_loaded(time: num, message: name).
corpus_loaded(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "corpus loaded").

# Corpus validation events
Decl corpus_validation(time: num, message: name).
corpus_validation(T, M) :- log_entry(T, /kernel, _, M, _, _), fn:contains(M, "check-mangle").

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
# AGGREGATION QUERIES (require fn:count)
# =============================================================================

# Error count by category
# Decl error_count_by_category(category: name, count: num).
# error_count_by_category(C, N) :- error_entry(_, C, _) |> do fn:group_by(C), let N = fn:count().

# =============================================================================
# STRESS TEST SUCCESS CRITERIA
# =============================================================================

# A stress test passes if:
# 1. No panics: panic_detected should return empty
# 2. No OOM: oom_event should return empty
# 3. Limited errors: error_entry count within threshold
# 4. No deadlocks: timeout_event count within threshold
# 5. Shards complete: shard_spawned count == shard_completed count (approx)

# Query all critical issues:
Decl critical_issue(time: num, type: name, message: name).
critical_issue(T, /panic, M) :- panic_detected(T, _, M).
critical_issue(T, /oom, M) :- oom_event(T, _, M).
critical_issue(T, /nil_pointer, M) :- nil_pointer_error(T, _, M).
critical_issue(T, /deadlock, M) :- deadline_exceeded(T, _, M).
