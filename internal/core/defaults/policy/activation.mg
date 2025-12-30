# Spreading Activation (Context Selection)
# Section 1 of Cortex Executive Policy


# 1. Base Activation (Recency) - High priority for new facts
activation(Fact, 100) :- new_fact(Fact).

# 2. Spreading Activation (Dependency)
# Energy flows from goals to required tools
activation(Tool, 80) :-
    active_goal(Goal),
    tool_capabilities(Tool, Cap),
    goal_requires(Goal, Cap).

# 3. Intent-driven activation
activation(Target, 90) :-
    user_intent(/current_intent, _, _, Target, _).

# 4. File modification spreads to dependents
activation(Dep, 70) :-
    modified(File),
    dependency_link(Dep, File, _).

# 5. Context Pruning - Only high-activation facts enter working memory
context_atom(Fact) :-
    activation(Fact, Score),
    Score > 30.
