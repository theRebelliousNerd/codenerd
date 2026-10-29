---
doc-class: deep-dive
subsystem: tools
implementation-status: proposed
last-verified: 2026-10-29
verified-against: 9dcb01b9736bf4763aa3ac391d29f133d73c6611
verification-method: static-source-inspection
supersedes: []
---

# Revision sentinel build exclusion parity

## Conclusion

The projected build-exclusion guard and the CodeDOM element-edit handler interpret optional revision strings differently. The projector skips its proposed-byte check when a nonempty revision differs from the current revision, assuming the handler will reject the edit. The handler instead treats recognized omission sentinels such as `"None"` as no revision requirement.

For an otherwise admitted header edit, this disagreement can leave an eligible new build exclusion unchecked by that mechanical guard. Repeating the same projector at another guard boundary does not supply an independent defense. The finding is conditional and source-derived: no mutation, test, model inference, training, or scanner was executed, and no overall completion or build success is claimed.

This is **not a newly introduced regression**. The earlier `f84ce0df57b21de3725b8fa02d13e88f74192714` handler already accepted `"None"`. The later normalization change centralized and narrowed sentinel handling; the existing semantic mismatch remains at inspected main `9dcb01b9736bf4763aa3ac391d29f133d73c6611`. [Earlier revision handling][earlier-revision] [Current revision handling][revision] [Projection shortcuts][projection]

## The two revision interpretations

`checkRevision` accepts an omission sentinel immediately. Otherwise it trims the supplied revision and compares it with the actual revision. The registry's optional-argument normalizer preserves string values for fields declared as strings, and `revision` is a string property. Consequently, the registry does not remove `"None"` before the projector sees it. [Handler semantics][revision] [Revision schema][revision-schema] [String preservation][normalization]

Both `projectEditElement` and `projectReplaceElement` contain an earlier shortcut: a nonempty revision unequal to the element revision returns without projecting proposed bytes. That precedes `codemodel.Apply` and `RejectAddedBuildExclusion`. The guard's stated contract permits skipping when the handler is known to refuse, but that premise is false for an accepted sentinel. [Projector contract][projection-contract] [Edit and replace projection][projection]

`RejectAddedBuildExclusion` applies to Go files and compares added constraints in the original and proposed bytes. It refuses a newly added exclusion when the proposed file is excluded and the original was not. Its tag evaluator explicitly treats `ignore` as unsatisfied. The witness's separate build-context assumption establishes the expected toolchain inclusion change, not merely the guard's classification. [Exclusion predicate][exclusion] [Constraint evaluation][constraint-evaluation]

The distinction is narrow:

| Revision input | Projector decision | Handler interpretation |
| --- | --- | --- |
| Omitted or empty | Project proposed edit | No revision requirement |
| Exact current revision | Project proposed edit | Matching revision |
| Genuine stale revision | Skip projection | Reject stale revision |
| Recognized `"None"` sentinel | Skip as nonempty mismatch | No revision requirement |
| Current revision padded with whitespace | Skip raw mismatch | Trim and accept |

The padded-current-revision row is another source-derived control, not an observed incident. The primary witness uses `"None"`, whose omission meaning is explicit.

## Conditional header edit

Start with an included `a.go`:

```go
package p

func A() int { return 1 }
```

Assume the effective build context does not satisfy the tag `ignore`. Consider this `edit_element` input:

```json
{
  "path": "a.go",
  "ref": "header",
  "old": "package p",
  "new": "//go:build ignore\n\npackage p",
  "revision": "None"
}
```

The witness requires a live context, a resolvable header reference, matching old text, and no foreign file change. Capability checks, project restrictions, write permissions, repair-scope rules, and Dreamer restrictions must all admit the operation. It does not bypass an earlier denial.

The projector resolves the header but sees a nonempty revision unequal to its actual revision, so it returns before evaluating the proposed header. The handler's revision check treats `"None"` as absent, applies the edit, and reaches the commit path. The source therefore supports a missed build-exclusion check for this admitted operation, subject to the stated remaining conditions. It does not establish that a real task or model will choose the request. [Projected edit][project-edit] [Handler execution][edit-handler]

