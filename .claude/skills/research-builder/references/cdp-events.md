# Chrome DevTools Protocol Events Reference

Comprehensive guide to CDP events for BrowserNERD fact transformation.

## Overview

Chrome DevTools Protocol (CDP) provides event-driven access to browser internals:
- **Network**: HTTP requests, responses, WebSocket frames
- **Runtime**: Console messages, exceptions, execution contexts
- **Page**: Navigation, lifecycle events, frames
- **DOM**: Document structure, mutations, attributes
- **Performance**: Metrics, traces, memory usage

## Network Domain

### NetworkRequestWillBeSent

Fired when a request is about to be sent.

```go
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    // Event fields
    requestID := e.RequestID.String()         // Unique request identifier
    url := e.Request.URL                      // Full URL
    method := e.Request.Method                // GET, POST, etc.
    headers := e.Request.Headers              // map[string]interface{}
    postData := e.Request.PostData            // Request body (if POST)
    initiator := e.Initiator                  // Script, parser, etc.
    timestamp := e.Timestamp.Time()           // When sent

    // Initiator types
    switch e.Initiator.Type {
    case proto.NetworkInitiatorTypeScript:
        // JavaScript fetch/XHR
    case proto.NetworkInitiatorTypeParser:
        // HTML parser (img, link, script tags)
    case proto.NetworkInitiatorTypeOther:
        // User navigation, redirect
    }

    // Transform to Mangle fact
    fact := mangle.Fact{
        Predicate: "net_request",
        Args: []interface{}{
            requestID,
            method,
            url,
            string(e.Initiator.Type),
            timestamp.Unix(),
        },
        Timestamp: timestamp,
    }
})
```

### NetworkResponseReceived

Fired when HTTP response is available.

```go
page.EachEvent(func(e *proto.NetworkResponseReceived) {
    requestID := e.RequestID.String()
    status := e.Response.Status              // 200, 404, 500, etc.
    statusText := e.Response.StatusText      // "OK", "Not Found"
    headers := e.Response.Headers            // Response headers
    mimeType := e.Response.MimeType          // "text/html", "application/json"
    url := e.Response.URL

    // Calculate latency (time from request to response)
    // Need to track request timestamps separately

    fact := mangle.Fact{
        Predicate: "net_response",
        Args: []interface{}{
            requestID,
            int64(status),
            0,      // Latency (calculate from request time)
            0,      // Duration (from LoadingFinished)
        },
        Timestamp: e.Timestamp.Time(),
    }
})
```

### NetworkLoadingFinished

Fired when request completes.

```go
page.EachEvent(func(e *proto.NetworkLoadingFinished) {
    requestID := e.RequestID.String()
    timestamp := e.Timestamp.Time()
    encodedDataLength := e.EncodedDataLength  // Bytes transferred

    // Calculate total duration
    // Need to track request start time
})
```

### NetworkLoadingFailed

Fired when request fails.

```go
page.EachEvent(func(e *proto.NetworkLoadingFailed) {
    requestID := e.RequestID.String()
    errorText := e.ErrorText             // "net::ERR_CONNECTION_REFUSED"
    canceled := e.Canceled               // User canceled?
    blockedReason := e.BlockedReason     // CORS, CSP, etc.

    fact := mangle.Fact{
        Predicate: "net_failure",
        Args: []interface{}{
            requestID,
            errorText,
            canceled,
        },
        Timestamp: e.Timestamp.Time(),
    }
})
```

### NetworkWebSocketCreated

Fired when WebSocket is created.

```go
page.EachEvent(func(e *proto.NetworkWebSocketCreated) {
    requestID := e.RequestID.String()
    url := e.URL
    initiator := e.Initiator

    fact := mangle.Fact{
        Predicate: "websocket_created",
        Args: []interface{}{requestID, url},
        Timestamp: time.Now(),
    }
})
```

## Runtime Domain

### RuntimeConsoleAPICalled

Fired when console API is called (console.log, console.error, etc.).

```go
page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
    // Event fields
    callType := e.Type                    // "log", "error", "warning", "debug", "info"
    args := e.Args                        // Array of RemoteObject
    timestamp := e.Timestamp.Time()
    stackTrace := e.StackTrace            // Call stack if available
    context := e.ExecutionContextID

    // Extract messages from arguments
    messages := make([]string, 0, len(e.Args))
    for _, arg := range e.Args {
        messages = append(messages, formatRemoteObject(arg))
    }

    message := strings.Join(messages, " ")

    fact := mangle.Fact{
        Predicate: "console_event",
        Args: []interface{}{
            string(callType),
            message,
            timestamp.Unix(),
        },
        Timestamp: timestamp,
    }
})

// Helper to format RemoteObject
func formatRemoteObject(obj *proto.RuntimeRemoteObject) string {
    switch obj.Type {
    case proto.RuntimeRemoteObjectTypeString:
        return obj.Value.String()
    case proto.RuntimeRemoteObjectTypeNumber:
        return fmt.Sprintf("%v", obj.Value.Num())
    case proto.RuntimeRemoteObjectTypeBoolean:
        return fmt.Sprintf("%v", obj.Value.Bool())
    case proto.RuntimeRemoteObjectTypeObject:
        if obj.Preview != nil {
            return obj.Preview.Description
        }
        return obj.Description
    default:
        return obj.Description
    }
}
```

