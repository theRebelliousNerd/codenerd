# Prompt evolution measures ranks and logical purchases; the kernel chooses
# the bounded cohort and admits screening and subsequent generation.
Decl evolution_evidence_rank(Group, Task, Version, Rank) bound [/string, /string, /string, /number].
Decl evolution_cohort_member(Group, Task, Version) bound [/string, /string, /string].
Decl evolution_screen_used(Cohort, Calls) bound [/string, /number].
Decl evolution_screen_allowed(Cohort) bound [/string].
Decl evolution_screen_observation(Cohort, Status) bound [/string, /name].
Decl evolution_screen_probability(Cohort, Probability) bound [/string, /float64].
Decl evolution_screen_threshold(Cohort, Yes) bound [/string, /float64].
Decl evolution_generation_allowed(Cohort) bound [/string].
