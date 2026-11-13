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
# /failed, /regressed and /over_budget count toward Failures; /no_defect and
# /stopped do not.
# A target that has failed logging.self_repair_max_failures times is left, as
# recurse leaves a node that fails the same way twice.

Decl self_repair_attempt(Target, Lane, Status, Job, Tries, Failures, UpdatedUnix) bound [/string, /name, /name, /string, /number, /number, /number].
# The optimize lane's measured mean before its fix, in milliseconds.
Decl self_repair_baseline(Target, MeanMs) bound [/string, /number].
Decl self_repair_clock(Unix) bound [/number].
# One row per operation the run timed (logging.StartTimer), from its audit log:
# Target is "timing:<system>.<operation>".
Decl self_timing(Target, System, Operation, Count, TotalMs, MeanMs) bound [/string, /string, /string, /number, /number, /number].
# The same operation's timings since the process's boot settled (the startup
# scan finished): the steady state the optimize lane ranks and judges.
Decl self_timing_settled(Target, Count, TotalMs, MeanMs) bound [/string, /number, /number, /number].
# Asserted while a self-repair fix turn runs (shared into every shard).
Decl self_repair_fixing() bound [].
# When each fix turn ran, from the ledger; a window still open ends at the
# clock.
Decl self_repair_fix_window(Job, StartUnix, EndUnix) bound [/string, /number, /number].
# A fix window still open: its chain is running (asserted with the window).
Decl self_repair_fix_open(Job, StartUnix) bound [/string, /number].
# An unattended fix chain that has run past logging.self_repair_fix_budget_sec.
# The chat stops it, undoes its writes and records the attempt /over_budget,
# which counts toward Failures like /failed.
Decl self_repair_fix_over_budget(Job) bound [/string].
# What an open fix chain has spent in model tokens since it began, measured by
# the chat from the session's usage (asserted with the budget question).
Decl self_repair_fix_spent(Job, Tokens) bound [/string, /number].
# Which budget an open fix chain has run past: /time or /tokens.
Decl self_repair_fix_budget_spent(Job, Kind) bound [/string, /name].

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
Decl self_repair_parked_job(Job) bound [/string].
Decl self_repair_finish_due(Job) bound [/string].
Decl self_repair_finish_due_any() bound [].
Decl self_repair_shard(Persona) bound [/name].
Decl self_repair_verified(Target) bound [/string].
Decl self_repair_regressed(Target) bound [/string].
# The evidence a /no_defect verdict was given on: the target's count (the
# problem's in the problems log, or the operation's timings) when it settled.
Decl self_repair_no_defect_evidence(Target, Count) bound [/string, /number].
Decl self_repair_evidence_count(Target, Count) bound [/string, /number].
Decl self_repair_evidence_regrown(Target) bound [/string].
Decl self_problem_at_in_fix(Signature, Unix, Job) bound [/string, /number, /string].
Decl self_problem_at_fixed(Signature, Unix) bound [/string, /number].
Decl self_problem_outside_fix(Signature) bound [/string].
Decl self_problem_from_repair(Signature, Job) bound [/string, /string].

config_param_required(/logging, /logging_self_repair_enabled).
config_param_required(/logging, /logging_self_repair_cooldown_sec).
config_param_required(/logging, /logging_self_repair_fix_budget_sec).
config_param_required(/logging, /logging_self_repair_fix_budget_tokens).
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
# A problem logged only while self-repair fix turns ran is the record of those
# attempts -- a provider call that timed out under the coder, a line about the
# attempt's own edits -- and not a defect a new diagnosis should chase: the
# attempt it belongs to has already been judged (failed, with what it tried).
# One logged at any second outside every fix window is still codeNERD's
# problem, and is due like any other. Observed live 2026-11-12: a failed fix
# turn's own log lines became the next four "hot problems", and self-repair
# began diagnosing its own exhaust.
self_problem_at_in_fix(Signature, Unix, Job) :-
    self_problem_at(Signature, Unix),
    self_repair_fix_window(Job, Start, End),
    Start <= Unix,
    Unix <= End.

self_problem_at_fixed(Signature, Unix) :- self_problem_at_in_fix(Signature, Unix, _).

self_problem_outside_fix(Signature) :-
    self_problem_at(Signature, Unix),
    !self_problem_at_fixed(Signature, Unix).

self_problem_from_repair(Signature, Job) :-
    self_problem_at_in_fix(Signature, _, Job),
    !self_problem_outside_fix(Signature).

