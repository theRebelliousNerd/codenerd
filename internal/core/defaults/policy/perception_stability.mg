# Perception Stability Rules
# Hypothesis: W3 (Kalman/Stability Filter for perception bypass)
# Predicates declared in: internal/core/defaults/schemas_perception_latency.mg

# NERD-EVOLVE-START: stability_filter

# Bypass authorized when:
#   1. Intent stability score exceeds threshold (>80 out of 100)
#   2. A prior understanding exists (we have something to reuse)
#   3. No topic change detected by the syntactic pre-check
#
# Derivation chain:
#   intent_stability(Score) + intent_prior(_,_,_) + ~topic_change_detected()
#   → llm_call_deferred()
#   → Go layer skips t.llmTransducer.Understand() call (~500ms-5s saved)
llm_call_deferred() :-
    intent_stability(Score),
    Score > 80,
    intent_prior(_, _, _),
    !topic_change_detected().

# NERD-EVOLVE-END: stability_filter
