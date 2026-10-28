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
//
// The transcript's JSON form is its public projection: text, tool calls and
// tool results. A turn also carries native blocks that projection has no place
// for -- a provider's server-tool results, replayed to it byte for byte, and
// signed reasoning -- and those are sent and billed. They are counted
// separately (nativeBlockTokens), so a history that replays a fetched page is
// not measured as one that replays a snippet.
func EstimateRequestTokens(system string, messages []types.Message, tools []types.ToolDefinition) (int, error) {
	transcript, err := json.Marshal(messages)
	if err != nil {
		return 0, fmt.Errorf("encode the transcript for its token estimate: %w", err)
	}
	total := EstimateTokens(system) + EstimateTokens(string(transcript)) + nativeBlockTokens(messages)
	if len(tools) == 0 {
		return total, nil
	}
	catalog, err := json.Marshal(tools)
	if err != nil {
		return 0, fmt.Errorf("encode the tool catalog for its token estimate: %w", err)
	}
	return total + EstimateTokens(string(catalog)), nil
}

// ServerContentTokens is the tokens one replayed server-tool block (raw is its
// provider JSON) puts in a request, in the tokens EstimateTokens counts. It is
// a bound, not a provider count: opaque encoded payloads are charged at what
// they decode to (types.ServerBlockChars), never as the base64 or ciphertext
// they travel as. A client that admits a request it builds from provider
// content outside any types.Message -- a paused turn it resumes -- counts that
// content with this.
func ServerContentTokens(raw []byte) int {
	return (types.ServerBlockChars(raw) + 3) / 4
}

// nativeBlockTokens is the tokens of what the messages' public projection
// omits and the provider is sent: server-tool blocks and signed reasoning. A
// redacted reasoning block is charged at the tokens the provider counted when
// it produced it where it reported them, because its encrypted signature runs
// ~13 characters per reasoning token and its length measures nothing. Unsigned
// reasoning is not replayed (the adapters drop it) and costs nothing.
func nativeBlockTokens(messages []types.Message) int {
	total := 0
	for i := range messages {
		for _, b := range messages[i].Content() {
			switch b.Kind {
			case types.BlockServerTool:
				total += ServerContentTokens(b.Raw)
			case types.BlockThinking:
				switch {
				case b.Signature == "":
				case b.Redacted && b.Tokens > 0:
					total += b.Tokens
				default:
					total += EstimateTokens(b.Text) + EstimateTokens(b.Signature)
				}
			}
		}
	}
	return total
}
