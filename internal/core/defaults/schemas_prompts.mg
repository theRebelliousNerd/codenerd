# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: PROMPTS
# Sections: 42, 45

# =============================================================================
# SECTION 42: DYNAMIC PROMPT COMPOSITION (Context Injection)
# =============================================================================
# These predicates enable kernel-driven system prompt assembly.
# The articulation layer queries these to build dynamic prompts for shards.

# -----------------------------------------------------------------------------
# 42.1 Base Prompt Templates
# -----------------------------------------------------------------------------

# shard_prompt_base(ShardType, BaseTemplate)
# Base template for each shard type (Type A, B, S, U)
# ShardType: /system, /ephemeral, /persistent, /user
Decl shard_prompt_base(ShardType, BaseTemplate) bound [/name, /string].

# -----------------------------------------------------------------------------
# 42.2 Context Atom Selection (Spreading Activation Output)
# -----------------------------------------------------------------------------

# shard_context_atom(ShardID, Atom, Relevance)
# Context atoms selected for injection into prompts (spreading activation output)
# Relevance: 0.0-1.0 score indicating how relevant the atom is
# NOTE: Named shard_context_atom to distinguish from existing context_atom(Fact) in Section 12
Decl shard_context_atom(ShardID, Atom, Relevance) bound [/string, /string, /number].

# shard_context_refreshed(ShardID, Atom, Timestamp)
# Marks a stale context atom as refreshed for a shard, suppressing repeated refresh loops.
Decl shard_context_refreshed(ShardID, Atom, Timestamp) bound [/string, /string, /number].

# -----------------------------------------------------------------------------
# 42.3 Specialist Knowledge (Type B Persistent Shards)
# -----------------------------------------------------------------------------

# specialist_knowledge(ShardID, Topic, Content)
# Specialist knowledge for Type B persistent shards
# Topic: domain identifier (e.g., /go_concurrency, /react_hooks, /sql_optimization)
Decl specialist_knowledge(ShardID, Topic, Content) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 42.4 Session-Level Customizations
# -----------------------------------------------------------------------------

# prompt_customization(SessionID, Key, Value)
# Session-level prompt customizations (user preferences)
# Key: customization key (e.g., /verbosity, /tone, /detail_level)
Decl prompt_customization(SessionID, Key, Value) bound [/string, /string, /string].

# -----------------------------------------------------------------------------
# 42.5 Campaign-Specific Constraints
# -----------------------------------------------------------------------------

# campaign_prompt_policy(CampaignID, ShardType, Constraint)
# Campaign-specific prompt constraints
# Constraint: rule or limitation to apply (e.g., "no external APIs", "strict typing")
Decl campaign_prompt_policy(CampaignID, ShardType, Constraint) bound [/string, /name, /string].

# -----------------------------------------------------------------------------
# 42.6 Learned Exemplars
# -----------------------------------------------------------------------------

# prompt_exemplar(ShardType, Category, Exemplar)
# Learned exemplars that should influence prompts
# Category: exemplar category (e.g., /code_style, /error_handling, /documentation)
# Exemplar: the learned example pattern or template
Decl prompt_exemplar(ShardType, Category, Exemplar) bound [/name, /name, /string].

# -----------------------------------------------------------------------------
# 42.7 Derived Predicates for Prompt Assembly
# -----------------------------------------------------------------------------

# prompt_ready(ShardID) - derived: all required prompt components are available
Decl prompt_ready(ShardID) bound [/string].

# has_specialist_knowledge(ShardID) - helper: shard has specialist knowledge loaded
Decl has_specialist_knowledge(ShardID) bound [/string].

# has_campaign_constraints(CampaignID, ShardType) - helper: campaign has constraints for shard type
Decl has_campaign_constraints(CampaignID, ShardType) bound [/string, /name].

# active_prompt_customization(Key, Value) - derived: active customization for current session
Decl active_prompt_customization(Key, Value) bound [/string, /string].

# prompt_context_budget(ShardID, TokensUsed, TokensAvailable) - context window tracking
Decl prompt_context_budget(ShardID, TokensUsed, TokensAvailable) bound [/string, /number, /number].

# context_overflow(ShardID) - derived: context exceeds available budget
Decl context_overflow(ShardID) bound [/string].

# -----------------------------------------------------------------------------
# 42.8 Active Shard Tracking
# -----------------------------------------------------------------------------

