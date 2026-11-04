---
doc-class: deep-dive
subsystem: campaign-recurse-build-environment
implementation-status: proposed-refinement
last-verified: 2026-11-04
verified-against: d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f
verification-method: static-source-inspection
---

# Recurse changes case-sensitive environment names before running a gate

## Finding and decision

The shared workspace command builder treats environment-variable names as case-insensitive on every platform. On Linux and macOS, an admitted declaration containing both `CHECK_PROFILE` and `check_profile` consequently loses one variable before execution. The overlay sorts names, adds the uppercase entry, then replaces it with the lowercase entry through `strings.EqualFold`. A valid project command can therefore receive a different environment from the one `nerd.md` declared, even with a correct executable, complete configuration handoff and fresh execution. [Builder][builder] [Merge][merge]

This is a concrete **environment-name identity defect**, not merely an environment digest omission. The same helper serves recurse gates and declared generators. There is also an integration discontinuity: recurse's repaired acceptance handoff keeps the original map, and the inspected direct tactile executor appends both entries without this case fold. Under a POSIX execution environment, a checker can fail in the outer gate and pass through acceptance solely because those routes materialize different variable names. [Handoff][handoff] [Acceptance][acceptance] [Direct environment][direct-env] [Generator][generator]

Specify and preserve the target execution platform's environment-name equality before projecting checker outcomes into policy. Retain the allowlist, declared overlay precedence, private runtime contract, process controls and Mangle verdict ownership. Do not fix this by uppercasing project names, widening ambient environment access, changing a user's declaration or weakening a failing check.

**Evidence boundary:** static source composition at the pinned public main, with conditional, unrun witnesses. No repository program, build, test, checker, generator, model inference, benchmark or fault injection was executed. No repository source, credential, configuration or setting was changed. Public source copies and this report are scratch artifacts. Guidance, campaign/build architecture, current source, tests as text, and neighboring audits were reviewed. The build architecture corpus contains older source maps; behavioral claims below use current code rather than treating those maps as execution evidence. [Guidance][guidance] [Architecture rules][architecture] [Build corpus][build-corpus]

## 1. The contract and reachable mechanism

### Declared names survive parsing, but not outer command construction

The project schema describes `Commands.Env` as variables that must be set for canonical commands, including prerequisites that would otherwise be invisible in argv. Its type is a string-to-string map. The inspected validation does not require uppercase names or reject the two distinct names used here. `fromNerdMD` copies that map onto canonical commands and explicit gates. [Schema][schema] [Validation][validation] [Detection][detection]

Both CLI and chat recurse enter `RunRecurseCycles`. A pass detects gates and measures them; node gates and workspace gates ultimately call `gates.Run`, whose `Command` passes `g.Env` to `build.WorkspaceCommand`. [CLI][cli] [Chat][chat] [Pass][pass] [Measurement][measurement] [Gate command][gate-command]

The builder prepares the allowed base environment, sets the workspace directory, sorts the declared map's keys and applies each through `setEnvVar`. That helper compares the existing name and incoming name using unconditional `strings.EqualFold`, then replaces the entire `name=value` string. It does not branch on operating system or backend execution platform. [Builder][builder] [Merge][merge]

For an initially absent pair, the source-level transformation is:

| Stage | Relevant entries |
|---|---|
| Project declaration | `CHECK_PROFILE=strict`, `check_profile=fixture` |
| Sorted overlay order | `CHECK_PROFILE`, then `check_profile` |
| After first insertion | `CHECK_PROFILE=strict` |
| After case-folded replacement | `check_profile=fixture` only |

This requires neither nondeterministic map iteration nor duplicate YAML keys. They are distinct names in the admitted map. The replacement changes both which name exists and which value survives.

Go's own subprocess implementation makes environment-key deduplication case-insensitive only on Windows. Microsoft documents that environment names on Linux and macOS are case-sensitive, unlike Windows. These primary sources support the platform distinction; they are not a runtime reproduction of codeNERD. [Go environment implementation][go-env] [Platform semantics][platform-env]

### Allowlisted ambient names are affected too

`AllowedEnv` also calls `setEnvVar` while assembling the fixed pass-through list and explicitly allowed additional names. A declared lowercase `path`, for example, can replace an inherited uppercase `PATH` in the constructed environment on POSIX rather than coexist with it. The initial executable has already been selected by `exec.CommandContext`; losing `PATH` may instead affect that child's later subprocesses. The actual consequence depends on the child and is not needed for the primary two-declared-key witness. [Allowed environment][allowed] [Builder][builder]

