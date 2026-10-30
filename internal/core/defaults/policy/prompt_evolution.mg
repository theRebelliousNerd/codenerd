config_param_required(/evolution, /evolution_cohort_size).
config_param_required(/evolution, /evolution_screen_calls).

evolution_cohort_member(Group, Task, Version) :-
    evolution_evidence_rank(Group, Task, Version, Rank),
    config_param(/evolution_cohort_size, Limit),
    Rank < Limit.

evolution_screen_allowed(Cohort) :-
    evolution_screen_used(Cohort, Calls),
    config_param(/evolution_screen_calls, Limit),
    Calls < Limit.

# Unavailability retains the existing count fallback; a budget deferral is
# never asserted as an unavailable observation.
evolution_generation_allowed(Cohort) :-
    evolution_screen_observation(Cohort, /unavailable).

evolution_generation_allowed(Cohort) :-
    evolution_screen_observation(Cohort, /answered),
    evolution_screen_probability(Cohort, Probability),
    evolution_screen_threshold(Cohort, Yes),
    :float:ge(Probability, Yes).
