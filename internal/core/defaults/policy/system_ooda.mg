# OODA Loop Coordination
# Extracted from system.mg

# Decl imports
# Moved to schemas_shards.mg
# Decl ooda_phase(Phase).
# Decl pending_intent(IntentID).
# Decl intent_ready_for_executive(IntentID).
# Decl has_next_action().
# Decl action_pending_permission(ActionID).
# Decl action_ready_for_routing(ActionID).
# Decl next_action(Action).
# Decl current_ooda_phase(Phase).
# Decl ooda_stalled(Reason).
# Decl escalation_needed(Target, Subject, Reason).

# OODA phases: Observe → Orient → Decide → Act
ooda_phase(/observe) :-
    pending_intent(IntentID),
    !intent_ready_for_executive(IntentID).

ooda_phase(/orient) :-
    intent_ready_for_executive(IntentID),
    pending_intent(IntentID),
    !has_next_action().

ooda_phase(/decide) :-
    pending_intent(IntentID),
    intent_ready_for_executive(IntentID),
    has_next_action().

ooda_phase(/act) :-
    action_pending_permission(_).

ooda_phase(/act) :-
    action_ready_for_routing(_).

# Helper for OODA phase detection
has_next_action() :-
    next_action(_).

# Current OODA state for debugging/monitoring.
# Prefer the most advanced phase when multiple are simultaneously true.
current_ooda_phase(/act) :-
    ooda_phase(/act).

current_ooda_phase(/decide) :-
    ooda_phase(/decide),
    !ooda_phase(/act).

current_ooda_phase(/orient) :-
    ooda_phase(/orient),
    !ooda_phase(/act),
    !ooda_phase(/decide).

current_ooda_phase(/observe) :-
    ooda_phase(/observe),
    !ooda_phase(/act),
    !ooda_phase(/decide),
    !ooda_phase(/orient).

# OODA loop stalled detection (30 second threshold)
ooda_stalled("no_action_derived") :-
    pending_intent(_),
    ooda_timeout().

# Escalate stalled OODA loop
escalation_needed(/ooda_loop, "stalled", Reason) :-
    ooda_stalled(Reason).
