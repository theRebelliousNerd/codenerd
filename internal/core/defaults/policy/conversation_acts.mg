# Conversation acts: what the user is doing with a turn, beside which task
# perception reads in it. The task vocabulary (43 verbs, taxonomy.go) has no
# word for "yes", "done", "that's bad", a pasted log or "what mode are you
# in?", so such turns were forced onto the nearest task verb and routed as
# work: feedback became a /debug job, a reply with nothing in it asked "what
# should I write?". Jev reads the act (turnjudge conversation_act, a Choice);
# jev_evidence.mg settles which option is chosen (jev_chosen, a merge
# lattice); this file says what the act does to routing. Perception still
# reads the task; the act only ever answers in the conversation or asks
# first, never starts work. A turn with no settled act routes as before.

conversation_act_option("request_for_work", /request_for_work).
conversation_act_option("reply_to_codenerd", /reply_to_codenerd).
conversation_act_option("question_about_codenerd", /question_about_codenerd).
conversation_act_option("feedback_on_codenerd", /feedback_on_codenerd).
conversation_act_option("correction_of_codenerd", /correction_of_codenerd).
conversation_act_option("shared_material", /shared_material).
conversation_act_option("stop_request", /stop_request).
conversation_act_option("small_talk", /small_talk).

act_asks_no_work(/question_about_codenerd).
act_asks_no_work(/feedback_on_codenerd).
act_asks_no_work(/shared_material).
act_asks_no_work(/stop_request).
act_asks_no_work(/small_talk).

act_answers_last_turn(/reply_to_codenerd).
act_answers_last_turn(/correction_of_codenerd).

act_answered_from_state(/question_about_codenerd).

conversation_act(Act) :-
    jev_chosen("conversation_act", Option),
    conversation_act_option(Option, Act).

conversation_act_option_known(Option) :- conversation_act_option(Option, _).
conversation_act_unmapped(Option) :-
    jev_choice("conversation_act", Option, _),
    !conversation_act_option_known(Option).

# Held work is answered by the go-ahead judgments (approve_lane): "yes" there
# is approves_held_work's to read, not the act's.
conversation_act_applies(Act) :-
    conversation_act(Act),
    !work_proposal_pending().

# Truth table, for a settled act with no work held (every other case: no row,
# perception's reading routes the turn):
#   act                     perception reads            disposition
#   request_for_work        anything                    none
#   asks no work            not a mutation; no file     answer_in_conversation
#     (question, feedback,  or tools needed, or a
#     material, stop, chat) question about codeNERD's
#                           state that names no file
#   asks no work            a mutation                  ask_first
#   asks no work            Jev also reads a request to none (the
#                           investigate codeNERD's own  self-diagnosis chain
#                           failures (about_codenerd)   may take it)
#   reply, correction       a target                    none (the task it
#                                                       carries is perception's)
#   reply, correction       no target, no file or       answer_in_conversation
#                           tools needed
# "Needs the workspace" keeps its lanes: a pasted path is read
# (gather_evidence), a question that needs a file is answered from it.
conversation_act_route(Act, /answer_in_conversation) :-
    conversation_act_applies(Act),
    act_asks_no_work(Act),
    user_intent(/current_intent, Category, _, _, _),
    /mutation != Category,
    !jev_yes("about_codenerd"),
    !needs_workspace_evidence().

# A question about codeNERD's own mode, settings or access is answered from
# the operating state the main agent is given (routing_arbitration.mg), even
# when perception named tools for it; only a file the turn names is read.
conversation_act_route(Act, /answer_in_conversation) :-
    conversation_act_applies(Act),
    act_answered_from_state(Act),
    user_intent(/current_intent, Category, _, _, _),
    /mutation != Category,
    !jev_yes("about_codenerd"),
    !intent_signal(/scope_is_file).

conversation_act_route(Act, /ask_first) :-
    conversation_act_applies(Act),
    act_asks_no_work(Act),
    user_intent(/current_intent, /mutation, _, _, _),
    !jev_yes("about_codenerd").

conversation_act_route(Act, /answer_in_conversation) :-
    conversation_act_applies(Act),
    act_answers_last_turn(Act),
    user_intent(/current_intent, _, _, Target, _),
    target_names_nothing(Target),
    !needs_workspace_evidence().

wants_direct_answer() :- conversation_act_route(_, /answer_in_conversation).

# A mutation the act says asks for no work: ask before acting
# (needs_clarification, jev_evidence.mg).
jev_disputes_mutation() :- conversation_act_route(_, /ask_first).
