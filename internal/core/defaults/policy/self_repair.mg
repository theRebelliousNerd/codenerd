# Self-repair: codeNERD fixing and speeding up its own source while debug-self
# is on, with no one asking (chat self_repair.go).
#
# The kernel decides; the chat executes. Go measures and records: the problems
# log (self_problem, self_problems.mg), the run's operation timings from the
# audit log (self_timing), every attempt's lifecycle from the ledger under
# .nerd/self_repair (self_repair_attempt), and the clock (self_repair_clock).
# This file derives what is due (self_repair_due), on which lane, by which
# persona (self_repair_shard), and each attempt's verdict after the restart
# onto its fix (self_repair_verified / self_repair_regressed). The chat starts
# the self-diagnosis chain (self_diagnosis.mg) for a due target: a read-only
# background diagnosis whose follow-up is the fix turn.
#
# Lifecycle (Status): /being_fixed (diagnosis or fix turn running) ->
# /pending_restart (the fix is built, self-checked and installed over the
# running binary) -> /verifying (a new process runs the fix) -> /verified, or
# /regressed (the problem came back, or the operation got no faster). A job or
# fix turn that ends without an installed fix is /failed; one the user stopped
# is /stopped. A diagnosis whose verdict (self_diagnosis.mg) found nothing of
# codeNERD's to fix for a target settles it as /no_defect without a fix turn.
# /failed and /regressed count toward Failures; /no_defect and /stopped do not.
# A target that has failed logging.self_repair_max_failures times is left, as
# recurse leaves a node that fails the same way twice.

Decl self_repair_attempt(Target, Lane, Status, Job, Tries, Failures, UpdatedUnix) bound [/string, /name, /name, /string, /number, /number, /number].
# The optimize lane's measured mean before its fix, in milliseconds.
Decl self_repair_baseline(Target, MeanMs) bound [/string, /number].
Decl self_repair_clock(Unix) bound [/number].
# One row per operation the run timed (logging.StartTimer), from its audit log:
# Target is "timing:<system>.<operation>".
Decl self_timing(Target, System, Operation, Count, TotalMs, MeanMs) bound [/string, /string, /string, /number, /number, /number].
# Asserted while a self-repair fix turn runs (shared into every shard).
Decl self_repair_fixing() bound [].

Decl self_timing_external(System) bound [/string].
Decl self_repair_protected_path(Match) bound [/string].
Decl self_repair_job_in_flight(Job) bound [/string].
Decl self_repair_jobs_in_flight(N) bound [/number].
Decl self_repair_busy() bound [].
Decl self_repair_awaiting_restart() bound [].
Decl self_repair_enabled() bound [].
Decl self_repair_slot_open() bound [].
Decl self_repair_held(Target) bound [/string].
Decl self_repair_fix_due(Target) bound [/string].
Decl self_repair_fix_due_any() bound [].
Decl self_optimize_candidate(Target, TotalMs) bound [/string, /number].
Decl self_optimize_top(TotalMs) bound [/number].
Decl self_optimize_due(Target) bound [/string].
Decl self_repair_due(Target, Lane) bound [/string, /name].
Decl self_repair_shard(Persona) bound [/name].
Decl self_repair_verified(Target) bound [/string].
Decl self_repair_regressed(Target) bound [/string].
# The evidence a /no_defect verdict was given on: the target's count (the
# problem's in the problems log, or the operation's timings) when it settled.
Decl self_repair_no_defect_evidence(Target, Count) bound [/string, /number].
Decl self_repair_evidence_count(Target, Count) bound [/string, /number].
Decl self_repair_evidence_regrown(Target) bound [/string].

config_param_required(/logging, /logging_self_repair_enabled).
config_param_required(/logging, /logging_self_repair_cooldown_sec).
config_param_required(/logging, /logging_self_repair_max_failures).
config_param_required(/logging, /logging_self_repair_max_jobs).
config_param_required(/logging, /logging_self_repair_verify_window_sec).
config_param_required(/logging, /logging_self_optimize_min_count).
config_param_required(/logging, /logging_self_repair_no_defect_regrowth).

# --- When a job may start ------------------------------------------------------

