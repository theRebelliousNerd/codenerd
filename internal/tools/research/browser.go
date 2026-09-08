package research

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
	"unicode/utf8"

	"codenerd/internal/browser"
	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// browserManager holds a shared browser session manager.
var (
	browserMgr         *browser.SessionManager
	browserKernel      types.Kernel
	browserKernelOwner *browser.SessionManager
	browserMgrMu       sync.RWMutex
)

// getBrowserManager returns the shared browser session manager.
func getBrowserManager() *browser.SessionManager {
	browserMgrMu.RLock()
	mgr := browserMgr
	browserMgrMu.RUnlock()
	if mgr != nil {
		return mgr
	}
	browserMgrMu.Lock()
	defer browserMgrMu.Unlock()
	if browserMgr == nil {
		browserMgr = browser.NewSessionManager(browser.DefaultConfig(), nil)
	}
	return browserMgr
}

// SetBrowserManager binds research tools to the Cortex-owned browser manager.
// Passing nil restores lazy standalone construction for narrow package use.
func SetBrowserManager(mgr *browser.SessionManager) {
	browserMgrMu.Lock()
	browserMgr = mgr
	if browserKernelOwner != mgr {
		browserKernel = nil
		browserKernelOwner = nil
	}
	browserMgrMu.Unlock()
}

// SetBrowserRuntime binds both browser control and browser reasoning to one
// Cortex. The kernel is the same live authority used for planning and policy.
func SetBrowserRuntime(mgr *browser.SessionManager, kernel types.Kernel) {
	browserMgrMu.Lock()
	browserMgr = mgr
	browserKernel = kernel
	browserKernelOwner = mgr
	browserMgrMu.Unlock()
}

func getBrowserKernel() types.Kernel {
	browserMgrMu.RLock()
	defer browserMgrMu.RUnlock()
	return browserKernel
}

// ClearBrowserManager removes mgr only if it is still the process binding.
// This prevents an older Cortex shutdown from detaching a newer Cortex manager.
func ClearBrowserManager(mgr *browser.SessionManager) {
	browserMgrMu.Lock()
	if browserMgr == mgr {
		browserMgr = nil
	}
	if browserKernelOwner == mgr {
		browserKernel = nil
		browserKernelOwner = nil
	}
	browserMgrMu.Unlock()
}

// BrowserNavigateTool returns a tool for navigating to a URL with a browser.
func BrowserNavigateTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_navigate",
		Description: "Navigate to a URL using a headless browser, useful for JavaScript-rendered pages",
		Category:    tools.CategoryResearch,
		Priority:    60,
		Execute:     executeBrowserNavigate,
		Schema: tools.ToolSchema{
			Required: []string{"url"},
			Properties: map[string]tools.Property{
				"url": {
					Type:        "string",
					Description: "The URL to navigate to",
				},
				"wait_stable": {
					Type:        "boolean",
					Description: "Wait for page to be stable before returning (default: true)",
					Default:     true,
				},
				"session_id": {
					Type:        "string",
					Description: "Optional session ID to reuse an existing browser session",
				},
			},
		},
	}
}

func executeBrowserNavigate(ctx context.Context, args map[string]any) (string, error) {
	url, _ := args["url"].(string)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	sessionID, _ := args["session_id"].(string)

	mgr := getBrowserManager()
	safeURL := mgr.SanitizeForEvidence(url)
	logging.BrowserDebug("Browser navigate: url=%s, session=%s", safeURL, sessionID)

	// Start browser if needed
	if err := mgr.Start(ctx); err != nil {
		return "", fmt.Errorf("failed to start browser: %w", err)
	}

	var session *browser.Session
	var err error

	if sessionID != "" {
		// Navigate existing session
		err = mgr.Navigate(ctx, sessionID, url)
		if err != nil {
			return "", fmt.Errorf("failed to navigate: %w", err)
		}
		sess, ok := mgr.GetSession(sessionID)
		if !ok {
			return "", fmt.Errorf("session not found after navigation")
		}
		session = &sess
	} else {
		// Create new session
		session, err = mgr.CreateSession(ctx, url)
		if err != nil {
			return "", fmt.Errorf("failed to create session: %w", err)
		}
	}

	logging.Browser("Browser navigated to %s (session=%s)", safeURL, session.ID)

	return fmt.Sprintf("Successfully navigated to %s\nSession ID: %s\nStatus: %s",
		safeURL, session.ID, session.Status), nil
}

// BrowserExtract limits for bounded evidence reads.
const (
	defaultBrowserExtractMaxChars = 8000
	maxBrowserExtractMaxChars     = 32000
	defaultBrowserExtractTimeout  = 10 * time.Second
)

