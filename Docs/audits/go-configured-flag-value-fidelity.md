---
doc-class: deep-dive
subsystem: build-gates-recurse-invocation
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: 89b4bd542baad66e973955660182bd2e848f07e1
verification-method: static-source-inspection
---

# Dropped Go flag values can turn a whole-module gate into a root-only pass

## Finding and decision

The shared Go argument adapter treats every configured token as an independent flag. A value token without a leading `-` is discarded. Consequently, an otherwise accepted `build.go_flags` value of `["-tags", "auditfeature"]` injects only `-tags`. With the ordinary declared test command `go test ./...`, the actual invocation becomes `go test -tags ./...`. Go consumes `./...` as the tag value, receives no package arguments, and enters local-directory mode. A healthy root package can therefore pass while an independent failing subpackage is never tested. [Configuration shape][config] [Argument adapter][append] [Go flag parser][go-parse] [Tag setter][go-tags] [Test modes][go-modes]

This is **argument-boundary corruption before verification**, not a lost diagnostic or an exit-code bug. The process can honestly exit zero for the narrower work. The ordinary gate's `Result.Argv` also retains the pre-injection command, so it still presents `go test ./...` rather than the actual altered argument vector. [Command construction][workspace-command] [Gate execution and receipt][gate-run]

Preserve flag/value ownership when applying configuration. Either support documented split-valued flags as complete options, or reject unsupported forms before launching a process. Do not silently remove a token and permit a later caller argument to become its replacement value. Preserve the existing precedence of explicit caller options, typed execution, environment filtering, process ownership and policy-owned verdicts.

## Evidence boundary and architectural fit

Public main was pinned to **89b4bd542baad66e973955660182bd2e848f07e1**, committed November 4, 2026 at 08:50:25 UTC. The reviewed module declares Go 1.27.3; the official release source used below is pinned to **2aae1000ea1d4b28923b96db28acf4e3b99d6aff**. These pins establish source behavior, not the version installed on a particular user's machine. [Repository revision][main] [Module][module] [Go release][go-release]

Inspection covered root, public-repository, campaign and configuration guidance; campaign and build architecture; the source path below; authored fixtures; the audit inventory and neighboring reports. Architecture text was treated as intent and navigation, not proof that every old wiring claim remains current. An independent source review challenged the flag-consumption witness. No repository program, compiler, build, test, gate, model inference, benchmark, fixture or fault injection ran. No repository file, implementation, credential, configuration or setting was changed. All outcomes and evaluations below are conditional and unrun.

The repository assigns creative work to the model and executive decisions to Mangle. That split depends on the host faithfully reporting which obligation a successful process observed. A model warning about a suspicious command can help, but it does not repair argument admission for every caller. The correction belongs in the shared build adapter and its evidence contract, with policy consuming accurate observations. [Root guidance][guidance] [Configuration guidance][config-guidance] [Recurse contract][journey]

## 1. The production path loses an option's value

### Configuration is loaded, then transformed

`config.BuildConfig.GoFlags` is a string slice of additional Go build/test flags. Its JSON and YAML names are `go_flags`. The loader's configuration check has no `Build` or `GoFlags` validation at this pin. `WorkspaceUserConfig` loads the workspace configuration; `GoInvocation` selects the test or build environment and returns the result of `AppendGoFlags`. `loadBuildConfig` copies the configured slice into the effective build configuration. The loss is therefore downstream of configuration loading, rather than the earlier missing-profile wiring problem. [Configuration shape][config] [Configuration load][config-load] [Configuration check][config-check] [Invocation][invocation] [Configuration copy][config-copy]

`AppendGoFlags` first collects names of flag-looking tokens already in the caller's arguments. It then iterates the configured slice one token at a time:

- `goFlagName("-tags")` returns `-tags`, so that token is retained.
- `goFlagName("auditfeature")` returns the empty string, so that token is skipped.
- The retained configured tokens are inserted immediately after the Go subcommand and before all original arguments.

There is no representation of a flag plus its value and no arity check at this boundary. The function cannot return an error. Its supported-subcommand table includes `build`, `list`, `test` and `vet`, so recurse discovery and several checks share the same transformation. [Argument adapter][append] [Subcommands][subcommands]

