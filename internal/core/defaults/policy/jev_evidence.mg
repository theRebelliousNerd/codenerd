# Jev evidence: typed, calibrated judgments about each chat turn from TypeSafe's
# System One model (internal/turnjudge), asked in parallel with perception.
# They enhance perception and never replace it. Every rule here makes the
# system more careful or more attentive: ask before acting, recognise a
# go-ahead, close a brief question the user has answered, start the
# self-diagnosis chain. None starts work that the other lanes would not start.
# A turn with no judgments (Jev off, or its call failed) routes exactly as
# before: a missing judgment is no evidence, never a "no".

# jev_judgment(Name, Percent): the probability, as a whole percent, that the
# named judgment is true (turnjudge.Result.Facts).
Decl jev_judgment(Name, Percent) bound [/string, /number].
# jev_judgment_about(Name, Subject): what an answered_N judgment asks about,
# the open brief question's text.
Decl jev_judgment_about(Name, Subject) bound [/string, /string].

Decl jev_yes(Name) bound [/string].
Decl jev_no(Name) bound [/string].
Decl jev_disputes_mutation() bound [].
Decl brief_question_answered(Question) bound [/string].

config_param_required(/jev, /jev_yes_percent).
config_param_required(/jev, /jev_no_percent).

jev_yes(Name) :-
    jev_judgment(Name, Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.
jev_no(Name) :-
    jev_judgment(Name, Percent),
    config_param(/jev_no_percent, No),
    Percent <= No.

# Ask before a mutation Jev reads differently: the user is not asking for
# files to change, the request does not say which code, or it only asks what
# would happen. Perception still decides what the work is; this only makes the
# clarify lane ask first.
jev_disputes_mutation() :-
    user_intent(/current_intent, /mutation, _, _, _),
    jev_no("changes_files").
jev_disputes_mutation() :-
    user_intent(/current_intent, /mutation, _, _, _),
    jev_no("names_target").
jev_disputes_mutation() :-
    user_intent(/current_intent, /mutation, _, _, _),
    jev_yes("hypothetical").

needs_clarification() :- jev_disputes_mutation().

# A go-ahead perception did not read as one ("yes, ship it" classified as a
# fix): Jev says the user approves the held work and asks for nothing beyond
# it ("go, and also add a test" is a new request, not the held one).
approve_lane() :-
    work_proposal_pending(),
    jev_yes("approves_held_work"),
    jev_no("adds_to_held_work").

# An open brief question the latest message answers stops holding the work
# (brief.mg reads it), even when perception kept it open.
brief_question_answered(Question) :-
    jev_judgment_about(Name, Question),
    jev_yes(Name),
    brief_open_question(Question).

# Read-only work about codeNERD's own failures is the self-diagnosis chain,
# whatever verb perception chose for it.
self_diagnosis_lane() :-
    route_background(),
    jev_yes("about_codenerd").

# "Stop" while background work runs stops it (chat background.go cancels
# every running job; each reports back as stopped).
Decl stop_background_requested() bound [].
stop_background_requested() :-
    jev_yes("wants_to_stop"),
    background_job(JobID, Shard, Task, /running).
