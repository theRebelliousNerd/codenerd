# init — Open Questions

> Last verified: 2026-08-15

## Product

1. ~~**Should interactive agent curation be default or opt-in?**~~ **Answered
   2026-08-15: default on, gated on a real terminal, `--no-interactive` opts
   out.** A cold start that silently installs a dozen specialists the user never
   saw is the worse default; the terminal gate keeps CI and the chat `/init`
   path (which sets `Interactive: false`) unaffected. Implemented in
   `curateAgents`.

2. ~~**Are Type U agents still a first-class init feature?**~~ **Answered
   2026-08-15: wired.** `nerd init --define-agent Name:role:topics` parses into
   `InitConfig.TypeUAgents` and merges before knowledge-base creation, so a Type
   U agent gets the same KB, `prompts.yaml`, registry entry and shard
   registration as a detected one. A name collision replaces the built-in,
   because both resolve to the same `.nerd/shards/{name}_knowledge.db`.

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
   Resolved for visibility (phase 5c ingests into `corpus.db`), but the atoms
   are still written to both `knowledge.db` and `corpus.db`. Is the LocalStore
   copy still earning its place, or should `knowledge.db` stop carrying prompt
   atoms entirely?

8. ~~**Tool generation**~~ **Answered 2026-08-15: purely on-demand.** Init
   measures and records `missing_tool_for(/project_init, /capability)` facts;
   the kernel decides when to act and Ouroboros builds through
   `ExecuteOuroborosLoop`. Init adds no path to `ToolGenerator`. Remaining
   question: should a policy rule derive `capability_gap_detected` from these
   `/project_init` needs, or must a session failure still be the trigger?

## Operations

9. **Default config provider (Gemini 3.5 Flash)**  
   Is this still the correct global default for all users of `createDefaultConfig`?

10. **Embedding hard-fail**  
    Should init degrade to non-vector KBs when embeddings unavailable, or keep hard-fail?

## Process

11. ~~**`debug_program_ERROR.mg` in package**~~ **Answered: dumps go to
    `.nerd/debug/` and are gitignored.** No `.mg` remains in the package tree
    and a test keeps it that way.

12. **Progress channel consumers**  
    Chat `/init` consumes `InitProgress` (status bar, `helpers_scan.go`). The
    CLI `runInit` still attaches no channel and prints phases directly — should
    the CLI grow a progress renderer, or is stdout the intended CLI surface?
