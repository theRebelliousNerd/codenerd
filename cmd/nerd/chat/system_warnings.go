package chat

import (
	"fmt"
	"strings"

	"codenerd/internal/logging"
)

// renderSystemWarnings turns a turn's warnings into the block appended to the
// user's reply, and records each one as it goes.
//
// Every string here is something the user is about to read. Until this logged
// them, none of it survived the turn: a live chat session put three "[Kernel]
// Mangle update dropped" lines and a learnings-hydration failure on screen, and
// not one of them appeared in any of the 26 log files that session wrote. A
// headless run lost them entirely, and a warning nobody can grep after the fact
// is not a diagnostic.
//
// The recording belongs here, at the one place warnings become visible, rather
// than at the thirty-odd places a warning is appended: a warning added later is
// then durable by construction instead of by whoever remembers.
func renderSystemWarnings(warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n**System Warnings:**\n")
	for _, w := range warnings {
		logging.Get(logging.CategorySession).Warn("user-visible warning: %s", w)
		b.WriteString(fmt.Sprintf("- %s\n", w))
	}
	return b.String()
}
