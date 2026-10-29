package sessionlog

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type transcriptDoc struct {
	Schema  string `json:"schema"`
	Session string `json:"session"`
	Agent   string `json:"agent"`
	Role    string `json:"role"`
	Events  []struct {
		Seq       int        `json:"seq"`
		Turn      int        `json:"turn"`
		Kind      string     `json:"kind"`
		Text      []string   `json:"text"`
		Tool      string     `json:"tool"`
		CallID    string     `json:"call_id"`
		IsError   bool       `json:"is_error"`
		Error     string     `json:"error"`
		ToolCalls []ToolCall `json:"tool_calls"`
	} `json:"events"`
}

func TestTranscriptIsValidJSONAfterEveryAppend(t *testing.T) {
	s, _ := installStore(t, "session-1", false) // debug off: transcripts do not depend on it
	ctx := WithTurn(WithAgent(context.Background(), "coder", "coder-1"), 2)
	tr := TranscriptFrom(ctx)
	if tr == nil {
		t.Fatal("an agent scope must yield a transcript with debug off")
	}
	path := sessionPath(s, "session-1", "coder-1", TranscriptFile)

	var doc transcriptDoc
	tr.Input(ctx, "fix the bug\nin foo.go")
	readJSON(t, path, &doc)
	if len(doc.Events) != 1 || doc.Events[0].Kind != EventInput || Join(doc.Events[0].Text) != "fix the bug\nin foo.go" {
		t.Fatalf("after input: %+v", doc)
	}
	tr.Assistant(ctx, "m", &Response{Text: Lines("calling"), StopReason: "tool_use", ToolCalls: []ToolCall{{ID: "c1", Name: "read_file"}}})
	readJSON(t, path, &doc)
	if len(doc.Events) != 2 || doc.Events[1].ToolCalls[0].Name != "read_file" {
		t.Fatalf("after assistant: %+v", doc.Events)
	}
	tr.ToolResult(ctx, "read_file", "c1", "package foo\n\nfunc F() {}\n", false)
	tr.ToolResult(ctx, "run_build", "c2", "boom", true)
	tr.Final(ctx, "done", nil)
	readJSON(t, path, &doc)
	if len(doc.Events) != 5 {
		t.Fatalf("want 5 events, got %d", len(doc.Events))
	}
	for i, e := range doc.Events {
		if e.Seq != i+1 || e.Turn != 2 {
			t.Errorf("event %d: seq %d turn %d", i, e.Seq, e.Turn)
		}
	}
	if got := Join(doc.Events[2].Text); got != "package foo\n\nfunc F() {}\n" || doc.Events[2].Tool != "read_file" || doc.Events[2].CallID != "c1" {
		t.Errorf("tool result did not round-trip: %+v", doc.Events[2])
	}
	if !doc.Events[3].IsError || doc.Events[4].Kind != EventFinal {
		t.Errorf("events: %+v", doc.Events[3:])
	}
	if doc.Schema != TranscriptSchema || doc.Session != "session-1" || doc.Agent != "coder-1" || doc.Role != "coder" {
		t.Errorf("header: %+v", doc)
	}
	if raw := readFile(t, path); !strings.Contains(raw, "\n    {\n      \"seq\": 1,") {
		t.Errorf("events are not indented as a 2-space document:\n%s", raw)
	}
}

func TestTranscriptKeptWhenTheAgentIsKilledMidRun(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx, cancel := context.WithCancel(WithAgent(context.Background(), "tester", "tester-1"))
	tr := TranscriptFrom(ctx)
	tr.Input(ctx, "run the tests")
	tr.ToolResult(ctx, "run_tests", "c1", "ok", false)
	cancel() // killed: no Final is ever written
	var doc transcriptDoc
	readJSON(t, sessionPath(s, "session-1", "tester-1", TranscriptFile), &doc)
	if len(doc.Events) != 2 {
		t.Fatalf("a killed agent's transcript must hold what happened before the kill, got %d events", len(doc.Events))
	}
}

