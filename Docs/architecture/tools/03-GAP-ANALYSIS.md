# tools — Gap Analysis

## Accepted existing-seed SQLite profile correction (2026-10-02)

**PARTIAL — GAP-TOOLS-SEED-PRAGMAS:** dependent audit `artifact:.corpus-build/runs/all-features-20261002/round6-affected-full.receipt.json` finds the new `cmd/tools/prompt_builder/main.go#reconcilePromptSeed` SQL open at line219 lacks a centralized pragma profile. Complete embedding/perception/system/shards-system/CLI packages pass in that aggregate gate, but SQL profile inventory fails. The second site in unchanged `internal/init/phase_ecosystem.go` is separately preexisting and not owned by this correction.

**PROPOSED UPLIFT:** after validating the existing target and before schema/reconciliation mutation, apply the centralized transactional existing-store profile. Do not reuse the destructive fresh bulk-build profile, tune an invalid target before validation, exempt the new site or weaken the inventory. Exit: independently observe the valid corpus profile; preserve exact vectors/task/project ownership, cancellation/rollback and invalid targets; complete builder tests pass; inventory no longer reports reconcilePromptSeed. Remaining baseline inventory failure stays visible.

## Accepted prompt seed maintenance contract (2026-10-02)

**PARTIAL — GAP-TOOLS-PROMPT-SEED-01:** `cmd/tools/prompt_builder/main.go#main` (`cmd/tools/prompt_builder/main.go:109`) now has an authored `-reconcile-embedded` branch; the root builder gate remains red because a valid-corpus fixture uses an illegal Windows filename. This helper belongs to the existing `tools` source roots, not `cli`. The initial operator target was accepted under GAP-CLI-PROMPT-SEED-01; implementation ownership is reconciled here before further source work. July claims below are historical, not new qualification.

**PROPOSED UPLIFT:** explicit maintenance of an existing valid prompt corpus uses compiled canonical atoms and the production prompt schema/reconciler API without provider credentials, deletion/truncation/replacement or silently ignored custom input. Default generation remains unchanged. Report actual canonical upsert/delete and retained/cleared counts; preserve exact vector/task values for unchanged effective input and invalidate changed input. Missing, invalid or directory targets are refused without replacement. Project-owned rows and their tags/vector rows survive reconciliation under GAP-PROMPT-RECONCILE-PRESERVATION.

Exit evidence is a root-run real SQLite builder gate covering legal Windows URI-sensitive paths, exact metadata/ownership, rollback and credential-free main; a guarded sqlite_vec build; and an owned candidate command receipt with integrity/content parity and per-atom retention checks. Those bounded maintenance controls now pass in `round5-reconciler-builder.receipt.json`; `round5-seed-retention.json` records 682 exact retained vectors and 29 changed-input invalidations across 923 rows before generated-asset publication. The first-boot prompt database carries the corrected perception row, but normal lifecycle remains red. Claiming semantic ANN parity additionally requires qualified vector namespace/task/index behavior; mixed legacy dimensions and unknown provenance are not made compatible by preserving bytes. See [operator contract and current receipts](../cli/07-PROMPT-SEED-MAINTENANCE.md). GAP-TOOLS-PROMPT-SEED-01 remains partial, not a global feature-completion claim.

## Accepted ordered corpus oracle maintenance

**PARTIAL — GAP-TOOLS-ATOM-GOLDEN-01:** the full root gate in `round5-prompt-perception-full.receipt.json` passes prompt/sync/perception/builder but fails `cmd/tools/validate_prompt_atoms/corpus_parity_test.go#TestCheckedInCorpusOrderedParity` (`cmd/tools/validate_prompt_atoms/corpus_parity_test.go:76`): its 917-ID golden predates six already-committed canonical atoms. Root independently reproduced the old 917-ID digest at 8c477797, then derived the current 923-ID digest and exact six additions with no removals in `round5-validator-golden-delta.json`. Added IDs are campaign/recurse/fix, campaign/recurse/improve and northstar/derive/classify, identity, requirements, vision. These are existing corpus additions, not newly introduced to satisfy the test.

**PROPOSED UPLIFT:** refresh only the expected count/digest to the verified canonical corpus and retire the misleading inline count narrative. Preserve strict validation, no-migration assertions, independent filesystem/embedded route equality and ordered identity hashing. Do not derive the expected value from the runtime under test, skip entries, weaken assertions or change source atoms. Exit: the full validator package passes with the exact 923-ID oracle and the above verified provenance.

