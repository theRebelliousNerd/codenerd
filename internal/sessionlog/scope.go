package sessionlog

import (
	"context"
	"sync"
)

type scopeKey struct{}
type turnKey struct{}
type taskKey struct{}

// Scope names the agent a model call belongs to, and so the folder its files
// land in. The harness installs one where an agent starts (a shard spawn, a
// subagent run, a campaign phase, the chat turn); everything the agent calls
// beneath that context inherits it.
type Scope struct {
	Role   string
	ID     string
	Folder string
	// Transcript says the agent keeps a transcript.json.
	Transcript bool

	state *scopeState
}

type scopeState struct {
	mu      sync.Mutex
	jit     *latest[JITReceipt]
	context *latest[ContextReceipt]
	working *WorkingSetNote
}

// WithAgent scopes ctx to an agent that keeps a transcript: its folder is
// "<role>-<id>".
func WithAgent(ctx context.Context, role, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, scopeKey{}, &Scope{
		Role: role, ID: id, Folder: AgentFolder(role, id), Transcript: true, state: &scopeState{},
	})
}

// WithChat scopes ctx to the main agent: calls land in chat/, and the
// conversation itself is session.json.
func WithChat(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if s := ScopeFrom(ctx); s != nil && s.Folder == ChatFolder {
		return ctx
	}
	return context.WithValue(ctx, scopeKey{}, &Scope{Role: "chat", Folder: ChatFolder, state: &scopeState{}})
}

// ScopeFrom returns the scope on ctx, or nil.
func ScopeFrom(ctx context.Context) *Scope {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(scopeKey{}).(*Scope)
	return s
}

// HasAgent reports whether ctx already carries a scope.
func HasAgent(ctx context.Context) bool { return ScopeFrom(ctx) != nil }

// WithTurn records the turn number calls beneath ctx belong to.
func WithTurn(ctx context.Context, n int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, turnKey{}, n)
}

// TurnFrom returns the turn number on ctx, or 0.
func TurnFrom(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	n, _ := ctx.Value(turnKey{}).(int)
	return n
}

// WithTask records the task id calls beneath ctx work on.
func WithTask(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, taskKey{}, id)
}

// TaskFrom returns the task id on ctx, or "".
func TaskFrom(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(taskKey{}).(string)
	return id
}
