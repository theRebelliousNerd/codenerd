---
doc-class: deep-dive
subsystem: campaign-recurse-portability
implementation-status: proposed
last-verified: 2026-11-03
verified-against: 1e902faad7974b133aac17920c125c2f6187cf41
verification-method: static-source-inspection
supersedes: []
---

# Recurse misses Go workspace modules without a root go.mod

## Finding and evidence boundary

Acquire the selected Go workspace's module inventory before deriving recurse's package graph and detected checks. A standard multi-module layout can have a root `go.work` and module-local `go.mod` files, with no module at the repository root. Current recurse enters Go discovery only when the root itself contains `go.mod`. Its other source collectors do not acquire Go files or module manifests. A Go-only repository in that layout therefore becomes the generic, language-less `.` node, and gate detection contributes no Go checks. [Graph entry][derive] [Source collector][collect] [Gate detection][detect]

This loses package visits, dependency ordering and named subsystem focus. The normal improvement guard then declines work without an applicable test. Adding an explicit workspace test whose program is `go` can make the command runnable but still does not give the fallback node its missing Go language, so it does not repair improvement admission. The required change is a shared module/package capability observation, not weaker test admission or an invented root module. [Focus][focus] [Improvement admission][improve] [Declared commands][declared] [Language applicability][applies]

**This is a portability and work-reachability gap, not a false-green result.** The CLI plan explicitly reports an empty gate set as unverified. It does not claim that absent checks passed. No incorrect retained commit, security bypass, runtime reproduction or measured performance effect is established. [Plan disclosure][plan]

All CodeNERD implementation references are pinned to public main [`1e902faad7974b133aac17920c125c2f6187cf41`][main], committed at 06:34:12 UTC and rechecked unchanged at approximately 06:45 UTC on November 3, 2026. This review read public source, guidance, architecture, audit records, authored tests and official Go documentation. No repository program, model, test, build, gate, benchmark or application path was executed. The fixtures below are proposals and conditional source traces. No implementation was made.

## The supported-language boundary

The recurse journey intends workspace-derived, dependency-first work rather than a fixed graph of CodeNERD's own packages. Root guidance assigns observations and typed execution to the host and executive decisions to Mangle. The portable-crucible proposal already calls for explicit adapter capabilities and honest degradation. This report identifies a concrete standard Go layout that currently degrades to the generic fallback, rather than proposing a new scheduler. [Journey][journey] [Guidance][guidance] [Portable proposal][portable]