# The clock is the ledger's (Go asserts both together), so the slot rules fire
# where the attempts are and never negate them blind in another shard.
self_repair_enabled() :-
    debug_self_active(),
    self_repair_clock(_),
    config_param(/logging_self_repair_enabled, 1).

self_repair_job_in_flight(Job) :-
    self_repair_attempt(_, _, /being_fixed, Job, _, _, _).

self_repair_jobs_in_flight(N) :-
    self_repair_job_in_flight(Job)
    |> do fn:group_by(), let N = fn:count().

self_repair_busy() :- self_repair_job_in_flight(_).

# A fix waiting for the restart is verified before the next one lands: two
# fixes in one build could not be told apart.
self_repair_awaiting_restart() :-
    self_repair_attempt(_, _, /pending_restart, _, _, _, _).

self_repair_slot_open() :-
    self_repair_enabled(),
    !self_repair_awaiting_restart(),
    !self_repair_busy().
self_repair_slot_open() :-
    self_repair_enabled(),
    !self_repair_awaiting_restart(),
    self_repair_jobs_in_flight(N),
    config_param(/logging_self_repair_max_jobs, Max),
    N < Max.

# --- What is not due -----------------------------------------------------------

self_repair_held(Target) :-
    self_repair_attempt(Target, _, /being_fixed, _, _, _, _).
self_repair_held(Target) :-
    self_repair_attempt(Target, _, /pending_restart, _, _, _, _).
self_repair_held(Target) :-
    self_repair_attempt(Target, _, /verifying, _, _, _, _).
# An optimization that held is not redone: the operation may still be the
# slowest, and it is someone else's turn.
self_repair_held(Target) :-
    self_repair_attempt(Target, /optimize, /verified, _, _, _, _).
# Failed the same way too often: left.
self_repair_held(Target) :-
    self_repair_attempt(Target, _, _, _, _, Failures, _),
    config_param(/logging_self_repair_max_failures, Max),
    Failures >= Max.
# Cooling down since its last change.
self_repair_held(Target) :-
    self_repair_attempt(Target, _, _, _, _, _, Updated),
    self_repair_clock(Now),
    config_param(/logging_self_repair_cooldown_sec, Cooldown),
    Until = fn:plus(Updated, Cooldown),
    Now < Until.
# A diagnosis that found nothing of codeNERD's to fix is not redone on the same
# evidence. It is due again only once the evidence has grown
# logging.self_repair_no_defect_regrowth times over what that diagnosis saw:
# the problem recurred that much more often, or the operation was timed that
# much more often, in the run that reads it. A new process's problems log
# starts its count again, so a problem that comes back after a restart must
# recur that often there too. It never counts as a failure.
self_repair_evidence_count(Target, Count) :-
    self_problem(Target, _, _, Count, _).
self_repair_evidence_count(Target, Count) :-
    self_timing(Target, _, _, Count, _, _).

self_repair_evidence_regrown(Target) :-
    self_repair_attempt(Target, _, /no_defect, _, _, _, _),
    self_repair_no_defect_evidence(Target, Base),
    self_repair_evidence_count(Target, Count),
    config_param(/logging_self_repair_no_defect_regrowth, Factor),
    Need = fn:mult(Base, Factor),
    Count >= Need.

self_repair_held(Target) :-
    self_repair_attempt(Target, _, /no_defect, _, _, _, _),
    !self_repair_evidence_regrown(Target).
# A failure Jev judged to come from around codeNERD (jev_evidence.mg) is not
# codeNERD's to fix.
self_repair_held(Target) :-
    self_problem_around(Target, _).

# --- What is due ---------------------------------------------------------------

# Repair: every hot problem of codeNERD's own log that is not held.
self_repair_fix_due(Signature) :-
    self_repair_slot_open(),
    self_problem_hot(Signature, _, _),
    !self_repair_held(Signature).

self_repair_fix_due_any() :- self_repair_fix_due(_).

# Optimize, once nothing is left to repair: the recurring operation that cost
# the run the most time in all (count times mean). Time spent waiting on a
# model, the network or a browser is not codeNERD's code to speed up.
self_timing_external("api").
self_timing_external("embedding").
self_timing_external("browser").

self_optimize_candidate(Target, Total) :-
    self_timing(Target, System, _, Count, Total, _),
    config_param(/logging_self_optimize_min_count, Min),
    Count >= Min,
    !self_timing_external(System),
    !self_repair_held(Target).

