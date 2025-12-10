# 450: JIT Prompt Compiler Predicates

**Purpose**: Complete reference for Section 45 of the codeNERD Mangle policy - predicates for JIT prompt compilation, atom selection, and spreading activation.

## Overview

Section 45 provides Mangle predicates for the JIT Prompt Compiler's atom selection and compilation logic. This system implements a sophisticated spreading activation algorithm that:

1. Selects prompt atoms based on multi-dimensional context matching
2. Resolves dependencies and conflicts among atoms
3. Orders atoms by category and relevance
4. Validates compilation integrity
5. Feeds back learning signals for autopoiesis

**Philosophy**: The prompt is not static text but a **logic-derived assembly** where the kernel selects the most relevant instruction atoms from a knowledge base based on current context dimensions (shard type, operational mode, campaign phase, language, framework, etc.).

## Quick Reference

| Predicate | Type | Purpose |
|-----------|------|---------|
| `prompt_atom/5` | EDB | Core atom metadata |
| `atom_selector/3` | EDB | Multi-value dimensional selectors |
| `atom_dependency/3` | EDB | Dependency relationships |
| `atom_conflict/2` | EDB | Mutual exclusion constraints |
| `compile_context/2` | Runtime | Current compilation context |
| `atom_matches_context/2` | IDB | Computed match scores |
| `atom_selected/1` | IDB | Final selected atoms |
| `final_atom/2` | IDB | Ordered atom list |
| `compilation_valid/0` | IDB | Validation predicate |

---

## 45.1 Prompt Atom Registry (EDB)

### Core Atom Metadata

```mangle
# prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory)
# Core atom metadata for selection
# AtomID: Unique identifier for the atom (string)
# Category: /identity, /safety, /hallucination, /methodology, /language,
#           /framework, /domain, /campaign, /init, /northstar, /ouroboros,
#           /context, /exemplar, /protocol
# Priority: Base priority score (0-100)
# TokenCount: Estimated token count for budget management
# IsMandatory: /true if atom must be included, /false otherwise
Decl prompt_atom(AtomID, Category, Priority, TokenCount, IsMandatory).
```

**Example**:
```mangle
prompt_atom("core_identity_v1", /identity, 90, 150, /true).
prompt_atom("go_error_handling", /methodology, 70, 200, /false).
prompt_atom("bubbletea_patterns", /framework, 65, 300, /false).
```

### Multi-Value Selectors

```mangle
# atom_selector(AtomID, Dimension, Value)
# Multi-value selectors for dimensional filtering
# Dimension: /operational_mode, /campaign_phase, /build_layer, /init_phase,
#            /northstar_phase, /ouroboros_stage, /intent_verb, /shard_type,
#            /language, /framework, /world_state
# Value: Name constant matching the dimension (e.g., /active, /coder, /go)
Decl atom_selector(AtomID, Dimension, Value).
```

**Example**:
```mangle
# An atom can match multiple selectors
atom_selector("go_error_handling", /language, /go).
atom_selector("go_error_handling", /shard_type, /coder).
atom_selector("go_error_handling", /operational_mode, /tdd_repair).

# Atoms can target multiple values in same dimension
atom_selector("test_patterns", /language, /go).
atom_selector("test_patterns", /language, /python).
atom_selector("test_patterns", /language, /rust).
```

### Dependencies and Conflicts

```mangle
# atom_dependency(AtomID, DependsOnID, DepType)
# DepType: /hard (must have), /soft (prefer), /order_only (just ordering)
Decl atom_dependency(AtomID, DependsOnID, DepType).

# atom_conflict(AtomA, AtomB)
# Mutual exclusion - cannot select both
Decl atom_conflict(AtomA, AtomB).

# atom_exclusion_group(AtomID, GroupID)
# Only one atom per group can be selected
Decl atom_exclusion_group(AtomID, GroupID).
```

