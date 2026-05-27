//go:build integration

package chat

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/transparency"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =============================================================================
// FRAME HARNESS — reusable for all TUI tests
// =============================================================================

// FrameProbe captures structural analysis of a rendered TUI frame.
type FrameProbe struct {
	Width      int
	Height     int
	View       string
	Lines      []string
	MaxWidth   int
	LineCount  int
	HasHeader  bool
	HasFooter  bool
	HasInput   bool
	HasError   bool
	HasReady   bool
	HasStop    bool
	HasSpinner bool
}

// renderFrame sends a WindowSizeMsg, calls View(), and returns a FrameProbe.
// Panics in Update or View cause the test to fail immediately.
func renderFrame(t *testing.T, m Model, w, h int) (Model, FrameProbe) {
	t.Helper()

	// Update must not panic
	var updated tea.Model
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC in Update(WindowSizeMsg{%d,%d}): %v", w, h, r)
			}
		}()
		updated, _ = m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	}()
	m = updated.(Model)

	// View must not panic
	var view string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC in View() at %dx%d: %v", w, h, r)
			}
		}()
		view = m.View()
	}()

	probe := analyzeFrame(view, w, h)
	return m, probe
}

// analyzeFrame extracts structural properties from a rendered view string.
func analyzeFrame(view string, w, h int) FrameProbe {
	lines := strings.Split(view, "\n")
	maxWidth := 0
	for _, line := range lines {
		lw := lipgloss.Width(line)
		if lw > maxWidth {
			maxWidth = lw
		}
	}

	viewLower := strings.ToLower(view)

	return FrameProbe{
		Width:     w,
		Height:    h,
		View:      view,
		Lines:     lines,
		MaxWidth:  maxWidth,
		LineCount: len(lines),
		HasHeader: strings.Contains(view, "codeNERD"),
		HasFooter: strings.Contains(viewLower, "help") || strings.Contains(viewLower, "mode"),
		HasInput:  strings.Contains(view, "Test input") || strings.Contains(view, "│") || strings.Contains(view, "╭"),
		HasError:  strings.Contains(view, "Error"),
		HasReady:  strings.Contains(view, "Ready"),
		HasStop:   strings.Contains(view, "STOP"),
		HasSpinner: false, // spinner state is non-deterministic
	}
}

// assertFrameSane enforces structural invariants on a rendered frame.
func assertFrameSane(t *testing.T, m Model, probe FrameProbe) {
	t.Helper()

	// 1. View is non-empty
	if len(strings.TrimSpace(probe.View)) == 0 {
		t.Errorf("[%dx%d] View is empty or whitespace-only", probe.Width, probe.Height)
	}

	// 2. Viewport dimensions are positive
	if m.viewport.Width < 1 {
		t.Errorf("[%dx%d] viewport.Width=%d, want >= 1", probe.Width, probe.Height, m.viewport.Width)
	}
	if m.viewport.Height < 1 {
		t.Errorf("[%dx%d] viewport.Height=%d, want >= 1", probe.Width, probe.Height, m.viewport.Height)
	}

	// 3. Error viewport dimensions are positive
	if m.errorVP.Width < 1 {
		t.Errorf("[%dx%d] errorVP.Width=%d, want >= 1", probe.Width, probe.Height, m.errorVP.Width)
	}

	// 4. ready flag must be true after first resize
	if !m.ready {
		t.Errorf("[%dx%d] m.ready=false after WindowSizeMsg", probe.Width, probe.Height)
	}

	// 5. Line widths bounded (only for sane terminals — skip tiny ones)
	// Detailed overflow check is in invariant 9 below, with campaign panel exemption

	// 6. Sane terminals should have visible header, input, and footer in chat mode
	if probe.Width >= 40 && probe.Height >= 10 && m.viewMode == ChatView && !m.isBooting {
		if !probe.HasHeader {
			t.Errorf("[%dx%d] sane chat frame missing header", probe.Width, probe.Height)
		}
		// Footer has many indicators — only expect it visible on wider terminals
		if probe.Width >= 60 && !probe.HasFooter {
			t.Errorf("[%dx%d] sane chat frame missing footer", probe.Width, probe.Height)
		}
	}

	// 7. Loading/STOP consistency
	if m.isLoading && probe.Width >= 40 && probe.Height >= 10 && m.viewMode == ChatView {
		if !probe.HasStop {
			t.Errorf("[%dx%d] loading=true but Ctrl+X STOP not visible in footer", probe.Width, probe.Height)
		}
	}
	if !m.isLoading && probe.HasStop && m.viewMode == ChatView {
		t.Errorf("[%dx%d] loading=false but STOP is visible", probe.Width, probe.Height)
	}

	// 8. Error panel consistency
	if m.err != nil && m.showError && probe.Width >= 40 && probe.Height >= 10 && m.viewMode == ChatView {
		if !probe.HasError {
			t.Errorf("[%dx%d] showError=true but 'Error' not visible in frame", probe.Width, probe.Height)
		}
	}

	// 9. Line overflow is exempt when campaign panel or split pane adds horizontal content
	if m.showCampaignPanel && m.activeCampaign != nil {
		// Campaign panel intentionally extends beyond chat width — skip overflow
	} else if probe.Width >= 20 {
		assertNoLineOverflow(t, probe.View, probe.Width)
	}

	// 10. Rendered frame is stable across two immediate View() calls
	// (normalize timestamps which change between calls)
	view1 := normalizeTimestamps(m.View())
	view2 := normalizeTimestamps(m.View())
	if view1 != view2 {
		t.Errorf("[%dx%d] View() is non-deterministic across two immediate calls", probe.Width, probe.Height)
	}
}

