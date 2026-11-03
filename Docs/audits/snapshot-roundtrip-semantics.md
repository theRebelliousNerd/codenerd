---
doc-class: deep-dive
subsystem: persist-snapshot-roundtrip
implementation-status: source-confirmed-gaps; remediation-proposed
last-verified: 2026-11-02
verified-against: 6cf8e45477e6699424b3d9912555c573f6cd777a
verification-method: static-source-inspection
---

# Snapshot round trips must preserve premises, typed values, and import outcomes

## Decision and evidence boundary

Keep the operator-driven snapshot workflow and its explicit, in-process assertion boundary. Repair the semantic contract around it: **a predicate filter must narrow the selected snapshot domain, not silently change EDB export into materialized export; values must retain their admitted Mangle types; and an import receipt must distinguish decoding, admission, and successful evaluation.** A matching checksum cannot establish any of those properties.

Four connected gaps are source-confirmed at the pinned revision:

1. `--predicate` is handled before `--derived` and calls `Query`, so it can select derived conclusions even when `--derived` is false. The help expressly describes ordinary export as EDB-only. [Selection][cli-selection], [help][cli-help]
2. Export can reconstruct a different type from the one admitted by the kernel. `Query` erases Name/String distinctions; default export instead reads raw host EDB values, which can predate Decl-directed adaptation. The snapshot writer re-runs Decl-blind `Fact.ToAtom`. [Kernel conversion][query-conversion], [raw EDB][raw-edb], [admission and storage][admit-store], [writer][writer]
3. Snapshot decoding preserves Names as `MangleAtom` but returns Strings as ordinary Go strings. A name-shaped String therefore becomes ambiguous again on a subsequent serialization or `--to-mangle` rendering. This is the **opposite-direction String→Name risk**, not a restatement of persist Q5's known Name→String case. [Decoder][decoder], [explicit types and heuristic][type-defs], [rendering][render-types]
4. `--assert` calls `LoadFacts`, whose bool-only insertion wrapper discards per-fact admission errors. An initialized kernel then defers evaluation; the CLI reports only before/after raw EDB counts and does not force a read. A nil return can therefore accompany rejected rows, and the receipt is not a successful-fixpoint receipt. [Import][cli-import], [bulk load][bulk-load], [discarded rejection][admit-store]

These are static findings and source-derived consequences, **not executed reproductions, observed user incidents, demonstrated authorization bypasses, or measured performance regressions**. No repository code, tests, runtime, inference, benchmarks, or CI were run or modified for this review. The proposed witnesses and acceptance suite below remain unrun. The inspected production paths establish reachability, not the frequency of affected data in live workspaces.

## 1. Existing ownership and the non-duplication boundary

Root `agents.md` requires source-grounded wiring, typed boundaries, and preservation of Mangle's executive role. Scoped core guidance specifically requires Decl-compatible facts and direct AST-type checks where conversion matters. The architecture guidance separates target guarantees from implementation evidence and warns against using old corpus claims as proof. This audit follows those distinctions; it does not rewrite the target to match the deficiency. [Root guidance][root-guide], [core guidance][core-guide], [architecture guidance][arch-guide]

The pinned Docs inventory already contains the full persist corpus, the typed-content-fingerprint audit, the EDB-alias-support audit, and a filesystem/name-oriented snapshot boundary analysis. Relevant ownership is:

| Existing document | Existing responsibility | Increment added here |
|---|---|---|
| Persist `OPEN-QUESTIONS`, Q5 | Core Name→plain-string conversion and deep/path-shaped Name degradation; general consumer migration | Opposite String→Name drift, admission-to-export mismatch, and a snapshot-specific typed acquisition boundary |
| Persist Q8 | Whole Float64 values decoded as integers by the current SimpleColumn behavior | Separate text-export precision loss for nonintegral values; no claim to fix Q8 |
| Persist safety/testing/gap documents | Codec parity, per-file publication, sidecars, explicit import, existing tests | End-to-end premises/type/receipt obligations that equalish codec tests do not establish |
| `mangle-edb-alias-support.md` | Multiple host representations supporting one admitted atom during delta removal | Snapshot set identity and export counts, not a new delta-removal audit |
| `mangle-content-fingerprint-typed-identity.md` | Typed identity in cache/fingerprint reuse | Serialized transfer identity, not a new fingerprint algorithm |
| Existing snapshot boundary analysis | Name resolution, directories, filesystem edge cases | Kernel acquisition, typed codec hops, import admission/evaluation |