The field does not state an equals-only restriction. More importantly, even an intentionally restricted configuration grammar would need to reject the pair: accepting it and executing a different command is not safe enforcement of that restriction. This report does not propose accepting arbitrary positional arguments or shell fragments as build flags.

### Ordinary recurse gates use this adapter

`gates.Detect` parses `nerd.md` commands, marks them workspace-scoped and lets a declared command replace detected gates of the same kind. A declaration of `commands.test: go test ./...` thus reaches a normal production test gate without requiring a custom executable or shell. [Detection and declarations][detect]

`gates.Command` chooses the expanded command vector and calls `build.WorkspaceCommand`. For the logical executable name `go`, that function calls `GoInvocation` and creates `exec.Cmd` with the transformed arguments. `gates.Run` executes that command through the shared process helper and determines pass from the actual process result and the gate's accepted exits. A normal zero exit sets `Passed=true`; `FindingsAll` returns no findings for a pass. [Command construction][workspace-command] [Gate execution][gate-run] [Finding admission][findings]

Recurse collects the workspace-scoped results during baseline measurement and stores them under their gate keys. Later changed attempts remeasure applicable workspace build/lint gates and the targeted or required test gate. Those consumers cannot recover a subpackage that the executable was never asked to inspect. [Baseline consumer][baseline] [Remeasurement consumer][remeasure]

## 2. Why the altered command can succeed

The official Go test driver registers the shared build flags, including `-tags`, and processes command arguments with `cmdflag.ParseOne`. For a known non-boolean flag without an equals value, that parser consumes the next token as the flag's value. It does not preserve the token as a package merely because it resembles `./...`. The tags setter splits a comma-separated string and stores nonempty entries; the shown implementation does not reject `./...` as a value. [Test flag registration][go-register] [Test argument processing][go-testflags] [Go flag parser][go-parse] [Tag setter][go-tags]

Go explicitly distinguishes local-directory testing, with no package arguments, from package-list testing. The former tests the current directory. Thus the adapter changes the meaning of the caller's existing package selector rather than merely omitting the configured tag. [Test modes][go-modes]

The parallel build command has the same option-value consumption rule. The primary finding does not depend on a build's output directory, JSON output, a failed parser, output truncation or test-result caching. [Standard flag parser][go-flag] [Go build entry][go-build]

## 3. Conditional witness: a green root beside a red subpackage

The following is a hypothetical fixture specification, not files created or commands run during this review.

Workspace layout:

```text
go.mod
root.go
root_test.go
broken/value.go
broken/value_test.go
nerd.md
.nerd/config.json
```

- The root module has a valid module path and a compatible Go version.
- `root.go` declares a simple library package. `root_test.go` contains a genuine passing test. Neither imports `broken`.
- `broken/value.go` is an ordinary untagged package with a function body referring to an undefined identifier. Its package and import declarations are otherwise valid.
- `broken/value_test.go` is also ordinary untagged source. No import from the root reaches this package.
- The otherwise valid workspace configuration has the fragment `"build": {"go_flags": ["-tags", "auditfeature"]}`.
- The valid project document declares `commands.test: go test ./...`. For the build counterpart it declares `commands.build: go build ./...` as well.

Assume the configured profile loads successfully, the logical `go` executable resolves to the inspected compatible toolchain, ordinary root checks pass, source and configuration stay unchanged, and no independent gate or startup check blocks the measurement. The command contains no explicit `-tags` that would override the configuration. There is no shell, permission change, concurrent writer, malformed report, missing executable or model-generated candidate in this witness.

The decisive vectors are:

| Boundary | Intended or observed argument vector |
|---|---|
| Declared test | `["go", "test", "./..."]` |
| Configured additional option | `["-tags", "auditfeature"]` |
| Faithful composition | `["go", "test", "-tags", "auditfeature", "./..."]` |
| Current composition | `["go", "test", "-tags", "./..."]` |
| Go interpretation of current composition | Custom tag value `./...`; no package arguments; current-directory test mode |
| Ordinary gate receipt's `Argv` | `["go", "test", "./..."]` |

