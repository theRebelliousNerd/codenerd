# Shell-tail effect admission: preserve every stage's authority requirements

Prepared 2026-10-28. **Static research only: no builds, tests, shell witness execution, or live model inference were performed.** Repository source was read through the GitHub connector; the external references below were read through web search. This is a source-backed conditional finding, not an observed production exploit.

## Finding and scope

**An admitted shell call can lose a mutating pipeline stage during effect classification while executing that stage unchanged.** The shared shell-effect gate explicitly claims to reject every shell mutation without deterministic task-scope authorization. Its `stripBenignOutputTail` helper instead drops a trailing command when its first word belongs to `safePipeReaders`. This includes `sort`, without validating the remaining arguments. `ClassifyShellEffect` applies the mutation/output-option checks only after that stage has disappeared. A proposed benign synthetic witness is `echo hello | sort -o guarded.txt`: the classifier sees `echo hello`, but on a POSIX host the runtime executes the original pipeline, whose `sort -o` writes `guarded.txt`.

**Important exposure limit:** `run_command` and `bash` are deliberately absent from the default turn catalog. This is not a claim that an ordinary default model turn can call them. The finding applies when a caller already has the shell capability (for example an explicit/custom effective envelope) or reaches the existing guarded registry entry path. It does not grant capability, does not bypass a missing Dreamer, and does not override a read-only restriction that denies execute effects. The source gap is the effect-admission contract on a supported executable surface, not a newly demonstrated default-entry compromise.

### Exact pins

- Main: `356f65e0e6bb639adaa90ab36212daa714114871`.
- Feature branch `feat/conversation-brief`: `c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c`.
- `internal/projectdoc/tool_gate.go` is byte-identical at both pins: blob `4b06438c39f5e74683f0ddc4a0ab8f0ea1cf330a`.
- `internal/tools/shell/execute.go` is byte-identical: blob `e035249fd7509e8f378deadd30dc8dbcdf43a6b8`.
- `internal/core/virtual_store_write_guard.go` is byte-identical: blob `b6899b1a078cf2789bce3d1ea5c84258f6304ce5`.
- Session call-site lines differ: main uses `checkShellEffect` at 2081, invokes it at 2508, and dispatches at 2643; the feature branch uses 2087, 2518, and 2653. The shared classifier and original-command execution remain the same.

## Existing work inspected; why this is distinct

Read repository `agents.md`, scoped tool/session/core guidance, and the feature branch audit inventory before selecting the seam. Fully inspected the current request-bound approval and tool-effect receipt audits, including their **As built** sections; inspected argument-coercion and write-mutation inventories. Searched the contents of all 42 top-level feature-branch `Docs/audits/*.md` files for `stripBenignOutputTail`, `ClassifyShellEffect`, `safePipeReaders`, `sort -o`, `shell-effect`, and related shell-mutation terms; no matches. This bounded search is not a proof that every historical document or nested audit lacks related work.

- `request-bound-human-approval.md` concerns exact consent, claim, resume, and immutable subjects. The shell-effect classifier itself imposes no mutation-scope requirement on this misclassified read-only command; independent gates may still require consent or deny it. Adding broader approvals is not the proposed fix.
- `tool-effect-receipts-and-validation-completeness.md` now preserves effects for recognized write tools. It explicitly leaves shell effects under their existing gates. This finding identifies a specific pre-execution classifier hole, not the already-documented missing durable replay ledger or concurrent writer attribution problem.
- The existing `toolWriteGuard` comment already identifies the broader absence of immutable task-baseline shell-write authorization. This finding is **not** a restatement that scoped shell writes remain unavailable: it shows an invocation incorrectly placed on the supposedly non-mutating side of that boundary.
- No TUI evidence authority, slash payload, edit precondition, or source-CAS claim is made.

## Source proof, with the gates retained

All links in this table are pinned to the feature-branch SHA unless marked main.

