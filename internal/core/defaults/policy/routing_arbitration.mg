# Routing Arbitration — the single DECIDE point for an interactive turn.
#
# Perception (fast model) emits typed facts; this file derives exactly which
# lane the turn takes. Go executes whatever derived and adds NO additional
# routing opinions of its own (it only falls back to the legacy booleans when
# the kernel is unavailable).
#
# EDB inputs (asserted by Go per turn, retract-before-assert):
#   user_intent(/current_intent, Category, Verb, Target, Constraint)
#   intent_signal(/is_question)            — perception's is_question signal
#   delegation_candidate(/current_intent, Shard, Conf)
#   multi_step_signal(Signal)
#
# Derived outputs (queried by Go):
#   route_decision(/respond_directly, /none)  — answer with prose, no shards
#   route_decision(/clarify, /none)           — ask before acting
#   route_decision(/multi_step, /none)        — decompose into steps
#   route_decision(/delegate, Shard)          — hand to a shard
#
# Lane precedence when multiple derive (Go applies in this order):
#   respond_directly > multi_step > delegate > clarify
# respond_directly is mutually exclusive with the rest by construction
# (!wants_direct_answer() on every other rule); multi_step/delegate can
# co-derive and Go prefers decomposition, matching the legacy waterfall order.

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
route_decision(/multi_step, /none) :-
    is_multi_step(),
    user_intent(/current_intent, /mutation, _, _, _),
    !wants_direct_answer().

# Delegate when the confidence gate passes (should_delegate, delegation.mg).
route_decision(/delegate, Shard) :-
    should_delegate(Shard),
    !wants_direct_answer().

# Clarify actionable-but-uncertain mutations: a shard exists for the verb but
# perception confidence is below the delegation gate.
route_decision(/clarify, /none) :-
    user_intent(/current_intent, /mutation, _, _, _),
    delegation_candidate(/current_intent, Shard, Conf),
    /none != Shard,
    Conf < 50,
    !wants_direct_answer().
