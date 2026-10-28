package sessionlog

import (
	"context"
	"fmt"
)

// RunAgent runs one agent's task under its own scope and keeps its
// transcript: the task goes in before run starts, the outcome after it ends,
// on every path -- a returned error, a cancellation, a panic -- and each entry
// is on disk when written, so an agent that is killed mid-run leaves a
// transcript complete up to the kill.
//
// This is the harness's hook, installed where a shard is spawned, a subagent
// runs or a campaign task starts. The agent and the model do nothing for it.
func RunAgent(ctx context.Context, role, id, task string, run func(context.Context) (string, error)) (result string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = WithAgent(ctx, role, id)
	tr := TranscriptFrom(ctx)
	tr.Input(ctx, task)
	defer func() {
		if p := recover(); p != nil {
			tr.Final(ctx, "", fmt.Errorf("panic: %v", p))
			panic(p)
		}
		tr.Final(ctx, result, err)
	}()
	return run(ctx)
}
