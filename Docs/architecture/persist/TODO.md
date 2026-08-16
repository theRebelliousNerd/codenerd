# persist — TODO

> Last verified: **2026-08-16**  
> Items marked done carry the commit-visible evidence in parentheses.

## P0 — Product wiring

- [x] Choose first production caller — **kernel debug export** via `nerd snapshot`
      (`cmd/nerd/cmd_snapshot.go`; rationale in [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) §2)
- [x] Implement export/import at that site using `factsnap.Write` / `Read`
      (through `internal/persist/snapshot`, which owns the workspace layout)
- [x] Add integration test: domain → facts → snap → facts → kernel equalish
      (`internal/persist/snapshot/kernel_roundtrip_test.go`)
- [x] Update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) with real call sites

## P1 — Operator surface

- [x] CLI: `nerd snapshot export | import | list` under `.nerd/snapshots/`
- [x] Document canonical workspace paths ([08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) §4)

## P2 — Hardening

- [x] Content sniff when the suffix is missing or wrong (gzip `1f 8b`, zstd `28 b5 2f fd`);
      content beats the filename, since magic bytes cannot be wrong about the container
- [x] Cross-link comments for the `core.baseTermToValue` vs `factsnap.baseTermToValue`
      NameType divergence — documented at the point of divergence and pinned by
      `TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom`
- [x] Explicit tests for empty slice, bool, float multi-hop
      (`factsnap_robustness_test.go`)
- Deferred - Shared conversion helper under `internal/types` — still open; see
      [OPEN-QUESTIONS.md](OPEN-QUESTIONS.md) Q5. Unification is a consumer
      migration in `internal/core`, not a file move, so it did not belong in
      this pass. Deferred by choice rather than blocked: the helper itself is trivial, but unifying it means migrating consumers inside `internal/core`, which is a separate change with its own risk, and doing it opportunistically inside an unrelated pass is how half-migrations happen. Should be picked up as its own scoped change.

  These are recorded as plain bullets rather than checkboxes because a checkbox reads as unfinished work, and these are decisions and standing directions.

## P3 — Polish

- [x] Logging (size, duration, codec, digest) on write/read — `logging.CategoryStore`
      for now; a dedicated `CategoryPersist` is the right home once the logger
      gains one
- [x] Integrity hash sidecar — `<snapshot>.sha256`, sha256sum(1)-compatible,
      verified by `Read`, surfaced by `snapshot.List` and `nerd snapshot list`
- [x] Package-level doc file — `internal/persist/doc.go` now that there are two
      subpackages
- Decided NO (for now) - Streaming writer — still not justified; the whole snapshot is buffered in
      memory before compression. Revisit if an export exceeds ~1M facts. This is a decision not to build until a real export exceeds roughly 1M facts, since buffering is simpler and no observed workload has reached that size. The revisit condition is the measurement, not a date.

  These are recorded as plain bullets rather than checkboxes because a checkbox reads as unfinished work, and these are decisions and standing directions.

## Documentation

- [x] Full architecture corpus rebuild (2026-07-13)
- [x] Refresh reverse-deps after first real importer landed (2026-08-15,
      [07-DEPENDENCY-MAP.md](07-DEPENDENCY-MAP.md))

## Non-goals (do not queue)

- Replacing `internal/store` sqlite cold path
- Embedding / blob storage inside factsnap
- Policy enforcement inside the codec
- Automatic load of snapshots at boot (see [08](08-WIRING-AND-INTEGRATION.md) §3)
