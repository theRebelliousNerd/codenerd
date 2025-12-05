---
name: rod-builder
description: Build production-ready browser automation with Rod (go-rod/rod). Use when implementing Chrome DevTools Protocol automation, web scraping, E2E testing, browser session management, or programmatic browser control. Includes Rod API patterns, CDP event handling, Chromium configuration, launcher flags, testing strategies, and production-grade best practices. (project)
---

# Rod Browser Automation Skill

## Purpose

Guide production-complete implementation of browser automation using **go-rod/rod**, a high-level DevTools Protocol driver for Go that provides:
- Direct Chrome/Chromium control via CDP (Chrome DevTools Protocol)
- Session management with incognito contexts
- Element interaction and JavaScript evaluation
- Network interception and monitoring
- Screenshot and PDF generation
- Event-driven architecture for real-time monitoring

## When to Use This Skill

Deploy this skill when:
- Building web scrapers or data extraction tools
- Creating browser automation workflows
- Implementing end-to-end testing with real browsers
- Developing browser session management systems
- Debugging web applications via CDP
- Creating screenshot/PDF generation services
- Building tools that require programmatic browser control

**Library**: github.com/go-rod/rod

## Core Rod Concepts

### Architecture

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

### Key Components

1. **Browser** - Connection to Chrome instance
2. **Page** - Single browser tab/window
3. **Element** - DOM element reference
4. **Launcher** - Chrome process manager
5. **Proto** - CDP type definitions

## Quick Start Pattern

```go
package main

import (
    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
)

func main() {
    // Launch browser
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    // Create page
    page := browser.MustPage("https://example.com")

    // Interact with page
    page.MustElement("h1").MustText() // "Example Domain"

    // Take screenshot
    page.MustScreenshot("screenshot.png")
}
```

## Browser Lifecycle

### Connecting to Existing Chrome

```go
// Chrome must be started with: --remote-debugging-port=9222
browser := rod.New().
    ControlURL("ws://localhost:9222").
    MustConnect()
defer browser.MustClose()
```

**Use Case**: Attach to user's running Chrome session

### Launching Chrome Programmatically

```go
import "github.com/go-rod/rod/lib/launcher"

// Basic launch
url := launcher.New().MustLaunch()
browser := rod.New().ControlURL(url).MustConnect()
defer browser.MustClose()

// Advanced launch with options
launch := launcher.New().
    Bin("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe").
    Headless(false).
    Devtools(false).
    Set("user-data-dir", "C:\\temp\\chrome-profile").
    Set("remote-debugging-port", "9222")

url, err := launch.Launch()
browser := rod.New().ControlURL(url).MustConnect()
```

**Use Case**: Full control over Chrome instance

### Incognito Mode

```go
// Create incognito browser
incognito := browser.MustIncognito()
defer incognito.MustClose()

// Pages in incognito context
page := incognito.MustPage("https://example.com")
```

**Use Case**: Isolated sessions without cookies/cache

## Page Management

### Navigation

```go
// Navigate and wait for load
page.MustNavigate("https://example.com").MustWaitLoad()

// Navigate with custom wait condition
page.Navigate("https://example.com")
page.MustWait(`() => document.readyState === 'complete'`)

// Get current URL
url := page.MustInfo().URL

// Go back/forward
page.MustNavigateBack()
page.MustNavigateForward()

// Reload
page.MustReload()
```

### Multiple Pages

```go
// List all pages
pages, _ := browser.Pages()
for _, p := range pages {
    info := p.MustInfo()
    fmt.Printf("Page: %s - %s\n", info.Title, info.URL)
}

// Create new page
page1 := browser.MustPage("https://example.com")
page2 := browser.MustPage("https://example.org")

// Close page
page1.MustClose()
```

### Page Context

```go
// Set custom timeout
page := page.Timeout(30 * time.Second)

// With context cancellation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
page = page.Context(ctx)
```

