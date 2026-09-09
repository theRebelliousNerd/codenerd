package broker

import "time"

// timeAfter is a shared deadline for tests that must prove termination.
func timeAfter() <-chan time.Time { return time.After(5 * time.Second) }
