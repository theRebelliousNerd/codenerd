# init — TODO

> Last verified: 2026-08-15
> Docs-only backlog; no code claims of completion.

## P0

- [x] Document operator embedding prerequisites next to CLI `nerd init` help text
      (`initCmd.Long` now states the `.nerd/config.json` "embedding" block, the
      ollama/genai providers, the CGO requirement and the hard-fail contract;
      pinned by `TestInitCmdHelp_WhenRead_ShouldStateEmbeddingPrerequisites`).
- [x] Confirm force-reinit never deletes `preferences.json` without explicit wipe path.
      **The audit found the opposite**: phase 8 marshalled a freshly defaulted
      `UserPreferences` over the whole file, destroying the ux v2.0 blocks, the
      learned intent corrections and the `agent_selection` history that phase 6
      had just written. `savePreferences` is now merge-preserving and returns the
      effective values; guarded by `preferences_preservation_test.go`.

## P1

- [x] Wire `InteractiveAgentSelection` when `InitConfig.Interactive` and TTY available
      (`curateAgents`, phase 6; terminal probe requires stdin *and* stdout to be
      character devices; `--no-interactive` opts out).
- [x] Wire `--define-agent` / Type U into CLI `runInit` merge path
      (`InitConfig.TypeUAgents` → `mergeTypeUAgents`, before KB creation).
- [x] Attach `ProgressChan` from chat `/init` — verified already implemented in
      `cmd/nerd/chat/helpers_scan.go` (`runInitialization` creates the channel,
      forwards each `InitProgress.Message` to the status bar, and closes it after
      `Initialize` returns).
- [x] Ingest `populateProjectAtoms` into `prompts/corpus.db` for JIT visibility
      (phase 5c `ingestProjectAtomsIntoCorpus`, after reconciliation, with a NULL
      `source_file` so the reconciler treats the rows as project-owned).

## P2

- [x] Improve framework detection (populate `ProjectProfile.Framework` from deps)
      (`detectFrameworkFromDependencies`, ranked so meta-frameworks outrank the
      view libraries they wrap and direct deps outrank transitive ones).
- [x] Label legacy KB quality fields in operator UX as atom-count population proxies (semantic replacement remains optional future work).
- [ ] Monorepo multi-root profiles. **Partially done**: manifest discovery is now
      a bounded walk (`findManifestFiles`, depth 4, vendor/node_modules skipped)
      instead of two hardcoded glob levels, so nested modules contribute their
      dependencies. Still open: emitting one `ProjectProfile` *per module* rather
      than one merged dependency set, and per-module entry points.
- [x] Hermetic tests for strategic knowledge JSON parsing with fake LLM
      (`strategic_knowledge_parsing_test.go`; found and fixed two real parse bugs
      — unfenced JSON *arrays* truncated to their first object, and a `}` inside
      a string value closing the object early).

## P3

- [x] Split `Initialize` into phase methods without behavior change — verified
      already done: `runPhase0Migrations` … `runPhase12PromptSync` plus
      `finalizeInitialization`, driven by `phaseRunner`.
- [ ] Relocate session persistence types to `internal/session` (breaking API care).
      `SessionState`, `ChatMessage` and `SessionHistory` still live in
      `internal/init`; consumers are `cmd/nerd/chat/session_persistence.go` and
      its tests.
- [x] Remove accidental `debug_program_ERROR.mg` from package tree / ignore dumps
      — verified already resolved: kernel fault dumps are written to
      `.nerd/debug/` (`kernel_eval.go`) and `.gitignore` line 132 ignores
      `debug_program_ERROR*.mg`. `TestPackageTree_WhenScanned_ShouldContainNoMangleDebugDumps`
      keeps the package tree clean.
- [x] Complete Ouroboros tool generation call site or delete dead `determineRequiredTools` UI noise.
      **Decided: neither.** `determineRequiredTools` is a real measurement, but
      init must not write/compile/register LLM-authored binaries during a cold
      start (and it holds the cheap worker client). `generateFactsFile` now emits
      `missing_tool_for(/project_init, /capability)` — the already-Declared
      predicate autopoiesis and campaign use — so the kernel decides and
      generation stays behind `ExecuteOuroborosLoop`. Also answers
      OPEN-QUESTIONS #8.

## Done (relative to older stubs)

- [x] Living architecture corpus rebuilt 2026-07-13 (this directory).
- [x] Shared KB + Type-3 parallel creation documented as implemented.
- [x] Researcher shard removal documented as intentional.