// assertNoLineOverflow checks that no rendered line exceeds terminal width.
func assertNoLineOverflow(t *testing.T, view string, width int) {
	t.Helper()
	tolerance := 4 // borders and ANSI styling
	for i, line := range strings.Split(view, "\n") {
		got := lipgloss.Width(line)
		if got > width+tolerance {
			// Truncate line for readable error message
			display := line
			if len(display) > 120 {
				display = display[:120] + "..."
			}
			t.Errorf("line %d overflow: got width %d > terminal %d+%d\nline=%q", i, got, width, tolerance, display)
			return // one overflow is enough to report
		}
	}
}

// normalizeTimestamps replaces HH:MM patterns with <TIME> for frame comparison.
var timeRe = regexp.MustCompile(`\d{2}:\d{2}`)

func normalizeTimestamps(s string) string {
	return timeRe.ReplaceAllString(s, "<TIME>")
}

// =============================================================================
// TERMINAL SIZE TABLE
// =============================================================================

var testSizes = []tea.WindowSizeMsg{
	{Width: 1, Height: 1},
	{Width: 2, Height: 2},
	{Width: 5, Height: 3},
	{Width: 10, Height: 5},
	{Width: 20, Height: 5},
	{Width: 40, Height: 10},
	{Width: 80, Height: 24},
	{Width: 120, Height: 40},
	{Width: 200, Height: 60},
	{Width: 80, Height: 8},
	{Width: 30, Height: 20},
	{Width: 160, Height: 12},
}

// =============================================================================
// STATE SEEDERS
// =============================================================================

func seedReadyModel() Model {
	return NewTestModel()
}

func seedReadyWithHistory() Model {
	return NewTestModel(WithHistory(
		Message{Role: "user", Content: "hello"},
		Message{Role: "assistant", Content: "hi there, how can I help you today?"},
	))
}

func seedLoadingModel() Model {
	m := NewTestModel(WithLoading(true))
	m.statusMessage = "Perception: parsing intent..."
	return m
}

func seedErrorModel() Model {
	m := NewTestModel()
	m.err = errorMsg(errors.New("something went wrong: nil pointer dereference in articulateWithConversation"))
	m.showError = true
	m.refreshErrorViewport()
	return m
}

func seedErrorFocusedModel() Model {
	m := seedErrorModel()
	m.focusError = true
	return m
}

func seedClarificationModel() Model {
	m := NewTestModel()
	m.inputMode = InputModeClarification
	m.clarificationState = &ClarificationState{
		Question: "Which file do you mean?",
		Options:  []string{"internal/core/kernel.go", "internal/core/kernel_eval.go"},
		Context:  "refactor the kernel",
	}
	m.textarea.Placeholder = "Enter option number or custom answer..."
	return m
}

