# Campaign Decisions: what the orchestrator does next, derived
#
# The campaign orchestrator is a driver. It runs builds, spawns agents, writes
# the campaign file and asserts what it measured; every decision about what
# happens next is a rule here, which it queries and acts on:
#
#   verify_task_route(Task, /build | /review)  how a /verify task is judged
#   checkpoint_verdict_outcome(Key, /pass | /fail | /inconclusive)
#                                              what a reviewer's verdict decides
#   task_next_move(Task, /retry | /retry_later | /repro_first | /fail | /replan)
#                                              what a failed task does next
#   phase_ckpt_move(Phase, /close_unverified | /replan | /recheck)
#                                              what a failed checkpoint leads to
#
# Thresholds are config_param rows from the campaign section of
# .nerd/config.json (policy/config_params.mg); each is declared required next
# to the rule that reads it.

# =============================================================================
# What a /verify task is evidence about
# =============================================================================
# A build is evidence only about code the phase wrote. A /verify task in a phase
# that wrote no Go is a review of the phase's artifacts, which leaves a durable
# finding; it is never `go build ./...`. Until 2026-09-22 Go routed /verify by a
# 25-keyword list over the description and ran the build for anything else:
# campaign 7b853890 "verified" two Markdown-content claims ("Verify shipped
# drafts use Go code only ...", "Verify 01-VISION.md, 05 and 06 specs declare
# built vs not-built status ...") with a 14-second Go build.
#
# The phase's write class comes from its tasks' declared write sets
# (task_write_ext, asserted with task_write_target) through write_class, the
# same table that decides what a turn's writes owe.
Decl task_writes_code(TaskID) bound [/string].
Decl phase_writes_code(PhaseID) bound [/string].
Decl verify_task_route(TaskID, Route) bound [/string, /name].

task_writes_code(TaskID) :-
    task_write_ext(TaskID, Ext),
    write_class(Ext, /go).

phase_writes_code(PhaseID) :-
    campaign_task(TaskID, PhaseID, Desc, Status, Type),
    task_writes_code(TaskID).

verify_task_route(TaskID, /build) :-
    campaign_task(TaskID, PhaseID, Desc, Status, /verify),
    phase_writes_code(PhaseID).

verify_task_route(TaskID, /review) :-
    campaign_task(TaskID, PhaseID, Desc, Status, /verify),
    !phase_writes_code(PhaseID).

# =============================================================================
# A checkpoint's verdict
# =============================================================================
# A reviewer (shard validation, the nemesis gauntlet) answers a checkpoint with
# checkpoint_verdict(Key, /pass | /fail, Reason, Confidence), keyed by the
# phase's key (its ID without the leading slash, which a quoted string in a
# control packet cannot carry and still match). Go used to take the first
# /pass or /fail row it read and ignore the confidence it had parsed, so a
# reviewer that asserted both could pass, and a 20%-sure /pass counted as a
# pass. The verdict is derived: any /fail fails; a /pass counts only at
# campaign.checkpoint_min_confidence or above; a /pass below it is
# inconclusive, which does not pass.
Decl checkpoint_verdict_fail(Key) bound [/string].
Decl checkpoint_verdict_confident_pass(Key) bound [/string].
Decl checkpoint_verdict_outcome(Key, Verdict) bound [/string, /name].

config_param_required(/campaign, /campaign_checkpoint_min_confidence).

checkpoint_verdict_fail(Key) :-
    checkpoint_verdict(Key, /fail, Reason, Confidence).

checkpoint_verdict_confident_pass(Key) :-
    checkpoint_verdict(Key, /pass, Reason, Confidence),
    config_param(/campaign_checkpoint_min_confidence, Floor),
    Confidence >= Floor.

checkpoint_verdict_outcome(Key, /fail) :-
    checkpoint_verdict_fail(Key).

checkpoint_verdict_outcome(Key, /pass) :-
    checkpoint_verdict_confident_pass(Key),
    !checkpoint_verdict_fail(Key).

checkpoint_verdict_outcome(Key, /inconclusive) :-
    checkpoint_verdict(Key, /pass, Reason, Confidence),
    !checkpoint_verdict_confident_pass(Key),
    !checkpoint_verdict_fail(Key).

# The verification methods the orchestrator can run (CheckpointRunner.Run).
# A phase whose objective names any other method can never be verified, and
# the campaign is blocked on it by name. The runner fails such a checkpoint
# closed; it used to report it passed ("Unknown verification method,
# skipping").
Decl verification_method_known(Method) bound [/name].
Decl phase_has_unknown_method(PhaseID) bound [/string].

verification_method_known(/tests_pass).
verification_method_known(/builds).
verification_method_known(/manual_review).
verification_method_known(/shard_validation).
verification_method_known(/nemesis_gauntlet).
verification_method_known(/none).

phase_has_unknown_method(PhaseID) :-
    phase_objective(PhaseID, Type, Desc, Method),
    !verification_method_known(Method).

campaign_blocked(CampaignID, /unverifiable_objective) :-
    current_campaign(CampaignID),
    campaign_phase(PhaseID, CampaignID, Name, Order, Status, Profile),
    phase_has_unknown_method(PhaseID).