# active_shard(ShardID, ShardType) - currently active shard being configured
Decl active_shard(ShardID, ShardType) bound [/string, /name].

# has_active_shard(ShardType) - helper for safe negation (0-arity for type)
# Use this instead of "!active_shard(/coder, _)" which has unbound variable
Decl has_active_shard(ShardType) bound [/name].

# shard_family(ShardID, Family) - shard belongs to a family (e.g., /planner, /coder)
Decl shard_family(ShardID, Family) bound [/string, /name].

# campaign_active(CampaignID) - currently active campaign
Decl campaign_active(CampaignID) bound [/string].

# -----------------------------------------------------------------------------
# 42.9 Injectable Context Derivation (Policy.mg Section 41)
# -----------------------------------------------------------------------------

# injectable_context(ShardID, Atom) - atoms selected for prompt injection
Decl injectable_context(ShardID, Atom) bound [/string, /string].

# injectable_context_priority(ShardID, Atom, Priority) - priority-tagged context
# Priority: /high, /medium, /low
Decl injectable_context_priority(ShardID, Atom, Priority) bound [/string, /string, /number].

# final_injectable(ShardID, Atom) - final set after budget filtering
Decl final_injectable(ShardID, Atom) bound [/string, /string].

# -----------------------------------------------------------------------------
# 42.10 Context Budget Management
# -----------------------------------------------------------------------------

# context_budget(ShardID, Budget) - available token budget for shard
Decl context_budget(ShardID, Budget) bound [/string, /number].

# context_budget_constrained(ShardID) - derived: shard has limited context budget
Decl context_budget_constrained(ShardID) bound [/string].

# context_budget_sufficient(ShardID) - derived: shard has adequate context budget
Decl context_budget_sufficient(ShardID) bound [/string].

# has_injectable_context(ShardID) - helper: shard has context to inject
Decl has_injectable_context(ShardID) bound [/string].

# has_high_priority_context(ShardID) - helper: shard has high-priority context
Decl has_high_priority_context(ShardID) bound [/string].

# -----------------------------------------------------------------------------
# 42.11 Context Staleness & Refresh
# -----------------------------------------------------------------------------

# context_stale(ShardID, Atom) - context atom is stale and needs refresh
Decl context_stale(ShardID, Atom) bound [/string, /string].

# has_stale_context(ShardID) - helper: shard has any stale context
Decl has_stale_context(ShardID) bound [/string].

# specialist_knowledge_updated(ShardID) - specialist knowledge was recently updated
Decl specialist_knowledge_updated(ShardID) bound [/string].

# -----------------------------------------------------------------------------
# 42.12 Trace Pattern Integration
# -----------------------------------------------------------------------------

# trace_pattern(TraceID, Pattern) - extracted pattern from a reasoning trace
Decl trace_pattern(TraceID, Pattern) bound [/string, /string].

# -----------------------------------------------------------------------------
# 42.13 Learning from Context Injection
# -----------------------------------------------------------------------------

# context_injection_effective(ShardID, Atom) - context injection led to success
Decl context_injection_effective(ShardID, Atom) bound [/string, /string].

# =============================================================================
# SECTION 45: JIT PROMPT COMPILER SCHEMAS
# =============================================================================
# Universal JIT Prompt Compiler for dynamic prompt assembly.
# Every LLM call gets a dynamically compiled prompt based on full context:
# operational mode, campaign phase, intent verb, test state, world model,
# shard type, init phase, northstar state, ouroboros stage, and more.

# -----------------------------------------------------------------------------
# 45.1 Prompt Atom Registry (EDB - loaded from SQLite databases)
# -----------------------------------------------------------------------------

# prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
# Core atom metadata for selection
# AtomID: Unique identifier for the atom (string)
# Category: /identity, /safety, /hallucination, /methodology, /language,
#           /framework, /domain, /campaign, /init, /northstar, /ouroboros,
#           /context, /exemplar, /protocol
# Priority: Base priority score (0-100)
# TokenCount: Estimated token count for budget management
# IsMandatory: /true if atom must be included, /false otherwise
Decl prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory) bound [/string, /name, /number, /number, /name].

# atom_selector(AtomID, Dimension, Value)
# Multi-value selectors for dimensional filtering
# Dimension: /operational_mode, /campaign_phase, /build_layer, /init_phase,
#            /northstar_phase, /ouroboros_stage, /intent_verb, /shard_type,
#            /language, /framework, /world_state
# Value: Name constant matching the dimension (e.g., /active, /coder, /go)
Decl atom_selector(AtomID, Dimension, Value) bound [/string, /name, /string].

