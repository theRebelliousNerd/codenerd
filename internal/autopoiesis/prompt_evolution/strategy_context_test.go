package prompt_evolution_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/learnjudge"
	"codenerd/internal/prompt"
	"codenerd/internal/sessionlog"
	"codenerd/internal/system"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"

	"github.com/stretchr/testify/require"
)

type strategyRoundTripper func(*http.Request) (*http.Response, error)

func (f strategyRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func strategyReply() *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{
		"model":"strategy-test-model",
		"answers":{
			"problem_kind":{"type":"choice","choice":"debugging","confidence":0.9,"probabilities":{"debugging":0.9}},
			"problem_kind_fits":{"type":"noul","noul":0.95}
		},
		"usage":{"input_tokens":37,"output_tokens":3}
	}`))}
}

type strategyObserver struct {
	mu      sync.Mutex
	reports []usage.Report
}

func (o *strategyObserver) Observed(r usage.Report) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reports = append(o.reports, r)
}

func (o *strategyObserver) snapshot() []usage.Report {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]usage.Report(nil), o.reports...)
}

// Exercise Compile -> source collection -> the evolver's concrete provider ->
// classifier -> JudgeProblemType -> TypeSafe.Client.Ask, including its actual
// usage and exchange adapters. Only the HTTP transport is replaced.
func strategyContextCompiler(t *testing.T, sec nerdconfig.TypeSafeConfig, transport strategyRoundTripper) (*prompt.JITPromptCompiler, *pe.PromptEvolver) {
	t.Helper()
	client, err := typesafe.NewClient(typesafe.Options{
		Model: "strategy-test-model", BaseURL: "https://strategy.invalid", APIKey: "test-key",
		HTTPClient: &http.Client{Transport: transport}, Retry: &typesafe.RetryPolicy{},
	})
	require.NoError(t, err)
	kernel, err := core.NewRealKernel()
	require.NoError(t, err)
	identity := prompt.NewPromptAtom("identity/context-probe", prompt.CategoryIdentity, "Investigate the reported fault.")
	identity.IsMandatory = true
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(prompt.NewEmbeddedCorpus([]*prompt.PromptAtom{identity})),
		prompt.WithKernel(system.NewKernelAdapter(kernel)),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, compiler.Close()) })
	evolver, err := pe.NewPromptEvolver(t.TempDir(), nil, pe.DefaultEvolverConfig(), compiler, nerdconfig.JITConfig{})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, compiler.Close())
		require.NoError(t, evolver.Close())
	})
	evolver.SetJev(client, &sec)
	compiler.RegisterStrategyProvider(evolver.NewStrategyAtomProviderFor())
	return compiler, evolver
}

func strategyContext() *prompt.CompilationContext {
	return prompt.NewCompilationContext().WithShard("/coder", "context-probe", "coder").
		WithSemanticQuery("fix the parser crash", 1).WithTokenBudget(10000, 1000)
}

func strategyMeter(t *testing.T) (*usage.Tracker, *strategyObserver, context.Context) {
	t.Helper()
	tracker, err := usage.NewTracker(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, tracker.Close()) })
	observer := &strategyObserver{}
	ctx := usage.WithTurnID(usage.NewContext(t.Context(), tracker), "strategy-turn")
	ctx = usage.WithSessionID(usage.WithObserver(ctx, observer), "strategy-session")
	return tracker, observer, sessionlog.WithTask(sessionlog.WithTurn(ctx, 7), "strategy-task")
}

func assertStrategyUsage(t *testing.T, tracker *usage.Tracker, observer *strategyObserver, count int) {
	t.Helper()
	reports := observer.snapshot()
	require.Len(t, reports, count)
	for _, r := range reports {
		require.Equal(t, typesafe.Provider, r.Provider)
		require.Equal(t, "strategy-test-model", r.Model)
		require.Equal(t, learnjudge.OpProblemType, r.Operation)
		require.Equal(t, 37, r.Usage.InputTokens)
		require.Equal(t, 3, r.Usage.OutputTokens)
		require.True(t, r.Usage.Reported())
	}
	want := usage.TokenCounts{Input: int64(37 * count), Output: int64(3 * count), Total: int64(40 * count)}
	require.Equal(t, want, tracker.Stats().TotalProject)
	require.Equal(t, want, tracker.TurnTokens("strategy-turn"))
	require.Equal(t, want, tracker.SessionTokens("strategy-session"))
	if count > 0 {
		require.Equal(t, want, tracker.Stats().ByOperation[learnjudge.OpProblemType])
	}
}

func TestStrategyCompilePreservesUsageAndExchangeContext(t *testing.T) {
	workspace := t.TempDir()
	log := sessionlog.NewStore(sessionlog.Options{Workspace: workspace, Session: "strategy-session", Enabled: func() bool { return true }})
	previous := sessionlog.Current()
	sessionlog.Install(log)
	t.Cleanup(func() { sessionlog.Install(previous) })
	var attempts atomic.Int32
	var requestContext context.Context
	var requestBody struct {
		State map[string]any `json:"state"`
	}
	compiler, _ := strategyContextCompiler(t, nerdconfig.TypeSafeConfig{}, func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		requestContext = r.Context()
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			return nil, err
		}
		return strategyReply(), nil
	})
	tracker, observer, ctx := strategyMeter(t)
	first, err := compiler.Compile(ctx, strategyContext())
	require.NoError(t, err)
	require.Equal(t, 1, first.Stats.StrategyAtoms)
	require.Same(t, tracker, usage.FromContext(requestContext))
	require.Equal(t, "strategy-turn", usage.TurnIDFromContext(requestContext))
	require.Equal(t, "fix the parser crash", requestBody.State["task"])
	require.NotContains(t, requestBody.State, "agent", "context repair changed classification evidence")
	assertStrategyUsage(t, tracker, observer, 1)

	// The complete compiler key hits. A changed budget misses that cache but
	// reuses the classifier's answer for the same task; neither bills again.
	warm, err := compiler.Compile(ctx, strategyContext())
	require.NoError(t, err)
	require.True(t, warm.Stats.CacheHit)
	require.Equal(t, first.Prompt, warm.Prompt)
	classified, err := compiler.Compile(ctx, strategyContext().WithTokenBudget(11000, 1000))
	require.NoError(t, err)
	require.False(t, classified.Stats.CacheHit)
	require.Equal(t, first.Prompt, classified.Prompt)
	require.EqualValues(t, 1, attempts.Load())
	assertStrategyUsage(t, tracker, observer, 1)

	// Exchange evidence is independent of delivery to the aggregate meter.
	dir, err := sessionlog.SessionDir(workspace, "strategy-session")
	require.NoError(t, err)
	files, err := filepath.Glob(filepath.Join(dir, sessionlog.JevFolder, "*.json"))
	require.NoError(t, err)
	require.Len(t, files, 1)
	raw, err := os.ReadFile(files[0])
	require.NoError(t, err)
	var exchange sessionlog.Exchange
	require.NoError(t, json.Unmarshal(raw, &exchange))
	require.Equal(t, 7, exchange.Correlation.Turn)
	require.Equal(t, "strategy-task", exchange.Correlation.Task)
	require.Equal(t, "strategy-session", exchange.Correlation.Session)
	require.Equal(t, learnjudge.OpProblemType, exchange.Purpose)
	require.NotNil(t, exchange.Usage)
	require.Equal(t, 37, exchange.Usage.InputTokens)
	require.Equal(t, 3, exchange.Usage.OutputTokens)
	require.Equal(t, "reported", exchange.Usage.Status)
	require.Len(t, exchange.Wire, 1)
	require.Equal(t, sessionlog.OutcomeOK, exchange.Outcome)
	n, err := log.Failures()
	require.NoError(t, err)
	require.Zero(t, n)
}
