# internal/core/defaults/schema - Intent Classification Schema

Mangle schema files defining the intent classification corpus for user input transduction. Maps natural language sentences to verbs, targets, and categories for shard routing.

## File Index

| File | Description |
|------|-------------|
| `agents.md` | Markdown reference documenting agent/shard definitions and their capabilities. Describes the different shard types (coder, tester, reviewer, researcher, nemesis) and their routing patterns for intent-to-shard mapping. |
| `intent.mg` | Legacy monolithic intent definition file (deprecated). Contains the original 400+ canonical sentence mappings before modularization. Retained for reference; prefer the modularized files for active development. |
| `intent_campaign.mg` | Intent definitions for campaign orchestration (multi-phase, long-running tasks). Defines sentences like "Start a campaign to rewrite auth" that trigger the campaign system for complex multi-step goals. |
| `intent_code_mutations.mg` | Intent definitions for code mutation operations (Sections 6-10). Covers fix, debug, refactor, create, and delete verbs with canonical sentences mapped to coder shard actions. |
| `intent_code_review.mg` | Intent definitions for code review and security analysis (Sections 4-5). Maps review and security-related sentences to the reviewer shard for code quality and vulnerability assessment. |
| `intent_conversational.mg` | Intent definitions for conversational interactions (Sections 2-3). Covers help requests, greetings, and meta-questions about the agent itself that require direct responses without shard spawning. |
| `intent_core.mg` | Core intent predicate declarations and fundamental definitions. Declares `intent_definition/3`, `intent_category/2`, and `verb_routing/4` predicates used across all intent files. |
| `intent_index.mg` | Main index file documenting the modularized intent architecture. Contains predicate declarations and documents which sections appear in which files. All intent files should be loaded together by the Mangle engine. |
| `intent_instructions.mg` | Intent definitions for instruction/preference commands. Maps configuration and preference-setting sentences to the appropriate system handlers for agent customization. |
| `intent_multi_step.mg` | Intent definitions for multi-step task patterns (Sections 23-24). Contains 200+ patterns for detecting compound requests like "first X, then Y, finally Z" with sequential, parallel, and conditional relationships. |
| `intent_mutations.mg` | Supplementary mutation intent patterns. Extends intent_code_mutations.mg with additional sentence variations for file creation, deletion, and modification operations. |
| `intent_operations.mg` | Intent definitions for system operations (Sections 12-22). Covers research, explain, tool generation, campaigns, git, search, explore, configure, knowledge, shadow mode, and miscellaneous operations. |
| `intent_qualifiers.mg` | Grammatical qualifier taxonomy for enhanced intent classification. Defines interrogative types (what/why/how/where), modal verbs (can/should/would), copular patterns, negation markers, and state adjectives. |
| `intent_queries.mg` | Intent definitions for query/read-only operations. Maps information-seeking sentences that don't modify code to appropriate response handlers. |
| `intent_stats.mg` | Intent definitions for codebase statistics queries (Section 1). Maps sentences like "How many files?" and "What is the project structure?" to direct shell command responses without shard spawning. |
| `intent_system.mg` | Intent definitions for system-level operations. Covers agent self-management, session control, and internal system commands. |
| `intent_testing.mg` | Intent definitions for testing operations (Section 11). Maps test-related sentences ("Run tests", "Generate unit tests", "Check coverage") to the tester shard for test execution and generation. |
| `learning.mg` | Schema definitions for the learning subsystem. Declares predicates for storing and retrieving learned patterns, preferences, and rejection/acceptance signals from the autopoiesis system. |
| `prompts.mg` | Schema definitions for the JIT prompt compilation system. Declares predicates for prompt atoms, context selectors, and compilation metadata used by the prompt compiler. |
| `split_intent.py` | Python utility script for splitting the monolithic intent.mg into modular files. Used during the initial modularization effort; retained for future maintenance or re-modularization tasks. |

## Architecture

The intent schema implements a three-layer classification system:

1. **Sentence → Verb/Target**: `intent_definition/3` maps canonical sentences to action verbs and targets
2. **Sentence → Category**: `intent_category/2` classifies as `/query`, `/mutation`, or `/instruction`
3. **Verb → Shard**: `verb_routing/4` in policy.mg routes verbs to appropriate shards

## Loading Order

All `.mg` files in this directory should be loaded together. The recommended order:
1. `intent_core.mg` (predicate declarations)
2. `intent_qualifiers.mg` (grammatical patterns)
3. Domain-specific files (stats, conversational, code_review, etc.)
4. `intent_multi_step.mg` (compound pattern detection)

---

**Remember: Push to GitHub regularly!**