func TestRunAgentRecordsInputAndOutcomeOnEveryPath(t *testing.T) {
	s, _ := installStore(t, "session-1", false)

	res, err := RunAgent(context.Background(), "reviewer", "r1", "review it", func(ctx context.Context) (string, error) {
		if ScopeFrom(ctx) == nil || ScopeFrom(ctx).Folder != "reviewer-r1" {
			t.Errorf("run must see the agent's scope, got %+v", ScopeFrom(ctx))
		}
		return "looks fine", nil
	})
	if res != "looks fine" || err != nil {
		t.Fatalf("RunAgent must pass the result through: %q %v", res, err)
	}
	_, err = RunAgent(context.Background(), "reviewer", "r2", "review again", func(ctx context.Context) (string, error) {
		return "", errors.New("provider down")
	})
	if err == nil {
		t.Fatal("RunAgent must pass the error through")
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = RunAgent(context.Background(), "reviewer", "r3", "review a third", func(ctx context.Context) (string, error) {
			panic("kaboom")
		})
	}()

	cases := map[string]struct{ final, errPart string }{
		"reviewer-r1": {"looks fine", ""},
		"reviewer-r2": {"", "provider down"},
		"reviewer-r3": {"", "panic: kaboom"},
	}
	for folder, want := range cases {
		var doc transcriptDoc
		readJSON(t, sessionPath(s, "session-1", folder, TranscriptFile), &doc)
		if len(doc.Events) != 2 || doc.Events[0].Kind != EventInput || doc.Events[1].Kind != EventFinal {
			t.Fatalf("%s: events %+v", folder, doc.Events)
		}
		if Join(doc.Events[1].Text) != want.final || !strings.Contains(doc.Events[1].Error, want.errPart) {
			t.Errorf("%s: final %+v, want %+v", folder, doc.Events[1], want)
		}
	}
}

func TestTranscriptResumesSequenceAfterARestart(t *testing.T) {
	s, on := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	TranscriptFrom(ctx).Input(ctx, "a")
	TranscriptFrom(ctx).Final(ctx, "b", nil)

	Install(NewStore(Options{Workspace: s.workspace, Session: "session-1", Enabled: on.Load}))
	TranscriptFrom(ctx).Input(ctx, "c")
	var doc transcriptDoc
	readJSON(t, sessionPath(s, "session-1", "coder-1", TranscriptFile), &doc)
	if len(doc.Events) != 3 || doc.Events[2].Seq != 3 {
		t.Fatalf("a restarted writer must append to the file it finds: %+v", doc.Events)
	}
}

func TestTranscriptConcurrentAppendsStayValid(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			TranscriptFrom(ctx).ToolResult(ctx, "t", "c", "line1\nline2", false)
		}()
	}
	wg.Wait()
	var doc transcriptDoc
	readJSON(t, sessionPath(s, "session-1", "coder-1", TranscriptFile), &doc)
	if len(doc.Events) != 50 {
		t.Fatalf("got %d events, want 50", len(doc.Events))
	}
	seen := map[int]bool{}
	for _, e := range doc.Events {
		if seen[e.Seq] {
			t.Fatalf("seq %d twice", e.Seq)
		}
		seen[e.Seq] = true
	}
}

func TestChatAndUnscopedContextsKeepNoTranscript(t *testing.T) {
	installStore(t, "session-1", true)
	if TranscriptFrom(WithChat(context.Background())) != nil {
		t.Error("the chat's record is session.json, not a transcript")
	}
	if TranscriptFrom(context.Background()) != nil {
		t.Error("no scope, no transcript")
	}
	var nilTr *Transcript
	nilTr.Input(context.Background(), "x") // nil-safe
	nilTr.Assistant(context.Background(), "m", &Response{})
}

func TestTranscriptBrokenFileIsCountedNotOverwritten(t *testing.T) {
	s, _ := installStore(t, "session-1", false)
	ctx := WithAgent(context.Background(), "coder", "coder-1")
	dir := sessionPath(s, "session-1", "coder-1")
	if _, err := s.folderDir("session-1", "coder-1"); err != nil {
		t.Fatal(err)
	}
	if err := writeFileBytes(dir+"/"+TranscriptFile, "{not json"); err != nil {
		t.Fatal(err)
	}
	TranscriptFrom(ctx).Input(ctx, "x")
	if n, _ := s.Failures(); n != 1 {
		t.Fatalf("failures = %d, want 1", n)
	}
	if got := readFile(t, dir+"/"+TranscriptFile); got != "{not json" {
		t.Fatalf("a file that is not ours must not be overwritten, got %q", got)
	}
}
