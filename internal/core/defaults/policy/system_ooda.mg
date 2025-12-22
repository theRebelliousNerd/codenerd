# System Shard Coordination - OODA Logic
# Domain: OODA Loop (Observe, Orient, Decide, Act)

Decl ooda_phase(Phase.Type<atom>).
Decl pending_intent(IntentID.Type<atom>).
Decl intent_ready_for_executive(IntentID.Type<atom>).
Decl has_next_action().
Decl next_action(Action.Type<atom>).
Decl action_pending_permission(ActionID.Type<string>).
Decl action_ready_for_routing(ActionID.Type<string>).
Decl current_ooda_phase(Phase.Type<atom>).
Decl ooda_stalled(Reason.Type<string>).
Decl ooda_timeout().
Decl escalation_needed(Domain.Type<atom>, Entity.Type<atom>, Reason.Type<string>).

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
