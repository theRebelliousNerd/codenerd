# CLI — Open Questions

1. **Parity policy:** Which features must exist on both Cobra and slash, vs TUI-only forever?
2. **Boot consolidation:** When can `performSystemBootLegacy` be deleted without breaking callers?
3. **Domain shards vs JIT:** Which verbs still require hard shard types vs pure atom selection?
4. **Auth UX:** Should `auth status` become the single readiness probe for all engines including chat banner?
5. **Assault campaigns:** What is the minimal CI-friendly assault profile that still catches kernel panics?
6. **Modularization:** Preferred package split for `chat/` (`boot`, `process`, `commands`, `model`, `wizards`)?
7. **Observability schema:** Should glass-box events share a versioned schema with transparency logs?