| Boundary | Source witness | What it establishes |
|---|---|---|
| Intended invariant | [tool_gate.go:275–278](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/projectdoc/tool_gate.go#L275-L278) | Shared fail-closed decision; shell mutations must not run without deterministic scope authority. |
| Tail erasure | [tool_gate.go:398–433](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/projectdoc/tool_gate.go#L398-L433) | `sort` is a safe-reader name. The helper checks a small set of shell metacharacters and the first field, then removes the entire tail; it does not check `-o` or an output operand. |
| Validation after erasure | [tool_gate.go:492–524](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/projectdoc/tool_gate.go#L492-L524) | `lower`, quote masking, known mutation checks, and `--output` / ` -o ` checks all operate on `stripped`, not the original pipeline. |
| Read-only conclusion | [tool_gate.go:546–559](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/projectdoc/tool_gate.go#L546-L559) and [338–349](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/projectdoc/tool_gate.go#L338-L349) | `echo` is a read-only prefix; read-only classification returns nil from admission. |
| First use | [executor_tools.go:2083–2104,2515–2520](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L2083-L2104) | Session delegates shell-effect admission to this shared classifier. |
| Second use | [virtual_store_write_guard.go:28–43](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/virtual_store_write_guard.go#L28-L43) | Registry defense-in-depth repeats the same classifier. Shell calls are not among the named file-mutation tools, so they go to preflight instead of per-target `ForbiddenByKernel`. |
| Capability still matters | [executor_tools.go:2387–2394](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/executor_tools.go#L2387-L2394) and [intent_routing_rules.mg:216–228,357–366](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/defaults/policy/intent_routing_rules.mg#L216-L228) | An effective allowlist is checked; default catalog explicitly excludes shell tools. Registration is not capability. |
| Dreamer still matters | [virtual_store_interactive_gate.go:159–205,272–306](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/virtual_store_interactive_gate.go#L159-L205) | A shell action requires a usable Dreamer and can still be rejected. The gap is not absence of that gate. |
| What Dreamer measures | [dreamer.go:476–497,669–707](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/core/dreamer.go#L476-L497) | Shell projection carries generic `/exec_cmd` and optionally `/exec_danger` based on a specific signature list. The list lacks the synthetic `sort -o` case; the output pathname is not projected as `/modified`. Custom policy may still deny it. |
| Original payload runs | [execute.go:139–166,203–222](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/shell/execute.go#L139-L222) | Original command remains in `command`; an unquoted pipe selects `sh -c` on non-Windows hosts; process execution receives the original command, not `stripped`. |
| Scope is cwd, not every operand | [execute.go:58–67,207–222](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/tools/shell/execute.go#L58-L67) | Working-directory containment is enforced. It is not a proof that a command's output operands are contained or authorized. The primary witness stays inside an isolated workspace. |
| Accounting consequence | [tool_gate.go:190–203](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/projectdoc/tool_gate.go#L190-L203), [tool_effects.go:226–254](https://github.com/theRebelliousNerd/codenerd/blob/c2d39e539ef4fb7dcb6ca6ff809d6c4bc9ada86c/internal/session/tool_effects.go#L226-L254) | Shell tool names are not recognized write-mutation tools; `settleToolEffects` returns before this path's write attribution and test-witness invalidation. This is a secondary consequence if the command is dispatched and writes, not a separate claim that receipts or later workspace checks are universally absent. |

**Broader evidence qualification:** acceptance transactions and post-edit closure have conditional workspace snapshots (`executor.go:1105–1106`; `executor_tools.go:528–556`). Those mechanisms may observe an eventual workspace change in their enabled scopes. The proven accounting claim is only that this call is skipped by `settleToolEffects` and its named write-path accounting; do not turn that into a claim that every later evidence check stays green. After-the-fact detection would not repair the pre-execution policy violation.

## Conditions and counterexamples

For the concrete proposed witness, assume a POSIX host with `sh` and standard `sort`, a valid workspace, a writable non-secret scratch target, a caller whose effective capability includes `run_command`, no restrictive task fact that bars execute effects, and functioning policy/Dreamer gates that admit the non-matching generic command. `sort -o` output semantics are documented by GNU. No claim is made about PowerShell's `sort` alias or a Windows machine without a POSIX executable.

Static evaluation of the code yields:

1. `stripBenignOutputTail("echo hello | sort -o guarded.txt")` removes the `sort` tail because it has an allowed first field and no excluded metacharacters.
2. The remaining `echo hello` has no mutation marker and matches the read-only prefix.
3. Both shell-effect gates use that result.
4. The original command still reaches `sh -c` if all independent gates permit it.
5. The proposed write does not pass the named-file mutation guard or receipt path.

Controls that should prevent overstatement:

- A normal default turn that requests `run_command` is rejected by capability admission before this seam.
- An execute-effect denial from read-only restrictions remains a denial.
- A missing Dreamer remains fail closed.
- `echo hello > guarded.txt` retains its redirection and is rejected by the classifier.
- `echo hello | head -n 1` is the legitimate convenience the existing elision is trying to preserve.
- `sort -o guarded.txt input.txt` is not an allowed read-only command; a direct call differs from the stripped-tail case.
- A quoted string containing pipe characters in an argument is not necessarily a pipeline. A replacement must use the runtime's parsing semantics, not a raw `LastIndex("|")` split.
- Typed `run_build`/`run_tests` reject custom command text and construct argv separately; they must not be described as this exact witness route.

## Proposed contract, not implementation

**Minimum defensible fix:** stop treating a whole tail as harmless based only on its executable name. Fail closed on unsupported pipelines, or support a very small exact grammar of bounded output readers and validate every option and operand before dropping any stage for effect classification. Moving the same short denylist before stripping is insufficient as a complete solution: option spellings, positional outputs, external helpers, and platform semantics differ.

**If preserving pipeline convenience is justified:** Go should parse one immutable, typed execution plan and measure the effects of every stage and redirection. Mangle should decide whether that plan's required effects, target resources, capability, and restrictions are admissible. Execution must consume the admitted plan without reparsing a different string or looking up a different executable. A plan that cannot be modeled remains unsupported; do not widen the default catalog or invent a human-consent fallback. Keep permission, Dreamer, project rules, source containment, and receipt obligations independent. A shell classifier is not an OS sandbox and cannot prove arbitrary project code harmless.

This proposal is deliberately narrower than a complete shell language implementation or a baseline-aware mutation authorization system. If arbitrary shell remains unavailable by default, refusing unsupported pipelines may be the best cost/correctness tradeoff.

## Safe future validation plan (not run)

Use only temporary workspaces and harmless text files. Begin with pure classifier/unit fixtures and dispatch counters; permit actual commands only in a separately authorized isolated integration suite.

| Future fixture | Required result |
|---|---|
| `echo hello | sort -o scratch.txt`; equivalent long option | Rejected before the process adapter; no scratch mutation and no success receipt. |
| Same mutating tail after two allowed readers | Every stage examined; no suffix erasure can conceal output effects. |
| Reader with positional output / helper-executing option | Unsupported until an explicit per-command contract exists; no binary-name-only grant. |
| Legitimate `head -n 1`, `tail -n 5`, `wc -l` tails | Either explicit supported read-only plan or a clear unsupported refusal; count compatibility cost. |
| Quoted pipes, escaped quotes, unmatched quotes, Windows alias variants | Same parse contract at classifier and dispatcher; malformed/unknown forms fail closed. |
| Effective envelope excludes shell | Existing capability denial stays first. |
| Kernel/Dreamer unavailable or restriction bars execute | Existing fail-closed denials preserved. |
| Protected non-secret target in nerd.md | No indirect writer reaches the target by pretending to be a reader. |
| Native, Piggyback, and direct registry routes | Equivalent admission; no secondary path retains the old helper. |
| Permitted typed build/test control | No regression in argv, timeout, cancellation, output capture, or receipt contract. |
| Synthetic permitted write in a future scoped model | Changed resource tracked; earlier check evidence invalidated; no blind replay after uncertain completion. |

## Costs and decision criteria

Compare three alternatives on identical authority: (A) reject raw pipelines; (B) a tiny structured reader-pipeline grammar; (C) a larger shell parser and effect model. Measure false accepts on a held-out command-variant corpus, false refusals on legitimate existing workflows, host parity, gate latency and allocations, parser/plan footprint, maintenance work per supported command, and full task time/tool calls/tokens spent recovering from refusals. Include failures and unresolved tasks; do not credit token savings from unverified work. No token, speed, or productivity improvement has been measured here.

A merits-first decision can favor A if B/C do not produce a meaningful verified task-success gain. Adding many option rules is ongoing security maintenance, not a one-time token optimization. Preserve existing typed tools as the preferred fallback and do not ask the model to choose whether to obey the gate.

## Primary references and their limited role

- [GNU Coreutils: sort invocation](https://www.gnu.org/software/coreutils/manual/html_node/sort-invocation.html) documents that the output option writes a named file rather than stdout. It establishes the witness's command semantics, not that codeNERD dispatched it.
- [MITRE CWE-184](https://cwe.mitre.org/data/definitions/184.html) describes incomplete disallowed-input lists and recommends positive validation alongside other protection. It motivates rejecting unknown option semantics; this audit does not assign a CVE or claim a formal CWE mapping assessment.
- [Saltzer and Schroeder, *The Protection of Information in Computer Systems*](https://www.cs.virginia.edu/~evans/cs551/saltzer/) is the original paper, reproduced by the university. Complete mediation and fail-safe defaults motivate checking every stage/resource. These principles are design guidance, not empirical proof that a proposed parser is correct or worthwhile.

## Completion status

The static seam, shared-gate composition, main/feature-branch equivalence, important default-catalog limitation, and accounting consequence are established from inspected source. Production exploitation, exact installed binary behavior, platform-wide reachability, test coverage, runtime failure rates, and any claimed efficiency benefit remain **unmeasured**. No change is proposed to permission authority without a separately authorized spec-first implementation and validation effort.

## As-built status

Verified and built 2026-10-28 on `main` (after `f74bacb2`). `tool_gate.go`'s tail helper was unchanged from the audited blob.

### Claim ledger

| Claim | Verdict | Evidence |
|---|---|---|
| `stripBenignOutputTail` drops a trailing stage on its first word alone (`sort` included), without checking options or operands | Real, fixed | Before: `tool_gate.go:398-433` (`safePipeReaders`). Now `shell_pipe_readers.go:83` drops a stage only when `isOutputReaderStage` (`:122`) proves every word inside the reader's positive grammar |
| Mutation and `-o`/`--output` checks run on the stripped string | Real; the same checks still run on what remains, which now keeps any unproven stage | `tool_gate.go:453` `ClassifyShellEffect` |
| `echo` is read-only, so the stripped witness classified read-only | Real; the witness now classifies `UnknownMutating` and `ValidateShellToolInvocation` refuses it | `shell_pipe_readers_test.go` |
| Both the session gate and the registry write guard use the same classifier | Confirmed; one fix covers both, plus the VirtualStore action path | `executor_tools.go:2087`, `virtual_store_write_guard.go:33`, `virtual_store_actions.go:611` |
| The runtime executes the original command, not the stripped one | Confirmed; not changed (stripping is classification-only) | `internal/tools/shell/execute.go` |
| Shell tools are absent from the default catalog; capability, read-only restrictions and Dreamer still gate first | Confirmed; not changed | `intent_routing_rules.mg`, `virtual_store_interactive_gate.go` |
| A raw `LastIndex("|")` split mis-splits quoted pipes | Real, fixed | `splitLastUnquotedPipe` (`shell_pipe_readers.go:102`) splits at the last unquoted pipe; `||` is not a pipe |
| Shell writes are not attributed by `settleToolEffects` | Confirmed; left open (below) | `tool_effects.go:227` (`isWriteMutationTool`) |

### What was built

- Ruling (alternative B, the tiny reader grammar): keep pipeline convenience for verification output, but only for stages proven to be readers. Each of `head`, `tail`, `wc`, `cat`, `grep`, `rg`, `sort`, `uniq` has a positive grammar: exact short flags (bundling allowed), short options that take a value, exact long options (no abbreviations), a numeric form where historical, and an operand bound (uniq: one, since its second operand is an output). Not listed and therefore refused: `sort -o/--output/-T/--compress-program`, `rg --pre/-z`, `tail -f/-F`. Unquoted `$`, backticks, globs, `~`, `\`, redirections and `;&` refuse the stage; `$`/backtick inside double quotes refuse it; single-quoted text is literal. A refused stage is kept, and its pipe classifies `UnknownMutating` (fail closed).
- The helper moved from `tool_gate.go` into `internal/projectdoc/shell_pipe_readers.go`.
- Spec: `Docs/architecture/projectdoc/09-SAFETY-AND-INVARIANTS.md` §8.4.

### Tests

`internal/projectdoc/shell_pipe_readers_test.go`: 20 write/helper/unprovable tails (long, abbreviated, bundled and attached `-o`, behind and before other readers, uniq output operand, `rg --pre`, `rg -z`, `tail -f`, variable and glob expansion, absolute reader path, unbalanced quote, `||`) never classify read-only or verification (17 of them did under the name-only rule; confirmed by swapping the old rule back in); 13 legitimate reader tails still elide to verification/read-only, including a quoted pipe in a grep pattern; the shared `ValidateShellToolInvocation` refuses the witness. The existing `tool_gate_test.go` pipe cases pass unchanged.

### Left open

- Alternative C (a full typed shell plan whose effects Mangle admits): not built; arbitrary shell remains outside the default catalog, so B's refusal is the right cost/correctness point now.
- PowerShell's `sort` alias (`Sort-Object`) and Windows hosts were not modeled; a Windows shell stage is refused unless it matches the POSIX grammar.
- Receipt attribution for an admitted mutating shell command (`settleToolEffects` skips shell tools): needs a measured write set, which the shell runner cannot provide; left to the scoped-shell-write work.
- The held-out command corpus, false-refusal rate and gate latency in "Costs and decision criteria" were not measured.
