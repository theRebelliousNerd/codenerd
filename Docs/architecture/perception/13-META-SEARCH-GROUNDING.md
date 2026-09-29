---
doc-class: shipped
subsystem: perception
implementation-status: shipped
last-verified: 2026-09-29
verified-against: bb7bafac
supersedes: []
---

# 13 — Meta (Muse Spark) search grounding

> Written to the `Docs/journeys/09-architecture-doc-standard.md` standard beside this package's
> July-2026 files, which are not rewritten yet. It answers question 2 (what runs) for one
> capability. Written after the code (backwards, as the Repo Contract allows for work already in
> flight on 2026-09-29).

## What runs

A Meta client can ground every completion in live web search, on par with the Gemini client's
Google Search grounding.

- **Config.** `meta` block, `MetaProviderConfig` (`internal/config/meta.go:14`):
  `enable_web_search` (nil = on) and `search_context_size` (`low` | `medium` | `high`, default
  `medium`, `internal/config/meta.go:46`); defaults `internal/config/meta.go:27`; read through
  `GetMetaConfig` (`internal/config/user_config.go:1878`) and copied onto every Meta client slot by
  the factory (`internal/perception/client_factory.go:177`). The classification client never
  searches: it runs on every interactive turn and search is billed per call.
- **Wire.** Meta grounds only on the Responses API. With search on, plain completions, the first
  tool turn and streaming all go to `POST {base}/responses` with the `web_search` tool and
  `include: ["web_search_call.results"]` (`internal/perception/client_meta_responses.go:80`).
  Chat Completions cannot carry the tool, so nothing grounded goes there. Streaming with search on
  is one non-streaming call emitted as one delta.
- **Control surface.** The grounding controller interface is vendor-neutral:
  `SetEnableWebSearch` / `IsWebSearchEnabled` (`internal/types/interfaces.go:275`,
  `internal/types/interfaces.go:296`), implemented by the Meta client
  (`internal/perception/client_meta_responses.go:951`) and the Gemini client. The old
  Google-named methods were renamed at every caller; no alias remains. The broker reports
  grounding capability from `types.GroundingCapable` first (`internal/broker/passthrough.go:122`),
  so init's strategic analysis and campaign planning ground on Meta as they do on Gemini.
- **Answer text.** A grounded run interleaves reasoning, commentary messages
  (`"phase": "commentary"`, "I'll search for..."), `web_search_call` items and a final message.
  The answer is every non-commentary message, whole, separated by a blank line
  (`metaAnswerText`, `internal/perception/client_meta_responses.go:654`; the `Phase` field at
  `internal/perception/client_meta_responses.go:250`). Observed live on
  `muse-spark-1.3-contributor`, 2026-09-29.
- **Sources.** `url_citation` annotations (what the answer cites) and `web_search_call.results`
  (every page retrieved) are both kept (`metaGroundingFromReply`,
  `internal/perception/client_meta_responses.go:1043`) and land where Gemini's grounding sources
  land.
- **Tool.** `grounded_web_search` (`GroundedWebSearch`,
  `internal/perception/client_openai_compat_grounding.go:166`) returns text, citations, every
  retrieved page (title, url, snippet) and usage (`internal/tools/research/grounded_web_search.go`),
  so a model can read an uncited page in full with `web_fetch`.

## Evidence

- `TestMetaGrounding_*` in `internal/perception/client_meta_grounding_test.go`: request bodies with
  search on and off, first tool turn and follow-up, citation + result parsing, reserved tool
  names, piggyback on Responses, empty retry, streaming, factory config,
  `TestMetaGrounding_CommentaryIsNotTheAnswer` (the live reply shape) and
  `TestMetaGrounding_SeparateAnswerMessagesAreNotGlued`.
- `TestGroundedWebSearch_ReturnsRetrievedPages` in
  `internal/tools/research/grounded_web_search_test.go`.
- Live: `muse-spark-1.3-contributor` accepted `web_search` on `/responses` (2026-09-29, three
  searches, a cited answer). The in-repo live test runs only with `NERD_LIVE_META_GROUNDING=1`.

## Not verified / open

- `text.format` (`json_object`) for piggyback on `/responses` is what the client sends; it has
  not been confirmed against Meta's API with a live call.
- `user_location` is not a config key; the tool omits it.
- Every grounded completion is search-capable; the model decides whether to search. Cost is
  $2.50 per 1,000 searches plus tokens (Meta pricing page, read 2026-09-29).
