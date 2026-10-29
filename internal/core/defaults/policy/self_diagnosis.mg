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
