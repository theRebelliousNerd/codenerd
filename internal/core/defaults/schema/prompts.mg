# Prompt Atom Schema & Standard Atoms
# Defines the vocabulary for JIT Prompt Compilation.

# --- 1. CORE SCHEMA ---

# atom(ID)
# Identifies a prompt atom.
# defined by: atom(ID).

# atom_category(ID, Category)
# Categorizes atoms for sorting/grouping.
# Categories: /identity, /protocol, /safety, /methodology, /hallucination, 
#             /language, /framework, /domain, /campaign, /init, /context, /exemplar
# defined by: atom_category(ID, Category).

# atom_description(ID, Text)
# Semantic description for vector embedding/search.
# defined by: atom_description(ID, Text).

# atom_content_type(ID, Type)
# Type of content: /standard, /concise, /min.
# defined by: atom_content_type(ID, Type).

# --- 2. CONTEXT TAGS (Normalized Link Table) ---

# atom_tag(ID, Dimension, Tag)
# Tagging system for context matching.
# Dimensions: /mode, /phase, /layer, /shard, /lang, /framework, /intent, /state
# defined by: atom_tag(ID, Dimension, Tag).

# --- 3. RELATIONS ---

# atom_requires(ID, DependencyID)
# Hard dependency: If ID is selected, DependencyID MUST be selected.
# defined by: atom_requires(ID, DependencyID).

# atom_conflicts(ID, ConflictID)
# Exclusion: ID cannot coexist with ConflictID (unless suppressed).
# defined by: atom_conflicts(ID, ConflictID).

# atom_exclusive(ID, GroupID)
# Mutual Exclusion: Only one atom from GroupID can be selected.
# defined by: atom_exclusive(ID, GroupID).

# --- 4. ATTRIBUTES ---

# atom_priority(ID, Score)
# Sorting priority (Higher = Earlier in prompt).
# defined by: atom_priority(ID, Score).

# is_mandatory(ID)
# Flag: This atom is mandatory if context matches.
# defined by: is_mandatory(ID).

# --- 5. RUNTIME INPUTS (Injected by Go) ---

# vector_hit(ID, Score)
# Atom found by vector search with similarity score.

# current_context(Dimension, Tag)
# The current environment state (e.g., current_context(/mode, /active)).

# token_budget(Limit)
# Available token budget.

