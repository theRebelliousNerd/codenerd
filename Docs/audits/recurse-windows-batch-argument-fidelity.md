---
doc-class: deep-dive
subsystem: campaign-recurse-windows-command-transport
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: fea75fdfd13053ad2b3626bc5742d2f518630c8e
verification-method: static-source-inspection
---

# Recurse batch-backed gates can check a different literal subject on Windows

## Finding and decision

A declared gate promises shell-like tokenization followed by direct, literal argument execution. On Windows, a batch-backed executable can introduce a second interpreter boundary after that tokenization. The shared command builder sends ordinary Go `Args`, without selecting a batch argument transport or rejecting unsupported literal inputs. Windows batch dispatch can consequently expand a percent-delimited variable inside a node path even though the gate declaration did not request shell expansion. The recorded `Result.Argv` still contains the unexpanded path. [Declaration contract][schema] [Builder][builder] [Run and recorded argv][run]

This is a **conditional subject/argument fidelity defect**, not a claim that every Windows batch invocation fails to launch. The decisive case uses the intended, fixed checker and an unchanged declared environment. It does not depend on PATH selecting the wrong tool, missing configuration during acceptance, a cache hit, diagnostic parsing, or a malicious executable.

Specify the execution kind and literal-argument contract at the common process-construction boundary. Native executable argv and shell program text are different contracts. Preserve typed checks, declaration authority, environment filtering, bounded capture, cancellation and policy-owned verdicts. Do not make every gate a free-form shell command as a workaround.

**Evidence boundary:** public source inspection at the pin above. No codeNERD program, test, build, gate, generator, model inference, benchmark or Windows process was executed. No implementation, repository, credentials or settings were changed. Witnesses and evaluations below are unrun. Root and subsystem guidance, campaign/build architecture, current implementation, relevant tests as text and neighboring audits were read. The older architecture source maps were treated as intent/context, not as runtime proof. [Root guidance][guidance] [Architecture standard][architecture] [Campaign corpus][campaign] [Build corpus][buildspec]

## 1. The reachable contract crossing

The project schema says a gate's command is split like shell input but not executed through a shell. It defines `{node}` as the workspace-relative node directory. Validation requires a node-scoped gate to name a supported placeholder; it does not establish an executable-format-specific encoding contract. [Schema][schema] [Validation][validation]

The route is explicit:

1. `fromNerdMD` uses `shellquote.Split` and retains argv, scope and the declared environment. [Declaration][declaration]
2. `Gate.ForNode` inserts the literal directory into the relevant argument. It does not request environment-variable substitution. [Expansion][expansion]
3. `gates.Command` calls `build.WorkspaceCommand`. Except for its Go-specific branch, the builder calls `exec.CommandContext(prog, argv[1:]...)`, then sets directory and environment. [Command and runner][run] [Builder][builder]
4. `processutil.Run` supplies noninteractive input, bounded pipe waiting and process-tree cancellation. Its Windows startup helper does not install a `SysProcAttr.CmdLine` or otherwise encode batch arguments before `cmd.Start`. [Process helper][process] [Windows startup][windows]
5. The result retains the requested argv and classifies the process exit. It has no report of the ultimate checker's received argument vector. [Result][run]

CLI and chat both reach this shared measurement path. Recurse retains each invocation and, in mission mode, stores its command, output and status; these are useful records, but the command field remains the pre-interpreter argv. [CLI][cli] [Chat][chat] [Measurement][measurement] [Mission observations][sidecar]

### The platform distinction is documented, not guessed from POSIX

The repository's module directive names Go 1.27.3. That release's `os/exec.Command` documentation explicitly distinguishes ordinary Windows argument quoting from the parsing used by `cmd.exe` and batch files. Its Windows syscall source constructs the command line with `makeCmdLine` when no explicit `CmdLine` is provided. The argument encoder treats quotes, backslashes, spaces and tabs, not percent-variable references. [Module][gomod] [Pinned Go API][goapi] [Pinned Go encoder][goencoder] [Pinned Go launch][golaunch]

A Go upstream change explicitly documents implicit Windows batch dispatch through `cmd.exe /c`. That change fixes executable-path selection, not a general literal-argument protocol: its remaining arguments still pass through the ordinary argument encoder. This report fixes the executable path independently and does not rely on the upstream path-selection bug. [Go batch-dispatch change][gobatch]

