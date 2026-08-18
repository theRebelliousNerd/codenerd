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

	assert.Equal(t, customStyle, styles.Layout.App)

	defaultStyles := ui.NewStyles(theme)
	assert.Equal(t, defaultStyles.Layout.Header, styles.Layout.Header)
}
