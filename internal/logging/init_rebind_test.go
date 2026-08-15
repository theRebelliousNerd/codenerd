package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// main() calls Initialize(os.Getwd()) before Cobra parses argv, then
// PersistentPreRunE calls it again with --workspace. Under a bare sync.Once the
// second call was a silent no-op and the whole run logged into the wrong tree.

func TestInitialize_WhenWorkspaceChanges_ShouldRebindSinks(t *testing.T) {
	first := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	second := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(first); err != nil {
		t.Fatalf("first Initialize: %v", err)
	}
	Get(CategoryKernel).Info("belongs to first workspace")

	if err := Initialize(second); err != nil {
		t.Fatalf("rebind Initialize: %v", err)
	}
	if got := BoundWorkspace(); got != second {
		t.Fatalf("BoundWorkspace = %q, want %q", got, second)
	}
	Get(CategoryKernel).Info("belongs to second workspace")
	CloseAll()

	firstKernel := readLog(t, first, "kernel")
	if strings.Contains(firstKernel, "belongs to second workspace") {
		t.Error("post-rebind line leaked into the previous workspace")
	}
	secondKernel := readLog(t, second, "kernel")
	if !strings.Contains(secondKernel, "belongs to second workspace") {
		t.Error("post-rebind line did not reach the new workspace")
	}
	if strings.Contains(secondKernel, "belongs to first workspace") {
		t.Error("pre-rebind line was duplicated into the new workspace")
	}
	// The rebind must be recorded where someone would look for it.
	if boot := readLog(t, second, "boot"); !strings.Contains(boot, "Logging rebound from workspace") {
		t.Error("expected the rebind to be noted in the new workspace's boot log")
	}
}

func TestInitialize_WhenSameWorkspaceRepeated_ShouldStayIdempotent(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	prefix := currentRunPrefix()

	// A relative path naming the same directory must not look like a rebind.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if rel, relErr := filepath.Rel(wd, ws); relErr == nil {
		if err := Initialize(rel); err != nil {
			t.Fatalf("relative Initialize: %v", err)
		}
	}
	if err := Initialize(ws); err != nil {
		t.Fatalf("repeat Initialize: %v", err)
	}
	if currentRunPrefix() != prefix {
		t.Error("re-initializing the same workspace started a new run prefix")
	}
}

func TestInitialize_WhenWorkspaceEmpty_ShouldError(t *testing.T) {
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(""); err == nil {
		t.Error("expected an error for an empty workspace")
	}
}

func TestApplyConfig_WhenInjectedBeforeInitialize_ShouldWinOverDiskConfig(t *testing.T) {
	// Disk says logging is off; boot says it is on. Boot has already parsed the
	// real config, so its answer is the authoritative one.
	ws := newWorkspace(t, `"debug_mode": false`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	ApplyConfig(Config{DebugMode: true, Level: "debug"})
	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if !IsDebugMode() {
		t.Fatal("injected config was overwritten by the on-disk config")
	}
	Get(CategoryKernel).Info("injected config wrote this")
	CloseAll()

	if !strings.Contains(readLog(t, ws, "kernel"), "injected config wrote this") {
		t.Error("expected a kernel log written under the injected config")
	}

	ClearInjectedConfig()
	if err := ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	if IsDebugMode() {
		t.Error("after ClearInjectedConfig the on-disk config must apply again")
	}
}
