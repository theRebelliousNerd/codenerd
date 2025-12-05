# Rod Browser Automation Patterns

Comprehensive patterns for go-rod/rod library usage in BrowserNERD.

## Installation and Setup

```go
import (
    "github.com/go-rod/rod"
    "github.com/go-rod/rod/lib/launcher"
    "github.com/go-rod/rod/lib/proto"
    "github.com/go-rod/rod/lib/launcher/flags"
)
```

## Browser Lifecycle

### Launch Chrome Programmatically

```go
func LaunchChrome(ctx context.Context) (*rod.Browser, func(), error) {
    launcher := launcher.New().
        Bin("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe").
        Headless(false).
        Devtools(false).
        Set(flags.Flag("remote-debugging-port"), "9222").
        Set(flags.Flag("user-data-dir"), tempDir)

    url, err := launcher.Launch()
    if err != nil {
        return nil, nil, fmt.Errorf("launch: %w", err)
    }

    browser := rod.New().ControlURL(url).Context(ctx)
    if err := browser.Connect(); err != nil {
        launcher.Cleanup()
        return nil, nil, fmt.Errorf("connect: %w", err)
    }

    cleanup := func() {
        browser.Close()
        launcher.Cleanup()
    }

    return browser, cleanup, nil
}
```

### Attach to Existing Chrome

```go
func AttachToChrome(ctx context.Context, debugURL string) (*rod.Browser, error) {
    browser := rod.New().ControlURL(debugURL).Context(ctx)
    if err := browser.Connect(); err != nil {
        return nil, fmt.Errorf("connect to chrome: %w", err)
    }
    return browser, nil
}

// Usage
browser, err := AttachToChrome(ctx, "ws://localhost:9222")
```

### Browser Options

```go
browser := rod.New().
    ControlURL(url).
    Context(ctx).
    Timeout(30 * time.Second).  // Default timeout for all operations
    Sleeper(rod.DefaultSleeper). // Polling strategy
    Logger(customLogger)         // Custom logging
```

## Page Management

### Create New Page

```go
// Regular page
page, err := browser.Page(proto.TargetCreateTarget{
    URL: "https://example.com",
})

// Incognito page
page, err := browser.Incognito().Page(proto.TargetCreateTarget{
    URL: "https://example.com",
})
```

### Attach to Existing Target

```go
// List all targets
targets, err := browser.GetTargets()

// Filter for pages
for _, target := range targets {
    if target.Type == proto.TargetTargetInfoTypePage {
        page, err := browser.PageFromTarget(target.TargetID)
        // ...
    }
}
```

### Page Lifecycle

```go
// Navigate
err := page.Navigate("https://example.com")
err = page.WaitLoad() // Wait for load event

// Wait for specific conditions
err = page.WaitStable(500 * time.Millisecond) // No DOM changes

// Get page info
info, err := page.Info()
// info.URL, info.Title, info.TargetID

// Close page
err = page.Close()
```

## Element Interaction

### Selectors

```go
// CSS selector
element := page.MustElement("button#submit")

// XPath
element := page.MustElementX("//button[@id='submit']")

// Wait for element
element, err := page.Timeout(5*time.Second).Element("button#submit")

// Multiple elements
elements, err := page.Elements("div.card")
```

### Actions

```go
// Click
err = element.Click(proto.InputMouseButtonLeft)

// Type text
err = element.Input("hello world")

// Select option
err = element.Select([]string{"option-value"})

// Get text
text, err := element.Text()

// Get attribute
value, err := element.Attribute("href")

// Get property
value, err := element.Property("value")
```

### Waiting Strategies

```go
// Wait for element visible
element, err := page.WaitVisible("div#content")

// Wait for element stable (no position/size changes)
err = element.WaitStable(time.Second)

// Wait for element enabled
err = element.WaitEnabled()

// Custom wait condition
err = page.Wait(`() => document.querySelector('#data').textContent !== 'Loading...'`)
```

## JavaScript Evaluation

### Execute JavaScript

