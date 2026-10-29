package research

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/browser"
)

func TestBrowserManagerBindingCompareAndClear(t *testing.T) {
	first := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	second := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	SetBrowserRuntime(first, nil)

	ClearBrowserManager(second)
	if got := getBrowserManager(); got != first {
		t.Fatal("clearing a different Cortex manager removed the live binding")
	}
	ClearBrowserManager(first)
	if got := getBrowserManager(); got != nil {
		t.Fatal("clearing the active manager constructed a fallback browser")
	}
	SetBrowserRuntime(nil, nil)
}

func TestBrowserRuntimeBindingKeepsKernelPairedWithManager(t *testing.T) {
	first := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	second := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	kernel := &browserReasoningKernel{}
	SetBrowserRuntime(first, kernel)

	ClearBrowserManager(second)
	if getBrowserKernel() != kernel {
		t.Fatal("clearing a different manager detached the live browser kernel")
	}
	ClearBrowserManager(first)
	if getBrowserKernel() != nil {
		t.Fatal("clearing the owning manager retained a stale browser kernel")
	}
	SetBrowserRuntime(nil, nil)
}

// With no runtime bound, every browser tool refuses with an error. Before the
// fallback manager was removed these helpers built one silently; now each
// entry must refuse before any helper dereferences the missing manager.
func TestBrowserToolsRefuseWhenUnbound(t *testing.T) {
	SetBrowserRuntime(nil, nil)
	tools := map[string]func(context.Context, map[string]any) (string, error){
		"audit":    executeBrowserAudit,
		"test":     executeBrowserTest,
		"evidence": executeBrowserEvidence,
		"observe":  executeBrowserObserve,
		"act":      executeBrowserAct,
		"mangle":   executeBrowserMangle,
		"wait":     executeBrowserWait,
		"reason":   executeBrowserReason,
		"specs":    executeBrowserSpecs,
	}
	for name, fn := range tools {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panicked with no browser bound: %v", r)
				}
			}()
			_, err := fn(context.Background(), map[string]any{"session_id": "s1"})
			if err == nil || !strings.Contains(err.Error(), "not bound") {
				t.Fatalf("err = %v, want the not-bound refusal", err)
			}
		})
	}
	recordBrowserToolEvidence("s1", "probe", map[string]any{}) // must not panic
}