Go's official documentation explains that build constraints determine file inclusion and that an unsatisfied conventional `ignore` constraint excludes a file. The new header is meaningful build configuration, not merely the word “ignore” in a body comment. These references establish language and command semantics, not the installed runtime or an executed build result. [Build constraints](https://pkg.go.dev/cmd/go#hdr-Build_constraints) [Pinned inclusion semantics][go-inclusion] [Pinned ignore convention][go-ignore]

## Why the remaining checks are not equivalent

The session write guard calls the projected build-exclusion check before execution. The registered virtual-store write guard calls the same projector. Both locations are useful defenses, but both inherit this revision shortcut; counting them as two independent checks would overstate coverage. [Session guard][session-guard] [Registered guard][registered-guard]

The element handler applies a structured edit. `codemodel.Apply` parses and formats source, handles imports, and checks preservation of sibling elements. Its successful parse verdict does not establish that the file remains included under the active build context. A syntactically valid build constraint can change inclusion without breaking those invariants. [Structured application][structured-apply] [Parse verdict][parse-verdict]

Commit performs live-context, dependency, original-byte, and concurrent-write checks before writing. Those safeguards matter, but the inspected commit path does not independently apply the build-exclusion rule to the final bytes. Thus they do not restore the specific projection check skipped on revision grounds. [Commit safeguards][commit]

This does not imply repository-wide absence of an exclusion defense. The Go syntax guard includes such a check on line-editing paths. Existing tests explicitly distinguish those paths from structural element edits. Other post-edit proof or acceptance gates may detect the changed workspace, repair it, or return an honest not-done result. No false-green or universal permission escape follows from this finding.

Syntax validation parses Go source, and CodeDOM validation also checks target-element existence. That post-execution target check may reject the operation independently; this audit does not assert that every validator accepts a header edit. It is not a substitute for the required before-write inclusion invariant. Dreamer admission must likewise remain satisfied; its projected target and graph effects do not supply the skipped proposed-byte comparison. [Syntax validation][syntax-validator] [CodeDOM validation][codedom-validator] [Projected Dreamer effects][dreamer] [Syntax guard][syntax-guard] [Element-path distinction][path-tests]

## Counterevidence and historical scope

A genuinely stale revision is a valid reason to expect handler rejection. Omitted or empty revisions do not take the erroneous shortcut, so guarded header controls can correctly reject the exclusion. Tests cover both edit and replacement controls without the sentinel. They also permit edits to already ignored files and harmless body mentions. Those permissions should remain: prohibiting every occurrence of build-tag text would be a different and overbroad contract. [Guarded edit control][edit-control] [Guarded replacement control][replace-control] [Benign controls][benign-controls] [Registered guard control][registered-control]

These tests were inspected, not run. Their existence does not demonstrate sentinel parity. Nor does the unguarded structural-edit fixture by itself prove a production failure; it helps establish that the handler's syntax route is not an independent inclusion guard.

A source-only screen of 92 current main audit documents found no exact projected-build-exclusion/revision-sentinel finding. The earlier edit-observation precondition audit concerns transferring observed revisions across batch edits, not a projector treating an accepted revision sentinel as proof of future refusal. This memo does not supersede that work. Three selected active lanes retained the same projector and commit blobs; this was not an exhaustive pull-request audit. [Adjacent precondition audit][predecessor]

Chronology matters: earlier revision handling recognized `"None"` and additional spellings. The current centralization did not create the primary witness. Any proposed repair should target the shared semantic contract, not describe a blanket rollback of the recent normalizer as the solution.

## Proposed shared revision semantics

The smallest correction is one semantic revision decoder used by both projection and execution. Classify input as absent, exact, stale, or unsupported using identical trimming, sentinel, and type rules. Only skip projection when that shared result proves the handler will refuse. A real stale revision must still produce an error; compatibility must not become permission to ignore an actual conflict.

A stronger design also shares one proposed-edit representation: resolved element identity, source revision, normalized revision requirement, proposed bytes, and relevant build context. The guard and handler should not independently reconstruct meaning from different forms of the same input. Preserve original payload provenance alongside the effective interpretation.

