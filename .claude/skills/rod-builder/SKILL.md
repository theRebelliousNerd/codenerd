---
name: rod-builder
description: Build production-ready browser automation with Rod (go-rod/rod). Use when implementing Chrome DevTools Protocol automation, web scraping, E2E testing, browser session management, or programmatic browser control. Includes Rod API patterns, CDP event handling, Chromium configuration, launcher flags, testing strategies, and production-grade best practices.
---

# Rod Browser Automation Skill

Build production-ready browser automation using **go-rod/rod**, a high-level DevTools Protocol driver for Go.

**Library**: `github.com/go-rod/rod`

## When to Use This Skill

- Building web scrapers or data extraction tools
- Creating browser automation workflows
- Implementing end-to-end testing with real browsers
- Developing browser session management systems
- Creating screenshot/PDF generation services

## Architecture

```text
Your Go Application
    |
    v
Rod Library (High-Level API)
    |
    v
Chrome DevTools Protocol (WebSocket)
    |
    v
Chrome/Chromium Browser
```

**Key Components**: Browser (connection), Page (tab), Element (DOM), Launcher (process manager)

## Quick Start

```go
package main

import "github.com/go-rod/rod"

func main() {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com")
    text := page.MustElement("h1").MustText()
    page.MustScreenshot("screenshot.png")
}
```

## Essential Patterns

### Browser Lifecycle

```go
// Launch with options
import "github.com/go-rod/rod/lib/launcher"

launch := launcher.New().
    Headless(false).
    Set("user-data-dir", "C:\\temp\\chrome-profile")
url := launch.MustLaunch()
browser := rod.New().ControlURL(url).MustConnect()
defer browser.MustClose()

// Incognito context
incognito := browser.MustIncognito()
defer incognito.MustClose()
page := incognito.MustPage("https://example.com")
```

### Navigation & Waiting

```go
page.MustNavigate("https://example.com").MustWaitLoad()
page.MustWait(`() => document.readyState === 'complete'`)
page.MustWaitElementsMoreThan("div.result", 0)
```

### Element Interaction

```go
// Selectors
element := page.MustElement("button#submit")      // CSS
element := page.MustElementX("//button[@id='x']") // XPath
elements := page.MustElements("div.card")         // Multiple

// Actions
element.MustClick()
element.MustInput("Hello World")
element.MustSelect("option-value")

// Data extraction
text := element.MustText()
html := element.MustHTML()
href := element.MustAttribute("href")
```

### JavaScript Evaluation

```go
result := page.MustEval(`(a, b) => a + b`, 10, 20)
sum := result.Int() // 30

// Element context
childCount := element.MustEval(`(el) => el.children.length`).Int()
```

### Network Interception

```go
import "github.com/go-rod/rod/lib/proto"

router := browser.HijackRequests()
defer router.MustStop()

router.MustAdd("*", func(ctx *rod.Hijack) {
    fmt.Printf("Request: %s\n", ctx.Request.URL())
    ctx.MustLoadResponse()
})
go router.Run()
```

### Screenshots & PDFs

```go
// Full page screenshot
data, _ := page.Screenshot(true, &proto.PageCaptureScreenshot{
    Format: proto.PageCaptureScreenshotFormatPng,
})
os.WriteFile("fullpage.png", data, 0644)

// PDF generation
pdf, _ := page.PDF(&proto.PagePrintToPDF{
    PrintBackground: true,
})
os.WriteFile("page.pdf", pdf, 0644)
```

## Common Pitfalls

| Pitfall | Wrong | Correct |
|---------|-------|---------|
| Not waiting | `page.MustElement("div.result")` | `page.MustWaitElementsMoreThan("div.result", 0)` |
| No timeout | `page.Navigate(url)` | `page.Timeout(30*time.Second).Navigate(url)` |
| Leaking pages | `page := browser.MustPage(url)` | `defer page.MustClose()` |
| Late event subscription | Subscribe after navigation | Subscribe before `page.Navigate()` |
| Ignoring JS errors | `page.MustEval(...)` | `result, err := page.Eval(...)` |

## Error Handling

```go
// Without Must* (returns error)
element, err := page.Element("button#submit")
if errors.Is(err, &rod.ElementNotFoundError{}) {
    // Handle not found
}

// With timeout
element, err := page.Timeout(5*time.Second).Element("button#submit")
if errors.Is(err, context.DeadlineExceeded) {
    // Handle timeout
}
```

## Production Checklist

- [ ] Always use `defer browser.MustClose()` and `defer page.MustClose()`
- [ ] Set reasonable timeouts (don't rely on defaults)
- [ ] Handle element not found errors gracefully
- [ ] Use incognito for isolated sessions
- [ ] Subscribe to events before navigation
- [ ] Block unnecessary resources for performance
- [ ] Use context for cancellation support
- [ ] Implement retry logic for flaky operations
- [ ] Consider rate limiting for web scraping

## Reference Documentation

| Reference | Contents |
|-----------|----------|
| [context7-comprehensive.md](references/context7-comprehensive.md) | Latest Rod patterns from Context7 |
| [chromium-guide.md](references/chromium-guide.md) | Chrome configuration, flags, debugging |
| [rod-api.md](references/rod-api.md) | Rod API reference, BrowserNERD integration |
| [cdp-events.md](references/cdp-events.md) | CDP events for fact transformation |
| [selectors.md](references/selectors.md) | CSS and XPath selector patterns |
| [examples.md](references/examples.md) | Web scraping, forms, infinite scroll, sessions |
| [troubleshooting.md](references/troubleshooting.md) | WebSocket errors, connection issues |

**Encountering errors?** Check `troubleshooting.md` first - includes solutions for "websocket bad handshake: 404", Chrome launch failures, element not found, and more.
