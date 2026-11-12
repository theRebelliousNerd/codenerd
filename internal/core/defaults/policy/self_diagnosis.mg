# "Debug yourself" is a chain the kernel lays out, not one verb (chat
# self_diagnosis.go): the problems log is projected as evidence
# (self_problems.mg), a read-only shard root-causes every hot problem in the
# background, and its findings return as a turn whose fix the work mode
# admits, proposes, or holds (routing_arbitration.mg).

Decl self_diagnosis_lane() bound [].

self_diagnosis_lane() :-
    route_background(),
    user_intent(/current_intent, _, /self_diagnose, _, _),
    !jev_no("about_codenerd").

# The object of a self-diagnosis is codeNERD itself, so the turn names no
# target and is not missing one. Without this, "debug yourself" (scope.target
# empty) derived needs_clarification from the empty target and became a
# multi-round deliberation instead of this chain (live TUI, 2026-11-12).
verb_target_implied(/self_diagnose).

# A question perception filed as /self_diagnose that Jev reads as not asking
# codeNERD to investigate its own failures ("is debug-self on?", "what mode are
# you in?") is answered from the operating state the main agent is given
# (routing_arbitration.mg), not with a diagnosis job. Observed dogfooding: the
# question "what work mode are you in, and is debug-self on?" started a
# background reviewer that replied "Nothing to diagnose".
wants_direct_answer() :-
    user_intent(/current_intent, /query, /self_diagnose, _, _),
    jev_no("about_codenerd").

# A question about the assistant's own mode or settings is answered from that
# state even when about_codenerd is unsettled ("what work mode are you in, and
# is debug-self on?" scored 0.31 there): the conversation act
# question_about_codenerd (conversation_acts.mg) reads it, for any verb.

# The chain's fix step is due only when the diagnosis found something of
# codeNERD's to fix (chat background.go asks self_diagnosis_fix_due when a
# diagnosis with a follow-up returns). The chat asserts
# self_diagnosis_finished(Job) for it, and Jev's verdict per target
# (self_diagnosis_verdict, jev_evidence.mg) when the diagnosis_verdict use is
# on. Truth table, per target of a finished job; every row has an outcome:
#   verdict at or below no      self_diagnosis_no_defect   the target is settled
#                               as nothing to fix (the ledger's /no_defect);
#   verdict above no            self_diagnosis_fix_target  the fix turn covers
#                               it (an unsettled verdict still needs a look);
#   no verdict for the job      (Jev off, or its call failed) the fix turn runs
#                               for every target, as before the verdict: a
#                               missing judgment is no evidence, never a "no".
# The job's fix turn is due when any target is a fix target, or when the job
# was not judged; a job whose every target found nothing to fix starts none.
Decl self_diagnosis_finished(Job) bound [/string].
Decl self_diagnosis_judged(Job) bound [/string].
Decl self_diagnosis_no_defect(Job, Target) bound [/string, /string].
Decl self_diagnosis_fix_target(Job, Target) bound [/string, /string].
Decl self_diagnosis_fix_due(Job) bound [/string].

self_diagnosis_judged(Job) :- self_diagnosis_verdict(Job, _, _).

self_diagnosis_no_defect(Job, Target) :-
    self_diagnosis_finished(Job),
    self_diagnosis_names_no_fix(Job, Target).

self_diagnosis_fix_target(Job, Target) :-
    self_diagnosis_finished(Job),
    self_diagnosis_verdict(Job, Target, _),
    !self_diagnosis_names_no_fix(Job, Target).

self_diagnosis_fix_due(Job) :- self_diagnosis_fix_target(Job, _).
self_diagnosis_fix_due(Job) :-
    self_diagnosis_finished(Job),
    !self_diagnosis_judged(Job).