Sources: [persist questions][questions], [persist invariants][persist-safety], [persist test inventory][persist-tests], [persist gaps][persist-gaps], [alias audit][alias-audit], [fingerprint audit][fingerprint-audit], [snapshot boundary analysis][old-boundary]. Their historical findings are not assumed still open unless re-established by current source. In particular, current EDB indexing already contains admitted-atom memoization and alias bookkeeping; this proposal must reuse or respect those mechanisms rather than recreate the older alias bug. [Current index][edb-index]

The persist corpus owns `internal/persist`; the repair also touches CLI selection/reporting, core acquisition/admission, and potentially `types` text rendering. Its eventual implementation needs coordinated specification updates in those existing owners, not a second competing persistence subsystem. [Persist ownership][persist-owner]

## 2. What actually runs

### 2.1 Export is two different production entry paths

`snapshotCmd` is registered on the root command. `runSnapshotExport` constructs a new workspace-rooted `RealKernel`, collects facts, and calls either `snapshot.Export` or `factsnap.WritePath` for explicit `--out`. The workspace constructor loads Mangle files, injects boot facts, and performs initial evaluation. This is a freshly booted kernel view, **not an attachment to the memory of a separate running agent**. [Registration][registration], [export][cli-export], [constructor][constructor]

The collector has this exact precedence:

| Inputs | Current acquisition | Domain/representation |
|---|---|---|
| No predicates, `--derived=false` | `GetBaseFacts()` | Raw host EDB rows |
| No predicates, `--derived=true` | `QueryAll()` | Declared predicates from the evaluated materialized store, flattened to `Fact` |
| Any predicate entries, either derived setting | One `Query(pred)` per nonblank entry | Evaluated query results, flattened to `Fact`; not an EDB-only selection |

`Query` ensures evaluation and reads matching store atoms. `QueryAll` also ensures evaluation before enumerating declared predicates. Consequently `--predicate` is not merely a post-filter over the default EDB acquisition. Repeated selectors are appended without collector deduplication; whitespace-only entries suppress the default branch and can produce an empty selection. `Query` also accepts patterns, although the flag advertises predicates; the revised contract must decide whether this is supported syntax or reject it explicitly. [Collector][cli-selection], [query read][query-read], [materialized read][query-all]

There is also a genuinely live post-execution caller: direct actions call `DumpKernelSnapshot(cortex.Kernel, workspace)` after the observed spawn, on success or failure. That helper accepts only a concrete `*RealKernel`, takes `GetBaseFacts`, and uses gzip `snapshot.Export`. This substantiates another production route affected by raw-EDB type re-encoding. It does **not** establish that every command or every Kernel wrapper can dump; unsupported wrappers return an empty path. [Direct action call][debug-call], [debug helper][debug-helper]

### 2.2 The libraries encode data, not an origin certificate

`snapshot.Export` sanitizes a workspace name and delegates encoding. `factsnap` converts every `Fact` with `ToAtom`, inserts atoms into an in-memory store, writes deterministic SimpleColumn, compresses, and computes the digest of the compressed bytes. No EDB/derived flag, rule identity, schema identity, origin classification, or selection manifest accompanies the `[]Fact` payload in this path. [Workspace export/import][workspace-store], [writer][writer]

This is a valid thin codec architecture, but not enough metadata for an importer to recover which rows were conclusions. A file containing facts is not self-describing proof that those facts were premises. Likewise, typed set identity is not the number of host representations fed into a set encoder: repeated selectors or two host values that encode to the same atom can make the input/report count differ from distinct encoded membership. The CLI currently prints `len(facts)` before any decoded-cardinality check. Exact duplicate-count outcomes should be pinned against the deployed fork in the proposed tests, not inferred from a pretty summary. [Export receipt][cli-export], [writer][writer], [current admitted-atom bookkeeping][edb-index]

### 2.3 Import is deliberately not durable restore

`snapshot.Import` resolves a file and calls `factsnap.Read`; read verifies an available sidecar before decoding. By default the CLI summarizes the decoded facts. `--to-mangle` writes a sorted textual representation. `--assert` boots a new local kernel, adds decoded facts, reports the EDB count delta, and returns. The command does not invoke a snapshot-to-workspace persistence API or a boot-time snapshot loader. [Read and verify][reader], [import command][cli-import], [text writer][text-writer]

