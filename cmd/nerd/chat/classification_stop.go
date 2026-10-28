package chat

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
)

// classificationStop ends a turn whose classification failed.
//
// A failed classification still returns a degraded /explain intent with a nil
// error (the compatibility contract every caller relies on), and routing that
// read the intent as the user's request continued as an ordinary answer: a
// rejected credential or an unreadable model answer reached articulation with
// neither its cause nor its safe message. The firewall (or processInput, on the
// direct transducer path) publishes the typed outcome; the kernel derives the
// disposition and the wording (policy/perception_outcome.mg), and this returns
// that notice as the turn's reply. Held work's go-ahead is the one case the
// policy lets through, so a stop here never swallows an approval.
//
// Independently admitted evidence is untouched: a stop request Jev read
// (stop_background_requested) was acted on before routing, from judgments that
// do not depend on the classification. A successful judgment does not make the
// failed classification admitted, and the failed classification does not make a
// separate stop request failed.
func (m Model) classificationStop(intent perception.Intent, warnings []string) (tea.Msg, bool) {
	stop, err := perception.ClassificationStopFor(m.kernel, "/current_intent", intent)
	if err != nil {
		var already *perception.ClassificationStop
		if !errors.As(err, &already) {
			logging.RoutingError("[processInput] classification disposition not derived: %v", err)
		}
	}
	if stop == nil {
		return nil, false
	}
	logging.Routing("[processInput] CLASSIFICATION %s (%s): disposition=%s stage=%s class=%s method=%s",
		stop.Action, stop.Outcome.Code, stop.Outcome.Disposition, stop.Outcome.Stage, stop.Outcome.Failure, stop.Outcome.Method)
	// The warnings the turn has gathered so far -- a background job stopped on
	// Jev's word, a fact that did not assert -- still reach the user.
	return responseMsg(stop.Notice + renderSystemWarnings(warnings)), true
}
