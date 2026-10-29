package chat

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Fast input reaches the chat one word per KeyRunes message. A word that is
// also a key name ("left", "up", "end", ...) is text: it was matched as that
// key by the textarea's keymap, so "and left uncommitted" moved the cursor
// back and the rest of the message landed mid-word.
func TestTypedWordsThatNameKeysAreText(t *testing.T) {
	words := []string{"and", "left", "uncommitted", "up", "to", "down", "right", "home", "end", "enter", "delete", "backspace", "tab", "pgup", "insert"}
	m := NewTestModel()
	m.textarea.Focus()
	want := ""
	for i, w := range words {
		if i > 0 {
			m = typeKey(t, m, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			want += " "
		}
		m = typeKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(w)})
		want += w
	}
	if got := m.textarea.Value(); got != want {
		t.Fatalf("textarea holds %q, want %q", got, want)
	}
	// A real arrow key still moves the cursor: the text is not a key, the key
	// still is.
	m = typeKey(t, m, tea.KeyMsg{Type: tea.KeyLeft})
	m = typeKey(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if got := m.textarea.Value(); got != want[:len(want)-1]+"X"+want[len(want)-1:] {
		t.Fatalf("after Left and X the textarea holds %q", got)
	}
}

func typeKey(t *testing.T, m Model, msg tea.KeyMsg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next.(Model)
}
