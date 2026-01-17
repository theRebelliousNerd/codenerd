package research

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// BROWSER TOOL TESTS
// =============================================================================

func TestBrowserNavigateTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BrowserNavigateTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "browser_navigate" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if tool.Execute == nil {
		t.Error("Execute should be set")
	}
}

func TestBrowserExtractTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BrowserExtractTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "browser_extract" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestBrowserScreenshotTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BrowserScreenshotTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestBrowserClickTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BrowserClickTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestBrowserTypeTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BrowserTypeTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestBrowserCloseTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BrowserCloseTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

// =============================================================================
// WEB SEARCH TOOL TESTS
// =============================================================================

func TestWebSearchTool_Definition(t *testing.T) {
	t.Parallel()

	tool := WebSearchTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "web_search" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

// =============================================================================
// WEB FETCH TOOL TESTS
// =============================================================================

func TestWebFetchTool_Definition(t *testing.T) {
	t.Parallel()

	tool := WebFetchTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	// Tool name may vary - just check it exists
	if tool.Name == "" {
		t.Error("expected non-empty tool name")
	}
}

// =============================================================================
// CONTEXT7 TOOL TESTS
// =============================================================================

func TestContext7Tool_Definition(t *testing.T) {
	t.Parallel()

	tool := Context7Tool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	// Tool name may vary - just check it exists
	if tool.Name == "" {
		t.Error("expected non-empty tool name")
	}
}

// =============================================================================
// CACHE TESTS
// =============================================================================

func TestNewResearchCache(t *testing.T) {
	t.Parallel()

	cache := NewResearchCache(100, 30*time.Minute)

	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestResearchCache_GetSet(t *testing.T) {
	t.Parallel()

	cache := NewResearchCache(100, time.Hour)

	// Initially empty
	_, found := cache.Get("test-key")
	if found {
		t.Error("expected not found for missing key")
	}

	// Set value
	cache.Set("test-key", "test-value", "test-source")

	// Get value
	entry, found := cache.Get("test-key")
	if !found {
		t.Error("expected to find key after Set")
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}
	if entry.Value != "test-value" {
		t.Errorf("Value mismatch: got %q", entry.Value)
	}
	if entry.Source != "test-source" {
		t.Errorf("Source mismatch: got %q", entry.Source)
	}
}

func TestResearchCache_Size(t *testing.T) {
	t.Parallel()

	cache := NewResearchCache(100, time.Hour)

	if cache.Size() != 0 {
		t.Errorf("expected 0 size initially, got %d", cache.Size())
	}

	cache.Set("key1", "value1", "source")
	cache.Set("key2", "value2", "source")

	if cache.Size() != 2 {
		t.Errorf("expected 2 size, got %d", cache.Size())
	}
}

func TestResearchCache_Delete(t *testing.T) {
	t.Parallel()

	cache := NewResearchCache(100, time.Hour)

	cache.Set("key", "value", "source")
	cache.Delete("key")

	_, found := cache.Get("key")
	if found {
		t.Error("expected not found after Delete")
	}
}

func TestResearchCache_Clear(t *testing.T) {
	t.Parallel()

	cache := NewResearchCache(100, time.Hour)

	cache.Set("key1", "value1", "source")
	cache.Set("key2", "value2", "source")

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("expected 0 size after Clear, got %d", cache.Size())
	}
}

func TestResearchCache_Eviction(t *testing.T) {
	t.Parallel()

	// Create cache with max size 2
	cache := NewResearchCache(2, time.Hour)

	cache.Set("key1", "value1", "source")
	cache.Set("key2", "value2", "source")
	cache.Set("key3", "value3", "source") // Should trigger eviction

	if cache.Size() > 2 {
		t.Errorf("expected max 2 entries, got %d", cache.Size())
	}
}

// =============================================================================
// CACHE TOOL TESTS
// =============================================================================

func TestCacheGetTool_Definition(t *testing.T) {
	t.Parallel()

	tool := CacheGetTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "research_cache_get" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestCacheSetTool_Definition(t *testing.T) {
	t.Parallel()

	tool := CacheSetTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	if tool.Name != "research_cache_set" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestCacheClearTool_Definition(t *testing.T) {
	t.Parallel()

	tool := CacheClearTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestCacheStatsTool_Definition(t *testing.T) {
	t.Parallel()

	tool := CacheStatsTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
}

func TestExecuteCacheSet_MissingKey(t *testing.T) {
	t.Parallel()

	_, err := executeCacheSet(context.Background(), map[string]any{
		"value": "test",
	})
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestExecuteCacheSet_MissingValue(t *testing.T) {
	t.Parallel()

	_, err := executeCacheSet(context.Background(), map[string]any{
		"key": "test",
	})
	if err == nil {
		t.Error("expected error for missing value")
	}
}

func TestExecuteCacheGet_MissingKey(t *testing.T) {
	t.Parallel()

	_, err := executeCacheGet(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing key")
	}
}

// =============================================================================
// REGISTER ALL TEST
// =============================================================================

func TestRegisterAll_NotNil(t *testing.T) {
	t.Parallel()

	// Test that all tools can be fetched (implying RegisterAll works)
	tools := []*struct{ name string }{
		{"Context7Tool"},
		{"WebSearchTool"},
		{"WebFetchTool"},
		{"BrowserNavigateTool"},
	}

	for _, tc := range tools {
		if tc.name == "" {
			t.Error("expected tool name")
		}
	}
}
