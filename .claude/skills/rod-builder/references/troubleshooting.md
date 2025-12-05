# Rod Troubleshooting Guide

Comprehensive solutions for common Rod browser automation issues.

## WebSocket Connection Errors

### Error: "websocket bad handshake: 404 Not Found"

This error means Rod cannot connect to Chrome's remote debugging port.

#### Cause 1: Chrome Not Started with Debugging

**Symptom:**
```
Error: websocket bad handshake: 404 Not Found
```

**Solution:**
Chrome must be started with `--remote-debugging-port` flag:

```bash
# Windows
"C:\Program Files\Google\Chrome\Application\chrome.exe" --remote-debugging-port=9222 --user-data-dir=C:\temp\chrome-debug

# macOS
/Applications/Google\ Chrome.app/Contents/MacOS/Google\ Chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug

# Linux
google-chrome --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-debug
```

**Verify Chrome is Listening:**
```bash
# Check if Chrome is accepting connections
curl http://localhost:9222/json

# Should return JSON like:
# [{"description":"","devtoolsFrontendUrl":"/devtools/inspector.html?ws=localhost:9222/devtools/page/...
```

If you get connection refused or 404, Chrome isn't listening.

#### Cause 2: Wrong Control URL Format

**Wrong:**
```go
// Missing protocol
browser := rod.New().ControlURL("localhost:9222").MustConnect()

// Wrong protocol
browser := rod.New().ControlURL("http://localhost:9222").MustConnect()
```

**Correct:**
```go
// Use ws:// for WebSocket
browser := rod.New().ControlURL("ws://localhost:9222").MustConnect()
```

**For DevTools URL from /json endpoint:**
```go
// If /json returns: ws=localhost:9222/devtools/page/ABC123
fullURL := "ws://localhost:9222/devtools/page/ABC123"
browser := rod.New().ControlURL(fullURL).MustConnect()
```

#### Cause 3: Port Already in Use

**Symptom:**
```
Error: websocket bad handshake: 404 Not Found
# OR
Error: listen tcp :9222: bind: address already in use
```

**Check What's Using the Port:**
```bash
# Windows
netstat -ano | findstr :9222

# macOS/Linux
lsof -i :9222
```

**Solutions:**
```bash
# Option 1: Kill existing Chrome
taskkill /F /IM chrome.exe  # Windows
killall "Google Chrome"      # macOS
pkill chrome                 # Linux

# Option 2: Use a different port
chrome --remote-debugging-port=9223
```

```go
// Then connect to the new port
browser := rod.New().ControlURL("ws://localhost:9223").MustConnect()
```

#### Cause 4: Chrome Instance from Launcher Not Ready

**Problem:**
```go
launcher := launcher.New()
url, _ := launcher.Launch()

// Connect immediately - Chrome might not be ready
browser := rod.New().ControlURL(url).MustConnect() // May fail!
```

**Solution - Add Retry:**
```go
func ConnectWithRetry(url string, maxAttempts int) (*rod.Browser, error) {
    var lastErr error

    for i := 0; i < maxAttempts; i++ {
        browser := rod.New().ControlURL(url)
        err := browser.Connect()
        if err == nil {
            return browser, nil
        }

        lastErr = err
        time.Sleep(500 * time.Millisecond)
    }

    return nil, fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}

// Usage
url, _ := launcher.New().Launch()
browser, err := ConnectWithRetry(url, 10)
```

### Error: "websocket: close 1006 (abnormal closure)"

**Cause:** Chrome crashed or was closed while Rod was connected.

**Solutions:**
```go
// 1. Handle disconnections gracefully
browser := rod.New().ControlURL(url)
err := browser.Connect()
if err != nil {
    if strings.Contains(err.Error(), "1006") {
        log.Println("Chrome disconnected, restarting...")
        // Restart Chrome and reconnect
    }
}

// 2. Monitor browser process
go func() {
    <-browser.Context().Done()
    log.Println("Browser disconnected")
    // Handle cleanup
}()

// 3. Keep Chrome alive
launcher := launcher.New().Leakless(false) // Don't auto-kill
```

## Chrome Launch Issues

### Error: "chrome not found"

**Symptoms:**
```
Error: cannot find chrome executable
```

**Solutions:**

**Option 1: Specify Chrome Path**
```go
launcher := launcher.New().
    Bin("C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"). // Windows
    // Bin("/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"). // macOS
    // Bin("/usr/bin/google-chrome"). // Linux
    MustLaunch()
```

**Option 2: Use Environment Variable**
```bash
export CHROME_BIN="/path/to/chrome"
```

