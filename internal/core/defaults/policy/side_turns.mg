# Side turns: a message typed while a turn runs work (chat side_turn.go). The
# running turn keeps its own per-turn facts, so a side turn is read into
# these predicates only, and the kernel decides what it is:
#   /stop      cancel the running turn (Jev: the user asks to stop);
#   /later     a request for work of its own, run as a full turn when the
#              running one ends (perception reads a mutation, or Jev says
#              the user asks for files to change; or no model read it and a
#              retry can settle that -- classification_action, perception_outcome.mg);
#   /status    a question about how the running work is going (Jev: the
#              user asks how running_work is going): answered from what the
#              work has done (running_work_wrote below, and the agents'
#              transcripts), never from a model's guess; nothing changes;
#   /converse  a reply, nothing more.

Decl side_turn_intent(Category, Verb) bound [/name, /name].
Decl side_turn_judgment(Name, Percent) bound [/string, /number].
Decl side_turn_wants(Action) bound [/name].
Decl side_turn_action(Action) bound [/name].

side_turn_wants(/status) :-
    side_turn_judgment("asks_about_running_work", Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

side_turn_wants(/stop) :-
    side_turn_judgment("wants_to_stop", Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

side_turn_wants(/later) :-
    side_turn_intent(/mutation, Verb).
side_turn_wants(/later) :-
    side_turn_judgment("changes_files", Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

# A message no model could read is not conversation to drop: when a retry can
# settle the failure (an outage, a rate limit, an unreadable answer) it waits
# for the running turn and is read again as a full turn. A blocker (rejected
# credentials) or a canceled read queues nothing: the same message would fail
# the same way, and the notice says what to change.
side_turn_wants(/later) :-
    classification_action(/side_turn, /retry).

side_turn_action(/stop) :-
    side_turn_wants(/stop).
side_turn_action(/later) :-
    side_turn_wants(/later),
    !side_turn_wants(/stop).
side_turn_action(/status) :-
    side_turn_wants(/status),
    !side_turn_wants(/stop),
    !side_turn_wants(/later).
side_turn_action(/converse) :-
    side_turn_intent(Category, Verb),
    !side_turn_wants(/stop),
    !side_turn_wants(/later),
    !side_turn_wants(/status).

# The brief. Perception reads a side turn with the running brief and returns
# one; Go asserts both, item by item (side_turn_brief, side_turn_base_brief:
# the brief the message was read against), and the kernel decides whether the
# message changed the task. It does only when the brief it returned differs
# from the one it read, it is plain conversation (/converse), and it is not a
# question: a status question, a stop, and deferred work (read again, brief
# and all, when it runs) change nothing, and say nothing about the brief.
# Until 2026-10-30 any returned brief was applied, or refused with a note
# when the brief had moved meanwhile, so "how is progress?" was told its
# change to the brief was not applied.
Decl side_turn_question() bound [].
Decl side_turn_returned_brief() bound [].
Decl side_turn_brief(Kind, Text) bound [/name, /string].
Decl side_turn_base_brief(Kind, Text) bound [/name, /string].
Decl side_turn_brief_moved() bound [].
Decl side_turn_work_ended() bound [].
Decl side_turn_brief_delta(Change, Kind, Text) bound [/name, /name, /string].
Decl side_turn_changes_task() bound [].
Decl side_turn_brief_update(Disposition) bound [/name].

side_turn_brief_delta(/adds, Kind, Text) :-
    side_turn_brief(Kind, Text),
    !side_turn_base_brief(Kind, Text).
side_turn_brief_delta(/drops, Kind, Text) :-
    side_turn_returned_brief(),
    side_turn_base_brief(Kind, Text),
    !side_turn_brief(Kind, Text).

side_turn_changes_task() :-
    side_turn_brief_delta(_, _, _),
    side_turn_action(/converse),
    !side_turn_question().

# /apply: the brief is the message's. /conflict: the brief moved while the
# message was read, so applying it would undo that change. /ended: the work
# it was about has ended.
side_turn_brief_update(/apply) :-
    side_turn_changes_task(),
    !side_turn_brief_moved(),
    !side_turn_work_ended().
side_turn_brief_update(/conflict) :-
    side_turn_changes_task(),
    side_turn_brief_moved(),
    !side_turn_work_ended().
side_turn_brief_update(/ended) :-
    side_turn_changes_task(),
    side_turn_work_ended().

# What the running work has written: every file_written row stamped at or
# after the foreground execution began (running_work_began, asserted by the
# side turn from the turn's own clock).
Decl running_work_began(Unix) bound [/number].
Decl running_work_wrote(Path) bound [/string].

running_work_wrote(Path) :-
    running_work_began(Start),
    file_written(Path, _, _, At),
    At >= Start.