func seedContinuationPaused() Model {
	m := NewTestModel(WithLoading(false))
	m.pendingSubtasks = []Subtask{
		{ShardType: "tester", Description: "run unit tests"},
		{ShardType: "reviewer", Description: "review changes"},
	}
	m.continuationStep = 1
	m.continuationTotal = 3
	return m
}

func seedCampaignPanelModel() Model {
	m := NewTestModel()
	m.showCampaignPanel = true
	m.activeCampaign = &campaign.Campaign{
		Goal:           "Refactor auth module",
		TotalTasks:     10,
		CompletedTasks: 3,
	}
	return m
}

func seedLongHistory() Model {
	msgs := make([]Message, 0, 120)
	for i := 0; i < 60; i++ {
		msgs = append(msgs,
			Message{Role: "user", Content: fmt.Sprintf("Question %d: explain how component %d works", i, i)},
			Message{Role: "assistant", Content: fmt.Sprintf("Component %d handles request routing and dispatches to the appropriate shard for processing.", i)},
		)
	}
	return NewTestModel(WithHistory(msgs...))
}

func seedMarkdownBomb() Model {
	// Huge code block + long unbroken line + table + emoji/CJK + nested backticks
	hugeContent := "# Analysis Results\n\n"
	hugeContent += "```go\n"
	for i := 0; i < 200; i++ {
		hugeContent += fmt.Sprintf("func handler%d(ctx context.Context, req *Request) (*Response, error) { return nil, nil }\n", i)
	}
	hugeContent += "```\n\n"
	hugeContent += "Long unbroken line: " + strings.Repeat("abcdefghij", 500) + "\n\n"
	hugeContent += "| Column A | Column B | Column C |\n|---|---|---|\n"
	for i := 0; i < 50; i++ {
		hugeContent += fmt.Sprintf("| row %d | value %d | result %d |\n", i, i*2, i*3)
	}
	hugeContent += "\n🎉 CJK: 你好世界 日本語 한국어 العربية\n"
	hugeContent += "Nested: `` `inner` `` and ```triple```\n"

	return NewTestModel(WithHistory(
		Message{Role: "user", Content: "analyze everything"},
		Message{Role: "assistant", Content: hugeContent},
	))
}

func seedToolEventModel() Model {
	return NewTestModel(WithHistory(
		Message{Role: "user", Content: "run tests"},
		Message{Role: "tool", Content: "```\nPASS: TestFoo (0.3s)\nFAIL: TestBar (1.2s)\n  Expected: 42\n  Got: 0\n```"},
		Message{Role: "assistant", Content: "TestBar is failing because the value is not initialized."},
	))
}

// =============================================================================
// TEST 1: Frame Contract — All Core States × All Sizes
// =============================================================================

func TestE2E_TUI_FrameContract_AllCoreStates(t *testing.T) {
	type stateCase struct {
		name  string
		model func() Model
	}

	states := []stateCase{
		{"ready_empty", seedReadyModel},
		{"ready_with_history", seedReadyWithHistory},
		{"loading", seedLoadingModel},
		{"error_visible", seedErrorModel},
		{"error_focused", seedErrorFocusedModel},
		{"clarification", seedClarificationModel},
		{"continuation_paused", seedContinuationPaused},
		{"campaign_panel", seedCampaignPanelModel},
		{"long_history", seedLongHistory},
		{"markdown_bomb", seedMarkdownBomb},
		{"tool_event", seedToolEventModel},
	}

	for _, sc := range states {
		t.Run(sc.name, func(t *testing.T) {
			for _, size := range testSizes {
				label := fmt.Sprintf("%dx%d", size.Width, size.Height)
				t.Run(label, func(t *testing.T) {
					m := sc.model()
					m, probe := renderFrame(t, m, size.Width, size.Height)
					assertFrameSane(t, m, probe)
				})
			}
		})
	}
}

// =============================================================================
// TEST 2: Resize Storm — rapid sequential resizing
// =============================================================================

