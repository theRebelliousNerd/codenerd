package chat

import (
	"errors"
	"path/filepath"
	"testing"

	ctxcompress "codenerd/internal/context"
	"codenerd/internal/shards"
	"codenerd/internal/store"
)

func newTestCompressor() *ctxcompress.Compressor {
	return ctxcompress.NewCompressor(nil, nil, nil)
}

func newTestLocalStore(t *testing.T) *store.LocalStore {
	t.Helper()
	ls, err := store.NewLocalStore(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = ls.Close() })
	return ls
}

func marshalTestState(t *testing.T, sessionID string) string {
	t.Helper()
	raw, err := ctxcompress.MarshalCompressedState(&ctxcompress.CompressedState{SessionID: sessionID})
	if err != nil {
		t.Fatalf("MarshalCompressedState: %v", err)
	}
	return string(raw)
}

// TestPrepareBootHydration_BindsSession covers prepareBootHydration Reset +
// SetSessionID: prior session state is cleared and the new ID is bound.
func TestPrepareBootHydration_BindsSession(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.prepareBootHydration("sess-bind")
	before := m.turnCount
	m.applyBootHydratedState("", 0, "sess-bind")
	if m.turnCount != before {
		t.Fatalf("turnCount changed on empty hydration: got %d want %d", m.turnCount, before)
	}
	if got := m.compressor.GetSessionID(); got != "sess-bind" {
		t.Fatalf("compressor session = %q want sess-bind", got)
	}
}

// TestPrepareBootHydration_NilCompressorNoop covers the nil guard.
func TestPrepareBootHydration_NilCompressorNoop(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = nil
	m.prepareBootHydration("sess-nil")
}

// TestRunBootDeferred_EmptyDB covers the DB call with no snapshot (910-911
// execute, hasState stays false) plus observer Start (918-919).
func TestRunBootDeferred_EmptyDB(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.localDB = newTestLocalStore(t)
	m.observerMgr = shards.NewBackgroundObserverManager(nil)
	cmd := m.runBootDeferred("sess-empty")
	if cmd == nil {
		t.Fatal("runBootDeferred returned nil")
	}
	msg, ok := cmd().(bootDeferredMsg)
	if !ok {
		t.Fatalf("got %T want bootDeferredMsg", cmd())
	}
	if msg.sessionID != "sess-empty" {
		t.Errorf("sessionID = %q want sess-empty", msg.sessionID)
	}
	if msg.hasState {
		t.Error("hasState = true with empty DB, want false")
	}
}

// TestRunBootDeferred_LoadsSnapshot covers the success branch (912-915):
// a stored snapshot is returned with hasState true.
func TestRunBootDeferred_LoadsSnapshot(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	ls := newTestLocalStore(t)
	m.localDB = ls
	stored := marshalTestState(t, "sess-stored")
	if err := ls.StoreCompressedState("sess-stored", 7, stored, 0.5); err != nil {
		t.Fatalf("StoreCompressedState: %v", err)
	}
	msg, ok := m.runBootDeferred("sess-stored")().(bootDeferredMsg)
	if !ok {
		t.Fatal("want bootDeferredMsg")
	}
	if !msg.hasState {
		t.Fatal("hasState = false after storing snapshot, want true")
	}
	if msg.turnNumber != 7 {
		t.Errorf("turnNumber = %d want 7", msg.turnNumber)
	}
	if msg.stateJSON != stored {
		t.Error("stateJSON mismatch")
	}
}

// TestHandleBootDeferred_StaleWithObserverErr covers stale + observerErr.
func TestHandleBootDeferred_StaleWithObserverErr(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.sessionID = "sess-new"
	m.turnCount = 7
	stale := bootDeferredMsg{sessionID: "sess-old", observerErr: errors.New("observer boom")}
	updated, _ := m.Update(stale)
	if got := updated.(Model); got.sessionID != "sess-new" || got.turnCount != 7 {
		t.Errorf("stale mutated session: %+v", got)
	}
}

// TestHandleBootDeferred_StaleSuccessLogs covers stale + no error with manager.
func TestHandleBootDeferred_StaleSuccessLogs(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.sessionID = "sess-new"
	m.observerMgr = shards.NewBackgroundObserverManager(nil)
	updated, _ := m.Update(bootDeferredMsg{sessionID: "sess-old"})
	if got := updated.(Model); got.sessionID != "sess-new" {
		t.Errorf("sessionID = %q want sess-new", got.sessionID)
	}
}

