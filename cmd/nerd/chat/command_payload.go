package chat

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/runeutil"
	tea "github.com/charmbracelet/bubbletea"
)

// Payload commands (/write, /append) and patch mode carry file content, not
// conversational tokens. Only the command envelope (verb and path) is
// parsed; the bytes after it reach the writer exactly as submitted. Command
// recognition keeps its own trimmed, sanitized view (handleCommand), which is
// never used as file data.

// patchTerminator ends patch mode. It is recognized only as a whole
// submission of exactly these bytes; a chunk that contains a newline is
// always patch data, so a removal line reading "-END--" can be sent inside a
// multi-line paste.
const patchTerminator = "--END--"

var errPayloadMissing = errors.New("missing content")

// splitPayloadCommand parses `<verb> <path><sep><payload>`. The separator is
// exactly one space, tab, "\n" or "\r\n" after the path; everything after it
// is the payload, byte for byte. Paths are a single whitespace-free token.
// An empty payload is refused: a stray trailing space must not truncate a
// file.
func splitPayloadCommand(raw string) (path, payload string, err error) {
	rest := strings.TrimLeft(raw, " \t\r\n")
	verbEnd := strings.IndexAny(rest, " \t\r\n")
	if verbEnd < 0 {
		return "", "", errPayloadMissing
	}
	rest = strings.TrimLeft(rest[verbEnd:], " \t\r\n")
	pathEnd := strings.IndexAny(rest, " \t\r\n")
	if pathEnd <= 0 {
		return "", "", errPayloadMissing
	}
	path = rest[:pathEnd]
	rest = rest[pathEnd:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else {
		rest = rest[1:]
	}
	if rest == "" {
		return "", "", errPayloadMissing
	}
	if err := checkTextPayload(rest); err != nil {
		return path, "", err
	}
	return path, rest, nil
}

// checkTextPayload refuses bytes a text command cannot carry faithfully: the
// display sanitizer would strip them, and stripping them from file content
// would be a silent rewrite.
func checkTextPayload(payload string) error {
	for i, r := range payload {
		if r == 0 || r == 0x1b || (r < 0x20 && r != '\n' && r != '\r' && r != '\t') || r == 0x7f {
			return fmt.Errorf("content holds control character U+%04X at byte %d; slash commands carry text only, so nothing was written", r, i)
		}
	}
	return nil
}

// patchChunk is one submission in patch mode. The submission boundary is a
// line boundary, so exactly one trailing line ending is dropped and every
// other byte (leading context-marker spaces, trailing spaces, blank lines)
// is kept.
func patchChunk(raw string) string {
	if strings.HasSuffix(raw, "\r\n") {
		return raw[:len(raw)-2]
	}
	return strings.TrimSuffix(raw, "\n")
}

// ingestPatchSubmission accumulates one raw patch-mode submission, or
// applies the patch on the terminator.
func (m Model) ingestPatchSubmission(raw string) (tea.Model, tea.Cmd) {
	chunk := patchChunk(raw)
	if chunk != patchTerminator {
		m.pendingPatchLines = append(m.pendingPatchLines, chunk)
		m.textarea.Reset()
		return m, nil
	}
	result := "No patch lines were received; nothing was applied."
	if len(m.pendingPatchLines) > 0 {
		result = applyPatchResult(m.workspace, strings.Join(m.pendingPatchLines, "\n")+"\n")
	}
	m.setInputMode(InputModeNormal)
	m.textarea.Placeholder = "Ask me anything... (Enter to send, Shift+Enter for newline, Ctrl+C to exit)"
	m = m.addMessage(Message{
		Role:    "assistant",
		Content: result,
		Time:    time.Now(),
	})
	m.textarea.Reset()
	m.viewport.SetContent(m.renderHistory())
	m.viewport.GotoBottom()
	return m, nil
}

// droppedInputNotice reports what the input box refused to take from a
// paste: the bubbles textarea cuts input at its character limit
// and its 10,000-line cap without a word, and a cut payload must not be
// sent as if whole. before and after are the box's lengths around the
// update; the expected growth is the runes as the box's own sanitizer
// renders them (tabs become four spaces, control characters go).
func droppedInputNotice(msg tea.KeyMsg, before, after, charLimit int) string {
	if msg.Type != tea.KeyRunes || !msg.Paste || len(msg.Runes) == 0 {
		return ""
	}
	runes := append([]rune(nil), msg.Runes...)
	want := len(runeutil.NewSanitizer().Sanitize(runes))
	dropped := before + want - after
	if dropped <= 0 {
		return ""
	}
	return fmt.Sprintf("The input box holds at most %d characters and 10,000 lines; %d of the %d characters just entered were not inserted. Nothing was sent: submit the content in parts, or put it in a file and point at the file.",
		charLimit, dropped, want)
}
