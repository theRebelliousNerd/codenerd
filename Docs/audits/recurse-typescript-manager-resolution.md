---
doc-class: deep-dive
subsystem: gates
implementation-status: proposed
last-verified: 2026-11-04
verified-against: 2c90e06cf9b590491b40ab72985c636a13336665
verification-method: static-source-inspection
supersedes: []
---

# Recurse's TypeScript fallback loses the project's package manager

## Finding and evidence boundary

**The detected TypeScript build gate switches to npm execution even after detecting a different package manager.** `jsGates` selects pnpm, Yarn or Bun from lockfiles and uses that manager for package scripts. But a project with `tsconfig.json` and no `build` script receives the unconditional command `npx tsc --noEmit`. This loses more than the command spelling: a package manager can own the installed compiler's resolution and loader. [Detection][detect]

A prepared Yarn Plug'n'Play project is a concrete counterexample. Its pinned `typescript` dependency can be available through Yarn without any `node_modules/.bin/tsc`. The fallback does not ask Yarn for that dependency. With no matching local/ancestor/global executable, npm interprets `tsc` as a package request, rather than locating the project's `typescript` package through PnP. Depending on the admitted npm version, configuration, cache and network, this can produce a package-resolution failure or enter npm's package-fetch/cache-install path. If `npx` itself is absent, codeNERD instead reports an unavailable gate even though the project's own compiler is available through Yarn. [Yarn PnP][yarn-pnp] [Yarn execution][yarn-run] [npm resolution][npm-resolution]

**Recommended contract:** a detected check must retain the selected project's package-manager and installed-tool identity. Missing or unsupported resolution is an unavailable/unsupported capability, not permission to obtain a similarly named package from another ecosystem path. Preserve explicit project-command precedence, noninteractive process ownership, cancellation and Mangle verdicts.

This is a source-derived portability and integration finding, not a reproduced incident, a claim that a particular registry package is malicious, or an assertion that a false keep occurred. CodeNERD source was inspected at public main **`2c90e06cf9b590491b40ab72985c636a13336665`**, observed November 4, 2026 at 11:08 UTC and rechecked unchanged at 11:17 UTC. The npm reference implementation is separately pinned below; it establishes a concrete supported execution mechanism, not the npm version installed on any user's machine. No application, gate, test, build, install, model call, benchmark or failure injection ran. No repository, credentials or settings were changed. The witnesses and evaluations are conditional and entirely unrun.

## Intended contract and production trace

Root guidance gives creative design to the model and measured planning, orchestration and safety to the deterministic executive. The gate package exists specifically to run a foreign workspace's own checks rather than impose Go-only verification. Its precedence contract puts explicit `nerd.md` commands first, then marker-based detection. A compiler discovered under one package manager must therefore not silently acquire a different tool-resolution contract before becoming evidence. This is host integration work, not a task for a more persuasive proposer or interrogator. [Guidance][guidance] [Gate purpose][gate-contract] [Mission boundary][mission-spec]

The reachable chain is:

1. Both the CLI and chat adapters invoke `campaign.RunRecurseCycles`. Each pass calls `gates.Detect` and records unavailable checks. Workspace-scoped runnable gates are measured when `len(r.state) == 0`, including the fresh-run baseline used by these witnesses; this is not an unconditional remeasurement on every pass. No model decision is needed to reach that baseline. [CLI][cli] [Chat][chat] [Pass][pass]
2. `Detect` preserves explicit canonical build commands and suppresses the detected build fallback when one exists. Otherwise it calls `jsGates` for a root `package.json`. [Precedence][precedence]
3. `jsGates` reads only the `scripts` field. It picks a manager from `pnpm-lock.yaml`, `yarn.lock` or Bun lockfiles. The script branches use `pm`, but the no-build-script TypeScript branch emits `[]string{"npx", "tsc", "--noEmit"}` regardless of `pm`. Its source label also omits the manager. [Detection][detect]
4. `split` tests only the command's first program. Finding `npx` admits the descriptor; it does not establish that the intended TypeScript package is installed or reachable. Conversely, absence of `npx` moves the gate into `Unavailable`; Yarn's availability is not consulted for this descriptor. [Partition][split]
5. Workspace measurement takes only `set.Gates`. `measure` dispatches the chosen descriptor through `gates.Run`, `build.WorkspaceCommand` and `processutil.Run`. None changes `npx` into the previously selected manager or adds an explicit TypeScript package identity/no-install contract. [Measurement][measure] [Run][run] [Command construction][command]
6. `processutil` supplies finite empty stdin. That is appropriate for unattended gates, but is not an installation refusal: the pinned npm implementation explicitly treats non-TTY execution as a branch that warns and proceeds with required installation, unless configuration has already forbidden it. [Noninteractive execution][process] [npm install branch][npm-install]

