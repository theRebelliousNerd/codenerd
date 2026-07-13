# Package Boundary and Write Discipline (codeNERD)

Vectryx "subgraph isolation" maps to codeNERD package and fact-space discipline.

## Rules

1. **Own your package.** Prefer writes under the feature's declared `source_paths[]`.
2. **Cross-package edits need contract.** Shared types/signatures live in the pinned
   `.corpus-build/contracts/<subsystem>.md` or an existing public interface package.
3. **Reserved hubs use intents.** Do not race-edit registration files; write intents.
4. **Mangle facts are scoped.** New predicates need Decl; avoid polluting global policy
   with feature-private rules — prefer feature `.mg` modules loaded deliberately.
5. **No silent world-model pollution.** World/scanner/store writes must be intentional
   and gated by policy when user-visible.
6. **Read before write** for persistent stores (SQLite knowledge DBs, learned corpus):
   check existing rows/keys before insert to avoid duplicate or clobbering facts.

## Anti-patterns

- Reaching into another package's unexported state
- Adding prompt prose to shard files instead of atoms
- Asserting Mangle facts without schema Decl
- Claiming VirtualStore route exists without registration evidence
