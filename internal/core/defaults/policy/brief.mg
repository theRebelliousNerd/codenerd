# The brief: what the conversation has settled about the work, built over
# turns. Perception returns the whole updated brief with each turn's
# understanding (perception.Brief); the chat replaces these facts with it.
# Big work runs from the brief: every shard and the main agent see it below,
# and in yolo mode a brief that is not ready holds work (routing_arbitration.mg).

Decl brief_goal(Text) bound [/string].
Decl brief_constraint(Text) bound [/string].
Decl brief_decision(Text) bound [/string].
Decl brief_open_question(Text) bound [/string].
# The main agent's judgment that the brief is enough to start work.
Decl brief_marked_ready() bound [].

Decl brief_present() bound [].
Decl brief_has_open_question() bound [].
Decl brief_ready() bound [].

brief_present() :- brief_goal(Goal).
# An open question the user's latest message answers (Jev's judgment,
# jev_evidence.mg) no longer holds the work.
brief_has_open_question() :-
    brief_open_question(Question),
    !brief_question_answered(Question).

# Ready is the agent's judgment, never against an open question it still
# holds: a brief that names one is not ready whatever it claims.
brief_ready() :-
    brief_present(),
    brief_marked_ready(),
    !brief_has_open_question().

injectable_context("*", Line) :-
    brief_goal(Goal),
    Line = fn:string:concat("Brief goal: ", Goal).
injectable_context("*", Line) :-
    brief_constraint(Constraint),
    Line = fn:string:concat("Brief constraint: ", Constraint).
injectable_context("*", Line) :-
    brief_decision(Decision),
    Line = fn:string:concat("Brief decision: ", Decision).
injectable_context("*", Line) :-
    brief_open_question(Question),
    Line = fn:string:concat("Brief open question: ", Question).
