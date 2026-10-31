package system

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/knowledge"
	"codenerd/internal/sessionlog"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

type refreshObserver struct {
	reports atomic.Int64
	unknown atomic.Bool
}

func (o *refreshObserver) Observed(usage.Report) { o.reports.Add(1) }
func (o *refreshObserver) UsageUnknown()         { o.unknown.Store(true) }

type refreshSearcher struct {
	MockLLMClient
	calls    atomic.Int64
	started  chan struct{}
	canceled chan struct{}
	finish   chan struct{}
	scope    *sessionlog.Scope
}

func (*refreshSearcher) SupportsGroundedWebSearch() bool { return true }
func (s *refreshSearcher) GroundedWebSearch(ctx context.Context, _ string) (*types.GroundedWebSearchResult, error) {
	if s.calls.Add(1) == 1 && s.started != nil {
		s.scope = sessionlog.ScopeFrom(ctx)
		close(s.started)
		<-ctx.Done()
		close(s.canceled)
		<-s.finish // cancellation must join, not merely send the signal
		return nil, ctx.Err()
	}
	u := types.UsageMetadata{InputTokens: 125, OutputTokens: 7, TotalTokens: 132, Known: types.UsageTokens}
	usage.RecordFromContext(ctx, usage.Report{Provider: "meta", Model: "muse-spark-1.3-contributor", Operation: "search", Usage: u})
	return &types.GroundedWebSearchResult{
		Text:      "The local deterministic fixture supplied this sourced guidance for safe calibration and repeatable testing.",
		Citations: []types.GroundedCitation{{URL: "https://example.invalid/local-fixture"}},
		Usage:     types.GroundedUsage{InputTokens: 125, OutputTokens: 7, TotalTokens: 132},
	}, nil
}

type immediateRefreshKernel struct{ SystemKernel }

func (k immediateRefreshKernel) Query(pred string) ([]core.Fact, error) {
	if pred == "knowledge_refresh_schedule" {
		return []core.Fact{{Predicate: pred, Args: []any{int64(0), int64(3600)}}}, nil
	}
	return k.SystemKernel.Query(pred)
}

func awaitRefresh[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for local refresh fixture")
		var zero T
		return zero
	}
}

