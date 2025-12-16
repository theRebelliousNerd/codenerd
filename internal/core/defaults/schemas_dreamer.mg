# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: DREAMER
# Sections: 38, 48

# =============================================================================
# SECTION 38: SPECULATIVE DREAMER (Precognition Layer)
# =============================================================================
# Projected facts produced by the Dreamer to simulate action effects.

# projected_action(ActionID, ActionType, Target)
Decl projected_action(ActionID, ActionType, Target).

# projected_fact(ActionID, FactType, Value)
# FactType: /file_missing, /file_exists, /modified
Decl projected_fact(ActionID, FactType, Value).

# panic_state(ActionID, Reason) - Derived: future state violates invariant
Decl panic_state(ActionID, Reason).

# dream_block(ActionID, Reason) - Derived: action blocked by Dreamer
Decl dream_block(ActionID, Reason).

# critical_file(Path) - Enumerates files whose deletion is catastrophic
Decl critical_file(Path).

# critical_path_prefix(Prefix) - Paths that should never be removed recursively
Decl critical_path_prefix(Prefix).

# =============================================================================
# SECTION 38B: DREAM STATE LEARNING (§8.3.1)
# =============================================================================
# Learnable insights extracted from Dream State multi-agent consultations.
# These facts persist confirmed learnings for tool generation and preference storage.

# dream_state(Hypothetical, Timestamp)
# Records a dream state consultation was performed
Decl dream_state(Hypothetical, Timestamp).

# dream_tool_need(ToolName, Description, Confidence, Hypothetical)
# Tool capability gap identified by Dream State consultation
# Routes to Ouroboros for potential tool generation
Decl dream_tool_need(ToolName, Description, Confidence, Hypothetical).

# dream_risk_pattern(RiskType, Content, Confidence)
# Safety/risk awareness learned from Dream State
# RiskType: /security, /data_integrity, /performance, /stability, /deployment, /general
Decl dream_risk_pattern(RiskType, Content, Confidence).

# dream_preference(Content, Confidence)
# User/project preference learned from Dream State
Decl dream_preference(Content, Confidence).

# dream_learning_confirmed(LearningID, Type, Content, Timestamp)
# Records a confirmed dream learning for audit trail
Decl dream_learning_confirmed(LearningID, Type, Content, Timestamp).

# =============================================================================
# SECTION 48: CROSS-MODULE SUPPORT PREDICATES
# =============================================================================
# Predicates used by policy.mg rules or Go code across multiple modules.

# -----------------------------------------------------------------------------
# 48.1 JIT Prompt Compiler Support (policy.mg Section 41)
# -----------------------------------------------------------------------------

# effective_prompt_atom(AtomID) - Derived: atom is effective (selected and led to success)
# Used for learning signals to improve prompt compilation over time
Decl effective_prompt_atom(AtomID).

# -----------------------------------------------------------------------------
# 48.2 Nemesis / Chaos Engineering Support (nemesis.go, chaos.mg)
# -----------------------------------------------------------------------------

# system_invariant_violated(InvariantID, Timestamp) - System invariant violation detected
# InvariantID: Identifier for the invariant (/http_500_rate, /deadlock_detected, etc.)
# Timestamp: When the violation was detected
# Used by NemesisShard and Thunderdome for chaos engineering
Decl system_invariant_violated(InvariantID, Timestamp).

# patch_diff(PatchID, DiffContent) - Stores patch diffs for analysis
# PatchID: Identifier for the patch
# DiffContent: The actual diff content as a string
# Used by NemesisShard for adversarial patch analysis
Decl patch_diff(PatchID, DiffContent).

# gauntlet_result(PatchID, Phase, Verdict, Timestamp) - Nemesis gauntlet outcome
# Verdict: /passed or /failed
Decl gauntlet_result(PatchID, Phase, Verdict, Timestamp).

# gauntlet_passed() - derived: at least one gauntlet passed in session
Decl gauntlet_passed().

# -----------------------------------------------------------------------------
# 48.3 Verification Support (verification.go)
# -----------------------------------------------------------------------------

# verification_summary(Timestamp, Total, Confirmed, Dismissed, DurationMs)
# Timestamp: When verification completed
# Total: Total number of hypotheses verified
# Confirmed: Number confirmed by LLM
# Dismissed: Number dismissed
# DurationMs: Duration in milliseconds
# Used by ReviewerShard hypothesis verification loop
Decl verification_summary(Timestamp, Total, Confirmed, Dismissed, DurationMs).