## Element Interaction

### Selectors

```go
// CSS selector
element := page.MustElement("button#submit")

// XPath
element := page.MustElementX("//button[@id='submit']")

// Wait for element (with timeout)
element, err := page.Timeout(5*time.Second).Element("div.loading")

// Find multiple elements
elements := page.MustElements("div.card")
for _, el := range elements {
    text := el.MustText()
    fmt.Println(text)
}

// Find element within element
parent := page.MustElement("div.container")
child := parent.MustElement("button")
```

### Actions

```go
// Click
element.MustClick()

// Type text (simulates keyboard)
element.MustInput("Hello World")

// Select dropdown option
element.MustSelect("option-value")

// Check/uncheck checkbox
checkbox := page.MustElement("input[type='checkbox']")
checkbox.MustClick() // Toggle

// Upload file
page.MustElement("input[type='file']").
    MustSetFiles("C:\\path\\to\\file.pdf")

// Hover
element.MustHover()

// Right click
element.MustClick(proto.InputMouseButtonRight)

// Double click
element.MustClickDouble()
```

### Getting Element Data

```go
// Get text content
text := element.MustText()

// Get HTML
html := element.MustHTML()

// Get attribute
href := element.MustAttribute("href")

// Get property (JavaScript property, not HTML attribute)
value := element.MustProperty("value")

// Get computed style
color := element.MustEval(`(el) => window.getComputedStyle(el).color`).String()

// Check visibility
visible := element.MustVisible()

// Get bounding box
box := element.MustShape()
// box.Box() returns x, y, width, height
```

### Waiting Strategies

```go
// Wait for element to appear
element := page.MustWaitElementsMoreThan("div.result", 0)[0]

// Wait for element to be visible
page.MustElement("div.modal").MustWaitVisible()

// Wait for element to be stable (no changes for duration)
element.MustWaitStable(time.Second)

// Wait for element to be enabled
element.MustWaitEnabled()

// Wait for custom condition
page.MustWait(`() => document.querySelector('#data').textContent !== 'Loading...'`)

// Wait for navigation
page.MustWaitNavigation()
```

## JavaScript Evaluation

### Execute JavaScript

```go
// Simple evaluation
result := page.MustEval(`1 + 2`)
sum := result.Int() // 3

// With arguments
result := page.MustEval(`(a, b) => a + b`, 10, 20)
sum := result.Int() // 30

// Access global objects
url := page.MustEval(`window.location.href`).String()

// Mutation (no return value)
page.MustEval(`document.title = 'New Title'`)

// Complex object return
result := page.MustEval(`({name: 'Alice', age: 30})`)
obj := result.Value.Map()
name := obj["name"].String() // "Alice"
age := obj["age"].Int()       // 30
```

### Element Context Evaluation

```go
element := page.MustElement("div#container")

// Evaluate with element as argument
childCount := element.MustEval(`(el) => el.children.length`).Int()

// Get computed styles
bgColor := element.MustEval(`(el) => window.getComputedStyle(el).backgroundColor`).String()
```

### Common JavaScript Patterns

```go
// Scroll to element
element.MustEval(`(el) => el.scrollIntoView()`)

// Trigger event
element.MustEval(`(el) => el.dispatchEvent(new Event('change'))`)

// Get all text nodes
texts := page.MustEval(`
    Array.from(document.querySelectorAll('*'))
        .filter(el => el.childNodes.length === 1 && el.childNodes[0].nodeType === 3)
        .map(el => el.textContent.trim())
        .filter(t => t.length > 0)
`).Arr()

// Check if element is in viewport
inView := element.MustEval(`(el) => {
    const rect = el.getBoundingClientRect();
    return rect.top >= 0 && rect.bottom <= window.innerHeight;
}`).Bool()
```

## CDP Events

### Network Monitoring

