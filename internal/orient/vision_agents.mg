# Vision-derived domain experts. After orientation, init asks the planner
# client which domain experts the project needs for what it is trying to
# build (internal/init/domain_experts.go), validates the answer, and asserts
# it here. These rules merge the experts into the same roster the stack
# catalog and imported agents feed: an expert named like a stack expert is
# that expert, with both topic sets.
#
# Roster bounds are orient_param rows read from config_param. Their values and
# defaults live in one place, config.OrientConfig (orient.domain_agent_max and
# its siblings), so `nerd config full` shows what is in force.

Decl orient_param_key(Key) bound [/name].
Decl orient_param(Key, Value) bound [/name, /number].

Decl vision_agent(Name, Why) bound [/string, /string].
Decl vision_agent_description(Name, Description) bound [/string, /string].
Decl vision_agent_topic(Name, Topic) bound [/string, /string].
Decl vision_agent_question(Name, Question) bound [/string, /string].
Decl vision_agent_fast_question(Name, Question) bound [/string, /string].
Decl vision_core_support(Name, Why) bound [/string, /string].
Decl vision_topic(Name, Topic) bound [/string, /string].
Decl domain_agent_priority(Priority) bound [/number].
Decl domain_default_permission(Perm) bound [/string].

# How many vision experts one init keeps, how many topics and research
# questions each may carry, how many imported subagents and modes join the
# roster, and after how many days a fast-moving topic's research is stale.
orient_param_key(/orient_domain_agent_max).
orient_param_key(/orient_domain_agent_topic_max).
orient_param_key(/orient_imported_agent_max).
orient_param_key(/orient_research_stale_days).

config_param_required(/orient, /orient_domain_agent_max).
config_param_required(/orient, /orient_domain_agent_topic_max).
config_param_required(/orient, /orient_imported_agent_max).
config_param_required(/orient, /orient_research_stale_days).

orient_param(Key, Value) :-
    orient_param_key(Key),
    config_param(Key, Value).

domain_agent_priority(95).
domain_default_permission("read_file").
domain_default_permission("code_graph").

vision_topic(Name, Topic) :- vision_agent_topic(Name, Topic).

core_agent_supported(Name) :-
    vision_core_support(Name, Why),
    core_agent(Name, CoreWhy).

orient_agent(Name, Why) :- vision_agent(Name, Why).

orient_agent_topic(Name, Topic) :-
    vision_agent(Name, Why),
    vision_agent_topic(Name, Topic).

orient_agent_description(Name, Description) :-
    vision_agent(Name, Why),
    vision_agent_description(Name, Description).

orient_agent_permission(Name, Perm) :-
    vision_agent(Name, Why),
    domain_default_permission(Perm).

orient_agent_priority(Name, Priority) :-
    vision_agent(Name, Why),
    domain_agent_priority(Priority).
