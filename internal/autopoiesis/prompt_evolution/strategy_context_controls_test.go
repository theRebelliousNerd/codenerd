package prompt_evolution_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	pe "codenerd/internal/autopoiesis/prompt_evolution"
	nerdconfig "codenerd/internal/config"

	"github.com/stretchr/testify/require"
)

func TestStrategyCompileSkippedClassificationDoesNotInventUsage(t *testing.T) {
	for _, name := range []string{"disabled", "unconfigured", "no-provider", "no-store", "no-shard", "empty-query"} {
		t.Run(name, func(t *testing.T) {
			sec := nerdconfig.TypeSafeConfig{}
			if name == "disabled" {
				off := false
				sec.ProblemTypes = &off
			}
			var attempts atomic.Int32
			compiler, evolver := strategyContextCompiler(t, sec, func(*http.Request) (*http.Response, error) {
				attempts.Add(1)
				return strategyReply(), nil
			})
			cc := strategyContext()
			switch name {
			case "unconfigured":
				evolver.SetJev(nil, nil)
			case "no-provider":
				compiler.RegisterStrategyProvider(nil)
			case "no-store":
				compiler.RegisterStrategyProvider(pe.NewStrategyAtomProvider(nil, nil))
			case "no-shard":
				cc.ShardType = ""
			case "empty-query":
				cc.SemanticQuery, cc.IntentTarget, cc.IntentVerb = "", "", ""
			}
			tracker, observer, ctx := strategyMeter(t)
			result, err := compiler.Compile(ctx, cc)
			require.NoError(t, err)
			require.NotEmpty(t, result.Prompt)
			require.Zero(t, attempts.Load())
			assertStrategyUsage(t, tracker, observer, 0)
		})
	}
}

func TestStrategyCompileUnansweredJevKeepsBoundedPatternFallback(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		name := "transport-failure"
		if timeout {
			name = "local-timeout"
		}
		t.Run(name, func(t *testing.T) {
			var attempts atomic.Int32
			deadlineErrors := make(chan error, 2)
			compiler, _ := strategyContextCompiler(t, nerdconfig.TypeSafeConfig{TurnJudgmentTimeout: "10ms"}, func(r *http.Request) (*http.Response, error) {
				attempts.Add(1)
				if timeout {
					<-r.Context().Done()
					deadlineErrors <- r.Context().Err()
					return nil, r.Context().Err()
				}
				return nil, errors.New("strategy transport unavailable")
			})
			tracker, observer, ctx := strategyMeter(t)
			// A different prompt budget forces a compiler miss while retaining
			// the exact classification query. An unanswered judgment must not
			// enter the classifier's successful-answer cache.
			for _, budget := range []int{10000, 11000} {
				result, err := compiler.Compile(ctx, strategyContext().WithTokenBudget(budget, 1000))
				require.NoError(t, err)
				require.False(t, result.Stats.CacheHit)
				require.Equal(t, 1, result.Stats.StrategyAtoms, "pattern fallback lost the debugging playbook")
				if timeout {
					select {
					case err := <-deadlineErrors:
						require.ErrorIs(t, err, context.DeadlineExceeded)
					case <-time.After(5 * time.Second):
						t.Fatal("configured Jev timeout did not reach the strategy transport")
					}
				}
			}
			require.EqualValues(t, 2, attempts.Load())
			assertStrategyUsage(t, tracker, observer, 0)
		})
	}
}