```go
import "github.com/go-rod/rod/lib/proto"

// Subscribe to network events
router := browser.HijackRequests()
defer router.MustStop()

router.MustAdd("*", func(ctx *rod.Hijack) {
    // Intercept request
    fmt.Printf("Request: %s %s\n", ctx.Request.Method(), ctx.Request.URL())

    // Modify request
    ctx.Request.SetHeader("Authorization", "Bearer token")

    // Continue or respond
    ctx.MustLoadResponse()

    // Log response
    fmt.Printf("Response: %d\n", ctx.Response.Payload().ResponseCode)
})

go router.Run()

// Navigate (requests will be intercepted)
page.MustNavigate("https://example.com")
```

### Request Blocking

```go
router.MustAdd("*.png", func(ctx *rod.Hijack) {
    // Block images
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})

router.MustAdd("*/ads/*", func(ctx *rod.Hijack) {
    // Block ad requests
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})
```

### Console Monitoring

```go
go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
    level := string(e.Type) // "log", "error", "warning"

    // Extract arguments
    messages := []string{}
    for _, arg := range e.Args {
        messages = append(messages, fmt.Sprintf("%v", arg.Value))
    }

    fmt.Printf("[%s] %s\n", level, strings.Join(messages, " "))
})()

// Page will emit console events
page.MustNavigate("https://example.com")
```

### Navigation Monitoring

```go
go page.EachEvent(func(e *proto.PageFrameNavigated) {
    fmt.Printf("Navigated to: %s\n", e.Frame.URL)
})()
```

## Cookies and Storage

### Cookie Management

```go
// Get cookies
cookies := page.MustCookies()
for _, cookie := range cookies {
    fmt.Printf("%s = %s\n", cookie.Name, cookie.Value)
}

// Set cookie
page.MustSetCookies(&proto.NetworkCookie{
    Name:     "session_id",
    Value:    "abc123",
    Domain:   "example.com",
    Path:     "/",
    Secure:   true,
    HTTPOnly: true,
})

// Delete cookies
page.MustSetCookies() // Clear all
```

### LocalStorage and SessionStorage

```go
// Get localStorage
data := page.MustEval(`
    JSON.stringify(Object.keys(localStorage).reduce((acc, key) => {
        acc[key] = localStorage.getItem(key);
        return acc;
    }, {}))
`).String()

// Set localStorage
page.MustEval(`localStorage.setItem('key', 'value')`)

// Clear localStorage
page.MustEval(`localStorage.clear()`)

// SessionStorage (same API)
page.MustEval(`sessionStorage.setItem('key', 'value')`)
```

## Screenshots and PDFs

### Screenshots

```go
// Full page screenshot
data, err := page.Screenshot(true, &proto.PageCaptureScreenshot{
    Format:  proto.PageCaptureScreenshotFormatPng,
    Quality: 100,
})
os.WriteFile("fullpage.png", data, 0644)

// Viewport only
data, err := page.Screenshot(false, nil)

// Element screenshot
element := page.MustElement("div#chart")
data, err := element.Screenshot(proto.PageCaptureScreenshotFormatPng, 90)
os.WriteFile("element.png", data, 0644)
```

### PDF Generation

```go
pdf, err := page.PDF(&proto.PagePrintToPDF{
    Landscape:           false,
    DisplayHeaderFooter: false,
    PrintBackground:     true,
    Scale:               1.0,
    PaperWidth:          8.5,
    PaperHeight:         11.0,
    MarginTop:           0.4,
    MarginBottom:        0.4,
    MarginLeft:          0.4,
    MarginRight:         0.4,
})
os.WriteFile("page.pdf", pdf, 0644)
```

## Common Patterns

### Web Scraping

```go
func ScrapeProducts(url string) ([]Product, error) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)

    // Wait for products to load
    page.MustWaitElementsMoreThan("div.product", 0)

    // Extract data
    products := []Product{}
    elements := page.MustElements("div.product")

    for _, el := range elements {
        product := Product{
            Name:  el.MustElement(".name").MustText(),
            Price: el.MustElement(".price").MustText(),
            URL:   el.MustElement("a").MustAttribute("href"),
        }
        products = append(products, product)
    }

    return products, nil
}
```

