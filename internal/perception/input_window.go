package perception

import (
	"errors"
	"fmt"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// ErrInputWindowExceeded is a request refused by a client whose input is
// capped (a slot's max_input_tokens, types.InputWindowReporter) because it is
// larger than the cap. It is refused before anything is sent, so it cost
// nothing, and retrying the same request cannot help.
var ErrInputWindowExceeded = errors.New("request is larger than the client's input window")

// admitInputWindow refuses a request larger than window, measured as the
// session executor measures it (prompt.EstimateRequestTokens), so a request
// the executor fit to the same window is never refused here. A window of 0 is
// no cap. A request that cannot be measured is refused too: a cap that cannot
// be checked is not a cap.
//
// The message names the cap and avoids the "status NNN" shapes
// providerFailureStatus reads, so a credential pool passes it through as is
// instead of rotating to the next key.
func admitInputWindow(window int, provider Provider, model, system string, messages []types.Message, tools []types.ToolDefinition) error {
	if window <= 0 {
		return nil
	}
	tokens, err := prompt.EstimateRequestTokens(system, messages, tools)
	if err != nil {
		return fmt.Errorf("%s %s: the request could not be measured against the slot's max_input_tokens of %d, so it was not sent: %w",
			provider, model, window, err)
	}
	if tokens <= window {
		return nil
	}
	return fmt.Errorf("%w: %s %s would be sent about %d input tokens and the slot's max_input_tokens is %d; nothing was sent",
		ErrInputWindowExceeded, provider, model, tokens, window)
}

// userTurn is a single-prompt request's transcript in the shape the session
// executor measures: one user message.
func userTurn(user string) []types.Message {
	return []types.Message{{Role: "user", Text: user}}
}