This is separate from the previously reported lookup-before-PATH-overlay issue. Here the correct executable can already be fixed by an absolute path, and no executable search is needed to observe the lost variable.

## 2. The repaired acceptance handoff preserves a different contract

The current source addresses the older missing-environment handoff. An ordinary selected finding copies `source.result.Gate.Env` into the attempt. `RecurseAttemptCampaign` clones that map into a private runtime contract. Acceptance verifies the workspace, command, gate ID and accepted exits, then calls `tools.CheckEnvironment` to render the map as sorted `KEY=VALUE` entries. That renderer retains both differently cased keys. [Handoff][handoff] [Campaign][campaign] [Runtime contract][runtime] [Overlay renderer][overlay-renderer]

`runAcceptanceRound` includes the resulting environment in the tactile command. For the inspected DirectExecutor, `buildEnvironment` appends the command overlay after the base environment. Its eventual `exec.Cmd` therefore retains both distinct POSIX names. [Acceptance][acceptance] [Direct launch][direct-launch] [Direct environment][direct-env]

Thus copying the map correctly is necessary but not sufficient for invocation equivalence:

- Outer gate materialization: one case-folded key survives.
- Direct-backed acceptance materialization: both exact keys survive on POSIX.
- The same declaration and same argv do not imply the same command prerequisites.

This report does not claim all tactile backends have the direct backend's behavior, nor that any particular production session uses that backend. Backend selection and ordinary admission must be qualified in a future integration witness. No change to that selection is proposed here.

## 3. Conditional witnesses, not executed fixtures

### A. Fresh project gate receives the wrong environment

Use an admitted Linux or macOS workspace with an available immutable checker. Select it by a known absolute executable path, or otherwise independently qualify that the intended executable starts. No credentials, network, special build flags, source race, tool installation, shell expansion or changed declaration is needed.

The project supplies:

```yaml
commands:
  env:
    CHECK_PROFILE: strict
    check_profile: fixture
gates:
  - id: environment-contract
    kind: audit
    run: "<qualified checker> {node}"
    scope: node
```

This is an illustrative declaration fragment; the actual fixture would include the normal `nerd/v1` frontmatter and a platform-correct quoted executable.

Define the checker independently: it requires both exact variables with the stated values and then checks one ordinary node input. If a prerequisite is missing, it reports a location-bearing diagnostic for that node and exits nonzero; when both are present and the source is valid, it exits zero. Use a real admitted node and an immutable checker outside the candidate write scope. This follows the existing contract fixture's pattern of a fixed checker rather than asking an LLM to manufacture the evidence. [Existing fixture][contract-test]

The unconditional source fact is the lost uppercase entry. Conditional consequences are:

1. Outer `gates.Run` starts the correct checker with only `check_profile=fixture` from the pair.
2. The checker fails its declared prerequisite despite an otherwise valid source.
3. The gate records an ordinary completed failure, not an unavailable executable.
4. For this node-scoped witness, measurement assigns each retained finding to the invocation's node, including a fallback finding when no diagnostic location is parsed. The failure can enter that node's measured finding set and normal repair selection; a parseable location is not required for this attribution. In a separate workspace-scoped variant, attribution instead depends on the reported target, and an unattributed fallback must not be assumed selectable as a node finding. [Result classification][result] [Measurement][measurement] [Finding parsing][findings]

The source defect exists even if independent policy, worker reasoning or review refuses to repair this environment-generated failure. Those safeguards can limit wasted work; they do not restore the declared process environment.

### B. Acceptance passes while the outer recheck still fails

Extend A with an independently real, authorized source repair and a direct-backed acceptance path:

- Start with the checker input genuinely broken.
- The candidate changes only that input to its independently specified correct value.
- The declaration, checker bytes, executable and backend remain unchanged.
- All ordinary execution admission, ownership and verification conditions still apply.

The acceptance contract carries both names, so the direct-backed checker can see the repaired input under the intended environment and return zero. The subsequent outer gate construction repeats the case fold, removes `CHECK_PROFILE`, and returns the prerequisite failure again. The outer target can therefore remain unresolved despite successful inner acceptance.

This is a conditional integration discrepancy, not a claim that a complete live run was observed, every attempt must be reverted, or any particular model will misdiagnose the failure. A fair test must record each route's exact nonsensitive key-presence result and isolate the environment mismatch from changed source, backend differences and unavailable tools.

