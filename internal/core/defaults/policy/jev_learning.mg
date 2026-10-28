# Learning-loop trigger: whether the user's latest message corrects something
# codeNERD did or said in its last turn (cmd/nerd/chat/jev_correction.go). Jev
# reads the last exchange (internal/learnjudge/correction.go); these rules
# decide. The learning loop runs in place of the turn only when the kernel
# derives learning_correction.
#
# Until 2026-10-28 a keyword match decided in Go before perception ran: "stop",
# "wrong" or "mistake" anywhere in a short message hijacked the turn into the
# learning loop. The keyword reading survives only as the fallback evidence for
# a turn Jev did not judge (Jev off, the correction_judgments switch off, the
# call failed or timed out), and a Jev judgment, whatever it says, outranks it.

# correction_judgment(Percent): Jev's probability, as a whole percent, that the
# latest message tells codeNERD its last reply or action was wrong.
Decl correction_judgment(Percent) bound [/number].
# correction_rebuke_phrase(): the latest message holds a rebuke phrase
# (perception.IsRebuke).
Decl correction_rebuke_phrase() bound [].

Decl correction_judged() bound [].
# learning_correction(Source): the turn is a correction to learn from; Source
# is /jev or /rebuke_phrase.
Decl learning_correction(Source) bound [/name].

correction_judged() :- correction_judgment(Percent).

# Truth table:
#   Jev judgment            rebuke phrase   learning_correction
#   at or above yes         any             /jev
#   below yes (no or        any             none: the turn routes as usual
#     unsettled)
#   none (off or failed)    yes             /rebuke_phrase
#   none                    no              none
learning_correction(/jev) :-
    correction_judgment(Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

learning_correction(/rebuke_phrase) :-
    correction_rebuke_phrase(),
    !correction_judged().
