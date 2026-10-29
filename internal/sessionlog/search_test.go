package sessionlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListShowsIDStartAgentsAndSize(t *testing.T) {
	ws := t.TempDir()
	makeSession(t, ws, "session-a", 0, HistoryFile, "chat/0001-session.json", "coder-1/transcript.json")
	makeSession(t, ws, "session-b", 0)
	infos, err := List(ws)
	if err != nil || len(infos) != 2 {
		t.Fatalf("List = %+v, %v", infos, err)
	}
	a := infos[0]
	if a.ID != "session-a" || a.Start.Year() != 2026 || a.Bytes == 0 || a.Files != 3 {
		t.Fatalf("session-a: %+v", a)
	}
	if strings.Join(a.Agents, ",") != "chat,coder-1" {
		t.Fatalf("agents = %v", a.Agents)
	}
	if empty, _ := List(t.TempDir()); len(empty) != 0 {
		t.Fatal("no sessions dir lists nothing")
	}
}

func TestSearchFindsLinesWithSessionAgentFileAndTurn(t *testing.T) {
	s, _ := installStore(t, "session-1", true)

	// session.json: two user turns.
	hist := `{"session_id":"session-1","messages":[
	  {"role":"user","content":"first question"},
	  {"role":"assistant","content":"first answer"},
	  {"role":"user","content":"please run go test\nthen report"},
	  {"role":"assistant","content":"ok"}]}`
	if err := os.MkdirAll(sessionPath(s, "session-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileBytes(sessionPath(s, "session-1", HistoryFile), hist); err != nil {
		t.Fatal(err)
	}

	// A subagent transcript with the text on turn 4.
	ctx := WithTurn(WithAgent(context.Background(), "coder", "coder-9"), 4)
	tr := TranscriptFrom(ctx)
	tr.Input(ctx, "make it compile")
	tr.ToolResult(ctx, "run_build", "c1", "ok\nundefined: Frobnicate\nexit 2", true)

	// An exchange file of the chat.
	cctx, call := Begin(WithTurn(WithChat(context.Background()), 2), CallMeta{Purpose: "session"})
	_ = cctx
	call.End(Result{Response: &Response{Text: Lines("I ran go test myself\nall green")}})

	hits, err := Search(s.workspace, "GO TEST", SearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %+v", hits)
	}
	byFile := map[string]Hit{}
	for _, h := range hits {
		byFile[h.File] = h
	}
	h := byFile[HistoryFile]
	if h.Session != "session-1" || h.Agent != "chat" || h.Turn != 2 || h.Line != "please run go test" {
		t.Errorf("history hit: %+v", h)
	}
	h = byFile["0001-session.json"]
	if h.Agent != "chat" || h.Turn != 2 || h.Line != "I ran go test myself" {
		t.Errorf("exchange hit: %+v", h)
	}

	hits, _ = Search(s.workspace, "undefined: frobnicate", SearchOptions{Agent: "coder"})
	if len(hits) != 1 || hits[0].Agent != "coder-9" || hits[0].File != TranscriptFile || hits[0].Turn != 4 || hits[0].Line != "undefined: Frobnicate" {
		t.Fatalf("transcript hit: %+v", hits)
	}
	if hits, _ = Search(s.workspace, "undefined", SearchOptions{Agent: "chat"}); len(hits) != 0 {
		t.Fatalf("--agent chat must not see the coder's transcript: %+v", hits)
	}
	if hits, _ = Search(s.workspace, "undefined", SearchOptions{Session: "session-other"}); len(hits) != 0 {
		t.Fatalf("--session filter: %+v", hits)
	}
	if hits, _ = Search(s.workspace, "", SearchOptions{}); len(hits) != 0 {
		t.Fatal("an empty needle matches nothing")
	}
	// The schema/encoding boilerplate is not a match.
	if hits, _ = Search(s.workspace, "join with", SearchOptions{}); len(hits) != 0 {
		t.Fatalf("file boilerplate matched: %+v", hits)
	}
	_ = filepath.Join
}
