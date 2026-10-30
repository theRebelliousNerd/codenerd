package verification

import (
	"strings"
	"unicode/utf8"

	"codenerd/internal/observation"
)

// changeEvidence selects only a representation supported by the frozen
// observations. In particular, an unknown preimage is not an empty file,
// and an absent final file still has a reviewable deletion diff.
func changeEvidence(label string, w observation.FileWrite) EvidenceItem {
	it := EvidenceItem{Representation: "unavailable"}
	// Binary or invalid UTF-8 observations have identities, but no text
	// representation. Do not send them through text diffing or disclosure.
	for _, state := range []observation.FileState{w.Before, w.After} {
		if state.Known && state.Exists && (strings.ContainsRune(state.Content, '\x00') || !utf8.ValidString(state.Content)) {
			it.Representation = "binary"
			it.Body = "(binary content; text detail unavailable)"
			it.unavailable = "binary content has no retained text representation"
			return it
		}
	}
	switch {
	case !w.After.Known:
		it.Body = "(what the attempt left could not be read)"
		it.unavailable = "the final file state is unknown"
	case !w.Before.Known && !w.After.Exists:
		it.Body = "(the attempt deleted the file; preimage unknown, deleted content unavailable)"
		it.unavailable = "the deleted file's preimage is unknown"
	case !w.Before.Known:
		it.Representation = "postimage"
		it.Body = w.After.Content
		it.rest = "the remaining postimage bytes (preimage unknown)"
	default:
		it.Representation = "diff"
		before, after := "", ""
		if w.Before.Exists {
			before = w.Before.Content
		}
		if w.After.Exists {
			after = w.After.Content
		}
		it.Body = renderDiff(label, before, after)
		it.rest = "the remaining diff bytes"
		if it.Body == "" && !w.After.Exists {
			if w.Before.Exists {
				it.Body = "(the attempt deleted an empty file)"
			} else {
				it.Body = "(the path was absent before and after the attempt)"
			}
		}
	}
	return it
}
