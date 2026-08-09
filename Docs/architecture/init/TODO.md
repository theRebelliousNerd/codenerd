# init — TODO

> Last verified: 2026-08-09
> Docs-only backlog; no code claims of completion.

## P0

- [ ] Document operator embedding prerequisites next to CLI `nerd init` help text (code change — track here).
- [ ] Confirm force-reinit never deletes `preferences.json` without explicit wipe path.

## P1

- [ ] Wire `InteractiveAgentSelection` when `InitConfig.Interactive` and TTY available.
- [ ] Wire `--define-agent` / Type U into CLI `runInit` merge path.
- [ ] Attach `ProgressChan` from chat `/init` if slash init exists.
- [ ] Ingest `populateProjectAtoms` into `prompts/corpus.db` for JIT visibility.

## P2

- [ ] Improve framework detection (populate `ProjectProfile.Framework` from deps).
- [x] Label legacy KB quality fields in operator UX as atom-count population proxies (semantic replacement remains optional future work).
- [ ] Monorepo multi-root profiles (beyond 2-level globs).
- [ ] Hermetic tests for strategic knowledge JSON parsing with fake LLM.

## P3

- [ ] Split `Initialize` into phase methods without behavior change.
- [ ] Relocate session persistence types to `internal/session` (breaking API care).
- [ ] Remove accidental `debug_program_ERROR.mg` from package tree / ignore dumps.
- [ ] Complete Ouroboros tool generation call site or delete dead `determineRequiredTools` UI noise.

## Done (relative to older stubs)

- [x] Living architecture corpus rebuilt 2026-07-13 (this directory).
- [x] Shared KB + Type-3 parallel creation documented as implemented.
- [x] Researcher shard removal documented as intentional.
