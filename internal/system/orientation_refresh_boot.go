package system

import (
	"context"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/orient"
	"codenerd/internal/types"
)

// refreshOrientationInBackground brings the repository orientation current
// without holding up the boot. The kernels booted on the projection the last
// refresh saved, at most one commit behind; this refresh re-measures what
// changed and applies the delta to the running cortex in one transaction, the
// same path an incremental scan uses (world.ApplyIncrementalResult). After a
// new commit the refresh re-evaluates the whole orientation program, which
// every kernel boot used to wait on (~55 s measured 2026-10-27).
func refreshOrientationInBackground(workspace string, kernel types.Kernel) {
	log := logging.Get(logging.CategoryBoot)
	started := time.Now()
	res, err := orient.Refresh(context.Background(), workspace, nil)
	if err != nil {
		log.Warn("orientation: background refresh failed, kernels keep the saved projection: %v", err)
		return
	}
	if res == nil || !res.Refreshed {
		log.Debug("orientation: current (%s)", time.Since(started).Round(time.Millisecond))
		return
	}
	transactor, ok := types.TransactorOf(kernel)
	if !ok {
		log.Warn("orientation: refreshed, but kernel %T cannot apply the delta; it applies at the next scan", kernel)
		return
	}
	if err := res.Apply(transactor); err != nil {
		log.Warn("orientation: refreshed, applying the delta failed: %v", err)
		return
	}
	log.Info("orientation: refreshed in the background in %s (%d rows added, %d removed)",
		time.Since(started).Round(time.Millisecond), len(res.Added), len(res.Removed))
}
