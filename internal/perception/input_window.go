package perception

import (
	"errors"
	"fmt"

	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// ErrInputWindowExceeded is a request refused by a client whose input is
// capped (a slot's max_input_tokens, types.InputWindowReporter) because it is
// larger than the cap. It is refused before that request is sent, so the
// refusal itself cost nothing, and retrying the same request cannot help. A
// turn the endpoint paused is resumed by further requests; one of those
// refused here leaves the requests before it sent and billed.
var ErrInputWindowExceeded = errors.New("request is larger than the client's input window")

// admitInputWindow refuses a request larger than window, measured as the
// session executor measures it (prompt.EstimateRequestTokens), so a request
// the executor fit to the same window is never refused here. A window of 0 is
// no cap. A request that cannot be measured is refused too: a cap that cannot
// be checked is not a cap.
//
// declared is the tokens of tool declarations the client appends to the wire
// request that are not in tools (the Anthropic server web tools), measured by
// the caller as tools are. It is added to the count, and a refusal says so: it
// is still a lower bound, because the provider's own prompt for those tools is
// not measurable here.
//
// The message names the cap and avoids the "status NNN" shapes
// providerFailureStatus reads, so a credential pool passes it through as is
// instead of rotating to the next key.
func admitInputWindow(window int, provider Provider, model, system string, messages []types.Message, tools []types.ToolDefinition, declared int) error {
	if window <= 0 {
		return nil
	}
	tokens, err := prompt.EstimateRequestTokens(system, messages, tools)
	if err != nil {
		return fmt.Errorf("%s %s: the request could not be measured against the slot's max_input_tokens of %d, so it was not sent: %w",
			provider, model, window, err)
	}
	tokens += declared
	if tokens <= window {
		return nil
	}
	if declared > 0 {
		return fmt.Errorf("%w: %s %s would be sent at least %d input tokens (%d of them its server tool declarations; the provider's own prompt for those tools is not counted) and the slot's max_input_tokens is %d; that request was not sent",
			ErrInputWindowExceeded, provider, model, tokens, declared, window)
	}
	return fmt.Errorf("%w: %s %s would be sent about %d input tokens and the slot's max_input_tokens is %d; that request was not sent",
		ErrInputWindowExceeded, provider, model, tokens, window)
}

// userTurn is a single-prompt request's transcript in the shape the session
// executor measures: one user message.
func userTurn(user string) []types.Message {
	return []types.Message{{Role: "user", Text: user}}
}