self_repair_held(Signature) :- self_problem_from_repair(Signature, _).

# A failure Jev judged to come from around codeNERD (jev_evidence.mg) is not
# codeNERD's to fix.
self_repair_held(Target) :-
    self_problem_around(Target, _).

# --- What is due ---------------------------------------------------------------

# A parked candidate first. A fix chain that ran out of budget with a change
# green on its own gates and a slower gate unfinished parked it (the chat's
# settleFixTurn: undone, kept with its evidence, /parked). Finishing it costs
# only the gates that had not finished; choosing a new target would throw the
# work away and pay for it again. So while one is parked and the slot is
# open, it is due, and no new target is.
self_repair_parked_job(Job) :-
    self_repair_attempt(_, _, /parked, Job, _, _, _).

self_repair_finish_due(Job) :-
    self_repair_slot_open(),
    self_repair_parked_job(Job).

self_repair_finish_due_any() :- self_repair_finish_due(_).

# Repair: every hot problem of codeNERD's own log that is not held.
self_repair_fix_due(Signature) :-
    self_repair_slot_open(),
    !self_repair_finish_due_any(),
    self_problem_hot(Signature, _, _),
    !self_repair_held(Signature).

self_repair_fix_due_any() :- self_repair_fix_due(_).

# Optimize, once nothing is left to repair: the recurring operation that cost
# the run the most time in all (count times mean) since its boot settled.
# Time spent waiting on a model, the network or a browser is not codeNERD's
# code to speed up. A boot evaluates the whole world once, and that cost is
# not the steady state's: an operation is a candidate only once it has
# recurred logging.self_optimize_min_count times after boot, ranked by what
# it cost after boot. Live 2026-11-12, kernel.evaluate's 244 ms mean was 22
# runs right after a boot over a 1M-fact world.
self_timing_external("api").
self_timing_external("embedding").
self_timing_external("browser").

self_optimize_candidate(Target, Total) :-
    self_timing(Target, System, _, _, _, _),
    self_timing_settled(Target, Count, Total, _),
    config_param(/logging_self_optimize_min_count, Min),
    Count >= Min,
    !self_timing_external(System),
    !self_repair_held(Target).

self_optimize_top(Max) :-
    self_optimize_candidate(Target, Total)
    |> do fn:group_by(), let Max = fn:max(Total).

self_optimize_due(Target) :-
    self_repair_slot_open(),
    !self_repair_finish_due_any(),
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
# before it was chosen, after the new process's boot settled: faster on
# average than its baseline (its settled mean when chosen), or not.
self_repair_verified(Target) :-
    self_repair_attempt(Target, /optimize, /verifying, _, _, _, _),
    self_repair_baseline(Target, Base),
    self_timing_settled(Target, Count, _, Mean),
    config_param(/logging_self_optimize_min_count, Min),
    Count >= Min,
    Mean < Base.
self_repair_regressed(Target) :-
    self_repair_attempt(Target, /optimize, /verifying, _, _, _, _),
    self_repair_baseline(Target, Base),
    self_timing_settled(Target, Count, _, Mean),
    config_param(/logging_self_optimize_min_count, Min),
    Count >= Min,
    Mean >= Base.

# --- A fix chain that does not converge -----------------------------------------

# Observed live 2026-11-12: a correct, tested change spent 55 minutes inside
# its own turn's gates (a step plan that went on after a failed step,
# specialist reviews, a pinning repair) with no verdict. An unattended chain
# has a budget; past it the work is stopped and undone, and the retry is told.
self_repair_fix_budget_spent(Job, /time) :-
    self_repair_fix_open(Job, Start),
    self_repair_clock(Now),
    config_param(/logging_self_repair_fix_budget_sec, Budget),
    Deadline = fn:plus(Start, Budget),
    Now > Deadline.

# The clock is not the only thing a chain spends. The same live chain put
# 1.55M input tokens into one step writing tests for a change an earlier step
# never made, well inside its time budget. No measurement of the spend is no
# verdict: the time budget still stands.
self_repair_fix_budget_spent(Job, /tokens) :-
    self_repair_fix_open(Job, _),
    self_repair_fix_spent(Job, Spent),
    config_param(/logging_self_repair_fix_budget_tokens, Budget),
    Spent > Budget.

self_repair_fix_over_budget(Job) :-
    self_repair_fix_budget_spent(Job, _).

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
