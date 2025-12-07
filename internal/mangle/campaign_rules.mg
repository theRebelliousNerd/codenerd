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
    not is_complete(Dep),
    not is_skipped(Dep).

eligible_task(Task) :-
    campaign_task(Task, Phase, _, /pending, _),
    campaign_phase(Phase, _, _, _, /in_progress, _),
    not is_blocked(Task).

# Optional: derive a single next task (highest priority: critical > high > normal > low)
priority_higher(/critical, /high).
priority_higher(/critical, /normal).
priority_higher(/critical, /low).
priority_higher(/high, /normal).
priority_higher(/high, /low).
priority_higher(/normal, /low).

higher_priority_exists(Task) :-
    eligible_task(Task2),
    campaign_task(Task, _, _, _, P1),
    campaign_task(Task2, _, _, _, P2),
    priority_higher(P2, P1).

next_campaign_task(Task) :-
    eligible_task(Task),
    not higher_priority_exists(Task).

# -------------------------------
# Failure aggregation / circuit breaker
# -------------------------------

task_fail_count(Task, Count) :-
    task_attempt(Task, _, /failure, _) |>
    do fn:group_by(Task), let Count = fn:Count(Task).

phase_fail_count(Phase, TotalFailures) :-
    campaign_task(Task, Phase, _, _, _),
    task_fail_count(Task, C) |>
    do fn:group_by(Phase), let TotalFailures = fn:Sum(C).

replan_trigger(CampID, /failure_threshold_exceeded, Now) :-
    campaign_phase(Phase, CampID, _, _, /in_progress, _),
    phase_fail_count(Phase, N),
    N > 5,
    Now = fn:now().