```go
// Simple evaluation
result, err := page.Evaluate(`1 + 2`)
// result.Value.Int() == 3

// With arguments
result, err := page.Evaluate(`(a, b) => a + b`, 1, 2)

// Access global objects
result, err := page.Evaluate(`window.location.href`)

// Mutation
_, err = page.Evaluate(`document.title = 'New Title'`)
```

### Element Context Evaluation

```go
element := page.MustElement("div#container")

// Evaluate in element context
result, err := element.Evaluate(`(element) => element.children.length`)

// Get computed style
result, err := element.Evaluate(`(el) => window.getComputedStyle(el).color`)
```

### Handling Results

```go
result, err := page.Evaluate(`({foo: 'bar', count: 42})`)

// Type assertions
obj := result.Value.Object()
foo := obj["foo"].String() // "bar"
count := obj["count"].Int() // 42

// Arrays
result, err := page.Evaluate(`[1, 2, 3]`)
arr := result.Value.Array()
for _, item := range arr {
    fmt.Println(item.Int())
}
```

## CDP Events

### Network Events

```go
// Request will be sent
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    fmt.Printf("Request: %s %s\n", e.Request.Method, e.Request.URL)
    fmt.Printf("Initiator: %s\n", e.Initiator.Type)
    fmt.Printf("RequestID: %s\n", e.RequestID)
})

// Response received
page.EachEvent(func(e *proto.NetworkResponseReceived) {
    fmt.Printf("Response: %d %s\n", e.Response.Status, e.Response.URL)
    fmt.Printf("Headers: %v\n", e.Response.Headers)
    fmt.Printf("MimeType: %s\n", e.Response.MimeType)
})

// Loading finished
page.EachEvent(func(e *proto.NetworkLoadingFinished) {
    fmt.Printf("Loaded: %s\n", e.RequestID)
    fmt.Printf("Timestamp: %v\n", e.Timestamp.Time())
})

// Loading failed
page.EachEvent(func(e *proto.NetworkLoadingFailed) {
    fmt.Printf("Failed: %s (%s)\n", e.RequestID, e.ErrorText)
    fmt.Printf("Canceled: %v\n", e.Canceled)
})
```

### Console Events

```go
page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
    level := string(e.Type) // "log", "error", "warning", "debug", "info"

    // Extract message arguments
    messages := make([]string, 0, len(e.Args))
    for _, arg := range e.Args {
        messages = append(messages, arg.Value.String())
    }

    fmt.Printf("[%s] %s\n", level, strings.Join(messages, " "))
    fmt.Printf("Timestamp: %v\n", e.Timestamp.Time())
    fmt.Printf("Stack: %v\n", e.StackTrace)
})
```

### DOM Events

```go
// Document updated
page.EachEvent(func(e *proto.DOMDocumentUpdated) {
    fmt.Println("DOM structure changed")
})

// Attribute modified
page.EachEvent(func(e *proto.DOMAttributeModified) {
    fmt.Printf("Attribute changed: %s=%s (node %d)\n",
        e.Name, e.Value, e.NodeID)
})

// Character data modified
page.EachEvent(func(e *proto.DOMCharacterDataModified) {
    fmt.Printf("Text changed: %s (node %d)\n",
        e.CharacterData, e.NodeID)
})
```

### Navigation Events

```go
// Frame navigated
page.EachEvent(func(e *proto.PageFrameNavigated) {
    fmt.Printf("Navigated: %s\n", e.Frame.URL)
    fmt.Printf("Frame ID: %s\n", e.Frame.ID)
})

// Frame started loading
page.EachEvent(func(e *proto.PageFrameStartedLoading) {
    fmt.Printf("Loading frame: %s\n", e.FrameID)
})

// Frame stopped loading
page.EachEvent(func(e *proto.PageFrameStoppedLoading) {
    fmt.Printf("Loaded frame: %s\n", e.FrameID)
})

// DOM content loaded
page.EachEvent(func(e *proto.PageDomContentEventFired) {
    fmt.Printf("DOMContentLoaded at %v\n", e.Timestamp.Time())
})

// Load event fired
page.EachEvent(func(e *proto.PageLoadEventFired) {
    fmt.Printf("Load complete at %v\n", e.Timestamp.Time())
})
```

