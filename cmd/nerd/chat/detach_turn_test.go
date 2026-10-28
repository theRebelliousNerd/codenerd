package chat

import (
	"context"
	"testing"

	"codenerd/internal/sessionlog"
)

// The streamed reply outlives its turn: it keeps the turn's values (here the
// chat's session-log scope and turn, which file its call under chat/), the
// turn's cancel does not stop it, and shutdown does.
func TestDetachFromTurn_KeepsValuesDropsTheTurnsCancelObeysShutdown(t *testing.T) {
	shutdown, quit := context.WithCancel(context.Background())
	defer quit()
	turn, endTurn := context.WithCancel(sessionlog.WithTurn(sessionlog.WithChat(context.Background()), 7))
	ctx, release := detachFromTurn(turn, shutdown)
	defer release()

	if sessionlog.ScopeFrom(ctx) == nil || sessionlog.TurnFrom(ctx) != 7 {
		t.Fatalf("the turn's session-log scope or turn was lost: scope=%v turn=%d", sessionlog.ScopeFrom(ctx), sessionlog.TurnFrom(ctx))
	}
	endTurn()
	if ctx.Err() != nil {
		t.Fatal("ending the turn cancelled the streamed reply")
	}
	quit()
	<-ctx.Done()
}
