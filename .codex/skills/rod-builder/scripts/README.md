# Rod Builder Scripts

Production-ready helper scripts for common Rod automation tasks.

## Available Scripts

### 1. chrome_launcher.go

Launches Chrome with remote debugging enabled for Rod to connect to.

**Usage:**
```bash
go run chrome_launcher.go --port 9222 --headless=false
```

**Options:**
- `--port` - Remote debugging port (default: 9222)
- `--headless` - Run in headless mode (default: false)
- `--user-data-dir` - Chrome user data directory (empty = temp)
- `--no-sandbox` - Disable sandbox for Docker/CI (default: false)

**Example:**
```bash
# Start Chrome for development
go run chrome_launcher.go

# Start headless Chrome for CI
go run chrome_launcher.go --headless=true --no-sandbox=true
```

### 2. scraper_template.go

Customizable web scraper with best practices built-in.

**Usage:**
```bash
go run scraper_template.go --url "https://example.com" --output data.json
```

**Options:**
- `--url` - URL to scrape (required)
- `--output` - Output file path (default: output.json)
- `--headless` - Run in headless mode (default: true)
- `--timeout` - Page load timeout (default: 30s)
- `--wait-for` - CSS selector to wait for before scraping
- `--selector` - CSS selector for items to scrape

**Examples:**
```bash
# Basic scraping
go run scraper_template.go --url "https://news.ycombinator.com" --selector ".athing"

# Wait for dynamic content
go run scraper_template.go --url "https://example.com" --wait-for ".loaded" --timeout 60s

# Scrape with custom output
go run scraper_template.go --url "https://example.com" --output results/data.json
```

### 3. session_manager.go

Reusable session management with persistence and cleanup.

**Usage:**
Copy this file into your project and customize:

```go
import "yourproject/session_manager"

browser := rod.New().MustConnect()
defer browser.MustClose()

sm := session_manager.NewSessionManager(browser, "sessions.json")
defer sm.Shutdown()

// Create session
session, err := sm.CreateSession("https://example.com")

// Fork session with cookies
forked, err := sm.ForkSession(session.ID, "")

// Cleanup stale sessions
removed := sm.CleanupStale(1 * time.Hour)
```

**Features:**
- Session persistence to disk
- Cookie management
- Session forking (copy cookies to new session)
- Automatic stale session cleanup
- Thread-safe operations

## Installation

All scripts require go-rod:

```bash
go get github.com/go-rod/rod
go get github.com/google/uuid # for session_manager
```

## Customization Tips

### chrome_launcher.go
- Add more Chrome flags as needed (e.g., `--disable-gpu`, `--window-size`)
- Implement auto-restart if Chrome crashes
- Add health check endpoint

### scraper_template.go
- Customize `scrape()` function for specific data extraction
- Add pagination handling
- Implement rate limiting
- Add retry logic with backoff

### session_manager.go
- Add session tags/metadata for categorization
- Implement session authentication helpers
- Add screenshot capture per session
- Implement session replay functionality

## Best Practices

1. **Always cleanup resources**: Use `defer` to ensure browser/page closure
2. **Handle errors gracefully**: Don't panic, return errors
3. **Set timeouts**: Prevent indefinite hangs
4. **Use context**: Enable cancellation propagation
5. **Log operations**: Track what the script is doing
6. **Persist state**: Save progress for recovery

## Testing

Run scripts with test URLs:

```bash
# Test scraper
go run scraper_template.go --url "https://example.com" --headless=false

# Test session manager
go run session_manager.go
```

## Production Deployment

When deploying to production:

1. **Use environment variables** for configuration
2. **Add structured logging** (e.g., logrus, zap)
3. **Implement metrics** (Prometheus, StatsD)
4. **Add health checks** for monitoring
5. **Handle signals** for graceful shutdown
6. **Use connection pooling** for efficiency
7. **Add rate limiting** to respect target sites
