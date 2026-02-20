package ui

import "testing"

func TestDetectTheme(t *testing.T) {
	t.Setenv("CODENERD_DARK_MODE", "1")
	dark := DetectTheme()
	if !dark.IsDark {
		t.Fatalf("expected dark theme when CODENERD_DARK_MODE=1")
	}

	t.Setenv("CODENERD_DARK_MODE", "")
	light := DetectTheme()
	if light.IsDark {
		t.Fatalf("expected light theme when CODENERD_DARK_MODE is unset")
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