This is a fresh observation path. It does not require stale gate baselines, receipt reuse, environment-key corruption, path quoting, missing import edges, concurrent writers or a failed recovery. It occurs before the later cycle ratchet decides whether any edit is kept.

## Why Yarn PnP discriminates the boundary

Yarn documents PnP as an installation mode that uses `.pnp.cjs` to describe dependency locations instead of a `node_modules` tree. It also documents that dependency binaries are exposed in the workspace that declares them. `yarn run` can execute a matching binary provided by a local dependency. Thus absence of `node_modules/.bin/tsc` does not establish absence of the project's compiler. [Yarn PnP][yarn-pnp] [Yarn execution][yarn-run]

The distinction between a binary and a package matters here. TypeScript 5.9.3's own manifest names its package `typescript` and exposes the binary `tsc`. They are different identities. A fallback that turns the binary name into a package name has not selected that manifest or the project's locked version. [TypeScript manifest][typescript-package]

In the pinned npm source, execution without an explicit package first checks matching local package `bin` metadata and filesystem binary locations, including global fallback. If those do not resolve, it pushes the first argument into the package list. For this descriptor that argument is `tsc`. It then loads the actual npm tree and resolves the missing package manifest; its manifest acquisition requests online preference. Subsequent steps can select the package's exposed binary and install into npm's cache. This inspected route does not ask `.pnp.cjs` to resolve the project's `typescript` dependency. [npm resolution][npm-resolution] [Manifest acquisition][npm-manifest] [npm installation][npm-install]

The witness below excludes a root `bin.tsc`, ancestor `node_modules/.bin/tsc`, global compiler and other compatibility shims. Those are real ways the npm route can find a command; they must not be silently assumed absent in a report about a particular installation. Even when one exists, its selection alone would not prove identity with the compiler locked by Yarn.

## Conditional witness A: a prepared compiler becomes unavailable

Consider an ordinary, otherwise admitted TypeScript workspace with these properties:

- `package.json` has a direct, pinned `typescript` development dependency and no `scripts.build` entry. It may have `scripts.typecheck: "tsc --noEmit"`; that name is not consulted by the fallback.
- `yarn.lock` and an explicit PnP configuration belong to a completed, usable Yarn installation. Its loader and dependency artifacts are already present. This is not an uninstalled project or a request to install anything.
- `tsconfig.json` includes an admitted source such as `src/subject.ts`.
- No canonical `nerd.md` build override is present.
- Yarn and Node are executable in the effective normal environment, and Yarn can resolve the root workspace's pinned TypeScript dependency. `npx` is absent.

The source-derived result is precise: `jsGates` still chooses the npm descriptor; `split` places `js:typecheck` in `Unavailable`; the baseline does not run it. The result summary retains a “gate that could not run” entry. The checker available through Yarn was never considered. [Detection][detect] [Partition][split] [Unavailable reporting][report]

This is not a silently recorded pass. Nor does it establish that the entire recurse run stops: other gates and ordinary policy still govern their own work. It establishes a false capability loss caused by substituting a launcher the project does not require.

## Conditional witness B: installed npx changes the selected package

Keep the same valid PnP workspace, but make a version of `npx` implementing the pinned reference behavior available. Also require:

- No root `bin.tsc`, relevant ancestor `node_modules/.bin/tsc`, global `tsc`, npm-installed matching package or deliberate PnP integration shim satisfies npm's preliminary lookup.
- The npm cache and configuration are controlled and recorded. There is no assumption that the Yarn dependency cache doubles as npm's actual installed tree.
- The source, lock and install state remain stable; no unrelated startup, platform, permission or cancellation failure intervenes.

Then:

1. `split` admits `js:typecheck` because `npx` exists.
2. The baseline launches `npx tsc --noEmit` from the workspace, noninteractively.
3. npm's missing-binary branch requests package **`tsc`**, not the locked **`typescript`** dependency available through Yarn.
4. If registry/cache resolution is unavailable, the process fails or cannot complete without having run that compiler. If resolution and npm policy permit the operation, the reference code can prepare/install a package in npm's cache and execute its advertised command. The precise downloaded version, package behavior and exit result are outside the evidence here.

The decisive failure is step 3: the named compiler requirement has been replaced by another package-resolution request. The report does not need a remote package to exit zero, make a harmful change, or impersonate TypeScript. It also does not claim every npm release has identical prompting or install policy. [npm resolution][npm-resolution] [npm install branch][npm-install] [npm documentation][npm-docs]

