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
