# browser_extract — scoped guidance

`browser_extract` reads bounded, redacted evidence from one live session.

- Lookup and element reads use a child context: earlier caller cancellation
  always wins, and a 10s extraction deadline applies even in long campaigns. Missing
  selectors therefore fail instead of hanging in `page.Element`.
- Errors wrap with `%w` so `errors.Is(err, context.Canceled)` and
  `errors.Is(err, context.DeadlineExceeded)` keep working.
- `max_chars` sizes one paging window over combined text and optional HTML
  in runes (default 8000, hard cap 32000), excluding the paging notice.
  Non-positive values select the default; oversized values clamp. `offset`
  starts the window (default 0, negatives clamp to 0); every rune stays
  reachable by paging, so the window is paging, never a cut.
- Order is sanitize then window: full text is redacted via
  `SanitizeForEvidence` first, then windowed on a rune boundary so UTF-8 is
  never split. Inputs fitting the window are not marked; a window ending
  before the content appends `[showing runes a-b of N; M more chars, page
  with offset=b]`, and a non-zero offset reaching the end appends
  `[end of content, N chars total]`.
- `include_html=true` appends outer HTML under `--- html ---` within the same
  total content budget. `false` returns text only. The flag is never ignored.
- Use Rod's context-bound calls directly; do not leave detached extraction
  goroutines behind after cancellation. A timed-out read must not close the tab.

Do not change browser authority, manager lifetime, or other sessions from
this tool. Keep each result bounded (one window); never return unbounded DOM
or HTML — but never silently drop content either: anything past the window
must be named with the offset that reaches it.

Page reads go through the browser (2026-09-29): `web_fetch`, `web_search` and
`context7_fetch` call `readBrowserPage` (isolated tab, load + DOM stability,
always closed), headless unless `research.browser_headless` is false. Do not
add a `net/http` page read. Browser tools refuse with `requireBoundBrowser`
when boot bound no manager; there is no fallback manager. Spec:
`Docs/architecture/tools/13-RESEARCH-READS-THROUGH-BROWSER.md`.
