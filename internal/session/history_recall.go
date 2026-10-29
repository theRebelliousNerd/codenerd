package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// historyHandlePrefix marks the handle an eviction notice names for the
// conversation messages the history window dropped. It shares the obs: family
// with the other retained-content handles recall_context redeems.
const historyHandlePrefix = "obs:hist:"

// historyEvictionHandle names one set of evicted messages by their content,
// so a handle read in an earlier turn -- when the window dropped fewer, or
// different, messages -- is recognised as stale instead of answering with
// whatever is evicted now.
func historyEvictionHandle(evicted []types.Message) string {
	h := sha256.New()
	for _, m := range evicted {
		h.Write([]byte(m.Role))
		h.Write([]byte{0})
		h.Write([]byte(m.Text))
		h.Write([]byte{0})
	}
	return historyHandlePrefix + hex.EncodeToString(h.Sum(nil))[:16]
}

// historyRecall stands behind recall_context for a working loop: an eviction
// handle returns the conversation messages the loop's history window dropped,
// and every other id goes to the working set.
//
// "Evicted context must remain recoverable" is a requirement of the design,
// and the window's own notice said "the session still holds them" -- while no
// verb could fetch them. The messages sat in a field only tests read, which is
// how the deadcode gate came to report the reader unreachable (2026-09-24).
// A model asked "as I said earlier" about a turn the window evicted could see
// that something was gone and had no way to get it back.
type historyRecall struct {
	working tools.ContextRecall
	handle  string
	evicted []types.Message
}

// Recall returns a page of the evicted conversation for its handle, and
// otherwise whatever the working set holds under id.
func (h historyRecall) Recall(ctx context.Context, id string, offset, limit int) (string, error) {
	trimmed := strings.TrimSpace(id)
	if !strings.HasPrefix(trimmed, historyHandlePrefix) {
		if h.working == nil {
			return "", fmt.Errorf("working context recall unavailable")
		}
		return h.working.Recall(ctx, id, offset, limit)
	}
	if len(h.evicted) == 0 {
		return "", fmt.Errorf("no conversation messages are evicted from this turn's window; %s names none", trimmed)
	}
	if trimmed != h.handle {
		return "", fmt.Errorf("%s names an earlier eviction; this turn's window evicted %d message(s), recall_context id=%q returns them",
			trimmed, len(h.evicted), h.handle)
	}
	body := []rune(renderEvictedHistory(h.evicted))
	if offset < 0 || offset > len(body) || limit < 0 {
		return "", fmt.Errorf("invalid history page bounds: offset %d, limit %d, %d chars", offset, limit, len(body))
	}
	end := len(body)
	if limit > 0 {
		end = min(len(body), offset+limit)
	}
	data, err := json.Marshal(struct {
		Handle   string `json:"handle"`
		Messages int    `json:"evicted_messages"`
		Page     string `json:"conversation"`
		Offset   int    `json:"offset"`
		Total    int    `json:"total_chars"`
		Next     int    `json:"next_offset"`
	}{h.handle, len(h.evicted), string(body[offset:end]), offset, len(body), end})
	return string(data), err
}

// Search is the working set's: the archive search recall_context offers when
// an id is unknown.
func (h historyRecall) Search(ctx context.Context, query string, offset, limit int) (string, error) {
	search, ok := h.working.(interface {
		Search(context.Context, string, int, int) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("context search unavailable")
	}
	return search.Search(ctx, query, offset, limit)
}

// renderEvictedHistory is the evicted conversation, oldest first, one message
// per role-labelled paragraph.
func renderEvictedHistory(evicted []types.Message) string {
	var b strings.Builder
	for i, m := range evicted {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(m.Text)
	}
	return b.String()
}
