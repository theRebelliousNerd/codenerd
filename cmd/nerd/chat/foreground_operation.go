package chat

import (
	"context"
	"fmt"
	"runtime/debug"

	"codenerd/internal/logging"
	"codenerd/internal/sessionlog"

	tea "github.com/charmbracelet/bubbletea"
)

// foregroundOperation runs a slash command's task as the foreground
// execution, under the same slot contract as a typed message (processInput,
// turnState): the slot is taken now, on the caller's Update thread, before
// the task can be scheduled; the task's cancel is registered when it starts,
// so Ctrl+X reaches it; and the slot is released when it settles, so a
// message typed meanwhile waits in the queue instead of running beside it.
//
// /spawn and the other delegating commands used to set the loading flag and
// run under a bare session context: Ctrl+X found no owner to cancel, cleared
// the spinner, said "Stopped" while the task ran on, and the next message took
// the apparently free slot (audit slash-operation-slot-admission).
//
// The task keeps the session context's ancestry (quitting still cancels it)
// and its usage and session labels. A stop that lands before the task starts
// is latched by begin, and the task does no work. name is what a stop before
// start reports.
func (m Model) foregroundOperation(name string, run func(ctx context.Context) tea.Msg) tea.Cmd {
	turnID := m.turn.accept()
	return func() (result tea.Msg) {
		// Every exit, a recovered panic included, releases this owner only.
		defer m.turn.end(turnID)
		defer func() {
			if r := recover(); r != nil {
				logging.API("PANIC in %s (recovered): %v\n%s", name, r, debug.Stack())
				result = errorMsg(fmt.Errorf("internal error in %s (recovered panic): %v", name, r))
			}
		}()
		ctx, cancel := m.sessionOperationContext()
		defer cancel()
		ctx = sessionlog.WithTask(ctx, foregroundTask(turnID))
		m.turn.begin(turnID, cancel)
		if err := ctx.Err(); err != nil {
			return errorMsg(fmt.Errorf("%s stopped before it started: %w", name, err))
		}
		return run(ctx)
	}
}
