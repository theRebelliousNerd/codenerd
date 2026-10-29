package system

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/retrieval"
)

// initSemanticSearcher builds the process's one Tier 4 searcher and starts
// warming it in the background.
//
// Every retrieval pass in the process ranks from it: the session executor's
// task passes, delegated tasks, and the chat's issue seed. Its vectors live in
// the workspace's FileVectorStore, keyed by content digest and embedding
// model, so a new process starts warm and the warm-up embeds only files that
// are new or edited since the last one. Turns never wait on the warm-up; until
// it lands, a pass ranks what is held and reports the rest as pending.
//
// No embedding engine means no searcher, and passes keep the definition-scan
// fallback. A store that cannot open leaves the searcher memory-only.
func initSemanticSearcher(bctx *bootContext) {
	if bctx == nil || bctx.embeddingEngine == nil {
		return
	}
	store, err := retrieval.OpenFileVectorStore(bctx.ctx, retrieval.FileVectorStorePath(bctx.workspace))
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("semantic tier: vectors will not persist across sessions: %v", err)
		store = nil
	}
	searcher := retrieval.NewEmbeddingSemanticSearcher(bctx.embeddingEngine, bctx.workspace, store)
	bctx.semanticSearcher = searcher

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	bctx.semanticWarmCancel = cancel
	bctx.semanticWarmDone = done
	go func() {
		defer close(done)
		warmSemanticCorpus(ctx, searcher)
	}()
}

// warmSemanticCorpus runs one warm-up pass until it completes or the Cortex
// closes. There is no wall clock: progress is persisted chunk by chunk, so a
// pass cut short by Close resumes where it stopped in the next process.
func warmSemanticCorpus(ctx context.Context, s *retrieval.EmbeddingSemanticSearcher) {
	start := time.Now()
	report, err := s.Warm(ctx)
	if err != nil {
		if ctx.Err() != nil {
			logging.Context("semantic corpus warm-up stopped at shutdown: %d of %d files hold vectors (%d embedded, %d from the store)", report.Covered, report.Eligible, report.Embedded, report.Stored)
			return
		}
		logging.Get(logging.CategoryContext).Warn("semantic corpus warm-up incomplete after %s: %d of %d files hold vectors: %v", time.Since(start).Round(time.Millisecond), report.Covered, report.Eligible, err)
		return
	}
	logging.Context("semantic corpus warmed in %s: %d of %d files (%d embedded, %d from the store, %d stale rows pruned)", time.Since(start).Round(time.Millisecond), report.Covered, report.Eligible, report.Embedded, report.Stored, report.Pruned)
}

// closeSemanticSearcher stops the warm-up, waits for it, and closes the
// searcher's store.
func (c *Cortex) closeSemanticSearcher() error {
	if c.semanticWarmCancel != nil {
		c.semanticWarmCancel()
		c.semanticWarmCancel = nil
	}
	if c.semanticWarmDone != nil {
		select {
		case <-c.semanticWarmDone:
		case <-time.After(closeStepTimeout):
			return fmt.Errorf("semantic corpus warm-up did not stop within %v", closeStepTimeout)
		}
		c.semanticWarmDone = nil
	}
	if c.SemanticSearcher != nil {
		err := runCloseStep("SemanticSearcher.Close", closeStepTimeout, c.SemanticSearcher.Close)
		c.SemanticSearcher = nil
		return err
	}
	return nil
}
