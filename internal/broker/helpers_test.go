package broker

import (
	"context"
	"time"
)

// timeAfter is a shared deadline for tests that must prove termination.
func timeAfter() <-chan time.Time { return time.After(5 * time.Second) }

// nil2ctx returns a background context. Named for the call sites that read
// better without an inline context.Background().
func nil2ctx() context.Context { return context.Background() }
