# Preliminary legacy migration ledger

**Evidence date:** 2026-07-13
**Scope:** every top-level `Docs/architecture/*/*.md` filename matching the validator's `LEGACY_PATTERNS`
**Policy:** non-destructive classification only; this ledger authorizes no deletion

## Result

The scan accounts for **261/261** matching files across all 38 realized corpora. Content comparison found **259 redirect-only stubs** and **2 intentional CLI canonical variants**. No matching file contains unique substantive architecture evidence that requires harvest, and no non-CLI match has an inbound Markdown link.

| Classification | Count | Meaning |
|---|---:|---|
| `keep` | 2 | Canonical CLI variants named by the CLI manifest and linked from canonical CLI docs. |
| `revise` | 0 | No matched file is an incomplete canonical authority. |
| `harvest` | 0 | No unique claim survived content comparison; all non-CLI content is supersession/navigation prose. |
| `compat` | 0 | Repo-wide Markdown resolution found no inbound link to a non-CLI match. |
| `remove` | 259 | Redirect-only stubs eligible only after gate D1. |

## Comparison and gate definitions

- **R0 — redirect-only:** 2–13 lines, 11–55 words, at least one outbound canonical link, and no source/runtime/spec/test claim beyond supersession and navigation. The semantic destination is normalized below even when a stub links only to its README.
- **K1 — canonical CLI variant:** the filename is part of `CLI_CANONICAL`, is substantive, and has resolved inbound links from the CLI README or implemented spec.
- **D1 — deletion gate:** re-read and destination-diff the exact `sha12` identity; confirm every normalized destination exists and retains the intended responsibility; rescan links, anchors, citations, and full-path consumers; reconcile stale canonical prose that says the stub exists; then require changed-corpus semantic review plus strict validator success. Any content drift, unique claim, or inbound compatibility need changes `remove` to `harvest`, `revise`, or `compat` before deletion.
- **N-A:** keep entries have no deletion gate.

The inbound-link check resolved Markdown links across **1,460 repo Markdown files**. It found three links total, all to the two kept CLI variants. Plain historical filename mentions remain in some README, TODO, and progress files; D1 deliberately requires reconciling those during cleanup even though they are not navigable compatibility dependencies.

## Complete file ledger

