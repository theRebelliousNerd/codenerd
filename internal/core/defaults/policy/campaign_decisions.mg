# Campaign Decisions: what the orchestrator does next, derived
#
# The campaign orchestrator is a driver. It runs builds, spawns agents, writes
# the campaign file and asserts what it measured; every decision about what
# happens next is a rule here, which it queries and acts on:
#
#   verify_task_route(Task, /build | /review)  how a /verify task is judged
#   checkpoint_verdict_outcome(Key, /pass | /fail | /inconclusive)
#                                              what a reviewer's verdict decides
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
Decl phase_writes_code(PhaseID) bound [/string].
Decl verify_task_route(TaskID, Route) bound [/string, /name].

phase_writes_code(PhaseID) :-
    campaign_task(TaskID, PhaseID, Desc, Status, Type),
    task_write_ext(TaskID, Ext),
    write_class(Ext, /go).

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
