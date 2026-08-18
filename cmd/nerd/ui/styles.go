// Package ui provides the visual styling for the codeNERD interactive CLI.
// Uses the official codeNERD brand color palette with light/dark mode support.
package ui

import (
	_ "embed"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/muesli/termenv"

	"codenerd/internal/features"
)

// Theme is a color scheme. Concrete values are BasicTheme so tests can
// construct fakes and JSON can round-trip the shipped palettes.
type Theme interface {
	IsDark() bool
	Background() lipgloss.Color
	OnBackground() lipgloss.Color
	Surface() lipgloss.Color
	OnSurface() lipgloss.Color
	Primary() lipgloss.Color
	OnPrimary() lipgloss.Color
	Secondary() lipgloss.Color
	OnSecondary() lipgloss.Color
	Container() lipgloss.Color
	OnContainer() lipgloss.Color
	Outline() lipgloss.Color
	OnSurfaceMuted() lipgloss.Color
	Destructive() lipgloss.Color
	Success() lipgloss.Color
	Warning() lipgloss.Color
	Info() lipgloss.Color
	Chart1() lipgloss.Color
	Chart2() lipgloss.Color
	Chart3() lipgloss.Color
	Chart4() lipgloss.Color
	Chart5() lipgloss.Color
	Renderer(w io.Writer) *lipgloss.Renderer
	ToJSON() ([]byte, error)
}

// BasicTheme is the JSON-serializable shipped palette.
type BasicTheme struct {
	dark           bool
	background     lipgloss.Color
	onBackground   lipgloss.Color
	surface        lipgloss.Color
	onSurface      lipgloss.Color
	primary        lipgloss.Color
	onPrimary      lipgloss.Color
	secondary      lipgloss.Color
	onSecondary    lipgloss.Color
	container      lipgloss.Color
	onContainer    lipgloss.Color
	outline        lipgloss.Color
	onSurfaceMuted lipgloss.Color
	destructive    lipgloss.Color
	success        lipgloss.Color
	warning        lipgloss.Color
	info           lipgloss.Color
	chart1         lipgloss.Color
	chart2         lipgloss.Color
	chart3         lipgloss.Color
	chart4         lipgloss.Color
	chart5         lipgloss.Color
}

func (t BasicTheme) IsDark() bool                 { return t.dark }
func (t BasicTheme) Background() lipgloss.Color   { return t.background }
func (t BasicTheme) OnBackground() lipgloss.Color { return t.onBackground }
func (t BasicTheme) Surface() lipgloss.Color      { return t.surface }
func (t BasicTheme) OnSurface() lipgloss.Color    { return t.onSurface }
func (t BasicTheme) Primary() lipgloss.Color      { return t.primary }
func (t BasicTheme) OnPrimary() lipgloss.Color    { return t.onPrimary }
func (t BasicTheme) Secondary() lipgloss.Color    { return t.secondary }
func (t BasicTheme) OnSecondary() lipgloss.Color  { return t.onSecondary }
func (t BasicTheme) Container() lipgloss.Color    { return t.container }
func (t BasicTheme) OnContainer() lipgloss.Color  { return t.onContainer }
func (t BasicTheme) Outline() lipgloss.Color      { return t.outline }
func (t BasicTheme) OnSurfaceMuted() lipgloss.Color {
	return t.onSurfaceMuted
}
func (t BasicTheme) Destructive() lipgloss.Color { return t.destructive }
func (t BasicTheme) Success() lipgloss.Color     { return t.success }
func (t BasicTheme) Warning() lipgloss.Color     { return t.warning }
func (t BasicTheme) Info() lipgloss.Color        { return t.info }
func (t BasicTheme) Chart1() lipgloss.Color      { return t.chart1 }
func (t BasicTheme) Chart2() lipgloss.Color      { return t.chart2 }
func (t BasicTheme) Chart3() lipgloss.Color      { return t.chart3 }
func (t BasicTheme) Chart4() lipgloss.Color      { return t.chart4 }
func (t BasicTheme) Chart5() lipgloss.Color      { return t.chart5 }