Preserve that boundary. The finding is not that an ordinary import silently installs permanent policy. Existing explicit-import design is valuable, and the source supports its limited scope. `--assert` is additive into the target's boot state, not exact replacement of a complete historical workspace; matching snapshot facts cannot by itself make the target's unrelated facts, policy, external state, or derivation history equal to the source.

## 3. Predicate selection must not promote conclusions to premises

The CLI itself explains the intended distinction: derived facts are recomputable conclusions, and importing them as EDB changes their role. Yet a nonempty predicate list enters the `Query` branch before `snapshotDerived` is inspected. The counterexample needs only a selected predicate with a derived answer. No codec failure, malformed file, checksum mismatch, or concurrency is required. [Help][cli-help], [branch][cli-selection]

**Proposed witness, unrun:** use a trusted fixture with `snap_seed(X)` and `snap_conclusion(X) :- snap_seed(X)`. Assert only a seed. Export `-p snap_conclusion` without `--derived`, decode, and inspect membership. Current source predicts the conclusion is present. Import those facts into a separate kernel with compatible declarations, then remove the seed support if any. The imported conclusion can remain as EDB because it was asserted as a premise; recomputation cannot retract an independently asserted fact merely because its original proof is absent. This is a semantic diagnostic witness, not a claim about a particular production safety predicate.

**Required contract:** selection order is `domain → predicates/patterns → typed serialization`, not `predicates → an unrelated acquisition API`. With default EDB mode, a derived-only predicate yields no rows (or an explicit empty-selection outcome); `--derived` opts into materialized-debug data. Classify at row/origin level where a predicate has both base and derived membership. A blanket ban on predicates that appear in rule heads would incorrectly remove legitimate EDB facts of mixed predicates.

For materialized snapshots, record `kind=materialized-debug` and the source program/schema identity, and refuse to describe them as restorable EDB. Whether an operator may explicitly promote those rows is a separate policy decision requiring unmistakable consent and disclosure. A source-side `--derived` choice is not durable destination-side evidence once the file is moved. Legacy files with no origin record need an explicit `origin=unknown` classification, not an invented EDB provenance.

Soufflé's provenance documentation provides a useful distinction: explanations depend on proof-related evaluation annotations and show inputs/intermediate tuples, rather than treating a result tuple as its own explanation. That supports keeping conclusion data separate from provenance. This is a design analogy, **not** a claim that CodeNERD uses Soufflé or should import its proof-tree implementation. [Soufflé provenance](https://souffle-lang.github.io/provenance)

## 4. Type identity is lost at more than one boundary

### 4.1 Raw EDB is not necessarily the admitted atom

`MangleString` explicitly means String and `MangleAtom` explicitly means Name. Ordinary strings retain a shape heuristic: a short slash-prefixed value such as `/tmp` can become a Name in `ToAtom`. Core admission can correct that initial choice when a narrow `/string` slot accepts the String but rejects the Name. It only performs this Name→String adaptation for an ordinary Go `string`; an explicit `MangleAtom` is checked as a Name. [Types][type-defs], [ToAtom][to-atom], [adapter][decl-adapter]

Insertion computes the admitted atom but appends the original host `Fact` to `k.facts`. `GetBaseFacts` copies those raw rows. Therefore using it avoids query flattening but does **not** preserve the result of admission. [Insertion][admit-store], [accessor][raw-edb]

**Minimal source-derived chain, unrun:**

```text
trusted declaration: Decl snap_path(Path) bound [/string].
host input:         Fact{Predicate: "snap_path", Args: []any{string("/tmp")}}
kernel admission:   Name("/tmp") is adapted to String("/tmp")
GetBaseFacts:       retains the original ordinary Go string("/tmp")
snapshot ToAtom:    no Decl is available; shape heuristic produces Name("/tmp")
snapshot Read:      Name becomes explicit MangleAtom("/tmp")
target admission:   explicit Name is not adapted to the /string slot; rejected
```

The input uses supported host APIs and a narrow trusted declaration. `/any` is **not** an equivalent fixture: it accepts the initial Name and does not establish the adaptation in the source. No claim is made that a stock producer has been observed passing this exact value through an exported workspace. The source nonetheless establishes that the default snapshot API cannot promise admitted-type fidelity for all admitted host facts.

