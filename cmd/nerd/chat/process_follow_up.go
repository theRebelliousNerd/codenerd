// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains conversation-context helpers and agent creation.
//
// HISTORY: this file used to hold a pre-perception follow-up substring
// detector (detectFollowUpQuestion / handleFollowUpQuestion). It was removed
// when routing arbitration moved to the kernel: the detector hijacked any
// input containing "what is the"/"that"/"this" whenever a shard had run
// earlier in the session, answering about the last shard result instead of
// the user's actual question. Follow-ups now flow through the single
// perception → route_decision → articulation pipeline; articulation carries
// lastShardResult and the shard history blackboard in ConversationContext, so
// "why is that bad?" still answers from prior context — deterministically.
package chat

// getRecentTurns returns the last N conversation turns.
func (m Model) getRecentTurns(n int) []Message {
	if len(m.history) <= n {
		return m.history
	}
	return m.history[len(m.history)-n:]
}
