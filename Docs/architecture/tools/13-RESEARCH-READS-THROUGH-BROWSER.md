---
doc-class: shipped
subsystem: tools
implementation-status: shipped
last-verified: 2026-10-28
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

- **One page read.** `readBrowserPage` (`internal/tools/research/browser_read.go`) opens an
  isolated tab, navigates, waits for load and then for DOM stability (the same stable-wait the
  `browser_wait` tool uses), reads the rendered HTML and text, and always closes the tab. A page
  that never goes quiet is an error with no body, never a blank-frame extract. A non-2xx document
  response is reported as `HTTP <code>`.
- **Headless by default.** `research.browser_headless` (`internal/config/research.go:52`, default
  true at `internal/config/research.go:69`) runs research reads in a separate headless Chrome
  (`BrowserRuntime.headlessManager`, `internal/tools/research/browser_runtime.go`), so a read never pops
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
- ~~A custom `browser.launch` does not reach the headless research browser until the factory
  passes it.~~ Closed 2026-10-28: the factory passes `browser.launch`, `browser.reaper`,
  `browser.extra_sensitive_keys`, `browser.max_tabs` and `browser.max_browsers` to
  `research.SetBrowserRuntime`; see "Ownership and retirement" below.

## Ownership and retirement (2026-10-28)

Spec for `Docs/audits/research-browser-ownership-and-evidence-retirement.md`; built.

- **One owner.** `research.BrowserRuntime` (`internal/tools/research/browser_runtime.go`) holds the
  bound interactive manager, the kernel, and the lazily started headless manager. Boot creates it
  (`initAutopoiesisAndBrowser`), `Cortex.Close` closes it before the interactive manager, and boot
  rollback runs `Cortex.Close`. `Close` stops admission, cancels and joins admitted reads, then
  shuts the headless Chrome down; it never touches a replacement runtime's binding.
- **One lease per read.** `readBrowserPage` leases the runtime: manager, kernel and cancellation
  are fixed for the read. The headless manager's sink and retractor are fixed to the runtime's
  kernel (`runtimeKernelSink`); nothing consults the mutable process binding mid-read.
- **Effective configuration.** Research inherits `browser.launch` (binary and flags, minus
  `--user-data-dir` and `--remote-debugging-*`, which are dropped and logged), `browser.reaper`,
  `browser.extra_sensitive_keys`, `browser.max_tabs` and `browser.max_browsers`. Unset
  `browser.launch` falls back to discovery. Research still forces headless, isolated tabs, no
  header ingestion, no bulk evidence recording, no session store, no debugger URL.
- **Evidence retirement.** The DOM sensor publishes `dom_updated` through
  `SessionManager.PublishSessionFacts` (budgeted, sanitized, tracked) and is joined before the tab
  closes. The headless manager has a kernel retractor. Tracked facts are the sanitized spelling the
  sink stored; `browser_epoch` and `browser_stream_saturated` are tracked with their epoch. A saturated
  epoch leaves `SessionFactStats.SaturatedEpochs`. A failed retraction is returned by
  `CloseSession` and queued (`PendingRetirements`) for exact-fact retry.
- **Terminal states.** A sensor that cannot install/publish, or a saturated stream, makes the
  read fail as observation-incomplete. Content read but cleanup failed is
  `browserPage.CleanupErr`; `web_fetch` and `web_search` append a "cleanup pending" notice, and
  Context7 (archived source text) logs it.

## Observation identity (2026-10-28)

- `web_search` reports "No results" only when the page carries DuckDuckGo's own empty-results
  state (a `no-results` class, `duckDuckGoReportsNoResults`); a page with no result cards and no
  such state (a challenge, consent page, changed layout) is an error saying nothing is established.
  The empty answer is scoped: the provider's answer at that time, not proof of absence.
- `readBrowserPage` reads `document.URL` and `document.baseURI` in the same evaluation as the DOM.
  `web_fetch` names the document when the browser ended somewhere other than the requested URL
  (`[document: ... (requested ...)]`) and resolves relative links against `baseURI`; credentials
  are dropped from emitted links.
- A paged `web_fetch` names the snapshot (`sha256:` of the converted document) its window was cut
  from; a continuation passing `snapshot` is refused when the document changed. The unpaged fetch
  archived whole by the working context and paged with `recall_context` remains the way to read
  one retained snapshot without navigating again.
- A research result in the working context (`recordWorkingResult`, any `EffectExternal` tool with
  no file argument) is a non-file observation with a stable revision, not an observation of the
  focus file at that file's digest.
