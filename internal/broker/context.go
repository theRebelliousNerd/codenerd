package broker

import "context"

type purposeKey struct{}
type requireExactKey struct{}

// WithPurpose tags a context so that every inference call made under it is
// attributed to the named subsystem. Call sites deep inside a tool loop
// inherit the purpose of the turn that started them, which is what makes
// "cost per session turn" computable at all.
//
// Nesting is innermost-wins, because context values shadow. A compression pass
// running inside a session turn is tagged compression, not session. That is the
// semantics a per-purpose cap needs: capping compression is only meaningful if
// compression's own spend is what lands in the compression account, whatever
// happened to invoke it.
//
// The consequence is that a purpose is the *kind* of work, not the container it
// ran in. "What did this campaign cost?" is not a purpose question -- the
// campaign's per-task turns are tagged session and its shards subagent -- it is
// a scope question, answered by grouping receipts on Receipt.Scope. What stays
// tagged campaign is the orchestrator's own planning and checkpointing, which
// is a genuinely useful number on its own: it is the overhead the campaign
// machinery charges on top of the work it dispatches.
func WithPurpose(ctx context.Context, p Purpose) context.Context {
	if ctx == nil || p == "" {
		return ctx
	}
	return context.WithValue(ctx, purposeKey{}, p)
}

// PurposeFromContext returns the purpose tagged on ctx, or PurposeUnattributed.
//
// Unattributed is a real account rather than a silent discard: spend that
// reaches the provider without a purpose is a wiring bug, and it should show up
// as a growing number somebody notices rather than disappear.
func PurposeFromContext(ctx context.Context) Purpose {
	if ctx == nil {
		return PurposeUnattributed
	}
	if p, ok := ctx.Value(purposeKey{}).(Purpose); ok && p != "" {
		return p
	}
	return PurposeUnattributed
}

// WithRequireExact marks a context as requiring an exact token count. Requests
// made under it are refused rather than admitted on an estimate.
//
// Use this where being wrong is worse than being slow — a request sized to sit
// just under a hard context limit, for example. Most callers should not set it:
// on providers with no counting endpoint it makes every call fail.
func WithRequireExact(ctx context.Context) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, requireExactKey{}, true)
}

// RequiresExact reports whether ctx demands an exact count.
func RequiresExact(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	req, _ := ctx.Value(requireExactKey{}).(bool)
	return req
}