### 4.2 Query exports lose an explicit String before the codec sees it

For predicate-filtered or `--derived` exports, `core.baseTermToValue` returns an ordinary string for both `ast.NameType` and `ast.StringType`. An admitted `String("/tmp")`, including one originally created from explicit `MangleString`, therefore arrives at the snapshot writer as ambiguous `string("/tmp")`. `ToAtom` selects Name. The loss occurs before compression; changing gzip to zstd cannot repair it. [Query conversion][query-conversion], [writer][writer], [ToAtom][to-atom]

This complements, rather than duplicates, Q5. Q5 already identifies Names with path-like spelling that fail the heuristic on re-encoding. Here an actual String with name-like spelling passes it. Both demonstrate that display-oriented query values cannot be a lossless serialization interface, but they have different affected inputs and acceptance cases.

### 4.3 Even a successful first codec hop is not a typed fixed point

Supplying `MangleString("/tmp")` directly to `factsnap.WritePath` does produce a String atom. But decoder `baseTermToValue` returns `c.Symbol` for String, not `MangleString`. The next `WritePath` or `Fact.String()` can therefore promote it to Name. A first read can appear textually identical while already losing the tag needed for the next hop. [Decoder][decoder], [ToAtom][to-atom], [rendering][render-types]

A narrow `/string` destination can repair the *first decoded ordinary-string* input during admission, so it would be wrong to claim every first-hop `MangleString` snapshot import fails. The definite problem is loss of type identity in the decoded representation and its downstream re-encoding/rendering consequences. Under a domain accepting both Name and String, admission need not recover the intended tag at all. Check the source AST, every decoded value, second-hop AST, and destination answers independently.

### 4.4 Reviewable text has another, separate numeric obligation

`writeFactsAsMangle` uses `Fact.String`, sorts its strings, and writes them without a parse-back equivalence check. `Fact.String` uses `%f` for floats to retain a decimal point for whole values. Go's default `%f` precision is six decimal places, so sufficiently small or high-precision **nonintegral** values are rounded in the review text. For example, the proposed unrun case `0.0000001` renders as `0.000000`; preserving a float-looking token is not preserving its value. [Text writer][text-writer], [float rendering][render-types], [Go formatting documentation](https://pkg.go.dev/fmt#hdr-Printing)

This is not Q8's whole-float SimpleColumn issue. It belongs to the text serialization contract, which needs a round-trip-safe floating representation supported by the actual parser, including signed zero, exponent handling, extremes, and rejection policy for non-finite values. Merely replacing `%f` with a generic renderer without parser tests could reintroduce the known whole-float ambiguity.

### 4.5 Scope the supported type domain explicitly

Do not label a fix lossless for all Mangle terms after repairing only Name/String. The snapshot decoder combines String and Bytes and has a fallback for other constants; the core query converter renders composite/time/duration constants to text. Existing persist docs already leave time/duration coverage open. Native structured terms, bytes, numeric identity, bool conventions, JSON legacy inputs, and upstream format behavior need either typed support or explicit rejection/compatibility classification. This audit establishes the concrete scalar paths above, not a complete reproduction catalogue for every constant kind. [Decoder][decoder], [query conversion][query-conversion], [persist test gaps][persist-tests]

## 5. Import accuracy: decoded, admitted, evaluated are different outcomes

`LoadFacts` sanitizes inputs, calls `addFactIfNewLocked` for each, and increments an added count only on true. That wrapper calls `addFactIfNewLockedErr` and discards its error. The underlying function can reject capacity and conversion/admission failures; an already-present fact is a different `(false, nil)` outcome. These are collapsed at the caller. If no facts were added, `LoadFacts` returns nil. If the kernel is initialized and some were added, it marks dirty and returns nil without evaluation. [Bulk load][bulk-load], [underlying outcomes][admit-store]

The CLI takes `before` and `after` from `GetBaseFacts` and prints the difference after a nil `LoadFacts`. This cannot distinguish duplicates from rejected rows, does not reconcile decoded rows with admitted typed membership, and does not establish the new facts successfully participate in a fixpoint. A zero-new receipt could be a harmless duplicate import, all rejected input, or a mixture. A nonzero receipt can be partial. The earlier `/tmp` chain supplies a concrete source-derived rejection mechanism; malformed names, incompatible declarations, and capacity boundaries are additional proposed cases. [Receipt][cli-import]

