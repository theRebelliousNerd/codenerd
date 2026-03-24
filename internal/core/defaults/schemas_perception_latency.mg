# Cortex Schemas — Perception Latency Optimization
# Hypothesis: W3 (Kalman/Stability Filter for perception bypass)

# NERD-EVOLVE-START: stability_filter

# =============================================================================
# W3: Intent Stability Tracking
# These predicates support the stability bypass mechanism in
# internal/perception/understanding_adapter.go.
# All four predicates are retracted and re-asserted each turn by the Go layer.
# =============================================================================

# intent_stability(Score) — stability score in range 0-100
# Score is the fraction of consecutive matching verb pairs × 100.
# Example: verbHistory = [modify,modify,modify] → score 100
# Example: verbHistory = [modify,verify,modify,verify] → score 0
# Asserted by: UnderstandingTransducer.assertStabilityFacts()
Decl intent_stability(Score) bound [/number].

# intent_prior(SemanticType, ActionType, Domain) — prior turn's understanding
# Asserted from lastUnderstanding when a prior understanding exists.
# Used by llm_call_deferred to gate bypass on prior state presence.
# Asserted by: UnderstandingTransducer.assertStabilityFacts()
Decl intent_prior(SemanticType, ActionType, Domain) bound [/name, /name, /name].

# llm_call_deferred() — derived: bypass authorized when stability is high
# and no topic change detected and a prior understanding exists.
# Queried by: UnderstandingTransducer.assertStabilityFacts() via QueryBool()
Decl llm_call_deferred() bound [].

# topic_change_detected() — asserted when syntactic pre-check fires
# Suppresses bypass: any signal (question mark, topic-shift keyword, length spike)
# causes this fact to be asserted, which blocks llm_call_deferred derivation.
# Asserted by: UnderstandingTransducer.assertStabilityFacts()
Decl topic_change_detected() bound [].

# NERD-EVOLVE-END: stability_filter
