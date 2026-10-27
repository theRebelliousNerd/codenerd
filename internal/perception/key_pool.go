package perception

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

type keyHealth struct {
	active   int
	disabled bool
	cooldown time.Time
}
type providerKeyPool struct {
	mu     sync.Mutex
	health []keyHealth
	next   int
}

var keyPoolRegistry = struct {
	sync.Mutex
	pools map[string]*providerKeyPool
}{pools: make(map[string]*providerKeyPool)}

func sharedKeyPool(provider Provider, keys []config.ResolvedProviderAPIKey) (*providerKeyPool, error) {
	// Registry keys are digests; secrets never appear in logs or errors.
	data, _ := json.Marshal(keys)
	sum := sha256.Sum256(append([]byte(provider), data...))
	identity := hex.EncodeToString(sum[:])
	keyPoolRegistry.Lock()
	defer keyPoolRegistry.Unlock()
	if pool := keyPoolRegistry.pools[identity]; pool != nil {
		return pool, nil
	}
	if len(keyPoolRegistry.pools) >= 128 {
		return nil, fmt.Errorf("too many distinct credential pool configurations in this process; restart to reclaim retired pools")
	}
	pool := &providerKeyPool{health: make([]keyHealth, len(keys))}
	keyPoolRegistry.pools[identity] = pool
	return pool, nil
}

// selectKey uses least-in-flight load balancing with rotating ties. Health is
// shared across every model/slot built from this provider's credential set.
func (p *providerKeyPool) selectKey(ctx context.Context, excluded map[int]bool, pinned int) (int, error) {
	for {
		if err := ctx.Err(); err != nil {
			return -1, err
		}
		p.mu.Lock()
		chosen := -1
		var earliest time.Time
		remaining := false
		now := time.Now()
		for step := 0; step < len(p.health); step++ {
			i := (p.next + step) % len(p.health)
			if pinned >= 0 && i != pinned {
				continue
			}
			h := p.health[i]
			if excluded[i] || h.disabled {
				continue
			}
			remaining = true
			if now.Before(h.cooldown) {
				if earliest.IsZero() || h.cooldown.Before(earliest) {
					earliest = h.cooldown
				}
				continue
			}
			if chosen < 0 || h.active < p.health[chosen].active {
				chosen = i
			}
		}
		if chosen >= 0 {
			p.health[chosen].active++
			p.next = (chosen + 1) % len(p.health)
			p.mu.Unlock()
			return chosen, nil
		}
		p.mu.Unlock()
		if !remaining {
			return -1, fmt.Errorf("no healthy untried credentials remain in the provider pool")
		}
		if err := waitAnthropic(ctx, time.Until(earliest)); err != nil {
			return -1, err
		}
	}
}

func (p *providerKeyPool) release(i int, status int, after time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health[i].active--
	switch {
	case status == 401 || status == 403:
		p.health[i].disabled = true
	case status == 429 || isTransientHTTPStatus(status):
		if after <= 0 {
			if status == 429 {
				after = time.Second
			} else {
				after = 250 * time.Millisecond
			}
		}
		until := time.Now().Add(after)
		if until.After(p.health[i].cooldown) {
			p.health[i].cooldown = until
		}
	}
}

type pooledCallError struct {
	provider Provider
	alias    string
	status   int
}

func (e *pooledCallError) Error() string {
	return fmt.Sprintf("%s credential %q: provider request failed with status %d", e.provider, e.alias, e.status)
}

type providerKeyRouter struct {
	provider        Provider
	timeout         time.Duration
	aliases         []string
	clients         []LLMClient
	pool            *providerKeyPool
	mu              sync.RWMutex
	configurationMu sync.RWMutex
	current         int
	pinned          int // server-side cached content cannot move between accounts
}

func (r *providerKeyRouter) Current() LLMClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.clients[r.current]
}
func (r *providerKeyRouter) Clients() []LLMClient { return append([]LLMClient(nil), r.clients...) }
func (r *providerKeyRouter) Configure(fn func(types.LLMClient)) {
	r.configurationMu.Lock()
	defer r.configurationMu.Unlock()
	for _, client := range r.clients {
		fn(client)
	}
}

func (r *providerKeyRouter) ConfigureCurrent(fn func(types.LLMClient)) {
	r.configurationMu.Lock()
	defer r.configurationMu.Unlock()
	fn(r.Current())
}

func (r *providerKeyRouter) Pin(enable bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if enable {
		r.pinned = r.current
	} else {
		r.pinned = -1
	}
}
func (r *providerKeyRouter) selected(i int) { r.mu.Lock(); defer r.mu.Unlock(); r.current = i }
func (r *providerKeyRouter) pin() int       { r.mu.RLock(); defer r.mu.RUnlock(); return r.pinned }