The correction must not silently weaken admission to make round trips pass. A source schema can differ from the target schema; rejection can be the right semantic result. The defect is reporting indistinguishable success and/or silently changing the value, not the existence of strict Decl checking.

**Proposed receipt contract:** `decoded`, `duplicate-input`, `already-present`, `admitted-new`, `rejected`, `evaluation-status`, `origin-kind`, and `durability=in-process-only`, with bounded indexed rejection diagnostics and a clear terminal outcome. Counts must state whether they describe payload rows, host representations, or distinct typed atoms. Acceptance requires a declared conservation equation over one chosen input identity, rather than comparing only `after-before`.

For strict imports, use a disposable candidate and publish a success receipt only after all required rows pass admission and evaluation. Because today's CLI already owns a throwaway kernel, failed candidates can be discarded without promising rollback on a shared runtime. If a future importer targets a shared kernel, it needs a genuine staging/commit protocol; that is a larger authority and concurrency change.

Do **not** assume the existing `KernelTransaction` provides all-or-nothing admission. Its current implementation intentionally lets good assertions land while collecting rejected assertions, then returns an error. Its atomic lock boundary is not rollback semantics. [Transaction implementation][transaction]

PostgreSQL makes the corresponding distinction explicit: ordinary `pg_restore` can continue and report errors, `--exit-on-error` stops, and `--single-transaction` promises all-or-none changes while implying exit-on-error. That is a useful model for naming import guarantees, not a request to add PostgreSQL or to equate its transaction mechanics with this kernel. [PostgreSQL `pg_restore`](https://www.postgresql.org/docs/18/app-pgrestore.html)

## 6. Integrity and deterministic bytes are necessary but separate

Retain the existing good behavior: canonical extensions, content sniffing, checksum verification when present, explicit no-sidecar reporting, deterministic SimpleColumn configuration, and per-file temp/sync/replace publication. None repairs a type already changed before serialization. A perfectly matching SHA-256 may certify exactly the wrongly typed bytes the exporter wrote. A sidecar is not a trusted signature or proof of EDB origin. [Writer][writer], [read/verify][reader], [persist safety][persist-safety]

RFC 8785 explains canonicalization as a way to obtain invariant representations for repeatable hashing/signing, with an explicitly constrained data model. The lesson here is **define typed semantic identity before choosing its canonical bytes**. CodeNERD's SimpleColumn path is not JCS, and adopting JCS for metadata would not make `[]any` Name/String collapse safe or settle Mangle float identity. [RFC 8785](https://www.rfc-editor.org/rfc/rfc8785)

Keep distinct checks for: same-codec repeatability under a pinned encoder; cross-codec decoded semantic equality; transport digest agreement; typed source/destination equality; origin preservation; and import admission/evaluation. Do not require gzip and zstd bytes to be identical. Do not normalize Name and String together just to make a cross-codec test pass.

Publication also retains independent failure costs: data and sidecar are separate replacements, path locks are in-process, and a sidecar write failure is logged rather than failing the already-written data export. A stale prior sidecar can differ from a missing one. These are existing publication concerns, not the central new finding; a metadata extension must not worsen them by adding yet another uncoordinated success marker. [Sidecar publication][writer], [published concurrency scope][persist-safety]

## 7. Scoped remediation, not a global query migration by accident

### A. Specify snapshot intent before serialization

Define an explicit EDB-transfer kind and materialized-debug kind. Apply predicate selection within that kind, deduplicate selectors, and specify whether patterns are admitted. Give empty selection an honest outcome. Decide whether a materialized file can be asserted at all, and how old origin-unknown files are handled. Preserve the operator-only import boundary and existing ability to inspect untrusted files.

### B. Acquire typed, admitted EDB without changing every query consumer

Add a narrow core export interface that obtains an immutable typed representation of admitted EDB under one program/generation boundary. EDB extraction must use the current declaration semantics, not raw `Fact.ToAtom` and not `QueryAll` followed by a predicate-name guess. Current index memoization associates admitted atoms with the `ProgramInfo` that admitted them; it is a plausible reuse seam, not an already-shipped snapshot API. Recompute stale/missing entries through the actual admission path, propagate failures, and retain exact predicate arity. [Current index][edb-index], [admission][decl-adapter]

For materialized-debug extraction, obtain actual typed store atoms after successful evaluation and classify the result accordingly. Freeze the exported view so concurrent mutations cannot create a cross-generation mixture; respect volatile/external state instead of claiming a generation token captures it. Existing `SnapshotIdentity` expressly marks external-dependent states nonreusable. [Generation contract][generation]

A snapshot-specific API avoids forcing all display, articulation, and pattern-matching consumers to change at once, as Q5 warns. It still incurs ownership, cloning, compatibility, and typed-codec work. A global conversion migration remains an alternative, but requires a separate caller inventory and is not the smallest scoped repair.

### C. Preserve tagged values through decode and text export

For the supported scalar subset, decode Name→`MangleAtom` and String→`MangleString`, and keep those tags through every subsequent conversion. Alternatively expose a typed atom transport API so no host heuristic participates. Enumerate unsupported types and fail clearly rather than silently flattening them. Preserve legacy read support as a labeled compatibility path; old JSON strings cannot reveal type information never stored.

Text rendering should be generated from the same typed representation, with a parser-backed equivalence check in tests and lossless numeric formatting. Sorting remains useful but cannot be the fidelity oracle. The public text must disclose materialized/unknown origin and target compatibility limits.

### D. Make assertion outcome an exact, evaluated receipt

Use the error-preserving admission primitive or a new reporting API. For the CLI, stage in its fresh kernel, classify duplicates/rejections, force a successful evaluation before claiming a usable restore, and never print an unqualified success for partial admission. If best-effort mode is retained, label it, expose rejections, and choose an explicit non-success exit policy for incomplete admission. Keep strict and best-effort guarantees distinct.

### E. Evolve metadata only as much as the contract requires

A versioned envelope/manifest can carry format version, kind, source program/schema identifier, selector, typed counts, and content digest. It must be bound to the data being verified; a detached unbound JSON description is not proof. Versioning does not justify automatic source-policy installation or trusting untrusted declarations. Prefer one coordinated publication unit or a documented fail-closed manifest/data protocol. Q6 already owns versioning; this audit supplies concrete semantic requirements for that decision rather than inventing a separate version scheme. [Open questions][questions]

## 8. Proposed verification matrix — none executed here

The oracle is the **admitted source AST and origin**, captured before any flattening, plus the destination's typed facts and relevant derived answers. Re-running `ToAtom` on an already-lossy `Query` result cannot reconstruct a trustworthy expected value. Compare predicate symbol, arity, constant kind, exact payload, and normalized set membership; only explicitly documented representation equivalences may be collapsed.

| Case | Required observation |
|---|---|
| EDB-only, derived-only, and mixed-origin predicates; with/without `-p`, with/without `--derived` | Filtering never enlarges the allowed origin domain; materialized kind disclosed |
| Repeated, blank, unknown, and pattern selectors; multiple arities | Explicit selector contract, deterministic dedup/counts, no accidental fallback |
| Ordinary `/tmp` admitted under `/string`, explicit `MangleString("/tmp")`, explicit Name, ordinary nonslash String | Source admission, first payload, decoded tag, destination admission, and second hop all checked separately |
| `/string`, `/name`, `/any`, and Name-or-String alternatives | Repair is not credited when a broad declaration merely accepts the wrong type |
| Two host aliases of one admitted atom; duplicate payload rows | Host-row counts distinguished from typed set membership; no false newly-restored count |
| Existing Q5 deep Name and Q8 whole-float cases | Retain named compatibility expectations; do not present them as newly repaired |
| Fractional values with more than six places, tiny magnitude, signed zero, extremes | Text round trip preserves supported value/type or rejects explicitly; no silent `%f` rounding |
| Bytes, time/duration, structured constants, null/unsupported host arguments, legacy JSON | Supported domain and compatibility/rejection behavior explicit |
| All duplicates; all rejected; valid-invalid-valid mixture; capacity boundary | Receipt conservation, accurate error/exit, no rejection hidden as duplicate |
| Admission succeeds but derivation fails | No evaluated-success receipt; failed candidate discarded in strict CLI mode |
| Source/target schema mismatch and target boot facts absent from source | Additive import and compatibility limits disclosed; no claim of whole-workspace equivalence |
| Remove original support after filtered/materialized import | EDB mode does not fossilize a conclusion; intentional promotion, if allowed, unmistakable |
| gzip/zstd, repeated writes, read→write→read, text parse-back | Typed parity independent of compressed bytes and rendering order |
| Corrupted, absent, stale sidecar; truncated codec; write/rename/sync failures | Existing transport/publication protections preserved; incomplete metadata never certified |
| Mutation or schema change during acquisition; clean clone; external predicates | One coherent snapshot identity or explicit failure/limited guarantee, not mixed state |
| Real CLI and direct-action `--dump-kernel` call paths | Same typed boundary actually used; wrapper refusal and in-process-only behavior preserved |

Existing tests deserve credit, but only for their scope. The kernel round-trip test uses one `code_defines` domain with relative `.go` paths and `/func`, then builds `%v` argument keys. That comparator erases Name/String wrapper differences and does not witness all admitted types. The codec tests normalize any slash-prefixed ordinary string to `MangleAtom`, broader than production's own name-shape heuristic. A shared mistake can therefore pass both codecs and the equalish comparator. [Kernel round-trip test][roundtrip-test], [codec comparator][codec-comparator], [codec parity][codec-parity]

The CLI tests establish readable default export, sorted text, local-only assertion behavior, and missing/unknown-reference handling. They do not by those assertions establish EDB-only predicate filtering, typed second-hop fidelity, per-row rejection accounting, or successful post-import fixpoint evaluation. Extend those entry-path tests rather than replacing them with codec-only tests. A kernel-boot skip must remain a skip/incomplete result, never count as a fidelity pass. [CLI tests][cli-tests], [kernel round-trip test][roundtrip-test]

## 9. Full cost accounting and fair evaluation

No speed, memory, file-size, token-saving, or end-user-quality gain has been measured. The repair can add work; correctness is not a cost-free encoder flag.

| Stage | Existing cost to charge | Additional proposal cost / failure exposure |
|---|---|---|
| Kernel boot | Mangle load, compile/initial evaluation, boot facts | Separate from serialization so cold-start cost is not hidden |
| Acquisition | Raw EDB slice copy or Query/QueryAll evaluation and materialization; repeated selector work | Typed admission/memo validation, snapshot locking/cloning, arity/origin classification |
| Encoding | Fact→atom conversion, in-memory fact store, complete uncompressed `bytes.Buffer`, compression | Tagged representation or envelope, validation, possible larger payload; unsupported-type errors |
| Publication | Compressed bytes, checksum, temporary data, fsync/replace, sidecar write, path lock | Manifest coordination, additional disk/inode work, compatibility/recovery handling |
| Read | Whole compressed-file read, digest, decompression/store reconstruction, decoded fact allocation | Bounded decode/metadata validation, tag preservation, decoded-size limits |
| Admission | Sanitization, dedup/indexing, declaration checks, retained EDB growth | Per-row outcome accounting, candidate state, bounded diagnostics |
| Validation | Today initialized `LoadFacts` returns before lazy evaluation | Required fixpoint work, failure cleanup, optional typed comparison; include this in end-to-end import latency |
| Text export | Render all strings, sort, builder, write | Exact formatting and typed rendering, parse-back testing; output expansion for extreme numbers |
| Operations | Schema/format compatibility and manual review | Legacy ambiguity, migration tooling/docs, user-visible strict/best-effort semantics |

Report peak RSS, allocations, compressed/uncompressed bytes, temporary disk footprint, lock hold/wait time, boot/acquisition/admission/evaluation/compression/I/O spans, p50/p95 end-to-end latency, and decoded/admitted/rejected/distinct-atom counts. `factsnap.Read` and `Verify` read the full compressed file; “no decode” verification is not constant-memory verification. Metadata and untrusted-input limits should be evaluated against both compressed and expanded size. [Read implementation][reader], [writer buffering][writer], [text writer][text-writer]

Compare at least: current path; selector-only repair; scalar-tag repair; typed admitted-EDB extraction; and exact admission/evaluation receipt. Use identical frozen source data, schemas, destination boot state, codec settings, hardware, and build tags. Separate cold and warm boot, sparse and broad filters, ordinary and adversarial typed inputs, alias-heavy and alias-free corpora, small files and realistic large worlds. Include gzip and zstd without crediting one for silently changing answers. The initial correctness oracle must be external to the lossy conversion being tested.

Measure both successful and failing imports. An all-rejected import that returns quickly is not an efficient restore; a baseline that omits the evaluation the target contract requires is not a fair latency comparator. Charge strict staging memory and wasted work on late failure, and bounded best-effort diagnostics. If a typed interface reuses admitted-atom memos, test stale policy and clone invalidation, and report the cost of cache misses as well as hits. If batching/streaming is introduced later, account for lost all-or-none semantics or the staging needed to retain them.

No extra model calls, embeddings, training, or live inference are necessary for this deterministic boundary repair. Any downstream claim about fewer failed debugging sessions, more reliable agent recovery, or saved tokens needs a separate end-to-end workload with unchanged task quality and full retry/recovery cost; it does not follow from a smaller snapshot or faster codec benchmark.

## 10. Checkable exit and non-goals

The scoped work is complete only when:

1. Predicate selection preserves the declared EDB/materialized domain through the real CLI.
2. Every supported exported value retains its admitted type and payload through binary decode, second hop, and reviewable text; unsupported/lossy compatibility cases are explicit.
3. Imports account for every input identity as duplicate, admitted, or rejected, and separately report evaluation status and in-process durability.
4. A strict successful receipt requires the intended candidate to evaluate successfully; partial or failed import cannot masquerade as full restoration.
5. Existing checksum, explicit-import, workspace-name, and publication protections remain intact, and complete costs are reported on semantically equivalent outcomes.

Non-goals are automatic boot restore, permanent policy adoption, universal provenance reconstruction, a wholesale rewrite of query/display consumers, an upstream float-format fix disguised as local work, or relaxing default-deny authorization to accept imported facts. Those require separate decisions and evidence. The immediate value is narrower and testable: **the debug artifact says what it contains, transports what the kernel actually admitted, and tells the operator what the destination actually accepted and evaluated.**

## Source index

All repository links below are pinned to `6cf8e45477e6699424b3d9912555c573f6cd777a`. External references were inspected November 2, 2026; they supply design distinctions, not runtime evidence for CodeNERD.

[root-guide]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/agents.md
[core-guide]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/agents.md
[arch-guide]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/architecture/AGENTS.md
[questions]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/architecture/persist/OPEN-QUESTIONS.md
[persist-safety]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/architecture/persist/09-SAFETY-AND-INVARIANTS.md
[persist-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/architecture/persist/10-TESTING-ALIGNMENT.md
[persist-gaps]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/architecture/persist/03-GAP-ANALYSIS.md
[persist-owner]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/architecture/persist/corpus.toml
[alias-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/audits/mangle-edb-alias-support.md
[fingerprint-audit]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/Docs/audits/mangle-content-fingerprint-typed-identity.md
[old-boundary]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/.quality_assurance/2026-09-29_11-32-11-PM_EST_snapshot_boundary_analysis.md
[registration]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/main.go#L321-L350
[cli-export]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_snapshot.go#L93-L142
[cli-selection]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_snapshot.go#L144-L174
[cli-help]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_snapshot.go#L42-L53
[cli-import]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_snapshot.go#L176-L225
[text-writer]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_snapshot.go#L250-L276
[constructor]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_init.go#L108-L143
[debug-call]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_direct_actions.go#L399-L409
[debug-helper]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_debug.go#L123-L153
[workspace-store]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/snapshot/snapshot.go#L98-L152
[writer]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/factsnap/factsnap.go#L108-L263
[reader]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/factsnap/factsnap.go#L278-L376
[decoder]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/factsnap/factsnap.go#L424-L501
[raw-edb]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_accessors.go#L5-L17
[query-read]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_query.go#L31-L214
[query-all]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_query.go#L369-L430
[query-conversion]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_query.go#L457-L500
[admit-store]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_facts.go#L464-L537
[bulk-load]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_facts.go#L64-L152
[decl-adapter]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_fact_decl.go#L158-L239
[type-defs]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/types/types.go#L44-L114
[to-atom]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/types/types.go#L218-L268
[render-types]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/types/types.go#L134-L175
[transaction]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_transactions.go#L76-L192
[edb-index]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_edb_index.go#L18-L145
[generation]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/core/kernel_generation.go#L18-L66
[roundtrip-test]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/snapshot/kernel_roundtrip_test.go#L23-L136
[codec-comparator]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/factsnap/factsnap_test.go#L15-L121
[codec-parity]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/internal/persist/factsnap/codec_parity_test.go#L11-L70
[cli-tests]: https://github.com/theRebelliousNerd/codenerd/blob/6cf8e45477e6699424b3d9912555c573f6cd777a/cmd/nerd/cmd_snapshot_test.go#L34-L181
