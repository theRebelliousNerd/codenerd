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
	// Name the file every warning lands in, with the session's totals so
	// far. This block is part of the assistant message: it enters the
	// conversation history, which is how the model itself learns where its
	// own warnings and logs live — until this line existed it could only
	// guess, and answered that it had no access to them.
	if path, warns, errs := logging.ProblemsSnapshot(); path != "" {
		b.WriteString(fmt.Sprintf("- Session log so far: %d error(s), %d warning(s), mirrored in `%s`\n", errs, warns, path))
	}
	return b.String()
}
