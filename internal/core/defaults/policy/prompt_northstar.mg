# Northstar Vision Reasoning
# Section 42 of Cortex Executive Policy

# --- Critical Path Derivation ---

# Derive critical capabilities (priority = /critical)
critical_capability(CapID) :-
    northstar_capability(CapID, _, _, /critical).

# Derive high-risk items (both likelihood AND impact are high)
high_risk(RiskID) :-
    northstar_risk(RiskID, _, /high, /high).

# Helper: risk has at least one mitigation
has_mitigation(RiskID) :-
    northstar_mitigation(RiskID, _).

# Derive unmitigated risks (high risk without any mitigation)
unmitigated_risk(RiskID) :-
    high_risk(RiskID),
    !has_mitigation(RiskID).

# --- Alignment Analysis ---

# Capability addresses persona need when serves relationship exists
capability_addresses_need(CapID, PersonaID, Need) :-
    northstar_serves(CapID, PersonaID),
    northstar_need(PersonaID, Need).

# Helper: persona is served by at least one capability
is_served_persona(PersonaID) :-
    northstar_serves(_, PersonaID).

# Helper: capability serves at least one persona
capability_is_linked(CapID) :-
    northstar_serves(CapID, _).

# Unserved persona - has needs but no capability serves them
unserved_persona(PersonaID, Name) :-
    northstar_persona(PersonaID, Name),
    northstar_need(PersonaID, _),
    !is_served_persona(PersonaID).

# Orphan capability - not linked to any persona
orphan_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, _, _),
    !capability_is_linked(CapID).

# --- Requirements Traceability ---

# Must-have requirements (priority = /must_have)
must_have_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, /must_have).

# Helper: requirement is supported by at least one capability
is_supported_req(ReqID) :-
    northstar_supports(ReqID, _).

# Orphan requirement - not linked to any capability
orphan_requirement(ReqID, Desc) :-
    northstar_requirement(ReqID, _, Desc, _),
    !is_supported_req(ReqID).

# Risk-addressing requirement
risk_addressing_requirement(ReqID, RiskID) :-
    northstar_addresses(ReqID, RiskID),
    high_risk(RiskID).

# Helper: risk is addressed by at least one requirement
risk_is_addressed(RiskID) :-
    northstar_addresses(_, RiskID).

# Unaddressed high risk - no requirement addresses it
unaddressed_high_risk(RiskID, Desc) :-
    high_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _),
    !risk_is_addressed(RiskID).

# --- Timeline Planning ---

# Immediate work (timeline = /now)
immediate_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /now, _).

# Near-term work (timeline = /6mo)
near_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /6mo, _).

# Long-term work (timeline = /1yr or /3yr)
long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /1yr, _).

long_term_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /3yr, _).

# Moonshot capabilities (timeline = /moonshot)
moonshot_capability(CapID, Desc) :-
    northstar_capability(CapID, Desc, /moonshot, _).

# --- Strategic Warnings ---

# Warning: critical capability with unmitigated high risk
strategic_warning(/critical_unmitigated_risk, CapID, RiskID) :-
    critical_capability(CapID),
    northstar_supports(ReqID, CapID),
    northstar_addresses(ReqID, RiskID),
    unmitigated_risk(RiskID).

# Warning: immediate work depends on unaddressed risk
strategic_warning(/immediate_risk_gap, CapID, RiskID) :-
    immediate_capability(CapID, _),
    unaddressed_high_risk(RiskID, _).

# --- Context Injection for Northstar ---

# Inject mission when planning or deciding actions
injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

injectable_context(/northstar_mission, Mission) :-
    northstar_defined(),
    northstar_mission(_, Mission),
    active_shard(ShardID, _),
    shard_family(ShardID, /coder).

# Inject critical capabilities during planning
injectable_context(/critical_cap, Desc) :-
    northstar_defined(),
    critical_capability(CapID),
    northstar_capability(CapID, Desc, _, _),
    active_shard(ShardID, _),
    shard_family(ShardID, /planner).

# Inject unmitigated risks as warnings
injectable_context(/unmitigated_risk_warning, Desc) :-
    northstar_defined(),
    unmitigated_risk(RiskID),
    northstar_risk(RiskID, Desc, _, _).

# Inject constraints always
injectable_context(/constraint, Desc) :-
    northstar_defined(),
    northstar_constraint(_, Desc).
