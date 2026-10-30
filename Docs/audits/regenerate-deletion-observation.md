---
doc-class: deep-dive
subsystem: generator-effect-observation
implementation-status: partially-implemented
last-verified: 2026-10-30
verified-against: 486db57ce490302c65d7f484e7f6070ee0e70862
verification-method: static-source-inspection
---

# Regenerate: preserve path membership when observing undeclared deletions

## Finding and scope

The declared-generator tool has a concrete observation gap: a generator can delete a previously clean tracked file outside its declared outputs without that deletion entering its `Undeclared` report. The cause is not Git's ability to report deletions. It is the comparison of a missing map entry with an empty digest used for an absent file.

**Status:** source-derived finding, not an executed reproduction. Reviewed public CodeNERD main `486db57ce490302c65d7f484e7f6070ee0e70862`. No implementation, tests, model calls, inference, training, or benchmarks were performed. This document concerns post-execution effect observation; it does not claim a sandbox escape, a permission bypass, or that the model can choose arbitrary generator commands.

## 1. Production mechanism and existing safeguards

[`RegenerateTool`](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/tools/shell/regenerate.go) exposes a generator ID, optional timeout, and output paths. `declaredGenerator` loads `nerd.md` and rejects missing IDs or missing declarations. `expandRegenerateArgs` fills paths from the declaration; supplied paths must match those outputs. `executeRegenerate` checks that expansion again, parses the declared command, and runs it through `build.WorkspaceCommand` and `processutil.Run` under a timeout.

This is not an unreachable helper: [`shell.RegisterAll`](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/tools/shell/register.go) registers `RegenerateTool`. [`tools/effects.go`](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/tools/effects.go) classifies it as `EffectWrite`, and [`projectdoc/tool_gate.go`](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/projectdoc/tool_gate.go) recognizes it as a write mutation. These safeguards remain relevant; registration alone does not grant a particular turn permission or capability.

Before and after the process, the handler takes two observations:

- `fileDigests` hashes declared outputs to classify them as changed or unchanged.
- `dirtyDigests` asks Git for `status --porcelain=v1 -z --untracked-files=all`, then hashes each reported path to detect changes outside the declared set.

The latter is a post-run detector. An undeclared effect has already happened when it is reported. Detection is not prevention, and the reviewed handler does not roll it back.

## 2. Exact failure mechanism

In `fileDigest`, any `os.ReadFile` error returns the empty string. The digest of a successfully read empty file is a nonempty SHA-256 hex string, so an empty *file* is not the same case as an absent file. The relevant collision is **missing map key versus failed/absent file observation**.

The after-run comparison is effectively:

```go
for p, digest := range after {
    if !declared[p] && before[p] != digest {
        result.Undeclared = append(result.Undeclared, p)
    }
}
```

A minimal source-derived witness is:

| Boundary | Undeclared tracked file `other.txt` | Dirty map |
|---|---|---|
| Before | Exists and matches HEAD | No `other.txt` key |
| Generator | Deletes `other.txt`; exits successfully | — |
| After | Git reports deletion; reading the path fails | `other.txt: ""` |
| Comparison | Missing `before["other.txt"]` yields Go's zero-value string | `"" != ""` is false |

The subsequent loop only checks paths that existed in the **before** dirty map and disappeared from the after map. It cannot recover this case: the initially clean file had no before-map entry.

If there are no other undeclared effects and the process succeeds, the handler can return a successful report without naming this deletion. This is narrower than claiming all deletions are missed: deleting an initially dirty, readable file normally compares a nonempty before digest with an empty after digest and is detected.

## 3. Existing evidence and its limits