**Example**:
```mangle
# Hard dependency - advanced atom requires basic atom
atom_dependency("advanced_bubbletea", "bubbletea_basics", /hard).

# Soft dependency - prefers but doesn't require
atom_dependency("rod_integration", "go_error_handling", /soft).

# Conflict - can't have both approaches
atom_conflict("sync_approach", "async_approach").

# Exclusion group - only one test framework
atom_exclusion_group("testify_patterns", "test_frameworks").
atom_exclusion_group("ginkgo_patterns", "test_frameworks").
atom_exclusion_group("goconvey_patterns", "test_frameworks").
```

### Atom Content

```mangle
# atom_content(AtomID, Content)
# Actual prompt text (loaded on demand, large strings)
Decl atom_content(AtomID, Content).
```

**Example**:
```mangle
atom_content("core_identity_v1", "You are codeNERD, a logic-first coding agent...").
```

---

## 45.2 Compilation Context (Runtime)

### Context Dimensions

```mangle
# compile_context(Dimension, Value)
# Current compilation context asserted by Go runtime
# Dimension matches atom_selector dimensions
Decl compile_context(Dimension, Value).

# compile_budget(TotalTokens)
# Available token budget for this compilation
Decl compile_budget(TotalTokens).

# compile_shard(ShardID, ShardType)
# Target shard for this compilation
Decl compile_shard(ShardID, ShardType).

# compile_query(QueryText)
# Semantic query for vector search boosting
Decl compile_query(QueryText).
```

**Example runtime facts**:
```mangle
# Go asserts these before compilation
compile_context(/shard_type, /coder).
compile_context(/operational_mode, /tdd_repair).
compile_context(/language, /go).
compile_context(/framework, /bubbletea).
compile_context(/world_state, /failing_tests).
compile_budget(8000).
compile_shard("coder_42", /coder).
compile_query("How to fix test failures in Go with Bubbletea").
```

---

## 45.3 Vector Search Results

```mangle
# vector_recall_result(Query, AtomID, SimilarityScore)
# Results from vector store semantic search
# Query: The search query text
# AtomID: Matched atom identifier
# SimilarityScore: Cosine similarity (0.0-1.0)
Decl vector_recall_result(Query, AtomID, SimilarityScore).
```

**Example**:
```mangle
# Go asserts vector search results
vector_recall_result("fix test failures", "tdd_repair_protocol", 0.92).
vector_recall_result("fix test failures", "go_error_handling", 0.87).
vector_recall_result("fix test failures", "test_patterns", 0.81).
```

---

## 45.4 Derived Selection (IDB)

### Match Scoring

```mangle
# atom_matches_context(AtomID, Score)
# Computed match score based on context dimensions
Decl atom_matches_context(AtomID, Score).
```

**Computed by**: Section 45.3 rules (spreading activation)

### Selection Predicates

```mangle
# atom_selected(AtomID)
# Atom passes all selection criteria
Decl atom_selected(AtomID).

# atom_excluded(AtomID, Reason)
# Atom excluded with reason: /conflict, /exclusion_group, /over_budget, /missing_dependency
Decl atom_excluded(AtomID, Reason).

# atom_dependency_satisfied(AtomID)
# All hard dependencies are satisfied
Decl atom_dependency_satisfied(AtomID).
```

### Helper Predicates

```mangle
# atom_meets_threshold(AtomID)
# Helper: atom would meet score threshold (40) for selection
Decl atom_meets_threshold(AtomID).

# has_unsatisfied_hard_dep(AtomID)
# Helper: atom has at least one unsatisfied hard dependency
Decl has_unsatisfied_hard_dep(AtomID).

# is_excluded(AtomID)
# Helper: atom is excluded for any reason (for safe negation)
Decl is_excluded(AtomID).

# atom_candidate(AtomID)
# Helper: atom passes initial selection criteria (score threshold + deps)
Decl atom_candidate(AtomID).

# atom_loses_conflict(AtomID)
# Helper: atom loses due to conflict with higher-scoring atom
Decl atom_loses_conflict(AtomID).

# atom_loses_exclusion(AtomID)
# Helper: atom loses due to exclusion group with higher-scoring atom
Decl atom_loses_exclusion(AtomID).
```

### Final Ordering

