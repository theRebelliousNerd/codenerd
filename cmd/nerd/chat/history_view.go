package chat

import (
	"fmt"

	"codenerd/internal/logging"
)

// articulationHistoryView is the compressed-history block of a reply's
// context once compression is active: the compressor's view when it was
// built. When it was not, the recent window still bounds the request -- the
// whole archive is not poured back in -- and the block states that the
// earlier messages are not in this context and why, so their absence is
// part of what the reply is told rather than something it cannot see.
// omitted is the number of earlier messages outside the recent window.
func articulationHistoryView(view string, err error, omitted int) string {
	if err == nil {
		return view
	}
	logging.Get(logging.CategoryContext).Warn("compressed history not built for this reply (%d earlier message(s) left out): %v", omitted, err)
	if omitted <= 0 {
		return ""
	}
	return fmt.Sprintf("[Conversation coverage] The %d earlier message(s) of this conversation are not in this context: their compressed summary could not be built (%v). Only the recent turns below are.", omitted, err)
}
