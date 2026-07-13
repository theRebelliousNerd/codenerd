# init — Open Questions

> Last verified: 2026-07-13

## Product

1. **Should interactive agent curation be default or opt-in?**  
   `DefaultInitConfig` sets `Interactive: true`, but CLI does not prompt. Which is the product truth?

2. **Are Type U agents still a first-class init feature?**  
   Full parse/validate suite exists; CLI merge path is unclear. Keep, wire, or demote to docs-only?

3. **What is the guaranteed minimum offline init?**  
   Without LLM/embedding/Context7, what must still succeed for `nerd chat` to be useful?

4. **Should `nerd scan` rebuild profile.mg from detectors?**  
   Today it reloads existing profile.mg; detectors live only in full init.

## Architecture

5. **Session types ownership**  
   `SessionState` / history live in `init` but are chat runtime concerns. Move to `session` package?

6. **Strategic knowledge vs northstar**  
   Both claim “vision.” How do `StrategicKnowledge` atoms and `northstar` store avoid dual sources of truth?

7. **Project atoms dual storage**  
   Comment in `populateProjectAtoms` admits corpus.db vs knowledge.db split — intended long-term design?

8. **Tool generation**  
   When Ouroboros/VirtualStore should be invoked from init vs purely on-demand during sessions?

## Operations

9. **Default config provider (Gemini 3.5 Flash)**  
   Is this still the correct global default for all users of `createDefaultConfig`?

10. **Embedding hard-fail**  
    Should init degrade to non-vector KBs when embeddings unavailable, or keep hard-fail?

## Process

11. **`debug_program_ERROR.mg` in package**  
    Keep for local debugging, gitignore, or delete from tree?

12. **Progress channel consumers**  
    Which TUI surfaces still consume `InitProgress` after chat refactors?
