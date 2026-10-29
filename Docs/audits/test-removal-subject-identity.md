---
doc-class: deep-dive
subsystem: session
implementation-status: proposed
last-verified: 2026-10-29
verified-against: b2be2799914ec942c90e08a7b487adc405c6428c
verification-method: static-source-inspection
supersedes: []
---

# Test removal subject identity

## Conclusion

The test-retention guard deliberately forgives a removed test when its production subject was deleted. The current lexical implementation can confuse unrelated production symbols with the same name. A deletion in one package can therefore exempt a missing test in another package even though that test's actual subject survives.

This is a bounded evidence-quality finding, not an observed end-to-end failure. The witness below assumes already-authorized writes, usable preimages, and otherwise valid source. It predicts the retention decision from source; no repository test, application-model inference, training, scanner, or implementation was run. Permission checks, target restrictions, and other completion gates remain authoritative.

The proposed extension is to preserve subject identity and survival evidence through the existing policy boundary. It should retain legitimate dead-code retirement without treating an arbitrary identifier match as proof that a missing test has become obsolete.

## Existing behavior worth preserving

The forgiveness rule addresses a real predecessor problem. N29 records that removing dead code together with tests whose only subject was that code was blocked by retention enforcement. The elite-harness account corroborates that intended retirement case. Requiring every historical test to survive would reinstate that problem. [N29][n29] [Harness account][ladder]

A separate fix already made moved-test recognition package-aware. The guard comments and movement controls distinguish a true same-package move from an unrelated namesake, including external test packages and build-tag exclusions. That is test identity: whether the removed test still exists somewhere valid. This proposal concerns subject identity: whether the production behavior that justified removing the test is actually gone. It should not be described as rediscovering or replacing the moved-test fix. [Predecessor F6][f6] [Moved-test scope][guard] [Movement controls][movement-tests]

## Where identity disappears

`removedTestFunctions` builds one deleted-production-function set from the written paths and preimages. It then considers tests missing from their original files. A missing test is omitted from the result when its name is present in the valid package test set **or** `testSubjectWasDeleted` returns true. [Candidate filtering][guard]

The production comparison is per file. For each written non-test Go file with a preimage, `deletedProductionFuncs` compares declarations before and after in that same file. It records bare function names and strips the receiver portion from method names. The resulting set does not preserve the package, receiver, or object whose removal produced a name. [Deletion collection][deleted]

The subject check parses the missing test's preimage and accepts any identifier token matching that set. It does not resolve that token to a declaration or establish that all behavior tested has disappeared. `funcDecls` uses `parser.SkipObjectResolution`; its token text covers the function and excludes comments. This is intentionally lexical evidence, not a type-checked subject graph. [Subject matching][subject] [Declaration parsing][parse]

Consequently, package collisions are not the only possible ambiguity. Receiver-name collisions, local bindings, and production functions relocated between files also deserve evaluation. These are source-derived candidate cases, not reported runtime observations.

## Conditional witness

Consider two packages in an admitted write turn:

- `alpha` contains a surviving `Reset` function and `x_test.go`. Its `TestReset` directly calls the local `Reset`; another test, `TestKept`, remains.
- `beta/y.go` contains an unrelated, unused `Reset`.
- The authorized change intentionally removes `beta.Reset`. While editing `alpha/x_test.go`, it also accidentally removes `TestReset`, preserving `TestKept` and valid imports.
- Preimages are available, current-file reads and parsing succeed, `alpha.Reset` is unchanged, and no other test is missing.

The production comparison contributes bare `Reset` to the global deleted-name set. The preimage of `alpha.TestReset` contains an identifier token `Reset`, so the subject check returns true. The missing test is excluded from the retention result even though its actual subject survives. That conclusion follows from the two filtering branches; it is not a claim that a model will choose these edits. [Filtering and deletion collection][guard] [Identifier acceptance][subject]

Keeping another valid test avoids conflating the witness with an entirely removed test file or empty-package behavior. Making the deleted function unused avoids an independent compile error caused by its removal. Neither condition guarantees overall acceptance: build, tests, coverage, pinning, critic, acceptance, or other gates can still reject the turn.

## Reachability and downstream evidence

