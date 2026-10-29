package broker

import "errors"

var (
	errNilRequest = errors.New("broker: nil request")
	// ErrNoCounter is returned when a broker is constructed without a counter.
	// It is an error rather than a silent pass-through because a broker that
	// cannot count is a broker that cannot enforce anything, and shipping one
	// silently would recreate the exact condition this package exists to end.
	ErrNoCounter = errors.New("broker: no counter configured")
	// ErrNoUnderlying is returned when a broker is constructed without a client.
	ErrNoUnderlying = errors.New("broker: no underlying LLM client")
)
