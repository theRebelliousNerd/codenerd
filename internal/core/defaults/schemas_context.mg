# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: CONTEXT COMPILATION
# Section CC: Context Compilation Pipeline

# NERD-EVOLVE-START: context_compilation_schemas_c1_c4
# Hypothesis: C1+C4 (Wire kernel-derived context selection + dependency-graph relevance)

# =============================================================================
# CC.1: Context Relevance & Selection (C1)
# =============================================================================

# context_relevant(Fact, Priority) - Fact relevance with atom-encoded priority level
# Fact: string identifier (file path, predicate name, or fact ID)
# Priority: /p100, /p95, /p90, /p85, /p80, /p70, /p60
# Derived by context_compilation.mg rules from user_intent, focus_resolution,
# modified, dependency_link, pytest_failure, context_atom
Decl context_relevant(Fact, Priority) bound [/string, /name].

# should_include_context(Fact, Priority) - Final inclusion gate with priority
# Carries priority for budget-limited selection in BuildContext().
# The Go layer queries this predicate, sorts by parsed priority, and
# selects the top N within the AtomReserve token budget.
Decl should_include_context(Fact, Priority) bound [/string, /name].

# =============================================================================
# CC.2: Retention
# =============================================================================

# context_must_retain(Predicate) - every fact of Predicate is carried in the
# window's constitutional section on every build, whatever relevance derived.
# Predicate: the predicate's name as a name atom (/permitted).
# Consumed by Compressor.getCoreFacts (internal/context/compressor.go), which
# retains a constitutional floor of its own only when this derives no row, and
# warns when it does.
Decl context_must_retain(Predicate) bound [/name].

# =============================================================================
# CC.3: Entity views -- what the window shows of a selected entity
# =============================================================================

# context_view(Predicate, Arity, Position, Rank, Scope) - the facts of Predicate
# (of that arity) whose argument at Position is an entity the turn selected
# (should_include_context) describe that entity and enter the active context.
# Rank: views fill the atom budget rank by rank across the selected entities,
#   lowest first, so a bulky view of one entity cannot crowd out the compact
#   views of the others.
# Scope: /focal - shown only of the entities the turn is working on
#   (context_focal); /any - shown of every selected entity.
# Consumed by Compressor.viewFacts (internal/context/entity_views.go), which
# reads the rows from the session's whole kernel through the argument index.
Decl context_view(Predicate, Arity, Position, Rank, Scope) bound [/name, /number, /number, /number, /name].

# context_focal(Entity) - an entity the turn is working on, as distinct from
# the neighbourhood selected around it.
Decl context_focal(Entity) bound [/string].

# context_neighbour_limit(N) - the most files a unit may have, or the most
# importers, for its files to be selected one by one as a focal file's
# neighbourhood. Past it the unit is selected as one entity (context_wide_unit).
Decl context_neighbour_limit(N) bound [/number].
Decl context_unit_size(Unit, N) bound [/string, /number].
Decl context_small_unit(Unit) bound [/string].
Decl context_few_importers(Unit) bound [/string].
Decl context_wide_unit(Unit) bound [/string].

# session_note(Key, Value) - a short-term observation the model recorded for
# this session with the piggyback memory operation "note"
# (protocol/piggyback/memory_ops), e.g. session_note("current_focus",
# "refactoring auth module"). One note per key: a later note replaces it, and
# the memory operation "forget" drops it. Asserted by
# Compressor.recordSessionNote (internal/context/compressor_turns.go).
Decl session_note(Key, Value) bound [/string, /string].

# =============================================================================
# CC.4: Dependency Reachability (C4)
# =============================================================================

# context_reachable(File, HopLevel) - File reachable from focal set via dependency graph
# File: file path string
# HopLevel: /hop0 (focal), /hop1 (direct deps/importers), /hop2 (2-hop reach)
# Derived by bounded hop traversal in context_compilation.mg using dependency_link.
# Falls back to empty set when dependency_link facts are sparse (pre-codeDOM indexing).
Decl context_reachable(File, HopLevel) bound [/string, /name].

# context_file_priority(File, Priority) - File priority by hop distance or test-failure state
# File: file path string
# Priority: /p100, /p95, /p90, /p85, /p80, /p60
# Derived from context_reachable hop level and test failure predicates.
Decl context_file_priority(File, Priority) bound [/string, /name].

# NERD-EVOLVE-END: context_compilation_schemas_c1_c4

# NERD-EVOLVE-START: context_compilation_schemas_c3
# Hypothesis: C3 (Observation Masking — replace LLM summarization)

# =============================================================================
# CC.3: Observation Masking
# =============================================================================

# turn_age_category(TurnID, Category) - Categorizes turns by age for masking decisions
# TurnID: string turn identifier (e.g., "turn_1")
# Category: /recent, /mid, /old, /ancient
# Asserted by Go from the compressor's turn tracking.
Decl turn_age_category(TurnID, Category) bound [/string, /name].

# should_mask_observation(TurnID) - Marks turns whose observation content should be masked
# Masked turns retain intent/focus/action atoms but drop verbose tool output/surface text.
# Derived from turn_age_category: old and ancient turns are masked.
Decl should_mask_observation(TurnID) bound [/string].

# should_preserve_reasoning(TurnID) - Marks turns whose reasoning chain must be preserved
# All turns with any age category are preserved (reasoning is always kept).
# This is the invariant: we mask observations, never reasoning.
Decl should_preserve_reasoning(TurnID) bound [/string].

# NERD-EVOLVE-END: context_compilation_schemas_c3
