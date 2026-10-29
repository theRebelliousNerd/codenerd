# Configuration Parameters: the bridge from .nerd/config.json to the policy
#
# A threshold the policy decides with is a fact, and the fact comes from the
# user's config: config_param(Key, Value), asserted by the component that owns
# the section (internal/config/params.go, ParamFacts). Keys are
# /<section>_<field> name constants; values are integers, because Mangle
# compares integers only -- a switch is 0 or 1 and a ratio is a percent.
#
# A rule over an absent threshold derives nothing, which for a cap means it
# never binds: task_exhausted over a missing /campaign_max_task_attempts would
# retry forever. So a rule's thresholds are declared required next to the rule,
# config_param_required(Section, Key), and the component that asserts the
# section refuses to start while config_param_missing(Section, Key) derives.

Decl config_param(Key, Value) bound [/name, /number].
Decl config_param_required(Section, Key) bound [/name, /name].
Decl has_config_param(Key) bound [/name].
Decl config_param_missing(Section, Key) bound [/name, /name].

has_config_param(Key) :- config_param(Key, Value).

config_param_missing(Section, Key) :-
    config_param_required(Section, Key),
    !has_config_param(Key).
