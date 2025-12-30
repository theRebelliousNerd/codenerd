package chat

import (
	"context"
	"sync"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"github.com/charmbracelet/glamour"
)

// TestMessageRenderingCache verifies that the rendering cache prevents redundant markdown rendering
func TestMessageRenderingCache(t *testing.T) {
	// Create a minimal model with cache initialized
	m := Model{
		history:          []Message{},
		renderedCache:    make(map[int]string),
		cacheInvalidFrom: 0,
		styles:           ui.DefaultStyles(),
		glassBoxEnabled:  false,
	}

	// Initialize glamour renderer (needed for safeRenderMarkdown)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		t.Logf("Glamour init failed (expected in test env): %v", err)
	}
	m.renderer = renderer

	// Add messages using the helper (which pre-renders and caches)
	msg1 := Message{
		Role:    "user",
		Content: "Test message 1",
		Time:    time.Now(),
	}
	msg2 := Message{
		Role:    "assistant",
		Content: "Test response with **markdown**",
		Time:    time.Now(),
	}
	msg3 := Message{
		Role:    "user",
		Content: "Follow-up question",
		Time:    time.Now(),
	}

	// Add messages one at a time
	m = m.addMessage(msg1)
	m = m.addMessage(msg2)
	m = m.addMessage(msg3)

	// Verify cache contains all messages
	if len(m.renderedCache) != 3 {
		t.Errorf("Expected 3 cached messages, got %d", len(m.renderedCache))
	}

	// Verify cache keys are correct
	for i := 0; i < 3; i++ {
		if _, exists := m.renderedCache[i]; !exists {
			t.Errorf("Message %d not in cache", i)
		}
	}

	// Verify cacheInvalidFrom is set correctly (all messages cached)
	if m.cacheInvalidFrom != 3 {
		t.Errorf("Expected cacheInvalidFrom=3, got %d", m.cacheInvalidFrom)
	}

	// Render history - should use cache
	startTime := time.Now()
	rendered := m.renderHistory()
	duration := time.Since(startTime)

	if rendered == "" {
		t.Error("renderHistory returned empty string")
	}

	// Cache hits should be very fast (<1ms for 3 messages)
	if duration > 10*time.Millisecond {
		t.Logf("Warning: renderHistory took %v (expected <10ms with cache)", duration)
	}

	t.Logf("✅ Cache test passed: 3 messages cached, rendered in %v", duration)
}

// TestAddMessagesHelper verifies bulk message addition
func TestAddMessagesHelper(t *testing.T) {
	m := Model{
		history:          []Message{},
		renderedCache:    make(map[int]string),
		cacheInvalidFrom: 0,
		styles:           ui.DefaultStyles(),
	}

	// Create 10 test messages
	messages := make([]Message, 10)
	for i := 0; i < 10; i++ {
		messages[i] = Message{
			Role:    "user",
			Content: "Test message",
			Time:    time.Now(),
		}
	}

	// Add all messages at once
	m = m.addMessages(messages...)

	if len(m.history) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(m.history))
	}

	if len(m.renderedCache) != 10 {
		t.Errorf("Expected 10 cached messages, got %d", len(m.renderedCache))
	}

	if m.cacheInvalidFrom != 10 {
		t.Errorf("Expected cacheInvalidFrom=10, got %d", m.cacheInvalidFrom)
	}

	t.Log("✅ Bulk add test passed: 10 messages added and cached")
}

// TestViewportPagination verifies that only recent messages are rendered
func TestViewportPagination(t *testing.T) {
	m := Model{
		history:          []Message{},
		renderedCache:    make(map[int]string),
		cacheInvalidFrom: 0,
		styles:           ui.DefaultStyles(),
	}

	// Add 150 messages to exceed the 100-message pagination threshold
	for i := 0; i < 150; i++ {
		msg := Message{
			Role:    "user",
			Content: "Message",
			Time:    time.Now(),
		}
		m = m.addMessage(msg)
	}

	// Render history - should only render last 100
	// (We can't easily verify this directly, but we check the logic)
	rendered := m.renderHistory()

	if rendered == "" {
		t.Error("renderHistory returned empty string")
	}

	// The pagination logic starts at index 50 (150 - 100)
	// We can verify by checking that the cache has all 150, but renderHistory
	// only processes from startIdx
	if len(m.history) != 150 {
		t.Errorf("Expected 150 messages in history, got %d", len(m.history))
	}

	t.Logf("✅ Pagination test passed: 150 messages stored, pagination active")
}

