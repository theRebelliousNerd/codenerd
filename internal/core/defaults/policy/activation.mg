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

# 4. File modification spreads to dependents, when they are few enough to be a
# selection (context_few_importers, context_compilation.mg). A modified file of
# a hub package activated all 672 of its importers; that set is impacted(), not
# what the window should carry.
activation(Dep, 70) :-
    modified(File),
    file_unit(File, Unit),
    context_few_importers(Unit),
    import_unit(Dep, Unit).

# 5. Context Pruning - Only high-activation facts enter working memory
context_atom(fn:string:concat(Fact)) :-
    activation(Fact, Score),
    Score > 30.
