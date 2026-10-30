package prompt_evolution_test

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/usage"

	"github.com/stretchr/testify/require"
)

func TestStrategyCompileShutdownCancelsAndJoinsRequestBeforeClosingDB(t *testing.T) {
	started := make(chan context.Context, 1)
	canceled := make(chan error, 1)
	returnRequest := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(returnRequest) }) }
	compiler, _ := strategyContextCompiler(t, nerdconfig.TypeSafeConfig{TurnJudgmentTimeout: "1h"}, func(r *http.Request) (*http.Response, error) {
		started <- r.Context()
		select {
		case <-r.Context().Done():
			canceled <- r.Context().Err()
			<-returnRequest
			return nil, r.Context().Err()
		case <-returnRequest: // cleanup also releases the pre-fix background call
			return strategyReply(), nil
		}
	})
	t.Cleanup(release)
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	compiler.RegisterShardDB("owned-probe", db)
	tracker, observer, ctx := strategyMeter(t)
	t.Cleanup(func() {
		release()
		_ = compiler.Close()
	})
	compiled := make(chan error, 1)
	go func() {
		_, err := compiler.Compile(ctx, strategyContext())
		compiled <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("strategy request did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- compiler.Close() }()
	select {
	case err := <-canceled:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("compiler shutdown did not reach the Jev strategy request")
	}
	// Delaying transport return proves Close joins live work, independently of
	// how promptly the waiting Compile call returns on owner cancellation.
	require.NoError(t, db.Ping(), "owned DB closed while the request was active")
	select {
	case <-closed:
		t.Fatal("Close returned before the strategy request exited")
	default:
	}
	release()
	select {
	case err := <-closed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close failed to join the canceled strategy request")
	}
	require.Error(t, db.Ping(), "owned DB remained open after Close")
	select {
	case err := <-compiled:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Compile did not return on shutdown")
	}
	assertStrategyUsage(t, tracker, observer, 0)
}

func TestStrategyCompileCanceledWaiterPreservesSharedRequestAndUsageOwner(t *testing.T) {
	started := make(chan context.Context, 1)
	answer := make(chan struct{})
	var answerOnce sync.Once
	release := func() { answerOnce.Do(func() { close(answer) }) }
	var attempts atomic.Int32
	compiler, _ := strategyContextCompiler(t, nerdconfig.TypeSafeConfig{TurnJudgmentTimeout: "1h"}, func(r *http.Request) (*http.Response, error) {
		attempts.Add(1)
		started <- r.Context()
		select {
		case <-answer:
			return strategyReply(), nil
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	})
	t.Cleanup(release)
	tracker, observer, owner := strategyMeter(t)
	t.Cleanup(func() {
		release()
		_ = compiler.Close()
	})
	ctx, cancel := context.WithCancel(owner)
	defer cancel()
	first := make(chan error, 1)
	go func() {
		_, err := compiler.Compile(ctx, strategyContext())
		first <- err
	}()
	var requestContext context.Context
	select {
	case requestContext = <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("strategy request did not start")
	}
	cancel()
	select {
	case err := <-first:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("canceled caller remained blocked on strategy classification")
	}
	require.NoError(t, requestContext.Err(), "waiter cancellation reached shared work")
	other := &strategyObserver{}
	second := make(chan *prompt.CompilationResult, 1)
	secondErr := make(chan error, 1)
	go func() {
		result, err := compiler.Compile(usage.WithObserver(t.Context(), other), strategyContext())
		second <- result
		secondErr <- err
	}()
	require.Eventually(t, func() bool { return compiler.GetStats().Preflights == 2 }, 5*time.Second, time.Millisecond)
	release()
	select {
	case err := <-secondErr:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("remaining consumer did not receive the shared compilation")
	}
	require.NotEmpty(t, (<-second).Prompt)
	require.EqualValues(t, 1, compiler.GetStats().TotalCompilations)
	require.EqualValues(t, 1, attempts.Load())
	assertStrategyUsage(t, tracker, observer, 1)
	require.Empty(t, other.snapshot(), "shared request billed again to a later consumer")
}