// Exercise the actual boot scheduler, broker, real policy/KBs and host barrier.
// All acquisition is a local scripted searcher, never a provider or web server.
func TestKnowledgeRefreshMissionSuspensionOwnsCancellationAndUsage(t *testing.T) {
	ws := t.TempDir()
	agentDir := filepath.Join(ws, ".nerd", "agents", "WidgetExpert")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "prompts.yaml"), []byte(widgetExpertPromptsYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	writeAgentsJSON(t, ws, []map[string]any{{"name": "WidgetExpert", "type": "persistent", "status": "ready", "topics": []string{"widget calibration"}}})
	inner := &refreshSearcher{started: make(chan struct{}), canceled: make(chan struct{}), finish: make(chan struct{})}
	t.Cleanup(func() {
		select {
		case <-inner.finish:
		default:
			close(inner.finish)
		}
	})
	sink := broker.NewRingSink(16)
	wrapped, err := broker.Wrap(inner, broker.Config{Provider: "meta", Model: "muse-spark-1.3-contributor", Counter: broker.NewEstimatingCounter(nil), Sink: sink})
	if err != nil {
		t.Fatal(err)
	}
	caller := &refreshObserver{}
	ctx := usage.WithObserver(usage.WithSessionID(t.Context(), "unrelated-boot-caller"), caller)
	cfg := config.DefaultUserConfig()
	cfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	steps := defaultBootSteps()
	steps[len(steps)-1].run = func(bctx *bootContext) error {
		kernel, main, planner, vs := bctx.kernel, bctx.llmClient, bctx.plannerLLMClient, bctx.virtualStore
		bctx.kernel, bctx.llmClient, bctx.plannerLLMClient, bctx.virtualStore = immediateRefreshKernel{kernel}, wrapped, nil, nil
		defer func() {
			bctx.kernel, bctx.llmClient, bctx.plannerLLMClient, bctx.virtualStore = kernel, main, planner, vs
		}()
		return initKnowledgeRefresh(bctx)
	}
	cortex, err := bootCortexWithSteps(ctx, BootConfig{Workspace: ws, SessionID: "refresh-owner", UserConfigOverride: cfg, LLMClientOverride: &MockLLMClient{}}, steps)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { requireWorkspaceReleased(t, cortex, ws) })
	awaitRefresh(t, inner.started)
	if inner.scope == nil || !inner.scope.Service || inner.scope.Role != "knowledge-refresh" || inner.scope.ID != "refresh-owner" {
		t.Fatalf("background service lost its session-log identity: %+v", inner.scope)
	}

	type leaseResult struct {
		release func()
		err     error
	}
	leases := make(chan leaseResult, 1)
	go func() {
		release, err := cortex.SuspendKnowledgeRefresh(t.Context())
		leases <- leaseResult{release, err}
	}()
	awaitRefresh(t, inner.canceled)
	select {
	case <-leases:
		t.Fatal("mission admitted before the acquisition joined")
	default:
	}
	close(inner.finish)
	lease := awaitRefresh(t, leases)
	if lease.err != nil {
		t.Fatal(lease.err)
	}
	t.Cleanup(lease.release)
	second, err := cortex.SuspendKnowledgeRefresh(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second)

	missionObserver := &refreshObserver{}
	missionCtx := usage.WithObserver(t.Context(), missionObserver)
	if _, err := cortex.RefreshKnowledge(missionCtx); !errors.Is(err, errKnowledgeRefreshSuspended) {
		t.Fatalf("no-web mission refresh admitted: %v", err)
	}
	lease.release()
	lease.release() // one owner cannot release another owner's suspension
	if _, err := cortex.RefreshKnowledge(missionCtx); !errors.Is(err, errKnowledgeRefreshSuspended) {
		t.Fatalf("remaining lease bypassed: %v", err)
	}
	if inner.calls.Load() != 1 {
		t.Fatalf("suspended refresh called source %d times", inner.calls.Load())
	}
	if caller.reports.Load() != 0 || caller.unknown.Load() || missionObserver.reports.Load() != 0 || missionObserver.unknown.Load() {
		t.Fatal("background usage leaked into an unrelated caller or blocked mission")
	}
	receipts := sink.Receipts()
	if len(receipts) != 1 || receipts[0].Scope != "refresh-owner" || receipts[0].Purpose != broker.PurposeAutopoiesis || receipts[0].Phase != "knowledge_refresh" || receipts[0].UsageStatus != broker.UsageUnreported {
		t.Fatalf("background receipt lost ownership or uncertainty: %+v", receipts)
	}

	// A separate Cortex retains ordinary explicit refresh and nested observers.
	otherWS := t.TempDir()
	writeAgentsJSON(t, otherWS, []map[string]any{{"name": "WidgetExpert", "type": "persistent", "status": "ready", "topics": []string{"independent widgets"}}})
	otherKernel, err := core.NewRealKernelWithWorkspace(otherWS)
	if err != nil {
		t.Fatal(err)
	}
	other := &Cortex{knowledgeRefresher: &knowledgeRefresher{workspace: otherWS, kernel: otherKernel,
		acquirer: knowledge.Acquirer{Sources: []knowledge.Source{knowledge.WebSource{Provider: "meta", Searcher: wrapped.(types.GroundedWebSearcher)}}}}}
	outer, nested := &refreshObserver{}, &refreshObserver{}
	explicitCtx := usage.WithObserver(usage.WithObserver(usage.WithSessionID(t.Context(), "explicit-owner"), outer), nested)
	if report, err := other.RefreshKnowledge(explicitCtx); err != nil || report.Added == 0 {
		t.Fatalf("unrelated Cortex refresh blocked: %+v %v", report, err)
	}
	if outer.reports.Load() != 1 || nested.reports.Load() != 1 {
		t.Fatalf("explicit nested usage reports lost: %d / %d", outer.reports.Load(), nested.reports.Load())
	}
	if caller.reports.Load() != 0 || missionObserver.reports.Load() != 0 {
		t.Fatal("unrelated refresh charged a mission/boot caller")
	}

	// No acquisition can write the original workspace while its lease is held.
	kb, err := store.NewLocalStore(types.AgentKnowledgeDBPath(filepath.Join(ws, ".nerd"), "widgetexpert"))
	if err != nil {
		t.Fatal(err)
	}
	atoms, err := kb.GetAllKnowledgeAtoms()
	_ = kb.Close()
	if err != nil || len(atoms) != 0 {
		t.Fatalf("canceled/suspended workspace changed: %d atoms, %v", len(atoms), err)
	}
	second()
	// Releasing the last lease wakes the real scheduler; hold its next barrier
	// to join the resumed acquisition before reading the receipt and KB.
	deadline := time.Now().Add(10 * time.Second)
	for inner.calls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if inner.calls.Load() < 3 {
		t.Fatal("ordinary background refresh did not resume")
	}
	end, err := cortex.SuspendKnowledgeRefresh(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer end()
	for _, rec := range sink.Receipts() {
		if rec.Scope == "refresh-owner" && (rec.Purpose != broker.PurposeAutopoiesis || rec.Phase != "knowledge_refresh") {
			t.Fatalf("resumed service lost attribution: %+v", rec)
		}
	}
}