For a later controlled fixture, a simple TypeScript assignment such as `export const answer: string = 42;` can provide an independently reviewable expected diagnostic. First establish that the prepared Yarn invocation reaches the pinned compiler and reports the error, then observe whether recurse reaches that same compiler. Do not use a successful npm launcher exit, a plan label, or a manufactured finding as the oracle. That experiment has not been run.

## Counterevidence and novelty boundary

- An explicit canonical build command in `nerd.md` suppresses this fallback. A package `build` script uses the detected manager. Those are working routing controls, not proof that the no-script branch preserves manager identity. [Precedence][precedence] [Detection][detect]
- In a conventional npm `node_modules` installation with the intended local `tsc` binary, npm's local-bin route can work. This report does not claim `npx tsc` is universally incorrect.
- Yarn can itself require an initial download if its version or package artifacts have not been prepared. The witnesses require a ready installation so that initial provisioning does not confound the manager switch.
- `processutil` remains a useful cancellation and bounded-pipe owner. Noninteractive execution is not the defect and should not be removed merely to trigger npm's prompt.
- There is no established false-green ratchet, remote compromise, data disclosure, observed installation, measured frequency or proven model-token cost in this review.

The bounded nonduplication screen checked the current audit inventory and queue, neighboring recurse reports, and repository searches for `npx`, `PnP`, Yarn and typecheck-related terms. The closest executable-selection audit concerns ambient `PATH` versus a declared environment: there the host selects the wrong executable for the *same program name*. Here it intentionally constructs a *different launcher and package-resolution route* before executable lookup, with no conflicting environment required. Correcting lookup-before-overlay alone would still run the chosen `npx`. [Prior executable selection][prior-executable]

The JavaScript workspace prerequisite report concerns bare source imports and graph closure; it explicitly leaves broader Yarn semantics for later qualification. Its witness never concerns execution of the TypeScript fallback. The spaced-diagnostic-path report assumes the intended TypeScript producer has already run. This report concerns obtaining that producer at all. The portable-crucible proposal already calls for capability-aware adapters; this is a concrete additional acceptance case, not a new general architecture. [Prior JS graph][prior-js] [Prior diagnostics][prior-diagnostics] [Portable proposal][portable]

The authored detector test checks pnpm script dispatch in one fixture, then checks fallback gate IDs in a separate fixture that declares npm/npx available. It does not combine the no-build-script fallback with a non-npm installation or verify the resolved compiler. Existing runner tests likewise do not establish this package-manager boundary. Tests were read, not run. [Detector fixtures][tests]

## Proposed contract and bounded correction

Update the owning gate/campaign specification before implementation. Retain the existing executive split and build on the shared invocation-contract work rather than add a model-selected command escape hatch.

1. **Preserve project authority.** Keep explicit canonical commands first. For detected fallback checks, carry selected package-manager identity, workspace/package context and installed-tool identity through admission and execution. Do not infer that finding a package-manager launcher proves a particular compiler is installed.
2. **Separate run from provision.** A fallback measurement may use an already installed, admitted compiler. If it is missing, unsupported or ambiguous, report that state. Provisioning or fetching packages is a separate effect with explicit authority and a pinned package identity; it must not arise merely from a binary-name guess.
3. **Use manager-specific semantics.** A bounded first slice can support an unambiguous root Yarn PnP project with a direct TypeScript dependency and prepared artifacts. A Yarn invocation that resolves the local binary is a candidate, not an instruction to substitute `pm run tsc` uniformly for every manager. Script/binary precedence, manager version, Bun/pnpm command behavior and no-install flags differ and require qualification.
4. **Keep uncertainty and evidence.** Record the effective launcher/vector, manager/version, dependency/compiler identity and resolution disposition needed to distinguish the real check from a provisioning or launcher failure. Retain unavailable/unverified reporting and do not manufacture a compiler finding for an unsupported resolution path.
5. **Preserve limits.** Do not mutate `PATH`, install npm as a workaround, change Yarn's linker, add a `build` script to the user's project, widen environment access, or silently fall back to a global compiler. Multiple/conflicting lockfiles and absent installation state should have explicit dispositions.

The minimal correction is at detected invocation construction and its availability contract. The same descriptor must reach planning, the real baseline and any acceptance handoff that reuses its command. A detector-only string assertion would not establish successful PnP execution or correct package identity.

## Costs, failure modes and fair evaluation, entirely unrun

Resolving the intended tool may require additional manifest/lock/install-state reads or one manager-aware probe. Account for subprocess startup, cache/loader reads, filesystem traversal, version capture and invalidation after dependency or configuration changes. Repeated per-node probes would be wasteful for a workspace-scoped check; reuse only under an identified unchanged install/configuration state. No model call is inherently needed for this deterministic selection.

