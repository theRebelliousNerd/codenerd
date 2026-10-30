package core

import (
	"context"
	"fmt"

	"codenerd/internal/logging"
	"codenerd/internal/transparency"
)

// SetWorldRefresher attaches the session's incremental scanner without a core
// -> world import cycle. The same scanner serves boot and explicit scans.
func (v *VirtualStore) SetWorldRefresher(refresh func(context.Context) error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.worldRefresher = refresh
}

func (v *VirtualStore) refreshWorld(ctx context.Context) error {
	v.mu.RLock()
	refresh := v.worldRefresher
	v.mu.RUnlock()
	if refresh == nil {
		return fmt.Errorf("world scanner is not attached")
	}
	return refresh(ctx)
}

// refreshWorldAfterFacts observes writes; it makes no permission or scheduling
// decision. Callers already passed the executive's action/tool boundary.
func (v *VirtualStore) refreshWorldAfterFacts(ctx context.Context, facts []Fact) error {
	for _, fact := range facts {
		if fact.Predicate != "modified" && fact.Predicate != "file_deleted" {
			continue
		}
		v.mu.RLock()
		attached := v.worldRefresher != nil
		bus := v.glassBoxBus
		v.mu.RUnlock()
		// Bare VirtualStores (without a Cortex) have no world scanner to maintain.
		if !attached {
			return nil
		}
		if err := v.refreshWorld(ctx); err != nil {
			logging.Get(logging.CategoryWorld).Warn("Post-write world refresh failed: %v", err)
			if bus != nil {
				bus.EmitImmediate(transparency.GlassBoxEvent{Category: transparency.CategoryRouting,
					Summary: "World refresh failed", Details: err.Error(), Source: "world"})
			}
			return err
		}
		return nil // one refresh per mutation batch, including multi-file edits
	}
	return nil
}
