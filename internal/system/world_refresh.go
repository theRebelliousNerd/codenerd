package system

import (
	"context"
	"errors"
	"fmt"

	"codenerd/internal/core"
	"codenerd/internal/world"
)

// attachWorldRefresher reuses the boot scanner for writes and explicit refresh
// actions. Scanner persists the fast world cache; link projection retires the
// previous generation instead of accumulating background copies.
func attachWorldRefresher(bctx *bootContext) error {
	if bctx.virtualStore == nil {
		return nil
	}
	scanner, kernel, database := bctx.scanner, bctx.kernel, bctx.localDB
	workspace, virtualStore := bctx.workspace, bctx.virtualStore
	if err := scanner.SetRefreshPublisher(func(ctx context.Context, res *world.IncrementalResult) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return publishWorldRefresh(virtualStore, res)
	}); err != nil {
		return err
	}
	virtualStore.SetWorldRefresher(func(ctx context.Context) error {
		_, err := scanner.Refresh(ctx, workspace, database, kernel,
			world.IncrementalOptions{SkipWhenUnchanged: true})
		return err
	})
	return nil
}

// Both writes are idempotent. A cold-mirror failure must not suppress graph
// retirement, and either failure keeps the scanner's receipt pending for replay.
func publishWorldRefresh(vs *core.VirtualStore, res *world.IncrementalResult) error {
	var errs []error
	if err := vs.PersistFactsToKnowledge(res.NewFacts, "fact", 5); err != nil {
		errs = append(errs, fmt.Errorf("persist world facts: %w", err))
	}
	if err := vs.ReplaceLinkFacts(res.RetractFacts, res.NewFacts, "scan"); err != nil {
		errs = append(errs, fmt.Errorf("project world links: %w", err))
	}
	return errors.Join(errs...)
}
