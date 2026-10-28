# Jev evidence: typed, calibrated judgments about each chat turn from TypeSafe's
# System One model (internal/turnjudge), asked in parallel with perception.
# They enhance perception and never replace it. Every rule here makes the
# system more careful or more attentive: ask before acting, recognise a
# go-ahead, close a brief question the user has answered, start the
# self-diagnosis chain, stop background work, add a restriction; and, for
# Jev's other uses, hand the model a file Jev judged relevant, flag a review
# finding Jev cannot see in its code, list a problem apart as a failure
# around codeNERD. None starts work that the other lanes would not start.
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

# A request Jev reads as wanting files changed, with no file, function or
# feature named, asks first, whatever category perception gave it. "Make the
# word handling better" arrived as a low-confidence /query /review: no lane
# derived, and the legacy delegate_task path ran a reviewer on it. A turn that
# answers held work is not a request: it names nothing because the held work
# does.
needs_clarification() :-
    user_intent(/current_intent, _, _, _, _),
    jev_yes("changes_files"),
    jev_no("names_target"),
    !work_proposal_pending(),
    !approve_lane().

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

# A reviewer finding Jev cannot see in the code it cites (turnjudge
# CheckFindings): the review is flagged for validation (reviewer.mg's
# reviewer_needs_validation), and the finding is named to the model. Nothing
# is hidden on Jev's word; the person and the model decide.
Decl review_finding_check(ReviewID, File, Line, Percent) bound [/string, /string, /number, /number].
Decl review_finding_refuted(ReviewID, File, Line) bound [/string, /string, /number].

review_finding_refuted(ReviewID, File, Line) :-
    review_finding_check(ReviewID, File, Line, Percent),
    config_param(/jev_no_percent, No),
    Percent <= No.

review_suspect(ReviewID, "jev_refuted_finding") :-
    review_finding_refuted(ReviewID, File, Line).

injectable_context("*", Text) :-
    review_finding_refuted(ReviewID, File, Line),
    Text = fn:string:concat("A review finding at ", File, ":", fn:number:to_string(Line), " is not visible in the code it cites; verify it before acting on it.").

# Restrictions Jev reads that perception's vocabulary missed ("just look,
# don't touch anything"; a what-if). The chat merges them into the turn's
# restriction (perception.Restriction.Merge), which only ever adds kinds, so
# constitution.mg section 7D then bars the effects. Jev can make a turn more
# restricted, never less.
Decl jev_restriction(Kind) bound [/name].
jev_restriction(/read_only) :- jev_yes("rules_out_changes").
jev_restriction(/hypothetical) :- jev_yes("hypothetical").

# Retrieval relevance (turnjudge.RelevanceJudge, retrieval/relevance.go):
# context_relevance(Issue, File, Percent, Kept) is Jev's probability that
# reading a Tier 2 candidate helps with the issue (Decl: schemas_knowledge.mg
# 52.2). A file Jev judges relevant is handed to the model in the retrieval
# brief whatever its keyword score: the keyword floor
# (retrieval.brief_min_relevance) measures word overlap, Jev reads the file.
# A file Jev judged irrelevant never reaches tiered_context_file.
retrieval_brief_file(Issue, File, Tier, Relevance) :-
    tiered_context_file(Issue, File, Tier, Relevance, _),
    context_relevance(Issue, File, Percent, /true),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

# Self-diagnosis triage (turnjudge.TriageProblems): Jev's probability that a
# logged problem is a defect in codeNERD's own code. At or below no it is a
# failure around codeNERD (network, a provider, the configuration, the
# machine), which the chain lists apart from the defects it works first
# (chat self_diagnosis.go). Unsettled stays a defect: it still needs a look.
Decl self_problem_triage(Signature, Category, Percent) bound [/string, /string, /number].
Decl self_problem_around(Signature, Category) bound [/string, /string].

self_problem_around(Signature, Category) :-
    self_problem_triage(Signature, Category, Percent),
    config_param(/jev_no_percent, No),
    Percent <= No.
