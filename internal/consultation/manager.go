package consultation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Manager runs consultations and coalesces identical in-flight ones.
type Manager struct {
	spawner Spawner

	mu       sync.Mutex
	inflight map[string]*flight
	seq      atomic.Uint64
}

// flight is one in-progress specialist computation shared by its consumers.
type flight struct {
	waiters  int
	cancel   context.CancelFunc
	done     chan struct{}
	artifact *Artifact
	err      error
}

// New returns a Manager over spawner. A nil spawner is allowed and fails every
// consultation visibly with ErrNoSpawner.
func New(spawner Spawner) *Manager {
	return &Manager{spawner: spawner, inflight: make(map[string]*flight)}
}

// Consult asks one specialist. Identical concurrent requests (same content
// key) share one computation; each caller waits under its own ctx, and a
// caller that gives up detaches without cancelling the others. The
// computation is cancelled only when no consumer is left waiting.
func (m *Manager) Consult(ctx context.Context, req Request) (*Response, error) {
	if m == nil || m.spawner == nil {
		return nil, fmt.Errorf("consultation with %s failed: %w", req.Specialist, ErrNoSpawner)
	}
	req.Specialist = strings.TrimSpace(req.Specialist)
	if req.Specialist == "" {
		return nil, errors.New("consultation request names no specialist")
	}
	if req.Requester == "" {
		req.Requester = DefaultRequester
	}
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("consult-%s-%d", req.Specialist, m.seq.Add(1))
	}

	key := contentKey(req)
	start := time.Now()
	reuse := ReuseCoalesced

	m.mu.Lock()
	f, ok := m.inflight[key]
	if ok {
		f.waiters++
	} else {
		reuse = ReuseComputed
		// The producer outlives any single consumer's deadline: it keeps the
		// first caller's values but not its cancellation, and stops when the
		// last consumer leaves.
		pctx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		f = &flight{waiters: 1, cancel: cancel, done: make(chan struct{})}
		m.inflight[key] = f
		go m.produce(pctx, key, f, req)
	}
	m.mu.Unlock()

	select {
	case <-f.done:
		m.leave(key, f)
		if f.err != nil {
			return nil, fmt.Errorf("consultation with %s failed: %w", req.Specialist, f.err)
		}
		return f.artifact.receipt(req, reuse, time.Since(start)), nil
	case <-ctx.Done():
		m.leave(key, f)
		return nil, fmt.Errorf("consultation with %s abandoned: %w", req.Specialist, ctx.Err())
	}
}

func (m *Manager) produce(ctx context.Context, key string, f *flight, req Request) {
	start := time.Now()
	result, err := m.spawner.SpawnConsultation(ctx, req.Specialist, buildPrompt(req))
	duration := time.Since(start)

	m.mu.Lock()
	if err != nil {
		f.err = err
	} else {
		f.artifact = parseArtifact(key, req.Specialist, result, duration)
	}
	// Nothing is retained after publication: a later identical request is a
	// new computation, because nothing proves the old answer still holds.
	if m.inflight[key] == f {
		delete(m.inflight, key)
	}
	m.mu.Unlock()
	close(f.done)
	f.cancel()
}

// leave detaches one consumer. The last one out cancels an unfinished
// producer and withdraws the flight so no new consumer joins a dying one.
func (m *Manager) leave(key string, f *flight) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f.waiters--
	if f.waiters > 0 {
		return
	}
	select {
	case <-f.done:
	default:
		if m.inflight[key] == f {
			delete(m.inflight, key)
		}
		f.cancel()
	}
}

// RequestBatchConsultation consults every named specialist in parallel and
// returns one Entry per distinct specialist, in first-occurrence order.
// Duplicate and blank names are folded. A failed entry never erases another
// entry's success. It satisfies campaign.ConsultationProvider.
func (m *Manager) RequestBatchConsultation(ctx context.Context, req BatchRequest) Batch {
	specialists := distinctSpecialists(req.Specialists)
	if len(specialists) == 0 {
		return Batch{RequestErr: errors.New("batch consultation names no specialist")}
	}

	question := strings.TrimSpace(req.Question)
	if topic := strings.TrimSpace(req.Topic); topic != "" {
		if question == "" {
			question = topic
		} else {
			question = "[" + topic + "] " + question
		}
	}

	entries := make([]Entry, len(specialists))
	var wg sync.WaitGroup
	for i, spec := range specialists {
		wg.Go(func() {
			resp, err := m.Consult(ctx, Request{
				Requester:        req.Requester,
				Specialist:       spec,
				Question:         question,
				Context:          req.Context,
				ResponseContract: req.ResponseContract,
			})
			entries[i] = entryFor(spec, resp, err)
		})
	}
	wg.Wait()
	return Batch{Entries: entries}
}

func entryFor(spec string, resp *Response, err error) Entry {
	switch {
	case err == nil:
		return Entry{Specialist: spec, Status: EntrySucceeded, Response: resp}
	case errors.Is(err, ErrNoSpawner):
		return Entry{Specialist: spec, Status: EntryFailed, Err: err}
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		return Entry{Specialist: spec, Status: EntryCancelled, Err: err, Retryable: true}
	default:
		return Entry{Specialist: spec, Status: EntryFailed, Err: err, Retryable: true}
	}
}

func distinctSpecialists(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}