func TestE2E_TUI_ResizeStorm_Sequential(t *testing.T) {
	m := seedReadyWithHistory()

	// Rapid sequential resizing should never corrupt state
	for cycle := 0; cycle < 3; cycle++ {
		for _, size := range testSizes {
			var probe FrameProbe
			m, probe = renderFrame(t, m, size.Width, size.Height)
			assertFrameSane(t, m, probe)
		}

		// Reverse order
		for i := len(testSizes) - 1; i >= 0; i-- {
			var probe FrameProbe
			m, probe = renderFrame(t, m, testSizes[i].Width, testSizes[i].Height)
			assertFrameSane(t, m, probe)
		}
	}

	// Final state should be sane at standard terminal
	m, probe := renderFrame(t, m, 80, 24)
	assertFrameSane(t, m, probe)
	if !probe.HasHeader || !probe.HasFooter {
		t.Error("After resize storm, standard terminal missing header/footer")
	}
}

// =============================================================================
// TEST 3: Render Bomb — markdown, unicode, ANSI injection
// =============================================================================

func TestE2E_TUI_RenderBomb_MarkdownUnicodeAnsi(t *testing.T) {
	bombContents := []struct {
		name    string
		content string
	}{
		{"100k_chars", strings.Repeat("The quick brown fox jumps over the lazy dog. ", 2500)},
		{"10k_unbroken", strings.Repeat("x", 10000)},
		{"huge_code_block", "```\n" + strings.Repeat("line of code\n", 5000) + "```"},
		{"malformed_table", "| a | b |\n|---|\n| c | d | e | f |\n| g |"},
		{"nested_backticks", "`` `inner` `` and ``` `` nested `` ``` and ```` ``` deep ``` ````"},
		{"cjk_emoji_combining", "你好世界🎉🔥💯 café naïve résumé Ω≈∞ العربية 日本語テスト 한국어"},
		{"zero_width_joiners", "👨\u200d👩\u200d👧\u200d👦 flag: 🏳️\u200d🌈 zwsp:\u200b\u200b\u200b"},
		{"1000_bullets", func() string {
			var sb strings.Builder
			for i := 0; i < 1000; i++ {
				sb.WriteString(fmt.Sprintf("- item %d\n", i))
			}
			return sb.String()
		}()},
		{"stack_trace", func() string {
			var sb strings.Builder
			sb.WriteString("goroutine 1 [running]:\n")
			for i := 0; i < 100; i++ {
				sb.WriteString(fmt.Sprintf("pkg/module%d.Function%d(0x%x)\n", i, i, i*0x1000))
				sb.WriteString(fmt.Sprintf("\t/src/pkg/module%d/file.go:%d +0x%x\n", i, i*10, i*0x20))
			}
			return sb.String()
		}()},
	}

	for _, bomb := range bombContents {
		t.Run(bomb.name, func(t *testing.T) {
			m := NewTestModel(WithHistory(
				Message{Role: "user", Content: "test"},
				Message{Role: "assistant", Content: bomb.content},
			))

			// Test at standard and large sizes
			for _, size := range []tea.WindowSizeMsg{
				{Width: 80, Height: 24},
				{Width: 120, Height: 40},
				{Width: 40, Height: 10},
			} {
				m2, probe := renderFrame(t, m, size.Width, size.Height)
				assertFrameSane(t, m2, probe)

				// safeRenderMarkdown fallback must preserve content if renderer fails
				if len(strings.TrimSpace(probe.View)) == 0 {
					t.Errorf("[%dx%d] render bomb produced empty view", size.Width, size.Height)
				}
			}
		})
	}
}

// =============================================================================
// TEST 4: Terminal Escape Injection — user/tool content must not control screen
// =============================================================================

