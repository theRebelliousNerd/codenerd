---
doc-class: deep-dive
subsystem: kernel-git-system-facts
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6
verification-method: static-source-inspection
---

# Git system-fact transport preserves porcelain columns

**Status:** Source-supported residual defect; proposed contract, not implemented or runtime-reproduced here.  
**Baseline:** public `main` at [`a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6`](https://github.com/theRebelliousNerd/codenerd/commit/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6), inspected 2026-10-30.  
**Scope:** Git command output → kernel `git_state` → session Git context → optional churn guidance.

## Finding

The porcelain parser now correctly preserves its input's leading status column, but its production command wrapper removes that column first. `UpdateSystemFacts` obtains `git status --porcelain` through `gitCmd`; the latter applies `strings.TrimSpace` to successful output. If the first status row is ` M file.go`, the parser receives `M file.go`. Its two-character status is consequently `M `, and the worktree modification is not counted as unstaged. Subsequent rows retain their leading columns. [Command and parser wiring](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L53-L74), [transport normalization](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L153-L160), [column interpretation](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L202-L232).

This is a residual at the command-to-parser boundary, not a claim that the parser repair is absent. `splitPorcelainLines` deliberately avoids leading trimming and its comments explain exactly why the whitespace is data. The existing tests correctly cover raw ` M`, `M `, `MM`, `A `, ` D`, `??`, and CRLF input. Those tests call `parseGitStatus` directly and therefore do not exercise `gitCmd`'s earlier normalization. [Existing repair](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L163-L193), [existing regression fixtures](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/git_untracked_test.go#L73-L123).

## Exact source-derived witness

Use a valid Git checkout with a committed HEAD and exactly 21 tracked files changed only in the worktree, with none staged or untracked. Let status output consist of 21 ordinary rows beginning ` M ` and ending with newlines.

1. The command succeeds. The existing status-success gate therefore admits its output for parsing.
2. Whole-output `TrimSpace` removes the leading space from the first row only; the other 20 rows are unchanged.
3. The first row's `status[1]` is now a space, so it contributes zero. Each remaining row contributes one.
4. The kernel publishes `git_state(/unstaged_count, "20")`, despite 21 worktree changes.
5. Session context receives 20. The articulation compilation-context mapper requires `GitUnstagedCount > 20`, so this path does not set `IsHighChurn`.

The smaller witness is one modified tracked file: it publishes count `"0"`. The modified-file path still survives parsing; this is **not** a claim that the file disappears from all context or that the whole repository is represented as clean. The renderer can still list modified paths, but renders an unstaged-count line only when the count is positive. [Fact publication](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L96-L105), [rendering](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/articulation/prompt_assembler.go#L747-L764).

These are deductions from source, not outputs collected by running Git, codeNERD, tests or a benchmark during this inspection.

## Production producer and consumer chain

| Boundary | Current source |
|---|---|
| Chat refresh | `process.go` calls `m.kernel.UpdateSystemFacts()` and returns an error if it fails. [Lines 832–842](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/cmd/nerd/chat/process.go#L832-L842) |
| Sharded kernel | `CortexKernel.UpdateSystemFacts` forwards to each shard's real kernel and propagates errors. [Lines 769–779](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/cortex_kernel.go#L769-L779) |
| Host measurement | Successful porcelain output is parsed and its count is asserted as a string-valued `git_state` attribute. [Lines 59–74](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L59-L74), [lines 96–105](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/kernel_sysfacts.go#L96-L105) |
| Session projection | `populateGitContext` queries `git_state`, handles slash-prefixed attributes, parses `unstaged_count`, and assigns `SessionContext.GitUnstagedCount`. [Lines 618–657](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/cmd/nerd/chat/model_session_context.go#L618-L657) |
| Compilation flag | The articulation mapper sets `IsHighChurn` only above 20. [Lines 216–219](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/articulation/prompt_assembler.go#L216-L219) |
| Optional atom eligibility | `world_state/high_churn/analysis` has `is_mandatory: false` and requires world state `high_churn`; atom context matching checks required world states. [Atom](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/prompt/atoms/world_state/high_churn.yaml#L4-L8), [matching](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/prompt/atoms.go#L539-L589) |

The guaranteed source-level effect is incorrect measurement and the resulting flag on this mapper. Optional atom eligibility is narrower than final prompt inclusion: retrieval, relevance, conflicts, dependencies and token budget can still exclude an eligible atom. Conversely, a model may discover the changed files through other tools or existing modified-file context. No mandatory-guidance omission, constitutional permission bypass, observed bad edit, or measured token/latency regression is claimed.

## Existing safeguards and counterexamples

- Failed status commands do not get parsed as a fabricated zero count. The per-command success gates remain valuable and are not implicated by this successful-command witness.
- The transaction retracts old Git fact groups and publishes the new groups together on the ordinary successful path; this finding is not a torn-update claim.
- The parser's whitespace repair correctly handles every row it receives intact. The defect is upstream, and only the first record loses its leading status space through whole-output trimming.
- First rows beginning `??`, `MM`, `A ` or another nonblank index code are not changed at the front by `TrimSpace`; all-untracked and fully staged scenarios are controls, not reproductions of this witness.
- With 22 ordinary modified rows, the incorrect count is 21 and still crosses the current threshold. The 21-row case is intentionally the boundary where one lost count changes eligibility.
- `git_state` attribute-name normalization in the chat consumer is already correct; there is no need to reopen the earlier slash-prefix transport issue.

## Proposed contract and repair alternatives

**Contract:** transport must preserve every byte that a downstream structured parser treats as data. Command-specific normalization belongs after that command's format has been interpreted. A successful kernel unstaged count must equal the count implied by the unmodified porcelain records, including the first record.

The narrow repair is to preserve raw stdout for the status request and leave scalar trimming for branch, HEAD and repository-root values. This can be a format-specific command helper or an explicit raw-output path. Another option is to have the common helper return raw output and normalize each scalar consumer separately; that has a broader caller-compatibility surface and needs those callers checked. Do not repair the parser by guessing that a first `M ` was originally ` M`: genuinely staged first rows are observationally indistinguishable after the bytes are lost.

A separate robustness option is `--porcelain -z` with a matching record parser. That would address newline/quoting/path-format concerns but is not necessary to establish or fix this first-column defect and should not be quietly bundled as though already validated. Nor should this audit change the meaning or threshold of high churn: whether worktree-change count is the desired churn signal is a different policy decision.

Root guidance requires source-boundary facts to match consumers and emphasizes real production wiring. The current parser repair and tests should be preserved, then extended across the command wrapper rather than replaced. [Root guidance](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/agents.md), [core guidance](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/core/agents.md), [Mangle producer/consumer guidance](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/internal/mangle/agents.md).

## Future verification and fair cost accounting — not run

1. In an isolated real Git fixture, commit a file, modify it only in the worktree, call the actual `UpdateSystemFacts`, and assert `git_state(/unstaged_count, "1")` plus the preserved modified path. The fixture must traverse `gitCmd`, not only call the parser.
2. Exercise 20, 21 and 22 modified-file cases through the session-context and articulation mapper; check exact counts and high-churn eligibility separately from optional final selection.
3. Preserve staged-only, staged-then-modified, untracked-only, deletion, empty status, CRLF, and first-versus-later record controls. A staged first record must never be inferred as unstaged.
4. Retain status-error and unborn-HEAD behavior: status success is independent of branch/log success. Also check scalar root/branch/HEAD values if the common helper's return contract changes.
5. Distinguish parser tests, host-Git integration tests, kernel fact assertions, and optional JIT selection tests in the report. Read-only source inspection is not a pass of any of these stages.

No performance gain is presumed. A narrow byte-preservation repair should not need extra subprocesses or full-store evaluations, but verify that rather than claiming it. If comparing alternatives, count all Git subprocesses across shards, stdout allocation/copying, parsing time, refresh latency, kernel transaction/evaluation time, and test setup cost; keep the same repository size, status semantics and evidence fidelity. For model-facing evaluation, report correctly represented Git state and eligible/selected guidance first, then total turns, retries and tokens per correctly completed task. More accurate context may increase prompt tokens; fewer tokens from missing guidance are not an efficiency improvement.

## Prior-work and evidence limits

The inspection read root/core/Mangle guidance and existing Mangle audit material, including the [February integration audit](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/mangle-audit-2026-02.md), [kernel-performance audit and as-built updates](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/audits/mangle-kernel-performance.md), and [external-audit routing record](https://github.com/theRebelliousNerd/codenerd/blob/a2fd16a1ae32e6c2e9260ff8119c49dccf7504a6/Docs/journeys/07-external-audit-2026-09-19.md). Bounded repository searches for `porcelain`, `splitPorcelainLines` and `unstaged_count` located the source repair and tests but no audit of this remaining upstream trim. That is a bounded novelty check, not proof that nobody has previously noticed it.

Only public connector source reads were used for the analysis. No repository code was changed, no local Git or shell process was run, and no application execution, model inference, training, tests, profiling or benchmarks were performed.

## Primary format reference

The [Git status porcelain format](https://git-scm.com/docs/git-status#_porcelain_format_version_1) specifies two status columns; leading spaces in those columns are meaningful. It also specifies the alternate NUL-terminated format, which requires a corresponding parser.