```mangle
# final_atom(AtomID, Order)
# Final ordered list for assembly
# Order = (CategoryOrder * 1000) + Score
Decl final_atom(AtomID, Order).
```

---

## 45.5 Scoring Rules (Spreading Activation)

### Base Score

```mangle
# All atoms start with their declared priority
atom_matches_context(AtomID, Priority) :-
    prompt_atom(AtomID, _, Priority, _, _).
```

### Contextual Boosts

Each dimension match adds a boost to the base priority:

| Dimension | Boost | Description |
|-----------|-------|-------------|
| **Shard Type Match** | +30 | Atoms designed for this shard type |
| **Intent Verb Match** | +25 | Verb-specific atoms (fix, debug, refactor) |
| **Operational Mode** | +20 | Mode-specific atoms (debugging, tdd_repair) |
| **World State Match** | +20 | World-state atoms (failing_tests, diagnostics) |
| **Campaign Phase** | +15 | Phase-specific atoms (planning, validating) |
| **Framework Match** | +15 | Framework-specific atoms (bubbletea, gin, rod) |
| **Init Phase Match** | +15 | Init-phase atoms (analysis, kb_agent) |
| **Ouroboros Stage** | +15 | Ouroboros-stage atoms (specification, refinement) |
| **Northstar Phase** | +15 | Northstar-phase atoms (doc_ingestion, requirements) |
| **Language Match** | +10 | Language-specific atoms (go, python) |
| **Build Layer Match** | +10 | Build-layer atoms (scaffold, service) |
| **Vector Similarity** | 0-30 | Similarity × 30 (semantic match) |
| **Mandatory Atoms** | 100 | Always max score |

### Boost Examples

```mangle
# Boost for shard type match (+30)
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /shard_type, ShardType),
    compile_shard(_, ShardType),
    Boosted = fn:plus(Priority, 30).

# Boost for intent verb match (+25)
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /intent_verb, Verb),
    compile_context(/intent_verb, Verb),
    Boosted = fn:plus(Priority, 25).

# Boost for operational mode match (+20)
atom_matches_context(AtomID, Boosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    atom_selector(AtomID, /operational_mode, Mode),
    compile_context(/operational_mode, Mode),
    Boosted = fn:plus(Priority, 20).

# Vector similarity boost (scaled 0-30)
atom_matches_context(AtomID, VecBoosted) :-
    prompt_atom(AtomID, _, Priority, _, _),
    compile_query(Query),
    vector_recall_result(Query, AtomID, Similarity),
    VecBoost = fn:mult(Similarity, 30),
    VecBoosted = fn:plus(Priority, VecBoost).

# Mandatory atoms always get max score (100)
atom_matches_context(AtomID, 100) :-
    prompt_atom(AtomID, _, _, _, /true).
```

**Example scoring**:
```mangle
# Atom: "go_error_handling"
# Base priority: 70
# Matches: language=/go (+10), shard_type=/coder (+30), intent_verb=/fix (+25)
# Vector similarity: 0.87 → 0.87 × 30 = 26.1
# Total score: 70 + 10 + 30 + 25 + 26 = 161

# Final score is MAX of all matching rules (due to union semantics)
atom_matches_context("go_error_handling", 161).
```

---

## 45.6 Category Ordering

### Order Constants

Categories determine prompt section order (lower numbers first):

```mangle
category_order(/identity, 1).
category_order(/safety, 2).
category_order(/hallucination, 3).
category_order(/methodology, 4).
category_order(/language, 5).
category_order(/framework, 6).
category_order(/domain, 7).
category_order(/campaign, 8).
category_order(/init, 8).         # Same as campaign
category_order(/northstar, 8).    # Same as campaign
category_order(/ouroboros, 8).    # Same as campaign
category_order(/context, 9).
category_order(/exemplar, 10).
category_order(/protocol, 11).
```

### Budget Allocation

Percentage of total token budget per category:

```mangle
category_budget(/identity, 5).
category_budget(/protocol, 12).
category_budget(/safety, 5).
category_budget(/hallucination, 8).
category_budget(/methodology, 15).
category_budget(/language, 8).
category_budget(/framework, 8).
category_budget(/domain, 15).
category_budget(/context, 12).
category_budget(/exemplar, 7).
category_budget(/campaign, 5).
category_budget(/init, 5).
category_budget(/northstar, 5).
category_budget(/ouroboros, 5).
```

### Final Ordering Rule

```mangle
# Order selected atoms by category first, then by match score within category
# Order value = (CategoryOrder * 1000) + Score
final_atom(AtomID, Order) :-
    atom_selected(AtomID),
    prompt_atom(AtomID, Category, _, _, _),
    category_order(Category, CatOrder),
    atom_matches_context(AtomID, Score),
    Order = fn:plus(fn:mult(CatOrder, 1000), Score).
```

**Example**:
```mangle
# Identity atom with score 90: Order = (1 × 1000) + 90 = 1090
# Protocol atom with score 85: Order = (11 × 1000) + 85 = 11085
# Identity atoms always appear first in final prompt
```

---

## 45.7 Dependency Resolution

### Stratification Strategy

Dependencies use a **score-based approach** to avoid circular reasoning with `atom_selected`:

1. **Key insight**: A dependency is "satisfiable" if the dependent atom would **meet the minimum score threshold (40)**, not if it's actually selected
2. This allows dependency checking to happen **before** selection (different stratum)
3. Prevents cycles: `atom_dependency_satisfied` → `atom_candidate` → `atom_selected`

### Dependency Rules

```mangle
# Helper: atom would meet score threshold (potential candidate)
atom_meets_threshold(AtomID) :-
    atom_matches_context(AtomID, Score),
    Score > 40.

# Helper: atom is mandatory (always meets threshold)
atom_meets_threshold(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true).

# Helper: atom has at least one unsatisfiable hard dependency
# A dependency is unsatisfiable if the target atom exists but wouldn't meet threshold
has_unsatisfied_hard_dep(AtomID) :-
    atom_dependency(AtomID, DepID, /hard),
    prompt_atom(DepID, _, _, _, _),
    !atom_meets_threshold(DepID).

# Atom dependencies are satisfied if no unsatisfiable hard deps exist
atom_dependency_satisfied(AtomID) :-
    prompt_atom(AtomID, _, _, _, _),
    !has_unsatisfied_hard_dep(AtomID).
```

**Example**:
```mangle
# Atom A depends on B with /hard
atom_dependency("advanced_pattern", "basic_pattern", /hard).

# If basic_pattern only scores 35 (< 40 threshold):
has_unsatisfied_hard_dep("advanced_pattern").  # Derives /true
atom_dependency_satisfied("advanced_pattern"). # Fails (negation)
# Result: advanced_pattern won't be selected
```

---

## 45.8 Selection Algorithm (Stratified)

Uses a **three-phase stratified approach** to avoid negative cycles:

### Phase 1: Candidate Identification (Stratum 0)

```mangle
# Candidate atoms pass score threshold and have satisfied dependencies
# Computed without any negation on selection predicates
atom_candidate(AtomID) :-
    atom_matches_context(AtomID, Score),
    Score > 40,
    atom_dependency_satisfied(AtomID).

# Mandatory atoms are always candidates
atom_candidate(AtomID) :-
    prompt_atom(AtomID, _, _, _, /true).
```

### Phase 2: Conflict Detection (Stratum 1)

```mangle
# An atom loses to a conflicting atom with higher score
atom_loses_conflict(AtomID) :-
    atom_candidate(AtomID),
    atom_conflict(AtomID, OtherID),
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# Handle symmetric conflicts
atom_loses_conflict(AtomID) :-
    atom_candidate(AtomID),
    atom_conflict(OtherID, AtomID),
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# An atom loses in exclusion group to higher-scoring atom
atom_loses_exclusion(AtomID) :-
    atom_candidate(AtomID),
    atom_exclusion_group(AtomID, GroupID),
    atom_exclusion_group(OtherID, GroupID),
    AtomID != OtherID,
    atom_candidate(OtherID),
    atom_matches_context(AtomID, MyScore),
    atom_matches_context(OtherID, OtherScore),
    OtherScore > MyScore.

# Helper: atom is excluded for any reason
is_excluded(AtomID) :-
    atom_loses_conflict(AtomID).

is_excluded(AtomID) :-
    atom_loses_exclusion(AtomID).

is_excluded(AtomID) :-
    prompt_atom(AtomID, _, _, _, _),
    !atom_dependency_satisfied(AtomID).
```

