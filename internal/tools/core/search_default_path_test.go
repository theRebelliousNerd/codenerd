package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecuteGrep_DefaultPath_SkipsHiddenButFindsVisible verifies the fix for
// the "." walk-root bug. The walk callback previously skipped any directory
// whose name starts with ".", and filepath.Walk invokes the callback for the
// root itself first with info.Name() == ".". When path defaults to "." that
// aborted the entire walk before any file was visited, making every default-
// path search return zero matches. The fix gates the hidden-directory skip on
// p != path so that "." and "./" walk normally while nested dot-directories
// such as .git and .nerd are still skipped.
func TestExecuteGrep_DefaultPath_SkipsHiddenButFindsVisible(t *testing.T) {
	// Do not run in parallel: this test chdirs globally.
	tmpDir := t.TempDir()

	const visibleToken = "UNIQUE_VISIBLE_TOKEN_abc123_789"
	const hiddenToken = "UNIQUE_HIDDEN_TOKEN_xyz789_012"

	visibleFile := filepath.Join(tmpDir, "visible.txt")
	if err := os.WriteFile(visibleFile, []byte("hello "+visibleToken+" world\n"), 0600); err != nil {
		t.Fatalf("write visible file: %v", err)
	}

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.Mkdir(hiddenDir, 0755); err != nil {
		t.Fatalf("mkdir .hidden: %v", err)
	}
	hiddenFile := filepath.Join(hiddenDir, "hidden.txt")
	if err := os.WriteFile(hiddenFile, []byte("secret "+hiddenToken+" inside hidden\n"), 0600); err != nil {
		t.Fatalf("write hidden file: %v", err)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir to tmpDir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()

	ctx := context.Background()

	// Search with NO path argument (defaults to ".") for the visible token.
	// Must be found.
	resultVisible, err := executeGrep(ctx, map[string]any{
		"pattern": visibleToken,
	})
	if err != nil {
		t.Fatalf("executeGrep visible (default path): %v", err)
	}
	if !strings.Contains(resultVisible, visibleToken) {
		t.Errorf("expected visible token %q to be found via default path \".\", got %q", visibleToken, resultVisible)
	}
	if strings.Contains(resultVisible, ".hidden") {
		t.Errorf("visible search should not return hidden path, got %q", resultVisible)
	}

	// Search with NO path argument for the hidden token.
	// Must NOT be found because .hidden is skipped.
	resultHidden, err := executeGrep(ctx, map[string]any{
		"pattern": hiddenToken,
	})
	if err != nil {
		t.Fatalf("executeGrep hidden (default path): %v", err)
	}
	if !strings.Contains(resultHidden, "No matches found") {
		t.Errorf("expected hidden token NOT to be found (hidden dir should be skipped), got %q", resultHidden)
	}
	if strings.Contains(resultHidden, ".hidden") || strings.Contains(resultHidden, "hidden.txt") {
		t.Errorf("hidden dir should be skipped, got %q", resultHidden)
	}
}
