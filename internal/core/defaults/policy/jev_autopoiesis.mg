# Autopoiesis checks: whether a chat request should be met with a campaign
# suggestion or a persistent-agent suggestion (cmd/nerd/chat/process.go,
# section 1.8; autopoiesis.Orchestrator.SuggestForRequest). Jev reads the
# request (internal/learnjudge/autopoiesis.go); these rules decide. A request
# Jev did not judge (Jev off, the autopoiesis_checks switch off, or its call
# failed) falls back to the keyword heuristic in autopoiesis/complexity.go and
# persistence.go, asserted as autopoiesis_heuristic(Kind). A Jev judgment,
# whatever it says, outranks the heuristic.

# autopoiesis_judgment(Name, Percent): Jev's probability, as a whole percent,
# that the named request judgment is true.
Decl autopoiesis_judgment(Name, Percent) bound [/string, /number].
# autopoiesis_heuristic(Kind): the keyword heuristic matched the request; Kind
# is /campaign or /persistent.
Decl autopoiesis_heuristic(Kind) bound [/name].

Decl autopoiesis_judged() bound [].
Decl autopoiesis_yes(Name) bound [/string].
# autopoiesis_campaign_suggested(Source): Source is /jev or /heuristic.
Decl autopoiesis_campaign_suggested(Source) bound [/name].
# persistent_agent_suggested(Kind, Source): Kind is /recurring (work that
# continues after the request) or /heuristic (the keyword heuristic's own
# need); Source is /jev or /heuristic.
Decl persistent_agent_suggested(Kind, Source) bound [/name, /name].

autopoiesis_judged() :- autopoiesis_judgment(Name, Percent).

autopoiesis_yes(Name) :-
    autopoiesis_judgment(Name, Percent),
    config_param(/jev_yes_percent, Yes),
    Percent >= Yes.

# Truth table for a campaign, from Jev's reading:
#   several deliverables  ordered stages  many files   campaign
#   yes                   yes             any          yes
#   yes                   any             yes          yes
#   any                   yes             yes          yes
#   yes                   not yes         not yes      no
#   not yes               not yes         any          no
#   not yes               any             not yes      no
# "not yes" is no or unsettled. A campaign is work in phases: several results
# that are staged or spread wide, or one result built in stages across many
# files ("implement a complete feature for billing"). One change spread wide
# but done in one pass (a rename across the codebase) is one task, and a
# request Jev could not settle is not a campaign on Jev's word.
autopoiesis_campaign_suggested(/jev) :-
    autopoiesis_yes("several_deliverables"),
    autopoiesis_yes("ordered_stages").
autopoiesis_campaign_suggested(/jev) :-
    autopoiesis_yes("several_deliverables"),
    autopoiesis_yes("many_files").
autopoiesis_campaign_suggested(/jev) :-
    autopoiesis_yes("ordered_stages"),
    autopoiesis_yes("many_files").
autopoiesis_campaign_suggested(/heuristic) :-
    autopoiesis_heuristic(/campaign),
    !autopoiesis_judged().

# A persistent agent: the work continues after the request is answered.
persistent_agent_suggested(/recurring, /jev) :-
    autopoiesis_yes("continues_after_request").
persistent_agent_suggested(/heuristic, /heuristic) :-
    autopoiesis_heuristic(/persistent),
    !autopoiesis_judged().
