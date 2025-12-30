# Chromium Configuration Guide for Rod

Comprehensive guide to Chrome/Chromium browser configuration, flags, and debugging.

## Chrome DevTools Protocol (CDP) Overview

Rod controls browsers via the Chrome DevTools Protocol (CDP), a WebSocket-based API that provides:

- **DOM inspection and manipulation**
- **Network monitoring and interception**
- **JavaScript debugging**
- **Performance profiling**
- **Accessibility tree access**
- **Security and storage management**

### Protocol Architecture

```
Rod Application (Go)
        |
        | WebSocket (ws://localhost:9222)
        v
Chrome DevTools Protocol Server
        |
        v
Browser Process
    |-- Page 1 (Target)
    |-- Page 2 (Target)
    |-- Service Worker (Target)
    |-- Extension Background (Target)
```

## Launcher Configuration

### Essential Flags

```go
import "github.com/go-rod/rod/lib/launcher"

launcher := launcher.New().
    // === Browser Mode ===
    Headless(true).               // Run without UI
    Devtools(false).              // Don't auto-open DevTools

    // === Debugging ===
    Set("remote-debugging-port", "9222").  // CDP port
    Set("remote-debugging-address", "0.0.0.0").  // Allow remote connections

    // === Profile & Data ===
    Set("user-data-dir", "/tmp/chrome-profile").  // Persistent profile
    Set("disk-cache-dir", "/tmp/chrome-cache").   // Separate cache
    Set("incognito", "").                         // Start in incognito

    // === Security (for Docker/CI) ===
    NoSandbox(true).              // Required in Docker
    Set("disable-setuid-sandbox", "").

    // === Performance ===
    Set("disable-gpu", "").                // Disable GPU acceleration
    Set("disable-dev-shm-usage", "").      // Use /tmp instead of /dev/shm
    Set("disable-software-rasterizer", "").
    Set("single-process", "").             // Single process mode (testing only)

    // === Features ===
    Set("disable-extensions", "").         // No extensions
    Set("disable-background-networking", "").
    Set("disable-sync", "").               // No Google sync
    Set("disable-translate", "").          // No translate popup
    Set("disable-popup-blocking", "").     // Allow popups
    Set("disable-infobars", "").           // No info bars

    // === Network ===
    Set("proxy-server", "http://proxy:8080").  // HTTP proxy
    Set("proxy-bypass-list", "localhost,127.0.0.1").
    Set("ignore-certificate-errors", "").  // Skip SSL verification

    // === Window ===
    Set("window-size", "1920,1080").       // Initial window size
    Set("start-maximized", "").            // Start maximized
    Set("start-fullscreen", "").           // Start fullscreen
    Set("kiosk", "").                      // Kiosk mode

    // === Custom Binary ===
    Bin("/path/to/chrome")                 // Custom Chrome path
```

### Platform-Specific Paths

```go
func getChromePath() string {
    switch runtime.GOOS {
    case "windows":
        paths := []string{
            `C:\Program Files\Google\Chrome\Application\chrome.exe`,
            `C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
            os.Getenv("LOCALAPPDATA") + `\Google\Chrome\Application\chrome.exe`,
        }
        for _, p := range paths {
            if _, err := os.Stat(p); err == nil {
                return p
            }
        }
    case "darwin":
        return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    case "linux":
        paths := []string{
            "/usr/bin/google-chrome",
            "/usr/bin/google-chrome-stable",
            "/usr/bin/chromium",
            "/usr/bin/chromium-browser",
            "/snap/bin/chromium",
        }
        for _, p := range paths {
            if _, err := os.Stat(p); err == nil {
                return p
            }
        }
    }
    return ""
}
```

## Environment Configurations

### Docker Configuration

```dockerfile
FROM golang:1.21-alpine

# Install Chromium and dependencies
RUN apk add --no-cache \
    chromium \
    nss \
    freetype \
    harfbuzz \
    ca-certificates \
    ttf-freefont

# Set Chrome path
ENV CHROME_BIN=/usr/bin/chromium-browser

# Run as non-root
RUN adduser -D chrome
USER chrome

WORKDIR /app
```

```go
// Docker-optimized launcher
func DockerLauncher() *launcher.Launcher {
    return launcher.New().
        Bin(os.Getenv("CHROME_BIN")).
        Headless(true).
        NoSandbox(true).
        Set("disable-dev-shm-usage", "").
        Set("disable-gpu", "").
        Set("single-process", "")  // Sometimes needed in containers
}
```

### CI/CD Configuration (GitHub Actions)

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Chrome
        uses: browser-actions/setup-chrome@latest
        with:
          chrome-version: stable

      - name: Run Tests
        env:
          CHROME_BIN: /usr/bin/google-chrome
        run: go test -v ./...
```

### Headless vs Headed Modes

