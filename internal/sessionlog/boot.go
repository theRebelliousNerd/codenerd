package sessionlog

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// BootOptions configures StartSession.
type BootOptions struct {
	Workspace string
	// Session is the session being booted. It is never pruned.
	Session string
	// TTL is the retention: session folders idle longer than this are deleted
	// at boot. Zero keeps everything.
	TTL time.Duration
	// Enabled overrides the exchange switch (default logging.IsDebugMode).
	Enabled func() bool
	// Now overrides the clock, for tests.
	Now func() time.Time
	// Heartbeat overrides the lease refresh period (default 1 minute).
	Heartbeat time.Duration
}

// Handle is a booted session log. Close releases the session's lease.
type Handle struct {
	Store    *Store
	Migrated MigrateReport
	Pruned   PruneReport

	stop chan struct{}
	wg   sync.WaitGroup
	once sync.Once
}

// StartSession is the startup sequence: move flat session files into their
// folders, prune expired sessions, claim this session with a lease, and
// install the store the broker and executor write through.
//
// A failure to migrate or prune is logged and does not stop the boot: the
// session log is never a reason the agent cannot start.
func StartSession(o BootOptions) *Handle {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	log := logging.Get(logging.CategorySession)
	h := &Handle{stop: make(chan struct{})}

	if rep, err := MigrateFlat(o.Workspace); err != nil {
		log.Warn("session log: migrating flat session files: %v", err)
	} else {
		h.Migrated = rep
		if rep.Moved > 0 {
			log.Info("session log: moved %d flat session file(s) into their folders", rep.Moved)
		}
		for _, c := range rep.Conflicts {
			log.Warn("session log: %s left in place: its folder already holds a session.json", c)
		}
	}

	store := Current()
	if store == nil || store.workspace != o.Workspace {
		store = NewStore(Options{Workspace: o.Workspace, Session: o.Session, Enabled: o.Enabled})
		Install(store)
	} else {
		store.SetSession(o.Session)
	}
	h.Store = store

	keep := map[string]bool{o.Session: true}
	for _, id := range store.Touched() {
		keep[id] = true
	}
	rep, err := Prune(o.Workspace, o.TTL, keep, now())
	if err != nil {
		log.Warn("session log: pruning expired sessions: %v", err)
	}
	h.Pruned = rep
	switch {
	case o.TTL <= 0:
		log.Info("session log: retention is off; no session is pruned")
	case len(rep.Removed) > 0 || len(rep.Failed) > 0:
		log.Info("session log: pruned %d session(s), %d bytes, older than %s; %d kept (%d running elsewhere); %d failed",
			len(rep.Removed), rep.Bytes, o.TTL, rep.Kept, len(rep.Running), len(rep.Failed))
	default:
		log.Debug("session log: nothing older than %s to prune (%d kept)", o.TTL, rep.Kept)
	}

	if o.Session != "" {
		h.refreshLeases(now)
	}

	period := o.Heartbeat
	if period <= 0 {
		period = time.Minute
	}
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		t := time.NewTicker(period)
		defer t.Stop()
		for {
			select {
			case <-h.stop:
				return
			case <-t.C:
				h.refreshLeases(now)
			}
		}
	}()
	return h
}

func (h *Handle) refreshLeases(now func() time.Time) {
	ids := h.Store.Touched()
	if s := h.Store.Session(); s != "" {
		ids = append(ids, s)
	}
	for _, id := range ids {
		dir, err := SessionDir(h.Store.workspace, id)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			h.Store.fail(err)
			continue
		}
		if err := writeLease(dir, now()); err != nil {
			h.Store.fail(err)
		}
	}
}

// Close stops the heartbeat and removes this process's leases.
func (h *Handle) Close() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		close(h.stop)
		h.wg.Wait()
		ids := h.Store.Touched()
		if s := h.Store.Session(); s != "" {
			ids = append(ids, s)
		}
		for _, id := range ids {
			if dir, err := SessionDir(h.Store.workspace, id); err == nil {
				_ = os.Remove(filepath.Join(dir, LeaseFile))
			}
		}
	})
}
