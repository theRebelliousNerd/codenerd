# 00 — Alignment & Vision Review: persist

> Last verified against codebase: **2026-07-13**  
> Status: Living Reference Document — code-grounded  
> Source: `internal/persist/factsnap/` (1 source file, 4 tests)

## 1. North-star statement

codeNERD separates **LLM creativity** from **Mangle executive control**. Persistence of *facts* is an executive/data concern: once facts exist as `types.Fact`, durable projection must be **deterministic, compact, and recoverable** without re-asking the model.

`factsnap` aligns with that split: it never interprets intent, never calls an LLM, and never issues `next_action`. It serializes atoms the kernel already trusts (or that a caller will re-validate on load).

## 2. Alignment dimensions

| Dimension | Score (0–5) | Evidence |
|-----------|-------------|----------|
| Creative/executive split | **5** | Pure codec library; no model I/O (`factsnap.go` package doc, imports) |
| Fact-flow fidelity | **3** | Correct `types.Fact` ↔ SimpleColumn round-trip *when used*; not on OODA hot path; **zero importers** |
| Constitutional safety | **2** | No `permitted(...)` (correct for a serializer); risk is *unreviewed re-assert* if a future caller loads snapshots into EDB without policy |
| JIT / atom discipline | **5** | N/A positively: no LLM-facing surface to corrupt with ad-hoc prompts |
| Deterministic durability | **5** | `SimpleColumn{Deterministic: true}`; atomic rename; codec parity tests |
| Observability | **1** | No `internal/logging` categories; errors via wrapped `fmt.Errorf` only |
| Test grounding | **5** | Round-trip, parity @ 10k, size regression, unknown-codec cleanup |
| Integration / wiring honesty | **2** | Package is clean but **dormant** — classic half-integrated utility |

**Overall alignment: 3.5 / 5** — excellent *library* alignment with the north star; weak *platform* alignment until something actually writes/reads snapshots in production flows.

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