func TestKnowledgeRefreshSuspensionCancellationAndWaitingRun(t *testing.T) {
	r := &knowledgeRefresher{}
	active, finish, err := r.admission.enter(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	cortex := &Cortex{knowledgeRefresher: r}
	ctx, cancel := context.WithCancel(t.Context())
	waiting := make(chan error, 1)
	go func() { _, _, err := r.admission.enter(ctx); waiting <- err }()
	cancel()
	if err := awaitRefresh(t, waiting); !errors.Is(err, context.Canceled) {
		t.Fatalf("queued run ignored cancellation: %v", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	paused := make(chan error, 1)
	go func() {
		release, err := cortex.SuspendKnowledgeRefresh(ctx)
		if release != nil {
			release()
		}
		paused <- err
	}()
	awaitRefresh(t, active.Done())
	cancel()
	if err := awaitRefresh(t, paused); !errors.Is(err, context.Canceled) {
		t.Fatalf("unjoined mission admitted: %v", err)
	}
}

func TestKnowledgeRefreshBootSuppressionAndCacheIsolation(t *testing.T) {
	ws := t.TempDir()
	writeCortexIdentityConfig(t, ws, `{}`)
	var calls int
	boot := func(ctx context.Context, ws, _ string, _ []string) (*Cortex, error) {
		calls++
		bctx := &bootContext{ctx: ctx, workspace: ws, sessionID: "cache-refresh", kernel: &MockSystemKernel{facts: []core.Fact{{Predicate: "knowledge_refresh_schedule", Args: []any{int64(3600), int64(3600)}}}}}
		if err := initKnowledgeRefresh(bctx); err != nil {
			return nil, err
		}
		return cortexFromBootContext(bctx), nil
	}
	normal, err := getOrBootCortex(t.Context(), ws, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	defer normal.Close()
	restricted, err := getOrBootCortex(WithoutKnowledgeRefresh(t.Context()), ws, "", nil, boot)
	if err != nil {
		t.Fatal(err)
	}
	defer restricted.Close()
	if normal == restricted || normal.knowledgeRefresher == nil || restricted.knowledgeRefresher != nil {
		t.Fatal("mission reused or scheduled unrestricted refresh")
	}
	again, err := getOrBootCortex(t.Context(), ws, "", nil, boot)
	if err != nil || again != normal || calls != 2 {
		t.Fatalf("mission changed ordinary cache wiring: same=%v boots=%d err=%v", again == normal, calls, err)
	}
	if release, err := restricted.SuspendKnowledgeRefresh(t.Context()); err != nil {
		t.Fatal(err)
	} else {
		release()
	}
	if _, err := restricted.RefreshKnowledge(t.Context()); err == nil {
		t.Fatal("boot-suppressed acquisition allowed explicitly")
	}
}
