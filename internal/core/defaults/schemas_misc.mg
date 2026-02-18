# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: MISC
# Sections: 43, 49, 50

# =============================================================================
# SECTION 43: NORTHSTAR VISION & SPECIFICATION
# =============================================================================
# The Northstar defines the project's grand vision, target users, capabilities,
# risks, requirements, and constraints. Used by /northstar command.

# -----------------------------------------------------------------------------
# 43.1 Core Vision
# -----------------------------------------------------------------------------

# northstar_mission(ID, Statement) - The one-sentence mission
Decl northstar_mission(ID, Statement) bound [/string, /string].

# northstar_problem(ID, Description) - Problem being solved
Decl northstar_problem(ID, Description) bound [/string, /string].

# northstar_vision(ID, Description) - Grand vision of success
Decl northstar_vision(ID, Description) bound [/string, /string].

# -----------------------------------------------------------------------------
# 43.2 Target Users (Personas)
# -----------------------------------------------------------------------------

# northstar_persona(PersonaID, Name) - Target user archetype
Decl northstar_persona(PersonaID, Name) bound [/string, /string].

# northstar_pain_point(PersonaID, PainPoint) - User pain points
Decl northstar_pain_point(PersonaID, PainPoint) bound [/string, /string].

# northstar_need(PersonaID, Need) - User needs
Decl northstar_need(PersonaID, Need) bound [/string, /string].

# -----------------------------------------------------------------------------
# 43.3 Capabilities Roadmap
# -----------------------------------------------------------------------------

# northstar_capability(CapID, Description, Timeline, Priority)
# Timeline: /now, /6mo, /1yr, /3yr, /moonshot
# Priority: /critical, /high, /medium, /low
Decl northstar_capability(CapID, Description, Timeline, Priority) bound [/string, /string, /name, /number].

# northstar_serves(CapID, PersonaID) - Capability serves persona
Decl northstar_serves(CapID, PersonaID) bound [/string, /string].

# -----------------------------------------------------------------------------
# 43.4 Risks & Mitigations (Red Teaming)
# -----------------------------------------------------------------------------

# northstar_risk(RiskID, Description, Likelihood, Impact)
# Likelihood/Impact: /high, /medium, /low
Decl northstar_risk(RiskID, Description, Likelihood, Impact) bound [/string, /string, /name, /number].

# northstar_mitigation(RiskID, Strategy) - Risk mitigation strategy
Decl northstar_mitigation(RiskID, Strategy) bound [/string, /name].

# -----------------------------------------------------------------------------
# 43.5 Requirements
# -----------------------------------------------------------------------------

# northstar_requirement(ReqID, Type, Description, Priority)
# Type: /functional, /non_functional, /constraint
# Priority: /must_have, /should_have, /nice_to_have
Decl northstar_requirement(ReqID, Type, Description, Priority) bound [/string, /name, /string, /number].

# northstar_supports(ReqID, CapID) - Requirement supports capability
Decl northstar_supports(ReqID, CapID) bound [/string, /string].

# northstar_addresses(ReqID, RiskID) - Requirement addresses risk
Decl northstar_addresses(ReqID, RiskID) bound [/string, /string].

# -----------------------------------------------------------------------------
# 43.6 Constraints
# -----------------------------------------------------------------------------

# northstar_constraint(ConstraintID, Description) - Hard project constraints
Decl northstar_constraint(ConstraintID, Description) bound [/string, /string].

# -----------------------------------------------------------------------------
# 43.7 Derived Predicates
# -----------------------------------------------------------------------------

# northstar_defined() - True if northstar has been set
Decl northstar_defined().

# critical_capability(CapID) - Derived: capability is critical priority
Decl critical_capability(CapID) bound [/string].

# high_risk(RiskID) - Derived: risk has high likelihood AND impact
Decl high_risk(RiskID) bound [/string].

# has_mitigation(RiskID) - Helper: risk has at least one mitigation
Decl has_mitigation(RiskID) bound [/string].

# unmitigated_risk(RiskID) - Derived: high risk without mitigation
Decl unmitigated_risk(RiskID) bound [/string].

# capability_addresses_need(CapID, PersonaID, Need) - Capability serves persona need
Decl capability_addresses_need(CapID, PersonaID, Need) bound [/string, /string, /string].

# is_served_persona(PersonaID) - Helper: persona is served by at least one capability
Decl is_served_persona(PersonaID) bound [/string].

# capability_is_linked(CapID) - Helper: capability serves at least one persona
Decl capability_is_linked(CapID) bound [/string].

# unserved_persona(PersonaID, Name) - Persona with needs but no capabilities
Decl unserved_persona(PersonaID, Name) bound [/string, /string].

# orphan_capability(CapID, Desc) - Capability not linked to any persona
Decl orphan_capability(CapID, Desc) bound [/string, /string].