**Option 3: Install Chrome in Standard Location**
```bash
# Use launcher.LookPath() to see where it's searching
import "github.com/go-rod/rod/lib/launcher"

path := launcher.NewBrowser().ExecPath()
fmt.Println("Chrome path:", path)
```

### Chrome Crashes on Launch

**Symptom:**
Chrome starts but immediately crashes.

**Common Causes & Solutions:**

**1. Missing --no-sandbox (Docker/CI)**
```go
launcher := launcher.New().
    NoSandbox(true).  // Required in Docker
    MustLaunch()
```

**2. Insufficient Permissions**
```bash
# Give Chrome execute permissions
chmod +x /path/to/chrome
```

**3. Missing Dependencies (Linux)**
```bash
# Install required libraries
sudo apt-get install -y \
    libnss3 \
    libatk-bridge2.0-0 \
    libdrm2 \
    libxkbcommon0 \
    libgbm1 \
    libasound2
```

**4. Display Issues (Headless)**
```go
// Force headless mode
launcher := launcher.New().
    Headless(true).
    Set("disable-gpu", ""). // Disable GPU
    MustLaunch()
```

## Element Selection Issues

### Error: "element not found"

**Symptoms:**
```go
element := page.MustElement("#button") // Panics: element not found
```

**Debugging Steps:**

**1. Verify Element Exists**
```go
// Use non-Must version to get error
element, err := page.Element("#button")
if err != nil {
    // Take screenshot to see current state
    page.MustScreenshot("debug.png")

    // Print page HTML
    html := page.MustHTML()
    fmt.Println("Page HTML:", html)

    return err
}
```

**2. Wait for Element**
```go
// Wait for element to appear
page.MustWaitElementsMoreThan("#button", 0)
element := page.MustElement("#button")

// Or with timeout
element, err := page.Timeout(10 * time.Second).Element("#button")
```

**3. Check Selector**
```go
// Test selector in browser DevTools first
// Press F12 → Console → Run:
// document.querySelector("#button")

// Try different selectors
selectors := []string{
    "#button",
    "button#button",
    "[data-testid='button']",
    "//button[@id='button']", // XPath
}

for _, sel := range selectors {
    if strings.HasPrefix(sel, "//") {
        if el, err := page.ElementX(sel); err == nil {
            fmt.Printf("Found with XPath: %s\n", sel)
            break
        }
    } else {
        if el, err := page.Element(sel); err == nil {
            fmt.Printf("Found with CSS: %s\n", sel)
            break
        }
    }
}
```

**4. Element in iframe**
```go
// Find iframe first
iframe := page.MustElement("iframe")

// Get iframe's page
iframePage := iframe.MustFrame()

// Now search in iframe
element := iframePage.MustElement("#button")
```

**5. Dynamic Content**
```go
// Wait for page to stabilize
page.MustWaitStable(time.Second)

// Wait for AJAX to complete
page.MustWait(`() => document.readyState === 'complete'`)
page.MustWait(`() => !document.querySelector('.loading')`)
```

## Page Navigation Issues

### Error: "navigation timeout"

**Symptoms:**
```go
page.MustNavigate("https://slow-site.com") // Times out
```

**Solutions:**

**1. Increase Timeout**
```go
// Set longer timeout
page = page.Timeout(60 * time.Second)
page.MustNavigate("https://slow-site.com")

// Or with context
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()

page = page.Context(ctx)
page.Navigate("https://slow-site.com")
```

**2. Don't Wait for Full Load**
```go
// Navigate without waiting
page.Navigate("https://slow-site.com")

// Wait for specific element instead
page.MustWaitElementsMoreThan("#content", 0)
```

**3. Handle Redirects**
```go
page.MustNavigate("https://site.com/redirect")
page.MustWaitNavigation() // Wait for redirect to complete
```

### Page Blank or Not Rendering

**Symptoms:**
Page loads but appears empty in screenshots.

**Solutions:**

**1. Wait for Content**
```go
page.MustNavigate("https://example.com")
page.MustWaitLoad() // Wait for load event

// Wait for specific content
page.MustWait(`() => document.querySelector('#content') !== null`)
```

**2. Enable JavaScript**
```go
// Ensure JavaScript isn't disabled
launcher := launcher.New().
    Set("disable-javascript", "false").
    MustLaunch()
```

**3. Check for Console Errors**
```go
go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
    if e.Type == proto.RuntimeConsoleAPICalledTypeError {
        fmt.Printf("JS Error: %v\n", e.Args)
    }
})()

page.MustNavigate("https://example.com")
time.Sleep(2 * time.Second) // Give time for errors to appear
```

