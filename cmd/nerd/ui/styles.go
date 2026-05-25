// Package ui provides the visual styling for the codeNERD interactive CLI.
// Uses the official codeNERD brand color palette with light/dark mode support.
package ui

import (
	_ "embed"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
)

// Theme holds the current color scheme
// TODO: Create Theme interface for easier testing and swapping of themes.
type Theme struct {
	IsDark bool

	// Backgrounds
	Background   lipgloss.Color
	OnBackground lipgloss.Color

	// Surfaces (Cards, etc)
	Surface   lipgloss.Color
	OnSurface lipgloss.Color

	// Brand Colors
	Primary   lipgloss.Color
	OnPrimary lipgloss.Color

	Secondary   lipgloss.Color
	OnSecondary lipgloss.Color

	// Containers
	Container   lipgloss.Color
	OnContainer lipgloss.Color

	// Outlines/Borders
	Outline        lipgloss.Color
	OnSurfaceMuted lipgloss.Color
	// Semantic Colors
	Destructive lipgloss.Color
	Success     lipgloss.Color
	Warning     lipgloss.Color
	Info        lipgloss.Color

	// Chart Colors
	Chart1 lipgloss.Color
	Chart2 lipgloss.Color
	Chart3 lipgloss.Color
	Chart4 lipgloss.Color
	Chart5 lipgloss.Color
}

// Renderer returns a lipgloss.Renderer initialized with the theme's settings.
// It accepts an optional io.Writer, defaulting to os.Stdout if nil.
func (t Theme) Renderer(w io.Writer) *lipgloss.Renderer {
	if w == nil {
		w = os.Stdout
	}
	r := lipgloss.NewRenderer(w)
	r.SetHasDarkBackground(t.IsDark)
	return r
}

// LightTheme returns the light mode theme
func LightTheme() Theme {
	return Theme{
		IsDark:         false,
		Background:     lipgloss.Color("#f4f5f6"),
		OnBackground:   lipgloss.Color("#101F38"),
		Surface:        lipgloss.Color("#ffffff"),
		OnSurface:      lipgloss.Color("#101F38"),
		Primary:        lipgloss.Color("#101F38"),
		OnPrimary:      lipgloss.Color("#ffffff"),
		Secondary:      lipgloss.Color("#8BC34A"),
		OnSecondary:    lipgloss.Color("#ffffff"),
		Container:      lipgloss.Color("#e1e4e8"),
		OnContainer:    lipgloss.Color("#101F38"),
		Outline:        lipgloss.Color("#dce0e5"),
		OnSurfaceMuted: lipgloss.Color("#d6dae0"),
		Destructive:    lipgloss.Color("#e53935"),
		Success:        lipgloss.Color("#8BC34A"),
		Warning:        lipgloss.Color("#FFC107"),
		Info:           lipgloss.Color("#2196F3"),
		Chart1:         lipgloss.Color("#e57373"),
		Chart2:         lipgloss.Color("#4db6ac"),
		Chart3:         lipgloss.Color("#29434e"),
		Chart4:         lipgloss.Color("#ffd54f"),
		Chart5:         lipgloss.Color("#ff8a65"),
	}
}

// DarkTheme returns the dark mode theme
func DarkTheme() Theme {
	return Theme{
		IsDark:         true,
		Background:     lipgloss.Color("#141d2b"),
		OnBackground:   lipgloss.Color("#f2f2f2"),
		Surface:        lipgloss.Color("#1a2536"),
		OnSurface:      lipgloss.Color("#f2f2f2"),
		Primary:        lipgloss.Color("#8BC34A"),
		OnPrimary:      lipgloss.Color("#101F38"),
		Secondary:      lipgloss.Color("#101F38"),
		OnSecondary:    lipgloss.Color("#ffffff"),
		Container:      lipgloss.Color("#1e2a3d"),
		OnContainer:    lipgloss.Color("#8BC34A"),
		Outline:        lipgloss.Color("#2a3850"),
		OnSurfaceMuted: lipgloss.Color("#2a3850"),
		Destructive:    lipgloss.Color("#e53935"),
		Success:        lipgloss.Color("#8BC34A"),
		Warning:        lipgloss.Color("#FFC107"),
		Info:           lipgloss.Color("#2196F3"),
		Chart1:         lipgloss.Color("#e57373"),
		Chart2:         lipgloss.Color("#4db6ac"),
		Chart3:         lipgloss.Color("#29434e"),
		Chart4:         lipgloss.Color("#ffd54f"),
		Chart5:         lipgloss.Color("#ff8a65"),
	}
}

// DetectTheme auto-detects based on terminal or returns light mode
// TODO: Add support for a configuration file (e.g., config.yaml) in addition to environment variables.
// TODO: Consider using a dedicated library like 'termenv' for more robust background color detection.
// TODO: IMPROVEMENT: Cache the detected theme to avoid repeated environment lookups.
func DetectTheme() Theme {
	// Support NO_COLOR standard (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return Theme{}
	}

	// Check for common dark mode indicators
	colorTerm := os.Getenv("COLORFGBG")
	if colorTerm != "" {
		// Format is usually "foreground;background"
		// If background is dark (0-8), use dark theme.
		// If background is light (7-15), use light theme.
		parts := strings.Split(colorTerm, ";")
		if len(parts) == 2 {
			bgStr := parts[1]
			// Try to parse background color index
			// Standard ANSI colors: 0-7 are widely used for dark backgrounds
			if bgIdx, err := strconv.Atoi(bgStr); err == nil {
				// Simple heuristic: 0-6 and 8 (dark grey) are likely dark backgrounds
				if (bgIdx >= 0 && bgIdx <= 6) || bgIdx == 8 {
					return DarkTheme()
				}
			}
		}
	}

	// Check for explicit dark mode preference
	if os.Getenv("CODENERD_DARK_MODE") == "1" {
		return DarkTheme()
	}

	// Default to light mode as specified
	return LightTheme()
}