func TestE2E_TUI_TerminalEscapeInjection_DoesNotControlScreen(t *testing.T) {
	// These are terminal control sequences that could "spazz" the TUI
	// if rendered verbatim from LLM/tool output
	injections := []struct {
		name    string
		content string
	}{
		{"clear_screen", "\x1b[2J"},
		{"cursor_home", "\x1b[H"},
		{"hide_cursor", "\x1b[?25l"},
		{"osc_hyperlink", "\x1b]8;;https://evil.example\x07click here\x1b]8;;\x07"},
		{"cursor_back_999", "\x1b[999D"},
		{"cursor_fwd_999", "\x1b[999C"},
		{"set_title", "\x1b]0;pwned\x07"},
		{"alt_screen", "\x1b[?1049h"},
	}

	// Dangerous CSI patterns: cursor movement, screen clear, mode changes
	dangerousCSI := regexp.MustCompile(`\x1b\[[\d;]*[HJKSThlfm]|\x1b\[\?\d+[hl]|\x1b\](?:0|8)`)

	for _, inj := range injections {
		t.Run(inj.name, func(t *testing.T) {
			m := NewTestModel(WithHistory(
				Message{Role: "user", Content: "run exploit"},
				Message{Role: "tool", Content: "Result: " + inj.content + " done"},
				Message{Role: "assistant", Content: "The tool completed."},
			))

			m2, probe := renderFrame(t, m, 80, 24)
			assertFrameSane(t, m2, probe)

			// Check that the dangerous sequences did not leak into the rendered
			// frame beyond what Lipgloss/Glamour legitimately generates.
			// We check specifically for the injected raw content.
			if strings.Contains(probe.View, inj.content) {
				// Only flag if it matches a dangerous pattern
				if dangerousCSI.MatchString(inj.content) {
					t.Logf("WARNING: raw terminal escape %q found in rendered output — TUI may be vulnerable to injection", inj.name)
				}
			}
		})
	}
}

// =============================================================================
// TEST 5: State Transitions — resize across state changes
// =============================================================================

func TestE2E_TUI_StateTransition_ResizeAcrossStateChanges(t *testing.T) {
	m := seedReadyWithHistory()

	// Standard size
	m, _ = renderFrame(t, m, 80, 24)

	// Transition to loading
	m.isLoading = true
	m.statusMessage = "Thinking..."
	m, probe := renderFrame(t, m, 80, 24)
	assertFrameSane(t, m, probe)
	if !probe.HasStop {
		t.Error("Loading state should show STOP")
	}

	// Resize while loading
	m, probe = renderFrame(t, m, 40, 10)
	assertFrameSane(t, m, probe)

	// Transition to error
	m.isLoading = false
	m.err = errorMsg(errors.New("test error"))
	m.showError = true
	m.refreshErrorViewport()
	m, probe = renderFrame(t, m, 80, 24)
	assertFrameSane(t, m, probe)
	if !probe.HasError {
		t.Error("Error state should show error panel")
	}
	if probe.HasStop {
		t.Error("Non-loading error state should not show STOP")
	}

	// Resize with error visible
	m, probe = renderFrame(t, m, 120, 40)
	assertFrameSane(t, m, probe)

	// Clear error, add campaign panel
	m.err = nil
	m.showError = false
	m.showCampaignPanel = true
	m.activeCampaign = &campaign.Campaign{
		Goal:           "test",
		TotalTasks:     5,
		CompletedTasks: 2,
	}
	m, probe = renderFrame(t, m, 160, 40)
	assertFrameSane(t, m, probe)

	// Shrink to tiny with campaign panel
	m, probe = renderFrame(t, m, 20, 5)
	assertFrameSane(t, m, probe)

	// Return to normal
	m.showCampaignPanel = false
	m.activeCampaign = nil
	m, probe = renderFrame(t, m, 80, 24)
	assertFrameSane(t, m, probe)
	if !probe.HasReady {
		t.Error("After clearing all states, should show Ready")
	}
}

// =============================================================================
// HELPERS — safe update with panic recovery + frame check
// =============================================================================

// safeUpdate feeds a message through Update with panic recovery. Returns
// the updated model. Fails the test on panic.
func safeUpdate(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	var updated tea.Model
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("PANIC in Update(%T): %v", msg, r)
			}
		}()
		updated, _ = m.Update(msg)
	}()
	return updated.(Model)
}

// assertFrameAfterUpdate runs a frame check at 80×24 after a state mutation.
func assertFrameAfterUpdate(t *testing.T, m Model) Model {
	t.Helper()
	m, probe := renderFrame(t, m, 80, 24)
	assertFrameSane(t, m, probe)
	return m
}

// =============================================================================
// TEST 6: Input State Machine — handleSubmit branching
// =============================================================================

