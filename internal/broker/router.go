package broker

import (
	"codenerd/internal/types"
	"context"
)

// ClientRouter selects an already-metered client for each logical attempt.
// Routing happens before the selected client's admission/counting. Wrapping a
// router must not add a second charge or erase optional provider interfaces.
type ClientRouter interface {
	Do(context.Context, func(context.Context, types.LLMClient) error) error
	Stream(context.Context, func(context.Context, types.LLMClient) (<-chan string, <-chan string, <-chan error)) (<-chan string, <-chan string, <-chan error)
	Current() types.LLMClient
	Clients() []types.LLMClient
	Configure(func(types.LLMClient))
	ConfigureCurrent(func(types.LLMClient))
	Pin(bool)
}

// WrapRouter preserves exactly the reference client's optional method set.
// Every selectable client must have that same provider/capability contract and
// already carry broker metering.
func WrapRouter(reference types.LLMClient, router ClientRouter) (types.LLMClient, error) {
	if router == nil {
		return nil, ErrNoUnderlying
	}
	for _, client := range router.Clients() {
		if !IsBrokered(client) {
			return nil, ErrNoCounter
		}
	}
	return Wrap(reference, Config{Router: router})
}

func routeCall[T any](ctx context.Context, router ClientRouter, call func(context.Context, types.LLMClient) (T, error)) (T, error) {
	var result T
	err := router.Do(ctx, func(ctx context.Context, client types.LLMClient) error {
		value, err := call(ctx, client)
		if err == nil {
			result = value
		}
		return err
	})
	return result, err
}

func (c *core) delegate() types.LLMClient {
	if c.cfg.Router != nil {
		return c.cfg.Router.Current()
	}
	return c.underlying
}

func (c *core) eachClient(fn func(types.LLMClient)) {
	if c.cfg.Router != nil {
		c.cfg.Router.Configure(fn)
		return
	}
	fn(c.underlying)
}
