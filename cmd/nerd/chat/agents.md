# North-star LLM callers

- North-star document analysis and automatic requirements generation are adapters to `internal/northstar/derive*.go`; keep prompts, paging and parsing in that library.
- Use the model shutdown context and report read failures. Preserve full source paths instead of identifying documents by basename.
- Requirements generation must carry existing IDs and all wizard evidence. Retire the old prose parser when its owner updates `northstar_types.go`; do not restore its runtime path.
