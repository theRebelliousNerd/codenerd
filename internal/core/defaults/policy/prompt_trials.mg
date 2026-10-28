# Prompt Evolution Trial Admission
# Declarations: schemas_prompts.mg (prompt evolution trials).
#
# A quarantined prompt candidate reaches a compile only through an
# operator-approved envelope that is unexpired and has exposure budget left.
# Every literal fails closed: no clock, no exposure row, or an exhausted or
# expired envelope derives nothing, and the compiler then serves only promoted
# candidates.

prompt_trial_admitted(AtomID, Version) :-
    prompt_trial_envelope(AtomID, Version, ExpiresAt, MaxExposures),
    prompt_trial_exposures(AtomID, Count),
    Count < MaxExposures,
    current_time(Now),
    Now < ExpiresAt.
