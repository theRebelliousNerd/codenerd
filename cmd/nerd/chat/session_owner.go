package chat

import (
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
)

// A session owns its conversation: its transcript, its compressed memory,
// and the conversational facts the kernel holds for it (the brief, the
// session notes, the held proposal, the shard results follow-ups read).
// Selecting another session -- /new-session, loading one, boot -- detaches
// the outgoing conversation and installs the selected one, and a save
// deferred under a session that is no longer open writes nothing (the
// switch saved it synchronously, and a late copy would regress it or stamp
// the open session's memory with the old identity).

// selectedSession is the open session's id, shared across Model copies.
type selectedSession struct {
	mu sync.Mutex
	id string
}

func (s *selectedSession) set(id string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.id = id
}

// open reports whether id is the open session. Before any session is
// selected (and with no tracker) every save is the open session's.
func (s *selectedSession) open(id string) bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id == "" || s.id == id
}

// conversationPredicates are the kernel facts a conversation owns: the brief
// and the session notes (session_note is "a note for the current session",
// schemas_context.mg).
var conversationPredicates = append([]string{"session_note"}, perception.BriefPredicates...)

// detachConversation removes the outgoing conversation's state from the
// shared kernel and the model. Workspace knowledge stays.
func (m Model) detachConversation() Model {
	m.pendingWork = nil
	m.goAheadWork = nil
	m.lastShardResult = nil
	m.shardResultHistory = nil
	if m.kernel == nil {
		return m
	}
	if m.turn != nil {
		m.turn.decide.Lock()
		defer m.turn.decide.Unlock()
	}
	tx := m.kernel.Transaction()
	for _, p := range conversationPredicates {
		tx.Retract(p)
	}
	if err := tx.Commit(); err != nil {
		logging.Get(logging.CategorySession).Error("session switch: detach conversation facts: %v", err)
	}
	return m
}
