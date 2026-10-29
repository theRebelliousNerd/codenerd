# usage — Corpus Progress

## 2026-07-13 — Full rebuild (SUBAGENT_INSTRUCTIONS)

- Re-read entire `internal/usage/` (2 src + 4 tests).
- Reverse-dep scan of `codenerd/internal/usage` across `cmd/` and `internal/`.
- Traced boot (`system/factory.go`), ZAI Track, shard `WithShardContext`, chat/CLI `NewContext`, UI page.
- Replaced thin auto-inventory stubs with **full new-named doc set** per rebuild contract:
  - README, IMPLEMENTED_SPEC
  - 00–12 series (vision, state, gaps, principles, architecture, API, deps, wiring, safety, tests, observability, failures)
  - TODO, OPEN-QUESTIONS, _progress
- No Go/Mangle/code changes.
- Flagship IMPLEMENTED_SPEC expanded with control-flow narrative, integration map, honesty about Events/Cost/ZAI-only producers.

## 2026-08-15 — P0/P4 backlog closeout (code + docs)

- **Producer coverage.** Every HTTP `perception.LLMClient` that receives token
  counts now meters through one helper (`internal/perception/usage_track.go`):
  zai, anthropic, openai, gemini, xai, openrouter, ollama, dashscope, meta,
  moonshot — chat, structured-output, tool, tool-result, grounded-search and
  streaming paths. Tracking sits on the shared funnels (`executeChat`,
  `completeNonStreaming`) where one exists, so a new caller cannot forget.
- **Streaming.** Each stream tracks exactly once, after the stream ends, from
  the final billed usage payload. xAI streaming delegates to the non-streaming
  path and deliberately does not track twice. Gemini folds `thoughtsTokenCount`
  into output because Google bills it as output.
- **Provider ids.** Standardized on the config engine names, with a test that
  checks each id against `config.SetAPIKeyForProvider`. Ollama no longer bills
  to `openai` despite sharing that transport.
- **Single owner.** `usage.Shared` is a refcounted per-workspace registry;
  Cortex and the chat model now hold the same tracker instead of two trackers
  overwriting one `usage.json`. Only the last `Close` shuts it down.
- **Not done:** CLI engines (`claude-cli`, `codex-cli`) stay unmetered because
  their decoders receive no token counts; cross-process coordination remains
  last-writer-wins.

## Status

| Item | State |
|------|-------|
| Research | Done |
| Full doc set | Done |
| Code changes | 2026-07-13: none. 2026-08-15: producer wiring, provider-id standardization, `Shared` registry |
