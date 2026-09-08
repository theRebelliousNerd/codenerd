# browser_extract — scoped guidance

`browser_extract` reads bounded, redacted evidence from one live session.

- Lookup and element reads use a child context: earlier caller cancellation
  always wins, and a 10s extraction deadline applies even in long campaigns. Missing
  selectors therefore fail instead of hanging in `page.Element`.
- Errors wrap with `%w` so `errors.Is(err, context.Canceled)` and
  `errors.Is(err, context.DeadlineExceeded)` keep working.
- `max_chars` covers combined text and optional HTML in runes (default 8000,
  hard cap 32000), excluding the explicit truncation notice.
  Non-positive values select the default; oversized values clamp.
- Order is sanitize then bound: full text is redacted via
  `SanitizeForEvidence` first, then cut on a rune boundary so UTF-8 is never
  split. Inputs exactly at the limit are not marked; strictly longer inputs
  append an explicit `[truncated ...]` marker.
- `include_html=true` appends outer HTML under `--- html ---` within the same
  total content budget. `false` returns text only. The flag is never ignored.
- Use Rod's context-bound calls directly; do not leave detached extraction
  goroutines behind after cancellation. A timed-out read must not close the tab.

Do not change browser authority, manager lifetime, or other sessions from
this tool. Keep results bounded; never return unbounded DOM or HTML.
