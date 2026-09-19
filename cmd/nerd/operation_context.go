package main

import "context"

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