### Phase 3: Final Selection (Stratum 2)

```mangle
# Final selection - candidates that are not excluded
atom_selected(AtomID) :-
    atom_candidate(AtomID),
    !is_excluded(AtomID).
```

**Example conflict resolution**:
```mangle
# Two conflicting atoms
atom_conflict("sync_approach", "async_approach").
prompt_atom("sync_approach", /methodology, 60, 200, /false).
prompt_atom("async_approach", /methodology, 70, 250, /false).

# Both are candidates (score > 40)
atom_candidate("sync_approach").
atom_candidate("async_approach").

# But async has higher score (70 > 60)
atom_loses_conflict("sync_approach").  # Derives /true
is_excluded("sync_approach").           # Derives /true
atom_selected("async_approach").        # Only async selected
```

---

## 45.9 Validation Rules

### Core Validation

```mangle
# Helper: at least one identity atom is selected
has_identity_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /identity, _, _, _).

# Helper: at least one protocol atom is selected
has_protocol_atom() :-
    atom_selected(AtomID),
    prompt_atom(AtomID, /protocol, _, _, _).

# Helper: at least one compilation error exists
has_compilation_error() :-
    compilation_error(_, _).

# Compilation is valid if: has identity, has protocol, no errors
compilation_valid() :-
    has_identity_atom(),
    has_protocol_atom(),
    !has_compilation_error().
```

### Error Detection

```mangle
# compilation_error(ErrorType, Details)
# ErrorType: /missing_mandatory, /circular_dependency, /unsatisfied_dependency, /budget_overflow
Decl compilation_error(ErrorType, Details).

# Error: missing mandatory atom (mandatory atom not selected)
compilation_error(/missing_mandatory, AtomID) :-
    prompt_atom(AtomID, _, _, _, /true),
    !atom_selected(AtomID).

# Error: circular dependency (simplified - full detection in Go)
# Direct cycle detection: A depends on B and B depends on A
compilation_error(/circular_dependency, AtomID) :-
    atom_dependency(AtomID, DepID, /hard),
    atom_dependency(DepID, AtomID, /hard).
```

**Usage**:
```mangle
# Query for validation
?compilation_valid()
# Result: /true if valid, no results if invalid

# Query for errors
?compilation_error(Type, Details)
# Result: All detected errors with details
```

---

## 45.10 Integration with Spreading Activation

### Activation Propagation

```mangle
# High activation for selected atoms
activation(AtomID, 95) :-
    atom_selected(AtomID).

# Medium activation for atoms matching context but not selected
activation(AtomID, 60) :-
    atom_matches_context(AtomID, Score),
    Score > 30,
    !atom_selected(AtomID).
```

This allows selected atoms to boost related facts in the broader spreading activation network (Section 9).

---

## 45.11 Learning Signals (Autopoiesis)

### Effectiveness Tracking

```mangle
# Signal: atom was selected and shard execution succeeded
effective_prompt_atom(AtomID) :-
    atom_selected(AtomID),
    compile_shard(ShardID, _),
    shard_executed(ShardID, _, /success, _).

# Learning signal: promote effective atoms to higher priority
learning_signal(/effective_prompt_atom, AtomID) :-
    effective_prompt_atom(AtomID).
```

These signals feed into Section 21 (Autopoiesis Learning) to adjust atom priorities over time based on success patterns.

---

## Complete Example: CoderShard Compilation

### Setup

