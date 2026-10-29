package chat

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	ctxcompress "codenerd/internal/context"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/store"
	"codenerd/internal/types"
)

// session_owner_test.go: a session owns its conversation. Starting fresh is
// fresh, loading B loads B, and a save made under A never stamps B's memory
// or regresses A.

// ownedSessionModel is session A with compressed memory, a brief, a note and
// a held proposal, on a real kernel, compressor and local store.
func ownedSessionModel(t *testing.T) Model {
	t.Helper()
	m, _ := journeyModel(t, config.WorkModePropose)
	db, err := store.NewLocalStore(filepath.Join(m.workspace, ".nerd", "knowledge.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m.localDB = db
	ck, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.compressor = ctxcompress.NewCompressor(ck, nil, m.client)
	m.selected = &selectedSession{}
	m.sessionID = "sess_a"
	m.selected.set("sess_a")
	if err := m.compressor.LoadState(&ctxcompress.CompressedState{SessionID: "sess_a", TurnNumber: 3,
		RollingSummary: ctxcompress.RollingSummary{Text: "A: the auth module", Segments: []ctxcompress.HistorySegment{{}}}}); err != nil {
		t.Fatal(err)
	}
	m.recordBrief(&perception.Brief{Goal: "harden the auth module"})
	if err := m.kernel.Assert(core.Fact{Predicate: "session_note", Args: []any{types.MangleString("current_focus"), types.MangleString("auth module")}}); err != nil {
		t.Fatal(err)
	}
	m.pendingWork = &pendingWork{Input: "rewrite auth", At: time.Now()}
	m.history = []Message{{Role: "user", Content: "harden auth"}, {Role: "assistant", Content: "proposed"}}
	return m
}

func assertConversationDetached(t *testing.T, m Model) {
	t.Helper()
	if b := m.currentBrief(); b != nil && b.Goal != "" {
		t.Fatalf("the old brief is still in the kernel: %+v", b)
	}
	if rows, _ := m.kernel.Query("session_note"); len(rows) != 0 {
		t.Fatalf("the old session note is still in the kernel: %v", rows)
	}
	if m.pendingWork != nil {
		t.Fatal("the old held proposal can still be approved with go")
	}
}

func TestNewSession_StartsFresh(t *testing.T) {
	t.Parallel()
	m := ownedSessionModel(t)
	next, _ := m.handleCmdNewSession("/new-session", []string{"/new-session"})
	m = next.(Model)
	if m.sessionID == "sess_a" {
		t.Fatal("no new session id")
	}
	if got := m.compressor.GetSessionID(); got != m.sessionID {
		t.Fatalf("compressor owned by %q, session is %q", got, m.sessionID)
	}
	if m.compressor.IsCompressionActive() {
		t.Fatal("the new session started with session A's compressed memory")
	}
	assertConversationDetached(t, m)
	if stateJSON, _, _, err := m.localDB.LoadLatestCompressedState(m.sessionID); err == nil && stateJSON != "" {
		state, err := ctxcompress.UnmarshalCompressedState([]byte(stateJSON))
		if err != nil || state.SessionID != m.sessionID || len(state.RollingSummary.Segments) != 0 {
			t.Fatalf("the new session's stored memory is %+v (err %v)", state, err)
		}
	}
	saved, err := session.LoadSessionHistory(m.workspace, "sess_a")
	if err != nil || len(saved.Messages) != 2 {
		t.Fatalf("session A was not saved before the switch: %+v %v", saved, err)
	}
	if last := lastAssistant(m); last == "" || !strings.Contains(last, "`sess_a` saved") {
		t.Fatalf("notice = %q", last)
	}
}

func TestLoadSession_DetachesTheOutgoingConversation(t *testing.T) {
	t.Parallel()
	m := ownedSessionModel(t)
	if err := session.SaveSessionHistory(m.workspace, "sess_b", []session.ChatMessage{{Role: "user", Content: "parse CSV"}}); err != nil {
		t.Fatal(err)
	}
	next, _ := m.loadSelectedSession("sess_b")
	m = next.(Model)
	assertConversationDetached(t, m)
	if got := m.compressor.GetSessionID(); got != "sess_b" {
		t.Fatalf("compressor owned by %q", got)
	}
}

// A compressed row stored under B that holds A's memory is not loaded as B's.
func TestLoadSession_RefusesAnotherSessionsStoredMemory(t *testing.T) {
	t.Parallel()
	m := ownedSessionModel(t)
	data, err := ctxcompress.MarshalCompressedState(m.compressor.GetState())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.localDB.StoreCompressedState("sess_b", 9, string(data), 0.5); err != nil {
		t.Fatal(err)
	}
	m.hydrateCompressorForSession("sess_b")
	if m.compressor.IsCompressionActive() || m.compressor.GetSessionID() != "sess_b" {
		t.Fatalf("B loaded A's memory (active=%v owner=%q)", m.compressor.IsCompressionActive(), m.compressor.GetSessionID())
	}
}

// A save deferred under A that runs after the switch to B writes nothing:
// not A's older history over the switch's save, not B's memory under A.
func TestDeferredSave_AfterTheSwitchWritesNothing(t *testing.T) {
	t.Parallel()
	m := ownedSessionModel(t)
	stale := m.saveSessionStateCmd()
	m.history = append(m.history, Message{Role: "user", Content: "one more"}, Message{Role: "assistant", Content: "ok"})
	if err := session.SaveSessionHistory(m.workspace, "sess_b", nil); err != nil {
		t.Fatal(err)
	}
	next, _ := m.loadSelectedSession("sess_b") // saves A's four messages, then switches
	m = next.(Model)
	if err := m.compressor.LoadState(&ctxcompress.CompressedState{SessionID: "sess_b", TurnNumber: 50,
		RollingSummary: ctxcompress.RollingSummary{Text: "B: CSV", Segments: []ctxcompress.HistorySegment{{}}}}); err != nil {
		t.Fatal(err)
	}
	stale()
	saved, err := session.LoadSessionHistory(m.workspace, "sess_a")
	if err != nil || len(saved.Messages) != 4 {
		t.Fatalf("the late save regressed A's history: %d messages (%v)", len(saved.Messages), err)
	}
	if stateJSON, _, _, err := m.localDB.LoadLatestCompressedState("sess_a"); err == nil && stateJSON != "" {
		state, _ := ctxcompress.UnmarshalCompressedState([]byte(stateJSON))
		if state != nil && state.SessionID != "sess_a" {
			t.Fatalf("A's row holds %q's memory", state.SessionID)
		}
	}
}
