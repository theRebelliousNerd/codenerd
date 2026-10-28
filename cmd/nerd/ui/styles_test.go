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

// TestLogoArt pins the logo's grid: six rows, nothing wider than 80 columns,
// and a blank gap where "code" ends and "NERD" begins.
func TestLogoArt(t *testing.T) {
	lines := strings.Split(strings.TrimRight(logoArt, "\n"), "\n")
	if len(lines) != 6 {
		t.Fatalf("logo has %d rows, want 6", len(lines))
	}
	for i, line := range lines {
		runes := []rune(line)
		if len(runes) > 80 {
			t.Errorf("row %d is %d columns, want <= 80", i, len(runes))
		}
		if len(runes) > logoNerdColumn && (runes[logoNerdColumn-1] != ' ' || runes[logoNerdColumn-2] != ' ') {
			t.Errorf("row %d has no gap before column %d: %q", i, logoNerdColumn, string(runes))
		}
	}
}