As defense in depth, consider checking final proposed bytes at the commit boundary with the workspace's effective build context. Preserve the existing distinctions between a newly excluded file, an already excluded file, a satisfied constraint, and a harmless mention. This is not a mandate to parse every possible write through the full CodeDOM path or reject all build-tag text.

Do not remove existing guards, bypass authorization, relax header resolution, or weaken concurrency checks. The host should deterministically establish revision and proposed-byte facts; a model should not decide that incompatible preflight and handler interpretations are equivalent. Successful mutation evidence must describe the one actual commit, not require replaying an edit.

## Cost and future evaluation

Shared sentinel classification and trimming are small operations proportional to revision-input length. Sharing parsed state and proposed bytes may avoid duplicate work; rebuilding them independently adds work proportional to affected source bytes. A commit backstop adds constraint inspection and build-context handling; the existing helper scans original and proposed text, so budget work proportional to those bytes rather than assuming constant-time header access. Keep provenance and receipt storage bounded. No latency, reliability, or token improvement has been measured, and no new model call is inherently required.

Future evaluation should compare the current baseline, shared revision semantics, and shared semantics plus a commit backstop. Exercise guarded direct-registry calls and the normal native and Piggyback serving paths using identical fixtures and build contexts. Deterministic requests or a deterministic model double can isolate admission behavior.

Include recognized sentinel variants, omitted and empty revisions, exact and padded current revisions, genuine stale revisions, and meaningful non-sentinel strings. Pair header edits and replacements with ordinary body edits. Test already excluded files, satisfied versus unsatisfied new tags, invalid or duplicate constraints, and body-only mentions.

Preserve denials for permissions, protected targets, repair-scope restrictions, canceled contexts, unresolved references, and foreign writes. Record projector classification, handler classification, skip reason, proposed bytes, actual file bytes, guard results, and commit count. A genuinely rejected operation must not mutate; a permitted ordinary edit should remain possible.

Measure classification disagreements, missed exclusion checks, false rejections, parse count, bytes inspected, wall time, memory, repair rounds, and model tokens. Keep downstream build or final-task outcomes separate: the primary success criterion is parity at the mechanical boundary, not a requirement to demonstrate false completion. All evaluation remains future work.

[earlier-revision]: https://github.com/theRebelliousNerd/codenerd/blob/f84ce0df57b21de3725b8fa02d13e88f74192714/internal/tools/codedom/element_edit.go#L177-L205
[revision]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/codedom/element_edit.go#L177-L194
[revision-schema]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/codedom/element_edit.go#L54
[normalization]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/omitted_args.go#L22-L58
[projection-contract]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_project.go#L14-L37
[projection]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_project.go#L262-L328
[project-edit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_project.go#L262-L298
[edit-handler]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/codedom/element_edit.go#L345-L381
[go-inclusion]: https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/cmd/go/alldocs.go#L2034-L2050
[go-ignore]: https://github.com/golang/go/blob/6e676ab2b809d46623acb5988248d95d1eb7939c/src/cmd/go/alldocs.go#L2141-L2147
[session-guard]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/session/write_guard.go#L211-L215
[registered-guard]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_write_guard.go#L94-L98
[structured-apply]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/world/codemodel/edit.go#L66-L157
[parse-verdict]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/world/codemodel/edit.go#L192-L199
[commit]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/codedom/commit.go#L86-L149
[syntax-guard]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/go_syntax_guard.go#L28-L39
[path-tests]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_paths_test.go#L43-L64
[edit-control]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_paths_test.go#L67-L80
[replace-control]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_paths_test.go#L129-L141
[benign-controls]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion_paths_test.go#L97-L125
[registered-control]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/virtual_store_build_exclusion_guard_test.go#L83-L94
[predecessor]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/Docs/audits/edit-observation-precondition-continuity.md
[exclusion]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion.go#L29-L65
[constraint-evaluation]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/tools/build_exclusion.go#L212-L280
[syntax-validator]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/validator_syntax.go#L155-L159
[codedom-validator]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/validator_codedom.go#L141-L169
[dreamer]: https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/core/dreamer.go#L435-L474

