package chat

import (
	"strings"
	"testing"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/campaign"
	"codenerd/internal/config"

	"charm.land/glamour/v2"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// layout_refit_test.go pins the invariant applyLayout exists for: whatever
// sidebar is open — logic split pane, campaign panel, both, neither — the
// chat is wrapped to the column it actually renders in, every line of the
// frame fits the terminal, and text survives a shrink by rewrapping, not
// by being clipped at the viewport edge.
//
// Measured before the fix (2026-10-27 probe): a 252-column user line in a
// 156-column viewport at 160; rendering still 252 wide after resizing to
// 80 (cache keyed by message index, never invalidated on width change);
// 150-column frame lines at a 120-column terminal with the campaign
// sidebar open (joined beside a full-width chat, width never reserved).

const wrapProbeProse = "The kernel derives routing decisions from perception facts and the shards execute them under constitutional constraints while embeddings index every knowledge atom for later semantic recall across the whole workspace and beyond simple keyword matching."

// newLayoutModel is a test model with the production layout set wired
// (renderer, split pane, list, side pages), since applyLayout sizes them.
func newLayoutModel(t *testing.T) Model {
	t.Helper()
	m := NewTestModel(WithHistory(
		Message{Role: "user", Content: wrapProbeProse},
		Message{Role: "assistant", Content: wrapProbeProse + "\n\n" + wrapProbeProse},
	))
	renderer, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(80))
	if err != nil {
		t.Fatal(err)
	}
	m.renderer = renderer
	sp := ui.NewSplitPaneViewWithRatio(m.styles, 80, 24, ui.DefaultSplitRatio)
	m.splitPane = &sp
	m.logicPane = sp.RightPane
	m.list = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.usagePage = ui.NewUsagePageModel(nil, m.styles)
	m.campaignPage = ui.NewCampaignPageModel()
	m.autoPage = ui.NewAutopoiesisPageModel()
	if m.Config == nil {
		m.Config = config.DefaultUserConfig()
	}
	return m
}

func layoutResize(t *testing.T, m Model, w, h int) Model {
	t.Helper()
	upd, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return upd.(Model)
}

func layoutKey(t *testing.T, m Model, r rune) Model {
	t.Helper()
	upd, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true})
	return upd.(Model)
}

func maxFrameLineWidth(view string) int {
	max := 0
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > max {
			max = w
		}
	}
	return max
}

// assertWrappedToColumn is the core invariant: the rendered history fits
// the viewport it renders in (nothing clipped), and the full frame fits
// the terminal.
func assertWrappedToColumn(t *testing.T, m Model, frameWidth int, state string) {
	t.Helper()
	if got := maxFrameLineWidth(m.renderHistory()); got > m.viewport.Width {
		t.Errorf("%s: rendered history line is %d wide in a %d-wide viewport (clipped, not wrapped)", state, got, m.viewport.Width)
	}
	if got := maxFrameLineWidth(m.View()); got > frameWidth {
		t.Errorf("%s: frame line is %d wide at a %d-wide terminal", state, got, frameWidth)
	}
	// The prose tail must survive wrapping: a clip would drop it.
	if !strings.Contains(m.renderHistory(), "keyword") || !strings.Contains(m.renderHistory(), "matching") {
		t.Errorf("%s: prose tail lost from rendered history (clipped instead of wrapped)", state)
	}
	if got := len(strings.Split(strings.TrimRight(m.View(), "\n"), "\n")); got > m.height {
		t.Errorf("%s: frame is %d lines tall at a %d-line terminal", state, got, m.height)
	}
}

func TestLayout_ResizeRewrapsHistory(t *testing.T) {
	m := newLayoutModel(t)
	m = layoutResize(t, m, 160, 40)
	assertWrappedToColumn(t, m, 160, "160 wide")

	wideViewport := m.viewport.Width
	m = layoutResize(t, m, 70, 24)
	if m.viewport.Width >= wideViewport {
		t.Fatalf("viewport did not shrink on resize: %d -> %d", wideViewport, m.viewport.Width)
	}
	assertWrappedToColumn(t, m, 70, "70 wide after shrink")

	m = layoutResize(t, m, 200, 50)
	assertWrappedToColumn(t, m, 200, "200 wide after grow")
}