Microsoft documents percent-delimited environment-variable substitution in command lines and scripts. The implication here is limited: an ordinary Args element containing such a reference is not necessarily opaque at this execution boundary. [Command interpreter semantics][cmd]

Do not infer that all batch scripts are unsupported, that a successful `LookPath` guarantees literal argument fidelity, or that the Go package's general no-shell description eliminates operating-system-specific script dispatch.

## 2. Conditional witness: the right checker receives the wrong node

Qualify this fixture on native Windows with the repository's supported Go toolchain before treating it as a reproduction. Use a short absolute, space-free path to a fixed `.cmd` checker launcher, outside candidate write scope. Keep its actual native checker immutable and network-free. No program with the same name, PATH override or unusual executable search is required.

The admitted workspace contains two JS source directories:

- `part%CHECK_LABEL%`: the intended node, containing a synthetic failing source condition.
- `partcore`: a different, healthy node.

The source scanner creates JS/TS nodes from source-file directories. A percent-bearing directory requires no import-language identifier and survives this directory projection. Ordinary workspace membership must be qualified; do not construct the case solely by injecting a fake graph. [JS node discovery][graph] [Relative directory][directory]

An illustrative gate declaration is:

```yaml
commands:
  env:
    CHECK_LABEL: core
gates:
  - id: literal-subject
    kind: audit
    run: 'C:/qualified/check.cmd {node}'
    scope: node
```

The actual fixture would include the required `nerd/v1` frontmatter. The launcher is a minimal fixed adapter to the native checker: it forwards exactly one positional argument, uses no `CALL` or other re-evaluation step, and preserves the native checker's exit code. The checker accepts one directory, reports which synthetic subject it received, and exits according to that directory's independently specified condition. Its own implementation must not perform environment expansion or choose a sibling directory.

Source-derived expected sequence:

| Boundary | Value or observation |
|---|---|
| Discovered node | `part%CHECK_LABEL%` |
| Expanded gate Args element | `part%CHECK_LABEL%` |
| Process environment | `CHECK_LABEL=core` |
| Batch command-line interpretation | The percent reference can become `core` before the launcher receives its argument. |
| Ultimate checker subject | `partcore`, under the qualified interpreter behavior |
| Retained Result.Argv | Still `part%CHECK_LABEL%` |

If the qualified launcher receives the expanded subject, the healthy sibling returns zero. `gates.Run` then reports an ordinary pass, and `FindingsAll` returns no findings for a pass. Recurse can therefore have a green node invocation without having checked the intended directory. This is already a meaningful measurement error; no claim that a later repair is necessarily adopted is required. [Exit classification][classification] [Finding extraction][findings] [Measurement][measurement]

The failure variant is also useful: if the expanded sibling is absent, the gate can report an ordinary checker failure against a path the declaration never named. The intended source need not be broken. This can create avoidable repair work, but actual selection and model behavior remain contingent on the normal policy and execution path.

A plain directory name is a negative control. Another is the same native checker invoked directly by its qualified `.exe`, bypassing the batch transport. Both must receive the original argument exactly. The witness is invalid if the adapter itself expands arguments, the native helper chooses the wrong subject, a different executable runs, or an admission rule excludes the source directory.

### Why quoting the placeholder in YAML is insufficient

Quotes used to form one token are consumed by `shellquote.Split`; they are not a durable proof of how a later interpreter parses that token. The requested typed argument and the platform's wire command line must be distinguished. Nor is “quote every argument” a complete proposed solution: Windows quoting, percent substitution, delayed expansion and batch forwarding have different rules.

This review does not assert that every shell metacharacter has the same consequence. Empty arguments, embedded JSON quotes, Unicode, trailing backslashes and ampersands belong in the characterization suite. The repository's own perception fixture notes prior `.cmd` argument loss and replaced its script fake with a native binary. That is corroborating engineering context, not a test result from this review or proof that the recurse route is repaired. [Existing fixture explanation][fake]

## 3. Existing work and novelty boundary

There is already a local recognition of Windows command-line grammar: tactile exposes a `CommandLine` field, and its Windows adapter installs it. A separate legacy verification-shell route uses that field for an explicitly shell-authored command. These are not called by the shared gate builder. They also do not establish that arbitrary literal argv can be safely inserted into a shell string. [Tactile type][tactile] [Platform application][platform] [Explicit legacy shell route][legacy]

