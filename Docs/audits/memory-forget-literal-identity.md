---
title: "Memory forget: a unique folded match is not literal target identity"
doc_class: audit
subsystem: user-memory
status: proposed
last_verified: 2026-11-02
source_revision: d8bfd7068ed4922d25ef970ff41de448b510016c
method: static source inspection
supersedes: []
---

# Memory forget: a unique folded match is not literal target identity

## Finding and decision

The shipped user-memory forget path has important protections: a host-selected memory domain, ambiguity refusal, deletion by one stored row ID, exact live-projection retraction, and explicit partial-failure reporting. The residual issue is **which row becomes that ID**. The deterministic resolver removes every non-letter/non-digit distinction before its highest-priority “exact” comparison. A sole stored `I prefer C++` can therefore be selected for a correctly preserved request to forget `I prefer C#`. The resulting receipt names the requested C# text rather than the C++ value actually selected. This is a conditional wrong-record deletion and misleading acknowledgement, not a bulk-delete or permission-bypass claim. [Mutation and receipt](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L76-L140); [resolver and folding](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L147-L212).

**Recommended contract:** loss-tolerant matching may discover candidates; a unique candidate does not by itself establish the identity of a destructive target. Preserve meaningful symbols across value equality, substring matching, and distinctive-key matching. Bind any explicit resolution to the selected record, and report the actual selected value or a previously displayed record handle—not just an echo of the request.

This is a source-derived witness at the pinned revision, not a reproduced incident or an estimate of frequency. No tests, inference, application execution, or repository changes were performed. Examples are synthetic. The finding assumes healthy storage and correctly preserved perception output; it does not require concurrency, storage corruption, or malicious input.

## Existing work this does not reopen