With faithful composition, the whole-module selection includes `broken`, whose compilation fails. With current composition, the root's real test can run and pass while `broken` is absent from the selected package set. The gate then records a pass and no findings. This is a conditional false assurance about the declared domain, despite a genuine successful root test. [Adapter][append] [Parser][go-parse] [Modes][go-modes] [Gate receipt][gate-run]

The corresponding declared build becomes `["go", "build", "-tags", "./..."]`. Go's module loader normalizes an empty package-pattern list to `.`. A healthy root library can compile without compiling the unimported broken package. This witness does not use either build adapter's temporary-directory output path. [Module pattern admission][go-module-patterns] [Empty-pattern default][go-default-patterns]

### Recurse discovery does not necessarily prevent the witness

`runGoList` starts with `list`, `-e`, `-json`, `./...`. Under the same configured pair, the injected `-tags` consumes `-e`, leaving the JSON mode and whole-module package selector in place. Losing error tolerance is itself undesirable, but the chosen fixture's undefined identifier is in a function body, not in package/import metadata. Go's source-header reader parses imports rather than type-checking that body. A successful package listing can therefore still discover the root and `broken` nodes, while the simpler declared build/test commands inspect only the root. [Discovery invocation][go-discovery] [Header reader][go-header]

This is a bounded reachability argument, not a guarantee that every broken repository reaches measurement. Package-loading errors, a different command template or independent startup validation may stop a particular run. The narrower gate-level failure stands without claiming an entire unattended campaign completes or retains a bad commit.

### Controls that distinguish the cause

1. Use `go_flags: ["-tags=auditfeature"]`: the value remains attached, and `./...` remains a package selector.
2. Keep split-form flags in the **declared command itself**, with no configured duplicate: those original arguments are not filtered by this loop, so the pair can survive. The defect is the configured-token path.
3. Put an explicit `-tags=other` in the caller's argv: existing-name precedence skips the configured `-tags`, so this particular dangling-option witness does not occur.
4. Import `broken` from the root: the root's dependency closure can expose the defect despite narrowed selection. That is incidental coverage, not preservation of the requested domain.
5. Add a normal flag immediately after `test`, such as `-count=1`: the dangling `-tags` may consume that flag instead of the package selector. The default detected node test is specifically `go test -count=1 -cover {pkg}`, so its package selector survives this particular consumption and it can still catch the broken package. Do not assume identical behavior for every template or equate a declared gate pass with overall recurse success. The intended custom tag is still lost, but other failures or omissions require their own trace. [Default detected gates][detect-defaults]

## 4. The receipt also needs the executed vector

`gates.Command` returns the original expanded `argv`, not `cmd.Args`. `Run` copies that value into `Result.Argv` before executing the transformed command. Its field comment describes this as the command as run. In the witness, the result's command still shows the broad selection and omits the injected dangling flag. [Receipt source][gate-run]

This mismatch is not required to cause the package omission, but it weakens diagnosis and the grounding of later consumers. A recurse attempt derives its check from the reporting result, and mission check sidecars project the same result argv. Retaining the declared invocation is useful; label it separately from the effective executable and arguments. Neither is a reason to log raw environment values or credentials. [Attempt handoff][attempt] [Mission sidecars][sidecars]

## 5. Distinctness from existing work

The current audit inventory and direct neighboring reports were checked. A bounded repository search for `AppendGoFlags` and `go_flags` returned the shared helper, wiring and tests, and the nearby reports below, but no report of discarded configured value tokens. This is not an exhaustive novelty claim across every historical branch or discussion. [Audit inventory][queue]

- **Campaign checkpoint build profile:** the earlier defect bypassed the shared invocation helper and consequently omitted a profile. This finding occurs inside that helper after the profile is loaded. Reusing it is necessary but not sufficient. [Checkpoint report][prior-profile]
- **Run-lint effective build-tag receipt:** that report concerns union versus replacement semantics when summarizing environment and command-line tags. Its witness uses intact equals-form options, and its proposed controls already include separate-argument forms and package boundaries. Here the newly isolated mechanism changes the command itself before execution; accurate tag summarization alone cannot restore its lost value or package selector. [Tag receipt report][prior-tags]
- **Explicit package-selector fidelity:** that report traces an explicit `.` becoming an omitted selection in typed package normalization. The present witness supplies `./...` directly through an ordinary declared gate; it survives declaration parsing and is then consumed as an option value. [Selector report][prior-selector]
- **Go build directory-output coverage:** that report concerns `-o <directory>` changing the compiler's scheduled roots. This witness has no output flag or temporary directory and also affects `go test`. [Build-output report][prior-output]
- **Recurse test-census and no-tests evidence:** the root really has a test and can execute it. Counting real tests or preserving known empty collection does not establish that the intended subpackages were selected. [Test-census report][prior-census] [No-tests report][prior-empty]