# atom_dependency(AtomID, DependsOnID, DepType)
# DepType: /hard (must have), /soft (prefer), /order_only (just ordering)
Decl atom_dependency(AtomID, DependsOnID, DepType) bound [/string, /string, /name].

# atom_conflict(AtomA, AtomB)
# Mutual exclusion - cannot select both
Decl atom_conflict(AtomA, AtomB) bound [/string, /string].

# atom_exclusion_group(AtomID, GroupID)
# Only one atom per group can be selected
Decl atom_exclusion_group(AtomID, GroupID) bound [/string, /string].

# atom_content(AtomID, Content)
# Actual prompt text (loaded on demand, large strings)
Decl atom_content(AtomID, Content) bound [/string, /string].

# -----------------------------------------------------------------------------
# 45.2 Compilation Context (Set by Go before compilation)
# -----------------------------------------------------------------------------

# compile_context(Dimension, Value)
# Current compilation context asserted by Go runtime
# Dimension matches atom_selector dimensions
Decl compile_context(Dimension, Value) bound [/name, /string].

# compile_budget(TotalTokens)
# Available token budget for this compilation
Decl compile_budget(TotalTokens) bound [/number].

# compile_shard(ShardID, ShardType)
# Target shard for this compilation
Decl compile_shard(ShardID, ShardType) bound [/string, /name].

# compile_query(QueryText)
# Semantic query for vector search boosting
Decl compile_query(QueryText) bound [/string].

# -----------------------------------------------------------------------------
# 45.3 Vector Search Results (Asserted by Go after vector search)
# -----------------------------------------------------------------------------

# vector_recall_result(Query, AtomID, SimilarityScore)
# Results from vector store semantic search
# Query: The search query text
# AtomID: Matched atom identifier
# SimilarityScore: Cosine similarity (0.0-1.0)
Decl vector_recall_result(Query, AtomID, SimilarityScore) bound [/string, /string, /number].

# -----------------------------------------------------------------------------
# 45.4 Derived Selection Predicates (IDB - computed by rules)
# -----------------------------------------------------------------------------

# atom_matches_context(AtomID, Score)
# Computed match score based on context dimensions
Decl atom_matches_context(AtomID, Score) bound [/string, /number].

# atom_selected(AtomID)
# Atom passes all selection criteria
Decl atom_selected(AtomID) bound [/string].

# atom_excluded(AtomID, Reason)
# Atom excluded with reason: /conflict, /exclusion_group, /over_budget, /missing_dependency
Decl atom_excluded(AtomID, Reason) bound [/string, /string].

# atom_dependency_satisfied(AtomID)
# All hard dependencies are satisfied
Decl atom_dependency_satisfied(AtomID) bound [/string].

# atom_meets_threshold(AtomID)
# Helper: atom would meet score threshold (40) for selection
Decl atom_meets_threshold(AtomID) bound [/string].

# has_unsatisfied_hard_dep(AtomID)
# Helper: atom has at least one unsatisfied hard dependency
Decl has_unsatisfied_hard_dep(AtomID) bound [/string].

# is_excluded(AtomID)
# Helper: atom is excluded for any reason (for safe negation)
Decl is_excluded(AtomID) bound [/string].

# atom_candidate(AtomID)
# Helper: atom passes initial selection criteria (score threshold + deps)
Decl atom_candidate(AtomID) bound [/string].

# atom_loses_conflict(AtomID)
# Helper: atom loses due to conflict with higher-scoring atom
Decl atom_loses_conflict(AtomID) bound [/string].

# atom_loses_exclusion(AtomID)
# Helper: atom loses due to exclusion group with higher-scoring atom
Decl atom_loses_exclusion(AtomID) bound [/string].

# final_atom(AtomID, Order)
# Final ordered list for assembly
Decl final_atom(AtomID, Order) bound [/string, /number].

# -----------------------------------------------------------------------------
# 45.5 Compilation Validation
# -----------------------------------------------------------------------------

# compilation_valid()
# True if compilation passes all constraints
Decl compilation_valid().

# compilation_error(ErrorType, Details)
# ErrorType: /missing_mandatory, /circular_dependency, /unsatisfied_dependency, /budget_overflow
Decl compilation_error(ErrorType, Details) bound [/name, /string].