# =============================================================================
# What a failed task does next
# =============================================================================
# Each failed attempt is recorded as task_attempt(Task, N, /failure, At) with a
# task_attempt_signal(Task, N, Signal) row per typed signal (the vocabulary is
# internal/campaign/failure_signals.go). The orchestrator records the attempt,
# asks for task_next_move and does it; no derived move fails the task.
#
#   /fail         the task has had campaign.max_task_attempts failed attempts
#   /replan       the same, the first time, when campaign.replan_at_attempt_cap:
#                 the task fails and the replanner may drop or retype it
#   /repro_first  a task that writes code has failed on a red suite
#                 campaign.repro_after_failures times, the last one included:
#                 a test run that reproduces the failure goes first
#   /retry_later  the attempt ended because the broker refused or its context
#                 ended; nothing about the task was wrong, and a retry can
#                 only usefully arrive later (the full backoff)
#   /retry        anything else (the shortened backoff)
#
# Until 2026-09-23 this was Go: a substring classifier over the error text
# ("connection", "eof", "i/o" meant transient) picked the backoff, a repro task
# was inserted after two "logic" failures of any mutating task -- a Markdown
# task's failed review included -- or after twenty minutes of failing, and a
# rule here called a task exhausted at attempt 3 while Go retried it to 4.
Decl task_failed_attempt(TaskID, Attempt) bound [/string, /number].
Decl campaign_task_failure_count(TaskID, Count) bound [/string, /number].
Decl task_last_failed_attempt(TaskID, Attempt) bound [/string, /number].
Decl task_exhausted(TaskID) bound [/string].
Decl task_replan_due(TaskID) bound [/string].
Decl red_suite_signal(Signal) bound [/name].
Decl task_red_suite_attempt(TaskID, Attempt) bound [/string, /number].
Decl task_red_suite_failures(TaskID, Count) bound [/string, /number].
Decl task_last_failure_red(TaskID) bound [/string].
Decl task_is_repro(TaskID) bound [/string].
Decl task_owes_repro(TaskID) bound [/string].
Decl retry_later_signal(Signal) bound [/name].
Decl task_last_failure_waits(TaskID) bound [/string].
Decl task_next_move(TaskID, Move) bound [/string, /name].

config_param_required(/campaign, /campaign_max_task_attempts).
config_param_required(/campaign, /campaign_replan_at_attempt_cap).
config_param_required(/campaign, /campaign_repro_after_failures).

task_failed_attempt(TaskID, N) :-
    task_attempt(TaskID, N, /failure, At).

campaign_task_failure_count(TaskID, Count) :-
    task_failed_attempt(TaskID, N)
    |> do fn:group_by(TaskID), let Count = fn:count().

task_last_failed_attempt(TaskID, Last) :-
    task_failed_attempt(TaskID, N)
    |> do fn:group_by(TaskID), let Last = fn:max(N).

task_exhausted(TaskID) :-
    campaign_task_failure_count(TaskID, Count),
    config_param(/campaign_max_task_attempts, Max),
    Count >= Max.

task_replan_due(TaskID) :-
    task_exhausted(TaskID),
    config_param(/campaign_replan_at_attempt_cap, 1),
    !task_replanned_at_cap(TaskID).

# A red suite: the turn's kernel verdict found tests or a test run not green,
# or a campaign test run found the suite red.
red_suite_signal(/tests_not_green).
red_suite_signal(/test_run_not_green).
red_suite_signal(/tests_red).

task_red_suite_attempt(TaskID, N) :-
    task_attempt_signal(TaskID, N, Signal),
    red_suite_signal(Signal).

task_red_suite_failures(TaskID, Count) :-
    task_red_suite_attempt(TaskID, N)
    |> do fn:group_by(TaskID), let Count = fn:count().

task_last_failure_red(TaskID) :-
    task_last_failed_attempt(TaskID, N),
    task_red_suite_attempt(TaskID, N).

# The repro task a /repro_first move inserts. It writes nothing, so it never
# owes a repro of its own; this names it as well.
task_is_repro(TaskID) :-
    task_inference(TaskID, From, Confidence, "/logic_failure_repro_guard").

task_owes_repro(TaskID) :-
    task_writes_code(TaskID),
    task_last_failure_red(TaskID),
    task_red_suite_failures(TaskID, Count),
    config_param(/campaign_repro_after_failures, Min),
    Count >= Min,
    !task_is_repro(TaskID).

retry_later_signal(/refused).
retry_later_signal(/deadline).
retry_later_signal(/canceled).

task_last_failure_waits(TaskID) :-
    task_last_failed_attempt(TaskID, N),
    task_attempt_signal(TaskID, N, Signal),
    retry_later_signal(Signal).

task_next_move(TaskID, /replan) :-
    task_replan_due(TaskID).

task_next_move(TaskID, /fail) :-
    task_exhausted(TaskID),
    !task_replan_due(TaskID).