```mangle
# ===== Atom Registry =====
prompt_atom("core_identity", /identity, 95, 150, /true).
prompt_atom("piggyback_protocol", /protocol, 90, 200, /true).
prompt_atom("go_basics", /language, 60, 300, /false).
prompt_atom("go_advanced", /language, 70, 400, /false).
prompt_atom("tdd_repair_guide", /methodology, 75, 350, /false).
prompt_atom("bubbletea_patterns", /framework, 65, 300, /false).

# ===== Selectors =====
atom_selector("go_basics", /language, /go).
atom_selector("go_basics", /shard_type, /coder).

atom_selector("go_advanced", /language, /go).
atom_selector("go_advanced", /shard_type, /coder).
atom_selector("go_advanced", /operational_mode, /tdd_repair).

atom_selector("tdd_repair_guide", /operational_mode, /tdd_repair).
atom_selector("tdd_repair_guide", /world_state, /failing_tests).

atom_selector("bubbletea_patterns", /framework, /bubbletea).
atom_selector("bubbletea_patterns", /language, /go).

# ===== Dependencies =====
atom_dependency("go_advanced", "go_basics", /hard).
atom_dependency("bubbletea_patterns", "go_basics", /soft).

# ===== Runtime Context =====
compile_context(/shard_type, /coder).
compile_context(/operational_mode, /tdd_repair).
compile_context(/language, /go).
compile_context(/framework, /bubbletea).
compile_context(/world_state, /failing_tests).
compile_budget(8000).
compile_shard("coder_42", /coder).
compile_query("Fix test failures in Go Bubbletea app").

# ===== Vector Results =====
vector_recall_result("Fix test failures...", "tdd_repair_guide", 0.95).
vector_recall_result("Fix test failures...", "go_advanced", 0.82).
vector_recall_result("Fix test failures...", "bubbletea_patterns", 0.78).
```

### Scoring Derivation

```mangle
# ===== core_identity (mandatory) =====
atom_matches_context("core_identity", 100).  # Mandatory

# ===== piggyback_protocol (mandatory) =====
atom_matches_context("piggyback_protocol", 100).  # Mandatory

# ===== go_basics =====
atom_matches_context("go_basics", 60).   # Base priority
atom_matches_context("go_basics", 70).   # +10 language match
atom_matches_context("go_basics", 100).  # +30 shard_type match
# Max score: 100

# ===== go_advanced =====
atom_matches_context("go_advanced", 70).   # Base priority
atom_matches_context("go_advanced", 80).   # +10 language match
atom_matches_context("go_advanced", 110).  # +30 shard_type match
atom_matches_context("go_advanced", 120).  # +20 operational_mode match
atom_matches_context("go_advanced", 94.6). # +24.6 vector (0.82 × 30)
# Max score: 120

# ===== tdd_repair_guide =====
atom_matches_context("tdd_repair_guide", 75).   # Base priority
atom_matches_context("tdd_repair_guide", 95).   # +20 operational_mode match
atom_matches_context("tdd_repair_guide", 115).  # +20 world_state match
atom_matches_context("tdd_repair_guide", 103.5).# +28.5 vector (0.95 × 30)
# Max score: 115

# ===== bubbletea_patterns =====
atom_matches_context("bubbletea_patterns", 65).   # Base priority
atom_matches_context("bubbletea_patterns", 80).   # +15 framework match
atom_matches_context("bubbletea_patterns", 75).   # +10 language match
atom_matches_context("bubbletea_patterns", 88.4). # +23.4 vector (0.78 × 30)
# Max score: 88
```

### Selection

