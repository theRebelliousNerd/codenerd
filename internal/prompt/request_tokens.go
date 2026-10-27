package prompt

import (
	"encoding/json"
	"fmt"

	"codenerd/internal/types"
)

// EstimateRequestTokens is the input one model request carries, in the tokens
// EstimateTokens counts: the system prompt, the transcript and the tool
// catalog, the transcript and the catalog in their JSON form. An empty catalog
// costs nothing; the transcript is always counted.
//
// It is the one measure of a request's input. The session executor fits a
// working request to its window with it, and a client that caps its own input
// (types.InputWindowReporter) refuses with it, so a request the executor fit
// is never refused by the client that serves it.
func EstimateRequestTokens(system string, messages []types.Message, tools []types.ToolDefinition) (int, error) {
	transcript, err := json.Marshal(messages)
	if err != nil {
		return 0, fmt.Errorf("encode the transcript for its token estimate: %w", err)
	}
	total := EstimateTokens(system) + EstimateTokens(string(transcript))
	if len(tools) == 0 {
		return total, nil
	}
	catalog, err := json.Marshal(tools)
	if err != nil {
		return 0, fmt.Errorf("encode the tool catalog for its token estimate: %w", err)
	}
	return total + EstimateTokens(string(catalog)), nil
}