func TestLayout_LogicSidebarRefitsAndInputStaysFullWidth(t *testing.T) {
	m := newLayoutModel(t)
	m = layoutResize(t, m, 120, 40)
	plainViewport := m.viewport.Width
	plainInput := m.textarea.Width()

	m = layoutKey(t, m, 'l') // Alt+L opens the logic split pane
	if !m.showLogic {
		t.Fatal("Alt+L did not open the logic pane")
	}
	if m.viewport.Width >= plainViewport {
		t.Errorf("logic sidebar open but viewport did not shrink: %d -> %d", plainViewport, m.viewport.Width)
	}
	// The input stacks below the split, across the full frame.
	if m.textarea.Width() != plainInput {
		t.Errorf("textarea width changed with the sidebar: %d -> %d", plainInput, m.textarea.Width())
	}
	assertWrappedToColumn(t, m, 120, "logic sidebar open")

	m = layoutKey(t, m, 'l') // close it again
	if m.showLogic {
		t.Fatal("Alt+L did not close the logic pane")
	}
	if m.viewport.Width != plainViewport {
		t.Errorf("viewport not restored after closing sidebar: %d, want %d", m.viewport.Width, plainViewport)
	}
	assertWrappedToColumn(t, m, 120, "logic sidebar closed")
}

func TestLayout_PaneModeCycleRefits(t *testing.T) {
	m := newLayoutModel(t)
	m = layoutResize(t, m, 120, 40)
	plainViewport := m.viewport.Width

	m = layoutKey(t, m, 'g') // Alt+G: single -> split
	if m.paneMode != ui.ModeSplitPane {
		t.Fatalf("paneMode = %v, want split", m.paneMode)
	}
	if m.viewport.Width >= plainViewport {
		t.Errorf("split via Alt+G did not refit the viewport: %d", m.viewport.Width)
	}
	assertWrappedToColumn(t, m, 120, "split via Alt+G")
}

func TestLayout_CampaignSidebarReservesWidth(t *testing.T) {
	m := newLayoutModel(t)
	m = layoutResize(t, m, 120, 40)
	plainViewport := m.viewport.Width

	// Campaign start path: panel appears with no terminal resize.
	m.activeCampaign = &campaign.Campaign{Goal: "Refactor auth", Title: "Auth refactor", TotalTasks: 10, CompletedTasks: 3}
	m.showCampaignPanel = true
	m = m.applyLayout()
	if !m.campaignBeside {
		t.Fatal("campaign panel should fit beside the chat at 120 wide")
	}
	if m.viewport.Width >= plainViewport {
		t.Errorf("campaign sidebar open but viewport did not shrink: %d -> %d", plainViewport, m.viewport.Width)
	}
	assertWrappedToColumn(t, m, 120, "campaign sidebar open")

	// Alt+C toggles it away again through the same refit.
	m = layoutKey(t, m, 'c')
	if m.showCampaignPanel {
		t.Fatal("Alt+C did not close the campaign panel")
	}
	if m.viewport.Width != plainViewport {
		t.Errorf("viewport not restored after campaign close: %d, want %d", m.viewport.Width, plainViewport)
	}
	assertWrappedToColumn(t, m, 120, "campaign sidebar closed")
}

func TestLayout_BothSidebarsAndNarrowStacking(t *testing.T) {
	m := newLayoutModel(t)
	m = layoutResize(t, m, 160, 44)
	m = layoutKey(t, m, 'l')
	m.activeCampaign = &campaign.Campaign{Goal: "Refactor auth", Title: "Auth refactor", TotalTasks: 10, CompletedTasks: 3}
	m.showCampaignPanel = true
	m = m.applyLayout()
	assertWrappedToColumn(t, m, 160, "both sidebars at 160")

	// Narrow: the campaign panel must stack below, never squeeze the chat
	// into a ribbon or widen the frame.
	m = layoutResize(t, m, 60, 30)
	if m.campaignBeside {
		t.Error("campaign panel beside the chat at 60 wide; want stacked")
	}
	assertWrappedToColumn(t, m, 60, "both sidebars at 60 (stacked)")
}
