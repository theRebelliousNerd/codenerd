# Corpus-build maintenance

The tracked `.agents/skills/corpus-build` package is the durable source for
project-local corpus-build behavior. Mirror intentional runtime changes into
`.codex/skills/corpus-build` after tests pass; do not overwrite unrelated local
Codex state.

- Keep architecture validation deterministic and read-only in `--check` mode.
- `--verify` may run only fixed, source-owned profiles with hard timeouts. Never
  execute commands parsed from prose, feature cards, or architecture documents.
- Treat structural validation, semantic corpus review, and executable receipts as
  separate evidence lanes.
- Add positive and negative regression cases for every new hard rule.
- Preserve subset validation for focused corpus packets and full portfolio parity
  for default discovery.
- Update `Docs/architecture/_rebuild/SUPERSTAR_CORPUS_STANDARD.md` when a schema or
  evidence contract changes materially.
