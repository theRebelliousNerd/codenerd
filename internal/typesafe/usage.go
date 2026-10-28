package typesafe

import (
	"context"
	"strings"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Provider is the usage report's provider name for every Jev call.
const Provider = "typesafe"

// defaultOperation names a call whose caller did not say which use it serves.
const defaultOperation = "jev"

type operationKey struct{}

// WithOperation names the use a call serves (turn judgments, retrieval
// relevance, ...) in its usage report, so the cost of each use is its own
// row in the usage tracker.
func WithOperation(ctx context.Context, op string) context.Context {
	if strings.TrimSpace(op) == "" {
		return ctx
	}
	return context.WithValue(ctx, operationKey{}, op)
}

func operationFrom(ctx context.Context) string {
	if op, ok := ctx.Value(operationKey{}).(string); ok {
		return op
	}
	return defaultOperation
}

// recordUsage reports one answered call to the usage tracker and observer the
// context carries (usage.RecordFromContext): the model that answered, and the
// billed input tokens. A call with no tracker on its context records nothing.
func (c *Client) recordUsage(ctx context.Context, resp *Response) {
	model := resp.Model
	if model == "" {
		model = c.model
	}
	usage.RecordFromContext(ctx, usage.Report{
		Model:     model,
		Provider:  Provider,
		Operation: operationFrom(ctx),
		Usage: types.UsageMetadata{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
			Known:        types.UsageTokens,
		},
	})
}
