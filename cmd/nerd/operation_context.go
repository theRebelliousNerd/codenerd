package main

import (
	"context"

	"codenerd/internal/config"

	"github.com/spf13/cobra"
)

// operationContext bounds a whole command by --timeout when the user set one.
//
// There is no default. An agentic run can take hours, and one that stops
// making progress is stopped by the working policy (a derived stall or a
// repeated failure), not by a clock: until 2026-09-19 every command ran under
// a 25-minute default, and ladder run R1-4d's review was cut at 25:01 of a
// turn that was still working. A zero or negative --timeout means no
// deadline; it used to mean one that had already passed.
func operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, timeout)
}

// commandContext is the context a subcommand runs on. The only deadline is
// the user's --timeout, applied by operationContext; unset means none. The
// cobra command's context stays the parent, so cancellation still propagates.
func commandContext(cmd *cobra.Command) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if cmd != nil {
		if ctx := cmd.Context(); ctx != nil {
			parent = ctx
		}
	}
	return operationContext(parent)
}

// llmCallContext bounds one LLM call by llm_timeouts.per_call_timeout.
// That duration is a request bound: the command deadline, when --timeout is
// set, is already on parent and is the one that fires when it is shorter.
// A non-positive value adds no second clock.
func llmCallContext(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	d := config.GetLLMTimeouts().PerCallTimeout
	if d <= 0 {
		return context.WithCancel(parent)
	}
	return context.WithTimeout(parent, d)
}