### Dialog Events

```go
page.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
    fmt.Printf("Dialog: %s - %s\n", e.Type, e.Message)

    // Auto-accept
    page.Browser().Call(ctx, &proto.PageHandleJavaScriptDialog{
        Accept: true,
        PromptText: "",
    })
})
```

## Screenshot and PDF

### Capture Screenshot

```go
// Full page screenshot
data, err := page.Screenshot(true, &proto.PageCaptureScreenshot{
    Format: proto.PageCaptureScreenshotFormatPng,
})
err = os.WriteFile("screenshot.png", data, 0644)

// Element screenshot
element := page.MustElement("div#chart")
data, err := element.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)

// Viewport screenshot
data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{})
```

### Generate PDF

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
err = os.WriteFile("page.pdf", pdf, 0644)
```

## Cookies and Storage

### Cookies

```go
// Get all cookies
cookies, err := page.Cookies([]string{})

// Get cookies for specific URLs
cookies, err := page.Cookies([]string{"https://example.com"})

// Set cookies
err = page.SetCookies([]*proto.NetworkCookie{
    {
        Name:   "session_id",
        Value:  "abc123",
        Domain: "example.com",
        Path:   "/",
        Secure: true,
        HTTPOnly: true,
    },
})

// Delete cookies
err = page.Browser().Call(ctx, &proto.NetworkDeleteCookies{
    Name:   "session_id",
    Domain: "example.com",
})
```

### LocalStorage and SessionStorage

```go
// Get localStorage
result, err := page.Evaluate(`
    JSON.stringify(Object.keys(localStorage).reduce((acc, key) => {
        acc[key] = localStorage.getItem(key);
        return acc;
    }, {}))
`)
storage := result.Value.Object()

// Set localStorage
_, err = page.Evaluate(`localStorage.setItem('key', 'value')`)

// Clear localStorage
_, err = page.Evaluate(`localStorage.clear()`)

// SessionStorage (same API)
_, err = page.Evaluate(`sessionStorage.setItem('key', 'value')`)
```

## Error Handling

### Timeouts

```go
// Per-operation timeout
page := page.Timeout(5 * time.Second)
element, err := page.Element("button#submit")
if errors.Is(err, context.DeadlineExceeded) {
    // Handle timeout
}

// Reset timeout
page = page.CancelTimeout()
```

### Error Types

```go
// Element not found
element, err := page.Element("div#nonexistent")
if errors.Is(err, &rod.ElementNotFoundError{}) {
    // Handle not found
}

// Navigation error
err = page.Navigate("https://invalid-url")
if errors.Is(err, &rod.NavigationError{}) {
    // Handle navigation failure
}

// Evaluation error
_, err = page.Evaluate(`invalid javascript`)
if errors.Is(err, &rod.EvalError{}) {
    // Handle JS error
}
```

## Best Practices

### Resource Cleanup

```go
func ProcessPage(ctx context.Context, browser *rod.Browser, url string) error {
    page, err := browser.Page(proto.TargetCreateTarget{URL: url})
    if err != nil {
        return err
    }
    defer page.Close() // Always clean up

    // Work with page...
    return nil
}
```

### Context Propagation

```go
// Use context for cancellation
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

page := browser.Context(ctx).MustPage("https://example.com")

// All operations respect context
err := page.Navigate("https://slow-site.com") // Will timeout after 30s
```

### Concurrent Page Operations

```go
func ProcessURLs(browser *rod.Browser, urls []string) error {
    var wg sync.WaitGroup
    errors := make(chan error, len(urls))

    for _, url := range urls {
        wg.Add(1)
        go func(url string) {
            defer wg.Done()

            page, err := browser.Page(proto.TargetCreateTarget{URL: url})
            if err != nil {
                errors <- err
                return
            }
            defer page.Close()

            // Process page...
        }(url)
    }

    wg.Wait()
    close(errors)

    // Check for errors
    for err := range errors {
        if err != nil {
            return err
        }
    }

    return nil
}
```

### Event Handler Cleanup

```go
// Cancel event handler when done
cancel := page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    // Handle event
})
defer cancel() // Clean up goroutine
```