// BrowserExtractTool returns a tool for extracting content from a browser page.
func BrowserExtractTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_extract",
		Description: "Extract bounded, redacted text from the current browser page. Extraction honors caller cancellation and a 10s maximum duration. Combined text and optional HTML are capped by max_chars, followed by a truncation notice when needed.",
		Category:    tools.CategoryResearch,
		Priority:    55,
		Execute:     executeBrowserExtract,
		Schema: tools.ToolSchema{
			Required: []string{"session_id"},
			Properties: map[string]tools.Property{
				"session_id": {
					Type:        "string",
					Description: "The browser session ID",
				},
				"selector": {
					Type:        "string",
					Description: "Optional CSS selector to extract specific element (default: body)",
					Default:     "body",
				},
				"include_html": {
					Type:        "boolean",
					Description: "Include bounded, redacted outer HTML after the text section (default: false)",
					Default:     false,
				},
				"max_chars": {
					Type:        "integer",
					Description: "Maximum combined text/HTML runes, excluding the truncation notice (default: 8000, hard cap: 32000)",
					Default:     defaultBrowserExtractMaxChars,
				},
			},
		},
	}
}

func executeBrowserExtract(ctx context.Context, args map[string]any) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	selector := "body"
	if sel, ok := args["selector"].(string); ok && sel != "" {
		selector = sel
	}
	includeHTML := boolArg(args, "include_html", false)
	maxChars := resolveBrowserExtractMaxChars(args)

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("browser extract cancelled: %w", err)
	}

	logging.BrowserDebug("Browser extract: session=%s, selector=%s", sessionID, selector)

	mgr := getBrowserManager()

	page, ok := mgr.Page(sessionID)
	if !ok || page == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	lookupCtx, cancel := withBrowserExtractDeadline(ctx)
	defer cancel()

	// Propagate caller cancellation to the blocking lookup. Rod honors the
	// page context. The child deadline also bounds a missing selector when the
	// enclosing campaign has hours left to run.
	el, err := page.Context(lookupCtx).Element(selector)
	if err != nil {
		if lookupErr := lookupCtx.Err(); lookupErr != nil {
			return "", fmt.Errorf("browser extract lookup for %q: %w", selector, lookupErr)
		}
		return "", fmt.Errorf("browser extract lookup for %q: %w", selector, err)
	}

	text, err := el.Context(lookupCtx).Text()
	if err != nil {
		if lookupErr := lookupCtx.Err(); lookupErr != nil {
			return "", fmt.Errorf("browser extract text for %q: %w", selector, lookupErr)
		}
		return "", fmt.Errorf("browser extract text for %q: %w", selector, err)
	}

	if includeHTML {
		html, htmlErr := el.Context(lookupCtx).HTML()
		if htmlErr != nil {
			if lookupErr := lookupCtx.Err(); lookupErr != nil {
				return "", fmt.Errorf("browser extract html for %q: %w", selector, lookupErr)
			}
			return "", fmt.Errorf("browser extract html for %q: %w", selector, htmlErr)
		}
		text += "\n\n--- html ---\n" + html
	}

	sanitized := mgr.SanitizeForEvidence(text)
	result, truncated, totalRunes := boundBrowserExtractChars(sanitized, maxChars)
	if truncated {
		result += fmt.Sprintf("\n...[truncated: showing %d of %d chars; max_chars=%d, hard cap=%d]", maxChars, totalRunes, maxChars, maxBrowserExtractMaxChars)
	}

	logging.Browser("Browser extract completed: %d chars (truncated=%v)", len(result), truncated)
	return result, nil
}

// resolveBrowserExtractMaxChars clamps caller max_chars to the conservative
// default and hard cap. Non-positive or missing values select the default.
func resolveBrowserExtractMaxChars(args map[string]any) int {
	maxChars := intArg(args, "max_chars", defaultBrowserExtractMaxChars)
	if maxChars <= 0 {
		return defaultBrowserExtractMaxChars
	}
	if maxChars > maxBrowserExtractMaxChars {
		return maxBrowserExtractMaxChars
	}
	return maxChars
}

// withBrowserExtractDeadline caps the whole extraction while preserving any
// earlier caller cancellation/deadline. It does not alter manager-owned pages.
func withBrowserExtractDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, defaultBrowserExtractTimeout)
}

// boundBrowserExtractChars truncates by rune count so multi-byte UTF-8 is never
// split. An input exactly at the limit is returned unmarked; only a strictly
// longer input reports truncation.
func boundBrowserExtractChars(value string, maxChars int) (string, bool, int) {
	total := utf8.RuneCountInString(value)
	if total <= maxChars {
		return value, false, total
	}
	count := 0
	for idx := range value {
		if count == maxChars {
			return value[:idx], true, total
		}
		count++
	}
	return value, false, total
}