# has_compilation_error()
# Helper: true if any compilation error exists
Decl has_compilation_error().

# has_identity_atom()
# Helper: true if at least one identity atom is selected
Decl has_identity_atom().

# has_protocol_atom()
# Helper: true if at least one protocol atom is selected
Decl has_protocol_atom().

# -----------------------------------------------------------------------------
# 45.6 Category Ordering
# -----------------------------------------------------------------------------

# category_order(Category, OrderNum)
# Determines section order in final prompt
Decl category_order(Category, OrderNum) bound [/name, /number].

# category_budget(Category, Percent)
# Budget allocation percentage per category
Decl category_budget(Category, Percent) bound [/name, /number].

# -----------------------------------------------------------------------------
# 45.7 Additional JIT Compiler Schemas (for jit_compiler.mg compatibility)
# -----------------------------------------------------------------------------

# atom(AtomID)
# Base predicate for prompt atom existence
Decl atom(AtomID) bound [/string].

# atom_category(AtomID, Category)
# Atom's primary category (identity, protocol, safety, methodology, etc.)
Decl atom_category(AtomID, Category) bound [/string, /name].

# atom_tag(AtomID, Dimension, Tag)
# Alternative tagging predicate used by jit_compiler.mg
# Functionally equivalent to atom_selector but with /mode, /phase, /layer dimensions
# Dimension: /mode, /phase, /layer, /shard, /lang, /framework, /intent, /state, /tag
# Tag: Context value (e.g., /active, /coder, /go, /debug_only, /dream_only)
Decl atom_tag(AtomID, Dimension, Tag) bound [/string, /name, /string].

# vector_hit(AtomID, Score)
# Vector search results injected by Go runtime before compilation
# AtomID: Matched atom identifier
# Score: Cosine similarity score (0.0-1.0)
Decl vector_hit(AtomID, Score) bound [/string, /number].

# current_context(Dimension, Tag)
# Runtime context state injected by Go (alternative to compile_context)
# Used by jit_compiler.mg for context matching
Decl current_context(Dimension, Tag) bound [/name, /string].

# is_mandatory(AtomID)
# Flag indicating atom must be selected if context matches
Decl is_mandatory(AtomID) bound [/string].

# atom_requires(AtomID, DependencyID)
# Hard dependency: AtomID requires DependencyID to be selected
Decl atom_requires(AtomID, DependencyID) bound [/string, /string].

# atom_conflicts(AtomA, AtomB)
# Mutual exclusion: AtomA and AtomB cannot both be selected
Decl atom_conflicts(AtomA, AtomB) bound [/string, /string].

# atom_priority(AtomID, Priority)
# Base priority score for atom ordering
Decl atom_priority(AtomID, Priority) bound [/string, /number].

# -----------------------------------------------------------------------------
# 45.8 Section 46 Selection Rule Schemas (IDB - computed by policy.mg Section 46)
# -----------------------------------------------------------------------------

# skeleton_category(Category)
# Categories that form the mandatory skeleton of every prompt
Decl skeleton_category(Category) bound [/name].

# mandatory_atom(AtomID)
# Atom must be included in prompt (Skeleton layer)
Decl mandatory_atom(AtomID) bound [/string].

# base_prohibited(AtomID)
# Base prohibition from context rules (Stratum 0, no dependency on mandatory)
Decl base_prohibited(AtomID) bound [/string].

# prohibited_atom(AtomID)
# Atom is blocked by firewall rules
Decl prohibited_atom(AtomID) bound [/string].

# candidate_atom(AtomID)
# Atom is a valid candidate for selection (Flesh layer)
Decl candidate_atom(AtomID) bound [/string].

# conflict_loser(AtomID)
# Helper: atom loses in conflict resolution (lower priority in conflict pair)
Decl conflict_loser(AtomID) bound [/string].

# selected_atom(AtomID)
# Final selection: mandatory OR valid candidate (not a conflict loser)
Decl selected_atom(AtomID) bound [/string].

# atom_context_boost(AtomID, BoostedPriority)
# Priority boost based on context matching
Decl atom_context_boost(AtomID, BoostedPriority) bound [/string, /number].

# has_skeleton_category(Category)
# Helper: true if at least one atom from this skeleton category is selected
Decl has_skeleton_category(Category) bound [/name].

# missing_skeleton_category(Category)
# Helper: skeleton category with no selected atoms (compilation error)
Decl missing_skeleton_category(Category) bound [/name].

