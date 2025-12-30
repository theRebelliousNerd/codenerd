package research

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"codenerd/internal/browser"
	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// browserManager holds a shared browser session manager.
var (
	browserMgr     *browser.SessionManager
	browserMgrOnce sync.Once
	browserMgrMu   sync.Mutex
)

// getBrowserManager returns the shared browser session manager.
func getBrowserManager() *browser.SessionManager {
	browserMgrOnce.Do(func() {
		browserMgr = browser.NewSessionManager(browser.DefaultConfig(), nil)
	})
	return browserMgr
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

	logging.BrowserDebug("Browser navigate: url=%s, session=%s", url, sessionID)

	mgr := getBrowserManager()

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

	logging.Browser("Browser navigated to %s (session=%s)", url, session.ID)

	return fmt.Sprintf("Successfully navigated to %s\nSession ID: %s\nStatus: %s",
		url, session.ID, session.Status), nil
}

// BrowserExtractTool returns a tool for extracting content from a browser page.
func BrowserExtractTool() *tools.Tool {
	return &tools.Tool{
		Name:        "browser_extract",
		Description: "Extract text content from the current browser page",
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
					Description: "Include raw HTML in output (default: false)",
					Default:     false,
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

	logging.BrowserDebug("Browser extract: session=%s, selector=%s", sessionID, selector)

	mgr := getBrowserManager()

	page, ok := mgr.Page(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	// Get text content
	el, err := page.Element(selector)
	if err != nil {
		return "", fmt.Errorf("element not found: %s", selector)
	}

	text, err := el.Text()
	if err != nil {
		return "", fmt.Errorf("failed to get text: %w", err)
	}

	logging.Browser("Browser extract completed: %d chars", len(text))
	return text, nil
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

	// Note: The browser package doesn't have a direct close session method,
	// so we just log this for now. The session will be cleaned up on shutdown.
	logging.Browser("Browser session marked for close: %s", sessionID)
	return fmt.Sprintf("Session %s marked for close", sessionID), nil
}
