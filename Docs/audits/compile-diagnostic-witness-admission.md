# Compile diagnostics are not analyzer selectors: a recurse witness contract audit

## Scope and status

Read-only source audit of CodeNERD `main` at **`4d984f9c9b719b013a6400f0c88deb48d2e5eb82`**, inspected 2026-10-29. No implementation changes, commands, tests, inference, or benchmarks were run. The behavior below is a source-derived conditional failure, not a reproduced production incident or a measured performance claim.

**Finding:** the error-free finding-group pipeline admits a normalized compile diagnostic as a selectable analyzer. For the stock Go vet tool, this constructs an invalid acceptance command: `go vet -compile ./pkg`. When that group becomes a recurse target, a repair can remove the actual compile error yet fail its acceptance witness because the command is invalid. The runner remains fail-closed; the problem is false rejection and misleading failure attribution, not acceptance of unsafe code.

This is separate from scoped snapshot conservation, comparison-domain admission, and effective build-tag receipts. Those controls can all work correctly while a target-specific witness names a nonexistent analyzer.

## 1. Source-to-policy evidence chain

| Stage | Implemented behavior | Source |
|---|---|---|
| Measurement | G2 invokes `go vet -json`; `readVet` delegates combined output to `lintreport.VetFindings`. | [errorfree.go](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/gates/errorfree.go#L111-L112), [reader](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/gates/errorfree.go#L272-L274) |
| Normalization | `CompileCheck = "compile"` explicitly identifies a compile error, not an analyzer. Positioned textual vet type-check errors become findings with that value. | [lintreport.go](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/lintreport/lintreport.go#L37-L55), [VetFindings](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/lintreport/lintreport.go#L94-L142) |
| Group admission | `FindingGroups` accepts any finding with a file and check, then calls `WitnessArgv`. The vet branch interpolates `"-" + check` without distinguishing diagnostic kind from analyzer identity. | [errorfree_groups.go](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/gates/errorfree_groups.go) |
| Work publication | `refreshGates` publishes each group as an `ef:` finding of kind lint. `errorFreeSpec` embeds the generated command as both the model's stated success condition and the attempt's check. | [recurse_error_free.go](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/campaign/recurse_error_free.go#L54-L100) |
| Mangle selection | Single-file groups rank with lint; multifile groups rank six. The exclusion rules name unused-code checks, not compile diagnostics. Per-visit attempted IDs are excluded from subsequent selection. | [recurse.mg](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/core/defaults/policy/recurse.mg#L130-L246) |
| Acceptance | After a changed group attempt, `RunWitness` sets `TargetOK`. Once `witnessRan` is true, the ordinary gate-derived target resolution branch cannot replace that result. | [recurse_cycle.go](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/campaign/recurse_cycle.go#L981-L1025) |
| Verdict | The Go adapter maps false `TargetOK` to `/open`; Mangle requires `/resolved` for a fix to succeed, in addition to its other guards. | [recurse_policy.go](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/campaign/recurse_policy.go), [recurse.mg](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/core/defaults/policy/recurse.mg) |

The adapter loses an important distinction before Mangle ever judges the attempt. The kernel is not choosing incorrectly among well-typed evidence; it is being given a target whose proposed success witness does not measure the target.

## 2. Why the stock vet command is invalid

Go obtains vet's supported flags from the selected analysis tool and combines them with build flags. Unknown explicit flags take its usage-error path. The stock vet analyzer suite does not include an analyzer called `compile`. The label used by CodeNERD is a normalization category for failure to type-check, not an analyzer registered by stock vet. See the pinned official [vet flag parser](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/go/internal/vet/vetflag.go) and [stock analyzer suite](https://github.com/golang/go/blob/2aae1000ea1d4b28923b96db28acf4e3b99d6aff/src/cmd/vendor/golang.org/x/tools/go/analysis/suite/vet/vet.go).

This conclusion is explicitly about **stock vet**. A custom vet tool can expose its own flag vocabulary; one could conceivably register a flag named `compile`. Such a tool must be identified and validated rather than assumed. The pinned Go source supports the mechanism; it is not a claim that every user's installed toolchain is that exact revision.

Staticcheck and golangci-lint also appear in the shared constructor, and their readers also normalize compile-related diagnostics. They deserve independent selector-contract checks. This audit does not claim their corresponding commands have the same exit behavior without checking those tools' own implementations and supported versions.

## 3. A concrete conditional selection schedule

The following schedule explains reachability without claiming an observed run:

1. A full error-free measurement returns a positioned vet type-check diagnostic in a package attributed to the current node. Normalization assigns check `compile`.
2. `FindingGroups` constructs its stable group and the command `go vet -compile ./pkg`; `refreshGates` exposes it as a lint target.
3. Ordinary build/test findings, when present and eligible, rank ahead. Let their attempts fail or revert without resolving this package's compile error. Each attempted finding ID is then excluded for the remainder of this visit. The compile group has a separate ID; it is not excluded merely because an ordinary build/test finding was attempted.
4. Provided no other eligible higher-priority candidate remains and no stall/refusal rule excludes this group, it can be selected. A single-file group ranks four; a multifile group waits until rank six.
5. Suppose its executor now repairs the underlying error and changes permitted files. The ordinary gates and scoped error-free snapshot may improve.
6. The group's acceptance command still asks stock vet to accept `-compile`. Its nonzero usage exit sets `TargetOK=false`. Ordinary target resolution does not override it because `witnessRan=true`.
7. This is a fix attempt, not an improvement-angle attempt. Its target remains `/open`, so the normal keep requirement is unavailable; the attempt is reverted unless a stronger refusal path applies.

**Important rescue path:** if the earlier build/test attempt succeeds and refreshed evidence removes the compile group, no bad group witness needs to run. Therefore this is not a claim that every compile failure disables recurse. It is an invalid target contract that remains reachable under the stated conditions.

Existing stall detection and the once-per-visit exclusion constrain repetition. They do not repair the acceptance command, and a stalled target can misrepresent the agent's inability to fix code when the actual defect is the witness adapter.

## 4. Exit failure is not always code failure

`RunWitness` uses the effective workspace build environment and `build.GoInvocation` for Go commands. That is a useful existing control and should remain. It then calls `cmd.Run()` without wiring output capture. A normally exited nonzero `exec.ExitError` becomes `(false, nil)`.

Consequently an invalid command-line usage failure is indistinguishable to this caller from a valid checker reporting unresolved findings. The caller's “could not run” diagnostic is emitted only when `err != nil`; this ordinary nonzero exit bypasses that branch. Mangle sees an open target, not an explicitly invalid witness.

Failing closed is correct: an invalid command must never count as a pass. But fail-closed does not require losing the reason. Preserve bounded stderr, exit status, tool identity, effective arguments, scope and config identity, and classify unsupported invocation separately from a valid negative result.

## 5. Proposed contract: admit a witness, not just a command

The following is proposed, not implemented or validated:

### A. Separate diagnostic category from executable selector

A normalized finding needs at least a diagnostic category (analyzer result, compile/type-check failure, tool failure), originating gate, original diagnostic code, and optional validated analyzer selector. Never infer that a category string is a supported CLI selector.

For the immediate stock-vet case, refusing to manufacture a `-compile` witness is preferable to supplying an impossible one. Preserve the compile finding in measurement and reporting; do not silently discard it as “unsupported.” Route it to a valid compile/type-check repair obligation or record that it is not yet independently actionable.

### B. Give compile obligations their own evidence path

Prefer an existing compatible build/type-check gate receipt when it proves the same package/configuration obligation. Beware of test files: `go build` alone does not prove test-source compilation. A replacement verifier must explicitly cover the relevant source variants, tags, platform, toolchain and dependencies.

Another option is to rerun the originating structured vet reader and require a verified measurement with the compile category absent. That allows other analyzer findings to remain, avoiding the overly strong requirement that unrelated warnings all disappear. It must treat tool failure, malformed output and incomplete scope as unverified rather than interpreting missing parsed compile rows as success.

### C. Make the Mangle boundary express witness validity

Represent witness admission, scope compatibility, and outcome class explicitly. Proposed outcome classes are satisfied, unsatisfied, and unverified; invalid invocation is a reason for unverified, not proof the code obligation remains unsatisfied. Mangle can then exclude non-admitted targets, derive repair obligations for verifier failures, and retain the independent gate-regression, forbidden-write and metric guards.

This should not turn Mangle into a command-string generator or give an LLM authority to invent acceptance commands. A deterministic tool adapter should supply validated receipts; policy should govern which obligations they discharge.

### D. Do not pay twice for identical evidence

The current changed-attempt path can measure scoped gates and then run a separate witness. A compatible structured measurement might already contain the needed evidence. Reuse is safe only with an exact match on source revision/state, tool identity, effective configuration, package scope and completeness. Otherwise rerun. This is an optimization hypothesis, not a claim that current snapshot records are sufficient for sound reuse.

## 6. Full cost accounting

The direct invalid process can be cheap; the surrounding failed attempt need not be. Account for plan/JIT construction, model input and output, retrieval, tool calls, workspace mutation, ordinary gate reruns, scoped error-free measurement, the invalid witness, revert work, journal/history growth and any later retries. Also count opportunity cost: another repair could have used that cycle.

A typed admission check adds parsing and contract-maintenance cost. Tool capability probing adds subprocess startup and version/configuration-sensitive cache invalidation. Bounded diagnostic receipts consume storage. Structured selective verification can rerun a broader analyzer suite than a valid single-check witness. Reusing existing measurements can save work but raises stale-evidence risk; exact compatibility checks and conservative fallback are part of its cost.

No token savings, latency win, reduced retry count or repaired production incident is measured here.

## 7. Fair evaluation and acceptance plan

These are proposed checks for an authorized implementation pass, not tests run during this audit:

| Case | Required outcome |
|---|---|
| Valid stock vet analyzer, such as printf | Preserve valid targeted verification and existing guard behavior. |
| Positioned compile diagnostic | Never synthesize a stock-vet analyzer selector named compile. Preserve diagnostic provenance. |
| Higher-priority ordinary gate repairs it | Refreshed evidence removes the group normally; no extra model attempt is needed. |
| Higher-priority attempt fails once | Demonstrate separate group eligibility, then show repaired code can satisfy the corrected compile obligation. |
| Compile error in test source | Do not accept ordinary non-test build evidence as sufficient. |
| Unknown or malformed analyzer selector | Reject witness admission explicitly, before expensive agent work when possible. |
| Custom vet tool | Validate against the selected tool, not a hard-coded stock list alone. |
| Tool unavailable, usage failure, malformed output, cancellation | No pass; retain distinct unverified reason and bounded diagnostics. |
| Target clears but another gate regresses | Revert as before. |
| Forbidden write or reduced test-count guard | Preserve refusal/reversion; witness correction cannot waive it. |
| Different tags, source state or package scope | Do not reuse incompatible evidence. |

The existing [finding-group end-to-end test](https://github.com/theRebelliousNerd/codenerd/blob/4d984f9c9b719b013a6400f0c88deb48d2e5eb82/internal/campaign/recurse_error_free_pick_test.go) exercises a valid staticcheck S1002 witness and a second change rejected for increased findings. That is useful positive and regression coverage; it does not establish the compile-category selector contract.

Compare baseline and proposed behavior with identical changes, toolchain/configuration, fixture state and cache conditions. Report false rejection and false acceptance separately, plus model turns, repeated measurements, CPU time, wall time, output bytes and stored receipt bytes. A useful result is not merely “fewer commands”: it is fewer invalid work attempts with no weaker evidence or relaxed safeguards.

## Bottom line

The Mangle-driven ratchet is only as meaningful as the witness contract at its Go boundary. This path already has grouping, deterministic prioritization, scoped remeasurement and independent guards. The missing distinction is smaller and more actionable: **a compile diagnostic category must not become an analyzer-selection flag.** Fix witness admission and evidence classification before adding model reasoning to explain repeated failed repairs.
