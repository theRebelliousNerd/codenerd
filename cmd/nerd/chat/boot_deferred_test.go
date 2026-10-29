package chat

import (
	"context"
	"testing"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/approval"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// bootTestModel returns a minimal Model with the UI pieces
// handleBootCompleteMsg touches (textarea, viewport, styles) plus the shared
// turn/background/selected handles InitChat always provides.
func bootTestModel(t *testing.T, b *approval.Broker) Model {
	t.Helper()
	ta := textarea.New()
	vp := viewport.New(80, 20)
	errVP := viewport.New(76, 10)
	m := Model{
		textarea:   ta,
		viewport:   vp,
		errorVP:    errVP,
		styles:     ui.DefaultStyles(),
		width:      120,
		height:     40,
		background: newBackgroundJobs(),
		turn:       newTurnState(),
		selected:   &selectedSession{},
		sessionID:  "pre-boot",
	}
	if err := m.attachApprovalHost(b); err != nil {
		t.Fatal(err)
	}
	// Shutdown ctx keeps ReportStatus / listeners from nil-deref paths.
	m.shutdownCtx, m.shutdownCancel = context.WithCancel(context.Background())
	return m
}

func bootTestBroker(t *testing.T) *approval.Broker {
	t.Helper()
	b, err := approval.NewBroker(approval.Options{TTL: time.Minute, PreviewMaxBytes: 4096, JournalRetention: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return b
}

// TestBootComplete_StaysUnder100msAndDefersSlowWork locks the performance
// contract from the problems log: handling bootCompleteMsg must stay under
// the 100ms Update target, with the slow wiring (compressor snapshot load,
// observer start) moved off the loop and delivered as bootDeferredMsg.
//
// It fails before the fix: runBootDeferred / bootDeferredMsg /
// handleBootDeferredMsg do not exist, so the boot handler does every slow
// step synchronously and there is no deferred message to apply.
func TestBootComplete_StaysUnder100msAndDefersSlowWork(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	now := time.Now()
	components := &SystemComponents{
		SessionID:       "sess-boot-fast",
		TurnCount:       3,
		InitialMessages: []Message{{Role: "assistant", Content: "✓ Shared bootstrap initialized", Time: now}},
		Approvals:       b,
		ObserverMgr:     nil,
		LocalDB:         nil,
		Compressor:      nil,
	}

	start := time.Now()
	updated, cmd := m.Update(bootCompleteMsg{components: components})
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Update(bootCompleteMsg) took %v, want <100ms", elapsed)
	}
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", updated)
	}
	if cmd == nil {
		t.Fatal("Update(bootCompleteMsg) returned nil cmd, want deferred wiring + scan batch")
	}

	// Nothing lost on the fast path: boot flags, session identity, initial
	// history and input placeholder are all set synchronously.
	if !got.isBooting {
		t.Error("isBooting = false after boot success, want true until scan completes")
	}
	if got.bootStage != BootStageScanning {
		t.Errorf("bootStage = %v, want BootStageScanning", got.bootStage)
	}
	if got.sessionID != "sess-boot-fast" {
		t.Errorf("sessionID = %q, want sess-boot-fast", got.sessionID)
	}
	if got.turnCount != 3 {
		t.Errorf("turnCount = %d, want 3", got.turnCount)
	}
	foundInit := false
	for _, msg := range got.history {
		if msg.Content == "✓ Shared bootstrap initialized" {
			foundInit = true
		}
	}
	if !foundInit {
		t.Error("InitialMessages not appended during fast boot handling")
	}
	if got.textarea.Placeholder != "Indexing workspace..." {
		t.Errorf("placeholder = %q, want Indexing workspace...", got.textarea.Placeholder)
	}

	// The slow work runs off the loop and returns as a message.
	deferredCmd := got.runBootDeferred(got.sessionID)
	if deferredCmd == nil {
		t.Fatal("runBootDeferred returned nil cmd, want bootDeferredMsg producer")
	}
	raw := deferredCmd()
	deferred, ok := raw.(bootDeferredMsg)
	if !ok {
		t.Fatalf("deferred cmd returned %T, want bootDeferredMsg", raw)
	}
	if deferred.sessionID != "sess-boot-fast" {
		t.Errorf("bootDeferredMsg.sessionID = %q, want sess-boot-fast", deferred.sessionID)
	}

	// Applying the deferred result is also fast and preserves identity.
	start = time.Now()
	applied, dcmd := got.Update(deferred)
	_ = dcmd
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("Update(bootDeferredMsg) took %v, want <100ms", elapsed)
	}
	final, ok := applied.(Model)
	if !ok {
		t.Fatalf("Update(bootDeferredMsg) returned %T, want Model", applied)
	}
	if final.sessionID != "sess-boot-fast" {
		t.Errorf("after deferred sessionID = %q, want sess-boot-fast", final.sessionID)
	}
	if final.turnCount != 3 {
		t.Errorf("after deferred turnCount = %d, want 3", final.turnCount)
	}
	if !final.isBooting {
		t.Error("after deferred isBooting = false, want true (scan still owns readiness)")
	}
}

// TestBootDeferred_StaleSessionRefusesHydration ensures the async gap cannot
// corrupt a session switch that landed before the deferred work returned.
func TestBootDeferred_StaleSessionRefusesHydration(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	m.sessionID = "sess-new"
	m.turnCount = 7

	stale := bootDeferredMsg{
		sessionID:  "sess-old",
		stateJSON:  `{"session_id":"sess-old"}`,
		turnNumber: 99,
		hasState:   true,
	}
	updated, _ := m.Update(stale)
	got := updated.(Model)
	if got.sessionID != "sess-new" {
		t.Errorf("sessionID = %q after stale deferred, want sess-new", got.sessionID)
	}
	if got.turnCount != 7 {
		t.Errorf("turnCount = %d after stale deferred, want 7 (no hydration from old session)", got.turnCount)
	}
}

// TestBootComplete_ErrorPathUnchanged keeps the failure lane intact: boot
// errors still unlock input immediately with the failure surfaced.
func TestBootComplete_ErrorPathUnchanged(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	updated, _ := m.Update(bootCompleteMsg{err: context.DeadlineExceeded})
	got := updated.(Model)
	if got.isBooting {
		t.Error("isBooting = true after boot failure, want false (input unlocked)")
	}
	if got.err == nil {
		t.Error("err = nil after boot failure, want the boot error surfaced")
	}
}

// Ensure the new message type flows through the tea runtime as a message.
var _ tea.Msg = bootDeferredMsg{}
