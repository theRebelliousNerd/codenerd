---
doc-class: deep-dive
subsystem: knowledge-refresh
implementation-status: proposed
continuation-of: knowledge-refresh-prune
last-verified: 2026-10-30
verified-against: 388205138ba6d47c1cff0c5e97c896dfa642a391
verification-method: static-source-inspection
---

# Retire repository knowledge when a complete snapshot has no importers

**Decision:** distinguish a trustworthy complete empty repository snapshot from a failed, unavailable, partial or unproductive acquisition. A complete replacement can contain zero rows. Preserving prior knowledge on generic empty responses is intentional and useful, but it cannot also be the only representation of confirmed source retirement.

**Finding:** after a module's final import disappears from a correctly refreshed world model, the repository knowledge source returns an empty successful result. The acquisition layer preserves its old positive importer list, changes the receipt to `empty`, and removes that source from the normal acquired-source age-refresh rule. The old list can therefore remain available to expert retrieval even though the current source no longer supports it.

This is a **scoped continuation of [knowledge refresh and pruning](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/knowledge-refresh-prune.md)**. The earlier generation fence protects newly acquired rows from an old prune decision. This continuation addresses the opposite boundary: a complete empty replacement cannot retire the old rows at all. It does not reopen the fixed generation-fenced deletion or recommend deleting knowledge after every empty response.

**Evidence status:** static inspection of public main `388205138ba6d47c1cff0c5e97c896dfa642a391`, rechecked on 2026-10-30. No application, tests, provider request, model inference or database experiment was executed. This is a conditional source witness, not an observed production incident, proven frequency estimate or measured answer-quality regression.

## The important qualification

The current source interface does **not** carry a world-snapshot completeness receipt. `KernelImports.Importers` can distinguish a missing kernel and a query error from a successful query, but a successful empty query alone does not establish that scanning covered every relevant file. `RepoSource.Available` only checks whether the importer callback is wired. [Importer query](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L118-L148), [availability and acquisition](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L259-L284).

Accordingly, the witness requires a complete, current and stable world snapshot with successful relevant parsing and publication. It does not infer repository-wide absence merely from `len(files)==0`. The implementation gap is that **even when the empty observation is trustworthy, the acquisition contract has no way to preserve that distinction and retire the superseded positive material**. An implementation must first establish the scope and completeness needed to authorize replacement; it must not invent that authority from nil error alone.

## Source to consumer trace

### The repository source makes a current positive claim