Relevant prior work:

- **Gate declared-PATH executable selection** owns resolving the wrong program under the wrong search environment. It explicitly leaves Windows batch behavior to platform qualification. This witness pins the intended launcher and isolates its argument transport. [Prior PATH audit][priorpath]
- **Recurse POSIX environment key identity** owns case-folding two distinct POSIX names. This witness has one ordinary Windows variable and preserves its name/value. [Prior environment audit][priorenv]
- **Recurse gate contract continuity** owns environment and accepted-exit handoff. Complete handoff does not make the process receive literal argv. [Prior handoff audit][priorcontract]
- **Workspace test receipt invocation identity** owns reusing observations across different contexts. This witness executes a fresh gate; faithful recording of requested argv is insufficient when the interpreter changes its meaning. [Prior receipt audit][priorreceipt]
- **Portable recurse deliberation crucible** supplies the broader adapter/capability direction. This report isolates one deterministic host-boundary obligation rather than adding another deliberation framework. [Prior portable design][priorportable]

The bounded novelty screen reviewed the public audit inventory/queue, relevant neighboring reports, and repository searches for `.cmd`, `cmd.exe`, `CmdLine`, batch quoting and percent expansion, including audit-directory code searches. No exact prior report of this recurse literal-subject crossing was found. This is not an exhaustive claim about historical branches, private reports or all platform behavior. [Queue][queue]

The shared builder is also used for declared generators. A common correction can benefit both callers, but no separate generator incident or unauthorized write is claimed here. [Generator caller][generator]

## 4. Proposed bounded execution contract

Define the invariant before selecting an implementation:

> An admitted literal argv invocation either delivers each argument to its intended checker with the declared meaning, or reports that this launcher/input combination is unsupported. A completed process with altered argument meaning cannot certify the requested subject.

1. **Represent the transport.** Distinguish native executable arguments from explicitly authored shell program text and supported script-launcher contracts. Bind identity and evidence to the effective launch description, not only the user-facing argv.
2. **Keep literal inputs literal.** A node path, selector or JSON value is data. Do not reinterpret it as a request to expand environment variables or command syntax.
3. **Prefer a defined native entrypoint when one exists.** For a known supported tool adapter, an explicit interpreter plus script entrypoint may avoid a batch shim. Resolve this under the same executable/environment/working-directory contract and preserve the tool's documented behavior. Do not guess npm internals or silently substitute tools.
4. **Fail explicitly for unsupported transport.** A conservative containment can reject ambiguous batch-backed literal inputs pending a qualified adapter. Surface a capability limitation, not a source failure or a pass. This reduces availability and must be described honestly.
5. **Do not expose a general shell escape hatch.** Any reviewed batch adapter must handle its complete supported input domain, preserve permission and effect constraints, and report the exact transport. Raw `CommandLine` is a mechanism, not proof of safety.
6. **Compose across outer checks and acceptance.** Qualify every executing route under the actual backend. The current private environment/accepted-exit contract remains necessary; do not weaken it to repair transport.
7. **Keep Mangle as the executive.** Host code measures and validates execution capability and observations. Policy retains admission and verdict authority. No model call is needed to decide whether a literal argument survived.

Do not add parent-process environment mutation, ambient-secret forwarding, installation, registry changes, broader write scope or disabled process cancellation.

## 5. Costs, failure modes and fair unrun evaluation

An execution-kind check and deterministic encoder/adapter should be small compared with a gate process, but no speedup is established. Costs include platform-specific code, conformance maintenance, tool-version qualification, refusal UX, potential extra launcher processes and richer receipt metadata. Explicit unsupported cases may block previously successful simple commands; measure that availability trade-off.

Do not solve the problem by replacing every `exec.CommandContext` with `cmd /c`, by concatenating arguments, by hiding mismatch through output parsing, by stripping percent characters from filenames, or by rewriting the user's declared subject. Protect the original launcher and fixture from candidate edits. Apply existing capture, timeout, process-tree and environment controls to the entire invocation.

All evaluation below is proposed and **unrun**:

