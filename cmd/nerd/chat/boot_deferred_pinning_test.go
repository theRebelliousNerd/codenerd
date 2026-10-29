package chat

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHandleBootCompleteMsg_IncludesDeferredWiring pins
// Model.handleBootCompleteMsg itself: the fast boot handler must schedule the
// slow wiring off the Update loop as bootDeferredMsg.
//
// It drives the function named (not a helper) and asserts the behaviour the
// change was made for: the returned Batch contains a bootDeferredMsg for this
// session. Revert handleBootCompleteMsg to the synchronous version (no
// runBootDeferred in the Batch) and this fails: the Batch completes without
// any bootDeferredMsg.
func TestHandleBootCompleteMsg_IncludesDeferredWiring(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	// Keep approvals for syncWorkModeFacts but clear the host so
	// listenApprovals returns nil and the returned Batch completes without
	// blocking on a live approval channel.
	m.approvalHost = nil

	components := &SystemComponents{
		SessionID:       "sess-pin-batch",
		TurnCount:       2,
		InitialMessages: []Message{{Role: "assistant", Content: "pin", Time: time.Now()}},
		Approvals:       nil,
		ObserverMgr:     nil,
		LocalDB:         nil,
		Compressor:      nil,
	}

	start := time.Now()
	got, cmd := m.handleBootCompleteMsg(bootCompleteMsg{components: components})
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("handleBootCompleteMsg took %v, want <100ms", elapsed)
	}
	if cmd == nil {
		t.Fatal("handleBootCompleteMsg returned nil cmd, want Batch with deferred wiring")
	}
	gotModel, ok := got.(Model)
	if !ok {
		t.Fatalf("handleBootCompleteMsg returned %T, want Model", got)
	}
	if gotModel.sessionID != "sess-pin-batch" {
		t.Errorf("sessionID = %q, want sess-pin-batch", gotModel.sessionID)
	}

	// Execute the Batch off the loop with a guard so a future blocking
	// listener cannot hang the suite; with nil buses/host it completes fast.
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var raw tea.Msg
	select {
	case raw = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Batch from handleBootCompleteMsg did not return within 2s")
	}
	batch, ok := raw.(tea.BatchMsg)
	if !ok {
		t.Fatalf("Batch returned %T, want tea.BatchMsg", raw)
	}
	found := false
	for _, subCmd := range batch {
		if subCmd == nil {
			continue
		}
		subMsg := subCmd()
		if d, ok := subMsg.(bootDeferredMsg); ok {
			found = true
			if d.sessionID != "sess-pin-batch" {
				t.Errorf("bootDeferredMsg.sessionID = %q, want sess-pin-batch", d.sessionID)
			}
		}
	}
	if !found {
		t.Fatal("Batch from handleBootCompleteMsg contains no bootDeferredMsg; slow work is not offloaded")
	}
}

// TestUpdateCore_RoutesBootDeferredMsg pins Model.updateCore: bootDeferredMsg
// sent through Update must reach handleBootDeferredMsg.
//
// It drives the caller (Update/updateCore) and asserts the routed behaviour:
// a hydrated snapshot advances the turn counter. Remove the
// `case bootDeferredMsg` from updateCore and the message falls through to the
// viewport path, leaving turnCount unchanged, so this fails.
func TestUpdateCore_RoutesBootDeferredMsg(t *testing.T) {
	b := bootTestBroker(t)
	m := bootTestModel(t, b)
	comp := newTestCompressor()
	sess := "sess-route-pin"
	m.compressor = comp
	m.sessionID = sess
	m.turnCount = 5

	stateJSON := marshalTestState(t, sess)
	msg := bootDeferredMsg{
		sessionID:  sess,
		stateJSON:  stateJSON,
		turnNumber: 99,
		hasState:   true,
	}

	updated, _ := m.Update(msg)
	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update(bootDeferredMsg) returned %T, want Model", updated)
	}
	if got.turnCount != 99 {
		t.Fatalf("turnCount = %d after Update(bootDeferredMsg), want 99 (deferred snapshot not applied; updateCore did not route)", got.turnCount)
	}
	if got.sessionID != sess {
		t.Errorf("sessionID = %q, want %q", got.sessionID, sess)
	}
}
