package sessionlog

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDebugOffWritesNoExchangeFiles(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx, call := Begin(WithChat(context.Background()), CallMeta{Provider: "p", Model: "m", Purpose: "session"})
	if call != nil {
		t.Fatal("Begin must return a nil call while debug is off")
	}
	if CaptureFrom(ctx) != nil {
		t.Fatal("no capture may be installed while debug is off")
	}
	call.End(Result{Response: &Response{Text: Lines("hi")}}) // nil-safe
	if _, err := os.Stat(SessionsDir(s.workspace)); !os.IsNotExist(err) {
		t.Fatalf("debug off must not create anything under .nerd/sessions: %v", err)
	}
	if Active() {
		t.Fatal("Active must be false while debug is off")
	}
}

func TestExchangeFileLayoutAndFormat(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	ctx := WithTurn(WithTask(WithChat(context.Background()), "task-9"), 3)
	ctx, call := Begin(ctx, CallMeta{Provider: "anthropic", Model: "claude-x", Method: "CompleteWithSystem", Purpose: "session", Phase: "repair"})
	if call == nil {
		t.Fatal("Begin returned nil with debug on")
	}
	system := "You are codeNERD.\n<rules>\n  keep <tags> & \"quotes\"\n</rules>\n"
	call.End(Result{
		Response: &Response{Text: Lines("line one\nline two"), StopReason: "end_turn"},
		Usage:    &Usage{InputTokens: 100, CacheReadTokens: 40, CacheWriteTokens: 10, OutputTokens: 7, Status: "reported"},
		Logical:  &LogicalRequest{System: Lines(system), User: Lines("go")},
	})

	path := sessionPath(s, "session-1", "chat", "0001-session-repair.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("exchange file missing: %v (have %v)", err, names(t, filepath.Dir(path)))
	}
	if !strings.Contains(string(raw), "\n  \"schema\": \"codenerd.exchange/1\",\n") {
		t.Fatalf("not pretty-printed with a 2-space indent:\n%s", raw)
	}
	if strings.Contains(string(raw), "\\u003c") {
		t.Fatalf("HTML escaped in the file:\n%s", raw)
	}
	var ex Exchange
	readJSON(t, path, &ex)
	if ex.Number != 1 || ex.Purpose != "session-repair" || ex.Outcome != OutcomeOK {
		t.Fatalf("header wrong: %+v", ex)
	}
	c := ex.Correlation
	if c.Session != "session-1" || c.Turn != 3 || c.Task != "task-9" || c.Agent != "chat" || c.Phase != "repair" {
		t.Fatalf("correlation wrong: %+v", c)
	}
	if Join(ex.LogicalRequest.System) != system {
		t.Fatalf("system prompt does not round-trip through its lines:\n got %q\nwant %q", Join(ex.LogicalRequest.System), system)
	}
	if Join(ex.Response.Text) != "line one\nline two" || ex.Response.StopReason != "end_turn" {
		t.Fatalf("response wrong: %+v", ex.Response)
	}
	if ex.Usage.CacheReadTokens != 40 || ex.Usage.CacheWriteTokens != 10 {
		t.Fatalf("usage wrong: %+v", ex.Usage)
	}
}

func TestEveryAgentGetsItsOwnFolder(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	for _, c := range []struct {
		ctx  context.Context
		meta CallMeta
		want string
	}{
		{WithChat(context.Background()), CallMeta{Purpose: "perception"}, "chat"},
		{WithAgent(context.Background(), "coder", "coder-1a2b"), CallMeta{Purpose: "subagent"}, "coder-1a2b"},
		{WithAgent(context.Background(), "campaign-phase", "p1"), CallMeta{Purpose: "campaign"}, "campaign-phase-p1"},
		{context.Background(), CallMeta{Purpose: "critic", Fallback: "critic"}, "critic"},
		{context.Background(), CallMeta{Purpose: "x"}, UnattributedFolder},
		{WithChat(context.Background()), CallMeta{Purpose: "judge", Folder: JevFolder}, "jev"},
	} {
		_, call := Begin(c.ctx, c.meta)
		call.End(Result{})
		if got := names(t, sessionPath(s, "session-1", c.want)); len(got) != 1 {
			t.Errorf("folder %q holds %v, want one exchange file", c.want, got)
		}
	}
}