A blanket “missing compiler means stop everything” policy can remove useful work unnecessarily. Conversely, accepting any executable called `tsc` weakens the contract. Measure both legitimate available-check acceptance and correct unsupported/missing dispositions. Manager shims that bootstrap themselves, stale PnP loaders, inaccessible caches, ambiguous locks, workspace-specific dependency visibility and script names shadowing binaries need explicit tests.

Proposed evaluation stages:

1. **Descriptor/admission fixtures:** npm, Yarn PnP, Yarn node-modules, pnpm and Bun; with/without `build`, canonical overrides and TypeScript; launcher present/absent; conflicting locks. Assert preserved authority, not only gate IDs.
2. **Prepared offline producer fixtures:** pin toolchain versions and artifacts. The PnP witness has no `node_modules`, ancestor/global compiler or npm cache escape hatch. Demonstrate the independently selected Yarn compiler path and version. Observe actual process invocation and package-resolution attempts for recurse. Use an offline failure or a controlled local registry fixture rather than fetching an arbitrary live package to prove the wrong request.
3. **Behavioral cases:** known valid and invalid TypeScript inputs through the actual baseline. Require the expected compiler identity and diagnostic, not merely an exit code. Add manager-native build-script and explicit-command controls that should remain unaffected.
4. **Effect/lifecycle cases:** non-TTY execution, disallowed network/install, cache miss, loader failure, cancellation, package-manager startup failure and output truncation. Missing capability must not cause an unrequested install, false pass, fake source finding or unbounded retry.
5. **Production composition:** CLI and chat ordinary recurse using a scripted executor/model only if needed to reach later attempts, real kernel and normal world-model/CodeDOM-enabled construction. Mission-mode evidence should receive the same correctly identified check without changing scope or promotion standards.

Compare the pinned implementation, a manager-aware installed-only adapter, and an explicit unsupported-capability fallback on the same frozen projects and budgets. Report intended-compiler reachability, diagnostic precision/recall, incorrect unavailable outcomes, unintended registry/cache effects, false passes, false source findings, startup/total wall time, subprocesses, host I/O and any downstream model work. Count unsupported/refused cases in the denominator. No performance gain or behavior pass is claimed here.

**Exit criterion:** a ready supported TypeScript installation can be measured through its actual project manager when no `build` script exists; an unavailable or unsupported installation is reported without substitution or implicit provisioning; valid explicit scripts and canonical commands retain their current precedence.

## Pinned sources

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/agents.md#L1-L24
[gate-contract]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/gates/gates.go#L1-L19
[mission-spec]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/Docs/architecture/campaign/MISSION-IMPROVEMENT.md#L217-L235
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/cmd/nerd/cmd_campaign_recurse.go#L253-L273
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/cmd/nerd/chat/campaign_recurse.go#L218-L241
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/campaign/recurse_cycle.go#L515-L563
[precedence]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/gates/detect.go#L15-L43
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/gates/detect.go#L175-L213
[split]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/gates/gates.go#L154-L195
[measure]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/campaign/recurse_cycle.go#L699-L785
[run]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/gates/run.go#L45-L99
[command]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/build/workspace_command.go#L29-L60
[process]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/processutil/command.go#L12-L49
[report]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/campaign/recurse_cycle.go#L171-L203
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/internal/gates/detect_test.go#L107-L135
[prior-executable]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/Docs/audits/gate-declared-path-executable-selection.md
[prior-js]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/Docs/audits/recurse-js-workspace-prerequisite-identity.md
[prior-diagnostics]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/Docs/audits/recurse-typescript-spaced-diagnostic-paths.md
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/2c90e06cf9b590491b40ab72985c636a13336665/Docs/audits/portable-recurse-deliberation-crucible.md
[yarn-pnp]: https://yarnpkg.com/features/pnp
[yarn-run]: https://yarnpkg.com/cli/run
[typescript-package]: https://github.com/microsoft/TypeScript/blob/v5.9.3/package.json#L1-L26
[npm-resolution]: https://github.com/npm/cli/blob/10ffa9f35c707fbf91733a07683a7ea96c38e416/workspaces/libnpmexec/lib/index.js#L98-L223
[npm-manifest]: https://github.com/npm/cli/blob/10ffa9f35c707fbf91733a07683a7ea96c38e416/workspaces/libnpmexec/lib/index.js#L22-L85
[npm-install]: https://github.com/npm/cli/blob/10ffa9f35c707fbf91733a07683a7ea96c38e416/workspaces/libnpmexec/lib/index.js#L243-L330
[npm-docs]: https://docs.npmjs.com/cli/v11/commands/npm-exec/
