package chat

import (
	"context"
	"fmt"

	"codenerd/internal/sessionlog"
)

// noteArticulationContext gives the session log the context selection behind
// the articulation call that follows: which turns the request carries, which
// were folded into the compressed session context, and the selector's own
// statistics (kernel versus Go fallback, and why). Nothing is recomputed; the
// numbers are the ones the compressor already keeps. A no-op unless debug mode
// is on.
func (m Model) noteArticulationContext(ctx context.Context, recent int, buildErr error) {
	if !sessionlog.Active() || m.compressor == nil {
		return
	}
	total := len(m.history)
	r := sessionlog.ContextReceipt{
		Source:       "compressor.GetContextString",
		SectionOrder: []string{"compressed_session_context", "recent_turns"},
		Stats:        m.compressor.GetSelectionStats(),
	}
	r.Included = append(r.Included, sessionlog.ContextItem{
		ID: "recent_turns", Kind: "history",
		Reason: fmt.Sprintf("the last %d of %d turns, inside the compressor's recent-turn window", recent, total),
	})
	if buildErr != nil {
		r.Excluded = append(r.Excluded, sessionlog.ContextItem{
			ID: "compressed_session_context", Kind: "history", Action: "excluded",
			Reason: "the compressed view could not be built: " + buildErr.Error(),
		})
	} else {
		r.Included = append(r.Included, sessionlog.ContextItem{
			ID: "compressed_session_context", Kind: "history",
			Reason: fmt.Sprintf("stands for the %d older turns outside the recent window", total-recent),
		})
	}
	sessionlog.NoteContext(ctx, r)
}
