package perception

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// affinityIndex is the credential a conversation prefers: a stable hash of
// its identity over the pool size. Prompt caches are scoped to the account or
// workspace a key belongs to, so a conversation whose consecutive calls hop
// between keys loses its cached prefix on every hop. Measured 2026-10-29 on a
// three-key Anthropic pool under least-in-flight-with-rotating-ties: 126 of
// 126 follow-up calls in one scope changed key, and they read 17% of their
// input from cache, against ~80% on a single-key provider in the same runs.
func affinityIndex(affinity string, n int) int {
	if affinity == "" || n <= 0 {
		return -1
	}
	sum := sha256.Sum256([]byte(affinity))
	v := uint64(0)
	for _, b := range sum[:8] {
		v = v<<8 | uint64(b)
	}
	return int(v % uint64(n))
}

// selectKey picks a credential. A call that carries a conversation identity
// (affinity) goes to that conversation's preferred key whenever the key is
// usable -- healthy, not cooling down, not already tried for this call -- so
// the conversation keeps its prompt cache; different conversations hash to
// different keys, which is what spreads the load. A call with no identity, or
// whose preferred key is unusable, falls back to least-in-flight with rotating
// ties. A pinned key wins over both. Health is shared across every model and
// slot built from this provider's credential set.
func (p *providerKeyPool) selectKey(ctx context.Context, excluded map[int]bool, pinned int, affinity string) (int, error) {
	for {
		if err := ctx.Err(); err != nil {
			return -1, err
		}
		p.mu.Lock()
		if pinned < 0 {
			if pref := affinityIndex(affinity, len(p.health)); pref >= 0 {
				h := p.health[pref]
				if !excluded[pref] && !h.disabled && !time.Now().Before(h.cooldown) {
					p.health[pref].active++
					p.mu.Unlock()
					return pref, nil
				}
			}
		}
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

// release settles one attempt's lease and what its failure says about the
// credential. Only a 401 says the credential itself is unusable; a 403 is a
// denial of this request -- a guardrail or moderation block, a model,
// resource or region the key may not use -- and says nothing about the
// credential's next, differently shaped request, so it leaves health alone
// (Docs/audits/credential-pool-failure-scope-and-recovery.md).
func (p *providerKeyPool) release(i int, status int, after time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.health[i].active--
	switch {
	case status == 401:
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

// pooledCallError is a pooled attempt's failure as the caller sees it: the
// provider, the credential's alias, the status and the adapter's own
// diagnostic (the provider's error type, message and request id), never the
// secret. A pool that reduced a 400 to "status 400" left an operator unable to
// tell which field the provider rejected. The adapter's error stays reachable
// through errors.Is/As, so a typed failure (a refusal, a truncation, the
// provider failure class) survives the pool. User-facing text is
// SafeProviderFailureMessage, never Error().
type pooledCallError struct {
	provider Provider
	alias    string
	status   int
	cause    error
}

func (e *pooledCallError) Error() string {
	msg := fmt.Sprintf("%s credential %q: provider request failed with status %d", e.provider, e.alias, e.status)
	if e.status == 403 {
		msg += " (the request was denied; the credential stays in the pool and no other credential is tried)"
	}
	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}
	return msg
}

func (e *pooledCallError) Unwrap() error { return e.cause }

// rotates reports whether a failure may move the same request to another
// credential: the credential is invalid (401), rate-limited (429) or the
// provider was transiently unavailable. A 403 never rotates: another key
// would only be asked to evade the denial.
func rotates(status int) bool {
	return status == 401 || status == 429 || isTransientHTTPStatus(status)
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
	// The conversation's identity is the scope the broker meters it under:
	// one per session and per subagent, the unit that shares a cached prefix.
	i, err := r.pool.selectKey(ctx, excluded, r.pin(), usage.SessionIDFromContext(ctx))
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
		// The transport cancels a failed attempt's own context to stop the
		// adapter's retries; that cancellation is not the caller's and must
		// not read as one.
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			safe.cause = err
		}
		return safe, r.pin() < 0 && rotates(status)
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
// One child context and active lease span the entire selected stream. The
// selected credential's terminal result is published unchanged except for the
// error the lease bookkeeping settles on; a pool that runs out of credentials
// publishes the last failure, and one with nothing to try ends incomplete.
func (r *providerKeyRouter) Stream(ctx context.Context, start func(context.Context, types.LLMClient) (<-chan string, <-chan string, <-chan types.StreamResult)) (<-chan string, <-chan string, <-chan types.StreamResult) {
	w, content, thoughts, done := types.NewThoughtStream()
	go func() {
		r.configurationMu.RLock()
		defer r.configurationMu.RUnlock()
		ctx, stop := context.WithTimeout(ctx, r.timeout)
		defer stop()
		defer w.Close()
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
				w.Fail(err)
				return
			}
			excluded[i] = true
			in, thinking, finished := func() (content <-chan string, thoughts <-chan string, done <-chan types.StreamResult) {
				defer func() {
					if recover() != nil {
						content, thoughts, done = types.FailedThoughtStream(
							fmt.Errorf("provider credential %q panicked during stream initialization", r.aliases[i]))
					}
				}()
				return start(child, r.clients[i])
			}()
			res, emitted := types.RelayStream(ctx, w, in, thinking, finished)
			cancel()
			final, retry := r.finish(ctx, i, attempt, res.Err)
			if retry && !emitted {
				lastErr = final
				continue
			}
			res.Err = final
			w.Finish(res)
			return
		}
		if lastErr != nil {
			w.Fail(lastErr)
		}
	}()
	return content, thoughts, done
}
