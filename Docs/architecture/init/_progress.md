# init — Corpus rebuild progress

| Date | Action |
|------|--------|
| 2026-08-15 | Backlog pass: wired Type U `--define-agent` and interactive agent curation into phase 6 (terminal-gated); ingested project prompt atoms into `prompts/corpus.db`; made `savePreferences` merge-preserving after finding force-reinit truncated the shared `preferences.json`; populated `ProjectProfile.Framework` from dependencies; replaced two-level manifest globs with a bounded monorepo walk; recorded tool needs as `missing_tool_for` facts instead of a "would be generated" print; fixed `extractJSON` for arrays and for braces inside string values; added hermetic strategic-knowledge tests. |
| 2026-08-09 | F-INIT-2 truth/safety packet: preserved user `.gitignore` and Mangle overlays, closed chat-owned initializer resources, enforced timeout/cancellation, rooted the kernel before load, added required failures and LLM provider/model outcome metrics, relabeled atom-count scores, and added regression/race coverage. |
| 2026-07-13 | Full architecture corpus rebuilt from `internal/init/` source (docs only). Replaced thin auto-inventory stubs with cli-quality narrative set per `Docs/architecture/_rebuild/SUBAGENT_INSTRUCTIONS.md`. |

## Deliverables

- README + IMPLEMENTED_SPEC (flagship)
- 00 alignment through 12 failure modes
- TODO, OPEN-QUESTIONS, this progress file
- Legacy differently-named stubs overwritten with pointers where retained

## Research notes

- 17 non-test Go files; 11 test files; 0 debug `.mg` dumps
- Downstream: `cmd/nerd` init/scan + chat session persistence
- Intentional stubs: analysis phase, tool generation, researcher shard removed
