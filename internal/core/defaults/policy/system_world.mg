# World Model Updates (World Model Ingestor)
# Extracted from system.mg

# Decl imports
# Moved to schemas_shards.mg
# Decl world_model_stale(File).
# Decl modified(File).
# Decl file_topology(File, PackageName, Imports, Types, Deps).
# Decl next_action(Action).
# Decl system_shard_healthy(ShardName).
# Decl file_in_project(File).
# Decl symbol_reachable(From, To).
# Decl dependency_link(From, To, Type).
# Decl symbol_reachable_bounded(From, To, MaxDepth).
# Decl symbol_reachable_safe(From, To).

# File change triggers world model update
world_model_stale(File) :-
    modified(File),
    file_topology(File, _, _, _, _).

# Trigger ingestor when world model is stale
next_action(/update_world_model) :-
    world_model_stale(_),
    system_shard_healthy(/world_model_ingestor).

# File topology derived from filesystem
file_in_project(File) :-
    file_topology(File, _, _, _, _).

# Symbol graph connectivity, DEMAND-DRIVEN.
#
# These were eager closures over dependency_link, and they took the whole kernel
# down on any repository of realistic size. Resolved import edges are dense —
# package-level fan-out gives codeNERD's own tree ~33k file->file edges — and an
# unbounded transitive closure plus a depth-15 path enumeration over that
# exhausted the 500,000 derived-fact ceiling before evaluation finished. The
# failure is not local: once the ceiling trips, the ENTIRE program fails, so
# every unrelated query returns zero rows. Measured on this repo, safe_action
# went from 120 to 0 with `lazy evaluation failed: fact size limit reached
# "path_of_length(From,To,Len)" 500020 > 500000`.
#
# Nothing read any of them. symbol_reachable, symbol_reachable_safe and
# path_of_length have no Go consumer and appear in no rule body — the cost was
# paid in full and the answer thrown away, which is this codebase's most common
# defect wearing its most expensive hat.
#
# The capability is worth keeping, so it is seeded instead of removed. A caller
# asserts reachability_query(File) for the root it actually cares about and the
# closure expands from there only; with no seed asserted these derive nothing
# and cost nothing. Recursion is on the accumulated result rather than the raw
# edge relation, so the seed genuinely bounds the search instead of being a
# filter applied after the fact.
#
# The dependency_links.go cap (maxResolvedDependencyLinks = 50000) does not help
# here and its comment says why it was chosen: it guards the EDB size. What
# broke is the DERIVED closure, and that broke at ~33k edges — comfortably under
# the cap, so the truncation warning never fired and the first symptom was a
# dead kernel.

Decl reachability_query(From) bound [/string].

symbol_reachable(From, To) :-
    reachability_query(From),
    dependency_link(From, To, _).

symbol_reachable(From, To) :-
    symbol_reachable(From, Mid),
    dependency_link(Mid, To, _).

# Bounded variant, same seeding. The depth cap alone was never the protection —
# depth 15 over a dense graph is exactly what exhausted the budget.

Decl path_of_length(From, To, Len).

path_of_length(From, To, 1) :-
    reachability_query(From),
    dependency_link(From, To, _).

path_of_length(From, To, Len) :-
    path_of_length(From, Mid, SubLen),
    dependency_link(Mid, To, _),
    Len = fn:plus(SubLen, 1),
    Len <= 15.

symbol_reachable_safe(From, To) :-
    path_of_length(From, To, _).

# symbol_reachable_bounded deprecated/removed to ensure safety.
# If something calls it, it will fail at compile time, which is better than runtime safety error.
# If I wanted to keep it, I'd need a generator for MaxDepth or to bind it.
# symbol_reachable_bounded(From, To, MaxDepth) :-
#    symbol_reachable_safe(From, To),
#    MaxDepth = 15.