```go
// Headless: No UI, faster, for CI/servers
launcher := launcher.New().Headless(true)

// Headed: Shows browser, for debugging
launcher := launcher.New().Headless(false).Devtools(true)

// New Headless Mode (Chrome 112+) - better compatibility
launcher := launcher.New().Set("headless", "new")
```

## Chrome Flags Reference

### Performance Flags

| Flag | Purpose |
|------|---------|
| `--disable-gpu` | Disable GPU hardware acceleration |
| `--disable-software-rasterizer` | Disable software rasterization |
| `--disable-dev-shm-usage` | Write shared memory to /tmp |
| `--single-process` | Run in single process (testing only) |
| `--no-zygote` | Disable zygote process |
| `--renderer-process-limit=1` | Limit renderer processes |
| `--memory-pressure-off` | Disable memory pressure handling |

### Security Flags

| Flag | Purpose |
|------|---------|
| `--no-sandbox` | Disable sandboxing (Docker) |
| `--disable-setuid-sandbox` | Disable setuid sandbox |
| `--ignore-certificate-errors` | Skip SSL verification |
| `--allow-insecure-localhost` | Allow insecure localhost |
| `--disable-web-security` | Disable web security (CORS) |
| `--disable-features=IsolateOrigins,site-per-process` | Disable site isolation |

### UI/UX Flags

| Flag | Purpose |
|------|---------|
| `--disable-infobars` | Hide info bars |
| `--disable-popup-blocking` | Allow popups |
| `--disable-default-apps` | No default apps |
| `--disable-extensions` | No extensions |
| `--disable-translate` | No translate prompts |
| `--disable-background-timer-throttling` | Don't throttle background timers |
| `--disable-renderer-backgrounding` | Keep renderers active |
| `--force-color-profile=srgb` | Force sRGB color profile |

### Network Flags

| Flag | Purpose |
|------|---------|
| `--proxy-server=host:port` | Set proxy server |
| `--proxy-bypass-list=hosts` | Bypass proxy for hosts |
| `--host-resolver-rules=rules` | Custom DNS resolution |
| `--disable-background-networking` | No background network |
| `--enable-features=NetworkService` | Use network service |

## Debugging Chrome/Rod Issues

### Enable Verbose Logging

```go
import "os"

// Rod debug logging
os.Setenv("rod", "show")        // Basic logging
os.Setenv("rod", "trace")       // Action tracing
os.Setenv("rod", "trace,cdp")   // CDP protocol messages

// Chrome logging
launcher := launcher.New().
    Set("enable-logging", "").
    Set("v", "1").                    // Verbosity level
    Set("log-level", "0")             // 0=INFO, 1=WARNING, 2=ERROR
```

### Remote Debugging

```go
// Start Chrome with remote debugging
launcher := launcher.New().
    Headless(false).
    Set("remote-debugging-port", "9222").
    Set("remote-debugging-address", "0.0.0.0")

// Chrome DevTools URL: chrome://inspect
// Or: http://localhost:9222

// List all debuggable targets
// curl http://localhost:9222/json
```

### Inspect Running Chrome

```bash
# List targets
curl http://localhost:9222/json

# Get browser version
curl http://localhost:9222/json/version

# Get protocol definition
curl http://localhost:9222/json/protocol
```

### Common Debug Scenarios

```go
// Debug element not found
func DebugElement(page *rod.Page, selector string) {
    // Take screenshot
    page.MustScreenshot("debug.png")

    // Get page source
    html, _ := page.HTML()
    os.WriteFile("debug.html", []byte(html), 0644)

    // Check if element exists anywhere
    elements, _ := page.Elements(selector)
    fmt.Printf("Found %d elements matching '%s'\n", len(elements), selector)

    // Try in all iframes
    frames, _ := page.Frames()
    for i, frame := range frames {
        if el, err := frame.Element(selector); err == nil {
            fmt.Printf("Found in iframe %d\n", i)
            _ = el
        }
    }
}

// Debug slow pages
func DebugPerformance(page *rod.Page) {
    metrics, _ := proto.PerformanceGetMetrics{}.Call(page)
    for _, m := range metrics.Metrics {
        fmt.Printf("%s: %.2f\n", m.Name, m.Value)
    }
}
```

## User Data Directory

### Understanding Profile Structure

```
user-data-dir/
|-- Default/                    # Default profile
|   |-- Bookmarks              # Bookmarks
|   |-- Cookies                # Cookies database
|   |-- History                # Browsing history
|   |-- Login Data             # Saved passwords
|   |-- Preferences            # Chrome settings
|   |-- Web Data               # Autofill data
|   |-- Cache/                 # Page cache
|   |-- Local Storage/         # localStorage
|   |-- Session Storage/       # sessionStorage
|   |-- IndexedDB/             # IndexedDB databases
|
|-- First Run                  # First run marker
|-- Local State                # Browser state
|-- Safe Browsing/             # Safe browsing data
```

