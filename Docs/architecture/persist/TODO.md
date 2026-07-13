# persist — TODO

> Last verified: **2026-07-13**  
> Docs-only backlog; items are recommendations, not scheduled work.

## P0 — Product wiring

- [ ] Choose first production caller (campaign fact bag **or** world code-index freeze **or** kernel debug export)
- [ ] Implement export/import at that site using `factsnap.Write` / `Read`
- [ ] Add integration test: domain → facts → snap → facts → domain/kernel equalish
- [ ] Update [08-WIRING-AND-INTEGRATION.md](08-WIRING-AND-INTEGRATION.md) with real call sites when done

## P1 — Operator surface

- [ ] Optional CLI: export/import fact snapshots under `.nerd/snapshots/`
- [ ] Document canonical workspace paths once chosen

## P2 — Hardening

- [ ] Optional content sniff when suffix missing (gzip `1f 8b`, zstd magic)
- [ ] Cross-link comments: `core.baseTermToValue` vs `factsnap.baseTermToValue` NameType divergence
- [ ] Explicit tests for empty slice, bool, float multi-hop
- [ ] Consider shared conversion helper under `internal/types` if drift becomes painful

## P3 — Polish

- [ ] Logging (size, duration, codec) on write/read
- [ ] Optional integrity hash sidecar
- [ ] Package-level doc file or root re-export if more subpackages appear
- [ ] Streaming writer only if multi-million fact dumps appear in practice

## Documentation

- [x] Full architecture corpus rebuild (2026-07-13)
- [ ] Refresh reverse-deps after first real importer lands

## Non-goals (do not queue)

- Replacing `internal/store` sqlite cold path
- Embedding / blob storage inside factsnap
- Policy enforcement inside the codec
