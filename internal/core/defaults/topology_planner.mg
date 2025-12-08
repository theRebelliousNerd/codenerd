# internal/mangle/topology_planner.mg
# =========================================================
# TOPOLOGY PLANNER
# =========================================================

# 1. Identify "Active" Layers
# A layer is active if we have high-confidence docs for it.
active_layer(Layer) :-
    doc_layer(_, Layer, Confidence),
    Confidence > 0.65.

# 2. Generate Phase Skeletons
# Every active layer becomes a proposed phase in the campaign.
proposed_phase(Layer) :-
    active_layer(Layer).

# 3. Generate Hard Dependencies
# If Layer A has lower priority number than Layer B, A must finish before B starts.
phase_dependency_generated(PhaseA, PhaseB) :-
    active_layer(LayerA),
    active_layer(LayerB),
    layer_priority(LayerA, ScoreA),
    layer_priority(LayerB, ScoreB),
    ScoreA < ScoreB,
    PhaseA = LayerA,
    PhaseB = LayerB.

# 4. Context Scoping (The "Pollution" Fix)
# Defines exactly which files are allowed in the context window for a phase.
phase_context_scope(Phase, DocPath) :-
    active_layer(Layer),
    doc_layer(DocPath, Layer, _),
    Phase = Layer.

# Also allow "scaffold" docs (config/env) to be visible to ALL phases
phase_context_scope(Phase, DocPath) :-
    active_layer(Phase),
    doc_layer(DocPath, /scaffold, _).
