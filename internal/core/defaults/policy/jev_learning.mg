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

# Tool-need trigger: whether a request, and a failed attempt at it, is a
# capability gap a generated tool should fill (autopoiesis/tool_need.go). Jev
# reads the request and the attempt (internal/learnjudge/toolneed.go). Until
# 2026-10-28 a capability regex, or any failed attempt at all, made a tool need
# in Go. The regex survives as fallback evidence for a request Jev did not
# judge; a failed attempt alone is never a gap.

# tool_need_judgment(Request, Name, Percent): Jev's probability, as a whole
# percent, that the named judgment holds for the request (a hash of its text).
Decl tool_need_judgment(Request, Name, Percent) bound [/string, /string, /number].
# tool_need_pattern(Request): a missingCapabilityPatterns regex matched.
Decl tool_need_pattern(Request) bound [/string].

Decl tool_need_judged(Request) bound [/string].
Decl tool_need_yes(Request, Name) bound [/string, /string].
# tool_gap_observed(Request, Source): the kernel's decision that the request is
# a capability gap; Source is /jev or /pattern.
Decl tool_gap_observed(Request, Source) bound [/string, /name].

tool_need_judged(Request) :- tool_need_judgment(Request, Name, Percent).

tool_need_yes(Request, Name) :-
    tool_need_judgment(Request, Name, Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

# Truth table:
#   asks_for_tool  failed_for_missing_tool  needs_missing_operation  gap
#   yes            any                      any                      /jev
#   not yes        yes                      yes                      /jev
#   otherwise                                                        none
#   no judgment, and the capability pattern matched                  /pattern
# A large task the harness can already do is not a gap: the coder does it.
tool_gap_observed(Request, /jev) :-
    tool_need_yes(Request, "asks_for_tool").
tool_gap_observed(Request, /jev) :-
    tool_need_yes(Request, "failed_for_missing_tool"),
    tool_need_yes(Request, "needs_missing_operation").
tool_gap_observed(Request, /pattern) :-
    tool_need_pattern(Request),
    !tool_need_judged(Request).

# Tool value: Jev's observations about generated tools (autopoiesis/
# tool_value.go). The measured ledger (cost, condensation, use counts) decides
# a tool's value class; these rules turn Jev's readings into evidence it reads.

# tool_call_hit(Tool, Call, Percent): Jev's probability that one call's output
# gave the caller what its input asked for.
Decl tool_call_hit(Tool, Call, Percent) bound [/string, /string, /number].
# tool_duplicate_judgment(Tool, Other, Percent): Jev's probability that another
# registered tool already does what Tool does; Other is the one it picked.
Decl tool_duplicate_judgment(Tool, Other, Percent) bound [/string, /string, /number].
# tool_harness_judgment(Tool, Percent): Jev's probability that the harness or
# one shell command already does what Tool does.
Decl tool_harness_judgment(Tool, Percent) bound [/string, /number].

Decl tool_call_landed(Tool, Call) bound [/string, /string].
Decl tool_call_missed(Tool, Call) bound [/string, /string].
# tool_jev_tally(Tool, Landed, Missed): how many of the tool's judged calls
# landed and how many missed, over every session the ledger covers (replayed
# from the autopoiesis ProfileStore by autopoiesis.ToolLedgerFacts; an
# unsettled call is in neither count). Go classifies each call once, as it is
# judged, by reading tool_call_landed / tool_call_missed back, and the ledger
# keeps the tallies; the per-call facts do not outlive that read.
Decl tool_jev_tally(Tool, Landed, Missed) bound [/string, /number, /number].
# tool_jev_verdict(Tool, Kind, Percent): a tool-level verdict Jev reached in an
# earlier process, Kind /duplicate or /harness_covers, with the probability it
# rested on. The thresholds in effect when it was read decided it; it is
# replayed so a restart keeps it.
Decl tool_jev_verdict(Tool, Kind, Percent) bound [/string, /name, /number].
# tool_value_evidence(Tool, Kind): Kind is /hits_mark (more calls landed than
# missed), /misses_mark (more missed than landed), /duplicate or
# /harness_covers.
Decl tool_value_evidence(Tool, Kind) bound [/string, /name].

tool_call_landed(Tool, Call) :-
    tool_call_hit(Tool, Call, Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.
tool_call_missed(Tool, Call) :-
    tool_call_hit(Tool, Call, Percent),
    config_param(/jev_no_percent, No),
    Percent <= No.

# Hits weigh against misses over the tool's whole judged history: the side with
# more calls names the evidence, a tie names neither. (An unsettled call counts
# for neither side.) How strong the hit side must be to earn a place is
# tool_value.mg's, from the tool_value section.
tool_value_evidence(Tool, /hits_mark) :-
    tool_jev_tally(Tool, Landed, Missed),
    Landed > Missed.
tool_value_evidence(Tool, /misses_mark) :-
    tool_jev_tally(Tool, Landed, Missed),
    Missed > Landed.
tool_value_evidence(Tool, /duplicate) :- tool_jev_verdict(Tool, /duplicate, Percent).
tool_value_evidence(Tool, /harness_covers) :- tool_jev_verdict(Tool, /harness_covers, Percent).
tool_value_evidence(Tool, /duplicate) :-
    tool_duplicate_judgment(Tool, Other, Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.
tool_value_evidence(Tool, /harness_covers) :-
    tool_harness_judgment(Tool, Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.
