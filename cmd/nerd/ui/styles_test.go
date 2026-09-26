package ui

import (
	"strings"
	"testing"
)

func TestDetectTheme(t *testing.T) {
	// NO_COLOR short-circuits DetectTheme before any other signal is read, and
	// it is commonly set in CI and agent shells. Without clearing it the first
	// two assertions test nothing but the ambient environment.
	resetCache := func() {
		themeMutex.Lock()
		cachedTheme = nil
		themeMutex.Unlock()
	}
	t.Cleanup(resetCache)

	t.Setenv("NO_COLOR", "")
	t.Setenv("COLORFGBG", "")

	resetCache()
	t.Setenv("CODENERD_DARK_MODE", "1")
	dark := DetectTheme()
	if !dark.IsDark() {
		t.Fatalf("expected dark theme when CODENERD_DARK_MODE=1")
	}

	resetCache()
	t.Setenv("NO_COLOR", "1")
	nocolor := DetectTheme()
	got, ok := nocolor.(BasicTheme)
	if !ok {
		t.Fatalf("expected BasicTheme when NO_COLOR is set, got %T", nocolor)
	}
	if got != (BasicTheme{}) {
		t.Fatalf("expected empty theme when NO_COLOR is set, got %+v", got)
	}
}

func TestThemeRenderer(t *testing.T) {
	// Light Theme
	light := LightTheme()
	rLight := light.Renderer(nil)
	if rLight.HasDarkBackground() {
		t.Errorf("expected light theme renderer to have light background")
	}

	// Dark Theme
	dark := DarkTheme()
	rDark := dark.Renderer(nil)
	if !rDark.HasDarkBackground() {
		t.Errorf("expected dark theme renderer to have dark background")
	}
}

func TestLogo(t *testing.T) {
	s := DefaultStyles()
	logo := Logo(s)
	if logo == "" {
		t.Error("Logo() returned empty string")
	}
	if len(logo) < 50 {
		t.Errorf("Logo() seems too short: %d chars", len(logo))
	}
	if strings.Count(logo, "\n") < 3 {
		t.Errorf("Logo() expected multiple lines, got %d", strings.Count(logo, "\n"))
	}
}