### Controls that discriminate the cause

- Rename the lowercase key to a noncolliding exact name and update the immutable fixture contract accordingly. Both should survive.
- Keep only one declared name and ensure no conflicting inherited name exists.
- Use an exact same-case overlay of an inherited variable: the declared value should still override.
- Keep the two names on Windows: they do not represent two independently addressable environment variables there, so POSIX expectations must not be imposed on that platform.
- Compare direct construction with the effective environment rather than only asserting that `Gate.Env` contains both keys.

## 4. Existing protections and novelty boundary

The current runtime-contract cloning, allowed-exit preservation, restore refusal and workspace checks are valuable repairs. The inspected contract test exercises an uppercase `CHECK_PROFILE` value, conflicting base value, task execution and acceptance; it does not use two distinct names related only by case. It was read, not run. The build environment admission test and gate allowlist test likewise cover selection and exclusion, not this key-equivalence boundary. [Contract test][contract-test] [Build admission test][build-test] [Gate allowlist test][allowlist-test]

The bounded screen included the current public audit inventory and queue, current `WorkspaceCommand` callers, searches for `setEnvVar`, `EqualFold environment` and case-insensitive environment terms, and direct reading of the nearest reports. No exact prior report of unconditional environment-name folding was found. This does not establish novelty across every branch, historical discussion or private project.

Related work credited:

- **Recurse gate contract continuity** owns the earlier loss of the environment map and accepted exits during handoff. Current source preserves that map; this defect happens when executing it. [Prior handoff][prior-handoff]
- **Gate declared-PATH executable selection** owns search-domain disagreement before launching a command. The primary witness here fixes the executable and uses only nonsensitive non-PATH keys. [Prior PATH][prior-path]
- **Workspace test receipt invocation identity** owns reuse across different invocation contexts. This witness executes fresh commands and does not need receipt reuse. [Prior receipt][prior-receipt]
- **Recurse checker runtime/subject applicability** owns false coverage exclusions based on the launcher language. The fixture can use a generic qualified checker and an admitted node, so no language exclusion is required. [Prior subject][prior-subject]

The shared builder's generator caller increases the value of fixing this at the materialization boundary. It does not justify a separate speculative generator incident: only the shared call and the same declaration input are established here. [Generator][generator]

## 5. Proposed bounded contract

1. **Preserve platform-defined name identity.** On a case-sensitive execution platform, only the exact same name can replace an earlier value. On Windows, use the intended case-insensitive environment contract. For remote/container backends, name equality belongs to the execution target, not an unrelated compiler target or assumed host.
2. **Preserve precedence separately from equality.** Allowed base values precede command-specific overlays; a same-name declared value still wins. Do not solve collision by dropping the whole environment or changing the allowlist.
3. **Unify effective semantics across the invocation family.** Outer gates, task checks, acceptance and declared generators should agree on declared-name interpretation. A common materializer or explicit backend contract can serve this purpose. Merely making map copies equal is not sufficient.
4. **Keep evidence private and bounded.** Diagnose key identity and prerequisite availability without dumping environment values into task prose, receipts or public logs. Existing host-only environment identity can describe the constructed command, but hashing a lossy construction does not make it faithful to the declaration. [Identity][identity]
5. **Do not expand the executive.** This is deterministic input construction. No extra model, prompt rule, novel planner or relaxed ratchet is needed. Go should produce the correct observation; Mangle retains the decision.

A minimal local correction is plausible, but no implementation or API has been selected or written in this review. The nearby `setEnvKey` helper uses exact matching while `setEnvVar` folds case; blindly replacing every helper with either behavior could fix one platform and regress another. Review the execution-domain contract before consolidation. [Other merge helper][other-merge]

## 6. Costs, failure modes and fair evaluation

The current overlay sorts its keys and linearly scans the growing environment for each insertion. Correct name equality need not introduce a subprocess, model call or new data store. A common resolver could reduce duplication but adds migration and backend conformance work. Measure actual allocations and construction time before claiming a speedup.

Avoid these repair failures:

- globally uppercasing names or using Unicode case folding where the target requires exact identity;
- changing precedence for true duplicate names;
- forwarding more ambient variables to compensate for missing prerequisites;
- exposing values while explaining collisions;
- assuming a Windows-hosted Linux container shares Windows environment semantics;
- fixing the outer gate while leaving acceptance or generator construction inconsistent;
- treating equal hashes of already altered environments as proof of original declaration fidelity.

