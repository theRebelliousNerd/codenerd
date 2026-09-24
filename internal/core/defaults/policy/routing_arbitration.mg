# Routing Arbitration — the single DECIDE point for an interactive turn.
#
# Perception (fast model) emits typed facts; this file derives which lane the
# turn takes. Go executes the lane that derived and has no routing opinion of
# its own. At most one lane derives, and none is a decision too: the turn is
# answered by articulation, with nothing delegated or decomposed. A kernel
# that cannot be asked routes the same way (cmd/nerd/chat/delegation_routing.go).
# Until 2026-09-23 an empty derivation fell back to Go copies of the delegation
# and multi-step gates, which answered in place of the kernel's "no" (sweep
# finding F12).
#
# EDB inputs (asserted by Go per turn, retract-before-assert):
#   user_intent(/current_intent, Category, Verb, Target, Constraint)
#   intent_signal(/is_question)            — perception's is_question signal
#   intent_signal(/clarified_already)      — the input the last clarification
#                                            asked about, sent again
#   intent_signal(/has_surface_response)   — perception already wrote a reply
#   delegation_candidate(/current_intent, Shard, Conf)
#   multi_step_signal(Signal)
#   multi_step_plan_step(Index, Shard)     — the decomposition /multi_step
#                                            would run, one row per step
#   config_param(/routing_delegation_min_confidence, Min)  (routing section)
#
# Derived outputs (queried by Go):
#   route_decision(/perception_answer, /none) — perception's reply is the answer
#   route_decision(/dream, /none)             — hypothetical: consult, act on nothing
#   route_decision(/assault, /none)           — an adversarial campaign on the workspace
#   route_decision(/respond_directly, /none)  — answer with prose, no shards
#   route_decision(/clarify, /none)           — ask before acting
#   route_decision(/multi_step, /none)        — decompose into steps
#   route_decision(/delegate, Shard)          — hand to a shard
#
# Lane precedence, derived here so that one lane at most holds:
#   perception_answer > dream > assault > respond_directly > multi_step > clarify > delegate
# The three early lanes exclude everything after them (!early_lane());
# respond_directly excludes the rest (!wants_direct_answer()); multi_step
# excludes clarify and delegate (!multi_step_lane()); clarify excludes delegate
# (!clarify_lane()) -- asking first is what the Go clarifiers that stood
# before this file did.
#
# Until 2026-09-23 three of these were Go decisions made before this file was
# asked: a conversational "fast path" (a verb table and target checks in
# isConversationalIntent), a /dream verb check, and two heuristic clarifiers
# (shouldClarifyIntent, and shouldAutoClarify -- a keyword match over the raw
# input for "plan", "project", "feature", ... that could override a derived
# /delegate). They are rules below; the keyword match is gone.

# =============================================================================
# Vocabulary
# =============================================================================

# Verbs whose outcome is always prose — never shard work.
conversational_verb(/greet).
conversational_verb(/converse).
conversational_verb(/help).
conversational_verb(/knowledge).
conversational_verb(/shadow).
conversational_verb(/configure).
conversational_verb(/explain).

# Verbs that delegate even when phrased as a question. "Can you review my
# code?" is an action request; the question mark is politeness, not intent.
# /research is deliberately absent: a question that happens to need research
# ("what is X?") must answer directly; an imperative "research X and write it
# up" arrives with is_question=false and delegates normally.
workhorse_verb(/review).
workhorse_verb(/review_enhance).
workhorse_verb(/security).
workhorse_verb(/audit).
workhorse_verb(/test).
workhorse_verb(/benchmark).
workhorse_verb(/profile).
workhorse_verb(/lint).
workhorse_verb(/fix).
workhorse_verb(/refactor).
workhorse_verb(/create).
workhorse_verb(/write).
workhorse_verb(/delete).
workhorse_verb(/debug).
workhorse_verb(/git).
workhorse_verb(/migrate).
workhorse_verb(/optimize).
workhorse_verb(/document).
workhorse_verb(/format).
workhorse_verb(/scaffold).
workhorse_verb(/campaign).
workhorse_verb(/assault).
workhorse_verb(/generate_tool).

# =============================================================================
# Direct answer detection
# =============================================================================

# Conversational verbs always answer directly, question or not.
wants_direct_answer() :-
    user_intent(/current_intent, _, Verb, _, _),
    conversational_verb(Verb).

# Questions answer directly unless the verb is an explicit workhorse request.
# This is the rule that stops "what is the JIT system?" from becoming a
# 20-minute reviewer/researcher delegation: investigate/analyze/research
# classifications of a question still terminate in prose.
wants_direct_answer() :-
    user_intent(/current_intent, /query, Verb, _, _),
    intent_signal(/is_question),
    !workhorse_verb(Verb).

# =============================================================================
# Lanes
# =============================================================================

# The perception pass already answered, and nothing in the workspace is
# needed to answer (the latency lane): its reply is the turn's answer, with no
# articulation pass. Only for a turn with no shard to hand to.
perception_answer_verb(/greet).
perception_answer_verb(/converse).
perception_answer_verb(/help).
perception_answer_verb(/knowledge).
perception_answer_verb(/shadow).
perception_answer_verb(/dream).
perception_answer_verb(/configure).
# A read of nothing in particular, and questions about the agent itself. A
# codebase /explain goes through articulation, which can ask for knowledge.
perception_answer_target(/read, "").
perception_answer_target(/read, "none").
perception_answer_target(/explain, "capabilities").
perception_answer_target(/explain, "session").