func TestConcurrentCallsNeverShareANumber(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	const n = 200
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, call := Begin(WithAgent(context.Background(), "coder", "c1"), CallMeta{Purpose: "session"})
			call.End(Result{Response: &Response{Text: []string{"x"}}})
		}()
	}
	wg.Wait()
	files := names(t, sessionPath(s, "session-1", "coder-c1"))
	if len(files) != n {
		t.Fatalf("%d files for %d calls: %v", len(files), n, files)
	}
	seen := map[string]bool{}
	for _, f := range files {
		prefix := f[:4]
		if seen[prefix] {
			t.Fatalf("number %s used twice", prefix)
		}
		seen[prefix] = true
	}
	if files[0][:4] != "0001" || files[n-1][:4] != "0200" {
		t.Fatalf("numbers are not dense 0001..0200: first %s last %s", files[0], files[n-1])
	}
}

func TestNumberingContinuesAcrossAStoreRestart(t *testing.T) {
	s, on := installStore(t, "session-1", true)
	for i := 0; i < 3; i++ {
		_, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
		call.End(Result{})
	}
	s2 := NewStore(Options{Workspace: s.workspace, Session: "session-1", Enabled: on.Load})
	Install(s2)
	_, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
	call.End(Result{})
	files := names(t, sessionPath(s, "session-1", "chat"))
	if len(files) != 4 || files[3][:4] != "0004" {
		t.Fatalf("a restarted store must continue at 0004, got %v", files)
	}
}

func TestOrderFollowsCallStartNotFinish(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	_, first := Begin(WithChat(context.Background()), CallMeta{Purpose: "a"})
	_, second := Begin(WithChat(context.Background()), CallMeta{Purpose: "b"})
	second.End(Result{})
	first.End(Result{})
	files := names(t, sessionPath(s, "session-1", "chat"))
	if len(files) != 2 || files[0] != "0001-a.json" || files[1] != "0002-b.json" {
		t.Fatalf("files = %v", files)
	}
}

func TestAFailedWriteIsCountedWarnedOnceAndNeverFailsTheCall(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	// A file where the session folder must be makes every write fail.
	if err := os.MkdirAll(SessionsDir(s.workspace), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(SessionsDir(s.workspace), "session-1"), []byte("in the way"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		ctx, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
		call.End(Result{Err: errors.New("provider said no")})
		if ctx == nil {
			t.Fatal("Begin must still hand back a usable context")
		}
	}
	n, first := s.Failures()
	if n != 3 || first == nil {
		t.Fatalf("Failures = %d, %v; want 3 and the first error", n, first)
	}
}

func TestEndRecordsAnErrorAndARefusal(t *testing.T) {
	s, _ := installStore(t, "session-1", true)
	_, a := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
	a.End(Result{Err: errors.New("boom")})
	_, b := Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
	b.End(Result{Err: errors.New("window exceeded"), Refused: true, Admission: map[string]any{"window": 1}})
	var ea, eb Exchange
	readJSON(t, sessionPath(s, "session-1", "chat", "0001-session.json"), &ea)
	readJSON(t, sessionPath(s, "session-1", "chat", "0002-session.json"), &eb)
	if ea.Outcome != OutcomeError || ea.Error != "boom" {
		t.Fatalf("error exchange: %+v", ea)
	}
	if eb.Outcome != OutcomeRefused || eb.Admission == nil {
		t.Fatalf("refused exchange: %+v", eb)
	}
}

func TestACallTaggedWithAPlaceholderSessionLandsInTheLiveSession(t *testing.T) {
	s, _ := installStore(t, "session-live", true)
	_, call := Begin(WithChat(context.Background()), CallMeta{Purpose: "session", Session: "current-session"})
	call.End(Result{})
	if got := names(t, SessionsDir(s.workspace)); len(got) != 1 || got[0] != "session-live" {
		t.Fatalf("sessions = %v, want only session-live", got)
	}
	s.Adopt("session-next")
	_, call = Begin(WithChat(context.Background()), CallMeta{Purpose: "session"})
	call.End(Result{})
	if len(names(t, sessionPath(s, "session-next", "chat"))) != 1 {
		t.Fatal("Adopt must redirect later calls to the new session")
	}
}
