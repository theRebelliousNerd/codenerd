---
doc-class: deep-dive
subsystem: knowledge-acquisition
implementation-status: proposed
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Knowledge acquisition receipts must describe the version actually read

**Decision:** separate a requested manifest version from the version and resolution context a source actually used. Make refresh policy consume the latter when deciding whether stored documentation is current. Choosing an arbitrary manifest and recording its version is not evidence that GoDoc read that version.

**Finding:** with root and nested Go modules requiring different versions of the same recognized dependency, subject selection can choose nested version B while the production GoDoc source reads root version A. GoDoc correctly labels its atoms A, but the acquisition ledger records B. A later root upgrade to C leaves actual documentation A pinned nowhere; nevertheless, B remains in the nested manifest, so the current version-change policy does not request refresh. This is a false acquisition receipt with a concrete refresh consequence, not just a missing multi-module feature.

**Evidence status:** static inspection of public main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked on 2026-10-30. The witness is source-predicted and conditional, not a reproduced incident or measured answer-quality regression. No application, Go command, test, database experiment, provider request, or model inference was executed. This document proposes work; it does not implement it.

## Existing contract and neighboring ownership

Root guidance requires stale evidence not to remain current after source changes and assigns executive decisions to Mangle. The relevant implementation contract is unusually explicit: `dependency_pin(Module, Version)` permits two manifests to pin two versions; `knowledge_acquired_version` represents the version a source acquired; and `/version_changed` means the acquired version is no longer pinned by any manifest. This audit accepts that existing any-manifest policy rather than silently replacing it with a stronger per-manifest obligation. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/agents.md), [version facts](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/schemas_kb_refresh.mg#L27-L34), [refresh meaning](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/schemas_kb_refresh.mg#L116-L121).

Bounded review covered main's expert, knowledge and JIT audit inventory and the 0715, 0747, 0817 and 0850 additions. The nearest previous finding, [Go major-version pin continuity](https://github.com/theRebelliousNerd/codenerd/blob/07bdcb50873498d22fce498f599073df00ac5750/Docs/audits/knowledge-go-major-version-pin-continuity.md), concerns family-root vocabulary failing to bind a real `/vN` module. Here an exact unversioned module match succeeds, and a nonempty version survives, but it is not the version the source reads. [Subject ownership](https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/knowledge-acquisition-subject-ownership.md) concerns slug collisions and replacement deletion; this witness requires neither. [Refresh/prune](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-refresh-prune.md) owns an older measurement authorizing deletion of a replacement. Receipt truthfulness is a different boundary. These are scoped distinctions, not an exhaustive novelty claim.

## Production path

### Manifest identity is collected and then discarded

`ManifestPin` retains ecosystem, module, version and workspace-relative manifest path. `ManifestPins` reads root and accepted nested manifests, parses exact Go requirements, then sorts by module, manifest path and version. Reading the root first does not preserve root priority after this sort. [Pin collection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/manifest_pins.go#L14-L90).

Cobra is a real vocabulary entry, not an invented unsupported package. `pinnedPackages` walks sorted pins, accepts the first exact match or valid package-inside-module match, emits a subject containing module, ecosystem and that pin's version, then breaks. It does not carry the selected manifest. `Subject.Key()` is the module string, not a manifest/version pair. [Cobra vocabulary](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/technology_vocabulary.go#L44), [first matching pin](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources.go#L77-L105), [subject fields and key](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L26-L58).

This is not an isolated helper. Init collects pins, builds these subjects and calls `knowledge.Fill` when research is enabled. The refresher uses the same subject builder. [Init](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/agents_knowledge.go#L130-L149), [refresh subject construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L101-L129).

### Source execution uses the root context rather than the selected manifest

Production `NewAcquirer` constructs one `GoDocSource` with `Workspace: cfg.Workspace`. Boot supplies the Cortex workspace root. GoDoc runs `go list -m` to observe a version and `go doc` to read package documentation in that same directory; the command adapter actually sets `cmd.Dir`. The subject's requested version is not used to select either command's resolution context. [Source wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L79-L102), [boot configuration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh_boot.go#L38-L58), [command directory and acquisition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L148-L238).

GoDoc does preserve what it observes in the atom: source `godoc/<module>@<version>`, concept `godoc/<package>@<version>`, and the content header all use the result of `ModuleVersion`. That is an important existing safeguard, not part of the defect. The mismatch is introduced when the acquirer makes the successful outcome: it unconditionally copies `subject.Version`, then persists that outcome under the `(source, subject)` primary key. [Observed atom identity](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L195-L238), [outcome construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L160-L188), [ledger schema](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L193-L205), [ledger write](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L305-L318).

### Refresh consumes the ledger rather than the atom provenance

Measurement emits `knowledge_acquired_version` from a successful nonempty ledger version. The refresher publishes all current module/version pairs, deduplicated across manifests. The Mangle rule derives version change only when an acquired version is absent from that union. Nothing in this path reconciles the receipt with the version carried by its GoDoc atoms. [Measurement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/measure.go#L35-L55), [current pin facts](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L101-L115), [version-change rule](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L64-L70).

The background component is registered in normal boot; selected refresh work calls `knowledge.Refresh`. These KB rows are not merely an audit log: the JIT collector retrieves selected and pushed expert knowledge and converts those results into prompt candidates. Actual inclusion still depends on retrieval, selection and budget. [Boot registration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/factory.go#L2919-L2922), [refresh consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L145-L169), [JIT consumer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L349-L386).

## Conditional witness

Use distinct valid Cobra versions A, B and C. These symbols describe a controlled source witness, not observed installed versions.

- The workspace root contains a valid `go.mod` requiring `github.com/spf13/cobra` at A.
- An accepted nested path `a/go.mod` requires that exact module at B.
- A ready agent has the `cobra` domain; research is enabled and GoDoc succeeds.
- Root command resolution selects A initially and C after the root requirement changes. There is no relevant replacement, exclusion, vendored divergence, alternate modfile, or transitive requirement that changes those selections.
- Workspace mode is disabled, or no `go.work` is selected from the current directory, ancestors or `GOWORK`. The nested module is not independently incorporated into the root dependency graph.
- Files and resolution context are stable across each acquisition. No concurrent mutation, partial write, source failure or concept collision is needed.

These conditions matter: a Go requirement is a minimum, not necessarily the effective selected version; MVS and workspace configuration can alter the result. In single-module context, commands operate on the module containing the working directory. This witness deliberately fixes the effective versions rather than assuming every written requirement is what every Go command reads. [Official Go module and workspace reference](https://go.dev/ref/mod#workspaces), [MVS](https://go.dev/ref/mod#minimal-version-selection).

For equal module strings, lexical manifest sorting places `a/go.mod` before `go.mod`. The resulting state is:

| Stage | Actual GoDoc material | Successful ledger version | Current manifest union |
|---|---|---|---|
| Initial acquisition | A, correctly labeled A | B | A and B |
| Root changes A to C | Still A | Still B | C and B |
| Next measurement before other refresh triggers | A is pinned nowhere | B still appears current | C and B |

At the last stage, the version rule sees acquired B and current B, so its negated dependency-pin condition fails. The decisive omission is not that root A was still useful to the nested module; it is that **A is no longer pinned anywhere**, while the wrong receipt B shields it from this trigger.

The witness isolates version freshness by keeping successful non-GoDoc sources younger than their respective stale thresholds, avoiding unresolved misses, and excluding an unavailable source that has newly become available or an old failed attempt due for retry. A later independent trigger may still reacquire the subject. The finding is therefore suppressed version-change admission and inconsistent provenance, not a guarantee of permanent staleness or harmful generated code.

## Existing controls and counterevidence

**Any-manifest retention is intentional.** If real acquired A remains pinned in a sibling manifest, today's policy does not demand refresh merely because another manifest changed. That is a legitimate weaker contract. A fair repair must not claim this retention is itself an established bug.

**Other refresh reasons exist.** Web, Context7 and repository acquisitions have age limits of 30, 90 and 7 days; failed attempts have a one-day retry threshold; new availability and gaps have separate rules. GoDoc has no age limit because version change is intended to reread it. The initial delay is 120 seconds, the recurring interval is 3600 seconds, and the budget is four subjects. Those controls bound work or provide independent repair opportunities; they do not make an inaccurate acquired-version fact accurate. [Age, scheduling and budget](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L8-L26), [admission rules](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L72-L105).

**Persistence has useful protections.** Fill skips a source already recorded as acquired; Refresh deliberately asks all sources again. Failed or empty replacement does not delete prior material. Pruning is now bounded by the measured generation, so a refresh replacement survives a previously derived prune. Preserve those behaviors. None binds the source's effective version to its receipt. [Fill and Refresh](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L207-L225), [replacement and retained concepts](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L250-L318), [generation-aware prune](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L174-L189).

**Existing tests establish narrower contracts.** Subject construction is tested with root Cobra and nested React, not two manifests requiring the same module differently. GoDoc's test verifies correctly versioned atom provenance with a fixed workspace, but constructs its subject without a Version and does not exercise receipt persistence or refresh derivation. These are useful tests; their composition gap is the proposed test target. [Subject test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/knowledge_sources_test.go#L15-L65), [GoDoc test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/knowledge_test.go#L141-L180).

## Repair the receipt before broadening scope

The smallest defensible change is an explicit source result that distinguishes requested metadata from observed acquisition identity. GoDoc already knows the version used to label its atoms; propagate that value as typed receipt data instead of reconstructing it from display prose or copying the request. Keep requested B for diagnostics, but record observed A for freshness. A failed version lookup returning `unknown` must remain unknown evidence, not silently become requested B.

Require agreement between a successful receipt and the material it covers. Define partial acquisition, empty results, failures and cancellation separately from the identity of retained last-successful material. Do not retroactively relabel existing A content B to make fields agree. Receipt updates and the materialization they attest should commit coherently; external source calls should remain outside any long database transaction.

This minimal correction can preserve the current any-manifest policy and the current root-resolution behavior. It fixes the A/B to C/B witness without multiplying subjects. It does **not** establish that every nested module receives documentation for its own version. State that remaining limitation instead of claiming complete monorepo support.

If complete multi-context coverage is desired, define that contract before implementation:

1. Preserve an explicit resolution context in subjects and durable ownership, rather than just a friendly manifest label.
2. Resolve dependencies and run GoDoc under that same context. Directory alone is not a complete identity when workspace mode or replacements apply.
3. Scope importer/package enumeration consistently; today's `KernelImports.Packages` unions matching imports from the world model without manifest context.
4. Key outcomes and replacement ownership by the chosen context/module/source identity, and derive refresh against the corresponding measured effective dependency identity.
5. Share immutable equivalent content where useful, but retain separate coverage receipts. Do not duplicate broad web research for every manifest merely because GoDoc needs distinct resolution contexts.

The unscoped importer behavior is visible in [production import enumeration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L110-L169). It is a repair-design constraint, not an additional demonstrated defect in the clean root-package witness.

Sorting root first is not a durable receipt fix. It can hide this fixture while leaving effective-version differences, nested-only resolution and workspace behavior undefined. Likewise, always selecting the highest manifest requirement substitutes a guess for observed resolution. Preserve the existing correct package-inside-parent-module prefix relation; this finding does not justify reversing it.

## Migration and operational cost

Existing acquisition tables are per-agent SQLite state with a `(source, subject)` primary key and additive column migration. Adding requested/observed identity fields can be additive; adding context to the key requires a deliberate table/index migration and compatible readers. [Current migration mechanism](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L325-L350).

Do not trust every legacy `version` as an observed version. For a legacy GoDoc receipt, consistent exact source/concept provenance across its surviving owned atoms may justify recovering the observed identity. Missing rows, mixed identities, ambiguous ownership or `unknown` require an explicit unverified state and bounded reacquisition. Keep useful old content labeled with its real provenance while coverage is repaired. Do not assign it to every current manifest, erase unrelated authored knowledge, reset every source's successful state, or treat reading today's manifest as proof about yesterday's acquisition.

A one-time migration must be resumable and idempotent. Record whether provenance has been checked so startup does not repeatedly schedule the same backfill. Reuse ordinary availability, retry, priority and budget controls; a migration must not turn unknown evidence into unbounded high-priority work. Preserve the existing prune generation fence and the separately audited source-owner replacement contract.

The narrow receipt fix can reuse GoDoc's existing version observation, so it need not add a Go subprocess or provider request on an ordinary successful acquisition. Its costs are interface changes across sources/mocks, small additional result/ledger metadata, compatibility handling, and new consistency checks. If manifests or effective context can change during acquisition, checking a context fingerprint before commit adds reads and possible retries; that protection is broader than the stable witness and should be evaluated separately.

Full context support costs more: with D resolution contexts and P relevant packages, documentation work can grow toward the sum of packages across contexts rather than the current union. It expands receipts, atoms or ownership links, embeddings, retrieval candidates and kernel facts. It can also consume the four-subject budget sooner unless equivalent contexts are deduplicated and fairness is defined. Version-triggered Refresh currently calls every source, so fixing a missed trigger may increase grounded-search and Context7 work even when only GoDoc content changed. This is correctness work with potential additional spend, not an established optimization.

## Fair evaluation

All verification below is proposed and unrun. Use deterministic source adapters first, but compose the real subject builder, acquirer, ledger measurement and policy consumer. A fake GoDoc command must assert the directory it receives; returning B everywhere would conceal the defect being tested.

The decisive fixture starts with root A and nested B, proves that the real builder selects B, produces root-context A documentation, then examines both the atom provenance and persisted receipt. After changing only root A to C, assert that A is absent from the current pin union. Baseline should show the wrong B receipt and missing version-change derivation; the narrow repair should record A and derive refresh. After a successful refresh, prove actual C material, a C receipt, and no redundant version-change admission on the next unchanged pass.

Include these controls:

- Root-only, identical root/nested versions, manifest-name ordering reversed, and duplicate pin facts.
- A remains genuinely pinned in another manifest: retain the current any-manifest semantics for the narrow fix.
- Root lacks the dependency while only a nested module has it: report unavailable/unsupported root resolution honestly rather than claiming nested coverage.
- Active workspace, ancestor or environment-selected workspace, MVS-selected versions, replacements, vendoring and alternate modfiles: explicitly validate or reject the intended resolution contract.
- Empty output, `unknown` lookup, failed package reads, partial success, cancellation, database failure, legacy empty version, inconsistent provenance, restart during migration and idempotent backfill.
- A newer generation is installed after measurement: existing prune protection still preserves it.
- Competing refresh subjects and the unchanged budget: distinguish correct derivation from delayed execution.
- Non-GoDoc stale/retry/new-source/gap controls: prove their ordinary admissions remain, and report when one masks the missing version trigger.
- Single-module `/vN` continuity and distinct-subject ownership controls so this repair does not regress the separately owned cases.

Then exercise the normal boot/refresher and explicit-refresh entry paths with an offline, isolated module fixture if execution is authorized. Observe actual command contexts and effective versions; do not use network success as the test oracle. JIT evaluation should measure whether version-appropriate expert rows are retrieved and ultimately included, not infer answer improvement merely from a fixed database field.

Report command counts, provider calls, wall time, database reads/writes, embeddings, facts, queue delay, retained rows, migration duration and repeat-run work on matched fixtures. Count the extra refreshes that correctness newly admits. For answer-quality evaluation, keep models, tasks, budgets and source availability matched and separate receipt correctness from row inclusion and downstream task success. No latency reduction, token saving, prevalence estimate or quality improvement is established by this static audit.

