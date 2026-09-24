# Campaign Evidence: what a task is handed from the work before it, derived
#
# A campaign task's brief carries the durable outputs of the tasks before it.
# Which ones, and in what form, is decided here:
#
#   task_evidence(Task, Path, /inline | /digest | /handle)
#
#   /inline  the whole artifact, never cut: the task depends on it (a declared
#            dependency edge) or its brief names it, and it is at most
#            campaign.upstream_inline_max_bytes
#   /digest  the artifact's outline, findings and citations with a recall
#            handle for the rest: a needed artifact over that size, or one the
#            task sits next to (an earlier task of its phase, or a phase its
#            phase depends on directly)
#   /handle  one line with a recall handle: an artifact only a transitive
#            upstream phase produced
#
# Until 2026-09-23 this was Go (internal/campaign/upstream_context.go): every
# /doc artifact of every transitive upstream phase, newest phase first, cut at
# 12 KiB each and 48 KiB in all, so later tasks got "5 of 9" artifacts by
# position -- a relevance decision made by a byte counter -- and the ones that
# made it were truncated mid-text. Campaign 7b853890's anchors were 46-48 KB on
# every round, while briefs named Docs/ files the collector never considered
# and 84% of the model's read_file calls read a file its anchor named.
#
# Go measures and asserts; it decides nothing here:
#   task_artifact_on_disk(Task, Path, ArtifactType, Ext, Bytes)
#       a declared artifact of the live campaign that is a regular file
#   task_brief_names(Task, Path)
#       the asked task's brief names the path, or its file name
#   task_output_path(Task, Path)
#       the asked task's own declared output (an artifact or write-set entry
#       that is the same file): what a task writes is not its evidence
# The facts live on the campaign shard (internal/shards/registration.go), next
# to campaign_task, task_dependency, task_order and phase_dependency.

Decl task_artifact_on_disk(TaskID, Path, ArtifactType, Ext, Bytes) bound [/string, /string, /name, /string, /number].
Decl task_brief_names(TaskID, Path) bound [/string, /string].
Decl task_output_path(TaskID, Path) bound [/string, /string].

config_param_required(/campaign, /campaign_upstream_inline_max_bytes).

# -----------------------------------------------------------------------------
# What counts as evidence: a completed task's prose output with content
# -----------------------------------------------------------------------------
# /doc is the campaign's own durable output (an analysis task's findings, a
# review); a Docs/ file a task created or modified is prose too, by its
# extension's write class (coder_safety.mg). Code is not handed over as prose.
Decl evidence_artifact(Producer, Path, Bytes) bound [/string, /string, /number].
Decl evidence_bytes(Path, Bytes) bound [/string, /number].

evidence_artifact(Producer, Path, Bytes) :-
    campaign_task(Producer, PhaseID, Desc, /completed, Type),
    task_artifact_on_disk(Producer, Path, /doc, Ext, Bytes),
    Bytes > 0.

evidence_artifact(Producer, Path, Bytes) :-
    campaign_task(Producer, PhaseID, Desc, /completed, Type),
    task_artifact_on_disk(Producer, Path, ArtType, Ext, Bytes),
    write_class(Ext, /doc),
    Bytes > 0.

evidence_bytes(Path, Bytes) :-
    evidence_artifact(Producer, Path, Bytes).

# -----------------------------------------------------------------------------
# Where the task stands relative to the artifact
# -----------------------------------------------------------------------------
# The phases a phase depends on for evidence, directly and transitively. A
# /soft dependency is an ordering preference, not an input.
Decl phase_evidence_edge(PhaseID, UpstreamPhaseID) bound [/string, /string].
Decl phase_upstream(PhaseID, UpstreamPhaseID) bound [/string, /string].

phase_evidence_edge(PhaseID, Up) :-
    phase_dependency(PhaseID, Up, /hard).

phase_evidence_edge(PhaseID, Up) :-
    phase_dependency(PhaseID, Up, /artifact).

phase_upstream(PhaseID, Up) :-
    phase_evidence_edge(PhaseID, Up).

phase_upstream(PhaseID, Up) :-
    phase_evidence_edge(PhaseID, Mid),
    phase_upstream(Mid, Up).

# needed: the task consumes it -- a declared dependency edge, or its brief
# names it.
Decl task_evidence_needed(TaskID, Path) bound [/string, /string].

task_evidence_needed(TaskID, Path) :-
    task_dependency(TaskID, Producer),
    evidence_artifact(Producer, Path, Bytes).

task_evidence_needed(TaskID, Path) :-
    task_brief_names(TaskID, Path),
    evidence_artifact(Producer, Path, Bytes),
    Producer != TaskID.

# near: an earlier task of the task's own phase, or a phase its phase depends
# on directly.
Decl task_evidence_near(TaskID, Path) bound [/string, /string].

