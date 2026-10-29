# Branch proposal: admit world-cache formats, not just the column declaration

## Status and decision

**Branch-only finding**, source-inspected on 2026-10-29. Main is [`9dcb01b9736bf4763aa3ac391d29f133d73c6611`](https://github.com/theRebelliousNerd/codenerd/tree/9dcb01b9736bf4763aa3ac391d29f133d73c6611); the active Mangle/store lane is [`a53cfc308e8836c72370adaa46deba3f7b00180d`](https://github.com/theRebelliousNerd/codenerd/tree/a53cfc308e8836c72370adaa46deba3f7b00180d). Main has not adopted this binary world-fact codec. No application, test, benchmark, model inference or training was run.

The lane replaces tagged-JSON world-cache arguments with binary arguments. Its ordinary forward migration is transactional and has useful tests. The proposed additional acceptance contract is:

> A persisted world cache may be reused only when its actual row encoding and coverage are compatible with the running reader. An incompatible or mixed cache must become a visible cache miss/degraded state that triggers controlled reconstruction, not a successful partial world snapshot.

The concrete concern is **sequential version alternation over one workspace cache**: new lane → current-main writer → new lane. An old writer can put valid tagged JSON into the converted table; the new reader then silently drops those rows, while a current filesystem manifest prevents their automatic regeneration. No promise that old-version rollback is supported was found. This is a release/migration acceptance decision, not an allegation that the repository violated a published downgrade guarantee.

## What the lane changes

During `NewLocalStore` initialization, the lane calls `migrateWorldFactsToBinary`. The migration inspects the declared type of `world_facts.args`. If it is exactly `TEXT`, it reads the legacy rows, decodes them, writes their binary replacements, then drops/renames the table and commits. An already non-TEXT declaration returns without inspecting row encodings. The replacement schema declares `args BLOB NOT NULL`, but is not a STRICT table and has no row-encoding CHECK. [Initialization](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/local_core.go#L469-L475), [migration](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/world_fact_codec.go#L149-L223), [schema](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/world_fact_codec.go#L226-L243)

The binary decoder requires version byte 1. Both all-world and per-file loaders skip a row on a decoding error and still return nil error if SQL iteration succeeds. Per-file loading records the fingerprint before decoding the arguments. Thus the table declaration and successful SQL read are not completeness certificates. [Decoder](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/world_fact_codec.go#L98-L115), [readers](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/local_world.go#L122-L200)

Row-skipping on decode failure predates this branch. The new deterministic trigger is a valid older writer using a newly shared format, not a newly invented generic corrupt-row problem.

## Conditional rolling-format witness

This witness is a **proposed isolated compatibility evaluation**, not instructions to alternate live binaries or mutate a user's database.

Let A be the lane implementation and B the pinned current-main implementation. Use at least two ordinary source files with populated fast-cache facts and a valid, nonempty file manifest.

1. **A opens the existing cache.** It successfully converts legacy JSON rows to binary and retains the canonical world-file metadata and filesystem manifest. This is the intended forward migration.
2. **B later opens the same database, sequentially.** Main's `CREATE TABLE IF NOT EXISTS` does not replace the existing BLOB-declared table. Its readers expect tagged JSON; binary rows do not satisfy that decoder and are skipped. [Main schema](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_core.go#L200-L216), [main readers](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_world.go#L124-L202), [JSON decoder](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/fact_codec.go#L60-L155)
3. **One file changes and B completes an incremental scan.** Main's existing update transaction deletes that file's cached fast rows and inserts tagged-JSON strings for the new facts. Other files' binary rows remain. SQLite's ordinary BLOB affinity does not prohibit TEXT values, so the table can now contain both formats. The scan saves the updated filesystem manifest. [Main writer](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/store/local_world.go#L205-L276), [scan update](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/world/incremental_scan.go#L605-L609), [manifest publication](https://github.com/theRebelliousNerd/codenerd/blob/9dcb01b9736bf4763aa3ac391d29f133d73c6611/internal/world/incremental_scan.go#L636-L655), [SQLite affinity contract](https://www.sqlite.org/datatype3.html#type_affinity)
4. **A opens again without another source change.** The declared type is still BLOB, so migration returns immediately. Its binary reader accepts the unaffected files' rows but rejects the newly written JSON rows.
5. **A's boot can accept that subset as its world snapshot.** Factory hydration treats any nonempty successfully loaded fact list as `loadedWorld`; after successful kernel loading it does not use the `scan.mg` fallback. With unchanged source files and the valid manifest from step 3, the incremental refresh returns `Unchanged` without regenerating the omitted file's facts. [Hydration and fallback](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/system/factory.go#L1433-L1455), [boot refresh](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/system/factory.go#L1461-L1499), [unchanged branch](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/world/incremental_scan.go#L251-L307)

Conditions matter: at least one valid unaffected binary row must survive and kernel loading must succeed for the nonempty-cache fallback suppression in step 5. With no valid rows, the text snapshot fallback may recover some or all facts if present and adequate. A later relevant source change or a successful explicit reconstruction can regenerate omitted rows. No permanent loss of source files, user knowledge, or database integrity is claimed.

The simpler A→B read-only rollback can likewise fail to decode the converted cache, but its impact depends more directly on the text-snapshot fallback. The mixed-format roll-forward is the stronger acceptance case because even a present text snapshot can be bypassed by a nonempty partial database result.

## Existing protections and recovery to preserve

- **Forward conversion is atomic.** Errors before commit roll the transaction back. The codec intentionally preserves the supported argument semantics, including name/string distinction and exact integer values. Do not present this as an untransactional migration or discard the performance improvement.
- **There are relevant tests.** The branch tests codec parity, malformed binary rejection, JSON-to-binary conversion, omission of already-undecodable legacy JSON, writes after conversion, and reopening with the new implementation. They do not alternate old and new writers. These are read test definitions, not results from this pass. [Codec and migration tests](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/store/world_fact_codec_test.go#L16-L127)
- **No concurrent writer is required.** The witness uses sequential handles/processes, consistent with respecting the store's single-writer discipline. It does not depend on concurrent database corruption.
- **The cache is reconstructible.** The explicit full-scan CLI scans source, validates the resulting facts with a kernel, then persists the fast snapshot and text scan artifact. When that operation and its per-file writes succeed under the desired reader, it provides a recovery path for the fast-cache witness. It does not imply that every deep cache or every historical row has been reconstructed. This memo recommends no live repair or deletion procedure. [Full-scan path](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/cmd/nerd/cmd_init_scan.go#L431-L487), [fast persistence](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/internal/world/persist.go#L25-L84)
- **Ordinary one-way operation is a control.** Fresh binary databases and one-time migration followed only by A writers are not shown to suffer this mixed-format failure. The existing same-reader round-trip goal remains valid.
- **No specific safety outcome is established.** Missing cached facts can change the world available to consumers, but this study did not trace a particular permission, verifier or model decision through the subset. Other source refresh and direct inspection capabilities remain available.

## Proposed contracts and options

### Minimum: explicit format and completeness admission

Separate database readability from world-cache completeness. A loader should expose incompatible/undecodable rows with file/depth scope or return a typed cache-rebuild requirement. It should not silently label a subset as a successful complete snapshot. Factory fallback admission must inspect that status, rather than using `len(cached) > 0` as the entire criterion.

Tie cache-format/schema epoch to the persisted source manifest or another independently checked coverage record. A filesystem-unchanged result is valid only if the cached representation for that same epoch remains usable. Unknown format should cause a controlled cache miss/rebuild decision; a failed or canceled reconstruction must preserve the previous usable state or expose unavailability, not publish a partial replacement.

### Choose the compatibility policy before shipping

1. **Backward-compatible rolling readers:** accept recognized JSON and binary encodings per row, preserving the same typed semantics, then rewrite legacy rows under a controlled migration. This directly handles roll-forward mixing, but does not make already-shipped old binaries understand binary rows.
2. **Versioned reconstructible cache namespace:** keep incompatible generations separate, with a format-specific manifest/coverage identity and deliberate retirement. A different table name alone is insufficient if both generations trust a shared manifest that says unchanged while one table is stale.
3. **Staged compatibility rollout:** first deploy readers and explicit version admission capable of detecting the future format; only later change writers. A single future schema marker cannot force an already-shipped old binary to check it.
4. **Explicitly unsupported rollback:** document the boundary and provide a safe, verified reconstruction workflow, while still making mixed/unknown rows visible to the new reader. “Unsupported” is not a reason for the current reader to silently accept a partial cache.

A STRICT table or encoding CHECK can prevent some old-format writes, but is not a complete repair: old readers still cannot decode binary rows, write failures can still leave the filesystem manifest and cached facts out of step, and consumers must not hide incomplete coverage. Likewise, simply adding a JSON fallback to A does not solve A→B readability.

## Cost and acceptance study

The branch's as-built note reports improved world-cache load time; these are maintainer measurements, not independently reproduced here. Preserve the speed objective while measuring the complete lifecycle. [Existing performance record](https://github.com/theRebelliousNerd/codenerd/blob/a53cfc308e8836c72370adaa46deba3f7b00180d/Docs/audits/mangle-kernel-performance.md#L383-L391)

- Dual-format recognition adds a small dispatch and a legacy decode path; normalization adds rewrite I/O and locking. Whole-table validation can erode warm-boot savings, so amortize it only under a sound generation/coverage invariant.
- Separate generations or dual-written compatibility copies increase disk usage, migration work and retirement complexity. Transactional conversion already retains all legacy rows in memory before insertion; measure peak memory, temporary database/journal growth and writer hold time, not only later decode speed.
- A forced rebuild costs census, hashing, parsing, persistence and kernel loading. Fail-visible behavior can reduce availability but is preferable to silently certifying a partial world. Avoid unconditional rescans on every boot as an unmeasured shortcut.
- Per-file decode status, counts and recovery receipts add API/migration maintenance. Logs should identify format/coverage without dumping source contents.

If later authorized, compare current JSON, branch binary, and proposed admission under identical isolated fixtures:

- One-way migration, repeated same-version reopen and fresh binary database controls.
- A→B without writes; A→B changed-file scan→A with at least one unaffected valid row; a full old-writer rewrite; recognized mixed rows; unknown version, truncated and malformed arguments.
- With and without a valid `scan.mg`; empty versus nonempty surviving subsets; unchanged versus changed manifests; fast and deep depths separately.
- Assert decoded typed fact-set equality or an explicit incomplete/cache-miss outcome—not just SQL row counts, no panic, or one surviving fact. Check boot hydration and the following incremental scan together.
- Preserve transaction rollback, cancellation, disk-error and joined-work behavior. Demonstrate controlled successful reconstruction, including a case where reconstruction fails without erasing unrelated durable data.
- Measure cold conversion, warm loads, alternating-version recovery, allocations, bytes read/written, lock time, database growth and resulting kernel fact sets. Do not count a faster empty/partial load as a performance success.

## Novelty boundary

Current guidance and existing audits were screened through research head [`23246584179fc6f489fc4bf9b665eaaef8c3374a`](https://github.com/theRebelliousNerd/codenerd/tree/23246584179fc6f489fc4bf9b665eaaef8c3374a/Docs/audits). The existing projection-cache allocation audit addresses a different binary consumer and malformed row-count allocation; the embedding migration audit addresses vector identity; scanner graph retirement addresses stale derived relationships. This proposal is the **world-fact codec's rolling-format/coverage contract across shared persisted state**, not another report of those issues or a claim that generic schema migration was absent.

