# internal/prompt/atoms

Central library of **prompt atoms** used by the JIT Prompt Compiler. Atoms are small, composable prompt fragments selected at runtime.

Atoms are the canonical “knowledge and instruction units” for the creative center. Instead of baking rules into shard code or relying on chat history, the compiler pulls the right atoms for the current context and assembles a task‑specific prompt on the fly. This keeps behavior logic‑first: Mangle decides what the model should know, and atoms provide the reusable language to express it.

The subfolders under `atoms/` are semantic namespaces aligned with major systems (mangle, shards, campaign, safety, world_state, etc.). If you’re not sure where a new capability belongs, ask “which executive system would care about this?” and place the atom there. Project‑local or experimental atoms should live under `.nerd/agents/` and get hydrated via `internal/prompt/sync/`.

## How to work in this area

- Add new atoms to the correct category folder. Keep atoms conceptually granular.
- Prefer stable filenames/IDs; other code may reference them by path or ID.
- If an atom needs project-specific variants, place them under `.nerd/agents/` instead.
- Atoms may be **long and encyclopedic** as long as they remain narrowly targeted to one concept.
- Use selectors aggressively (`shard_types`, `intent_verbs`, `languages`, `frameworks`, `world_states`, campaign phases, etc.) so long atoms only appear when relevant.
- For alternate “takes” on the same concept, use `conflicts_with` or `is_exclusive` to avoid co-selection.
- Add `content_concise` / `content_min` variants for very large atoms to allow budget downshifting.

## Useful cross‑refs

- `internal/prompt/compiler.go`: atom selection + assembly.
- `internal/context/`: context atoms that pair with prompt atoms.
- `internal/prompt/sync/`: tooling for atom synchronization.
