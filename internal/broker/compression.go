package broker

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// maxCompressionRetries bounds how many times a cut answer is sent back to the
// model to be restated within the limit. Two is enough to tell an answer that
// was merely verbose from one that genuinely cannot fit, and the second kind
// must reach the caller as an error rather than be retried into a budget hole.
const maxCompressionRetries = 2

// compressionInstruction is what the model is told when its previous answer to
// the same request was cut at the output ceiling. It asks for the same content,
// not less of it: compression, not omission, and no deferral. That is the
// rule this repo runs on — an output that does not fit is sent back to be said
// in fewer characters, never cut and never quietly accepted.
func compressionInstruction(t *types.OutputTruncated) string {
	limit := "the output limit"
	if t.LimitTokens > 0 {
		limit = fmt.Sprintf("the output limit of %d tokens", t.LimitTokens)
	}
	after := ""
	if t.OutputTokens > 0 {
		after = fmt.Sprintf(" after %d tokens", t.OutputTokens)
	}
	return fmt.Sprintf("Your previous response to this exact request was cut off%s because it reached %s. "+
		"Produce the complete response again so that it fits within the limit: keep every required element "+
		"and every structural requirement of the format, compress the wording, and drop repetition and any "+
		"quoted material the reader already has. Do not stop early, do not summarise what you would have "+
		"said, and do not defer any part of it to a later turn.", after, limit)
}

// withInstruction appends the compression instruction to a prompt.
func withInstruction(prompt, instruction string) string {
	if instruction == "" {
		return prompt
	}
	if prompt == "" {
		return instruction
	}
	return prompt + "\n\n" + instruction
}

// metered runs one provider call under admission and settlement. It is the
// single path every call takes: admit, observe, call, settle.
func metered[T any](c *core, ctx context.Context, req *Request, usage func(T) *types.UsageMetadata, call func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	receipt, refusal := c.admit(ctx, req)
	if refusal != nil {
		c.settleRefusal(receipt)
		return zero, refusal
	}
	ctx, obs := c.observed(ctx)
	out, err := call(ctx)
	var reported *types.UsageMetadata
	if usage != nil {
		reported = usage(out)
	}
	c.settle(receipt, req, obs, reported, err)
	return out, err
}

// compressing runs a call through metered and, when the provider reports that
// the output was cut at its ceiling, runs it again with a compression
// instruction appended — to the system prompt, or to the prompt itself for a
// method that has no system prompt (toUser) — up to maxCompressionRetries
// times. Every pass is admitted and settled on its own, because every pass was
// billed.
//
// The partial output never escapes: after the last retry the caller receives
// the typed error and no text, so nothing downstream can mistake a document
// with its ending missing for an answer.
func compressing[T any](c *core, ctx context.Context, req *Request, toUser bool, usage func(T) *types.UsageMetadata, call func(ctx context.Context, instruction string) (T, error)) (T, error) {
	instruction := ""
	for attempt := 0; ; attempt++ {
		attemptReq := req
		if instruction != "" {
			// The counter must see the prompt the provider will see.
			copyReq := *req
			if toUser {
				copyReq.User = withInstruction(copyReq.User, instruction)
			} else {
				copyReq.System = withInstruction(copyReq.System, instruction)
			}
			attemptReq = &copyReq
		}
		out, err := metered(c, ctx, attemptReq, usage, func(ctx context.Context) (T, error) {
			return call(ctx, instruction)
		})
		trunc, cut := types.AsOutputTruncated(err)
		if !cut {
			return out, err
		}
		if attempt >= maxCompressionRetries {
			logging.Get(logging.CategoryAPI).Error(
				"%s: output still cut at the provider's ceiling after %d restatements; failing the call rather than accepting a partial answer (%v)",
				req.Method, maxCompressionRetries, trunc)
			return out, err
		}
		logging.Get(logging.CategoryAPI).Warn(
			"%s: output cut at the provider's ceiling (%v); asking the model to restate within the limit (restatement %d of %d)",
			req.Method, trunc, attempt+1, maxCompressionRetries)
		instruction = compressionInstruction(trunc)
	}
}