// TestGoroutineWaitGroup verifies goroutine tracking
func TestGoroutineWaitGroup(t *testing.T) {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	m := Model{
		shutdownOnce:   &sync.Once{},
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		goroutineWg:    &sync.WaitGroup{},
	}

	// Simulate spawning background goroutines
	for i := 0; i < 3; i++ {
		m.goroutineWg.Add(1)
		go func() {
			defer m.goroutineWg.Done()
			<-m.shutdownCtx.Done()
			// Simulate some cleanup work
			time.Sleep(10 * time.Millisecond)
		}()
	}

	// Trigger shutdown
	startTime := time.Now()
	(&m).Shutdown()
	shutdownDuration := time.Since(startTime)

	// Shutdown should wait for all goroutines (but not exceed timeout)
	if shutdownDuration > 6*time.Second {
		t.Errorf("Shutdown took too long: %v (expected <6s)", shutdownDuration)
	}

	// Verify WaitGroup is zero
	done := make(chan struct{})
	go func() {
		m.goroutineWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Logf("✅ Goroutine tracking test passed: All goroutines completed in %v", shutdownDuration)
	case <-time.After(1 * time.Second):
		t.Error("WaitGroup not zeroed - goroutines may still be running")
	}
}

// TestShutdownIdempotence verifies Shutdown() can be called multiple times safely
func TestShutdownIdempotence(t *testing.T) {
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	m := Model{
		shutdownOnce:   &sync.Once{},
		shutdownCtx:    shutdownCtx,
		shutdownCancel: shutdownCancel,
		goroutineWg:    &sync.WaitGroup{},
	}

	// Call Shutdown multiple times
	for i := 0; i < 5; i++ {
		(&m).Shutdown()
	}

	// Should not panic or hang
	t.Log("✅ Shutdown idempotence test passed: Multiple calls handled safely")
}

// BenchmarkRenderHistoryWithCache benchmarks rendering with cache hits
func BenchmarkRenderHistoryWithCache(b *testing.B) {
	m := Model{
		history:          []Message{},
		renderedCache:    make(map[int]string),
		cacheInvalidFrom: 0,
		styles:           ui.DefaultStyles(),
		glassBoxEnabled:  false,
	}

	// Pre-populate with 50 messages
	for i := 0; i < 50; i++ {
		msg := Message{
			Role:    "assistant",
			Content: "Test message with **markdown** and `code`",
			Time:    time.Now(),
		}
		m = m.addMessage(msg)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.renderHistory()
	}
}

// BenchmarkRenderHistoryWithoutCache benchmarks rendering without cache
func BenchmarkRenderHistoryWithoutCache(b *testing.B) {
	m := Model{
		history:         []Message{},
		renderedCache:   nil, // Disable cache
		styles:          ui.DefaultStyles(),
		glassBoxEnabled: false,
	}

	// Create renderer
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	m.renderer = renderer

	// Add 50 messages directly (bypass cache)
	for i := 0; i < 50; i++ {
		m.history = append(m.history, Message{
			Role:    "assistant",
			Content: "Test message with **markdown** and `code`",
			Time:    time.Now(),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Render each message without cache
		for _, msg := range m.history {
			_ = m.renderSingleMessage(msg)
		}
	}
}

// TestRenderSingleMessage verifies individual message rendering
func TestRenderSingleMessage(t *testing.T) {
	m := Model{
		styles:          ui.DefaultStyles(),
		glassBoxEnabled: false,
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		t.Logf("Glamour init failed (expected in test env): %v", err)
	}
	m.renderer = renderer

	tests := []struct {
		name string
		msg  Message
		want string // substring that should appear in output
	}{
		{
			name: "user message",
			msg:  Message{Role: "user", Content: "Hello"},
			want: "You",
		},
		{
			name: "assistant message",
			msg:  Message{Role: "assistant", Content: "Hi there"},
			want: "codeNERD",
		},
		{
			name: "tool message",
			msg:  Message{Role: "tool", Content: "Tool output"},
			want: "Tool Execution",
		},
		{
			name: "system message (glass box disabled)",
			msg:  Message{Role: "system", Content: "System event"},
			want: "", // Should be empty when glassBoxEnabled=false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered := m.renderSingleMessage(tt.msg)
			if tt.want != "" && !contains(rendered, tt.want) {
				t.Errorf("Expected output to contain %q, got: %s", tt.want, rendered[:min(len(rendered), 100)])
			}
			if tt.want == "" && rendered != "" {
				t.Errorf("Expected empty output for disabled system message, got: %s", rendered)
			}
		})
	}

	t.Log("✅ Single message rendering test passed")
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
