package ui

import (
	"strings"
	"testing"
)

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

func TestLogo(t *testing.T) {
	styles := DefaultStyles()
	logo := Logo(styles)
	if len(logo) == 0 {
		t.Error("Logo returned empty string")
	}
	// Check for a substring from the logo
	if !strings.Contains(logo, "___") {
		t.Error("Logo does not contain expected ASCII art")
	}
}