task_next_move(TaskID, /repro_first) :-
    campaign_task_failure_count(TaskID, Count),
    !task_exhausted(TaskID),
    task_owes_repro(TaskID).

task_next_move(TaskID, /retry_later) :-
    campaign_task_failure_count(TaskID, Count),
    !task_exhausted(TaskID),
    !task_owes_repro(TaskID),
    task_last_failure_waits(TaskID).

task_next_move(TaskID, /retry) :-
    campaign_task_failure_count(TaskID, Count),
    !task_exhausted(TaskID),
    !task_owes_repro(TaskID),
    !task_last_failure_waits(TaskID).

# =============================================================================
# What a failed checkpoint leads to
# =============================================================================
# phase_checkpoint_failure(Phase, Run) is one failed checkpoint run of the
# phase since its budget was last armed (a resume re-arms it), mirrored from
# the campaign's own record (Phase.CheckpointFailures), so a resumed campaign
# rebuilds the same view. The orchestrator asks phase_ckpt_move after each
# failure and does it:
#
#   /close_unverified  campaign.max_checkpoint_attempts failures: the phase
#                      closes /unverified, not completed, and its hard
#                      dependents stay blocked
#   /replan            below the cap, when campaign.replan_on_checkpoint_failure:
#                      the phase gains one task briefed with the checkpoint's
#                      findings whole, scoped to what its tasks write
#                      (appendCheckpointRemediation), and stays open
#   /recheck           below the cap otherwise: the phase stays open and its
#                      checkpoint runs again
#
# Until 2026-09-23 this was Go: a const cap of 3 and an unconditional replan,
# while campaign.max_checkpoint_attempts and replan_on_checkpoint_failure were
# published to the kernel and read by nothing (sweep finding F3).
Decl phase_checkpoint_failure(PhaseID, Run) bound [/string, /number].
Decl phase_ckpt_failures(PhaseID, Count) bound [/string, /number].
Decl phase_ckpt_exhausted(PhaseID) bound [/string].
Decl phase_ckpt_move(PhaseID, Move) bound [/string, /name].

config_param_required(/campaign, /campaign_max_checkpoint_attempts).
config_param_required(/campaign, /campaign_replan_on_checkpoint_failure).

phase_ckpt_failures(PhaseID, Count) :-
    phase_checkpoint_failure(PhaseID, Run)
    |> do fn:group_by(PhaseID), let Count = fn:count().

phase_ckpt_exhausted(PhaseID) :-
    phase_ckpt_failures(PhaseID, Count),
    config_param(/campaign_max_checkpoint_attempts, Max),
    Count >= Max.

phase_ckpt_move(PhaseID, /close_unverified) :-
    phase_ckpt_exhausted(PhaseID).

phase_ckpt_move(PhaseID, /replan) :-
    phase_ckpt_failures(PhaseID, Count),
    !phase_ckpt_exhausted(PhaseID),
    config_param(/campaign_replan_on_checkpoint_failure, 1).

phase_ckpt_move(PhaseID, /recheck) :-
    phase_ckpt_failures(PhaseID, Count),
    !phase_ckpt_exhausted(PhaseID),
    config_param(/campaign_replan_on_checkpoint_failure, 0).

# =============================================================================
# A generated document that is a repetition loop (sweep finding F12)
# =============================================================================
# Observed live: "1. End. 2. Finish. 3. Complete." about 1500 times, written as
# a 19KB document and counted done. Go measures the document
# (generated_output_vocab); whether it is a loop is decided here, with the
# campaign section's thresholds, and a loop fails the attempt. Until 2026-09-23
# Go decided with four constants of its own, re-generated with a prompt of its
# own, and wrote a placeholder the task then counted as done.
config_param_required(/campaign, /campaign_degenerate_min_tokens).
config_param_required(/campaign, /campaign_degenerate_distinct_permille).
config_param_required(/campaign, /campaign_degenerate_long_words).
config_param_required(/campaign, /campaign_degenerate_long_min_distinct).

generated_output_long_enough(TaskID) :-
    generated_output_vocab(TaskID, Tokens, _, _),
    config_param(/campaign_degenerate_min_tokens, Min),
    Tokens >= Min.

# Nothing but counters and punctuation.
generated_output_degenerate(TaskID) :-
    generated_output_long_enough(TaskID),
    generated_output_vocab(TaskID, _, 0, _).

# A handful of words cycling: few distinct words per thousand.
generated_output_degenerate(TaskID) :-
    generated_output_long_enough(TaskID),
    generated_output_vocab(TaskID, _, Words, Distinct),
    Words > 0,
    Permille = fn:div(fn:mult(Distinct, 1000), Words),
    config_param(/campaign_degenerate_distinct_permille, Floor),
    Permille < Floor.

# A long document from a tiny vocabulary, whatever its prefix.
generated_output_degenerate(TaskID) :-
    generated_output_long_enough(TaskID),
    generated_output_vocab(TaskID, _, Words, Distinct),
    config_param(/campaign_degenerate_long_words, Long),
    Words > Long,
    config_param(/campaign_degenerate_long_min_distinct, MinDistinct),
    Distinct < MinDistinct.
