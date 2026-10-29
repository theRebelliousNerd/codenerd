package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// assertFactsWithFallback asserts a batch of facts and, when the batch is
// rejected, retries each fact on its own so one bad fact does not cost the rest.
//
// The fallback itself is deliberate and stays: AssertBatch is all-or-nothing, so
// a single malformed activation weight would otherwise drop twenty good ones.
// What did not stay is the silence. The retry loop used to be
//
//	for _, f := range facts { cp.kernel.Assert(f) }
//
// with Assert's error dropped as a bare statement, at nine sites covering the
// whole campaign context-paging path. A paging pass that landed nothing looked
// identical to one that landed everything: the phase then ran with the wrong
// activation profile — wrong files boosted, browser schemas never suppressed,
// compressed atoms never decayed — and nothing anywhere said why.
//
// These facts are advisory. They are activation weights and phase context atoms
// that steer which facts the JIT prompt sees; the campaign still executes
// without them, so a partial failure is tolerated rather than escalated. It is
// logged at Warn with the count and the first cause so a badly-behaved phase can
// be explained after the fact.
func assertFactsWithFallback(kernel core.Kernel, facts []core.Fact, what string) {
	if kernel == nil || len(facts) == 0 {
		return
	}

	batchErr := kernel.AssertBatch(facts)
	if batchErr == nil {
		return
	}

	failed := 0
	var firstErr error
	for _, f := range facts {
		if err := kernel.Assert(f); err != nil {
			failed++
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if failed == 0 {
		logging.CampaignDebug("%s: batch assert failed (%v), all %d facts landed individually",
			what, batchErr, len(facts))
		return
	}

	logging.CampaignWarn("%s: %d of %d facts never reached the kernel (batch: %v; first fact: %v)",
		what, failed, len(facts), batchErr, firstErr)
}