All proposed evaluation is **unrun**:

| Level | Cases and required observation |
|---|---|
| Materialization | Two declared case-distinct keys on POSIX both survive with exact values; same-case override keeps the declared value; map insertion order does not affect the result. |
| Ambient overlay | An explicitly allowed lowercase name coexists with its uppercase peer on POSIX; declaring `path` does not delete `PATH`; unlisted synthetic secrets still remain excluded. |
| Platform controls | Linux/macOS exact-name cases and native Windows duplicate-name cases use platform-correct expectations, including empty values and mixed-case names. |
| Fresh execution | An immutable no-network checker observes the expected key set; capture only synthetic values or key-presence assertions. Do not mutate the parent environment concurrently. |
| Recurse integration | Use actual `nerd.md` acquisition, outer gate, selected attempt, task check, acceptance and after-check. Correctly repaired source must not remain red solely because one route removed a declared name. |
| Negative controls | Genuine source failure, missing executable, changed runtime contract, forbidden write, cancellation and restore-without-contract remain blocked or accurately failed. |
| Shared caller | A declared generator receives the same name interpretation while existing output ownership and authorization checks remain intact. |
| Performance | Compare identical declaration sizes and base environments; report construction time, allocations, extra subprocesses, verification disagreement and unnecessary repair rounds separately. |

First establish the deterministic transformation and process witness offline. Only then, under separate authorization, consider a production CLI/chat run with CodeDOM and ordinary policy enabled. Keep fixtures used to develop the fix separate from held-out project configurations. A direct helper test does not establish backend parity; a Windows pass does not establish POSIX fidelity; an eventual model workaround does not erase the original transport defect.

**Closure criterion:** an admitted command's case-distinct POSIX names survive to the process, ordinary same-name override semantics and security filtering remain intact, and the actual outer/acceptance routes no longer disagree for the isolated witness. No claim of complete cross-platform hermetic execution follows.

## Pinned source references

[guidance]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/agents.md
[architecture]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/Docs/architecture/AGENTS.md
[build-corpus]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/Docs/architecture/build/INTERNALS.md
[builder]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/build/workspace_command.go#L30-L60
[merge]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/build/workspace_command.go#L107-L115
[allowed]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/build/workspace_command.go#L91-L104
[schema]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/projectdoc/nerdmd.go#L197-L207
[validation]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/projectdoc/nerdmd.go#L458-L556
[detection]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/gates/detect.go#L50-L91
[cli]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/cmd/nerd/cmd_campaign_recurse.go#L253-L276
[chat]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/cmd/nerd/chat/campaign_recurse.go#L218-L244
[pass]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/recurse_cycle.go#L515-L566
[measurement]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/recurse_cycle.go#L759-L840
[gate-command]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/gates/run.go#L44-L100
[result]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/gates/run.go#L65-L82
[findings]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/gates/findings.go#L97-L185
[handoff]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/recurse_cycle.go#L1168-L1177
[campaign]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/recurse_attempt.go#L95-L112
[runtime]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/acceptance_runtime.go#L20-L45
[overlay-renderer]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/tools/campaign_check.go#L65-L77
[acceptance]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/orchestrator_acceptance.go#L156-L197
[direct-launch]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/tactile/direct.go#L161-L176
[direct-env]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/tactile/direct.go#L299-L319
[generator]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/tools/shell/regenerate.go#L190-L216
[contract-test]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/campaign/recurse_contract_continuity_test.go#L20-L149
[build-test]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/build/live_test_admission_test.go#L11-L58
[allowlist-test]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/gates/run_test.go#L86-L98
[identity]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/processutil/identity.go#L14-L46
[other-merge]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/internal/build/env.go#L584-L621
[prior-handoff]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/Docs/audits/recurse-gate-contract-continuity.md
[prior-path]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/Docs/audits/gate-declared-path-executable-selection.md
[prior-receipt]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/Docs/audits/workspace-test-receipt-invocation-identity.md
[prior-subject]: https://github.com/theRebelliousNerd/codenerd/blob/d0ed3b7a8d8b1db6da5b808ae49887dcecc6e53f/Docs/audits/recurse-checker-runtime-subject-applicability.md
[go-env]: https://go.dev/src/os/exec/exec.go#L1260
[platform-env]: https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.core/about/about_environment_variables?view=powershell-7.6