### Profile Management

```go
// Use existing profile (persists cookies, localStorage)
launcher := launcher.New().
    Set("user-data-dir", "/path/to/profile")

// Use specific profile within user data dir
launcher := launcher.New().
    Set("user-data-dir", "/path/to/user-data").
    Set("profile-directory", "Profile 1")

// Clean temporary profile (auto-deleted)
launcher := launcher.New()  // Rod creates temp dir

// Persistent but isolated profile
func NewIsolatedProfile() string {
    dir, _ := os.MkdirTemp("", "chrome-profile-")
    return dir
}
```

## Memory Management

### Chrome Memory Optimization

```go
launcher := launcher.New().
    // Limit renderer processes
    Set("renderer-process-limit", "2").

    // Reduce memory usage
    Set("disable-features", "TranslateUI").
    Set("disable-background-networking", "").
    Set("disable-sync", "").

    // Aggressive garbage collection
    Set("js-flags", "--expose-gc")
```

### Monitor Memory Usage

```go
// Get memory info
func GetMemoryInfo(page *rod.Page) {
    // JavaScript heap
    result := page.MustEval(`() => ({
        used: performance.memory.usedJSHeapSize,
        total: performance.memory.totalJSHeapSize,
        limit: performance.memory.jsHeapSizeLimit
    })`)
    fmt.Printf("JS Heap: %v\n", result.Value)

    // Force garbage collection (if --expose-gc)
    page.MustEval(`() => { if (window.gc) gc(); }`)
}
```

## CDP Protocol Reference

### Commonly Used Domains

| Domain | Purpose |
|--------|---------|
| `Page` | Page lifecycle, navigation, screenshots |
| `DOM` | Document structure, queries |
| `Network` | Requests, responses, cookies |
| `Runtime` | JavaScript evaluation, console |
| `Input` | Keyboard, mouse, touch events |
| `Emulation` | Device emulation, geolocation |
| `Performance` | Metrics, tracing |
| `Security` | Certificate handling |
| `Storage` | Cache, IndexedDB, localStorage |
| `Target` | Browser targets (pages, workers) |

### Direct CDP Calls

```go
import "github.com/go-rod/rod/lib/proto"

// Enable a domain
page.EnableDomain(&proto.NetworkEnable{})

// Call any CDP method
result, err := page.Call(
    context.Background(),
    "",  // Session ID (empty for page)
    "Network.setCacheDisabled",
    map[string]bool{"cacheDisabled": true},
)

// Type-safe CDP calls
err := proto.EmulationSetDeviceMetricsOverride{
    Width:             1920,
    Height:            1080,
    DeviceScaleFactor: 2,
    Mobile:            false,
}.Call(page)
```

## Browser Extensions

### Load Extension

```go
launcher := launcher.New().
    Set("disable-extensions-except", "/path/to/extension").
    Set("load-extension", "/path/to/extension")
```

### Extension Manifest (manifest.json)

```json
{
    "manifest_version": 3,
    "name": "Rod Extension",
    "version": "1.0",
    "permissions": ["activeTab", "storage"],
    "background": {
        "service_worker": "background.js"
    },
    "content_scripts": [{
        "matches": ["<all_urls>"],
        "js": ["content.js"]
    }]
}
```

## Mobile Emulation

```go
// Emulate mobile device
err := proto.EmulationSetDeviceMetricsOverride{
    Width:             375,
    Height:            812,
    DeviceScaleFactor: 3,
    Mobile:            true,
}.Call(page)

// Set user agent
err = proto.EmulationSetUserAgentOverride{
    UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15",
}.Call(page)

// Emulate touch events
err = proto.EmulationSetTouchEmulationEnabled{
    Enabled: true,
}.Call(page)
```

## Geolocation Spoofing

```go
err := proto.EmulationSetGeolocationOverride{
    Latitude:  37.7749,
    Longitude: -122.4194,
    Accuracy:  100,
}.Call(page)
```

## Timezone Override

```go
err := proto.EmulationSetTimezoneOverride{
    TimezoneID: "America/New_York",
}.Call(page)
```

## Best Practices

### Production Checklist

- [ ] Use headless mode in production
- [ ] Set appropriate timeouts
- [ ] Configure resource limits
- [ ] Use user data dir for persistent sessions
- [ ] Enable logging for debugging
- [ ] Handle Chrome crashes gracefully
- [ ] Monitor memory usage
- [ ] Clean up zombie processes
- [ ] Use incognito for isolated sessions
- [ ] Block unnecessary resources

### Security Considerations

- Avoid `--disable-web-security` in production
- Use `--no-sandbox` only in trusted containers
- Validate all input before passing to browser
- Be cautious with `--ignore-certificate-errors`
- Isolate browser processes from sensitive data
