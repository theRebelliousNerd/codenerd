package research

import (
	"testing"

	"codenerd/internal/browser"
)

func TestBrowserManagerBindingCompareAndClear(t *testing.T) {
	first := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	second := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserManager(first)

	ClearBrowserManager(second)
	if got := getBrowserManager(); got != first {
		t.Fatal("clearing a different Cortex manager removed the live binding")
	}
	ClearBrowserManager(first)
	if got := getBrowserManager(); got == first || got == nil {
		t.Fatal("clearing the active manager did not restore lazy standalone construction")
	}
	SetBrowserManager(nil)
}
