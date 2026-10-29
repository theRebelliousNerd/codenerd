# persist — Open Questions

> Last verified: **2026-08-15**

## Q1 — Who owns the first call site? — **RESOLVED (2026-08-15)**

**Answer: kernel debug export, surfaced as `nerd snapshot export|import|list`.**

It boots a workspace kernel locally (`core.NewRealKernelWithWorkspace`), so it
needs no API key, no network and no shards, and nothing it writes is read back
automatically. Campaign and world remain candidates, and now have a paved path
through `internal/persist/snapshot` instead of raw `factsnap` calls.

## Q2 — Default codec for production exports? — **RESOLVED (2026-08-15)**

**Answer: gzip stays the default; zstd is one flag away (`--codec zstd`).**

The default optimises for a snapshot being readable by anything (browser,
`gunzip`, another language's stdlib) because the first caller's purpose is
handing a fact dump to a human. Size-sensitive callers — a world index freeze
would be the first — should pass `factsnap.CodecZstd` explicitly.

## Q3 — Rehydrate policy UX? — **RESOLVED (2026-08-15)**

**Answer: C, explicit operator confirmation.**

`nerd snapshot import` summarises by default. `--assert` loads facts into a
kernel that lives and dies with the process. `--to-mangle` renders sorted
Datalog for review; moving that file into `.nerd/mangle/` is a deliberate human
act. There is no boot-time snapshot load and none should be added: a snapshot
becomes untrusted input the moment it leaves the process that wrote it.

## Q4 — Should `Read` sniff magic bytes? — **RESOLVED (2026-08-15)**

**Answer: yes, and content outranks the filename.**

`resolveReadCodec` sniffs gzip (`1f 8b`) and zstd (`28 b5 2f fd`). A suffix can
be lost or lie after a rename; magic bytes cannot be wrong about the container.
A disagreement is logged at warn level and the content wins. Anything with no
recognised magic falls back to the legacy JSON decode, so JSON snapshots are
unaffected.

## Q5 — Unify atom conversion with core? — **still open**

The divergence is now *documented and pinned* rather than accidental:

- `core.baseTermToValue` returns a plain `string` for `ast.NameType`; its
  consumers are query pattern matching and articulation, and core's own tests
  assert that behaviour.
- `factsnap.baseTermToValue` returns `types.MangleAtom`, because a snapshot is
  re-encoded through `types.Fact.ToAtom()` on the next hop, and a plain string
  only survives via `isValidMangleNameConstant` — a heuristic that rejects
  anything with a file extension or more than two slashes. `"/a/b/c.go"` would
  silently degrade from a name constant to a string constant.

So unification is a **consumer migration inside `internal/core`**, not a file
move: a shared helper under `internal/types` has to pick one behaviour and
update the other package's callers and tests. Until someone takes that on,
`TestNameConstant_WhenRoundTripped_ShouldStayMangleAtom` and the core query
tests pin the two behaviours independently so the gap cannot widen unnoticed.

## Q6 — Snapshot versioning?

SimpleColumn has its own header. Do we need an outer envelope with
`codenerd_snapshot_version` for future migrations?

**Status:** open; the `.sha256` sidecar is a natural place to grow a metadata
line if it becomes necessary, but nothing needs it yet. Defer until the first
format-breaking mangle-go upgrade hurts.

## Q7 — Directory naming: `persist` vs `factsnap`? — **RESOLVED (2026-08-15)**

`internal/persist` now has two subpackages (`factsnap`, `snapshot`) and an
umbrella `doc.go`. The directory name earns its keep; no move.

## Q8 — Whole-valued floats degrade to integers — **open, upstream**

`ast.Float64(2.0)` renders as the text `2` in SimpleColumn, so the reader
returns `int64(2)`. Fractional values survive intact, and everything is stable
from the second hop onward.
`TestFloat_WhenRoundTrippedTwice_ShouldBeStableAfterFirstHop` pins the exact
behaviour. Fixing it properly means changing how the mangle-go fork renders
float constants; a caller that must preserve float identity for whole values
should carry its own type-tag argument today.

## Resolved / non-questions

| Topic | Resolution |
|-------|------------|
| Is the package pre-implementation? | **No** — full library, tests, and a production caller |
| Does factsnap enforce `permitted`? | **No** — out of scope; the caller owns policy |
| Are gzip/zstd semantically different? | **No** — proven by `TestCodecParity` |
| Can two writers share a snapshot path? | **Yes, safely** — per-path lock plus temp+fsync+rename |
