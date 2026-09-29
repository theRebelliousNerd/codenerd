---
doc-class: shipped
subsystem: tools
implementation-status: shipped
last-verified: 2026-09-29
verified-against: b069d3e4
supersedes: []
---

# 13 — Research reads pages through codeNERD's browser

> Written to `Docs/journeys/09-architecture-doc-standard.md` beside this package's July-2026
> files, which are not rewritten yet. Question 2 (what runs) for one capability, written after
> the code (work in flight on 2026-09-29).

## What runs

Steve, 2026-09-29: "instead of fetch for web page pulling or whatever we are using, use the
browser capabilities native to codenerd to scrape data and do research." No research tool reads
a web page with `net/http` any more.

- **One page read.** `readBrowserPage` (`internal/tools/research/browser_read.go:205`) opens an
  isolated tab, navigates, waits for load and then for DOM stability (the same stable-wait the
  `browser_wait` tool uses), reads the rendered HTML and text, and always closes the tab. A page
  that never goes quiet is an error with no body, never a blank-frame extract. A non-2xx document
  response is reported as `HTTP <code>`.
- **Headless by default.** `research.browser_headless` (`internal/config/research.go:52`, default
  true at `internal/config/research.go:69`) runs research reads in a separate headless Chrome
  (`researchHeadlessManager`, `internal/tools/research/browser_read.go:139`), so a read never pops
  a window even when the interactive browser (`browser.headless`) is visible. A missing Chrome is
  an error naming `browser.launch`.
- **The three tools.** `web_fetch` (`internal/tools/research/web_fetch.go:56`) reads through the
  browser, then keeps its HTML-to-markdown conversion and paging. `web_search`
  (`internal/tools/research/web_search.go:47`) drives the DuckDuckGo HTML results page in the
  browser and parses the live DOM. `context7_fetch` (`internal/tools/research/context7.go:43`)
  reads each documentation URL through the browser; the Context7 key is no longer sent to raw
  GitHub, where it was never a credential.
- **No hidden second browser.** `getBrowserManager` (`internal/tools/research/browser.go`) returns
  the manager boot bound with `SetBrowserRuntime`, or nil. Every browser tool refuses at its entry
  with `requireBoundBrowser` when nothing is bound (`TestBrowserToolsRefuseWhenUnbound`).
- **Kernel action.** A kernel `web_search` action's `Target` becomes the query
  (`internal/core/virtual_store_actions.go`; `TestHandleModularTool_WebSearchTargetIsQuery`).

Search discovery on Meta is `grounded_web_search`
(`Docs/architecture/perception/13-META-SEARCH-GROUNDING.md`); the browser then reads the pages
it returns.

## Evidence

`internal/tools/research/browser_read_test.go` drives real Chrome against local servers: static
HTML, a page that inserts its content by script after load, `text/plain` markdown, tab count back
to baseline after each read, and no `net/http` use in the research tools. It ran (Chrome found at
the standard Windows path) on 2026-09-29.

## Not verified / open

- The live DuckDuckGo site was not opened; headless Chrome may receive a challenge page there.
  The parser is proven on a local document with the same markup.
- A custom `browser.launch` does not reach the headless research browser until the factory
  passes it (`internal/system/factory.go`); auto-detected Chrome is used.
