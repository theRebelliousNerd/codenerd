package session

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/projectdoc"
	"codenerd/internal/types"
)

// What the user's request allowed beyond editing ("fix X, then commit it")
// reaches the shell-effect gate here. Go measures: the kinds the turn's
// context carries (perception.Authorization, read from the user's own words
// by Jev), the turn that owns the call, the files the turn has written, and
// the shape of the call. The kernel decides (policy/constitution.mg, section
// 7E). Nothing the model or a tool returned can build an authorization: the
// only producer of the context value is the chat turn that read the user's
// message, and a turn that carries none asserts nothing here.

// assertTurnAuthorization tells the kernel which kinds the request allowed
// this turn. A kind that could not be recorded is an error: the turn then
// runs with the gate's default, which refuses, and says why.
func (e *Executor) assertTurnAuthorization(turn types.MangleAtom, a perception.Authorization) error {
	if a.IsZero() {
		return nil
	}
	if e.kernel == nil {
		return fmt.Errorf("turn authorization %v needs a kernel to be decided", a.Kinds())
	}
	for _, kind := range a.Kinds() {
		fact := types.Fact{Predicate: "turn_authorization", Args: []any{turn, types.MangleAtom("/" + string(kind))}}
		if !e.assertTurnFact(fact) {
			return fmt.Errorf("turn authorization %q could not be recorded in the kernel", kind)
		}
	}
	logging.Session("Turn %s authorized by the request: %v", turn, a.Kinds())
	return nil
}

// withShellScope puts the turn's identity, workspace and live write set on
// its context, so the executor's gate and the tool-layer guard decide the same
// call from the same measurements.
func (e *Executor) withShellScope(ctx context.Context, turn types.MangleAtom, result *ExecutionResult) context.Context {
	return projectdoc.WithShellScope(ctx, projectdoc.ShellScope{
		Turn:      turn,
		Workspace: e.workspaceForVerification(),
		WriteSet: func() []string {
			if result == nil {
				return nil
			}
			return append([]string(nil), result.WrittenPaths...)
		},
	})
}

// authorizeShellMutation asks whether the turn's own request answers the
// shell-effect refusal. It returns the authorized step, or the refusal
// (possibly with what the request did allow, so the model does not guess).
func (e *Executor) authorizeShellMutation(ctx context.Context, call ToolCall, refusal error) (string, error) {
	if e.kernel == nil {
		return "", refusal
	}
	return projectdoc.AuthorizeShellMutation(ctx, e.kernel, call.Name, call.Args, refusal)
}
