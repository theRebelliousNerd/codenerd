# internal/browser - Browser Automation & DOM Reification

This package provides browser automation with DOM/React reification into Mangle facts, adapted from BrowserNERD for the Cortex Browser Physics Engine.

**Related Packages:**
- [internal/mangle](../mangle/CLAUDE.md) - Mangle engine receiving DOM facts
- [internal/shards/researcher](../shards/researcher/CLAUDE.md) - Web research using browser

## Architecture

Implements the Browser Physics Engine (Section 9.0):
- Chrome DevTools Protocol via go-rod/rod
- DOM elements reified as Mangle facts
- Honeypot detection using Mangle rules
- Session management for multi-tab research

## File Index

| File | Description |
|------|-------------|
| `session_manager.go` | Browser session management with Rod CDP automation. Exports `Session` (ID/TargetID/URL/Title/Status), `Config` (DebuggerURL/Headless/Viewport/Timeouts), `SessionManager` managing multiple sessions, and methods for navigation, screenshots, and DOM extraction via Chrome DevTools Protocol. |
| `honeypot.go` | Honeypot detection using Mangle rules for safe web scraping. Exports `DetectionResult` (ElementID/Selector/Reasons/Confidence), `Link` (Href/Text/IsHoneypot), `HoneypotDetector`, `AnalyzePage()` emitting DOM facts and querying `is_honeypot` rule, and confidence scoring. |

## Key Types

### Session
```go
type Session struct {
    ID         string
    TargetID   string
    URL        string
    Title      string
    Status     string
    CreatedAt  time.Time
    LastActive time.Time
}
```

### HoneypotDetector
```go
type HoneypotDetector struct {
    engine *mangle.Engine
}

func (d *HoneypotDetector) AnalyzePage(page *rod.Page) ([]DetectionResult, error)
```

## Mangle Integration

DOM elements are emitted as facts:
```datalog
dom_element("elem-123", /a, /visible).
element_style("elem-123", /display, "none").
is_honeypot("elem-123").  # Derived by rules
```

## Dependencies

- `github.com/go-rod/rod` - Chrome DevTools Protocol
- `internal/mangle` - Mangle engine for rule evaluation
- `internal/logging` - Structured logging

## Testing

```bash
go test ./internal/browser/...
```

---

**Remember: Push to GitHub regularly!**
