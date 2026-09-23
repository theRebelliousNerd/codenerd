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
#   delegation_candidate(/current_intent, Shard, Conf)
#   multi_step_signal(Signal)
#   config_param(/routing_delegation_min_confidence, Min)  (routing section)
#
# Derived outputs (queried by Go):
#   route_decision(/respond_directly, /none)  — answer with prose, no shards
#   route_decision(/clarify, /none)           — ask before acting
#   route_decision(/multi_step, /none)        — decompose into steps
#   route_decision(/delegate, Shard)          — hand to a shard
#
# Lane precedence, derived here so that one lane at most holds:
#   respond_directly > multi_step > delegate > clarify
# respond_directly excludes the rest (!wants_direct_answer() on every other
# lane); multi_step excludes delegate and clarify (!multi_step_lane()); a
# single candidate cannot be both at and below the confidence threshold, so
# delegate and clarify exclude each other by construction.

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

route_decision(/respond_directly, /none) :-
    wants_direct_answer().

# Decompose multi-step MUTATIONS only. Questions are never decomposed — a
# multi-part question is still answered in one prose pass.
multi_step_lane() :-
    is_multi_step(),
    user_intent(/current_intent, /mutation, _, _, _),
    !wants_direct_answer().

route_decision(/multi_step, /none) :-
    multi_step_lane().

# Delegate when the confidence gate passes (should_delegate, delegation.mg),
# unless the turn decomposes: the decomposition runs the steps.
route_decision(/delegate, Shard) :-
    should_delegate(Shard),
    !wants_direct_answer(),
    !multi_step_lane().

# Clarify actionable-but-uncertain mutations: a shard exists for the verb but
# perception confidence is below the delegation gate. Not twice for the same
# input: a user who sends back the request the last clarification asked
# about has declined to narrow it, and asking again loops. The Go clarifiers
# have always stood down on that input; the derived lane did not, and asked
# forever (TestE2E_TaskRouting_ClarifierLoopGuard_SameInputNoReClarify, red
# from 2026-09-18 until 2026-09-23).
route_decision(/clarify, /none) :-
    user_intent(/current_intent, /mutation, _, _, _),
    delegation_candidate(/current_intent, Shard, Conf),
    /none != Shard,
    config_param(/routing_delegation_min_confidence, Min),
    Conf < Min,
    !wants_direct_answer(),
    !multi_step_lane(),
    !intent_signal(/clarified_already).
