# init — Corpus rebuild progress

| Date | Action |
|------|--------|
| 2026-07-13 | Full architecture corpus rebuilt from `internal/init/` source (docs only). Replaced thin auto-inventory stubs with cli-quality narrative set per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. |

## Deliverables

- README + IMPLEMENTED_SPEC (flagship)
- 00 alignment through 12 failure modes
- TODO, OPEN-QUESTIONS, this progress file
- Legacy differently-named stubs overwritten with pointers where retained

## Research notes

- 16 non-test Go files; 7 test files; 1 debug `.mg` dump
- Downstream: `cmd/nerd` init/scan + chat session persistence
- Intentional stubs: analysis phase, tool generation, researcher shard removed