# must_have_requirement(ReqID, Desc) - Requirements with must_have priority
Decl must_have_requirement(ReqID, Desc) bound [/string, /string].

# is_supported_req(ReqID) - Helper: requirement is supported by at least one capability
Decl is_supported_req(ReqID) bound [/string].

# orphan_requirement(ReqID, Desc) - Requirement not linked to any capability
Decl orphan_requirement(ReqID, Desc) bound [/string, /string].

# risk_addressing_requirement(ReqID, RiskID) - Requirement that addresses high risk
Decl risk_addressing_requirement(ReqID, RiskID) bound [/string, /string].

# risk_is_addressed(RiskID) - Helper: risk is addressed by at least one requirement
Decl risk_is_addressed(RiskID) bound [/string].

# unaddressed_high_risk(RiskID, Desc) - High risk with no requirement addressing it
Decl unaddressed_high_risk(RiskID, Desc) bound [/string, /string].

# immediate_capability(CapID, Desc) - Capabilities with /now timeline
Decl immediate_capability(CapID, Desc) bound [/string, /string].

# near_term_capability(CapID, Desc) - Capabilities with /6mo timeline
Decl near_term_capability(CapID, Desc) bound [/string, /string].

# long_term_capability(CapID, Desc) - Capabilities with /1yr or /3yr timeline
Decl long_term_capability(CapID, Desc) bound [/string, /string].

# moonshot_capability(CapID, Desc) - Capabilities with /moonshot timeline
Decl moonshot_capability(CapID, Desc) bound [/string, /string].

# strategic_warning(Type, CapID, RiskID) - Strategic gaps and warnings
Decl strategic_warning(Type, CapID, RiskID) bound [/name, /string, /string].

# =============================================================================
# SECTION 48: OBSERVATIONS
# =============================================================================
# observation(Key, Value) - free-form observations to capture learnings.
Decl observation(Key, Value) bound [/string, /string].

# =============================================================================
# SECTION 49: CONTINUATION PROTOCOL (Multi-Step Task Execution)
# =============================================================================
# Enables natural multi-step task chaining in the TUI. The kernel signals
# "there's more work to do" after each shard execution, and the TUI can
# auto-continue based on user-selected mode (Auto/Confirm/Breakpoint).

# -----------------------------------------------------------------------------
# 49.1 Shard Result Tracking
# -----------------------------------------------------------------------------

# shard_result(TaskID, Status, ShardType, TaskDescription, ResultSummary)
# TaskID: Unique identifier for this execution
# Status: /complete, /incomplete, /code_generated, /tests_needed, /review_needed
# ShardType: /coder, /reviewer, /tester, /researcher
# TaskDescription: What was requested
# ResultSummary: Brief summary of output
Decl shard_result(TaskID, Status, ShardType, TaskDescription, ResultSummary) bound [/string, /name, /name, /string, /string].

# pending_test(TaskID, Description) - Test needs to be written for generated code
Decl pending_test(TaskID, Description) bound [/string, /string].

# pending_review(TaskID, Description) - Review needed for changes
Decl pending_review(TaskID, Description) bound [/string, /string].

# -----------------------------------------------------------------------------
# 49.2 Continuation Signals (Derived in policy.mg)
# -----------------------------------------------------------------------------

# has_pending_subtask(TaskID, Description, ShardType) - Derived: there's more work
# Populated by rules that detect incomplete workflows
Decl has_pending_subtask(TaskID, Description, ShardType) bound [/string, /string, /name].

# should_auto_continue/0 - Derived: continuation should proceed automatically
# True when has_pending_subtask exists and no blocking conditions
Decl should_auto_continue().

# continuation_blocked(Reason) - Derived: continuation is blocked
# Reason: /needs_clarification, /user_interrupted, /max_steps_reached
Decl continuation_blocked(Reason) bound [/string].

# has_continuation_block/0 - Helper: true if any continuation block exists
Decl has_continuation_block().

# -----------------------------------------------------------------------------
# 49.3 User Control
# -----------------------------------------------------------------------------

# interrupt_requested - User pressed Ctrl+X to stop execution
Decl interrupt_requested().

# continuation_step(StepNumber, TotalSteps) - Current progress
Decl continuation_step(StepNumber, TotalSteps) bound [/number, /number].

# max_continuation_steps(Limit) - Safety limit (default 10)
Decl max_continuation_steps(Limit) bound [/number].

# =============================================================================
# SECTION 50: BENCHMARK SCHEMAS (REFERENCE)
# =============================================================================
# Benchmark-specific predicates (SWE-bench, HumanEval, MBPP) are in:
#   internal/core/defaults/benchmarks.mg
#
# This keeps the core schemas focused on general-purpose code assistance.
# Load benchmarks.mg only when running benchmark evaluations.