### Form Automation

```go
func SubmitForm(url, username, password string) error {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage(url)

    // Fill form
    page.MustElement("input[name='username']").MustInput(username)
    page.MustElement("input[name='password']").MustInput(password)

    // Submit
    page.MustElement("button[type='submit']").MustClick()

    // Wait for redirect
    page.MustWaitNavigation()

    // Verify success
    if page.MustInfo().URL == url+"/dashboard" {
        return nil
    }

    return errors.New("login failed")
}
```

### Infinite Scroll

```go
func ScrollToBottom(page *rod.Page) {
    for {
        // Get current height
        oldHeight := page.MustEval(`document.body.scrollHeight`).Int()

        // Scroll down
        page.MustEval(`window.scrollTo(0, document.body.scrollHeight)`)

        // Wait for new content
        time.Sleep(500 * time.Millisecond)

        // Check if height changed
        newHeight := page.MustEval(`document.body.scrollHeight`).Int()
        if newHeight == oldHeight {
            break // Reached bottom
        }
    }
}
```

### Session Reuse

```go
type SessionManager struct {
    browser *rod.Browser
    pages   map[string]*rod.Page
    mu      sync.RWMutex
}

func (sm *SessionManager) GetPage(id string) (*rod.Page, error) {
    sm.mu.RLock()
    page, exists := sm.pages[id]
    sm.mu.RUnlock()

    if exists {
        return page, nil
    }

    return nil, errors.New("page not found")
}

func (sm *SessionManager) CreatePage(id, url string) (*rod.Page, error) {
    sm.mu.Lock()
    defer sm.mu.Unlock()

    page := sm.browser.MustIncognito().MustPage(url)
    sm.pages[id] = page

    return page, nil
}
```

## Error Handling

### Graceful Failures

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

// Navigation errors
err = page.Navigate("https://invalid-url")
if errors.Is(err, &rod.NavigationError{}) {
    // Handle navigation failure
}
```

### Retries

```go
func ClickWithRetry(page *rod.Page, selector string, maxAttempts int) error {
    for i := 0; i < maxAttempts; i++ {
        element, err := page.Timeout(2*time.Second).Element(selector)
        if err != nil {
            time.Sleep(time.Second)
            continue
        }

        err = element.Click()
        if err == nil {
            return nil
        }

        time.Sleep(time.Second)
    }

    return fmt.Errorf("failed after %d attempts", maxAttempts)
}
```

## Testing Patterns

### Integration Tests

```go
func TestLogin(t *testing.T) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    page := browser.MustPage("https://example.com/login")

    // Test login flow
    page.MustElement("input[name='username']").MustInput("testuser")
    page.MustElement("input[name='password']").MustInput("testpass")
    page.MustElement("button[type='submit']").MustClick()

    page.MustWaitNavigation()

    // Assert
    assert.Contains(t, page.MustInfo().URL, "/dashboard")
    assert.Contains(t, page.MustElement("h1").MustText(), "Welcome")
}
```

### Test Helpers

```go
func NewTestBrowser(t *testing.T) (*rod.Browser, func()) {
    t.Helper()

    launcher := launcher.New().Headless(true).NoSandbox(true)
    url := launcher.MustLaunch()

    browser := rod.New().ControlURL(url).MustConnect()

    cleanup := func() {
        browser.MustClose()
        launcher.Cleanup()
    }

    return browser, cleanup
}

// Usage
func TestSomething(t *testing.T) {
    browser, cleanup := NewTestBrowser(t)
    defer cleanup()

    page := browser.MustPage("https://example.com")
    // ... test code
}
```

## Performance Optimization

### Resource Blocking

```go
// Block unnecessary resources
router := browser.HijackRequests()
defer router.MustStop()