| Layer | Required observation |
|---|---|
| Native Windows transport | A fixed observer records exact argv for ordinary names, spaces, percent references, JSON quotes, empty strings, Unicode and trailing backslashes. It must not perform its own expansion. |
| Batch controls | Compare the direct native checker, a simple batch wrapper and each supported adapter. Record actual toolchain/OS/launcher versions. A helper that only inspects the pre-launch Cmd object does not test transport. |
| Full gate path | Load real nerd.md, discover actual JS directories, admit the gate, run the intended launcher, and compare Result.Argv with the independently recorded received subject. |
| False-green witness | The intended node fails and its expanded-name sibling passes. The intended obligation must not be certified from the sibling's result. |
| Failure witness | The expanded sibling is absent. Transport failure must not be misreported as a defect in the intended source. |
| Recurse integration | Exercise CLI and chat with normal world/CodeDOM and policy. No model is needed for the deterministic measurement fixture; only separately authorized end-to-end work should assess repair behavior. |
| Shared callers/backends | Qualify generator and acceptance paths separately; a direct-Windows result does not establish Docker, WSL or POSIX behavior. |
| Safety controls | Missing launcher, stopped context, denied effect, unknown execution kind, genuine failing checker, unchanged invocation, and disallowed environment variable retain their existing semantics. |
| Cost | Report extra processes, latency, allocations, refused otherwise-valid invocations, wrong-subject passes/failures and unnecessary repair attempts separately. |

Existing gate tests exercise Go subprocess outcomes and unstartable/cancelled commands. The contract-continuity fixture uses the native test executable, which correctly isolates environment and exit handoff but does not exercise a Windows batch launcher. These tests were read, not run; their existence is not coverage of this transport boundary. [Gate tests][runtest] [Contract fixture][fixture]

**Closure criterion:** a supported batch-backed declared gate demonstrably checks the exact intended literal subject on Windows, or is explicitly held before its result can certify that subject; native invocation behavior, admitted environment/exit contracts and process safeguards remain intact. This does not claim complete cross-platform hermetic execution.

## Pinned references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/architecture/AGENTS.md
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/architecture/campaign/README.md
[buildspec]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/architecture/build/INTERNALS.md
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/projectdoc/nerdmd.go#L167-L207
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/projectdoc/nerdmd.go#L458-L556
[declaration]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/detect.go#L54-L90
[gate]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/gates.go#L60-L81
[expansion]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/gates.go#L129-L147
[admission]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/gates.go#L173-L197
[builder]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/build/workspace_command.go#L30-L60
[run]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/run.go#L44-L100
[classification]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/run.go#L65-L82
[process]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/processutil/command.go#L27-L50
[windows]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/processutil/proc_tree_windows.go#L39-L80
[gomod]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/go.mod#L1-L3
[graph]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/campaign/recurse_workspace.go#L552-L572
[directory]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/campaign/recurse_workspace.go#L345-L350
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/campaign/recurse_cycle.go#L759-L839
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/findings.go#L116-L139
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/cmd/nerd/cmd_campaign_recurse.go#L253-L276
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/cmd/nerd/chat/campaign_recurse.go#L218-L244
[sidecar]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/campaign/recurse_mission_storage.go#L80-L151
[generator]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/tools/shell/regenerate.go#L189-L216
[tactile]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/tactile/types.go#L49-L58
[platform]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/tactile/platform_windows.go#L191-L202
[legacy]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/core/virtual_store_actions.go#L1219-L1229
[fake]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/perception/fake_cli_test.go#L12-L20
[fixture]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/campaign/recurse_contract_continuity_test.go#L20-L85
[runtest]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/internal/gates/run_test.go#L23-L84
[priorpath]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/audits/gate-declared-path-executable-selection.md
[priorenv]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/audits/recurse-posix-environment-key-identity.md
[priorcontract]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/audits/recurse-gate-contract-continuity.md
[priorreceipt]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/audits/workspace-test-receipt-invocation-identity.md
[priorportable]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/audits/portable-recurse-deliberation-crucible.md
[queue]: https://github.com/theRebelliousNerd/codenerd/blob/fea75fdfd13053ad2b3626bc5742d2f518630c8e/Docs/audits/ASTRA_QUEUE.md
[goapi]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/os/exec/exec.go#L394-L414
[goencoder]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/syscall/exec_windows.go#L48-L114
[golaunch]: https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/syscall/exec_windows.go#L307-L360
[gobatch]: https://github.com/golang/go/commit/42602f0ab744f24840ebf8ba3180bd493c74ea3e
[cmd]: https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/cmd

