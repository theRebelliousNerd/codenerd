# 03 — Gap Analysis: persist

> Last verified against codebase: **2026-08-15**

## 1. Spec vs reality matrix

| Desired capability | Reality | Gap? |
|--------------------|---------|------|
| Compact fact file format | SimpleColumn + gzip/zstd implemented | **No** |
| Round-trip fidelity for common facts | Tests at 1k–10k with equalish compare | **No** |
| Atomic writes | unique temp + sync + chmod + rename + dir fsync | **No** |
| Legacy JSON migration | `Read` + `LegacyJSON` | **No** |
| Production export path | `nerd snapshot export` via `snapshot.Export` | **Closed 2026-08-15** |
| Production import / rehydrate | `nerd snapshot import` (summary / `--assert` / `--to-mangle`) | **Closed 2026-08-15** |
| CLI operator surface (`nerd snapshot …`) | `export`, `import`, `list` | **Closed 2026-08-15** |
| Content sniff if extension wrong | `sniffCodec` + `resolveReadCodec`; content beats suffix | **Closed 2026-08-15** |
| Shared atom conversion with core | Still duplicated, now documented at the divergence and pinned by tests | **Yes — P2** (unification is a core consumer migration; OPEN-QUESTIONS Q5) |
| Logging category | `logging.CategoryStore` debug/warn on write and read | **Partially closed** — a dedicated `CategoryPersist` is still wanted |
| Streaming / append snapshots | Full rewrite only | **Yes — P3** (may stay non-goal) |
| Integrity hash | `<file>.sha256` sidecar, verified on read | **Closed 2026-08-15** |
| Root `package persist` doc | `internal/persist/doc.go` | **Closed 2026-08-15** |
| Concurrency-safe multi-writer | In-process per-path lock; cross-process still caller's job | **Partially closed** |
| Whole-valued float fidelity | Degrades to int64 (upstream rendering) | **Yes — open, upstream (Q8)** |
| Policy enforcement inside factsnap | None | **Non-gap** (correct boundary) |

## 2. Priority backlog (from gaps)

### P0 — Wire or explicitly demote — **DONE (2026-08-15)**

Resolved by option 3 + 4 together: kernel debug export exposed as
`nerd snapshot export|import|list` (`cmd/nerd/cmd_snapshot.go`), with the
workspace layout factored into `internal/persist/snapshot`. It was chosen over
campaign and world because it boots its own kernel locally — no API key, no
network, no shards — and nothing it writes is read back automatically.

Still unwired, now with a paved path:

1. Campaign phase / assault **fact bag** export alongside JSONL results.
2. World scan freeze of `code_*` facts.

The `AUDIT.md` “clean but unwired” note for `internal/persist/factsnap` is
obsolete.

### P1 — Operator path — **DONE (2026-08-15)**

`nerd snapshot export|import|list`, canonical paths documented in
[08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) §4.

### P2 — Hardening — **mostly done**

- Magic-byte detection: done, and content outranks the suffix.
- NameType divergence: documented at `factsnap.baseTermToValue` and pinned by
  `TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom`. The matching note
  could not be added in `internal/core` in this pass; the factsnap comment
  carries the full explanation and the file reference both ways.
- Integration test: `internal/persist/snapshot/kernel_roundtrip_test.go`.
- Remaining: shared conversion helper under `internal/types` (OPEN-QUESTIONS Q5).

### P3 — Polish — **mostly done**

- Logging: `CategoryStore` debug lines carry path, codec, bytes, digest prefix
  and duration. A dedicated `CategoryPersist` still belongs in the logger.
- SHA-256 sidecar: done, sha256sum(1)-compatible.
- Package doc: `internal/persist/doc.go`.
- Remaining: streaming writer, if multi-million fact dumps appear.

## 3. Non-gaps (do not “fix”)

| Item | Why not a gap |
|------|----------------|
| No Mangle Decl | Not a logic package |
| No prompt atoms | No LLM surface |
| Two small subpackages instead of one file | The codec and the workspace layout change for different reasons |
| Uses third-party zstd | Already in `go.mod`; gzip default avoids hard dependency at call sites that only use `Write` |
| Order of `Read` output unstable | Documented; tests sort; SimpleColumn is set-oriented |

## 4. Risk if ignored

The wiring risk is retired; what remains is the risk of the *next* callers not
landing:

1. **A single-caller utility** whose only consumer is an operator command.
2. **Parallel reinvention** — other subsystems keep writing JSON fact dumps
   rather than reaching for `snapshot.Export`.

## 5. Recommended decision

**Wired; now grow callers, not codec surface.** The next high-value caller is a
world code-index freeze (large, highly compressible, `ToFacts()` already exists);
campaign fact bags are second.
