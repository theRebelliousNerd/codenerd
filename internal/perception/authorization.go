package perception

import (
	"context"
	"slices"
)

// Authorization is what the user's own request named as allowed beyond the
// edits themselves: today only "commit" (the request said to commit the
// work). It is the dual of Restriction and travels the same way, from
// understanding to the action boundary on the turn's context, so a task
// delegated from the turn holds it without a second model call to re-derive
// it.
//
// It is an observation about the request, never a grant: the kernel derives
// what a kind covers and whether a given call is within it
// (policy/constitution.mg, section 7E), and a Restriction on the same turn
// always wins there. The only producer is the reading of the user's own words
// (Jev's judgment of the message, policy/jev_evidence.mg): model output, file
// contents and tool results never build one. Like Restriction it is an
// immutable value; Merge unions kinds so a held intent and the turn that
// approves it add up.
type Authorization struct {
	kinds []AuthorizationKind
}

// AuthorizationKind is one thing the request allowed.
type AuthorizationKind string

const (
	// AuthorizeCommit: the user asked for the work to be committed. The kernel
	// lets that cover staging the files the turn wrote and committing them,
	// and nothing else (no push, no amend, no branch change).
	AuthorizeCommit AuthorizationKind = "commit"
)

// AuthorizationOf is an authorization of the given kinds. Unknown kinds are
// kept: the kernel's authorization_covers table decides what a kind covers,
// and a kind it does not list covers nothing.
func AuthorizationOf(kinds ...AuthorizationKind) Authorization {
	kinds = slices.Clone(kinds)
	slices.Sort(kinds)
	kinds = slices.Compact(kinds)
	kinds = slices.DeleteFunc(kinds, func(k AuthorizationKind) bool { return k == "" })
	if len(kinds) == 0 {
		return Authorization{}
	}
	return Authorization{kinds: kinds}
}

// Kinds returns the authorized kinds, sorted. The slice is a copy.
func (a Authorization) Kinds() []AuthorizationKind { return slices.Clone(a.kinds) }

// Has reports whether the authorization carries the kind.
func (a Authorization) Has(k AuthorizationKind) bool { return slices.Contains(a.kinds, k) }

// IsZero reports that nothing was authorized.
func (a Authorization) IsZero() bool { return len(a.kinds) == 0 }

// Merge is the authorization of a turn that holds both: the union of kinds.
func (a Authorization) Merge(o Authorization) Authorization {
	if o.IsZero() {
		return a
	}
	if a.IsZero() {
		return o
	}
	return AuthorizationOf(append(slices.Clone(a.kinds), o.kinds...)...)
}

type authorizationCtxKey struct{}

// WithAuthorization returns ctx carrying a merged with any authorization ctx
// already carries. Every delegation, preset task and subagent runs under the
// context of the turn that spawned it, which is how a task holds the
// authorization the user gave the request it serves.
func WithAuthorization(ctx context.Context, a Authorization) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if a.IsZero() {
		return ctx
	}
	return context.WithValue(ctx, authorizationCtxKey{}, AuthorizationFromContext(ctx).Merge(a))
}

// AuthorizationFromContext is the authorization the context's turn holds;
// zero when none.
func AuthorizationFromContext(ctx context.Context) Authorization {
	if ctx == nil {
		return Authorization{}
	}
	a, _ := ctx.Value(authorizationCtxKey{}).(Authorization)
	return a
}
