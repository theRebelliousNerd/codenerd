package system

import (
	"context"
	"fmt"

	"codenerd/internal/world"
)

// attachWorldRefresher reuses the boot scanner for writes and explicit refresh
// actions. Scanner persists the fast world cache; link projection retires the
// previous generation instead of accumulating background copies.
func attachWorldRefresher(bctx *bootContext) {
	if bctx.virtualStore == nil {
		return
	}
	scanner, kernel, database := bctx.scanner, bctx.kernel, bctx.localDB
	workspace, virtualStore := bctx.workspace, bctx.virtualStore
	virtualStore.SetWorldRefresher(func(ctx context.Context) error {
		res, err := scanner.Refresh(ctx, workspace, database, kernel,
			world.IncrementalOptions{SkipWhenUnchanged: true})
		if err != nil {
			return err
		}
		if res == nil || res.Unchanged {
			return nil
		}
		if err := virtualStore.PersistFactsToKnowledge(res.NewFacts, "fact", 5); err != nil {
			return fmt.Errorf("persist world facts: %w", err)
		}
		if err := virtualStore.ReplaceLinkFacts(res.RetractFacts, res.NewFacts, "scan"); err != nil {
			return fmt.Errorf("project world links: %w", err)
		}
		return nil
	})
}
