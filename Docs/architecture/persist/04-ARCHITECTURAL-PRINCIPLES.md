# 04 — Architectural Principles: persist / factsnap

> Last verified against codebase: **2026-07-13**  
> These principles are **binding** for changes under `internal/persist/`.

## P1 — Facts only

Serialize `[]types.Fact` (and only that shape). Do not grow factsnap into a general filesystem, config, or chat-log store.

## P2 — Semantic codecs, not semantic forks

Gzip vs zstd (and Auto→gzip) must remain **semantically identical**. Any new codec must pass a parity test like `TestCodecParity`.

## P3 — Deterministic SimpleColumn

Always write with `SimpleColumn{Deterministic: true}` unless a measured, documented reason appears. Determinism enables reproducible artifacts and stable tests.

## P4 — Atomic publish

Never leave a partial final path. Pattern: write `*.tmp` → sync → close → rename; remove tmp on failure. Preserve this for every new write entrypoint.

## P5 — Extension is the contract

Canonical extensions (`.sc.gz`, `.sc.zst`) are part of the public API. `CanonicalPath` / `ensureExt` own rewriting; callers may pass logical basenames.

## P6 — Avoid core import cycles

Do not import `internal/core` from factsnap. Local `atomToFact` is intentional. If consolidation is needed, extract a **neutral** helper under `internal/types` or a tiny `internal/facts` convert package — not core→persist or persist→core.

## P7 — Preserve name-constant round-trips

Name constants must survive Write→Read as values that `ToAtom()` re-encodes as names (today: `types.MangleAtom`). Do not “simplify” to bare strings without updating tests and `ToAtom` symmetry.

## P8 — Policy lives outside

factsnap never implements `permitted(...)`. Loaders that assert into the kernel own safety.

## P9 — Prefer compression that earns its keep

Default gzip for portability; zstd for size. Keep `TestSizeComparison` (or successor) so regressions where SimpleColumn+gzip loses to JSON are caught.

## P10 — Wiring audit before deletion

Zero importers is **not** proof of dead code. This package is a known dormant integration point. Audit callers, tests, and design intent before removal.

## P11 — Errors are wrapped and prefixed

All public errors use `factsnap: …` prefixes with `%w` where appropriate. Keep that for greppability.

## P12 — Library stays dependency-light

Default write path must not force zstd usage. Optional zstd is fine because the module already depends on klauspost; do not add heavy new deps for marginal format gains.
