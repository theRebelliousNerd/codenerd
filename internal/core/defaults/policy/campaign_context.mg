# Campaign Context Logic
# Context paging and activation

# Context Paging (Phase-Aware Spreading Activation)

# Boost activation for current phase context
activation(Fact, 150) :-
    current_phase(PhaseID),
    phase_context_atom(PhaseID, Fact, _).

# Boost files matching current task's target
activation(Target, 140) :-
    next_campaign_task(TaskID),
    campaign_task(TaskID, _, _, _, _),
    task_artifact(TaskID, _, Target, _).

# Suppress context from completed phases
activation(Fact, -50) :-
    context_compression(PhaseID, _, _, _),
    phase_context_atom(PhaseID, Fact, _).