```mangle
# ===== Dependency Satisfaction =====
atom_meets_threshold("go_basics").         # Score 100 > 40
atom_meets_threshold("go_advanced").       # Score 120 > 40
atom_meets_threshold("tdd_repair_guide").  # Score 115 > 40
atom_meets_threshold("bubbletea_patterns").# Score 88 > 40

atom_dependency_satisfied("go_basics").         # No dependencies
atom_dependency_satisfied("go_advanced").       # Depends on go_basics (meets threshold)
atom_dependency_satisfied("tdd_repair_guide").  # No dependencies
atom_dependency_satisfied("bubbletea_patterns").# Soft dep on go_basics (OK)

# ===== Candidates =====
atom_candidate("core_identity").      # Mandatory
atom_candidate("piggyback_protocol"). # Mandatory
atom_candidate("go_basics").          # Score > 40, deps satisfied
atom_candidate("go_advanced").        # Score > 40, deps satisfied
atom_candidate("tdd_repair_guide").   # Score > 40, deps satisfied
atom_candidate("bubbletea_patterns"). # Score > 40, deps satisfied

# ===== No Conflicts/Exclusions =====
# (None defined in this example)

# ===== Final Selection =====
atom_selected("core_identity").
atom_selected("piggyback_protocol").
atom_selected("go_basics").
atom_selected("go_advanced").
atom_selected("tdd_repair_guide").
atom_selected("bubbletea_patterns").
```

### Ordering

```mangle
# final_atom(AtomID, Order) where Order = (CatOrder × 1000) + Score

final_atom("core_identity", 1100).        # (1 × 1000) + 100
final_atom("go_basics", 5100).            # (5 × 1000) + 100
final_atom("go_advanced", 5120).          # (5 × 1000) + 120
final_atom("tdd_repair_guide", 4115).     # (4 × 1000) + 115
final_atom("bubbletea_patterns", 6088).   # (6 × 1000) + 88
final_atom("piggyback_protocol", 11100).  # (11 × 1000) + 100
```

### Assembled Prompt Order

1. **core_identity** (Order 1100) - Identity category first
2. **tdd_repair_guide** (Order 4115) - Methodology category
3. **go_basics** (Order 5100) - Language category
4. **go_advanced** (Order 5120) - Language category (higher score)
5. **bubbletea_patterns** (Order 6088) - Framework category
6. **piggyback_protocol** (Order 11100) - Protocol category last

### Validation

```mangle
?compilation_valid()
# Result: /true

# Breakdown:
has_identity_atom().      # core_identity selected ✓
has_protocol_atom().      # piggyback_protocol selected ✓
!has_compilation_error(). # No errors ✓
```

---

## Common Patterns

### Pattern 1: Progressive Enhancement

```mangle
# Basic → Intermediate → Advanced chain
atom_dependency("go_intermediate", "go_basics", /hard).
atom_dependency("go_advanced", "go_intermediate", /hard).

# Only advanced selected if all dependencies meet threshold
# Creates natural progression in prompt complexity
```

### Pattern 2: Mutually Exclusive Approaches

```mangle
# Two conflicting methodologies
atom_conflict("waterfall_approach", "agile_approach").

# Higher-scoring approach wins automatically
# Prevents contradictory instructions in prompt
```

### Pattern 3: Feature Groups

```mangle
# Only one test framework
atom_exclusion_group("testify_guide", "test_frameworks").
atom_exclusion_group("ginkgo_guide", "test_frameworks").
atom_exclusion_group("goconvey_guide", "test_frameworks").

# Highest-scoring framework selected
# Keeps prompt focused on one tool
```

### Pattern 4: Conditional Specialization

```mangle
# Debug atoms only active when debugging
atom_selector("debug_workflow", /operational_mode, /debugging).
atom_selector("debug_workflow", /world_state, /error_detected).

# TDD atoms only active during repair
atom_selector("tdd_loop", /operational_mode, /tdd_repair).
atom_selector("tdd_loop", /world_state, /failing_tests).

# Context automatically selects relevant expertise
```

### Pattern 5: Multi-Language Support

```mangle
# Atoms can target multiple languages
atom_selector("error_handling_general", /language, /go).
atom_selector("error_handling_general", /language, /rust).
atom_selector("error_handling_general", /language, /python).

# Single atom serves multiple contexts
# Reduces atom proliferation
```

---

## Integration with codeNERD Architecture

### Go → Mangle Flow