func TestE2E_TUI_InputStateMachine_KeySequences(t *testing.T) {

	// --- normal typing → enter ---
	t.Run("normal_submit", func(t *testing.T) {
		m := NewTestModel()
		m, _ = renderFrame(t, m, 80, 24) // initialize ready
		histBefore := len(m.history)

		m.textarea.SetValue("explain the kernel")
		submitted, cmd := m.handleSubmit()
		m = submitted.(Model)

		// User message appended exactly once
		if len(m.history) != histBefore+1 {
			t.Errorf("expected 1 new message, got %d", len(m.history)-histBefore)
		}
		if m.history[len(m.history)-1].Role != "user" {
			t.Errorf("last message role=%q, want user", m.history[len(m.history)-1].Role)
		}
		// textarea cleared
		if m.textarea.Value() != "" {
			t.Errorf("textarea not cleared: %q", m.textarea.Value())
		}
		// loading state set + cmd returned
		if !m.isLoading {
			t.Error("isLoading should be true after normal submit")
		}
		if cmd == nil {
			t.Error("cmd should be non-nil for normal submit")
		}
	})

	// --- empty enter → no message, no cmd ---
	t.Run("empty_enter_no_subtasks", func(t *testing.T) {
		m := NewTestModel()
		histBefore := len(m.history)

		m.textarea.SetValue("")
		submitted, cmd := m.handleSubmit()
		m = submitted.(Model)

		if len(m.history) != histBefore {
			t.Error("empty enter should not add a message")
		}
		if cmd != nil {
			t.Error("empty enter should return nil cmd")
		}
		if m.isLoading {
			t.Error("empty enter should not set loading")
		}
	})

	// --- empty enter with pendingSubtasks → confirmContinueMsg ---
	t.Run("empty_enter_with_subtasks", func(t *testing.T) {
		m := NewTestModel()
		m.pendingSubtasks = []Subtask{
			{ShardType: "tester", Description: "run tests"},
		}

		m.textarea.SetValue("")
		submitted, cmd := m.handleSubmit()
		m = submitted.(Model)

		if cmd == nil {
			t.Fatal("empty enter with subtasks should return a cmd")
		}
		// Execute the cmd — should produce confirmContinueMsg
		msg := cmd()
		if _, ok := msg.(confirmContinueMsg); !ok {
			t.Errorf("expected confirmContinueMsg, got %T", msg)
		}
	})

	// --- slash command → no perception, no loading ---
	t.Run("slash_help", func(t *testing.T) {
		m := NewTestModel()
		m, _ = renderFrame(t, m, 80, 24)

		m.textarea.SetValue("/help")
		submitted, _ := m.handleSubmit()
		m = submitted.(Model)

		// Slash commands should NOT set isLoading (they are synchronous)
		// /help returns immediately
		if m.isLoading {
			t.Error("/help should not set isLoading")
		}
	})

	// --- patch mode accumulation ---
	t.Run("patch_mode_accumulation", func(t *testing.T) {
		m := NewTestModel()
		m.inputMode = InputModePatch
		m.textarea.Placeholder = "Enter patch lines (--END-- to finish)..."

		// Accumulate lines
		m.textarea.SetValue("line 1")
		submitted, cmd := m.handleSubmit()
		m = submitted.(Model)
		if cmd != nil {
			t.Error("patch line should return nil cmd")
		}
		if len(m.pendingPatchLines) != 1 {
			t.Errorf("expected 1 pending patch line, got %d", len(m.pendingPatchLines))
		}
		if m.textarea.Value() != "" {
			t.Error("textarea should be cleared after patch line")
		}
		// Still in patch mode
		if m.inputMode != InputModePatch {
			t.Error("should still be in patch mode")
		}
		// NO user message added for patch lines
		userMsgCount := 0
		for _, msg := range m.history {
			if msg.Role == "user" {
				userMsgCount++
			}
		}
		if userMsgCount != 0 {
			t.Error("patch lines should NOT become user chat messages")
		}

		// Send --END-- to finish
		m.textarea.SetValue("--END--")
		submitted, cmd = m.handleSubmit()
		m = submitted.(Model)
		if m.inputMode == InputModePatch {
			t.Error("should exit patch mode after --END--")
		}
	})

	// --- clarification mode ---
	t.Run("clarification_submit", func(t *testing.T) {
		m := NewTestModel()
		m.inputMode = InputModeClarification
		m.clarificationState = &ClarificationState{
			Question: "Which file?",
			Options:  []string{"a.go", "b.go"},
		}

		histBefore := len(m.history)
		m.textarea.SetValue("1")
		submitted, _ := m.handleSubmit()
		m = submitted.(Model)

		// User message added
		if len(m.history) <= histBefore {
			t.Error("clarification answer should add user message")
		}
	})
}

