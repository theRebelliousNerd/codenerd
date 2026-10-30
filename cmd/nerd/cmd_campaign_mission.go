package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/consultation"
	"codenerd/internal/core"
	"codenerd/internal/deliberation"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/sessionlog"
	coresys "codenerd/internal/system"
)

type missionProgressOutput struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *missionProgressOutput) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writer.Write(b)
}

func missionProgressContext(ctx context.Context, w io.Writer) context.Context {
	return sessionlog.WithLiveProgress(ctx, func(_ context.Context, ev sessionlog.LiveEvent) {
		if ev.Text != "" {
			fmt.Fprintf(w, "[%s] %s\n", ev.Agent, ev.Text)
		}
	})
}

func newRecurseMission(cortex *coresys.Cortex, root, text string, cfg *config.UserConfig) (*campaign.RecurseMission, error) {
	if cortex == nil || cortex.Kernel == nil || cortex.ShardManager == nil || cortex.VirtualStore == nil {
		return nil, fmt.Errorf("mission needs the booted kernel, shard manager and tools")
	}
	routing, explain := cfg.GetRoutingConfig(), cfg.GetExplainConfig()
	read := deliberation.ReadTools{Kernel: cortex.Kernel, Registry: cortex.VirtualStore.GetModularTools(), ContextBytes: routing.DeliberationContextBytes}
	consult := consultation.New(session.TaskConsultationSpawner{Executor: cortex.TaskExecutor})
	s := &deliberation.Service{Kernel: cortex.Kernel, Workspace: root, Session: cortex.SessionID(), Config: routing,
		Explain: core.ExplainOptions{MaxProofs: explain.MaxProofs, MaxDepth: explain.MaxDepth, MaxNodes: explain.MaxNodes,
			MaxEdges: explain.MaxEdges, MaxBytes: explain.MaxBytes},
		RunShard: func(ctx context.Context, role string, frame deliberation.Frame) (string, error) {
			body, err := json.Marshal(frame)
			if err != nil {
				return "", err
			}
			return cortex.ShardManager.Spawn(ctx, role, string(body))
		}, Probe: read.Execute,
		Consult: func(ctx context.Context, req deliberation.Consultation, request string) (string, error) {
			restricted := perception.RestrictionOf("mission deliberation reads only", perception.RestrictReadOnly)
			ctx = perception.WithRestriction(ctx, perception.RestrictionFromContext(ctx).Merge(restricted))
			answer, err := consult.Consult(ctx, consultation.Request{Requester: deliberation.Proposer,
				Specialist: req.Specialist, Question: req.Question, Context: request})
			if err != nil {
				return "", err
			}
			return answer.Advice, nil
		}}
	return &campaign.RecurseMission{Text: text, Limits: cfg.GetMissionConfig(), Deliberator: s}, nil
}