// Styles holds all the styled components
// TODO: Group related styles into sub-structs (e.g. TextStyles, LayoutStyles) for better API organization.
// TODO: IMPROVEMENT: Add a method to serialize/deserialize theme to JSON for user customization.
type Styles struct {
	Theme Theme

	// Layout
	App     lipgloss.Style
	Header  lipgloss.Style
	Footer  lipgloss.Style
	Content lipgloss.Style
	Sidebar lipgloss.Style

	// Text
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Body     lipgloss.Style
	Muted    lipgloss.Style
	Bold     lipgloss.Style

	// Interactive
	Prompt        lipgloss.Style
	PromptCursor  lipgloss.Style
	UserInput     lipgloss.Style
	AgentResponse lipgloss.Style

	// Status
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style

	// Code
	CodeBlock  lipgloss.Style
	InlineCode lipgloss.Style

	// Components
	Spinner     lipgloss.Style
	ProgressBar lipgloss.Style
	Divider     lipgloss.Style
	Badge       lipgloss.Style
}

// NewStyles creates a new Styles instance with the given theme
// TODO: Consider using a builder pattern or functional options for Styles configuration if complexity grows.
// TODO: IMPROVEMENT: Use functional options for Styles configuration.
func NewStyles(theme Theme) Styles {
	return Styles{
		Theme: theme,

		// Layout styles
		App: lipgloss.NewStyle().
			Background(theme.Background).
			Foreground(theme.OnBackground),

		Header: lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(theme.OnPrimary).
			Padding(0, 2).
			Bold(true),

		Footer: lipgloss.NewStyle().
			Foreground(theme.OnSurfaceMuted).
			Padding(0, 2),

		Content: lipgloss.NewStyle().
			Padding(1, 2),

		// Text styles
		Title: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(theme.OnSurfaceMuted).
			Italic(true),

		Body: lipgloss.NewStyle().
			Foreground(theme.OnSurface),

		Muted: lipgloss.NewStyle().
			Foreground(theme.OnSurfaceMuted),

		Bold: lipgloss.NewStyle().
			Foreground(theme.OnSurface).
			Bold(true),

		// Interactive styles
		Prompt: lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Bold(true),

		PromptCursor: lipgloss.NewStyle().
			Foreground(theme.Secondary).
			Background(theme.Secondary),

		UserInput: lipgloss.NewStyle().
			Foreground(theme.OnSurface),

		AgentResponse: lipgloss.NewStyle().
			Foreground(theme.OnSurface).
			PaddingLeft(2).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(theme.Secondary),

		// Status styles
		Success: lipgloss.NewStyle().
			Foreground(theme.Success).
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(theme.Destructive).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(theme.Warning).
			Bold(true),

		Info: lipgloss.NewStyle().
			Foreground(theme.Info),

		// Code styles
		CodeBlock: lipgloss.NewStyle().
			Background(theme.Surface).
			Foreground(theme.OnSurface).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Outline),

		InlineCode: lipgloss.NewStyle().
			Background(theme.Container).
			Foreground(theme.OnContainer).
			Padding(0, 1),

		// Component styles
		Spinner: lipgloss.NewStyle().
			Foreground(theme.Secondary),

		ProgressBar: lipgloss.NewStyle().
			Foreground(theme.Secondary),

		Divider: lipgloss.NewStyle().
			Foreground(theme.Outline),

		Badge: lipgloss.NewStyle().
			Background(theme.Secondary).
			Foreground(theme.OnSecondary).
			Padding(0, 1).
			Bold(true),
	}
}

// DefaultStyles returns styles with the default (light) theme
func DefaultStyles() Styles {
	return NewStyles(DetectTheme())
}

//go:embed logo.txt
var logoArt string

// Logo returns the codeNERD ASCII logo
func Logo(s Styles) string {
	return s.Title.Foreground(s.Theme.Primary).Render(logoArt)
}

// Divider returns a horizontal divider
func (s Styles) RenderDivider(width int) string {
	return s.Divider.Render(strings.Repeat("─", width))
}

// AdjustColor modifies the brightness and saturation of a lipgloss.Color.
// Note: This utility only supports adjusting hex color strings. ANSI color codes (e.g. "212") will be returned unmodified.
// Returns the original color if parsing fails.
// Factors > 1.0 increase the property, < 1.0 decrease it.
func AdjustColor(c lipgloss.Color, lightnessFactor float64, saturationFactor float64) lipgloss.Color {
	colorStr := string(c)
	if colorStr == "" {
		return c
	}

	col, err := colorful.Hex(colorStr)
	if err != nil {
		return c
	}

	h, s, l := col.Hsl()

	s = s * saturationFactor
	if s > 1.0 {
		s = 1.0
	} else if s < 0.0 {
		s = 0.0
	}

	l = l * lightnessFactor
	if l > 1.0 {
		l = 1.0
	} else if l < 0.0 {
		l = 0.0
	}

	newC := colorful.Hsl(h, s, l)
	return lipgloss.Color(newC.Hex())
}