The authored `AppendGoFlags` fixtures check equals-form and boolean options, explicit-name precedence, unsupported subcommands and empty configuration. The shared invocation fixture checks an intact `-mod=mod` option. Those are useful controls, but the inspected fixtures do not assert preservation or rejection of a configured split-valued pair. Tests were read, not run. [Adapter fixtures][tests] [Invocation fixture][invocation-test]

## 6. Repair contracts

### Configuration and argument admission

Choose and document one grammar for `build.go_flags`:

- A token-vector contract can support known non-boolean flags with separate values, preserving the option and its value as one unit during precedence handling.
- A bounded equals-form contract can require values to be attached and reject split-valued input before execution. Explain the unsupported input without silently substituting defaults or dropping tokens.

In either design, boolean flags, value-bearing flags, empty values, repeated flags, `--`, test-binary tails and unsupported subcommands need explicit treatment. Do not infer arity from whether the next token starts with a hyphen: Go values can themselves start with one. Do not turn validation into another permissive shell parser. Preserve configured order where the tool's repeated-option semantics depend on it.

If validation introduces an error, that error must reach the caller before process creation. `WorkspaceUserConfig` currently logs load errors and returns nil, so putting a new validator only there would risk running an unconfigured command unless that fallback contract also changes. Specify the fail-closed propagation, rather than merely adding a validation function. Missing-file defaults remain a separate legitimate state. [Loader fallback][invocation]

### Conservation of the caller's work

For an admitted composition, prove that adding configuration cannot change which original tokens are interpreted as package selectors, unless the owner explicitly requested that change. This obligation is stronger than checking that the token `./...` still appears somewhere in `cmd.Args`: the broken vector retains those bytes while changing their syntactic role.

Represent declared options, admitted options and the final command distinctly. Use the effective execution vector for process identity and receipts, while retaining declared intent for explanation. Continue keeping sensitive environment values out of persistent task or mission records.

### Executive ownership and failures

Host code owns grammar validation, faithful composition and observation. Mangle remains responsible for the gate/ratchet verdict over the admitted evidence. Unknown configuration or an unproven selection must not become a passing obligation simply because a narrower command exits zero. Do not repair this by expanding the model's shell access, permitting a model to waive a gate, or weakening existing guards.

## 7. Costs and failure modes

The present helper is linear in the number of caller and configured tokens, with a set of names and a new argument slice. A structured bounded parser can retain that order of work, but requires an option-arity/semantics contract that follows supported Go versions. An equals-only admission contract is smaller but can reject configurations users reasonably expected to work, so migration and clear errors are part of its cost.

Corrected selection can run more packages and expose genuine errors. That additional compilation/test cost is fulfillment of the requested work, not an efficiency regression to hide. Evaluate latency against equal verification domains. Adding another build or test merely to compensate would add tool cost without fixing all other users of the shared helper.

Failure risks include incorrectly pairing boolean options, mishandling values beginning with `-`, dropping a value when an entire configured option is overridden, changing repeated-option precedence, mutating an input slice, and silently falling back after validation errors. Non-Go commands, existing explicit options, environment filtering and subprocess cancellation must retain their current independent contracts.

## 8. Proposed evaluation, entirely unrun

Start with deterministic argument admission and the real production gate path. Keep fixtures tiny enough that actual selected packages can be independently identified.