Go's official tutorial places `go.work` above module directories and executes a selected package from that workspace directory. The module reference defines workspace membership through `use` directives and permits multiple main modules. A root `go.mod` is consequently not a prerequisite for a valid Go workspace. These sources establish the layout's meaning; they are not evidence that CodeNERD executed it. [Go workspace tutorial](https://go.dev/doc/tutorial/workspaces) [Go workspace reference](https://go.dev/ref/mod#workspaces)

The current CLI help names root `go.mod` among detection markers. That accurately describes the detector's narrow behavior. The gap is the distance between that implementation and portable Go workspace support, not a claim that the help promises every build system or that all Go repositories fail. [CLI contract][cli-contract]

## Producer and consumer trace

### Root-marker admission loses all Go packages

`DeriveWorkspaceDAG` canonicalizes the selected root, refreshes shared workspace membership and collects admitted sources. It calls `scanGoPackages` only inside a root `go.mod` existence check. The collector groups Python files, JS/TS files and Cargo manifests; it does not gather `.go`, `go.mod` or `go.work` for another Go discovery path. With only Go modules below a root `go.work`, none of the language scanners adds a node. The fallback adds `.` with an empty language. SCC projection gives it `Paths=["."]`, no language entries and no prerequisites. The closing wiring, review and bench stages are then appended. [Derivation][derive] [Collection][collect] [Node creation][node-create] [Projection][projection] [Closing stages][close]

No `go list` failure is required: the Go subprocess is never called for this layout. The existing producer can collect production, in-package-test and external-test imports once invoked, but those repaired fields cannot help an invocation excluded before package acquisition. [Go producer][go-producer]

### Focus and ordering consume the incomplete inventory faithfully

Planning and each live pass derive this graph. `FilterDAG` refuses a subsystem absent from it; it cannot expand an unknown module package into prerequisites. An owner asking to focus the real package `a` therefore receives an unknown-subsystem error instead of a focused visit. Without focus, the kernel sees only the fallback and closing stages. Its structural validation and ordering can succeed because that supplied graph is internally sound. They do not prove source inventory completeness. [Plan route][plan-route] [Live pass][pass] [Focus][focus] [Graph facts][graph-facts]

The CLI calls the shared cycle driver with the selected workspace. Chat obtains known subsystem IDs from `DeriveWorkspaceDAG` and calls that same driver. This boundary is reachable through both ordinary entry paths, subject to their independent boot, Git, permission and mission admission. [CLI entry][cli] [Chat inventory][chat-inventory] [Chat entry][chat]

### Detection has the same root-only assumption

`gates.detect` emits Go build, vet and test gates only when root `go.mod` exists. It neither reads `go.work` nor searches its member manifests. Without project declarations or other language markers, the resulting set has zero runnable gates and zero unavailable gates. A Go toolchain being installed does not change this: availability is evaluated only for gates that were constructed. [Detection][detect] [Availability split][split]

The plan renders the generic nodes and its explicit no-gates warning. In the live driver, workspace measurement returns without checks when the set is empty, and `nodeGates` produces no own gates. There are no ordinary measured gate findings to drive fixes. An ordinary improvement either has no measurable angle or is skipped because no runnable test can verify the node. Cross-cutting stages skip that improvement step. These controls prevent unsupported adoption and should remain intact. [Plan][plan] [Workspace measurement][measurement] [Node gates][node-gates] [Visit][visit] [Improvement guard][improve]

### An explicit direct Go test repairs only detection

Consider an otherwise valid `nerd.md` declaring `commands.test: go test ./a/... ./z/...`. `fromNerdMD` creates a workspace-scoped Test gate and `languageOf` labels it `go`. Workspace baseline measurement does run declared workspace gates independently of a node's language. Thus it would be incorrect to say owner-declared checks never execute in this layout. [Declarations][declared] [Language inference][language] [Workspace measurement][measurement]

The improvement consumer asks for an own-node Test or an applicable workspace Test. An empty node language list does not match `go`, so the explicit direct command still cannot certify an ordinary improvement of fallback `.` through that admission. A declared node-scoped Go gate is similarly filtered from `nodeGates`. The source has no command-declaration-to-DAG-language reconciliation. [Applicability][applies] [Node selection][node-gates] [Improvement admission][improve]

Unknown programs such as `make` or a script are intentionally language-neutral and can pass this particular applicability test. A suitable authorized wrapper can therefore permit generic whole-tree work, subject to metrics and all other guards. It still does not restore `a` and `z` as nodes, their prerequisite edge, or named focus. Do not turn that escape hatch into a claim that no workaround exists, or recommend disguising Go as a wrapper instead of fixing acquisition. [Language inference][language]

## Minimal conditional witness

Use an admitted, clean Git repository whose root contains no `go.mod`, no root Go files and no marker or sources for another supported language. Go is installed. The intended active workspace is the root's readable `go.work`; no ambient or configured override selects a different workspace.

- `go.work`: a valid Go version and `use` entries for `./a` and `./z`.
- `a/go.mod`: module `example.com/a` at a compatible Go version.
- `a/a.go`: package `a`, importing `example.com/z` and using an exported symbol.
- `a/a_test.go`: an ordinary test of package `a`.
- `z/go.mod`: module `example.com/z` at a compatible Go version.
- `z/z.go` and `z/z_test.go`: the exported implementation and an ordinary test.
- All these files are admitted by repository membership, with no relevant ignore or forbidden-path exclusion.

These are generic proposed fixture names, not a description of a private project. The initial case has no `nerd.md` gate declarations and no optional error-free `--gates` override.

From the inspected code:

1. Go package acquisition is skipped because root `go.mod` is absent.
2. The graph contains `.`, `wiring`, `review` and `bench`, not `a` and `z`.
3. It has no observed a-to-z prerequisite relation or package-level visits.
4. Focusing `a` fails as an unknown subsystem.
5. Native gate detection returns no Go gates, rather than marking installed Go unavailable.
6. The CLI plan discloses no checks. In an otherwise admitted live run, ordinary improvements cannot pass test admission.

Add only the direct declared test described above as the second case. The test becomes a runnable workspace gate, while graph identity and improvement applicability remain unchanged. Use a language-neutral wrapper as a countercontrol: it may admit generic fallback work but must not be credited with recovering package ordering.

## Existing controls and nonduplication

The bounded screen read the current audit inventory, queue, relevant recurse reports and returned code-search matches. Exact `go.work` search under `Docs/audits` returned three unrelated files; `multi-module` returned two; the targeted campaign and gate searches returned no `go.work` hit. GitHub search was indexed at `9de30f80`, while the important source files were read directly at the newer pinned main. This is a bounded novelty check, not a claim about every private discussion or historical branch.

- **Go test dependency closure:** its omission occurs after package discovery; current source now carries `Imports`, `TestImports` and `XTestImports`. Here package discovery is not entered. [Prior Go audit][prior-go]
- **Python from-import and Cargo inheritance:** those reports concern an absent edge between acquired nodes. Here the module packages and their language are all absent. The Cargo report already records its landed repair. [Python audit][prior-python] [Cargo audit][prior-cargo]
- **Gate language domain:** concerns lost per-path language membership in an acquired collapsed polyglot node. This witness has one language, no cycle and no SCC ambiguity. [Language-domain audit][prior-language]
- **Workspace-test scheduling:** its landed repair schedules fresh applicable workspace tests. The rootless Go fallback cannot satisfy that applicability relation. Preserve the fix. [Workspace-test audit][prior-workspace]
- **Retrieval Go module identity:** concerns parsing a root module declaration for Tier 3 retrieval and explicitly excludes multi-module workspaces. It does not acquire recurse's graph or checks. [Retrieval audit][prior-retrieval]

Existing Go graph and detection fixtures use root `go.mod`; a separate graph fixture deliberately checks the single-node fallback for a README-only workspace. These are useful controls, not coverage for an active multi-module Go workspace. Authored tests were inspected, not executed. [Graph fixture][graph-tests] [Fallback fixture][fallback-test] [Gate fixture][gate-tests]

Retain canonical roots, membership and project exclusions, structural graph validation, Mangle ordering, declared-command precedence, test admission, current write guards, cancellation, compensation and the keep/revert ratchet. A `go.work` member outside the selected repository must not become an authorized writable node merely because the toolchain can see it.

## Proposed capability contract

1. **Resolve the effective build context.** Observe the selected workspace/module mode, admitted manifest identity, toolchain/profile and effective configuration. Account for an explicit `GOWORK` value, `off`, ancestor discovery and malformed/missing inputs. Record external workspace references as boundaries unless their inclusion is separately authorized. The official Go rules make environment and invocation directory part of this context. [Go workspace selection](https://go.dev/ref/mod#workspaces)
2. **Acquire module inventory before packages.** Use a grammar-aware `go.work` reader or an authorized, bounded Go metadata query. Canonicalize and admit each local module directory. Do not treat every nested `go.mod` as active or assume an unrelated module belongs to the selected workspace. Distinguish complete acquisition, unsupported context and failure.
3. **Collect packages for the admitted modules.** Preserve package identity and the production/test import relationships across them. A change to the root existence condition alone is insufficient: the current producer still issues one `go list -e -json ./...` from the selected root. Go's current package loader rejects a local wildcard whose directory prefix lies outside its active modules, apart from the standard-library and module-cache exceptions. The proposed root sits above `a` and `z`, not inside either. Define module-aware package selectors or per-module collection explicitly, with cancellation, output limits and no unapproved dependency/toolchain acquisition. [Current invocation][go-producer] [Go local-pattern admission](https://go.dev/src/cmd/go/internal/modload/load.go#L518)
4. **Use the same inventory for checks.** Derive declared detection scope and per-node Go language from that observed context. Keep the owner's command overrides authoritative. Record the package domain a fresh check covers; a Go language label alone is not a complete execution witness.
5. **Keep uncertainty explicit.** Recognized Go workspace mode with unresolved members must not become an apparently complete generic empty-language graph. A generic plan can remain available with a named limitation. Mangle should decide which obligations are admissible from host-produced facts; the host must not relax the tested requirement.
6. **Bind and refresh identity.** Refresh when workspace membership, `go.work`, member manifests, package configuration or relevant source changes. Preserve existing per-pass discovery and invalidate cached results rather than reusing a previous module inventory under a new context.

The target result is package-aware recurse over admitted local modules, not automatic reach into all paths named by a manifest. Broader arbitrary monorepo discovery and unsupported build systems remain separate capabilities.

## Unrun evaluation and costs

Start with independently specified package inventories and edges. Exercise actual graph discovery, gate detection, plan output, focus filtering and normal CLI/chat admission before claiming the production path is fixed.

| Proposed case | Required observation |
|---|---|
| Root `go.work`, two local modules, no root `go.mod` | Both package nodes, Go language and cross-module prerequisite edge are acquired. |
| Focus dependent `a` | The local prerequisite `z` is included and ordered first. |
| Direct declared Go test | Declared argv/environment survive; the correct package nodes can satisfy test applicability. |
| Neutral wrapper test | Preserve generic command support without inventing package coverage. |
| Existing single root module | Preserve current graph, tests and detection behavior. |
| Root module plus additional selected modules | Explicitly account for every admitted module; distinguish unselected nested modules. |
| `GOWORK=off`, alternate or ancestor workspace | Inventory and command execution agree on the declared context. |
| Missing, malformed, forbidden or outside-root member | Visible incomplete/blocked outcome; no unauthorized node. |
| Cross-module test-only imports | Preserve the recently repaired test prerequisite closure. |
| Mixed-language repository | Go acquisition does not erase other language nodes or broaden their gates. |
| Tool absence, metadata cancellation or bounded-output failure | No complete-empty success or passing check. |
| Manifest edit and resumed pass | Refresh identity and admission without replaying stale scope. |
| No usable test gate | Retain the current visible improvement hold. |

Compare module/package recall, missing and excess edges, focused membership, actual visits, command package coverage and false applicability. Charge additional manifest reads, subprocesses, package-loading work, memory, output bytes and cache invalidation. Generic fallback may do less work precisely because it omitted the requested structure; that is not a valid performance win. Model/tool savings from better order remain unmeasured.

**Completion condition:** a standard rootless Go workspace can be planned and swept at package granularity, dependency focus includes its admitted local prerequisites, and fresh verification refers to the same workspace/module context. Unavailable acquisition or verification stays visible and cannot authorize a kept improvement.

## Pinned references

[main]: https://github.com/theRebelliousNerd/codenerd/commit/1e902faad7974b133aac17920c125c2f6187cf41
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/agents.md
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/journeys/10-forever-loop.md#L3-L15
[derive]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace.go#L64-L100
[collect]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace.go#L264-L317
[node-create]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace.go#L119-L129
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace.go#L199-L234
[close]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace.go#L237-L258
[go-producer]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace.go#L367-L450
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/gates/detect.go#L124-L156
[declared]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/gates/detect.go#L19-L91
[language]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/gates/detect.go#L93-L121
[applies]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/gates/gates.go#L115-L128
[split]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/gates/gates.go#L166-L196
[focus]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_dag.go#L35-L77
[plan-route]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_plan.go#L38-L67
[graph-facts]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_plan.go#L82-L133
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_cycle.go#L515-L570
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_cycle.go#L692-L745
[node-gates]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_cycle.go#L993-L1005
[visit]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_cycle.go#L1052-L1107
[improve]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_cycle.go#L1181-L1224
[cli-contract]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/cmd/nerd/cmd_campaign_recurse.go#L51-L91
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/cmd/nerd/cmd_campaign_recurse.go#L248-L263
[plan]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/cmd/nerd/cmd_campaign_recurse.go#L294-L345
[chat-inventory]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/cmd/nerd/chat/campaign_recurse.go#L143-L154
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/cmd/nerd/chat/campaign_recurse.go#L218-L228
[graph-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace_test.go#L79-L110
[fallback-test]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/campaign/recurse_workspace_test.go#L204-L215
[gate-tests]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/internal/gates/detect_test.go#L76-L90
[portable]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/portable-recurse-deliberation-crucible.md
[prior-go]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/recurse-go-test-dependency-closure.md
[prior-python]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/recurse-python-from-import-prerequisites.md
[prior-cargo]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/recurse-cargo-inherited-dependency-closure.md
[prior-language]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/recurse-gate-language-domain.md
[prior-workspace]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/recurse-workspace-test-witness.md
[prior-retrieval]: https://github.com/theRebelliousNerd/codenerd/blob/1e902faad7974b133aac17920c125c2f6187cf41/Docs/audits/retrieval-go-module-directive-identity.md