type themeJSON struct {
	IsDark         bool   `json:"is_dark"`
	Background     string `json:"background"`
	OnBackground   string `json:"on_background"`
	Surface        string `json:"surface"`
	OnSurface      string `json:"on_surface"`
	Primary        string `json:"primary"`
	OnPrimary      string `json:"on_primary"`
	Secondary      string `json:"secondary"`
	OnSecondary    string `json:"on_secondary"`
	Container      string `json:"container"`
	OnContainer    string `json:"on_container"`
	Outline        string `json:"outline"`
	OnSurfaceMuted string `json:"on_surface_muted"`
	Destructive    string `json:"destructive"`
	Success        string `json:"success"`
	Warning        string `json:"warning"`
	Info           string `json:"info"`
	Chart1         string `json:"chart_1"`
	Chart2         string `json:"chart_2"`
	Chart3         string `json:"chart_3"`
	Chart4         string `json:"chart_4"`
	Chart5         string `json:"chart_5"`
}

func (t BasicTheme) toDTO() themeJSON {
	return themeJSON{
		IsDark:         t.dark,
		Background:     string(t.background),
		OnBackground:   string(t.onBackground),
		Surface:        string(t.surface),
		OnSurface:      string(t.onSurface),
		Primary:        string(t.primary),
		OnPrimary:      string(t.onPrimary),
		Secondary:      string(t.secondary),
		OnSecondary:    string(t.onSecondary),
		Container:      string(t.container),
		OnContainer:    string(t.onContainer),
		Outline:        string(t.outline),
		OnSurfaceMuted: string(t.onSurfaceMuted),
		Destructive:    string(t.destructive),
		Success:        string(t.success),
		Warning:        string(t.warning),
		Info:           string(t.info),
		Chart1:         string(t.chart1),
		Chart2:         string(t.chart2),
		Chart3:         string(t.chart3),
		Chart4:         string(t.chart4),
		Chart5:         string(t.chart5),
	}
}

func (t *BasicTheme) fromDTO(d themeJSON) {
	t.dark = d.IsDark
	t.background = lipgloss.Color(d.Background)
	t.onBackground = lipgloss.Color(d.OnBackground)
	t.surface = lipgloss.Color(d.Surface)
	t.onSurface = lipgloss.Color(d.OnSurface)
	t.primary = lipgloss.Color(d.Primary)
	t.onPrimary = lipgloss.Color(d.OnPrimary)
	t.secondary = lipgloss.Color(d.Secondary)
	t.onSecondary = lipgloss.Color(d.OnSecondary)
	t.container = lipgloss.Color(d.Container)
	t.onContainer = lipgloss.Color(d.OnContainer)
	t.outline = lipgloss.Color(d.Outline)
	t.onSurfaceMuted = lipgloss.Color(d.OnSurfaceMuted)
	t.destructive = lipgloss.Color(d.Destructive)
	t.success = lipgloss.Color(d.Success)
	t.warning = lipgloss.Color(d.Warning)
	t.info = lipgloss.Color(d.Info)
	t.chart1 = lipgloss.Color(d.Chart1)
	t.chart2 = lipgloss.Color(d.Chart2)
	t.chart3 = lipgloss.Color(d.Chart3)
	t.chart4 = lipgloss.Color(d.Chart4)
	t.chart5 = lipgloss.Color(d.Chart5)
}

// ToJSON serializes the theme to JSON.
func (t BasicTheme) ToJSON() ([]byte, error) {
	return json.MarshalIndent(t.toDTO(), "", "  ")
}

// FromJSON deserializes the theme from JSON.
func (t *BasicTheme) FromJSON(data []byte) error {
	var dto themeJSON
	if err := json.Unmarshal(data, &dto); err != nil {
		return err
	}
	t.fromDTO(dto)
	return nil
}

// Renderer returns a lipgloss.Renderer initialized with the theme's settings.
// It accepts an optional io.Writer, defaulting to os.Stdout if nil.
func (t BasicTheme) Renderer(w io.Writer) *lipgloss.Renderer {
	if w == nil {
		w = os.Stdout
	}
	r := lipgloss.NewRenderer(w)
	r.SetHasDarkBackground(t.dark)
	return r
}

// LightTheme returns the light mode theme
func LightTheme() Theme {
	return BasicTheme{
		dark:           false,
		background:     lipgloss.Color("#f4f5f6"),
		onBackground:   lipgloss.Color("#101F38"),
		surface:        lipgloss.Color("#ffffff"),
		onSurface:      lipgloss.Color("#101F38"),
		primary:        lipgloss.Color("#101F38"),
		onPrimary:      lipgloss.Color("#ffffff"),
		secondary:      lipgloss.Color("#8BC34A"),
		onSecondary:    lipgloss.Color("#ffffff"),
		container:      lipgloss.Color("#e1e4e8"),
		onContainer:    lipgloss.Color("#101F38"),
		outline:        lipgloss.Color("#dce0e5"),
		onSurfaceMuted: lipgloss.Color("#d6dae0"),
		destructive:    lipgloss.Color("#e53935"),
		success:        lipgloss.Color("#8BC34A"),
		warning:        lipgloss.Color("#FFC107"),
		info:           lipgloss.Color("#2196F3"),
		chart1:         lipgloss.Color("#e57373"),
		chart2:         lipgloss.Color("#4db6ac"),
		chart3:         lipgloss.Color("#29434e"),
		chart4:         lipgloss.Color("#ffd54f"),
		chart5:         lipgloss.Color("#ff8a65"),
	}
}