### RuntimeExceptionThrown

Fired when exception is thrown and not handled.

```go
page.EachEvent(func(e *proto.RuntimeExceptionThrown) {
    exception := e.ExceptionDetails
    message := exception.Text
    lineNumber := exception.LineNumber
    columnNumber := exception.ColumnNumber
    url := exception.URL
    stackTrace := exception.StackTrace

    fact := mangle.Fact{
        Predicate: "exception_thrown",
        Args: []interface{}{
            message,
            url,
            int64(lineNumber),
            int64(columnNumber),
        },
        Timestamp: time.Now(),
    }
})
```

## Page Domain

### PageFrameNavigated

Fired when frame has navigated.

```go
page.EachEvent(func(e *proto.PageFrameNavigated) {
    frame := e.Frame
    url := frame.URL
    frameID := frame.ID
    parentFrameID := frame.ParentID  // Empty if main frame

    // Get previous URL (need to track in state)
    fromURL := getPreviousURL(frameID)

    fact := mangle.Fact{
        Predicate: "navigation_event",
        Args: []interface{}{
            fromURL,
            url,
            time.Now().Unix(),
        },
        Timestamp: time.Now(),
    }

    // Update state
    updateURL(frameID, url)
})
```

### PageDomContentEventFired

Fired when DOMContentLoaded event fires.

```go
page.EachEvent(func(e *proto.PageDomContentEventFired) {
    timestamp := e.Timestamp.Time()

    fact := mangle.Fact{
        Predicate: "page_event",
        Args: []interface{}{"DOMContentLoaded", timestamp.Unix()},
        Timestamp: timestamp,
    }
})
```

### PageLoadEventFired

Fired when Load event fires.

```go
page.EachEvent(func(e *proto.PageLoadEventFired) {
    timestamp := e.Timestamp.Time()

    fact := mangle.Fact{
        Predicate: "page_event",
        Args: []interface{}{"Load", timestamp.Unix()},
        Timestamp: timestamp,
    }
})
```

### PageJavascriptDialogOpening

Fired when JavaScript dialog (alert, confirm, prompt) appears.

```go
page.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
    dialogType := e.Type    // "alert", "confirm", "prompt", "beforeunload"
    message := e.Message
    defaultPrompt := e.DefaultPrompt

    // Auto-handle to prevent blocking
    page.Browser().Call(ctx, &proto.PageHandleJavaScriptDialog{
        Accept:     true,
        PromptText: "",
    })

    fact := mangle.Fact{
        Predicate: "dialog_event",
        Args: []interface{}{string(dialogType), message},
        Timestamp: time.Now(),
    }
})
```

## DOM Domain

### DOMDocumentUpdated

Fired when Document has been totally updated.

```go
page.EachEvent(func(e *proto.DOMDocumentUpdated) {
    // Document structure changed significantly
    // Good time to snapshot DOM

    fact := mangle.Fact{
        Predicate: "dom_updated",
        Args: []interface{}{
            getCurrentSessionID(),
            time.Now().Unix(),
        },
        Timestamp: time.Now(),
    }
})
```

### DOMAttributeModified

Fired when element attribute is modified.

```go
page.EachEvent(func(e *proto.DOMAttributeModified) {
    nodeID := int64(e.NodeID)
    name := e.Name
    value := e.Value

    fact := mangle.Fact{
        Predicate: "dom_attr_changed",
        Args: []interface{}{nodeID, name, value},
        Timestamp: time.Now(),
    }
})
```

### DOMCharacterDataModified

Fired when text content changes.

```go
page.EachEvent(func(e *proto.DOMCharacterDataModified) {
    nodeID := int64(e.NodeID)
    characterData := e.CharacterData

    fact := mangle.Fact{
        Predicate: "dom_text_changed",
        Args: []interface{}{nodeID, characterData},
        Timestamp: time.Now(),
    }
})
```

## Input Domain

### InputClick

Not an event - must be tracked manually via instrumentation:

```javascript
// Inject click tracker
document.addEventListener('click', (e) => {
    const nodeId = e.target.getAttribute('data-node-id');
    window.__browserNERD__.recordClick(nodeId, Date.now());
}, true);
```

Then retrieve from injected state.

## Event Subscription Best Practices

### Subscribe Early

```go
// Subscribe BEFORE navigating
page.EachEvent(func(e *proto.NetworkRequestWillBeSent) { ... })
page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) { ... })

// Then navigate
page.Navigate("https://example.com")
```