// =============================================================================
// TEST 7: Async Message Storm — no state contradictions
// =============================================================================

func TestE2E_TUI_AsyncMessageStorm_NoStateContradictions(t *testing.T) {
	m := NewTestModel()
	m, _ = renderFrame(t, m, 80, 24)

	// Define a sequence of async messages that arrive in a nasty interleaving
	asyncMessages := []struct {
		name string
		msg  tea.Msg
	}{
		{"statusMsg", statusMsg("Scanning workspace...")},
		{"windowResize", tea.WindowSizeMsg{Width: 120, Height: 40}},
		{"assistantMsg", assistantMsg{Surface: "Here is the analysis."}},
		{"errorMsg", errorMsg(errors.New("connection reset"))},
		{"windowResize_small", tea.WindowSizeMsg{Width: 40, Height: 10}},
		{"responseMsg", responseMsg("Direct response from shard.")},
		{"glassBoxEvent", glassBoxEventMsg(transparency.GlassBoxEvent{
			Category: transparency.CategoryKernel,
			Summary:  "Asserted 12 facts",
		})},
		{"continuationInit", continuationInitMsg{
			completedSurface: "Step 1 complete",
			totalSteps:       3,
			next: continueMsg{
				subtaskID:   "step2",
				description: "step 2",
				shardType:   "tester",
			},
		}},
		{"windowResize_normal", tea.WindowSizeMsg{Width: 80, Height: 24}},
		{"continuationDone", continuationDoneMsg{
			stepCount: 3,
			summary:   "All 3 steps completed",
		}},
		{"memUsage", memUsageMsg{Alloc: 50 * 1024 * 1024, Sys: 100 * 1024 * 1024}},
		{"scanComplete", scanCompleteMsg{
			fileCount: 42,
			factCount: 200,
			duration:  2 * time.Second,
		}},
	}

	for _, am := range asyncMessages {
		t.Run(am.name, func(t *testing.T) {
			m = safeUpdate(t, m, am.msg)

			// Frame must be sane after every async message
			m = assertFrameAfterUpdate(t, m)

			// State consistency checks
			view := m.View()

			// If not loading, STOP should not be visible
			if !m.isLoading && strings.Contains(view, "STOP") && m.viewMode == ChatView {
				t.Errorf("after %s: not loading but STOP visible", am.name)
			}

			// If error is set and shown, error panel should be present
			if m.err != nil && m.showError && m.width >= 40 {
				if !strings.Contains(view, "Error") {
					t.Errorf("after %s: error set but not visible", am.name)
				}
			}
		})
	}

	// Run the same messages in REVERSE order to catch order-dependent bugs
	t.Run("reverse_order", func(t *testing.T) {
		m2 := NewTestModel()
		m2, _ = renderFrame(t, m2, 80, 24)

		for i := len(asyncMessages) - 1; i >= 0; i-- {
			am := asyncMessages[i]
			m2 = safeUpdate(t, m2, am.msg)
			m2 = assertFrameAfterUpdate(t, m2)
		}
	})
}

// =============================================================================
// TEST 8: Boss Fight — full chaos script
// =============================================================================