// BrowserScreenshotTool returns a tool for capturing screenshots.
func BrowserScreenshotTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_screenshot",
		Description: "Capture a screenshot of the current browser page",
		Category:    tools.CategoryResearch,
		Priority:    50,
		Execute:     executeBrowserScreenshot,
		Schema: tools.ToolSchema{
			Required: []string{"session_id"},
			Properties: map[string]tools.Property{
				"session_id": {
					Type:        "string",
					Description: "The browser session ID",
				},
				"full_page": {
					Type:        "boolean",
					Description: "Capture full page or just viewport (default: false)",
					Default:     false,
				},
			},
		},
	}
}

func executeBrowserScreenshot(ctx context.Context, args map[string]any) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	fullPage := false
	if fp, ok := args["full_page"].(bool); ok {
		fullPage = fp
	}

	logging.BrowserDebug("Browser screenshot: session=%s, full_page=%v", sessionID, fullPage)

	mgr := getBrowserManager()

	data, err := mgr.Screenshot(ctx, sessionID, fullPage)
	if err != nil {
		return "", fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// Return base64-encoded image
	encoded := base64.StdEncoding.EncodeToString(data)

	logging.Browser("Browser screenshot captured: %d bytes", len(data))
	return fmt.Sprintf("data:image/png;base64,%s", encoded), nil
}

// BrowserClickTool returns a tool for clicking elements.
func BrowserClickTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_click",
		Description: "Click an element on the page",
		Category:    tools.CategoryResearch,
		Priority:    50,
		Execute:     executeBrowserClick,
		Schema: tools.ToolSchema{
			Required: []string{"session_id", "selector"},
			Properties: map[string]tools.Property{
				"session_id": {
					Type:        "string",
					Description: "The browser session ID",
				},
				"selector": {
					Type:        "string",
					Description: "CSS selector for the element to click",
				},
			},
		},
	}
}

func executeBrowserClick(ctx context.Context, args map[string]any) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	selector, _ := args["selector"].(string)
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	logging.BrowserDebug("Browser click: session=%s, selector=%s", sessionID, selector)

	mgr := getBrowserManager()

	if err := mgr.Click(ctx, sessionID, selector); err != nil {
		return "", fmt.Errorf("failed to click: %w", err)
	}

	logging.Browser("Browser clicked: %s", selector)
	return fmt.Sprintf("Clicked element: %s", selector), nil
}

// BrowserTypeTool returns a tool for typing into input fields.
func BrowserTypeTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_type",
		Description: "Type text into an input field",
		Category:    tools.CategoryResearch,
		Priority:    50,
		Execute:     executeBrowserType,
		Schema: tools.ToolSchema{
			Required: []string{"session_id", "selector", "text"},
			Properties: map[string]tools.Property{
				"session_id": {
					Type:        "string",
					Description: "The browser session ID",
				},
				"selector": {
					Type:        "string",
					Description: "CSS selector for the input element",
				},
				"text": {
					Type:        "string",
					Description: "Text to type",
				},
			},
		},
	}
}

func executeBrowserType(ctx context.Context, args map[string]any) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	selector, _ := args["selector"].(string)
	if selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	text, _ := args["text"].(string)
	if text == "" {
		return "", fmt.Errorf("text is required")
	}

	logging.BrowserDebug("Browser type: session=%s, selector=%s, text_len=%d", sessionID, selector, len(text))

	mgr := getBrowserManager()

	if err := mgr.Type(ctx, sessionID, selector, text); err != nil {
		return "", fmt.Errorf("failed to type: %w", err)
	}

	logging.Browser("Browser typed %d chars into %s", len(text), selector)
	return fmt.Sprintf("Typed %d characters into: %s", len(text), selector), nil
}

// BrowserCloseTool returns a tool for closing browser sessions.
func BrowserCloseTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_close",
		Description: "Close a browser session",
		Category:    tools.CategoryResearch,
		Priority:    40,
		Execute:     executeBrowserClose,
		Schema: tools.ToolSchema{
			Required: []string{"session_id"},
			Properties: map[string]tools.Property{
				"session_id": {
					Type:        "string",
					Description: "The browser session ID to close",
				},
			},
		},
	}
}

func executeBrowserClose(ctx context.Context, args map[string]any) (string, error) {
	sessionID, _ := args["session_id"].(string)
	if sessionID == "" {
		return "", fmt.Errorf("session_id is required")
	}

	logging.BrowserDebug("Browser close: session=%s", sessionID)

	if err := getBrowserManager().CloseSession(ctx, sessionID); err != nil {
		return "", fmt.Errorf("failed to close browser session: %w", err)
	}
	logging.Browser("Browser session closed: %s", sessionID)
	return fmt.Sprintf("Session %s closed", sessionID), nil
}