func (r *providerKeyRouter) attempt(ctx context.Context, excluded map[int]bool) (context.Context, context.CancelFunc, *keyPoolAttempt, int, error) {
	i, err := r.pool.selectKey(ctx, excluded, r.pin())
	if err != nil {
		return ctx, nil, nil, -1, err
	}
	child, cancel := context.WithCancel(ctx)
	attempt := &keyPoolAttempt{cancel: cancel}
	child = context.WithValue(child, keyPoolAttemptContext{}, attempt)
	child = usage.WithCredentialAlias(child, string(r.provider)+"/"+r.aliases[i])
	return child, cancel, attempt, i, nil
}

func (r *providerKeyRouter) finish(ctx context.Context, i int, attempt *keyPoolAttempt, err error) (error, bool) {
	status, after, network := attempt.result()
	if status == 0 && network {
		status = 503
	}
	if status == 0 && err != nil && ctx.Err() == nil {
		status = providerFailureStatus(err)
	}
	r.pool.release(i, status, after)
	r.selected(i)
	if ctx.Err() != nil {
		return ctx.Err(), false
	}
	if err == nil && status == 0 {
		return nil, false
	}
	if status != 0 {
		safe := &pooledCallError{provider: r.provider, alias: r.aliases[i], status: status}
		return safe, r.pin() < 0 && (status == 401 || status == 403 || status == 429 || isTransientHTTPStatus(status))
	}
	return err, false
}

func (r *providerKeyRouter) Do(ctx context.Context, call func(context.Context, types.LLMClient) error) error {
	r.configurationMu.RLock()
	defer r.configurationMu.RUnlock()
	ctx, stop := context.WithTimeout(ctx, r.timeout)
	defer stop()
	excluded := map[int]bool{}
	var lastErr error
	for len(excluded) < len(r.clients) {
		child, cancel, attempt, i, err := r.attempt(ctx, excluded)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if lastErr != nil {
				return lastErr
			}
			return err
		}
		excluded[i] = true

		err = func() (result error) {
			defer cancel()
			defer func() {
				if recover() != nil {
					result = fmt.Errorf("provider credential %q panicked during completion", r.aliases[i])
				}
			}()
			return call(child, r.clients[i])
		}()

		final, retry := r.finish(ctx, i, attempt, err)
		if !retry {
			return final
		}
		lastErr = final
	}
	return lastErr
}

// Stream retries only before either content or thoughts have been emitted.
// One child context and active lease span the entire selected stream.
func (r *providerKeyRouter) Stream(ctx context.Context, start func(context.Context, types.LLMClient) (<-chan string, <-chan string, <-chan error)) (<-chan string, <-chan string, <-chan error) {
	content, thoughts, errs := make(chan string, 64), make(chan string, 64), make(chan error, 1)
	go func() {
		r.configurationMu.RLock()
		defer r.configurationMu.RUnlock()
		ctx, stop := context.WithTimeout(ctx, r.timeout)
		defer stop()
		defer close(content)
		defer close(thoughts)
		defer close(errs)
		excluded := map[int]bool{}
		var lastErr error
		for len(excluded) < len(r.clients) {
			child, cancel, attempt, i, err := r.attempt(ctx, excluded)
			if err != nil {
				if ctx.Err() != nil {
					err = ctx.Err()
				} else if lastErr != nil {
					err = lastErr
				}
				errs <- err
				return
			}
			excluded[i] = true
			in, thinking, failures := func() (content <-chan string, thoughts <-chan string, errs <-chan error) {
				defer func() {
					if recover() != nil {
						failure := make(chan error, 1)
						failure <- fmt.Errorf("provider credential %q panicked during stream initialization", r.aliases[i])
						close(failure)
						content = nil
						thoughts = nil
						errs = failure
					}
				}()
				return start(child, r.clients[i])
			}()
			emitted := false
			var callErr error
			canceled := false
			for in != nil || thinking != nil || failures != nil {
				select {
				case value, ok := <-in:
					if !ok {
						in = nil
						continue
					}
					if value != "" {
						emitted = true
					}
					select {
					case content <- value:
					case <-ctx.Done():
						canceled = true
					}
				case value, ok := <-thinking:
					if !ok {
						thinking = nil
						continue
					}
					if value != "" {
						emitted = true
					}
					select {
					case thoughts <- value:
					case <-ctx.Done():
						canceled = true
					}
				case failure, ok := <-failures:
					if !ok {
						failures = nil
						continue
					}
					if failure != nil && callErr == nil {
						callErr = failure
					}
				case <-ctx.Done():
					canceled = true
				}
				if canceled {
					callErr = ctx.Err()
					break
				}
			}
			cancel()
			final, retry := r.finish(ctx, i, attempt, callErr)
			if retry && !emitted {
				lastErr = final
				continue
			}
			if final != nil {
				errs <- final
			}
			return
		}
		if lastErr != nil {
			errs <- lastErr
		}
	}()
	return content, thoughts, errs
}