// TestHandleBootDeferred_AppliesState covers hasState dispatch (941-942).
func TestHandleBootDeferred_AppliesState(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.sessionID = "sess-apply"
	m.compressor = newTestCompressor()
	m.turnCount = 1
	msg := bootDeferredMsg{sessionID: "sess-apply", hasState: true, stateJSON: "{invalid", turnNumber: 99}
	updated, _ := m.Update(msg)
	if got := updated.(Model); got.turnCount != 1 {
		t.Errorf("turnCount = %d want 1 (invalid snapshot must not advance)", got.turnCount)
	}
}

// TestHandleBootDeferred_ObserverErr covers non-stale observer error.
func TestHandleBootDeferred_ObserverErr(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.sessionID = "sess-obs-err"
	updated, _ := m.Update(bootDeferredMsg{sessionID: "sess-obs-err", observerErr: errors.New("start failed")})
	if got := updated.(Model); got.sessionID != "sess-obs-err" {
		t.Errorf("sessionID = %q", got.sessionID)
	}
}

// TestHandleBootDeferred_ObserverOK covers non-stale observer success log.
func TestHandleBootDeferred_ObserverOK(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.sessionID = "sess-obs-ok"
	m.observerMgr = shards.NewBackgroundObserverManager(nil)
	updated, _ := m.Update(bootDeferredMsg{sessionID: "sess-obs-ok"})
	if got := updated.(Model); got.sessionID != "sess-obs-ok" {
		t.Errorf("sessionID = %q", got.sessionID)
	}
}

// TestApplyBootHydratedState_NilCompressor covers the nil guard.
func TestApplyBootHydratedState_NilCompressor(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = nil
	m.turnCount = 5
	m.applyBootHydratedState(marshalTestState(t, "sess-nil"), 99, "sess-nil")
	if m.turnCount != 5 {
		t.Errorf("turnCount = %d want 5", m.turnCount)
	}
}

// TestApplyBootHydratedState_EmptyRefreshesBudget covers empty snapshot.
func TestApplyBootHydratedState_EmptyRefreshesBudget(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.turnCount = 5
	m.applyBootHydratedState("", 99, "sess-empty")
	if m.turnCount != 5 {
		t.Errorf("turnCount = %d want 5", m.turnCount)
	}
}

// TestApplyBootHydratedState_InvalidJSON covers the parse error.
func TestApplyBootHydratedState_InvalidJSON(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.turnCount = 5
	m.applyBootHydratedState("{not-json", 99, "sess-bad")
	if m.turnCount != 5 {
		t.Errorf("turnCount = %d want 5", m.turnCount)
	}
}

// TestApplyBootHydratedState_SessionMismatch covers the refuse-other-session branch.
func TestApplyBootHydratedState_SessionMismatch(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.turnCount = 5
	m.applyBootHydratedState(marshalTestState(t, "sess-other"), 99, "sess-mine")
	if m.turnCount != 5 {
		t.Errorf("turnCount = %d want 5 (mismatched snapshot refused)", m.turnCount)
	}
}

// TestApplyBootHydratedState_LoadsAndAdvancesTurn covers load + turn advance.
func TestApplyBootHydratedState_LoadsAndAdvancesTurn(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.prepareBootHydration("sess-match")
	m.turnCount = 3
	m.applyBootHydratedState(marshalTestState(t, "sess-match"), 9, "sess-match")
	if m.turnCount != 9 {
		t.Errorf("turnCount = %d want 9 (snapshot ahead advances)", m.turnCount)
	}
}

// TestApplyBootHydratedState_OlderTurnKeepsCount covers turnNumber <= turnCount.
func TestApplyBootHydratedState_OlderTurnKeepsCount(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.compressor = newTestCompressor()
	m.prepareBootHydration("sess-old-turn")
	m.turnCount = 9
	m.applyBootHydratedState(marshalTestState(t, "sess-old-turn"), 2, "sess-old-turn")
	if m.turnCount != 9 {
		t.Errorf("turnCount = %d want 9 (older snapshot must not rewind)", m.turnCount)
	}
}