The normal TUI mutation route calls `VerifyWithRetry`, which uses the verifier's observed execution, with session execution choosing isolated or inline handling and the inline path reaching `ProcessWithIntent`. Terminal verification enters `verifyCompletedToolTurn`; the shared post-edit round driver includes `removed_tests`. Round creation requires write effects and a kernel, and policy assigns the removal-check obligation to a turn that wrote. [TUI entry][tui] [Observed execution][observed] [Session execution][task-execution] [Terminal verification][terminal-verification] [Round driver][round-driver] [Round preconditions][round-preconditions] [Turn policy][turn-policy]

The removal gate itself requires a result, write effects, and preimages. If `removedTestsGateRed` is false, it exits before requesting a removal repair. The synchronization layer writes the verification marker and missing-test facts, then derives the gate state. Policy derives passing `/test_retention` when the marker exists and no removed-test facts remain. The host also has an empty-list fallback that is not red. Neither recovers a candidate already filtered out. [Gate entry][change-gate] [Fact synchronization][facts] [Retention policy][retention-policy]

Closure remeasurement is conditional on write effects, a completed build round, and snapshot state; its removal check also requires a prior `/removed_tests` round. When performed, it calls `removedTestFunctions`, and the executor has another call to the same helper. These are remeasurements, but not independent subject-identity checks. Repeating the same name-based rule cannot restore the identity discarded upstream. This supports a narrow false exemption in the retention evidence, not a blanket permission bypass or a claim that `/done` will pass. [Remeasurement conditions][closure-conditions] [Closure evidence][closure] [Executor reuse][executor-reuse]

## Counterevidence and current limits

The subject tests document the narrow retirement purpose and cover legitimate deleted subjects, surviving subjects, no production deletion, and whole-identifier near-name behavior. Ordinary retention tests cover a missing test turning the gate red and restoration returning it to green. Movement controls and a Cortex parity test provide additional relevant coverage. These files were inspected, not executed. [Subject controls][subject-tests] [Retention controls][retention-tests] [Movement controls][movement-tests] [Cortex parity][cortex-tests]

Those controls are valuable without demonstrating package or receiver resolution. Conversely, an unknown preimage or parse failure is not proof of an unrelated-subject exemption: the current logic consciously lacks affirmative evidence in such cases. The proposal should preserve that distinction rather than relabel every incomplete analysis as a compiler failure.

## Proposed identity preserving extension

Keep the current bounded Go analysis, but change the evidence it produces:

1. **Preserve provenance.** Record canonical module and package identity, source file, receiver where relevant, declaration identity, and before/after revision. File location is provenance, not the sole identity of a production symbol.
2. **Check survival across affected packages.** Index each affected package once for the relevant before and final views. Moving the same production object between files must not establish retirement.
3. **Resolve only justified references.** Start with unambiguous direct local-package references and imported selectors, including aliases. Support external test packages explicitly. An actual imported subject can legitimately be retired; a same-directory-only restriction would incorrectly forbid that case.
4. **Expose uncertainty.** Generics, interface dispatch, dynamic references, shadowing, mixed subjects, and unavailable configuration need explicit handling. Where the bounded resolver cannot prove the retirement relation, retain the obligation or surface a policy or human-review reason. Do not ask a model to confer authoritative symbol identity.
5. **Keep policy in control.** Emit raw missing-test candidates and exemption proof or uncertainty as typed facts from the Go AST or CodeDOM layer. Let Mangle policy decide exemption versus required repair under the actual task constraints, rather than erasing the candidate before kernel evaluation.

Useful classifications are moved, subject retired, unresolved, and retention required. “Subject retired” should require evidence that the test's relevant subject really disappeared, not merely that one helper it mentions was removed. Legitimate N29 retirement should remain possible without restoring tests for behavior deliberately removed.

The existing repair prompt and budget can remain. No new model call is inherent in collecting better host-owned evidence; the change is the quality and auditability of the facts feeding the existing decision.

## Cost and implementation tradeoffs

An affected-package AST approach needs before/final indexes, lexical binding information, and caches invalidated by revision and relevant configuration. A plausible work bound is proportional to changed bytes, affected-package bytes, and inspected references; this is a design estimate, not a measurement. Provenance and multiple views add memory costs.

