# Strategy Selection
# Section 2 of Cortex Executive Policy


# TDD Repair Loop for bug fixes
active_strategy(/tdd_repair_loop) :-
    user_intent(/current_intent, _, /fix, _, _),
    diagnostic(/error, _, _, _, _).

active_strategy(/tdd_repair_loop) :-
    user_intent(/current_intent, _, /debug, _, _).

# Exploration for queries
active_strategy(/breadth_first_survey) :-
    user_intent(/current_intent, /query, /explore, _, _).

active_strategy(/breadth_first_survey) :-
    user_intent(/current_intent, /query, /explain, _, _).

# Code generation for scaffolding
active_strategy(/project_init) :-
    user_intent(/current_intent, /mutation, /scaffold, _, _).

active_strategy(/project_init) :-
    user_intent(/current_intent, /mutation, /init, _, _).

# Refactor guard for modifications
active_strategy(/refactor_guard) :-
    user_intent(/current_intent, /mutation, /refactor, _, _).

# Section 17: Knowledge Atom Integration

# When high-confidence knowledge about the domain exists
# Knowledge atoms inform strategy selection (confidence on 0-100 scale)
active_strategy(/domain_expert) :-
    knowledge_atom(_, _, _, Confidence),
    Confidence > 80,
    user_intent(/current_intent, _, _, _, _).

# Section 20: Campaign Start Trigger

# Trigger campaign mode when user wants to start a campaign
active_strategy(/campaign_planning) :-
    user_intent(/current_intent, /mutation, /campaign, _, _).

# Alternative triggers for campaign-like requests
active_strategy(/campaign_planning) :-
    user_intent(/current_intent, /mutation, /build, Target, _),
    target_is_large(Target).

active_strategy(/campaign_planning) :-
    user_intent(/current_intent, /mutation, /implement, Target, _),
    target_is_complex(Target).