| Case | Required observation |
|---|---|
| Split `-tags` plus `go test ./...`, healthy root and broken unimported subpackage | Preserve the intended pair and fail for the subpackage, or reject configuration before starting. Never claim a whole-module pass from root-only execution. |
| Equals-form control | Same intended selection with no new rejection. |
| Explicit caller `-tags` overriding configured split option | Drop the complete overridden option, not just its name or a detached value. |
| Multiple caller packages | No package becomes an option value and no selector disappears. |
| Root has no tests | Keep absence-of-tests semantics separate from package selection. |
| Value begins with `-`, boolean option, empty value and repeated valued option | Preserve or explicitly reject according to the declared grammar and Go semantics. |
| Unknown flag, trailing orphan value, `--`, or unsupported subcommand | No silent partial application; no accidental shell or test-binary argument admission. |
| Build, list, vet and ordinary declared test routes | Each consumer receives a valid admitted vector; discovery and checking use the same intended build profile. |
| Configuration-read/validation failure | No unintended unconfigured fallback after the new failure boundary. |
| Gate receipt and mission sidecar | Declared and executed argv are distinguishable and the latter matches the launched process. |
| Cancellation, process-start failure, forbidden write and independent failing gate | Existing controls remain effective. |

Use the actual `Detect` → `WorkspaceCommand`/`GoInvocation` → `Run` path, then the recurse baseline consumer. A helper-only vector test will not establish that production loads the same profile or records the same command. Use an independent package/test witness, rather than counting the same parser's output as its own oracle. Do not require a paid model to establish this deterministic defect.

For a later, separately authorized system comparison, hold the source fixtures, toolchain, configuration, required package domain, gate declarations and budgets fixed. Include working equals-form configurations and explicit overrides, not only failing examples. Report false domain passes, false rejections, selected-package agreement, parser/admission errors, actual process count, wall time, host CPU/memory, retained receipt bytes, and recovery effort. Count rejected and failed attempts in denominators. Only after deterministic parity is established should model-task outcomes be measured; no quality or performance improvement is claimed here.

## Source register

[main]: https://github.com/theRebelliousNerd/codenerd/commit/89b4bd542baad66e973955660182bd2e848f07e1
[module]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/go.mod#L1-L3
[go-release]: https://github.com/golang/go/commit/2aae1000ea1d4b28923b96db28acf4e3b99d6aff
[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/agents.md#L1-L63
[config-guidance]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/config/agents.md#L1-L19
[journey]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/journeys/10-forever-loop.md#L1-L62
[config]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/config/build.go#L3-L20
[config-load]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/config/user_config.go#L618-L676
[config-check]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/config/check.go#L112-L496
[invocation]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/invocation.go#L12-L86
[config-copy]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/env.go#L494-L507
[append]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/env.go#L299-L358
[subcommands]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/env.go#L57-L68
[detect]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/gates/detect.go#L15-L71
[detect-defaults]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/gates/detect.go#L125-L132
[workspace-command]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/workspace_command.go#L30-L60
[gate-run]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/gates/run.go#L17-L99
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/gates/findings.go#L109-L121
[baseline]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/campaign/recurse_cycle.go#L700-L750
[remeasure]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/campaign/recurse_cycle.go#L1324-L1352
[go-register]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/testflag.go#L32-L37
[go-testflags]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/testflag.go#L225-L342
[go-parse]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/cmdflag/flag.go#L53-L117
[go-tags]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/work/build.go#L353-L392
[go-modes]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/test/test.go#L95-L118
[go-flag]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/flag/flag.go#L1075-L1146
[go-build]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/work/build.go#L461-L479
[go-module-patterns]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/modload/load.go#L234-L250
[go-default-patterns]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/search/search.go#L474-L481
[go-discovery]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/campaign/recurse_workspace.go#L368-L420
[go-header]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/go/build/read.go#L265-L314
[attempt]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/campaign/recurse_cycle.go#L1152-L1180
[sidecars]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/campaign/recurse_mission_storage.go#L80-L149
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/ASTRA_QUEUE.md
[prior-profile]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/campaign-checkpoint-build-profile.md
[prior-tags]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/run-lint-effective-build-tag-receipt.md
[prior-selector]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/explicit-package-selector-fidelity.md
[prior-output]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/go-build-directory-output-coverage.md
[prior-census]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/recurse-test-census-semantic-admission.md
[prior-empty]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/Docs/audits/recurse-no-tests-evidence-continuity.md
[tests]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/env_features_test.go#L228-L282
[invocation-test]: https://github.com/theRebelliousNerd/codenerd/blob/89b4bd542baad66e973955660182bd2e848f07e1/internal/build/invocation_test.go#L1-L86
