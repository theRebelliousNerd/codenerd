package ui

import (
	"github.com/charmbracelet/lipgloss"
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
	if !dark.IsDark {
		t.Fatalf("expected dark theme when CODENERD_DARK_MODE=1")
	}

	t.Setenv("NO_COLOR", "1")
	nocolor := DetectTheme()
	empty := Theme{}
	if nocolor != empty {
		t.Fatalf("expected empty theme when NO_COLOR is set, got %v", nocolor)
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

func TestAdjustColor(t *testing.T) {
	// Original: #101F38 -> H: 217.500000, S: 0.555556, L: 0.141176
	// With 1.5x saturation and lightness: #092b63

	tests := []struct {
		name        string
		color       string
		lightness   float64
		saturation  float64
		expectedHex string
	}{
		{
			name:        "Increase lightness and saturation",
			color:       "#101F38",
			lightness:   1.5,
			saturation:  1.5,
			expectedHex: "#092b63",
		},
		{
			name:        "Decrease lightness and saturation",
			color:       "#101F38",
			lightness:   0.5,
			saturation:  0.5,
			expectedHex: "#0d1117",
		},
		{
			name:        "Clamp upper bounds",
			color:       "#8BC34A",
			lightness:   5.0,
			saturation:  5.0,
			expectedHex: "#ffffff",
		},
		{
			name:        "Clamp lower bounds",
			color:       "#8BC34A",
			lightness:   -1.0,
			saturation:  -1.0,
			expectedHex: "#000000",
		},
		{
			name:        "Invalid color returns original",
			color:       "not-a-color",
			lightness:   1.5,
			saturation:  1.5,
			expectedHex: "not-a-color",
		},
		{
			name:        "Empty color returns original",
			color:       "",
			lightness:   1.5,
			saturation:  1.5,
			expectedHex: "",
		},
		{
			name:        "No change factors",
			color:       "#101F38",
			lightness:   1.0,
			saturation:  1.0,
			expectedHex: "#101f38",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AdjustColor(lipgloss.Color(tt.color), tt.lightness, tt.saturation)
			// Lowercase the result because colorful might use lowercase hex
			// whereas we might use uppercase in tests. AdjustColor returns the hex directly.
			// Let's compare strings directly and allow for case differences.
			if strings.ToLower(string(result)) != strings.ToLower(tt.expectedHex) {
				t.Errorf("AdjustColor() = %v, want %v", result, tt.expectedHex)
			}
		})
	}
}