### Handle Event Races

```go
// Use buffered channel to prevent dropped events
eventChan := make(chan mangle.Fact, 1000)

page.EachEvent(func(e *proto.NetworkRequestWillBeSent) {
    fact := transformToFact(e)
    select {
    case eventChan <- fact:
    default:
        log.Warn("event buffer full, dropping fact")
    }
})

// Process events in separate goroutine
go func() {
    for fact := range eventChan {
        engine.AddFacts(ctx, []mangle.Fact{fact})
    }
}()
```

### Cancel Subscriptions

```go
// Store cancellation functions
cancels := []func(){}

cancel := page.EachEvent(func(e *proto.NetworkRequestWillBeSent) { ... })
cancels = append(cancels, cancel)

// Clean up when done
defer func() {
    for _, cancel := range cancels {
        cancel()
    }
}()
```

## State Tracking for Derived Facts

Many CDP events require stateful tracking:

```go
type EventTracker struct {
    mu sync.RWMutex

    // Track request start times for latency calculation
    requestTimes map[string]time.Time

    // Track previous URLs for navigation
    frameURLs map[string]string

    // Track pending requests for orphan detection
    pendingRequests map[string]bool
}

func (t *EventTracker) OnRequestWillBeSent(e *proto.NetworkRequestWillBeSent) {
    t.mu.Lock()
    defer t.mu.Unlock()

    t.requestTimes[e.RequestID.String()] = e.Timestamp.Time()
    t.pendingRequests[e.RequestID.String()] = true
}

func (t *EventTracker) OnResponseReceived(e *proto.NetworkResponseReceived) mangle.Fact {
    t.mu.Lock()
    defer t.mu.Unlock()

    reqID := e.RequestID.String()
    startTime, exists := t.requestTimes[reqID]

    var latency int64
    if exists {
        latency = e.Timestamp.Time().Sub(startTime).Milliseconds()
    }

    delete(t.pendingRequests, reqID)

    return mangle.Fact{
        Predicate: "net_response",
        Args: []interface{}{
            reqID,
            int64(e.Response.Status),
            latency,
            0, // Duration filled in LoadingFinished
        },
        Timestamp: e.Timestamp.Time(),
    }
}
```

## Fact Transformation Patterns

### Timestamp Normalization

```go
// CDP uses TimeSinceEpoch (seconds.microseconds)
cdpTime := e.Timestamp.Time()

// Store as Unix seconds for Mangle
unixSeconds := cdpTime.Unix()

// Or milliseconds for higher precision
unixMillis := cdpTime.UnixMilli()
```

### Header Extraction

```go
func extractHeaders(headers map[string]interface{}) []mangle.Fact {
    facts := make([]mangle.Fact, 0, len(headers))

    for key, value := range headers {
        fact := mangle.Fact{
            Predicate: "net_header",
            Args: []interface{}{
                requestID,
                "request", // or "response"
                key,
                fmt.Sprintf("%v", value),
            },
            Timestamp: time.Now(),
        }
        facts = append(facts, fact)
    }

    return facts
}
```

### Status Code Categories

```go
func categorizeStatus(status int) string {
    switch {
    case status >= 200 && status < 300:
        return "success"
    case status >= 300 && status < 400:
        return "redirect"
    case status >= 400 && status < 500:
        return "client_error"
    case status >= 500:
        return "server_error"
    default:
        return "unknown"
    }
}
```

## Performance Considerations

### Event Throttling

```go
// Throttle high-frequency events
type Throttler struct {
    mu         sync.Mutex
    lastEmit   time.Time
    minInterval time.Duration
}

func (t *Throttler) ShouldEmit() bool {
    t.mu.Lock()
    defer t.mu.Unlock()

    now := time.Now()
    if now.Sub(t.lastEmit) < t.minInterval {
        return false
    }

    t.lastEmit = now
    return true
}

// Usage
domThrottler := &Throttler{minInterval: 100 * time.Millisecond}

page.EachEvent(func(e *proto.DOMAttributeModified) {
    if domThrottler.ShouldEmit() {
        // Process event
    }
})
```

### Batching

```go
// Batch facts before sending to engine
type FactBatcher struct {
    mu       sync.Mutex
    batch    []mangle.Fact
    maxSize  int
    maxWait  time.Duration
    engine   *mangle.Engine
    timer    *time.Timer
}

func (b *FactBatcher) Add(fact mangle.Fact) {
    b.mu.Lock()
    defer b.mu.Unlock()

    b.batch = append(b.batch, fact)

    if len(b.batch) >= b.maxSize {
        b.flush()
    } else if b.timer == nil {
        b.timer = time.AfterFunc(b.maxWait, b.flush)
    }
}

func (b *FactBatcher) flush() {
    if len(b.batch) == 0 {
        return
    }

    b.engine.AddFacts(context.Background(), b.batch)
    b.batch = nil
    b.timer = nil
}
```
