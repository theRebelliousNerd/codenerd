# Working-context recall must preserve text after an embedded NUL

## Finding and decision

**A working observation can be stored intact but recalled as a successful prefix-only result.** At main [`388205138ba6d47c1cff0c5e97c896dfa642a391`](https://github.com/theRebelliousNerd/codenerd/commit/388205138ba6d47c1cff0c5e97c896dfa642a391), the working archive stores a Go string as SQLite TEXT, then implements character paging with `substr(body,...)` and reports its extent with `length(body)`. For a body containing an embedded NUL character, the pinned SQLite implementation stops both operations at that character. Recall returns no error, reports the prefix as the total, and cannot reach the suffix by increasing the offset.

The ordinary `read_file` producer can supply such an observation: a small admitted text file's contents are rendered into numbered lines without escaping NUL before the result is archived. This does not require a generated tool, free-form shell, failed process, oversized source, malformed UTF-8 or database corruption.

**Decision:** make archive paging and extent accounting preserve the whole admitted observation, including U+0000, while retaining the existing character-offset contract, scope ownership, revision disclosure and request-budget controls. Do not fix the symptom by stripping NULs or by silently changing character offsets to byte offsets.

This is a source-derived, conditional finding. Main, guidance, active branches and research additions through [`research-main-20261030-1346`](https://github.com/theRebelliousNerd/codenerd/commit/0b9eab6cc9f406a17bec847a84c3259e4e0cbf41) were checked on October 30, 2026. No application, tests, SQL experiment, model inference or repository changes were performed. The demonstrated boundary is **failed recovery through the working archive**, not erased database bytes, universal loss of the original observation, an observed bad edit, or a permission bypass.

## The production path admits and preserves the body before recall

### 1. The working loop installs a real scoped archive

The executor creates a task-private working set and installs `historyRecall` around it in the tool context. Numeric working handles route to that set; the separate `obs:hist:` family uses its own in-memory conversation paging. The working store records ownership before opening its database, and every body lookup includes the scope. [Working-loop installation](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L186-L215), [handle routing](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/history_recall.go#L41-L75), [store ownership and schema](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_store.go#L53-L99).

The model-facing `recall_context` tool explicitly describes character offsets and says an omitted limit returns the remaining body. It validates integral bounds and calls the installed recall interface. This is not an internal byte-oriented helper mistakenly used by a synthetic caller. [Tool contract and dispatch](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/core/context_recall.go#L26-L103).

### 2. An ordinary file read can contain a literal NUL

Choose a small workspace file such as `sample.txt`, with ordinary readable text before and after one actual U+0000 character. The notation `\x00` in this report denotes that character, **not** four printable backslash/x/zero/zero bytes. Keep it well below the line and region-byte limits.

After ordinary path resolution and admission, `executeReadFile` calls `ReadFileForTool`; that helper rejects directories but otherwise returns `os.ReadFile` bytes. The read codec carries the content into the selected region. Its text renderer appends numbered source lines with `%s`, not JSON string escaping. For the small selected region, the NUL and following text survive: the shared line splitter and region joiner operate only on newline delimiters. [Line/region handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/observation/precondition/precondition.go#L407-L424). [Tool read](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/core/file_ops.go#L207-L298), [raw file helper](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/projectdoc/tool_gate.go#L176-L187), [region projection](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/observation/fileread.go#L119-L171), [text and numbered-line rendering](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/observation/fileread.go#L275-L364).

The registry returns the handler's string, and the session returns that modular result without a content re-encoding step. Write-effect settlement returns a non-write read's output unchanged. The accounting path archives `out` before provider transport serialization. `recordWorkingResult` puts the body directly into `WorkingRecord`, with its source revision and digest identity. Later JSON escaping on a model request cannot retroactively change what was stored. [Registry result](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/tools/registry.go#L420-L463), [session result](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L2795-L2809), [archive call](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/executor_tools.go#L1102-L1113), [non-write settlement](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/tool_effects.go#L231-L233), [record publication](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L370-L403).

The witness requires the ordinary tool allowlist, restriction/path/safety checks and active working scope to admit the calls. It does not bypass them. A request refused before dispatch supplies no successful file-read witness.

### 3. The database writer is not the truncation point

The store computes the digest from the complete body and binds `r.Body` into the TEXT column inside its transaction. It preserves duplicate-ID idempotency and the short handle. [Digest](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_store.go#L22-L44), [bound body insertion](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_store.go#L279-L314).

The actual driver matters. The repository pins `github.com/mattn/go-sqlite3 v1.14.52`, and this store opens the `sqlite3` driver. At that tag's commit `b0be46fa28d17ee0b65c79774ac0dad84b6db068`, string binding supplies the explicit byte length; text extraction uses the reported column length with `GoStringN`. Neither is a NUL-terminated Go-string conversion. [Dependency](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/go.mod), [driver binding](https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L2234-L2246), [driver extraction](https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3.go#L2816-L2819).

## The precise SQL/character contract mismatch

`WorkingStore.Read` selects `substr(body,offset+1,limit)` and `length(body)`. An omitted/nonpositive limit becomes a large positive limit; that does not change SQLite's NUL semantics. `WorkingSet.Recall` then computes `next_offset` from the returned body's rune count, capped at the SQL total. The result therefore looks like a completed successful page, not an explicit unsupported observation. [Read query](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_store.go#L380-L403), [recall metadata](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_set.go#L104-L121).

SQLite documents that TEXT can contain NUL and that its `length` counts only the prefix before the first NUL. This is expected backend behavior, not a SQLite storage defect. [SQLite NUL documentation](https://www.sqlite.org/nulinstr.html), [length and substring units](https://www.sqlite.org/lang_corefunc.html#length).

The matching dependency source establishes the substring behavior, rather than assuming it from the length documentation. The pinned driver's header identifies SQLite **3.53.4** and source ID `bf7c7f30031888f4e796e429ab3978879485813aaca6f641c7b33e4e09459bcc`; the official SQLite 3.53.4 tag has that same manifest ID. Its TEXT substring loops stop at a zero byte both while advancing the offset and while collecting the requested characters. BLOB handling is separately length-bounded. [Driver's SQLite identity](https://github.com/mattn/go-sqlite3/blob/b0be46fa28d17ee0b65c79774ac0dad84b6db068/sqlite3-binding.h#L150-L152), [matching release identity](https://github.com/sqlite/sqlite/blob/b09c88c14082339b66c7b7158d609a771e64ca69/manifest.uuid), [matching substring implementation](https://github.com/sqlite/sqlite/blob/b09c88c14082339b66c7b7158d609a771e64ca69/src/func.c#L356-L444). A build deliberately linked to another system SQLite should be evaluated against that actual library too.

### Minimal observation and production consequence

For a unit-level stored body `alpha<NUL>omega`, default recall predicts:

- body `alpha`;
- `total_chars=5`;
- `next_offset=5`;
- no SQL/recall error.

A later offset beyond the NUL does not recover `omega`: the TEXT offset loop stops at the same NUL. For the ordinary file-read producer, the stored observation also includes its path header, line number and footer. Let P be its character count before the NUL; default recall returns that prefix with total/next equal to P, dropping the following source text and any later footer from this **recall projection**. Do not substitute five as the production observation's total.

The archive search is a useful countercheck. It uses `instr(body,query)`, which in the matching SQLite source searches using explicit input byte extents and can locate a term after the NUL. Yet its `body_chars` column still comes from `length(body)`. A suffix-only query can therefore identify the stored observation while its advertised extent and body recall remain shortened. This is not a claim that archive search is wholly unable to find the record. [Search query](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_store.go#L244-L276), [matching instr implementation](https://github.com/sqlite/sqlite/blob/b09c88c14082339b66c7b7158d609a771e64ca69/src/func.c#L243-L298).

For a model-facing consequence, keep the task scope alive and let ordinary policy compaction replace the earlier result with its recall handle. The handle promises recovery from offset zero or by pages. An admitted recall then cannot restore the suffix despite the record still existing. The original result may already have been seen; the source may still be reread; logs or direct database access may retain it. Those recovery routes limit impact but do not make the advertised archive replay faithful. [Compaction pointer](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context.go#L916-L923).

## Controls and nonduplication

Preserve the current scope-local handles, live-owner retention, transactional saves, revision/staleness metadata, whole-result recording and final request-size guard. Recall is historical evidence, never current verification or new execution authority. Ordinary Unicode without NUL and printable escaped sequences are negative controls. The separate `obs:hist:` implementation pages a Go rune slice, so this SQL finding must not be generalized to every retained-content family.

Existing tests cover whole observations, large-output handles, stale recall, live recalled-result retention and invalid handle rejection. They substantiate real recovery protections; a NUL-bearing admitted body exercises a different backend representation boundary. [Whole-result and size handling](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/session/working_context_test.go#L109-L182), [stale recall](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/internal/context/working_ledger_test.go#L135-L152).

The [context-ledger cost audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/context-ledger-prefix-costs.md) already owns scoped, paginated, recoverable evidence and warns against replacing metadata-only selection with a full-body cache. The [working-handle allocation audit](https://github.com/theRebelliousNerd/codenerd/blob/388205138ba6d47c1cff0c5e97c896dfa642a391/Docs/audits/working-handle-allocation.md) concerns ordinal allocation cost, not the body decoder. General evidence-conservation principles are not new here. Targeted prior-audit searches found no earlier report of this working TEXT/NUL page boundary; that is a bounded inventory claim, not exhaustive historical proof.

All five inspected active lane/QA heads retain the same `substr(body,...),length(body)` read and search-length expressions: `a53cfc30`, `80af1863`, `aa7988de`, `767a3291`, `51cfd426`. Other portions of those files differ.

## Repair choices and complete costs

Define the stored-body contract first: valid UTF-8 including U+0000 should round-trip as the same character sequence. Character counts, page boundaries, search extents and the whole-body path must agree. Invalid UTF-8 is a separate explicit policy, not required for this witness.

A minimal compatible repair can preserve the normal TEXT paging fast path and use a NUL-safe decoder for affected records, with truthful full character metadata. Loading a whole affected body into Go and paging a rune slice is simple, but makes a small requested page cost a whole-body read and conversion; benchmark that rather than describing it as bounded-page IO. A more scalable representation can store length-aware bytes with character indexes or chunks. Merely casting to BLOB makes offsets count bytes and would break the advertised Unicode character API.

Compute and bind any new count/index metadata to the exact persisted body and digest. Handle existing rows lazily or by a bounded migration without changing handles or deleting suffixes. Escaping NUL only at storage and returning the escape literally changes the original observation and its offsets unless a reversible codec is defined. Rejecting future unsupported bodies must be explicit and must not retroactively strand already accepted records.

Costs include read/decoding work, metadata storage, indexing or migration, backward compatibility, and request guard interactions. Correct recovery can produce a longer prompt or require additional pages; that is the cost of restoring evidence, not an automatic token saving. Avoiding repeated futile recall or unnecessary rereads is a possible downstream benefit, not a measured result.

## Unrun evaluation

1. On the actual pinned driver and normal store, save/reopen/read bodies with NUL at the beginning, middle and end; multiple NULs; multibyte characters around it; empty text; and literal backslash escapes. Verify exact reconstructed character sequence, digest, total, page progress and errors. Compare every paginated reconstruction with the original, not a second SQL `substr` oracle.
2. Exercise the normal permitted `read_file` → executor archive → compaction handle → `recall_context` path with a small `.txt` fixture. Keep normal workspace, JIT, safety, working-policy and request-budget wiring. Verify suffix evidence in the actual returned JSON body and rendered request, not only that Save succeeded.
3. Search for a unique suffix term and verify the handle and extent agree with recall. Preserve scope isolation, stale-revision disclosure, missing/retired handles, canceled reads and other handle families. Test legacy rows and the deployed SQLite build configuration.
4. Measure archive/body bytes read, decoding/allocation peak, SQL latency, metadata/migration work, page count, repeat recalls, rereads, and complete request tokens. Then compare paired tasks for recovered evidence, verified correctness, repair turns, provider/local cost and wall time at equal authority and context ceilings. Include failures and ordinary NUL-free workloads; no performance or task-quality gain is established here.

**Acceptance:** an admitted observation remains recoverable without silently treating its first NUL as end-of-observation, and the repair preserves character semantics and the existing executive/authority boundaries.