A full `go/types` variant could resolve more cases but brings startup, dependency, build-tag, and toolchain work. It should be compared with a package-aware baseline and a bounded identity resolver, not assumed necessary for every edit.

Stricter evidence may produce more holds, repair rounds, tool use, and model tokens. Those costs can outweigh a cheap parser improvement. Measure the entire completion path, including false holds and user-visible correctness, before claiming a performance or reliability gain.

## Future evaluation

Evaluation is proposed only. Compare the current heuristic, a package-aware baseline, and identity-plus-survival evidence through the normal TUI route after admitted writes. A deterministic model double can make the written changes reproducible; exercise both native and Piggyback consumers of the shared decision. Capture raw missing tests, exemption reasons, derived gate facts, repair requests, and final outcomes separately.

The negative controls should include the cross-package `Reset` witness, identical method names on distinct receivers, local-variable shadowing, same-package production relocation, and a mixed-subject test with only one helper removed. Positive retirement controls should include a sole local subject and a genuinely removed imported subject reached through an alias.

Preserve same-package and external-test-package moves, near-name matching, and restored-test green behavior. Include a build-tagged namesake that is not a valid move; comments and ordinary string-only mentions without a matching identifier; deliberately unsupported non-Go and `init` cases; and explicit uncertainty for missing preimages, unparseable source, or current-file read failures. An independent red gate must stay red, and a permission-denied write must never dispatch.

Report false exemptions separately from false holds; parser/type-check wall time and peak memory; affected-package count; repair rounds, model tokens, and tools; and task completion and correctness. Do not infer a speedup or end-to-end safety rate from source inspection.

## Audit scope and open questions

The audit screened 92 current Markdown audit documents without finding this exact subject-identity issue. The research lane at `5e19781d7f852f1f896ff11d37673fe9a2f92295` adds distinct literal-fidelity, taxonomy-completeness, late-cancellation, and process-cleanup work. Selected active branch heads `7291bb758f847a0a628dccc470d38af34b74f8c1` and `cfabac55661500defd025f3ce5c686ccac91144d` were also checked: the guard, change gates, closure evidence, gate facts, and safety policy blobs were unchanged against the pinned main revision. This was not an exhaustive pull-request audit.

All source links below pin the inspected main commit; a stale search index was not treated as current source. The implementation remains proposed. Open questions include retirement semantics for mixed-behavior tests, renamed or replaced tests, and the proof threshold when configuration or dynamic dispatch prevents reliable subject resolution.

[guard]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_removal_guard.go#L18-L44
[deleted]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_removal_guard.go#L52-L94
[subject]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_removal_guard.go#L104-L123
[parse]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/pin_gate.go#L83-L163
[n29]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/Docs/journeys/07-external-audit-2026-09-19.md#L183-L190
[ladder]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/Docs/journeys/05-elite-harness-ladder.md#L111-L122
[movement-tests]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_removal_guard_test.go#L49-L91
[subject-tests]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_removal_subject_test.go#L9-L141
[retention-tests]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_retention_gate_test.go#L10-L64
[observed]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/verification/verifier.go#L286-L293
[task-execution]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/task_executor.go#L395-L441
[round-driver]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/executor_tools.go#L458-L576
[round-preconditions]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/executor_tools.go#L599-L613
[turn-policy]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/defaults/policy/turn_rounds.mg#L45-L70
[change-gate]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/change_gates.go#L451-L466
[facts]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/gate_facts.go#L503-L539
[retention-policy]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/core/defaults/policy/coder_safety.mg#L255-L276
[closure]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/change_evidence.go#L174-L177
[executor-reuse]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/executor.go#L2788-L2789
[f6]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/Docs/journeys/07-external-audit-2026-09-19.md#L419
[tui]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/cmd/nerd/chat/process.go#L560-L571
[terminal-verification]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/executor_tools.go#L229-L236
[closure-conditions]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/change_evidence.go#L70-L85
[cortex-tests]: https://github.com/theRebelliousNerd/codenerd/blob/b2be2799914ec942c90e08a7b487adc405c6428c/internal/session/test_retention_cortex_test.go#L14-L65

