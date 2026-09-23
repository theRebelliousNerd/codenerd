# Campaign Decisions: what the orchestrator does next, derived
#
# The campaign orchestrator is a driver. It runs builds, spawns agents, writes
# the campaign file and asserts what it measured; every decision about what
# happens next is a rule here, which it queries and acts on:
#
#   verify_task_route(Task, /build | /review)  how a /verify task is judged
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