router.MustAdd("*.css", func(ctx *rod.Hijack) {
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})
router.MustAdd("*.woff*", func(ctx *rod.Hijack) {
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})

go router.Run()
```

### Parallel Pages

```go
func ScrapeConcurrently(urls []string) []Result {
    browser := rod.New().MustConnect()
    defer browser.MustClose()

    var wg sync.WaitGroup
    results := make([]Result, len(urls))

    for i, url := range urls {
        wg.Add(1)
        go func(index int, u string) {
            defer wg.Done()

            page := browser.MustPage(u)
            defer page.MustClose()

            results[index] = extractData(page)
        }(i, url)
    }

    wg.Wait()
    return results
}
```

## Common Pitfalls

### 1. Not Waiting for Elements

```go
// WRONG: Element might not exist yet
element := page.MustElement("div.result")

// CORRECT: Wait for element
page.MustWaitElementsMoreThan("div.result", 0)
element := page.MustElement("div.result")
```

### 2. Forgetting Context Propagation

```go
// WRONG: No timeout
page.Navigate("https://slow-site.com")

// CORRECT: With timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
page.Context(ctx).Navigate("https://slow-site.com")
```

### 3. Leaking Pages

```go
// WRONG: Page never closed
func scrape(url string) {
    page := browser.MustPage(url)
    // ... do work
} // page leaks

// CORRECT: Always cleanup
func scrape(url string) {
    page := browser.MustPage(url)
    defer page.MustClose()
    // ... do work
}
```

### 4. Race Conditions with Events

```go
// WRONG: Subscribe after navigation
page.Navigate("https://example.com")
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    // May miss early requests
})

// CORRECT: Subscribe before
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    // Captures all requests
})
page.Navigate("https://example.com")
```

### 5. Not Handling JavaScript Errors

```go
// WRONG: JS error crashes evaluation
result := page.MustEval(`undefinedFunction()`)

// CORRECT: Catch JS errors
result, err := page.Eval(`undefinedFunction()`)
if err != nil {
    // Handle JS error
}
```

## Production Checklist

- [ ] Always use `defer browser.MustClose()`
- [ ] Set reasonable timeouts (don't rely on defaults)
- [ ] Handle element not found errors gracefully
- [ ] Clean up pages when done (`defer page.MustClose()`)
- [ ] Use incognito for isolated sessions
- [ ] Subscribe to events before navigation
- [ ] Block unnecessary resources for performance
- [ ] Use context for cancellation support
- [ ] Test with headless and headed modes
- [ ] Handle JavaScript errors in evaluations
- [ ] Implement retry logic for flaky operations
- [ ] Log CDP errors for debugging
- [ ] Consider rate limiting for web scraping

## References

For detailed API documentation and examples:

- `references/context7-comprehensive.md` - **Complete Rod patterns from Context7 (latest official docs)**
- `references/chromium-guide.md` - **Chrome/Chromium configuration, flags, debugging, and CDP protocol**
- `references/rod-api.md` - Rod API reference and BrowserNERD integration patterns
- `references/cdp-events.md` - Chrome DevTools Protocol events for fact transformation
- `references/selectors.md` - CSS and XPath selector patterns
- `references/examples.md` - Common automation scenarios
- `references/troubleshooting.md` - **Debug WebSocket errors, connection issues, and common problems**

**Quick Reference by Topic:**

| Topic | Reference File |
|-------|----------------|
| Latest Rod API patterns | `context7-comprehensive.md` |
| Chrome flags & configuration | `chromium-guide.md` |
| CDP events & monitoring | `cdp-events.md` |
| WebSocket/connection errors | `troubleshooting.md` |
| Element selectors | `selectors.md` |

**Encountering errors?** Check `troubleshooting.md` first - includes solutions for "websocket bad handshake: 404", Chrome launch failures, element not found, and more.
