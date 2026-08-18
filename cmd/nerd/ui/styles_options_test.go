package ui_test

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"

	"codenerd/cmd/nerd/ui"
)

func TestNewStylesFunctionalOptions(t *testing.T) {
	theme := ui.LightTheme()
	customStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))

	styles := ui.NewStyles(theme, ui.WithApp(customStyle))

	// Should have the custom style for App
	assert.Equal(t, customStyle, styles.App)

	// Other styles should be default
	defaultStyles := ui.NewStyles(theme)
	assert.Equal(t, defaultStyles.Header, styles.Header)
}
