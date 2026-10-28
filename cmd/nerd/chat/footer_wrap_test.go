package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The footer wraps to the terminal instead of being cut off: every line fits,
// and the hotkeys survive a narrow terminal.
func TestRenderFooter_WrapsToWidth(t *testing.T) {
	m := NewTestModel(WithSize(60, 40))
	footer := m.renderFooter()
	for _, line := range strings.Split(footer, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Fatalf("footer line is %d wide at width 60: %q", w, line)
		}
	}
	if !strings.Contains(footer, "/help") || !strings.Contains(footer, "Shift+Tab: mode") {
		t.Fatalf("hotkeys were cut off:\n%s", footer)
	}
}
