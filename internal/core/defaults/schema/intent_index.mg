# Intent Definition Schema - Main Index
# Maps Canonical Sentences to Mangle Actions for intent classification.
#
# ARCHITECTURE:
# - Verbs with ShardType="/none" are answered DIRECTLY by the agent
# - Verbs with ShardType="/reviewer|coder|tester|researcher" spawn that shard
# - Verbs with ShardType="/tool_generator" trigger autopoiesis
# - Verbs with ShardType="/campaign" trigger campaign orchestration
#
# This file contains 400+ canonical sentences covering all codeNERD capabilities.
# The intent definitions are modularized across multiple files for maintainability.

# =============================================================================
# PREDICATE DECLARATIONS
# =============================================================================

Decl intent_definition(Sentence, Verb, Target).
Decl intent_category(Sentence, Category).

# =============================================================================
# MODULARIZED INTENT FILES
# =============================================================================
#
# The intent definitions are split across the following files:
#
# 1. intent_stats.mg             - Section 1: Codebase statistics (/stats)
# 2. intent_conversational.mg    - Sections 2-3: Help & greetings (/help, /greet)
# 3. intent_code_review.mg       - Sections 4-5: Code review & security (/review, /security)
# 4. intent_code_mutations.mg    - Sections 6-10: Fix, debug, refactor, create, delete
# 5. intent_testing.mg           - Section 11: Testing (/test)
# 6. intent_operations.mg        - Sections 12-22: Research, explain, tools, campaigns, git, search, etc.
# 7. intent_multi_step.mg        - Sections 23-24: Multi-step task patterns (/multi_step)
#
# All files should be loaded together by the Mangle engine.
# =============================================================================
