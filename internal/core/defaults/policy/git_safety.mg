# Git-Aware Safety / Chesterton's Fence
# Section 13 of Cortex Executive Policy

# Declarations live in schemas_safety.mg, schemas_intent.mg, and schemas_analysis.mg.

git_recent_change_days(2).

# Recent change by another author. Age is computed here, from the commit's unix
# time against current_time, so the facts stay measurements and the rule replays
# at any pinned clock. The singletons current_user, the window and current_time
# bind before the cross product with git_history.
recent_change_by_other(File) :-
    current_user(CurrentUser),
    git_recent_change_days(Days),
    current_time(Now),
    Window = fn:mult(Days, 86400),
    Cutoff = fn:minus(Now, Window),
    git_history(File, _, Author, CommitTime, _),
    Author != CurrentUser,
    CommitTime > Cutoff.

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