The earlier [memory recall-and-forget audit’s shipped status](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/Docs/audits/memory-recall-and-forget-contract.md#L148-L152) records the implemented repair: consistent preference storage, legacy adoption, host-only durable-memory selection, single-ID deletion, exact live retirement, and scoped receipts. Those mechanisms are present and should remain. That audit already identifies disambiguation and broader erasure inventory as separate concerns. This follow-on addresses the residual literal-identity rule inside the newly centralized resolver, not the old absence of a working durable forget.

The separate archival-lifecycle issue is also not this witness. Current [RememberUserPreference](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/store/user_memory.go#L29-L69) renews `last_accessed` and retires an exact archived copy in the same transaction; the old [archival audit](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/Docs/audits/user-memory-archival-lifecycle.md#L1-L35) should not be read as proof that its original gap remains. Here an active, valid preference is selected incorrectly before an otherwise correctly scoped delete.

Store guidance expressly calls for reopened-store and negative-query checks rather than treating a row count as proof of runtime recall. That is the right evaluation standard for this residual issue too. [Store guidance](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/store/agents.md#L1-L18).

## Production path and preservation boundary

| Step | Pinned implementation and qualification |
| --- | --- |
| Shared session dependencies | Chat bootstrap takes its kernel, transducer, and local store from the shared Cortex boot. This is not merely an isolated helper API. [Shared bootstrap](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/cmd/nerd/chat/session_shared_boot.go#L90-L113). |
| User request interpretation | The TUI prefers the running perception firewall and otherwise calls `ParseIntentWithContext`; perception errors return. This is natural-language interpretation, not a guaranteed literal slash command. [Perception route](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/cmd/nerd/chat/process.go#L235-L255). |
| Target-to-operation conversion | When `ActionType` is `forget` and `Scope.Target` is nonempty, `extractMemoryOperations` copies that target directly into `Key` and sets `MemoryDomainUserMemory`. It does not strip `#` or `+` here. This does not prove a model always preserves the original user wording. [Memory extraction](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/perception/understanding_adapter.go#L695-L722). |
| Actual application | On the processing route that reaches the memory stage, chat calls `processMemoryOperations`, which applies the operations against the session kernel and local store. Earlier returns and differently classified requests are outside the witness. [Processing stage](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/cmd/nerd/chat/process.go#L445-L456); [application and notices](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/cmd/nerd/chat/process_helpers.go#L68-L80); [dependency accessors](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/cmd/nerd/chat/control_outcome.go#L48-L62). |
| Domain enforcement | Only the user-memory domain enters `forgetUserMemory`; other forgets address session notes. `Domain` has `json:"-"`, so a model control packet cannot set it through JSON. [Domain dispatch](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_ops.go#L90-L95); [host-only domain](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/articulation/protocol_types.go#L210-L219). |
| Candidate set | `UserPreferences` loads active cold-storage facts of type `preference`, the same type hydration uses. The primary witness assumes those rows are decoded successfully. It is not an inventory of archives, transcripts, vectors, or every stored memory. [Candidate type](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/store/user_memory.go#L9-L26); [typed reader](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/store/local_cold.go#L190-L236). |

The preserved-target condition is essential. A production journey must show that perception actually emitted `Scope.Target = "I prefer C#"`; a classifier that rewrites it, misses the action, or causes an earlier return exercises a different boundary. Once that target reaches the operation, the collision below is wholly deterministic.

## Resolution and the narrow witness

The resolver lowercases each target and argument, retains Unicode letters and digits, replaces runs of everything else with a space, and trims. It gathers three tiers: folded value equality; bidirectional folded containment subject to a four-rune minimum on the contained side; then folded equality with a non-generic record key. It returns the **first nonempty tier**, not a semantic confidence estimate. Multiple candidates in that tier refuse deletion; one proceeds. The four-rune rule constrains containment, not folded equality. [Constants](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L36-L41); [Tier selection and normalization](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L143-L212).

A sufficient single-threaded sequence is:

1. The active preference set has one relevant row, ID `r`, predicate `preference`, value `I prefer C++`; no competing row is in the winning tier. The normal remember writer stores the original value without this punctuation folding. [Value storage](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/store/user_memory.go#L42-L69).
2. A real user request is interpreted as a user-memory forget with target `I prefer C#`, and reaches the memory stage.
3. Both strings fold to `i prefer c`. The resolver places row `r` in its “exact” tier. With exactly one candidate, `forgetUserMemory` selects it.
4. With a healthy store, `DELETE FROM cold_storage WHERE id = ?` removes `r`. Its affected-row count can correctly be one. The count says nothing about whether C++ was the intended subject. [ID-based delete](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/store/user_memory.go#L76-L88).
5. If the selected row’s live projection is present and exact retraction succeeds, that C++ projection is retired. The shared projection helper preserves the original single-string argument; the kernel mutation is exact **for the selected row**, not for the request’s C# meaning. [Live retirement](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L129-L139); [Shared projection](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/core/virtual_store_predicates.go#L532-L560).
6. The applied receipt is `removed the saved long-term preference matching "I prefer C#"`. That is constructed from `target`, not `preferenceText(row)`. Chat appends the host receipt to its warnings as `[Memory] ...`. The source establishes this host notice; it does not establish every later model-generated response. [Receipt subject](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L113-L139); [Notice propagation](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/cmd/nerd/chat/process_helpers.go#L74-L79).

With no later reinsertion, fresh preference hydration will not obtain deleted row `r` from the active store. That is a consequence of the same typed reader and projection path, not a claim that every conceivable copy is erased or that a future model can never mention C++. [Preference hydration](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/core/virtual_store_predicates.go#L647-L662).

**Countercondition:** if both `I prefer C++` and `I prefer C#` are active candidates, both enter the folded exact tier. The caller reports two matches and removes nothing. That protection works; the failure arises when the lossy equivalence class contains only one stored candidate, even though the requested literal is different. Adding a literal-exact-first tier alone would help the two-row case but would not fix the sole wrong candidate if the old fallback still authorizes deletion.

Nor is fixing only the equality tier sufficient. A target such as `I prefer C# for tools` and sole value `I prefer C++ for tools at work` can satisfy the current folded containment rule. A distinctive key can lose meaningful punctuation too. Those are additional synthetic selector cases for evaluation, not claims about known user data. Safe identity checks must survive **all** fallback tiers rather than merely move the collision to a lower one.

## Working safeguards and remaining limits

- **No blind multi-row removal:** zero matches or multiple winning-tier matches return unresolved without deletion. The generic `preference` key is excluded from the key tier. This does not forbid the word “preference” from occurring in a value. [No/ambiguous-match handling](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L97-L112); [Key restriction](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L170-L184).
- **No arbitrary predicate retraction:** the persistent operation deletes one row ID; the live operation derives that row’s exact projection. A model packet’s forget stays session-note-only. These boundaries contain the scope but cannot repair earlier subject selection.
- **Real error states:** missing removable storage is unsupported, candidate-read and delete errors fail, and unavailable/failing exact retraction after durable deletion is partially applied. No rollback of the durable delete is implied. With a nil kernel, the function returns after persistent deletion. A zero-row delete reports already absent. [Error and partial-success behavior](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget.go#L85-L140).
- **Deliberate usability tolerance exists:** inspected tests cover two-value ambiguity, case/trailing-punctuation matching, short containment rejection, distinctive keys, and domain separation. These tests were read, not run. They explain why removing every tolerant match without replacement would be a usability regression. [Existing selector and domain controls](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget_test.go#L119-L185).
- **No affected-row counter defect is asserted:** one correctly deleted row and an applied disposition can coexist with a wrong target. `Took()` checks the disposition stage; it does not independently verify literal identity. [Disposition semantics](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/articulation/disposition.go#L65-L85).

The witness needs no concurrent writer or SQL injection. It is not a claim of forensic erasure, cross-user access, malicious prompt injection, or universal memory loss. Candidate-reader degradation, archived-only records, and other memory stores require separate contracts; the primary case uses an ordinary readable active row.

## Proposed identity and acknowledgement contract

1. **Keep original text authoritative.** Preserve the raw target, raw stored value, and an explicit match classification. Define which transformations are safe for automatic destructive selection. Case and sentence punctuation may be useful conveniences, but programming-language symbols, operators, paths, identifiers, and numeric punctuation cannot be assumed decorative. Do not silently rewrite stored content to manufacture equivalence.
2. **Separate discovery from authorization.** Exact original identity, or a narrowly specified safe equivalence, may select one record. Lossy equality, containment, distinctive-key aliases, or future embedding matches should produce candidates, not automatic permission to delete an unmatched literal. A single candidate can still require clarification. If no safe match exists, retain memory and expose the unresolved target.
3. **Bind a resolution to one record.** Let the person inspect the actual candidate and choose a host-bound handle. Do not send a free-form “yes” back through the same fuzzy resolver and assume it means the same row. If selection spans turns, define expiry and revalidate the selected record/content at mutation; a stale choice should conflict rather than target a different current record. This is proposed design, not an asserted concurrency exploit at this pin.
4. **Make the receipt describe the mutation.** Identify the actual selected value, or an unambiguous handle to the candidate already shown, plus saved/live status. If request and selected display differ, make that difference visible before deletion. Do not replace “matching requested text” with another success sentence that conceals the chosen record.
5. **Retain current scope and failure semantics.** Keep host-only domains, generic-key exclusion, single-ID deletion, exact projection construction, and honest partial-success reporting. A new resolver must not broaden forget into a bulk operation or promise removal from archives, histories, or external systems.

Stable resource identity versus a display label is a useful design analogy: SCIM separately defines provider-issued stable resource IDs and human-facing display names. CodeNERD is not a SCIM service, and this standard does not prove its row IDs have SCIM’s lifetime guarantees. It supports keeping identity distinct from a convenient textual search label. [RFC 7643 §3.1](https://www.rfc-editor.org/rfc/rfc7643.html#section-3.1), [§4.1.1](https://www.rfc-editor.org/rfc/rfc7643.html#section-4.1.1).

Unicode normalization is likewise not permission to discard meaning. UAX #15 distinguishes canonical and compatibility equivalence and cautions against indiscriminate compatibility normalization. CodeNERD’s custom punctuation fold is neither NFC nor NFKC; adopting a Unicode normalizer alone would not define safe destructive matching. [Unicode UAX #15, §§1.1–1.2](https://www.unicode.org/reports/tr15/#Normalization_Forms). The human-AI interaction guidelines’ recommendations to resolve uncertainty and support correction motivate a clarification path, not a measured claim about this product. [Amershi et al., CHI 2019, guidelines 9–11](https://www.microsoft.com/en-us/research/wp-content/uploads/2019/01/Guidelines-for-Human-AI-Interaction-camera-ready.pdf).

## Costs, recovery, and failure modes

| Choice | Benefit and cost to account for |
| --- | --- |
| Strict literal/symbol-preserving comparison | Reduces this class of destructive false positives; increases false negatives for paraphrases, typographical changes, and users who cannot recall exact wording. Measure clarification burden rather than calling refusal free. |
| Candidate presentation and explicit choice | Makes uncertainty inspectable; adds UI state, user time, accessibility work, cancellation handling, and potentially extra model turns. Prefer host-owned candidate handles over paying for a second semantic guess. |
| Richer matching policy | Requires multilingual and technical-token coverage, versioned behavior, and regression fixtures. Unicode normalization/case policy has complexity and dependencies; no universal punctuation allowlist is assumed correct. |
| Indexes or cached canonical forms | Current resolution loads rows and scans argument text. More representations add allocation/storage and invalidation work; a new index needs migration and compatibility rules. No speedup is established by this audit. |
| Cross-turn record binding | Adds state lifetime and stale-selection checks. Do not hold a database lock across a human reply or model call; use a short final validation/mutation boundary. Preserve existing partial handling for the separate kernel effect. |
| Truthful receipts and diagnostics | Better subject visibility can duplicate sensitive memory into conversation or logs. Keep candidate displays appropriately scoped and avoid permanent full-content audit trails merely to prove deletion. A displayed handle can avoid repeating the entire value. |
| Recovery | A person can deliberately remember a mistakenly removed preference again, but that requires noticing and knowing the content. Do not silently reinsert deleted content or claim backups/undo exist. Adding undo changes retention and privacy semantics and needs its own authorization/expiry policy. |

Correct selection does not make storage and kernel changes one transaction, cure a failed live retraction, or guarantee physical erasure. Conversely, more confirmations can make forgetting frustrating or incomplete. The evaluation must report both wrong deletions and legitimate requests left unresolved, not optimize only for fewer deletes.

## Fair evaluation proposal — not run

Use the pinned implementation as baseline and the proposed selector/receipt contract as the comparator, with identical synthetic stores, operation sequences, and injected errors. Keep classifier evaluation separate from deterministic matching so a favorable perception rewrite cannot hide the resolver defect.

| Case | Required observation |
| --- | --- |
| Sole C++ value; preserved C# target | Record selected ID and original value, disposition, affected rows, exact live facts, host receipt, and reopened-store contents. Baseline is expected from source to delete the wrong row; proposed behavior must retain it unless explicitly selected. |
| Both C++ and C# values | Preserve baseline ambiguity refusal as a control. If a new literal tier safely resolves C#, verify C++ survives in storage and the kernel. |
| Exact intended C++ request | Successful deletion of only the intended row; no blanket “reject technical punctuation” workaround. |
| Partial and distinctive-key fallback | Exercise meaningful-symbol differences in every tier, including sole-candidate cases, not just exact-fold collisions. |
| Existing tolerant use cases | Retain or deliberately document changes to case/trailing punctuation, distinctive-key aliases, short fragments, no match, and multiple matches. Include unrelated preferences and session notes as negative controls. |
| Unicode/technical literals | Paired cases for composed text, combining marks, symbols, paths, operators, and numbers, with expected identity specified in advance. Do not infer correctness from visual similarity alone. |
| Failure and repeat handling | Missing store, read/delete error, zero affected rows, missing/failing live retractor, repeat forget, and later explicit remember. Assert accurate domain and partial-state notices. |
| Real user journey | Drive the actual TUI route in a separately authorized evaluation; record the produced `Scope.Target`, route taken, selected record, and host-visible result. Attribute classification errors separately. |
| Proposed confirmation lifecycle | If implemented, evaluate cancel, expiry, changed/deleted candidate, and repeated confirmation without silently rerunning fuzzy selection. |

Follow the existing fresh-hydration and scoped-memory test pattern, but assert **which value survives**, not merely that the total fell by one. The existing tests already inspect live and freshly hydrated preferences; extend that strength rather than replacing it with selector-only checks. [Existing durable/live verification](https://github.com/theRebelliousNerd/codenerd/blob/d8bfd7068ed4922d25ef970ff41de448b510016c/internal/context/memory_forget_test.go#L76-L115).

Report wrong-record deletion rate, intended deletion completion, unresolved/clarification rate, receipt-to-mutation agreement, extra turns, latency, allocations, and retained sensitive display state. Use the same hardware, store sizes, candidate ordering, and error schedule for comparisons; isolate model cost and human-response time from deterministic lookup time. No correctness percentage, runtime improvement, or production incidence is claimed here.

**Acceptance boundary:** a successful durable forget must be attributable to the record the person actually selected, not merely the only record left after meaning-changing folding. Preserve the shipped memory-domain and exact-mutation safeguards while closing that earlier identity gap.

