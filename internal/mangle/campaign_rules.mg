# Campaign-specific logic extensions
# Transitive closure, inferred dependencies, eligibility, and failure-driven replans

# -------------------------------
# Reachability / cycle detection
# -------------------------------

# Base and recursive reachability over phase dependencies
phase_reachable(A, B) :-
    phase_dependency(A, B, _).

phase_reachable(A, C) :-
    phase_reachable(A, B),
    phase_dependency(B, C, _).

# Cycle detection
validation_error(PhaseID, /circular_dependency, "cycle_detected") :-
    phase_reachable(PhaseID, PhaseID).

# -------------------------------
# Inferred dependencies from artifacts
# -------------------------------

produces_artifact(Task, Path) :-
    task_artifact(Task, /source_file, Path, _).

produces_artifact(Task, Path) :-
    task_artifact(Task, /test_file, Path, _).

consumes_artifact(Task, Path) :-
    task_artifact(Task, /file_modify, Path, _).

consumes_artifact(Task, Path) :-
    task_artifact(Task, /test_write, Path, _).

consumes_artifact(Task, Path) :-
    task_artifact(Task, /test_run, Path, _).

inferred_dependency(Consumer, Producer) :-
    consumes_artifact(Consumer, Path),
    produces_artifact(Producer, Path),
    Consumer != Producer.

effective_dependency(T, Dep) :-
    task_dependency(T, Dep).

effective_dependency(T, Dep) :-
    inferred_dependency(T, Dep).

# -------------------------------
# Eligibility and blocking
# -------------------------------

is_complete(Task) :-
    campaign_task(Task, _, _, /completed, _).

is_skipped(Task) :-
    campaign_task(Task, _, _, /skipped, _).

is_blocked(Task) :-
    effective_dependency(Task, Dep),
    !is_complete(Dep),
    !is_skipped(Dep).

eligible_task(Task) :-
    campaign_task(Task, Phase, _, /pending, _),
    campaign_phase(Phase, _, _, _, /in_progress, _),
    !is_blocked(Task).

# Optional: derive a single next task (highest priority: critical > high > normal > low)
priority_higher(/critical, /high).
priority_higher(/critical, /normal).
priority_higher(/critical, /low).
priority_higher(/high, /normal).
priority_higher(/high, /low).
priority_higher(/normal, /low).

higher_priority_exists(Task) :-
    eligible_task(Task2),
    task_priority(Task, P1),
    task_priority(Task2, P2),
    priority_higher(P2, P1).

# Deterministic tie-breaker by task_order (lower index wins)
higher_priority_exists(Task) :-
    eligible_task(Task2),
    task_priority(Task, Priority),
    task_priority(Task2, Priority),
    task_order(Task, Order1),
    task_order(Task2, Order2),
    Order2 < Order1.

next_campaign_task(Task) :-
    eligible_task(Task),
    !higher_priority_exists(Task).

# -------------------------------
# Failure aggregation / circuit breaker
# -------------------------------

task_fail_count(Task, Count) :-
    task_attempt(Task, _, /failure, _) |>
    do fn:group_by(Task), let Count = fn:count().

phase_fail_count(Phase, TotalFailures) :-
    campaign_task(Task, Phase, _, _, _),
    task_fail_count(Task, C) |>
    do fn:group_by(Phase), let TotalFailures = fn:sum(C).

replan_trigger(CampID, /failure_threshold_exceeded, 0) :-
    campaign_phase(Phase, CampID, _, _, /in_progress, _),
    phase_fail_count(Phase, N),
    N > 5.

# -------------------------------
# Stuck detection and debug helpers
# -------------------------------

has_pending_tasks(Phase) :-
    campaign_task(_, Phase, _, /pending, _).

has_running_tasks(Phase) :-
    campaign_task(_, Phase, _, /in_progress, _).

# Phase is stuck if in-progress but with no runnable or active tasks
phase_stuck(Phase) :-
    campaign_phase(Phase, _, _, _, /in_progress, _),
    !has_pending_tasks(Phase),
    !has_running_tasks(Phase).

# Explain why a task is blocked (missing dependency completion)
debug_why_blocked(Task, Dependency) :-
    campaign_task(Task, _, _, /pending, _),
    effective_dependency(Task, Dependency),
    !is_complete(Dependency),
    !is_skipped(Dependency).

# Bubble stuck phases up as campaign blocks for the orchestrator
campaign_blocked(CampaignID, "phase_stuck") :-
    phase_stuck(Phase),
    campaign_phase(Phase, CampaignID, _, _, _, _).