self_optimize_top(Max) :-
    self_optimize_candidate(Target, Total)
    |> do fn:group_by(), let Max = fn:max(Total).

self_optimize_due(Target) :-
    self_repair_slot_open(),
    !self_repair_fix_due_any(),
    self_optimize_top(Max),
    self_optimize_candidate(Target, Max).

self_repair_due(Target, /repair) :- self_repair_fix_due(Target).
self_repair_due(Target, /optimize) :- self_optimize_due(Target).

# The persona that reads codeNERD's source for a diagnosis is the one the
# self-diagnosis verb routes to.
self_repair_shard(Persona) :- verb_persona(/self_diagnose, Persona).

# --- Verdicts after the restart ------------------------------------------------

# A repaired problem that is logged again by the process running its fix came
# back.
self_repair_regressed(Signature) :-
    self_repair_attempt(Signature, /repair, /verifying, _, _, _, Since),
    self_problem_seen(Signature, _, _, _, Last),
    Last >= Since.

# One that stayed away for the verify window held.
self_repair_verified(Signature) :-
    self_repair_attempt(Signature, /repair, /verifying, _, _, _, Since),
    self_repair_clock(Now),
    config_param(/logging_self_repair_verify_window_sec, Window),
    Due = fn:plus(Since, Window),
    Now >= Due,
    !self_repair_regressed(Signature).

# An optimized operation is judged once it has recurred as often as it had to
# before it was chosen: faster on average than its baseline, or not.
self_repair_verified(Target) :-
    self_repair_attempt(Target, /optimize, /verifying, _, _, _, _),
    self_repair_baseline(Target, Base),
    self_timing(Target, _, _, Count, _, Mean),
    config_param(/logging_self_optimize_min_count, Min),
    Count >= Min,
    Mean < Base.
self_repair_regressed(Target) :-
    self_repair_attempt(Target, /optimize, /verifying, _, _, _, _),
    self_repair_baseline(Target, Base),
    self_timing(Target, _, _, Count, _, Mean),
    config_param(/logging_self_optimize_min_count, Min),
    Count >= Min,
    Mean >= Base.

# --- What a self-repair may never touch ----------------------------------------

# codeNERD does not widen the rules that constrain it. While a self-repair fix
# turn runs, the safety gate, the permission logic, the approval and work-mode
# code, and self-repair's own trigger are project_forbidden_path, which the
# executor's write guard (internal/projectdoc/gate.go) and coder_edit_barred
# (coder_edit_bar.mg) both refuse.
self_repair_protected_path("internal/core/defaults/policy/constitution.mg").
self_repair_protected_path("internal/core/defaults/policy/coder_edit_bar.mg").
self_repair_protected_path("internal/core/defaults/policy/coder_safety.mg").
self_repair_protected_path("internal/core/defaults/policy/codedom_safety.mg").
self_repair_protected_path("internal/core/defaults/policy/git_safety.mg").
self_repair_protected_path("internal/core/defaults/policy/projectdoc.mg").
self_repair_protected_path("internal/core/defaults/policy/self_repair.mg").
self_repair_protected_path("internal/core/defaults/schemas_projectdoc.mg").
self_repair_protected_path("internal/core/defaults/schemas_safety.mg").
self_repair_protected_path("internal/core/constitution").
self_repair_protected_path("internal/core/virtual_store_constitution.go").
self_repair_protected_path("internal/core/dreamer.go").
self_repair_protected_path("internal/mangle/grant_path.go").
self_repair_protected_path("internal/projectdoc/gate.go").
self_repair_protected_path("internal/session/write_guard").
self_repair_protected_path("internal/session/human_approval.go").
self_repair_protected_path("cmd/nerd/chat/yolo.go").
self_repair_protected_path("cmd/nerd/chat/work_mode.go").
self_repair_protected_path("cmd/nerd/chat/approval_host.go").
self_repair_protected_path("cmd/nerd/chat/self_repair").
self_repair_protected_path("nerd.md").

project_forbidden_path(Match, "self-repair never edits codeNERD's safety gate, permission or approval logic, or its own trigger") :-
    self_repair_fixing(),
    self_repair_protected_path(Match).