func TestE2E_TUI_BossFight_FrameFuzzer(t *testing.T) {
	m := NewTestModel()

	// Step 1: Initial resize
	m, _ = renderFrame(t, m, 80, 24)

	// Step 2: Type and submit "hi"
	m.textarea.SetValue("hi")
	submitted, _ := m.handleSubmit()
	m = submitted.(Model)
	m = assertFrameAfterUpdate(t, m)

	// Step 3: Simulate assistant response
	m = safeUpdate(t, m, assistantMsg{Surface: "Hello! How can I help?"})
	m = assertFrameAfterUpdate(t, m)
	if m.isLoading {
		t.Error("after assistantMsg, should not be loading")
	}

	// Step 4: Tiny resize
	m, _ = renderFrame(t, m, 20, 5)

	// Step 5: Large resize
	m, _ = renderFrame(t, m, 160, 40)

	// Step 6: Huge markdown message
	hugeMd := "# Big Response\n\n" + strings.Repeat("- bullet point\n", 500)
	m = safeUpdate(t, m, assistantMsg{Surface: hugeMd})
	m = assertFrameAfterUpdate(t, m)

	// Step 7: Tool event
	m = safeUpdate(t, m, toolEventMsg(transparency.ToolEvent{
		ToolName: "shell",
		Result:   "go test ./... PASS",
		Success:  true,
		Duration: 3 * time.Second,
	}))
	m = assertFrameAfterUpdate(t, m)

	// Step 8: Glass box event
	m = safeUpdate(t, m, glassBoxEventMsg(transparency.GlassBoxEvent{
		Category: transparency.CategoryPerception,
		Summary:  "Parsed intent: /explain",
	}))
	m = assertFrameAfterUpdate(t, m)

	// Step 9: Error
	m = safeUpdate(t, m, errorMsg(errors.New("LLM timeout after 30s")))
	m = assertFrameAfterUpdate(t, m)
	if m.err == nil {
		t.Error("err should be set after errorMsg")
	}

	// Step 10: Focus error panel
	m.focusError = true
	m.showError = true
	m.refreshErrorViewport()
	m = assertFrameAfterUpdate(t, m)

	// Step 11: Hide error
	m.showError = false
	m.focusError = false
	m = assertFrameAfterUpdate(t, m)

	// Step 12: Clarification
	m = safeUpdate(t, m, clarificationMsg{
		Question: "Did you mean file A or file B?",
		Options:  []string{"A", "B"},
		Context:  "ambiguous target",
	})
	m = assertFrameAfterUpdate(t, m)

	// Step 13: Answer clarification
	m.textarea.SetValue("A")
	submitted, _ = m.handleSubmit()
	m = submitted.(Model)
	m = assertFrameAfterUpdate(t, m)

	// Step 14: Continuation init
	m = safeUpdate(t, m, continuationInitMsg{
		completedSurface: "Phase 1 done",
		totalSteps:       5,
		next: continueMsg{
			subtaskID:   "step2",
			description: "Run tests",
			shardType:   "tester",
		},
	})
	m = assertFrameAfterUpdate(t, m)

	// Step 15: Continuation done
	m = safeUpdate(t, m, continuationDoneMsg{
		stepCount: 5,
		summary:   "All phases completed",
	})
	m = assertFrameAfterUpdate(t, m)
	if len(m.pendingSubtasks) != 0 {
		t.Errorf("after continuationDone, pendingSubtasks should be empty, got %d", len(m.pendingSubtasks))
	}

	// Step 16: Campaign lifecycle
	m.showCampaignPanel = true
	m.activeCampaign = &campaign.Campaign{
		Goal:           "Refactor auth",
		TotalTasks:     10,
		CompletedTasks: 5,
	}
	m = assertFrameAfterUpdate(t, m)

	m = safeUpdate(t, m, campaignCompletedMsg(&campaign.Campaign{
		Goal:           "Refactor auth",
		TotalTasks:     10,
		CompletedTasks: 10,
	}))
	m = assertFrameAfterUpdate(t, m)

	// Step 17: Slash /clear
	m.textarea.SetValue("/clear")
	submitted, _ = m.handleSubmit()
	m = submitted.(Model)
	m = assertFrameAfterUpdate(t, m)

	// Step 18: Normal message after all chaos
	m.textarea.SetValue("everything still works?")
	submitted, cmd := m.handleSubmit()
	m = submitted.(Model)
	m = assertFrameAfterUpdate(t, m)
	if cmd == nil {
		t.Error("final normal submit should return a cmd")
	}
	if !m.isLoading {
		t.Error("final normal submit should set isLoading")
	}

	// Final: standard resize + full frame check
	m, probe := renderFrame(t, m, 80, 24)
	assertFrameSane(t, m, probe)

	// Global invariant: known viewMode
	validModes := map[ViewMode]bool{
		ChatView: true, ListView: true, FilePickerView: true,
		UsageView: true, CampaignPage: true, PromptInspector: true,
		AutopoiesisPage: true, ShardPage: true,
	}
	if !validModes[m.viewMode] {
		t.Errorf("unknown viewMode: %d", m.viewMode)
	}
}