> Last verified: **2026-08-15**

## Spec vs reality matrix

| Desired property | Reality | Severity | Priority |
|------------------|---------|----------|----------|
| All path tools contained in workspace | **Closed** — file ops, search, codedom | High | Done |
| Shell cannot cwd outside workspace | **Closed** — `shell.resolveWorkingDir` | High | Done |
| Default deny for tools | **Closed in tools** — `Registry.SetAllowlist`; session wiring pending | High | Wiring |
| Mangle lists every registered tool | **Closed** — golden test in both directions | Medium | Done |
| Single allowlist source | Config + soft FilterByIntent + Mangle | Medium | P1 |
| Categories have tools | **Closed** — `Tool.AltCategories` | Low | Done |
| codedom doc matches register | **Closed** — golden test pins doc lists | Low | Done |
| Workspace root not env-global | **Closed in tools** — `Registry.SetWorkspaceRoot` → ctx; factory wiring pending | Medium | Wiring |
| Tool results always feed multi-turn | Piggyback single pass; non-TRP clients | Medium | P1 (session) |
| Search uses coerceInt for max_results | **Closed** — one shared `tools.CoerceInt` | Medium | Done |
| Full AST codedom | Regex only | Low (by design) | — |
| Persistent research cache | **Closed** — `.nerd/cache/research`; boot wiring pending | Low | Wiring |
| Metrics on tool success rates | **Closed** — `Registry.AllMetrics()` | Low | Done |

## Non-gaps (do not “fix”)

| Observation | Why not a gap |
|-------------|---------------|
| No local `.mg` in package | Correct — policy lives in core/mangle corpora |
| Dual VS + Global hydrate | Intentional for session vs VS consumers |
| GroundingHelper not a Tool | Correct library design |
| Ouroboros separate registry | Different lifecycle (compiled binaries) |
| delete_file refuses directories | Intentional safety |

## Priority backlog (compressed)

Items 1-12 below are closed in `internal/tools`. What remains is **wiring**:
each mechanism exists and is tested, but three of them have no caller yet, and
a mechanism with no caller enforces nothing.

### Remaining — wiring, owned outside this package

- `internal/session`: call `tools.Registry.SetAllowlist(&tools.Allowlist{Enforced: cfg.EnableSafetyGate, Names: cfg.AllowedTools})` when the effective config changes, so the registry-level envelope binds to the JIT config.
- `internal/system/factory.go`: call `tools.SetGlobalWorkspaceRoot(abs)` beside the existing `os.Setenv("CODENERD_WORKSPACE_ROOT", abs)`, so containment stops depending on process-global state.
- `internal/core`: install a `tools.FactSink` that asserts `tool_execution(ToolName, Success, Timestamp)`, and call `research.EnableDiskCache(workspaceRoot)` at boot.
- `internal/core/virtual_store_tools.go`: `HydrateModularTools` still registers every family into two registries; collapsing that removes the last dual-map drift risk.

### Closed

1. `resolveWorkspacePath` applied to glob/grep/codedom path args.  
2. shell and git `working_dir` (and git pathspec) contained to the workspace.  
3. Registry fails closed on an enforced-but-empty allowlist.  
4. `intent_routing.mg` synced with the full RegisterAll catalog, pinned by a golden test.  
5. One shared `tools.CoerceInt` for every numeric tool argument.  
6. Workspace root threaded via registry -> context; env demoted to a fallback.  
7. `Tool.AltCategories` so `/review` and `/attack` resolve to real tools.  
8. `core/doc.go` and `shell/doc.go` realigned, pinned by a golden test.  
9. `logging.Tools*` used throughout the package.  
10. Disk-backed research cache under `.nerd/cache/research`.  
11. `Registry.SetFactSink` emits one record per completed execution.  
12. `Registry.Metrics` / `AllMetrics` track calls, successes, failures, durations.

## Gap ownership

Many “tool safety” gaps are **session/core contracts**, not pure tools bugs. Fixes may land in:

- `internal/tools/core` (containment)  
- `internal/session` (allowlist closed default)  
- `internal/mangle` (catalog)  
- `internal/jit/config` (AllowedTools population)
