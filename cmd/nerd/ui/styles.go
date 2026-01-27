// Package ui provides the visual styling for the codeNERD interactive CLI.
// Uses the official codeNERD brand color palette with light/dark mode support.
package ui

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette based on codeNERD brand guidelines
// TODO: Refactor these global variables into a structured theme definition or configuration to avoid global state.
// TODO: Add support for high-contrast accessibility mode in the color palette.
// TODO: IMPROVEMENT: Move global color variables to a Theme struct.
var (
	// Light Mode Colors (Default)
	LightBackground = lipgloss.Color("#f4f5f6") // hsl(200, 7%, 96%)
	LightForeground = lipgloss.Color("#101F38") // Dark Blue - hsl(220, 58%, 14%)
	LightPrimary    = lipgloss.Color("#101F38") // Dark Blue
	LightAccent     = lipgloss.Color("#8BC34A") // Lime Green - hsl(88, 50%, 60%)
	LightSecondary  = lipgloss.Color("#e1e4e8") // hsl(220, 15%, 90%)
	LightMuted      = lipgloss.Color("#d6dae0") // hsl(220, 15%, 85%)
	LightBorder     = lipgloss.Color("#dce0e5") // hsl(220, 15%, 88%)
	LightCard       = lipgloss.Color("#ffffff") // White

	// Dark Mode Colors
	DarkBackground = lipgloss.Color("#141d2b") // hsl(220, 58%, 10%)
	DarkForeground = lipgloss.Color("#f2f2f2") // hsl(0, 0%, 95%)
	DarkPrimary    = lipgloss.Color("#8BC34A") // Lime Green (flipped)
	DarkAccent     = lipgloss.Color("#101F38") // Dark Blue (flipped)
	DarkSecondary  = lipgloss.Color("#1e2a3d") // Darker blue
	DarkMuted      = lipgloss.Color("#2a3850") // Muted dark
	DarkBorder     = lipgloss.Color("#2a3850") // Border dark
	DarkCard       = lipgloss.Color("#1a2536") // Card dark - hsl(220, 50%, 15%)

	// Semantic Colors (same in both modes)
	Destructive = lipgloss.Color("#e53935") // Red - hsl(0, 84.2%, 60.2%)
	Success     = lipgloss.Color("#8BC34A") // Lime Green
	Warning     = lipgloss.Color("#FFC107") // Yellow
	Info        = lipgloss.Color("#2196F3") // Blue

	// Chart Colors
	Chart1 = lipgloss.Color("#e57373") // Orange hsl(12, 76%, 61%)
	Chart2 = lipgloss.Color("#4db6ac") // Teal hsl(173, 58%, 39%)
	Chart3 = lipgloss.Color("#29434e") // Dark Blue hsl(197, 37%, 24%)
	Chart4 = lipgloss.Color("#ffd54f") // Yellow hsl(43, 74%, 66%)
	Chart5 = lipgloss.Color("#ff8a65") // Orange-Red hsl(27, 87%, 67%)
)

// Theme holds the current color scheme
type Theme struct {
	Background lipgloss.Color
	Foreground lipgloss.Color
	Primary    lipgloss.Color
	Accent     lipgloss.Color
	Secondary  lipgloss.Color
	Muted      lipgloss.Color
	Border     lipgloss.Color
	Card       lipgloss.Color
	IsDark     bool
}

// LightTheme returns the light mode theme
func LightTheme() Theme {
	return Theme{
		Background: LightBackground,
		Foreground: LightForeground,
		Primary:    LightPrimary,
		Accent:     LightAccent,
		Secondary:  LightSecondary,
		Muted:      LightMuted,
		Border:     LightBorder,
		Card:       LightCard,
		IsDark:     false,
	}
}

// DarkTheme returns the dark mode theme
func DarkTheme() Theme {
	return Theme{
		Background: DarkBackground,
		Foreground: DarkForeground,
		Primary:    DarkPrimary,
		Accent:     DarkAccent,
		Secondary:  DarkSecondary,
		Muted:      DarkMuted,
		Border:     DarkBorder,
		Card:       DarkCard,
		IsDark:     true,
	}
}

// DetectTheme auto-detects based on terminal or returns light mode
// TODO: Add support for a configuration file (e.g., config.yaml) in addition to environment variables.
// TODO: Consider using a dedicated library like 'termenv' for more robust background color detection.
// TODO: IMPROVEMENT: Use `muesli/termenv` for robust theme detection.
func DetectTheme() Theme {
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
// TODO: Add utility function to adjust color brightness/saturation
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
			Foreground(theme.Foreground),

		Header: lipgloss.NewStyle().
			Background(theme.Primary).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(0, 2).
			Bold(true),

		Footer: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(0, 2),

		Content: lipgloss.NewStyle().
			Padding(1, 2),

		// Text styles
		Title: lipgloss.NewStyle().
			Foreground(theme.Primary).
			Bold(true).
			MarginBottom(1),

		Subtitle: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true),

		Body: lipgloss.NewStyle().
			Foreground(theme.Foreground),

		Muted: lipgloss.NewStyle().
			Foreground(theme.Muted),

		Bold: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			Bold(true),

		// Interactive styles
		Prompt: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Bold(true),

		PromptCursor: lipgloss.NewStyle().
			Foreground(theme.Accent).
			Background(theme.Accent),

		UserInput: lipgloss.NewStyle().
			Foreground(theme.Foreground),

		AgentResponse: lipgloss.NewStyle().
			Foreground(theme.Foreground).
			PaddingLeft(2).
			BorderLeft(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(theme.Accent),

		// Status styles
		Success: lipgloss.NewStyle().
			Foreground(Success).
			Bold(true),

		Error: lipgloss.NewStyle().
			Foreground(Destructive).
			Bold(true),

		Warning: lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true),

		Info: lipgloss.NewStyle().
			Foreground(Info),

		// Code styles
		CodeBlock: lipgloss.NewStyle().
			Background(theme.Card).
			Foreground(theme.Foreground).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border),

		InlineCode: lipgloss.NewStyle().
			Background(theme.Secondary).
			Foreground(theme.Primary).
			Padding(0, 1),

		// Component styles
		Spinner: lipgloss.NewStyle().
			Foreground(theme.Accent),

		ProgressBar: lipgloss.NewStyle().
			Foreground(theme.Accent),

		Divider: lipgloss.NewStyle().
			Foreground(theme.Border),

		Badge: lipgloss.NewStyle().
			Background(theme.Accent).
			Foreground(lipgloss.Color("#ffffff")).
			Padding(0, 1).
			Bold(true),
	}
}

// DefaultStyles returns styles with the default (light) theme
func DefaultStyles() Styles {
	return NewStyles(DetectTheme())
}

// Logo returns the codeNERD ASCII logo
func Logo(s Styles) string {
	// TODO: Extract this hardcoded ASCII art to a resource file or constant to declutter the code.
	// TODO: IMPROVEMENT: Load ASCII art from a separate resource file.
	logo := `
   ___          _      _  _ ___ ___ ___  
  / __|___  __| |___ | \| | __| _ \   \ 
 | (__/ _ \/ _` + "`" + ` / -_)| .` + "`" + ` | _||   / |) |
  \___\___/\__,_\___||_|\_|___|_|_\___/ 
`
	return s.Title.Foreground(s.Theme.Primary).Render(logo)
}

// Divider returns a horizontal divider
func (s Styles) RenderDivider(width int) string {
	return s.Divider.Render(strings.Repeat("─", width))
}