| Path | Class | Canonical destination | Shape | Inbound | sha12 | Reason | Gate |
|---|---|---|---:|---:|---|---|---|
| `Docs/architecture/articulation/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/38W/3 links | 0 | `bf79feae822e` | R0 | D1 |
| `Docs/architecture/articulation/02-CURRENT-STATE-ARTICULATION.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `50ac2a73a725` | R0 | D1 |
| `Docs/architecture/articulation/03-GAP-ANALYSIS-ARTICULATION.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `34ad942e94ba` | R0 | D1 |
| `Docs/architecture/articulation/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/26W/2 links | 0 | `0e06b50eaffe` | R0 | D1 |
| `Docs/architecture/articulation/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/17W/1 links | 0 | `e9e22132d8fa` | R0 | D1 |
| `Docs/architecture/articulation/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `66f9b4ede2c1` | R0 | D1 |
| `Docs/architecture/articulation/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/15W/1 links | 0 | `1f86e43fc577` | R0 | D1 |
| `Docs/architecture/autopoiesis/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 4L/25W/3 links | 0 | `dca6da5cfbd6` | R0 | D1 |
| `Docs/architecture/autopoiesis/02-CURRENT-STATE-AUTOPOIESIS.md` | `remove` | 02-CURRENT-STATE.md | 4L/14W/1 links | 0 | `6e45253df6a0` | R0 | D1 |
| `Docs/architecture/autopoiesis/03-GAP-ANALYSIS-AUTOPOIESIS.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/14W/1 links | 0 | `e76737faab4a` | R0 | D1 |
| `Docs/architecture/autopoiesis/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/25W/2 links | 0 | `e5b2658aec78` | R0 | D1 |
| `Docs/architecture/autopoiesis/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/25W/2 links | 0 | `bf9823890de2` | R0 | D1 |
| `Docs/architecture/autopoiesis/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/14W/1 links | 0 | `cbfa8cbaccff` | R0 | D1 |
| `Docs/architecture/autopoiesis/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/14W/1 links | 0 | `87d2e43244d9` | R0 | D1 |
| `Docs/architecture/browser/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 9L/46W/4 links | 0 | `64aa9baa0895` | R0 | D1 |
| `Docs/architecture/browser/02-CURRENT-STATE-BROWSER.md` | `remove` | 02-CURRENT-STATE.md | 4L/11W/1 links | 0 | `f4d87d7c2d4c` | R0 | D1 |
| `Docs/architecture/browser/03-GAP-ANALYSIS-BROWSER.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/11W/1 links | 0 | `09f62681dda4` | R0 | D1 |
| `Docs/architecture/browser/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 5L/21W/2 links | 0 | `669a92ad5593` | R0 | D1 |
| `Docs/architecture/browser/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/13W/1 links | 0 | `693ced330b0b` | R0 | D1 |
| `Docs/architecture/browser/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/11W/1 links | 0 | `579416716516` | R0 | D1 |
| `Docs/architecture/browser/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/12W/1 links | 0 | `f8163535d9b5` | R0 | D1 |
| `Docs/architecture/build/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 2L/24W/2 links | 0 | `5c343b2b5d0c` | R0 | D1 |
| `Docs/architecture/build/02-CURRENT-STATE-BUILD.md` | `remove` | 02-CURRENT-STATE.md | 2L/13W/1 links | 0 | `5020db714c10` | R0 | D1 |
| `Docs/architecture/build/03-GAP-ANALYSIS-BUILD.md` | `remove` | 03-GAP-ANALYSIS.md | 2L/13W/1 links | 0 | `daefcabf19c8` | R0 | D1 |
| `Docs/architecture/build/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 2L/24W/2 links | 0 | `f3a6f402fc2a` | R0 | D1 |
| `Docs/architecture/build/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 2L/15W/1 links | 0 | `e5bf66da140d` | R0 | D1 |
| `Docs/architecture/build/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 2L/13W/1 links | 0 | `5a77215bef84` | R0 | D1 |
| `Docs/architecture/build/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 2L/26W/2 links | 0 | `e3525bc1c148` | R0 | D1 |
| `Docs/architecture/campaign/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/30W/3 links | 0 | `d244bdb033c3` | R0 | D1 |
| `Docs/architecture/campaign/02-CURRENT-STATE-CAMPAIGN.md` | `remove` | 02-CURRENT-STATE.md | 6L/20W/1 links | 0 | `5b180a555ff4` | R0 | D1 |
| `Docs/architecture/campaign/03-GAP-ANALYSIS-CAMPAIGN.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/20W/1 links | 0 | `a3909d1260c8` | R0 | D1 |
| `Docs/architecture/campaign/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 9L/30W/2 links | 0 | `d6078b2dc08f` | R0 | D1 |
| `Docs/architecture/campaign/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/22W/1 links | 0 | `b17ad1281640` | R0 | D1 |
| `Docs/architecture/campaign/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/20W/1 links | 0 | `afb89fde4667` | R0 | D1 |
| `Docs/architecture/campaign/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 8L/34W/2 links | 0 | `05a3d0d08237` | R0 | D1 |
| `Docs/architecture/cli/02-CURRENT-STATE-CLI.md` | `keep` | self (canonical CLI variant) | 150L/839W/2 links | 1 | `a40a4ec94c0b` | K1 | N-A |
| `Docs/architecture/cli/03-GAP-ANALYSIS-CLI.md` | `keep` | self (canonical CLI variant) | 58L/362W/0 links | 2 | `0229426aaa26` | K1 | N-A |
| `Docs/architecture/config/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 4L/24W/2 links | 0 | `ce01b8649279` | R0 | D1 |
| `Docs/architecture/config/02-CURRENT-STATE-CONFIG.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `06ebc9d82572` | R0 | D1 |
| `Docs/architecture/config/03-GAP-ANALYSIS-CONFIG.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `132fe40a3133` | R0 | D1 |
| `Docs/architecture/config/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/26W/2 links | 0 | `d7b546365b8e` | R0 | D1 |
| `Docs/architecture/config/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/26W/2 links | 0 | `e636dfb79494` | R0 | D1 |
| `Docs/architecture/config/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `7a2dd51d1952` | R0 | D1 |
| `Docs/architecture/config/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/15W/1 links | 0 | `0254b1f5186a` | R0 | D1 |
| `Docs/architecture/context/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 6L/24W/2 links | 0 | `b67cc442c454` | R0 | D1 |
| `Docs/architecture/context/02-CURRENT-STATE-CONTEXT.md` | `remove` | 02-CURRENT-STATE.md | 6L/21W/1 links | 0 | `a4ff357cf651` | R0 | D1 |
| `Docs/architecture/context/03-GAP-ANALYSIS-CONTEXT.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/21W/1 links | 0 | `b0797fea86b5` | R0 | D1 |
| `Docs/architecture/context/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/32W/2 links | 0 | `6cea204763ef` | R0 | D1 |
| `Docs/architecture/context/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/32W/2 links | 0 | `f59b09085184` | R0 | D1 |
| `Docs/architecture/context/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/21W/1 links | 0 | `b28f416e8a2c` | R0 | D1 |
| `Docs/architecture/context/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/21W/1 links | 0 | `d6825200b740` | R0 | D1 |
| `Docs/architecture/core/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/30W/3 links | 0 | `ef42a04ff3d9` | R0 | D1 |
| `Docs/architecture/core/02-CURRENT-STATE-CORE.md` | `remove` | 02-CURRENT-STATE.md | 6L/16W/1 links | 0 | `6dc603d65445` | R0 | D1 |
| `Docs/architecture/core/03-GAP-ANALYSIS-CORE.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/16W/1 links | 0 | `f2ab0def3432` | R0 | D1 |
| `Docs/architecture/core/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 7L/26W/2 links | 0 | `adc0fc9f1495` | R0 | D1 |
| `Docs/architecture/core/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 7L/26W/2 links | 0 | `f602084c5b86` | R0 | D1 |
| `Docs/architecture/core/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/16W/1 links | 0 | `0829f7e670fc` | R0 | D1 |
| `Docs/architecture/core/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/16W/1 links | 0 | `4173ac448f4b` | R0 | D1 |
| `Docs/architecture/diff/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 5L/31W/3 links | 0 | `67bd387f829f` | R0 | D1 |
| `Docs/architecture/diff/02-CURRENT-STATE-DIFF.md` | `remove` | 02-CURRENT-STATE.md | 5L/20W/2 links | 0 | `2d427087dbef` | R0 | D1 |
| `Docs/architecture/diff/03-GAP-ANALYSIS-DIFF.md` | `remove` | 03-GAP-ANALYSIS.md | 5L/20W/2 links | 0 | `ac4c1275bb9c` | R0 | D1 |
| `Docs/architecture/diff/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 5L/31W/3 links | 0 | `50a8fdd04cff` | R0 | D1 |
| `Docs/architecture/diff/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 5L/22W/2 links | 0 | `47dd5913372c` | R0 | D1 |
| `Docs/architecture/diff/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 5L/20W/2 links | 0 | `1eaf25e454df` | R0 | D1 |
| `Docs/architecture/diff/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/34W/2 links | 0 | `f7bf98cca4b4` | R0 | D1 |
| `Docs/architecture/embedding/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 9L/43W/4 links | 0 | `8300181dc4be` | R0 | D1 |
| `Docs/architecture/embedding/02-CURRENT-STATE-EMBEDDING.md` | `remove` | 02-CURRENT-STATE.md | 5L/18W/2 links | 0 | `360fb9904fa5` | R0 | D1 |
| `Docs/architecture/embedding/03-GAP-ANALYSIS-EMBEDDING.md` | `remove` | 03-GAP-ANALYSIS.md | 5L/18W/2 links | 0 | `105d019be5ce` | R0 | D1 |
| `Docs/architecture/embedding/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 9L/31W/3 links | 0 | `075c4eb54435` | R0 | D1 |
| `Docs/architecture/embedding/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/29W/3 links | 0 | `f2066d320ee8` | R0 | D1 |
| `Docs/architecture/embedding/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 5L/18W/2 links | 0 | `cfbbcc962fed` | R0 | D1 |
| `Docs/architecture/embedding/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/36W/3 links | 0 | `89e4353aa0b5` | R0 | D1 |
| `Docs/architecture/features/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 13L/55W/5 links | 0 | `ea744338437b` | R0 | D1 |
| `Docs/architecture/features/02-CURRENT-STATE-FEATURES.md` | `remove` | 02-CURRENT-STATE.md | 8L/33W/2 links | 0 | `d6fa3db5885a` | R0 | D1 |
| `Docs/architecture/features/03-GAP-ANALYSIS-FEATURES.md` | `remove` | 03-GAP-ANALYSIS.md | 8L/33W/2 links | 0 | `836711eb812b` | R0 | D1 |
| `Docs/architecture/features/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 11L/43W/3 links | 0 | `00835617f1ca` | R0 | D1 |
| `Docs/architecture/features/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 11L/43W/3 links | 0 | `7a27a6667a1b` | R0 | D1 |
| `Docs/architecture/features/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 8L/33W/2 links | 0 | `827c7ba5e2d4` | R0 | D1 |
| `Docs/architecture/features/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 8L/33W/2 links | 0 | `2960418a5c00` | R0 | D1 |
| `Docs/architecture/init/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/48W/3 links | 0 | `146b585bb286` | R0 | D1 |
| `Docs/architecture/init/02-CURRENT-STATE-INIT.md` | `remove` | 02-CURRENT-STATE.md | 6L/15W/1 links | 0 | `cd1fd6236fd9` | R0 | D1 |
| `Docs/architecture/init/03-GAP-ANALYSIS-INIT.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/15W/1 links | 0 | `73131becdd02` | R0 | D1 |
| `Docs/architecture/init/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/26W/2 links | 0 | `b88526d9e1a5` | R0 | D1 |
| `Docs/architecture/init/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/26W/2 links | 0 | `d7d49eac9ba8` | R0 | D1 |
| `Docs/architecture/init/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/15W/1 links | 0 | `e0b8c4722a70` | R0 | D1 |
| `Docs/architecture/init/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/15W/1 links | 0 | `396dbed9e9e3` | R0 | D1 |
| `Docs/architecture/jit/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 11L/42W/4 links | 0 | `f3cdbf4ac0a1` | R0 | D1 |
| `Docs/architecture/jit/02-CURRENT-STATE-JIT.md` | `remove` | 02-CURRENT-STATE.md | 6L/20W/1 links | 0 | `19fb62098fa6` | R0 | D1 |
| `Docs/architecture/jit/03-GAP-ANALYSIS-JIT.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/20W/1 links | 0 | `de0827215e89` | R0 | D1 |
| `Docs/architecture/jit/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/22W/1 links | 0 | `02178ef15f83` | R0 | D1 |
| `Docs/architecture/jit/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/22W/1 links | 0 | `91eec273f451` | R0 | D1 |
| `Docs/architecture/jit/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/20W/1 links | 0 | `18a7b798439b` | R0 | D1 |
| `Docs/architecture/jit/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/20W/1 links | 0 | `c308089511bd` | R0 | D1 |
| `Docs/architecture/logging/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/39W/3 links | 0 | `4f7d639febd6` | R0 | D1 |
| `Docs/architecture/logging/02-CURRENT-STATE-LOGGING.md` | `remove` | 02-CURRENT-STATE.md | 4L/16W/1 links | 0 | `ed3a54468384` | R0 | D1 |
| `Docs/architecture/logging/03-GAP-ANALYSIS-LOGGING.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/16W/1 links | 0 | `bdb448e4d6f9` | R0 | D1 |
| `Docs/architecture/logging/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/18W/1 links | 0 | `e3c822fd80ad` | R0 | D1 |
| `Docs/architecture/logging/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/27W/2 links | 0 | `d4989f69468f` | R0 | D1 |
| `Docs/architecture/logging/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/16W/1 links | 0 | `9ceff0a1b445` | R0 | D1 |
| `Docs/architecture/logging/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/16W/1 links | 0 | `f7927c079a05` | R0 | D1 |
| `Docs/architecture/mangle/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 7L/28W/2 links | 0 | `0f7253bf8b08` | R0 | D1 |
| `Docs/architecture/mangle/02-CURRENT-STATE-MANGLE.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `fe29c1c7a93e` | R0 | D1 |
| `Docs/architecture/mangle/03-GAP-ANALYSIS-MANGLE.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `f62a36df7e4e` | R0 | D1 |
| `Docs/architecture/mangle/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 7L/21W/2 links | 0 | `245b8938c573` | R0 | D1 |
| `Docs/architecture/mangle/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/17W/1 links | 0 | `254387c984aa` | R0 | D1 |
| `Docs/architecture/mangle/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `f57f3002ca6b` | R0 | D1 |
| `Docs/architecture/mangle/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 5L/27W/2 links | 0 | `b40a0a9fff16` | R0 | D1 |
| `Docs/architecture/mcp/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 12L/45W/4 links | 0 | `3c7d60c302ba` | R0 | D1 |
| `Docs/architecture/mcp/02-CURRENT-STATE-MCP.md` | `remove` | 02-CURRENT-STATE.md | 6L/24W/3 links | 0 | `860f0ebb7a60` | R0 | D1 |
| `Docs/architecture/mcp/03-GAP-ANALYSIS-MCP.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/19W/2 links | 0 | `ef6cc4927e77` | R0 | D1 |
| `Docs/architecture/mcp/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/30W/3 links | 0 | `e3b3dc7214ae` | R0 | D1 |
| `Docs/architecture/mcp/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/30W/3 links | 0 | `0ee9555a49ed` | R0 | D1 |
| `Docs/architecture/mcp/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/19W/2 links | 0 | `1065ad1317fb` | R0 | D1 |
| `Docs/architecture/mcp/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/19W/2 links | 0 | `83b7d6a11d69` | R0 | D1 |
| `Docs/architecture/northstar/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/38W/3 links | 0 | `c2cdb4ac9059` | R0 | D1 |
| `Docs/architecture/northstar/02-CURRENT-STATE-NORTHSTAR.md` | `remove` | 02-CURRENT-STATE.md | 4L/20W/2 links | 0 | `4222381c80be` | R0 | D1 |
| `Docs/architecture/northstar/03-GAP-ANALYSIS-NORTHSTAR.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `f62a36df7e4e` | R0 | D1 |
| `Docs/architecture/northstar/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/26W/2 links | 0 | `832d27b62938` | R0 | D1 |
| `Docs/architecture/northstar/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/26W/2 links | 0 | `b3974646895f` | R0 | D1 |
| `Docs/architecture/northstar/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `f57f3002ca6b` | R0 | D1 |
| `Docs/architecture/northstar/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 5L/32W/1 links | 0 | `99b60c2b087f` | R0 | D1 |
| `Docs/architecture/observability/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 12L/50W/3 links | 0 | `3d1834d1167a` | R0 | D1 |
| `Docs/architecture/observability/02-CURRENT-STATE-OBSERVABILITY.md` | `remove` | 02-CURRENT-STATE.md | 6L/17W/1 links | 0 | `128a1e1334d9` | R0 | D1 |
| `Docs/architecture/observability/03-GAP-ANALYSIS-OBSERVABILITY.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/17W/1 links | 0 | `46be778351cb` | R0 | D1 |
| `Docs/architecture/observability/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 9L/27W/2 links | 0 | `47207e468588` | R0 | D1 |
| `Docs/architecture/observability/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/19W/1 links | 0 | `b9d37e1185af` | R0 | D1 |
| `Docs/architecture/observability/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/17W/1 links | 0 | `246e8a6ca246` | R0 | D1 |
| `Docs/architecture/observability/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/17W/1 links | 0 | `ad507ce47142` | R0 | D1 |
| `Docs/architecture/perception/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/39W/4 links | 0 | `9e35ac38dcdc` | R0 | D1 |
| `Docs/architecture/perception/02-CURRENT-STATE-PERCEPTION.md` | `remove` | 02-CURRENT-STATE.md | 6L/30W/3 links | 0 | `9f960d4c8be8` | R0 | D1 |
| `Docs/architecture/perception/03-GAP-ANALYSIS-PERCEPTION.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/25W/2 links | 0 | `2567b463469f` | R0 | D1 |
| `Docs/architecture/perception/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/36W/3 links | 0 | `85d11049e53b` | R0 | D1 |
| `Docs/architecture/perception/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/36W/3 links | 0 | `35aa135cd9af` | R0 | D1 |
| `Docs/architecture/perception/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/25W/2 links | 0 | `415cedcd7319` | R0 | D1 |
| `Docs/architecture/perception/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/25W/2 links | 0 | `6ff7e173ad46` | R0 | D1 |
| `Docs/architecture/persist/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/28W/3 links | 0 | `4f5091b1c5bf` | R0 | D1 |
| `Docs/architecture/persist/02-CURRENT-STATE-PERSIST.md` | `remove` | 02-CURRENT-STATE.md | 4L/14W/1 links | 0 | `82ef0af8db5f` | R0 | D1 |
| `Docs/architecture/persist/03-GAP-ANALYSIS-PERSIST.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/14W/1 links | 0 | `8b32f3dca5d4` | R0 | D1 |
| `Docs/architecture/persist/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 7L/24W/2 links | 0 | `8a9b80cf6fff` | R0 | D1 |
| `Docs/architecture/persist/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/16W/1 links | 0 | `bf4aeecd2e6c` | R0 | D1 |
| `Docs/architecture/persist/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/14W/1 links | 0 | `596a4973d65e` | R0 | D1 |
| `Docs/architecture/persist/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/14W/1 links | 0 | `241f03f8b57e` | R0 | D1 |
| `Docs/architecture/prompt/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 6L/24W/2 links | 0 | `a217c67cd577` | R0 | D1 |
| `Docs/architecture/prompt/02-CURRENT-STATE-PROMPT.md` | `remove` | 02-CURRENT-STATE.md | 6L/21W/1 links | 0 | `c2008c664dc4` | R0 | D1 |
| `Docs/architecture/prompt/03-GAP-ANALYSIS-PROMPT.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/21W/1 links | 0 | `4c4502113eb0` | R0 | D1 |
| `Docs/architecture/prompt/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/32W/2 links | 0 | `76cf34c0c8d7` | R0 | D1 |
| `Docs/architecture/prompt/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/32W/2 links | 0 | `ed612ce5c9d6` | R0 | D1 |
| `Docs/architecture/prompt/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/21W/1 links | 0 | `556c515a6492` | R0 | D1 |
| `Docs/architecture/prompt/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 8L/51W/3 links | 0 | `d2c144640d30` | R0 | D1 |
| `Docs/architecture/regression/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/38W/3 links | 0 | `bd157a6782fb` | R0 | D1 |
| `Docs/architecture/regression/02-CURRENT-STATE-REGRESSION.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `dda74136ecbb` | R0 | D1 |
| `Docs/architecture/regression/03-GAP-ANALYSIS-REGRESSION.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `a5c7cc5ccff2` | R0 | D1 |
| `Docs/architecture/regression/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/26W/2 links | 0 | `916e9f25b795` | R0 | D1 |
| `Docs/architecture/regression/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/17W/1 links | 0 | `9d3dcbe8e188` | R0 | D1 |
| `Docs/architecture/regression/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `391ebcd5cf31` | R0 | D1 |
| `Docs/architecture/regression/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/15W/1 links | 0 | `00fd39718762` | R0 | D1 |
| `Docs/architecture/retrieval/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 7L/22W/2 links | 0 | `6c315ed27247` | R0 | D1 |
| `Docs/architecture/retrieval/02-CURRENT-STATE-RETRIEVAL.md` | `remove` | 02-CURRENT-STATE.md | 4L/14W/1 links | 0 | `bca64a880fd3` | R0 | D1 |
| `Docs/architecture/retrieval/03-GAP-ANALYSIS-RETRIEVAL.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/14W/1 links | 0 | `070a02c63fb3` | R0 | D1 |
| `Docs/architecture/retrieval/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/16W/1 links | 0 | `28c05bc82e11` | R0 | D1 |
| `Docs/architecture/retrieval/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/25W/2 links | 0 | `df5f512652d4` | R0 | D1 |
| `Docs/architecture/retrieval/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/14W/1 links | 0 | `f8322eba21a7` | R0 | D1 |
| `Docs/architecture/retrieval/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/14W/1 links | 0 | `128ccee9aa0c` | R0 | D1 |
| `Docs/architecture/session/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/38W/3 links | 0 | `53e4430108f7` | R0 | D1 |
| `Docs/architecture/session/02-CURRENT-STATE-SESSION.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `fe29c1c7a93e` | R0 | D1 |
| `Docs/architecture/session/03-GAP-ANALYSIS-SESSION.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `f62a36df7e4e` | R0 | D1 |
| `Docs/architecture/session/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 7L/21W/2 links | 0 | `75ef1a24c3da` | R0 | D1 |
| `Docs/architecture/session/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/17W/1 links | 0 | `254387c984aa` | R0 | D1 |
| `Docs/architecture/session/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `f57f3002ca6b` | R0 | D1 |
| `Docs/architecture/session/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/30W/1 links | 0 | `6e8870aa0c5b` | R0 | D1 |
| `Docs/architecture/shards/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 5L/27W/3 links | 0 | `928929ce513e` | R0 | D1 |
| `Docs/architecture/shards/02-CURRENT-STATE-SHARDS.md` | `remove` | 02-CURRENT-STATE.md | 5L/20W/2 links | 0 | `0ad7b9dea738` | R0 | D1 |
| `Docs/architecture/shards/03-GAP-ANALYSIS-SHARDS.md` | `remove` | 03-GAP-ANALYSIS.md | 5L/20W/2 links | 0 | `880fbb3f95bf` | R0 | D1 |
| `Docs/architecture/shards/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 5L/31W/3 links | 0 | `48d9908fb0af` | R0 | D1 |
| `Docs/architecture/shards/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 5L/31W/3 links | 0 | `64ab69488dc9` | R0 | D1 |
| `Docs/architecture/shards/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 5L/20W/2 links | 0 | `e4581de923c8` | R0 | D1 |
| `Docs/architecture/shards/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/33W/3 links | 0 | `083cb50d70fc` | R0 | D1 |
| `Docs/architecture/sqlpragmas/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/40W/3 links | 0 | `6688581a19ad` | R0 | D1 |
| `Docs/architecture/sqlpragmas/02-CURRENT-STATE-SQLPRAGMAS.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `fe29c1c7a93e` | R0 | D1 |
| `Docs/architecture/sqlpragmas/03-GAP-ANALYSIS-SQLPRAGMAS.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `f62a36df7e4e` | R0 | D1 |
| `Docs/architecture/sqlpragmas/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 7L/21W/2 links | 0 | `75ef1a24c3da` | R0 | D1 |
| `Docs/architecture/sqlpragmas/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/22W/2 links | 0 | `50be5dc13ac0` | R0 | D1 |
| `Docs/architecture/sqlpragmas/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/11W/1 links | 0 | `68b275bc208f` | R0 | D1 |
| `Docs/architecture/sqlpragmas/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 5L/29W/2 links | 0 | `440f8ef2f10f` | R0 | D1 |
| `Docs/architecture/store/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/40W/3 links | 0 | `bb1ee467e6b2` | R0 | D1 |
| `Docs/architecture/store/02-CURRENT-STATE-STORE.md` | `remove` | 02-CURRENT-STATE.md | 4L/20W/2 links | 0 | `eb8766854465` | R0 | D1 |
| `Docs/architecture/store/03-GAP-ANALYSIS-STORE.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `34ad942e94ba` | R0 | D1 |
| `Docs/architecture/store/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/26W/2 links | 0 | `0e06b50eaffe` | R0 | D1 |
| `Docs/architecture/store/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/26W/2 links | 0 | `0b74ad197ba8` | R0 | D1 |
| `Docs/architecture/store/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `66f9b4ede2c1` | R0 | D1 |
| `Docs/architecture/store/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/15W/1 links | 0 | `1f86e43fc577` | R0 | D1 |
| `Docs/architecture/system/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/41W/3 links | 0 | `8eced542fbfe` | R0 | D1 |
| `Docs/architecture/system/02-CURRENT-STATE-SYSTEM.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `75ed1bcc9e7e` | R0 | D1 |
| `Docs/architecture/system/03-GAP-ANALYSIS-SYSTEM.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `e3fff33f07b8` | R0 | D1 |
| `Docs/architecture/system/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/17W/1 links | 0 | `5649d243a107` | R0 | D1 |
| `Docs/architecture/system/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/17W/1 links | 0 | `85d7fd995dd9` | R0 | D1 |
| `Docs/architecture/system/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `d80f013ddc02` | R0 | D1 |
| `Docs/architecture/system/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 5L/28W/2 links | 0 | `814712956641` | R0 | D1 |
| `Docs/architecture/tactile/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 11L/33W/4 links | 0 | `f131f23fda53` | R0 | D1 |
| `Docs/architecture/tactile/02-CURRENT-STATE-TACTILE.md` | `remove` | 02-CURRENT-STATE.md | 6L/24W/2 links | 0 | `0f0c178bf98a` | R0 | D1 |
| `Docs/architecture/tactile/03-GAP-ANALYSIS-TACTILE.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/19W/1 links | 0 | `cccbce4bf9c8` | R0 | D1 |
| `Docs/architecture/tactile/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/30W/2 links | 0 | `e1188e63c306` | R0 | D1 |
| `Docs/architecture/tactile/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/30W/2 links | 0 | `e12d6d448316` | R0 | D1 |
| `Docs/architecture/tactile/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/19W/1 links | 0 | `4acc0ff41b67` | R0 | D1 |
| `Docs/architecture/tactile/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/19W/1 links | 0 | `2ad3695d5862` | R0 | D1 |
| `Docs/architecture/testing/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 12L/45W/4 links | 0 | `3c7d60c302ba` | R0 | D1 |
| `Docs/architecture/testing/02-CURRENT-STATE-TESTING.md` | `remove` | 02-CURRENT-STATE.md | 11L/35W/3 links | 0 | `75eea70d9d03` | R0 | D1 |
| `Docs/architecture/testing/03-GAP-ANALYSIS-TESTING.md` | `remove` | 03-GAP-ANALYSIS.md | 11L/35W/3 links | 0 | `cf8acdb48e54` | R0 | D1 |
| `Docs/architecture/testing/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 11L/41W/3 links | 0 | `7c08c6a3b68d` | R0 | D1 |
| `Docs/architecture/testing/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 12L/49W/4 links | 0 | `70d7800f1011` | R0 | D1 |
| `Docs/architecture/testing/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 11L/39W/3 links | 0 | `c2febd042c76` | R0 | D1 |
| `Docs/architecture/testing/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 10L/31W/2 links | 0 | `d1bb074ac69d` | R0 | D1 |
| `Docs/architecture/tools/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/42W/4 links | 0 | `28a108817872` | R0 | D1 |
| `Docs/architecture/tools/02-CURRENT-STATE-TOOLS.md` | `remove` | 02-CURRENT-STATE.md | 6L/28W/2 links | 0 | `93ce18d4d865` | R0 | D1 |
| `Docs/architecture/tools/03-GAP-ANALYSIS-TOOLS.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/28W/2 links | 0 | `950b87f89a1f` | R0 | D1 |
| `Docs/architecture/tools/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/39W/3 links | 0 | `89a292cd7d43` | R0 | D1 |
| `Docs/architecture/tools/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/39W/3 links | 0 | `b91b6606ddcf` | R0 | D1 |
| `Docs/architecture/tools/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/28W/2 links | 0 | `efb77cad719b` | R0 | D1 |
| `Docs/architecture/tools/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/28W/2 links | 0 | `f4f2a665479d` | R0 | D1 |
| `Docs/architecture/transparency/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 11L/51W/4 links | 0 | `326769afb0f4` | R0 | D1 |
| `Docs/architecture/transparency/02-CURRENT-STATE-TRANSPARENCY.md` | `remove` | 02-CURRENT-STATE.md | 6L/25W/3 links | 0 | `493c8f40add7` | R0 | D1 |
| `Docs/architecture/transparency/03-GAP-ANALYSIS-TRANSPARENCY.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/14W/1 links | 0 | `4a9d5a29faed` | R0 | D1 |
| `Docs/architecture/transparency/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/25W/2 links | 0 | `5219905f9e07` | R0 | D1 |
| `Docs/architecture/transparency/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/25W/2 links | 0 | `989f079630d2` | R0 | D1 |
| `Docs/architecture/transparency/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/14W/1 links | 0 | `3f95cd37bf5e` | R0 | D1 |
| `Docs/architecture/transparency/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/20W/1 links | 0 | `230426a0e5de` | R0 | D1 |
| `Docs/architecture/types/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 12L/49W/3 links | 0 | `6b0e6b2c75bb` | R0 | D1 |
| `Docs/architecture/types/02-CURRENT-STATE-TYPES.md` | `remove` | 02-CURRENT-STATE.md | 6L/17W/1 links | 0 | `49e728cdc3f5` | R0 | D1 |
| `Docs/architecture/types/03-GAP-ANALYSIS-TYPES.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/18W/1 links | 0 | `7c9e15dcc114` | R0 | D1 |
| `Docs/architecture/types/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/20W/1 links | 0 | `9e3c4091daf6` | R0 | D1 |
| `Docs/architecture/types/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 7L/29W/2 links | 0 | `539ac1fb9fd6` | R0 | D1 |
| `Docs/architecture/types/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/17W/1 links | 0 | `778f1dcc7805` | R0 | D1 |
| `Docs/architecture/types/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/18W/1 links | 0 | `bbc7ec032802` | R0 | D1 |
| `Docs/architecture/usage/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/37W/3 links | 0 | `3199762b2906` | R0 | D1 |
| `Docs/architecture/usage/02-CURRENT-STATE-USAGE.md` | `remove` | 02-CURRENT-STATE.md | 6L/23W/1 links | 0 | `9e6ffc6965b5` | R0 | D1 |
| `Docs/architecture/usage/03-GAP-ANALYSIS-USAGE.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/23W/1 links | 0 | `0e2e2034b906` | R0 | D1 |
| `Docs/architecture/usage/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/25W/1 links | 0 | `8bbc1c8bf277` | R0 | D1 |
| `Docs/architecture/usage/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/25W/1 links | 0 | `18254390255f` | R0 | D1 |
| `Docs/architecture/usage/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/23W/1 links | 0 | `3b92e532c38c` | R0 | D1 |
| `Docs/architecture/usage/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/23W/1 links | 0 | `6436d4a615ae` | R0 | D1 |
| `Docs/architecture/ux/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 8L/38W/3 links | 0 | `2e72f9b1418a` | R0 | D1 |
| `Docs/architecture/ux/02-CURRENT-STATE-UX.md` | `remove` | 02-CURRENT-STATE.md | 4L/16W/2 links | 0 | `b07370a94e9d` | R0 | D1 |
| `Docs/architecture/ux/03-GAP-ANALYSIS-UX.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/16W/2 links | 0 | `52587e9bb2c6` | R0 | D1 |
| `Docs/architecture/ux/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/18W/2 links | 0 | `9a646b6d3253` | R0 | D1 |
| `Docs/architecture/ux/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/27W/3 links | 0 | `7ece65f33f8c` | R0 | D1 |
| `Docs/architecture/ux/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/16W/2 links | 0 | `cad5d44461a2` | R0 | D1 |
| `Docs/architecture/ux/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 6L/34W/3 links | 0 | `fecde4d3b1f6` | R0 | D1 |
| `Docs/architecture/verification/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 6L/33W/2 links | 0 | `a3d0d2afa73b` | R0 | D1 |
| `Docs/architecture/verification/02-CURRENT-STATE-VERIFICATION.md` | `remove` | 02-CURRENT-STATE.md | 6L/22W/1 links | 0 | `84e8e6227e21` | R0 | D1 |
| `Docs/architecture/verification/03-GAP-ANALYSIS-VERIFICATION.md` | `remove` | 03-GAP-ANALYSIS.md | 6L/22W/1 links | 0 | `7c05c2125eba` | R0 | D1 |
| `Docs/architecture/verification/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 6L/33W/2 links | 0 | `4ae24168760c` | R0 | D1 |
| `Docs/architecture/verification/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 6L/33W/2 links | 0 | `61d671c25323` | R0 | D1 |
| `Docs/architecture/verification/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 6L/22W/1 links | 0 | `3b27bd106135` | R0 | D1 |
| `Docs/architecture/verification/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 7L/38W/2 links | 0 | `aed4f1d51a29` | R0 | D1 |
| `Docs/architecture/world/01-DOMAIN-MODEL.md` | `remove` | 01-VISION.md; 05-INTERNAL-ARCHITECTURE.md; 06-PUBLIC-API-AND-TYPES.md | 10L/35W/3 links | 0 | `2f9b4f3e9b19` | R0 | D1 |
| `Docs/architecture/world/02-CURRENT-STATE-WORLD.md` | `remove` | 02-CURRENT-STATE.md | 4L/15W/1 links | 0 | `1df4a16782eb` | R0 | D1 |
| `Docs/architecture/world/03-GAP-ANALYSIS-WORLD.md` | `remove` | 03-GAP-ANALYSIS.md | 4L/15W/1 links | 0 | `4fa363b61423` | R0 | D1 |
| `Docs/architecture/world/04-INVARIANTS-AND-GATES.md` | `remove` | 04-ARCHITECTURAL-PRINCIPLES.md; 09-SAFETY-AND-INVARIANTS.md | 4L/26W/2 links | 0 | `669a75a78348` | R0 | D1 |
| `Docs/architecture/world/05-CROSS-SYSTEM-WIRING.md` | `remove` | 07-DEPENDENCY-MAP.md; 08-WIRING-AND-INTEGRATION.md | 4L/26W/2 links | 0 | `ffa8eeec9e3a` | R0 | D1 |
| `Docs/architecture/world/06-TESTING-STRATEGY.md` | `remove` | 10-TESTING-ALIGNMENT.md | 4L/15W/1 links | 0 | `c05827e6a437` | R0 | D1 |
| `Docs/architecture/world/08-FAILURE-MODES.md` | `remove` | 12-FAILURE-MODES.md | 4L/15W/1 links | 0 | `ba1ec5ed3b39` | R0 | D1 |

## Uncertainties

- This is a preliminary live-tree ledger, not deletion approval. Concurrent corpus edits can invalidate `sha12` identities and must trigger reclassification.
- The comparison establishes that the matched stubs themselves contain no unique substantive claims. It does not yet certify the semantic completeness or truth of every canonical destination; that belongs to the per-corpus source-grounded audit.
- Non-Markdown consumers that construct filenames dynamically cannot be disproved by a text scan. D1 therefore requires a final full-path consumer scan and bounded compatibility check immediately before removal.