task_evidence_near(TaskID, Path) :-
    campaign_task(TaskID, PhaseID, Desc, Status, Type),
    campaign_task(Producer, PhaseID, PDesc, /completed, PType),
    Producer != TaskID,
    task_order(Producer, ProducerOrder),
    task_order(TaskID, Order),
    ProducerOrder < Order,
    evidence_artifact(Producer, Path, Bytes).

task_evidence_near(TaskID, Path) :-
    campaign_task(TaskID, PhaseID, Desc, Status, Type),
    phase_evidence_edge(PhaseID, Up),
    campaign_task(Producer, Up, PDesc, /completed, PType),
    evidence_artifact(Producer, Path, Bytes).

# far: any phase upstream of the task's phase, however far. A cyclic plan does
# not make a phase its own upstream.
Decl task_evidence_far(TaskID, Path) bound [/string, /string].

task_evidence_far(TaskID, Path) :-
    campaign_task(TaskID, PhaseID, Desc, Status, Type),
    phase_upstream(PhaseID, Up),
    Up != PhaseID,
    campaign_task(Producer, Up, PDesc, /completed, PType),
    evidence_artifact(Producer, Path, Bytes).

# -----------------------------------------------------------------------------
# The form it is handed in
# -----------------------------------------------------------------------------
Decl task_evidence(TaskID, Path, Mode) bound [/string, /string, /name].

task_evidence(TaskID, Path, /inline) :-
    task_evidence_needed(TaskID, Path),
    !task_output_path(TaskID, Path),
    evidence_bytes(Path, Bytes),
    config_param(/campaign_upstream_inline_max_bytes, Max),
    Bytes <= Max.

task_evidence(TaskID, Path, /digest) :-
    task_evidence_needed(TaskID, Path),
    !task_output_path(TaskID, Path),
    evidence_bytes(Path, Bytes),
    config_param(/campaign_upstream_inline_max_bytes, Max),
    Bytes > Max.

task_evidence(TaskID, Path, /digest) :-
    task_evidence_near(TaskID, Path),
    !task_evidence_needed(TaskID, Path),
    !task_output_path(TaskID, Path).

task_evidence(TaskID, Path, /handle) :-
    task_evidence_far(TaskID, Path),
    !task_evidence_near(TaskID, Path),
    !task_evidence_needed(TaskID, Path),
    !task_output_path(TaskID, Path).

# -----------------------------------------------------------------------------
# A /verify task's report is hollow
# -----------------------------------------------------------------------------
# A /verify task checks the deliverables of the tasks it depends on: the prose a
# /document, /file_create or /file_modify task wrote. The report is hollow when
# it is under campaign.verify_report_min_bytes while its producer was owed
# evidence -- the shape of audit campaign a19dd99f's "No input content
# supplied" phase-4 report, written with nothing in front of it.
#
# Until 2026-09-23 Go decided this from text: a 40-rune floor and five phrases
# ("no findings", "nothing to verify", ...) over the upstream artifacts and the
# report, whose path it guessed from the verify task's first artifact, a regex
# over its description, or the first upstream artifact. For a decomposer's
# /verify the first artifact is its own persisted review, so a retry judged the
# verify's previous clean review ("no findings") hollow. What a report SAYS has
# no typed signal -- a clean review and an empty one use the same words -- so
# that is the reviewer's to judge (verify_task_route /review), not a phrase
# list's.
Decl verify_report_task_type(TaskType) bound [/name].
Decl verify_report(VerifyID, Producer, Path, Bytes) bound [/string, /string, /string, /number].
Decl task_evidence_count(TaskID, Count) bound [/string, /number].
Decl verify_report_hollow(VerifyID, Path, Bytes, Owed) bound [/string, /string, /number, /number].

config_param_required(/campaign, /campaign_verify_report_min_bytes).

verify_report_task_type(/document).
verify_report_task_type(/file_create).
verify_report_task_type(/file_modify).

verify_report(VerifyID, Producer, Path, Bytes) :-
    campaign_task(VerifyID, PhaseID, Desc, Status, /verify),
    task_dependency(VerifyID, Producer),
    campaign_task(Producer, PPhase, PDesc, /completed, PType),
    verify_report_task_type(PType),
    evidence_artifact(Producer, Path, Bytes).

task_evidence_count(TaskID, Count) :-
    task_evidence(TaskID, Path, Mode)
    |> do fn:group_by(TaskID), let Count = fn:count().

verify_report_hollow(VerifyID, Path, Bytes, Owed) :-
    verify_report(VerifyID, Producer, Path, Bytes),
    task_evidence_count(Producer, Owed),
    config_param(/campaign_verify_report_min_bytes, Min),
    Bytes < Min.