// DarkTheme returns the dark mode theme
func DarkTheme() Theme {
	return BasicTheme{
		dark:           true,
		background:     lipgloss.Color("#141d2b"),
		onBackground:   lipgloss.Color("#f2f2f2"),
		surface:        lipgloss.Color("#1a2536"),
		onSurface:      lipgloss.Color("#f2f2f2"),
		primary:        lipgloss.Color("#8BC34A"),
		onPrimary:      lipgloss.Color("#101F38"),
		secondary:      lipgloss.Color("#101F38"),
		onSecondary:    lipgloss.Color("#ffffff"),
		container:      lipgloss.Color("#1e2a3d"),
		onContainer:    lipgloss.Color("#8BC34A"),
		outline:        lipgloss.Color("#2a3850"),
		onSurfaceMuted: lipgloss.Color("#2a3850"),
		destructive:    lipgloss.Color("#e53935"),
		success:        lipgloss.Color("#8BC34A"),
		warning:        lipgloss.Color("#FFC107"),
		info:           lipgloss.Color("#2196F3"),
		chart1:         lipgloss.Color("#e57373"),
		chart2:         lipgloss.Color("#4db6ac"),
		chart3:         lipgloss.Color("#29434e"),
		chart4:         lipgloss.Color("#ffd54f"),
		chart5:         lipgloss.Color("#ff8a65"),
	}
}

var (
	cachedTheme Theme
	themeMutex  sync.RWMutex
)

// DetectTheme auto-detects based on terminal or returns light mode
// TODO: Add support for a configuration file (e.g., config.yaml) in addition to environment variables.
func DetectTheme() Theme {
	themeMutex.RLock()
	if cachedTheme != nil {
		t := cachedTheme
		themeMutex.RUnlock()
		return t
	}
	themeMutex.RUnlock()

	theme := detectTheme()

	themeMutex.Lock()
	cachedTheme = theme
	themeMutex.Unlock()

	return theme
}

func detectTheme() Theme {
	// Support NO_COLOR standard (https://no-color.org/)
	if os.Getenv("NO_COLOR") != "" {
		return BasicTheme{}
	}

	// Check for explicit dark mode preference. Resolved via internal/features
	// so .nerd/config.json's `features.dark_mode` and the legacy
	// CODENERD_DARK_MODE env var both work (env wins).
	if features.IsDarkModeEnabled() {
		return DarkTheme()
	}

	// Use termenv for robust background color detection
	if termenv.HasDarkBackground() {
		return DarkTheme()
	}
	// Default to light mode as specified
	return LightTheme()
}

type LayoutStyles struct {
	App     lipgloss.Style
	Header  lipgloss.Style
	Footer  lipgloss.Style
	Content lipgloss.Style
	Sidebar lipgloss.Style
}

type TextStyles struct {
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Body     lipgloss.Style
	Muted    lipgloss.Style
	Bold     lipgloss.Style
}

type InteractiveStyles struct {
	Prompt        lipgloss.Style
	PromptCursor  lipgloss.Style
	UserInput     lipgloss.Style
	AgentResponse lipgloss.Style
}

type StatusStyles struct {
	Success lipgloss.Style
	Error   lipgloss.Style
	Warning lipgloss.Style
	Info    lipgloss.Style
}

type CodeStyles struct {
	CodeBlock  lipgloss.Style
	InlineCode lipgloss.Style
}

type ComponentStyles struct {
	Spinner     lipgloss.Style
	ProgressBar lipgloss.Style
	Divider     lipgloss.Style
	Badge       lipgloss.Style
}

// Styles holds all the styled components
type Styles struct {
	Theme Theme

	Layout      LayoutStyles
	Text        TextStyles
	Interactive InteractiveStyles
	Status      StatusStyles
	Code        CodeStyles
	Components  ComponentStyles
}

// StyleOption represents a functional option for configuring Styles.
type StyleOption func(*Styles)

