# internal/mangle/doc_taxonomy.mg
# =========================================================
# DOCUMENT LAYER TAXONOMY
# =========================================================

# 1. Architectural Layers & Priorities (Lower runs first)
# Config, Env, Setup
layer_priority(/scaffold, 10).
# Types, Interfaces, Entities
layer_priority(/domain_core, 20).
# Schemas, Repositories, Migrations
layer_priority(/data_layer, 30).
# Business Logic, Use Cases
layer_priority(/service, 40).
# HTTP, gRPC, CLI, API
layer_priority(/transport, 50).
# Wiring, Main, E2E
layer_priority(/integration, 60).

# 2. Logic for Layer Distance (Used for conflict detection)
# NOTE: Uses transform pipeline for arithmetic per Mangle spec
layer_distance(L1, L2, Dist) :-
    layer_priority(L1, P1),
    layer_priority(L2, P2),
    P1 >= P2
    |> let Dist = fn:minus(P1, P2).

layer_distance(L1, L2, Dist) :-
    layer_priority(L1, P1),
    layer_priority(L2, P2),
    P2 > P1
    |> let Dist = fn:minus(P2, P1).

# 3. Validation: Detect "God Documents"
# If a doc maps to layers that are too far apart (e.g., Scaffold AND Integration),
# it suggests the doc is too broad and might confuse the planner.
doc_conflict(Doc, L1, L2) :-
    doc_layer(Doc, L1, _),
    doc_layer(Doc, L2, _),
    L1 != L2,
    layer_distance(L1, L2, Dist),
    Dist > 30.
