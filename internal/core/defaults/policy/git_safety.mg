# Git-Aware Safety / Chesterton's Fence
# Section 13 of Cortex Executive Policy

# Declarations live in schemas_safety.mg, schemas_intent.mg, and schemas_analysis.mg.

chesterton_fence_warning(File, "high_churn_file") :-
    user_intent(/current_intent, /mutation, /refactor, File, _),
    churn_rate(File, Freq),
    Freq > 5.

# Trigger clarification for Chesterton's Fence
clarification_needed(File) :-
    chesterton_fence_warning(File, _).
