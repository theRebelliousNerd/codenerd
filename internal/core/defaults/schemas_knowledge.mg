# Cortex 1.5.0 Schemas (EDB Declarations)
# Version: 1.5.0
# Philosophy: Logic determines Reality; the Model merely describes it.

# Modular Schema: KNOWLEDGE
# Sections: 24, 25, 26, 44, 52

# =============================================================================
# SECTION 24: KNOWLEDGE ATOMS (Research Results)
# =============================================================================

# knowledge_atom(SourceURL, Concept, Title, Confidence)
Decl knowledge_atom(SourceURL, Concept, Title, Confidence) bound [/string, /string, /string, /number].

# code_pattern(Concept, PatternCode)
Decl code_pattern(Concept, PatternCode) bound [/string, /string].

# anti_pattern(Concept, PatternCode, Reason)
Decl anti_pattern(Concept, PatternCode, Reason) bound [/string, /string, /string].

# research_complete(Query, AtomCount, DurationSeconds)
Decl research_complete(Query, AtomCount, DurationSeconds) bound [/string, /number, /number].

# =============================================================================
# SECTION 25: LSP INTEGRATION (Language Server Protocol)
# =============================================================================

# lsp_definition(Symbol, FilePath, Line, Column)
Decl lsp_definition(Symbol, FilePath, Line, Column) bound [/string, /string, /number, /number].

# lsp_reference(Symbol, RefFile, RefLine)
Decl lsp_reference(Symbol, RefFile, RefLine) bound [/string, /string, /number].

# lsp_hover(Symbol, Documentation)
Decl lsp_hover(Symbol, Documentation) bound [/string, /string].

# lsp_diagnostic(FilePath, Line, Severity, Message)
Decl lsp_diagnostic(FilePath, Line, Severity, Message) bound [/string, /number, /name, /string].

# =============================================================================
# SECTION 26: DERIVATION TRACE (Glass Box Interface)
# =============================================================================

# derivation_trace(Conclusion, RuleApplied, Premises)
Decl derivation_trace(Conclusion, RuleApplied, Premises) bound [/string, /string, /string].

# proof_tree_node(NodeID, ParentID, Fact, RuleName)
Decl proof_tree_node(NodeID, ParentID, Fact, RuleName) bound [/string, /string, /string, /string].

# =============================================================================
# SECTION 44: SEMANTIC MATCHING (Vector Search Results)
# =============================================================================
# These facts are asserted by the SemanticClassifier after vector search.
# They provide semantic similarity signals to the inference engine.

# semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)
# UserInput: Original user query string
# CanonicalSentence: Matched sentence from intent corpus
# Verb: Associated verb from corpus (name constant like /review)
# Target: Associated target from corpus (string)
# Rank: 1-based position in results (1 = best match)
# Similarity: Cosine similarity * 100 (0-100 scale, integer)

# Derived: suggested verb from semantic matching
# Populated by inference rules when semantic matches exist

# Derived: compound suggestions from multiple semantic matches

# learned_exemplar(Pattern, Verb, Target, Constraint, Confidence)
# Learned user patterns that influence intent classification.
# Decl learned_exemplar imported from schema/learning.mg

# verb_composition(Verb1, Verb2, ComposedAction, Priority)
# Defines valid verb compositions for compound suggestions
# NOTE: Primary declaration is in taxonomy.mg - removed duplicate here
# Decl verb_composition(Verb1, Verb2, ComposedAction, Priority).

# =============================================================================
# SECTION 52: SPARSE RETRIEVAL SCHEMA
# =============================================================================
# General-purpose predicates for keyword-based file discovery.
# Used for large codebases, issue-driven development, and context building.

# -----------------------------------------------------------------------------
# 52.1 Keyword Extraction
# -----------------------------------------------------------------------------

# issue_text(IssueID, Text)
# Raw issue/problem description for issue-driven workflows.
Decl issue_text(IssueID, Text) bound [/string, /string].

# issue_keyword(IssueID, Keyword, Weight)
# Keywords extracted from issue description with importance weights.
# Weight: 1.0 = highest (primary), 0.5 = medium, 0.2 = low
Decl issue_keyword(IssueID, Keyword, Weight) bound [/string, /string, /number].

# keyword_weight(Keyword, Category)
# Category: /primary, /secondary, /tertiary
Decl keyword_weight(Keyword, Category) bound [/string, /name].

# -----------------------------------------------------------------------------
# 52.2 File Candidates
# -----------------------------------------------------------------------------

# keyword_hit(File, Keyword, Count)
# Records how many times a keyword appears in a file.
Decl keyword_hit(File, Keyword, Count) bound [/string, /string, /number].

# candidate_file(File, RelevanceScore)
# Files identified as potentially relevant to the current issue.
Decl candidate_file(File, RelevanceScore) bound [/string, /number].

# file_mentioned(File, IssueID)
# File was explicitly mentioned in the issue description.
Decl file_mentioned(File, IssueID) bound [/string, /string].

# -----------------------------------------------------------------------------
# 52.3 Tiered Context
# -----------------------------------------------------------------------------

# context_tier(File, Tier)
# Tier: /tier1 (mentioned), /tier2 (keyword), /tier3 (import), /tier4 (semantic)
Decl context_tier(File, Tier) bound [/string, /name].

# tiered_context_file(IssueID, File, Tier, Relevance, TokenCount)
# Individual file selected for context with tier and relevance.
Decl tiered_context_file(IssueID, File, Tier, Relevance, TokenCount) bound [/string, /string, /name, /number, /number].

# issue_context(IssueID, TotalFiles, TotalTokens)
# Summary of context built for an issue.
Decl issue_context(IssueID, TotalFiles, TotalTokens) bound [/string, /number, /number].

# -----------------------------------------------------------------------------
# 52.4 Activation Boost
# -----------------------------------------------------------------------------

# activation_boost(File, BoostAmount)
# Additional activation score for issue-related files.
Decl activation_boost(File, BoostAmount) bound [/string, /number].