Production `NewAcquirer` wires `RepoSource.Importers` to the kernel importer reader. That reader queries `dependency_link`, matches exact module paths or their package descendants, deduplicates files and returns them. Nonempty acquisition produces one atom with concept `repo/<module>/importers`, source `repo/world`, confidence 0.9, and text saying the module is imported by those files and recommending them as project conventions. It is not labeled a historical list. [Production wiring](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L79-L102), [import enumeration](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/wiring.go#L125-L148), [positive atom](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L285-L292).

This is reachable in normal knowledge acquisition. Init builds manifest/domain subjects and calls `knowledge.Fill`; normal Cortex boot constructs the same typed-source acquirer for the background refresher. [Init](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/init/agents_knowledge.go#L130-L149), [refresher construction](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh_boot.go#L38-L58).

### Empty results preserve the old material

If the importer callback succeeds with zero files, `RepoSource.Acquire` returns `nil, nil`. The acquirer maps no error plus zero atoms to `StatusEmpty`. Replacement only considers `StatusAcquired`, so it does not remove the earlier importer atom. When writing outcomes, the non-acquired path explicitly keeps the prior concept list but updates status, atom count, detail, attempted time and version. [Empty producer result](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/sources.go#L271-L284), [classification](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L170-L188), [replacement and ledger update](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L267-L318).

This policy is deliberate: failures and empty research results are not evidence that the last useful answer should be destroyed. The mistake would be treating the intentional retention rule as missing code without recognizing that it protects valuable material. The unresolved composition is source-specific: an exhaustive current importer set can legitimately shrink to empty, while an empty web response is generally not an exhaustive replacement of prior knowledge.

The nonzero boundary already works differently. A successful repository result shrinking from two importers to one still produces an atom, is marked acquired, and replaces its prior list. The final transition from one importer to zero takes the retention path instead.

### Policy follows the new empty receipt

Measurement reads acquisition status and age from the ledger independently of held knowledge rows. `knowledge_acquired` means some source acquired the subject. The stale rule requires a source outcome of `/acquired`; retry requires `/failed`; new availability requires `/unavailable`. The empty repository outcome matches none of those states. [Measurement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/measure.go#L35-L83), [acquired and recent attempt](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L37-L43), [refresh rules](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L64-L105).

When another source still successfully covers the subject, subject-level acquired status also suppresses `/unlearned`. Thus the old atom remains while the repository source's ordinary age-based remeasurement disappears. This does not mean no future work can occur: another source may become stale, fail or become available, a gap may arise, the dependency version may change, or a later acquisition may produce a new positive list.

### The retained row remains on the expert retrieval path

The JIT collector retrieves selected and pushed expert knowledge. Conversion renders stored content with knowledge source identity and confidence as optional prompt atoms. These are valuable qualifications, but the old importer claim is not automatically withdrawn merely because its separate acquisition receipt became empty. Final inclusion still depends on retrieval, policy and fitting; a stale stored row does not prove a stale row was delivered or acted on. [Knowledge collection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L349-L386), [retrieval and conversion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/prompt/compiler_db.go#L425-L466).

## A clean complete zero witness

Use an ordinary root Go module requiring `github.com/spf13/cobra`, with a ready agent that retains its Cobra domain.

1. A successful initial acquisition stores a repository-source list containing the sole importing file. GoDoc also successfully acquires the same module.
2. Eight whole days later, remove the final import. Keep the manifest requirement and expert domain unchanged, so the subject remains wanted and no version change is involved.
3. Complete a successful world refresh. All relevant files are supported, scanned and parsed, publication succeeds, and no other matching import remains. Hold the source stable through knowledge acquisition.
4. The repository receipt is older than its seven-day threshold, so `/stale` derives. Give this subject room inside the existing refresh budget.
5. On the selected refresh, repository enumeration returns zero files; GoDoc and any other configured successful sources remain acquired. There are no unresolved gaps or unrelated failure/availability triggers.

Normal chat scanning calls `Scanner.Refresh`. Its pending-publication path applies incremental results; the kernel applier retracts the prior file facts and loads current ones. The witness requires those operations to succeed. It therefore does not rely on an unfixed world-cache or graph-projection retirement bug. [Scan entry](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/cmd/nerd/chat/helpers_scan.go#L155-L170), [world publication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/world/refresh.go#L113-L135), [kernel delta](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/world/apply_incremental.go#L60-L78).

After step 5, the current kernel has no matching importer, the repository ledger says empty, and the expert KB still contains the old positive list. At the next measurement, that receipt no longer matches repository stale admission. Other successful source outcomes prevent subject-unlearned admission. This is the specific contradiction.

A later web or Context7 stale event may re-run all sources, but repeated repository emptiness still preserves the same old list. If a nonempty repository result eventually reappears, normal replacement can repair it. The claim is neither inevitable permanent staleness nor inevitable bad model output.

## Existing safeguards remain necessary

- **Freshness and scheduling are real.** Repository acquisitions age after seven days, web after 30 and Context7 after 90. The source rule uses whole-day age strictly greater than the threshold; eight days isolates it. The schedule starts after 120 seconds, repeats hourly, and admits at most four subjects per run. [Constants](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L8-L26).
- **Retry is not absent.** Failed outcomes wait at least one day; unlearned and gap work reject recently attempted subjects. The problem is the semantics of a complete empty result, not a blanket missing retry window. [Admission](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L72-L105).
- **Pruning is not freshness repair.** It requires age over 60 days, an agent with at least 20 retrievals, and a concept never retrieved. It cannot address the eight-day witness. If the importer list was previously retrieved, that positive usage also prevents this pruning rule from retiring it later. [Pruning](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/core/defaults/policy/knowledge_refresh.mg#L110-L124).
- **The existing generation fence works for its purpose.** A prune plan is bound to the measured generation, preserving a newer replacement. An authoritative-empty repair must keep that protection and must not turn a stale negative receipt into authority to remove newer material. [Generation-fenced deletion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/system/knowledge_refresh.go#L174-L189).
- **Generic retention and subsequent Fill are intentional.** Fill skips acquired sources but may ask an empty source again on later initialization. A successful nonempty answer can replace old material. Neither fact makes a repeated empty result retire the prior positive claim. [Fill and Refresh contract](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L207-L225), [skip state](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/acquire.go#L250-L268).

## A scoped repair

First define what makes repository enumeration complete for a module and source generation. A receipt should bind the workspace, relevant scanned scope, observed generation, completeness/disposition and resulting membership. Go measures those properties; Mangle determines whether that evidence authorizes current coverage, replacement or another attempt. A wired callback and nil query error are not substitutes for this contract.

Two representations are defensible:

**A qualified negative atom.** A trusted complete zero result can produce one current observation saying no importers were found within the named complete scope. It replaces the prior positive list and remains an acquired repository observation eligible for the ordinary seven-day refresh. This is a small downstream change, but may add negative material to prompts and should not overstate an indexed-scope result as a repository-global fact.

**A typed complete-empty replacement.** Keep zero knowledge rows, but record successful complete coverage and its generation separately from atom count. Replacement may retire only the previously owned importer material even when there is no new atom. Policy must age complete-empty coverage too, so later added imports are discovered rather than leaving an empty subject permanently settled. This avoids prompt clutter but broadens the result, ledger, policy and reporting contracts.

For incomplete, failed or unavailable observations, keep the useful last material without claiming it was freshly confirmed. Preserve its last-success provenance and freshness separately from the latest attempt. An unverified retained row can remain historical or qualified rather than serving as an unqualified current importer assertion. Do not repair this by converting every generic empty response to acquired, by retrying every empty source every hour, or by deleting all concepts mentioned in an empty outcome.

Replacement must remain source/subject-owned and generation-bound, with material and the receipt committed coherently. The separately audited [subject-ownership collision](https://github.com/theRebelliousNerd/codenerd/blob/77069960b69aba14a67dbd682d6f65435e8ada50/Docs/audits/knowledge-acquisition-subject-ownership.md) is still relevant implementation guidance; this continuation does not require a collision to demonstrate its own problem.

## Migration and costs

Existing empty receipts cannot be retroactively treated as proof of complete absence: they may come from a short web answer, incomplete index, failed-to-cover scope or genuinely empty complete repository snapshot. Mark legacy repository empties as unverified and arrange bounded remeasurement under ordinary admission controls. Do not erase historical atoms solely because their current receipt says empty.

A negative-atom design can avoid a new row-key scheme, but still needs a trustworthy completeness boundary and a way to avoid letting old decisions replace newer generations. A typed result design needs additive or versioned ledger fields, source adapters, policy/schema updates, compatibility readers and a resumable migration. Keep authored knowledge and other sources outside its replacement scope.

Reuse scanner completion and generation metadata where possible; rescanning the repository or launching a full build on every knowledge refresh would be an unjustified default. Carrying an existing receipt can be cheap, but proving completeness where it is not currently represented is real engineering work. Atomic source replacement adds write coordination; snapshot revalidation can add retries. Exposing qualified retained rows may increase prompt bytes or reduce usable current coverage.

Negative atoms add rows, embeddings and retrieval candidates. Zero-row receipts save that materialization but require independent freshness accounting. Either repair may admit more necessary repository remeasurement than the current empty state. Because Refresh currently re-asks all sources, newly admitted work can also increase web/Context7 calls unless a separately justified source-selective design is adopted. Count that cost rather than calling the change a token optimization.

## Fair evaluation

All evaluation below is proposed and unrun. Compose the real source, acquisition persistence, measurement and Mangle refresh plan rather than asserting only that an importer callback returned an empty slice.

The decisive fixture should demonstrate positive acquisition, successful removal of the last world import, eight-day stale admission, source execution, retained baseline positive material, and the baseline empty receipt's subsequent lack of source-age refresh. For a repair, assert scoped retirement or a qualified negative replacement, continued age-based remeasurement, and correct positive replacement when an import is later added.

Controls should include:

- Two importers to one, one to zero, zero to zero and zero to one.
- Complete zero versus incomplete coverage, unsupported relevant files, partial scan, kernel missing, query error, cancellation and publication failure.
- Stable versus changed source generation between enumeration and commit; a newer positive list must survive an older zero decision.
- Another source acquired versus all sources empty, and stale, failed, unavailable and gap triggers independently.
- Seven/eight and 60/61 whole-day boundaries, prior concept retrieval, insufficient agent use, and a subject displaced by the four-subject budget.
- Short or empty web/Context7 responses: preserve the intentional non-destructive behavior unless those sources provide an equivalent explicit replacement contract.
- Legacy empty receipts, mixed ownership, migration restart, repeated startup and idempotent backfill.
- Real boot and explicit policy-refresh entry paths, with current world publication and a ready expert roster.

Current tests cover positive importer output and absent-kernel unavailability; refresh tests cover nonempty shrinking replacement and preservation after failure. They do not establish the complete-zero lifecycle. Preserve those assertions while adding the composition test. [Repository-source test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/knowledge_test.go#L182-L194), [replacement test](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/knowledge/refresh_test.go#L31-L67).

Measure source queries, completeness checks, SQL transactions, rows and vectors added/retired, kernel facts, queue delay, provider calls, prompt candidates and actual included claims. Hold model, task, corpus and budgets matched for any later answer-quality study. Separate removal of unsupported current claims from mere reduction in row count; a system that deletes all knowledge after a transient empty response is not a successful repair.

## Scope and branch review

This continuation was screened against main's knowledge/expert/JIT audits and research additions through 0919. [Scanner graph source retirement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/scanner-graph-source-retirement.md) concerns the separate SQLite graph projection; here the live world may already be correct. The 0850 major-version and 0919 observed-version receipt reports concern dependency identity, not an unchanged dependency whose repository-use set became empty.

At inspected QA head `767a3291d6287b238417f2466592a9002810d05a`, source, acquirer and refresh-policy contents matched main. The four older lane/QA heads inspected returned file-not-found for these knowledge files; that is not evidence of a competing repair. This is bounded source and inventory review, not an exhaustive novelty claim.