## Performance Issues

### Rod is Slow

**Symptoms:**
Operations take much longer than expected.

**Solutions:**

**1. Block Unnecessary Resources**
```go
router := browser.HijackRequests()
defer router.MustStop()

// Block images, CSS, fonts
router.MustAdd("*.png", func(ctx *rod.Hijack) {
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})
router.MustAdd("*.css", func(ctx *rod.Hijack) {
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})
router.MustAdd("*.woff*", func(ctx *rod.Hijack) {
    ctx.Response.Fail(proto.NetworkErrorReasonBlockedByClient)
})

go router.Run()
```

**2. Use Headless Mode**
```go
launcher := launcher.New().Headless(true).MustLaunch()
// Headless is ~30% faster
```

**3. Reuse Browser Instances**
```go
// SLOW: Create new browser each time
func scrape(url string) {
    browser := rod.New().MustConnect()
    defer browser.MustClose()
    // ...
}

// FAST: Reuse browser
browser := rod.New().MustConnect()
defer browser.MustClose()

for _, url := range urls {
    page := browser.MustPage(url)
    // ... scrape
    page.MustClose()
}
```

**4. Parallelize Operations**
```go
var wg sync.WaitGroup

for _, url := range urls {
    wg.Add(1)
    go func(u string) {
        defer wg.Done()
        page := browser.MustPage(u)
        defer page.MustClose()
        // ... scrape
    }(url)
}

wg.Wait()
```

## Debugging Tips

### Enable Debug Logging

```go
// Rod debug output
os.Setenv("rod", "show")

// More verbose
os.Setenv("rod", "trace")

// CDP protocol messages
os.Setenv("rod", "trace,cdp")
```

### Inspect What Rod Sees

```go
// Take screenshot
page.MustScreenshot("debug.png")

// Get HTML
html := page.MustHTML()
os.WriteFile("debug.html", []byte(html), 0644)

// Get console logs
go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
    fmt.Printf("[%s] %v\n", e.Type, e.Args)
})()

// Monitor network
go page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    fmt.Printf("Request: %s %s\n", e.Request.Method, e.Request.URL)
})()
```

### Non-Headless Debugging

```go
// See what's happening
launcher := launcher.New().
    Headless(false).   // Show browser window
    Devtools(true).    // Open DevTools
    MustLaunch()

// Slow down actions
rod.SlowMotion(500 * time.Millisecond)
```

### Keep Browser Open After Error

```go
launcher := launcher.New().Leakless(false) // Don't auto-close
url, _ := launcher.Launch()
browser := rod.New().ControlURL(url).MustConnect()

// On error, browser stays open for inspection
defer func() {
    if r := recover(); r != nil {
        fmt.Println("Error occurred, browser left open")
        fmt.Println("Press Enter to close...")
        bufio.NewReader(os.Stdin).ReadString('\n')
        browser.MustClose()
        launcher.Cleanup()
    }
}()
```

## Common Gotchas

### 1. Must* Methods Panic

```go
// DANGEROUS: Panics on error
element := page.MustElement("#button")

// SAFE: Returns error
element, err := page.Element("#button")
if err != nil {
    // Handle error
}
```

### 2. Forgetting to Close Resources

```go
// MEMORY LEAK
for _, url := range urls {
    page := browser.MustPage(url)
    // ... work
} // Pages never closed!

// CORRECT
for _, url := range urls {
    page := browser.MustPage(url)
    // ... work
    page.MustClose() // Clean up
}
```

### 3. Race Conditions with Events

```go
// WRONG: Events missed
page.Navigate("https://example.com")
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    // May miss early requests
})

// CORRECT: Subscribe before navigation
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    // Captures all requests
})
page.Navigate("https://example.com")
```

### 4. Context Cancellation Not Propagated

```go
// WRONG: Context not used
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

browser := rod.New().MustConnect()
page := browser.MustPage("https://slow-site.com") // Ignores context!

// CORRECT: Propagate context
browser := rod.New().Context(ctx).MustConnect()
page := browser.MustPage("https://slow-site.com") // Respects timeout
```

## Getting Help

When asking for help, include:

1. **Full error message**
2. **Rod version**: `go list -m github.com/go-rod/rod`
3. **Chrome version**: Check in browser or launcher output
4. **Minimal reproduction code**
5. **What you've already tried**
6. **Debug logs**: `os.Setenv("rod", "trace")`

**Useful Resources:**
- Rod GitHub Issues: https://github.com/go-rod/rod/issues
- Rod Documentation: https://go-rod.github.io
- CDP Protocol Docs: https://chromedevtools.github.io/devtools-protocol/