[`regenerate_test.go`](https://github.com/theRebelliousNerd/codenerd/blob/486db57ce490302c65d7f484e7f6070ee0e70862/internal/tools/shell/regenerate_test.go) covers declared-output expansion, ordinary declared copying, and an undeclared copy/write. The undeclared-write test changes a clean file to readable bytes, producing a nonempty digest and exercising the successful branch of the detector. It does not establish the clean-to-deleted case above.

The mechanism is deterministically implied by the map lookup and sentinel representation, but the audit did not execute the proposed witness. Actual production occurrence depends on a declared generator performing such a deletion, having permission to execute, and its exit status being successful. No frequency, exploitation, user impact, or measured resource saving is claimed.

A related, distinct limitation is explicitly represented by `UndeclaredUnchecked`: failure of either Git-status observation records a reason, but does not by itself turn an otherwise successful invocation into an error. That is a policy choice requiring an explicit contract, not proof of the membership bug. It should not be silently conflated with a fully observed successful run.

## 4. Proposed contract

For every candidate path outside the declared output set, distinguish:

1. whether it was in the before/after dirty-path set;
2. whether the filesystem object exists;
3. whether its relevant state was observed successfully;
4. what content/state identity was observed.

An observed clean-to-deleted transition must produce a named undeclared effect. An observation error must remain unknown rather than being interpreted as unchanged. A process exit code of zero is a process outcome, not proof that all effects matched the declaration.

The report should separate process outcome, declared-output changes, observed undeclared effects, and observation coverage. Downstream consumers should not have to infer coverage from an empty `Undeclared` list.

## 5. Repair options and costs

### Minimal correction: preserve map membership

Check the two-value map lookup. A path newly appearing in Git's dirty set outside the declared outputs is a changed candidate even when its digest is empty. This repairs the exact clean-to-deleted case with negligible additional comparison work and no additional process invocation.

It does **not** solve all state-identity problems. Read failures still collapse together; preexisting deleted versus newly unreadable paths may remain indistinguishable. Tests should describe this as the narrow fix rather than a complete effect inventory.

### Stronger observation: typed file state

Represent observed state with explicit presence, read outcome, digest, and—if required by the intended contract—object kind and executable mode. Preserve errors rather than converting them to a digest sentinel. Compare the union of dirty paths with set membership intact.

This adds state-handling complexity and metadata reads. It need not add another Git process. Ordinary hashing remains proportional to bytes of dirty files read before and after, not the entire repository. Mode-only changes, symlinks, and unreadable paths need deliberate semantics rather than accidental behavior.

### Wider inventory or containment

A wider filesystem inventory could observe effects outside Git's normal status scope, but costs more I/O and still does not attribute concurrent writes to the generator. Isolation or write confinement is a different architectural project with platform and compatibility costs; do not describe this detector repair as implementing it.

For any option, do not automatically replay a successful generator to repair a report, and do not automatically restore an undeclared path without an exact owned preimage and conflict policy. Those actions can duplicate effects or overwrite concurrent work.

## 6. Acceptance matrix for an implementation owner

These are proposed tests; none were run for this document.

| Case | Required observation |
|---|---|
| Clean tracked undeclared file deleted | Named undeclared path and non-successful contract outcome |
| Initially dirty readable undeclared file deleted | Still detected |
| Declared output deleted | Declared changed state, not undeclared; separate policy decides whether output absence is acceptable |
| Clean undeclared file rewritten with readable bytes | Existing behavior preserved |
| Preexisting unrelated dirty file untouched | No new undeclared effect |
| Preexisting dirty file restored to clean | Existing reversion detection preserved |
| Missing versus unreadable file | Explicit distinction or documented narrow-fix limitation |
| Git status unavailable before/after | Explicit incomplete coverage, never implied complete validation |
| Generator exits nonzero or times out after deletion | Preserve both process failure and effect evidence where observation succeeds |
| Concurrent unrelated writer | Report observational limits; do not claim causation from temporal comparison alone |

Use a tiny temporary Git repository and a deterministic local generator fixture. Assert both the error/result contract and the exact named path; checking only the exit code can miss the reporting defect. Add a direct comparison-unit case so the missing-key/empty-value distinction is obvious even without a subprocess fixture.

## 7. Efficiency and fair evaluation

The immediate benefit sought is correct effect evidence, not a claimed speedup. Compare existing and corrected detectors on the same worktree states and generator outcomes. Track missed undeclared transitions, false positives on preexisting dirt, explicit unknown observations, bytes hashed, Git invocations, wall time, and peak report size.

The current implementation already performs two Git-status calls and hashes dirty-path content on both sides. A membership-only correction should retain that work profile; a typed-state version adds metadata/error handling. Any savings from fewer repair turns remain hypothetical until measured end to end. Large dirty worktrees, process output buffering, concurrent mutations, and non-Git workspaces remain separate cost and correctness considerations.

## Decision

Fix the missing-membership comparison first, preserve the distinction between unknown and unchanged, and keep the claim precise: **a declared generator's observed undeclared deletion must not disappear because two different states share an empty string.**

## Primary format reference

[Git status porcelain version 1](https://git-scm.com/docs/git-status#_porcelain_format_version_1) defines worktree deletion status and NUL-terminated output used by the detector.