```go
// 1. Go assembles compilation context
kernel.AddFact("compile_context", /shard_type, /coder)
kernel.AddFact("compile_context", /language, /go)
kernel.AddFact("compile_budget", 8000)

// 2. Vector search for semantic atoms
results := vectorStore.Search(query)
for _, r := range results {
    kernel.AddFact("vector_recall_result", query, r.AtomID, r.Score)
}

// 3. Mangle derives selection
kernel.Query("atom_selected(AtomID)")
kernel.Query("final_atom(AtomID, Order)")

// 4. Go assembles prompt text
atoms := kernel.Query("final_atom(AtomID, Order) |> do fn:group_by(), let _ = fn:Count()")
orderedAtoms := sortByOrder(atoms)
prompt := assemblePrompt(orderedAtoms)
```

### Mangle → Go Flow

```go
// After shard execution, feed back results
if shardResult.Success {
    kernel.AddFact("shard_executed", shardID, task, /success, result)
}

// Mangle derives learning signals
kernel.Query("learning_signal(/effective_prompt_atom, AtomID)")

// Go updates atom priorities in persistence
for _, signal := range learningSignals {
    db.IncrementPriority(signal.AtomID, delta)
}
```

---

## Debugging Queries

### What atoms are selected?

```mangle
?atom_selected(AtomID)
```

### What's the score for a specific atom?

```mangle
?atom_matches_context("go_advanced", Score)
# Returns all matching scores (base + boosts)
```

### Why was an atom excluded?

```mangle
?atom_excluded("my_atom", Reason)
# Reason: /conflict, /exclusion_group, /missing_dependency
```

### What's the final order?

```mangle
?final_atom(AtomID, Order)
# Order atoms by Order value ascending
```

### What dependencies failed?

```mangle
?has_unsatisfied_hard_dep("my_atom")
# Returns /true if dependencies not met
```

### Is compilation valid?

```mangle
?compilation_valid()
# Returns /true if valid, no results if invalid

?compilation_error(Type, Details)
# Returns all errors if invalid
```

### What atoms matched the query vector?

```mangle
?vector_recall_result(Query, AtomID, Score)
# Shows semantic matches
```

### What's in the compile context?

```mangle
?compile_context(Dimension, Value)
# Shows all active context dimensions
```

---

## Performance Considerations

### Optimization Tips

1. **Use exclusion groups** instead of pairwise conflicts for large sets
   - `atom_exclusion_group` is O(n) vs O(n²) for pairwise `atom_conflict`

2. **Limit vector results** to top-k (e.g., 20) most similar
   - Prevents excessive `atom_matches_context` derivations

3. **Set realistic thresholds** (default: 40)
   - Lower threshold = more candidates = slower conflict resolution

4. **Avoid deep dependency chains**
   - Each level adds stratification complexity
   - Keep chains < 5 levels

5. **Cache category ordering** in Go
   - Static facts don't need recomputation

### Stratification Complexity

Current stratification levels:

- **Stratum 0**: `atom_matches_context`, `atom_meets_threshold`, `atom_dependency_satisfied`
- **Stratum 1**: `atom_candidate`, `has_unsatisfied_hard_dep`
- **Stratum 2**: `atom_loses_conflict`, `atom_loses_exclusion`, `is_excluded`
- **Stratum 3**: `atom_selected`
- **Stratum 4**: `final_atom`
- **Stratum 5**: `compilation_valid`, `has_identity_atom`, etc.

Total: 6 strata (efficient for semi-naive evaluation)

---

## Related Sections

- **Section 9**: Spreading Activation (receives activation from selected atoms)
- **Section 21**: Autopoiesis Learning (consumes `learning_signal` facts)
- **Section 7B**: Virtual Predicates (FFI for `compile_context`, `vector_recall_result`)
- **Section 11**: Intent Classification (generates `compile_context(/intent_verb, ...)`)

---

## References

- Implementation: `c:\CodeProjects\codeNERD\internal\core\defaults\policy.mg` (Lines 2895-3227)
- Schema: `c:\CodeProjects\codeNERD\internal\core\defaults\schemas.mg` (Lines 2635-2783)
- Go Integration: `c:\CodeProjects\codeNERD\internal\articulation\prompt_assembler.go`

---

**Next**: See [900-ECOSYSTEM](900-ECOSYSTEM.md) for Go integration patterns with the JIT Prompt Compiler.
