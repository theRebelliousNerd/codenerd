# 00 — Alignment & Vision Review: persist

> Last verified against codebase: **2026-08-15**  
> Status: Living Reference Document — code-grounded  
> Source: `internal/persist/` (`factsnap`, `snapshot`, `doc.go`; 6 test files) plus `cmd/nerd/cmd_snapshot.go`

## 1. North-star statement

codeNERD separates **LLM creativity** from **Mangle executive control**. Persistence of *facts* is an executive/data concern: once facts exist as `types.Fact`, durable projection must be **deterministic, compact, and recoverable** without re-asking the model.

`factsnap` aligns with that split: it never interprets intent, never calls an LLM, and never issues `next_action`. It serializes atoms the kernel already trusts (or that a caller will re-validate on load).

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Pure codec library; no model I/O (`factsnap.go` package doc, imports) |
| Fact-flow fidelity | **4** | `types.Fact` ↔ SimpleColumn round trip proven against a real kernel on both ends (`kernel_roundtrip_test.go`); still off the OODA hot path by design |
| Constitutional safety | **4** | No `permitted(...)` inside the codec (correct), and the caller refuses to assert without an explicit flag; no boot-time load exists to smuggle facts in |
| JIT / atom discipline | **5** | N/A positively: no LLM-facing surface to corrupt with ad-hoc prompts |
| Deterministic durability | **5** | `SimpleColumn{Deterministic: true}`; unique-temp + fsync + rename + dir fsync; sha256 sidecar; codec parity tests |
| Observability | **3** | `logging.CategoryStore` debug lines with codec, bytes, digest and duration; `nerd snapshot list` answers "what is on disk" |
| Test grounding | **5** | Round-trip, parity @ 10k, size regression, contention, integrity, multi-hop type contracts, command tests |
| Integration / wiring honesty | **4** | One deliberate caller with a documented rationale; campaign and world still open, and the docs say so |

**Overall alignment: 4.3 / 5** — the library was always well aligned; it now has a
production path that respects the executive/creative split (the kernel decides
what is true, the CLI only projects it to disk) and refuses to become a silent
boot-time fact source.

## 3. What “good” looks like (persist-specific)

| Good | Bad |
|------|-----|
| Snapshot is a pure function of `[]types.Fact` + codec | Snapshot embeds LLM prose blobs as unstructured JSON without fact shape |
| Write is atomic and deterministic | Partial files left on crash |
| Extension encodes codec | Magic-byte-only detection with silent misreads (today is suffix-only — callers must honor extensions) |
| Compression beats JSON on realistic corpora | Gzip larger than JSON (guarded by `TestSizeComparison`) |
| Callers rehydrate under policy | Blind `kernel.Assert` of untrusted snapshot files |
| Wiring audited before deletion | Delete package because “no importers” without reading tests + design |

## 4. Verdict

Living, high-quality **utility** with intentional Mangle SimpleColumn choice. Primary misalignment is **non-use**, not incorrect design. Next alignment work is product wiring (export/import surfaces), not more codecs.

## 5. Related corpora

- [`types`](../types/) — fact value types  
- [`core`](../core/) — live EDB / `atomToFact` sibling  
- [`store`](../store/) — alternate durability (sqlite)  
- [`campaign`](../campaign/) — JSON artifact precedent  
