package observation

import (
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/retain"
)

// Withheld knowledge.
//
// A compile admits retrieved knowledge rows best-first until the share of the
// prompt budget the kernel gives retrieved knowledge is spent; a row that does
// not fit is left out whole, never cut. Its text is retained here and the
// prompt carries a pointer naming the row and its handle, so the model can
// read the row with recall_context when it needs it instead of the harness
// spending the window on it up front.

const knowledgeHandlePrefix = "obs:kn:"

const knowledgeRetentionKind = "knowledge_row"

// Knowledge retains withheld knowledge rows.
type Knowledge struct {
	store *retain.Store
}

// NewKnowledge creates a withheld-knowledge store over its own retention.
func NewKnowledge(cfg retain.Config) *Knowledge {
	cfg.Prefix = knowledgeHandlePrefix
	return &Knowledge{store: retain.New(cfg)}
}

var (
	sharedKnowledgeOnce sync.Once
	sharedKnowledge     *Knowledge
)

// SharedKnowledge is the process-wide withheld-knowledge store: the compile
// that withholds a row and the recall that reads it are different call sites.
func SharedKnowledge() *Knowledge {
	sharedKnowledgeOnce.Do(func() {
		sharedKnowledge = NewKnowledge(retain.DefaultConfig())
	})
	return sharedKnowledge
}

// KnowledgeHandlePrefix is the prefix every withheld-knowledge handle carries.
func KnowledgeHandlePrefix() string { return knowledgeHandlePrefix }

// Retain keeps a withheld row's full text and returns its handle, "" when it
// could not be retained (empty text or a store too small for it).
func (k *Knowledge) Retain(text string) string {
	if k == nil || text == "" {
		return ""
	}
	return k.store.Mint(knowledgeRetentionKind, []byte(text))
}

// Read returns a withheld row's text: lines [offset, offset+limit), the rest
// from offset when limit is not positive. The reply says which lines it holds
// when it is not the whole row.
func (k *Knowledge) Read(handle string, offset, limit int) (string, error) {
	if k == nil {
		return "", fmt.Errorf("withheld knowledge unavailable")
	}
	handle = strings.TrimSpace(handle)
	if !strings.HasPrefix(handle, knowledgeHandlePrefix) {
		return "", fmt.Errorf("%q is not a withheld-knowledge handle (%s...)", handle, knowledgeHandlePrefix)
	}
	kind, payload, err := k.store.Get(handle)
	if err != nil {
		return "", fmt.Errorf("withheld knowledge %s: %w", handle, err)
	}
	if kind != knowledgeRetentionKind {
		return "", fmt.Errorf("withheld knowledge %s: unexpected kind %q", handle, kind)
	}
	lines := strings.Split(string(payload), "\n")
	if offset < 0 {
		offset = 0
	}
	if offset >= len(lines) {
		return "", fmt.Errorf("withheld knowledge %s has %d lines; offset %d is past the end", handle, len(lines), offset)
	}
	end := len(lines)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	body := strings.Join(lines[offset:end], "\n")
	if offset == 0 && end == len(lines) {
		return body, nil
	}
	return fmt.Sprintf("[withheld knowledge %s lines %d-%d of %d]\n%s", handle, offset+1, end, len(lines), body), nil
}
