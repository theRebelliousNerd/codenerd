# Git-Aware Safety / Chesterton's Fence
# Section 13 of Cortex Executive Policy

# Declarations live in schemas_safety.mg, schemas_intent.mg, and schemas_analysis.mg.

# Recent change by another author (within 2 days)
recent_change_by_other(File) :-
    git_history(File, _, Author, Age, _),
    current_user(CurrentUser),
    Author != CurrentUser,
    Age < 2.

# Chesterton's Fence warning - warn before deleting recently-changed code
chesterton_fence_warning(File, "recent_change_by_other") :-
    user_intent(/current_intent, /mutation, /delete, File, _),
    recent_change_by_other(File).

chesterton_fence_warning(File, "high_churn_file") :-
    user_intent(/current_intent, /mutation, /refactor, File, _),
    churn_rate(File, Freq),
    Freq > 5.

# Trigger clarification for Chesterton's Fence
clarification_needed(File) :-
    chesterton_fence_warning(File, _).
