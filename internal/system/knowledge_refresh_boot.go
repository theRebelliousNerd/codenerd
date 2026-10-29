package system

import (
	"context"

	nerdinit "codenerd/internal/init"
	"codenerd/internal/knowledge"
	"codenerd/internal/logging"
)

// initKnowledgeRefresh is the last boot step: it schedules the background
// knowledge refresh (knowledge_refresh.go) on the Cortex's lifetime. The
// sources are the session's own: grounded web search on the configured main
// and planner clients, Context7 through the Cortex's research tools (the
// same context7_fetch the model calls, on the Cortex-owned browser runtime),
// the pinned go doc, and the repository's importers from the world model.
func initKnowledgeRefresh(bctx *bootContext) error {
	if bctx == nil || bctx.kernel == nil {
		return nil
	}
	schedule, err := refreshSchedule(bctx.kernel)
	if err != nil {
		logging.Get(logging.CategoryContext).Warn("knowledge refresh not scheduled: %v", err)
		return nil
	}
	var context7 func(ctx context.Context, topic string) (string, error)
	if bctx.virtualStore != nil {
		if registry := bctx.virtualStore.GetModularTools(); registry != nil {
			context7 = func(ctx context.Context, topic string) (string, error) {
				result, err := registry.Execute(ctx, "context7_fetch", map[string]any{"topic": topic})
				if err != nil {
					return "", err
				}
				return result.Result, nil
			}
		}
	}
	refresher := &knowledgeRefresher{
		workspace: bctx.workspace,
		kernel:    bctx.kernel,
		engine:    bctx.embeddingEngine,
		acquirer: knowledge.NewAcquirer(knowledge.Config{
			Searchers: nerdinit.KnowledgeSearchers(bctx.llmClient, bctx.plannerLLMClient),
			Context7:  context7,
			Workspace: bctx.workspace,
			Kernel:    bctx.kernel,
		}),
	}
	lifetime, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		refresher.serve(lifetime, schedule)
	}()
	bctx.knowledgeRefresher = refresher
	bctx.knowledgeRefreshCancel = cancel
	bctx.knowledgeRefreshDone = done
	return nil
}
