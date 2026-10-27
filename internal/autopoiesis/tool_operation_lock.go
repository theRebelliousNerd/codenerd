package autopoiesis

import (
	"context"
	"fmt"
	"sync"
)

// toolOperationLocks coordinates publication and execution for each generated
// tool independently. Waiters retain the entry until they acquire or cancel,
// so a lock cannot be replaced while another operation still refers to it.
type toolOperationLocks struct {
	mu    sync.Mutex
	locks map[string]*toolOperationLock
}

type toolOperationLock struct {
	permit       chan struct{}
	refs         int
	waiters      int
	stateChanged chan struct{}
}

func (locks *toolOperationLocks) acquire(ctx context.Context, name string) (func(), error) {
	if ctx == nil {
		return nil, fmt.Errorf("tool operation context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	locks.mu.Lock()
	if locks.locks == nil {
		locks.locks = make(map[string]*toolOperationLock)
	}
	state := locks.locks[name]
	if state == nil {
		state = &toolOperationLock{permit: make(chan struct{}, 1), stateChanged: make(chan struct{}, 1)}
		state.permit <- struct{}{}
		locks.locks[name] = state
	}
	state.refs++
	locks.signalStateChange(state)
	locks.mu.Unlock()

	select {
	case <-state.permit:
		if err := ctx.Err(); err != nil {
			state.permit <- struct{}{}
			locks.releaseRef(name, state)
			return nil, err
		}
		return locks.releaseFunc(name, state), nil
	default:
	}

	locks.mu.Lock()
	state.waiters++
	locks.signalStateChange(state)
	locks.mu.Unlock()

	select {
	case <-ctx.Done():
		locks.mu.Lock()
		state.waiters--
		locks.signalStateChange(state)
		locks.mu.Unlock()
		locks.releaseRef(name, state)
		return nil, ctx.Err()
	case <-state.permit:
		locks.mu.Lock()
		state.waiters--
		locks.signalStateChange(state)
		locks.mu.Unlock()
		if err := ctx.Err(); err != nil {
			state.permit <- struct{}{}
			locks.releaseRef(name, state)
			return nil, err
		}
		return locks.releaseFunc(name, state), nil
	}
}

func (locks *toolOperationLocks) releaseFunc(name string, state *toolOperationLock) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			state.permit <- struct{}{}
			locks.releaseRef(name, state)
		})
	}
}

func (locks *toolOperationLocks) releaseRef(name string, state *toolOperationLock) {
	locks.mu.Lock()
	defer locks.mu.Unlock()
	state.refs--
	locks.signalStateChange(state)
	if state.refs == 0 && locks.locks[name] == state {
		delete(locks.locks, name)
	}
}

func (locks *toolOperationLocks) signalStateChange(state *toolOperationLock) {
	select {
	case state.stateChanged <- struct{}{}:
	default:
	}
}
