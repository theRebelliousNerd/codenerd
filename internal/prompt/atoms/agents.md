# internal/prompt/atoms

Central library of **prompt atoms** used by the JIT Prompt Compiler. Atoms are small, composable prompt fragments selected at runtime.

## How to work in this area

- Add new atoms to the correct category folder. Keep atoms granular.
- Prefer stable filenames; other code may reference them by path.
- If an atom needs project‑specific variants, place them under `.nerd/agents/` instead.

## Useful cross‑refs

- `internal/prompt/compiler.go`: atom selection + assembly.
- `internal/context/`: context atoms that pair with prompt atoms.
- `internal/prompt/sync/`: tooling for atom synchronization.