// NewStyles creates a new Styles instance with the given theme.
func NewStyles(theme Theme, opts ...StyleOption) Styles {
	s := Styles{
		Theme: theme,

		Layout: LayoutStyles{
			App: lipgloss.NewStyle().
				Background(theme.Background()).
				Foreground(theme.OnBackground()),

			Header: lipgloss.NewStyle().
				Background(theme.Primary()).
				Foreground(theme.OnPrimary()).
				Padding(0, 2).
				Bold(true),

			Footer: lipgloss.NewStyle().
				Foreground(theme.OnSurfaceMuted()).
				Padding(0, 2),

			Content: lipgloss.NewStyle().
				Padding(1, 2),
		},

		Text: TextStyles{
			Title: lipgloss.NewStyle().
				Foreground(theme.Primary()).
				Bold(true).
				MarginBottom(1),

			Subtitle: lipgloss.NewStyle().
				Foreground(theme.OnSurfaceMuted()).
				Italic(true),

			Body: lipgloss.NewStyle().
				Foreground(theme.OnSurface()),

			Muted: lipgloss.NewStyle().
				Foreground(theme.OnSurfaceMuted()),

			Bold: lipgloss.NewStyle().
				Foreground(theme.OnSurface()).
				Bold(true),
		},

		Interactive: InteractiveStyles{
			Prompt: lipgloss.NewStyle().
				Foreground(theme.Secondary()).
				Bold(true),

			PromptCursor: lipgloss.NewStyle().
				Foreground(theme.Secondary()).
				Background(theme.Secondary()),

			UserInput: lipgloss.NewStyle().
				Foreground(theme.OnSurface()),

			AgentResponse: lipgloss.NewStyle().
				Foreground(theme.OnSurface()).
				PaddingLeft(2).
				BorderLeft(true).
				BorderStyle(lipgloss.ThickBorder()).
				BorderForeground(theme.Secondary()),
		},

		Status: StatusStyles{
			Success: lipgloss.NewStyle().
				Foreground(theme.Success()).
				Bold(true),

			Error: lipgloss.NewStyle().
				Foreground(theme.Destructive()).
				Bold(true),

			Warning: lipgloss.NewStyle().
				Foreground(theme.Warning()).
				Bold(true),

			Info: lipgloss.NewStyle().
				Foreground(theme.Info()),
		},

		Code: CodeStyles{
			CodeBlock: lipgloss.NewStyle().
				Background(theme.Surface()).
				Foreground(theme.OnSurface()).
				Padding(1, 2).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.Outline()),

			InlineCode: lipgloss.NewStyle().
				Background(theme.Container()).
				Foreground(theme.OnContainer()).
				Padding(0, 1),
		},

		Components: ComponentStyles{
			Spinner: lipgloss.NewStyle().
				Foreground(theme.Secondary()),

			ProgressBar: lipgloss.NewStyle().
				Foreground(theme.Secondary()),

			Divider: lipgloss.NewStyle().
				Foreground(theme.Outline()),

			Badge: lipgloss.NewStyle().
				Background(theme.Secondary()).
				Foreground(theme.OnSecondary()).
				Padding(0, 1).
				Bold(true),
		},
	}

	for _, opt := range opts {
		opt(&s)
	}

	return s
}

// DefaultStyles returns styles with the default (light) theme
func DefaultStyles() Styles {
	return NewStyles(DetectTheme())
}

//go:embed logo.txt
var logoArt string

// Logo returns the codeNERD ASCII logo
func Logo(s Styles) string {
	return s.Text.Title.Foreground(s.Theme.Primary()).Render(logoArt)
}

// Divider returns a horizontal divider
func (s Styles) RenderDivider(width int) string {
	return s.Components.Divider.Render(strings.Repeat("─", width))
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

// WithApp configures the App style
func WithApp(style lipgloss.Style) StyleOption {
	return func(s *Styles) {
		s.Layout.App = style
	}
}

// WithHeader configures the Header style
func WithHeader(style lipgloss.Style) StyleOption {
	return func(s *Styles) {
		s.Layout.Header = style
	}
}

// WithFooter configures the Footer style
func WithFooter(style lipgloss.Style) StyleOption {
	return func(s *Styles) {
		s.Layout.Footer = style
	}
}

// WithContent configures the Content style
func WithContent(style lipgloss.Style) StyleOption {
	return func(s *Styles) {
		s.Layout.Content = style
	}
}

// WithSidebar configures the Sidebar style
func WithSidebar(style lipgloss.Style) StyleOption {
	return func(s *Styles) {
		s.Layout.Sidebar = style
	}
}
