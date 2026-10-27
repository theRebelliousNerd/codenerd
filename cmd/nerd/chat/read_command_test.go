package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// /read must complete the exchange the model itself proposes ("drop it in
// with /read and I'll tally the problems fast"): the contents enter history
// as user-supplied evidence and a turn actually runs. The pre-fix handler
// rendered the file as an assistant message and returned no command, so
// the agent never spoke again (2026-10-27 live).
func TestCommand_Read_SuppliesFileAndStartsTurn(t *testing.T) {
	ws := t.TempDir()
	body := "line one\n[ERROR] first problem\n[WARN] second problem\nline four\n"
	if err := os.WriteFile(filepath.Join(ws, "problems.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel()
	m.workspace = ws

	newModel, cmd := m.handleCommand("/read problems.log how many problems are in it?")
	result := newModel.(Model)

	if cmd == nil {
		t.Fatal("/read returned no command: no turn will ever run on the supplied file")
	}
	if !result.isLoading {
		t.Error("/read did not mark the model loading for the turn it starts")
	}
	if len(result.history) < 2 {
		t.Fatalf("history = %d messages, want the file and the question", len(result.history))
	}
	fileMsg := result.history[len(result.history)-2]
	questionMsg := result.history[len(result.history)-1]
	if fileMsg.Role != "user" || !strings.Contains(fileMsg.Content, "[ERROR] first problem") {
		t.Errorf("file message = %s: %q, want user-supplied contents", fileMsg.Role, fileMsg.Content)
	}
	if !strings.Contains(fileMsg.Content, "lines 1-4 of 4") {
		t.Errorf("file message does not state its line span: %q", fileMsg.Content)
	}
	if questionMsg.Role != "user" || questionMsg.Content != "how many problems are in it?" {
		t.Errorf("question message = %s: %q", questionMsg.Role, questionMsg.Content)
	}
}

// Without an explicit question, /read asks the turn to answer the earlier
// unanswered question about the file — the exact flow in which the model
// itself requested the /read.
func TestCommand_Read_NoQuestionAsksEarlierQuestion(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel()
	m.workspace = ws

	newModel, cmd := m.handleCommand("/read a.txt")
	result := newModel.(Model)
	if cmd == nil {
		t.Fatal("/read without a question started no turn")
	}
	last := result.history[len(result.history)-1]
	if !strings.Contains(last.Content, "earlier question") || !strings.Contains(last.Content, "a.txt") {
		t.Errorf("synthetic question = %q", last.Content)
	}
}

// The documented [start:end] range is honored, and the byte cap announces
// itself instead of silently dropping the tail.
func TestCommand_Read_RangeAndTruncationMarker(t *testing.T) {
	ws := t.TempDir()
	var b strings.Builder
	for i := 1; i <= 400; i++ {
		fmt.Fprintf(&b, "log line %04d %s\n", i, strings.Repeat("x", 60))
	}
	if err := os.WriteFile(filepath.Join(ws, "big.log"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel()
	m.workspace = ws

	newModel, _ := m.handleCommand("/read big.log 2:3 what is on these lines?")
	result := newModel.(Model)
	fileMsg := result.history[len(result.history)-2].Content
	if !strings.Contains(fileMsg, "log line 0002") || !strings.Contains(fileMsg, "log line 0003") {
		t.Errorf("range 2:3 missing its lines:\n%s", fileMsg)
	}
	if strings.Contains(fileMsg, "log line 0004") || strings.Contains(fileMsg, "log line 0001") {
		t.Errorf("range 2:3 leaked lines outside the range:\n%s", fileMsg)
	}

	m2 := NewTestModel()
	m2.workspace = ws
	newModel2, _ := m2.handleCommand("/read big.log")
	truncated := newModel2.(Model).history
	fileMsg2 := truncated[len(truncated)-2].Content
	if !strings.Contains(fileMsg2, "Truncated at 16000 bytes") || !strings.Contains(fileMsg2, "/read big.log ") {
		t.Errorf("oversize read carries no truncation marker:\n%.300s", fileMsg2)
	}
}

func TestParseReadRange(t *testing.T) {
	cases := []struct {
		in         string
		start, end int
		ok         bool
	}{
		{"10:50", 10, 50, true},
		{":50", 0, 50, true},
		{"10:", 10, 0, true},
		{"how", 0, 0, false},
		{"1:2:3", 0, 0, false},
		{":", 0, 0, false},
		{"a:b", 0, 0, false},
	}
	for _, tc := range cases {
		start, end, ok := parseReadRange(tc.in)
		if start != tc.start || end != tc.end || ok != tc.ok {
			t.Errorf("parseReadRange(%q) = %d,%d,%v want %d,%d,%v", tc.in, start, end, ok, tc.start, tc.end, tc.ok)
		}
	}
}