perception_answer_lane() :-
    intent_signal(/has_surface_response),
    user_intent(/current_intent, _, Verb, _, _),
    perception_answer_verb(Verb),
    delegation_candidate(/current_intent, /none, _).
perception_answer_lane() :-
    intent_signal(/has_surface_response),
    user_intent(/current_intent, _, Verb, Target, _),
    perception_answer_target(Verb, Target),
    delegation_candidate(/current_intent, /none, _).

# "What if ...": consult the shards, execute nothing. A /dream turn perception
# already answered takes the lane above (the routing matrix pins that order).
dream_lane() :-
    user_intent(/current_intent, _, /dream, _, _),
    !perception_answer_lane().

# An adversarial assault on the workspace -- a soak, stress or torture test
# campaign -- is a campaign of its own (/campaign assault): a long run with its
# own artifacts, not one shard's task and not prose, whether or not it is
# phrased as a question. Perception names it: /assault, whose taxonomy entry
# carries the soak/stress/torture/gauntlet phrasings. Until 2026-09-24 this was
# a Go check made before this file was asked, which also keyword-matched the
# raw input of a /campaign turn for "assault", "stress test", "adversarial"...
assault_lane() :-
    user_intent(/current_intent, _, /assault, _, _),
    !perception_answer_lane().

early_lane() :- perception_answer_lane().
early_lane() :- dream_lane().
early_lane() :- assault_lane().

route_decision(/perception_answer, /none) :-
    perception_answer_lane().

route_decision(/dream, /none) :-
    dream_lane().

route_decision(/assault, /none) :-
    assault_lane().

route_decision(/respond_directly, /none) :-
    wants_direct_answer(),
    !early_lane().

# Decompose multi-step MUTATIONS only. Questions are never decomposed — a
# multi-part question is still answered in one prose pass. And only into a
# plan: a request the decomposition cannot split is delegated whole.
multi_step_lane() :-
    is_multi_step(),
    multi_step_plan_ready(),
    user_intent(/current_intent, /mutation, _, _, _),
    !wants_direct_answer(),
    !early_lane().

# A plan is two steps or more, and every step names the shard that runs it:
# the multi-step executor skips a step without one, so the request would run
# in part. The decomposer's clause parsing also yields steps like "/the" or
# "/greet" (a synonym found inside "this") from a single request; they name
# no shard. Until 2026-09-23 Go checked only the step count, after the lane had
# derived: a one-step plan fell through, and the turn was neither decomposed
# nor delegated.
multi_step_plan_unrouted() :-
    multi_step_plan_step(_, /none).

multi_step_plan_ready() :-
    multi_step_plan_step(I, _),
    multi_step_plan_step(J, _),
    I < J,
    !multi_step_plan_unrouted().

route_decision(/multi_step, /none) :-
    multi_step_lane().

# Delegate when the confidence gate passes (should_delegate, delegation.mg),
# unless the turn decomposes (the decomposition runs the steps) or must ask
# first.
route_decision(/delegate, Shard) :-
    should_delegate(Shard),
    !wants_direct_answer(),
    !multi_step_lane(),
    !clarify_lane(),
    !early_lane().

# A delegated mutation runs the verification loop: the persona's attempts,
# each judged (delegation_move, delegation.mg). Read-only work -- a review, an
# analysis -- has nothing written to verify, and each attempt's extra judge
# call would only be latency. Was shouldVerifyDelegation, in Go, until
# 2026-09-24.
route_verifies() :-
    route_decision(/delegate, _),
    user_intent(/current_intent, /mutation, _, _, _).

# Clarify before acting, and not twice for the same input: a user who sends
# back the request the last clarification asked about has declined to narrow
# it, and asking again loops. The Go clarifiers always stood down on that
# input; the derived lane did not, and asked forever
# (TestE2E_TaskRouting_ClarifierLoopGuard_SameInputNoReClarify, red from
# 2026-09-18 until 2026-09-23).
route_decision(/clarify, /none) :-
    clarify_lane().

clarify_lane() :-
    needs_clarification(),
    !wants_direct_answer(),
    !multi_step_lane(),
    !early_lane(),
    !intent_signal(/clarified_already).

# An actionable-but-uncertain mutation: a shard exists for the verb but
# perception confidence is below the delegation gate.
needs_clarification() :-
    user_intent(/current_intent, /mutation, _, _, _),
    delegation_candidate(/current_intent, Shard, Conf),
    /none != Shard,
    config_param(/routing_delegation_min_confidence, Min),
    Conf < Min.

# A request whose verb acts on something and that names nothing to act on
# ("fix it", "run the tests", "plan a new auth system"): ask what, before any
# lane acts on a guess. Was the target check of shouldClarifyIntent and the
# "needs details" of shouldAutoClarify, in Go.
acts_on_target(/search).
acts_on_target(/run).
acts_on_target(/test).
acts_on_target(/diff).
acts_on_target(/git).
acts_on_target(/build).
acts_on_target(/fix).
acts_on_target(/refactor).
acts_on_target(/review).
acts_on_target(/generate).
acts_on_target(/create).
acts_on_target(/scaffold).

target_names_nothing("").
target_names_nothing("none").

needs_clarification() :-
    user_intent(/current_intent, _, Verb, Target, _),
    acts_on_target(Verb),
    target_names_nothing(Target).
needs_clarification() :-
    user_intent(/current_intent, _, _, Target, _),
    target_names_nothing(Target),
    delegation_candidate(/current_intent, Shard, _),
    /none != Shard.

# The kernel already has a question for this intent (clarification.mg: an
# unknown or unmapped verb, an unreachable model).
needs_clarification() :-
    clarification_question(/current_intent, _).
