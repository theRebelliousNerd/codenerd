package system

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"

	"github.com/stretchr/testify/require"
)

func TestRouterAsyncRequestCarriesIntegerTimeoutSeconds(t *testing.T) {
	k, err := core.NewRealKernel()
	require.NoError(t, err)
	cfg := DefaultRouterConfig()
	cfg.DefaultRoutes = []ToolRoute{{ActionPattern: "review_probe", ToolName: "review_probe_tool", Timeout: 17 * time.Second}}
	router := NewTactileRouterShardWithConfig(cfg)
	router.Kernel = k
	require.NoError(t, k.Assert(core.Fact{
		Predicate: "permitted_action",
		Args:      []any{"review-route", types.MangleAtom("/review_probe"), "target", "{}", time.Now().Unix()},
	}))
	require.NoError(t, router.processPermittedActions(context.Background()))
	rows, err := k.Query(`exec_request("review_probe_tool", "target", 17, CallID, Timestamp)`)
	require.NoError(t, err)
	require.Len(t, rows, 1, "the old float64 timeout was rejected by /number admission")
}

func TestPerceptionAmbiguityKeepsTheCompleteInputAsStrings(t *testing.T) {
	k, err := core.NewRealKernel()
	require.NoError(t, err)
	shard := NewPerceptionFirewallShard()
	shard.SetParentKernel(k)
	shard.SetLLMClient(stubLLMClient{response: strings.Replace(hypotheticalModifyEnvelope, `"confidence": 0.9`, `"confidence": 0.2`, 1)})
	attachPerceptionPrompt(t, shard)
	input := "/" + strings.Repeat("uncertain request ", 30)
	_, err = shard.Perceive(context.Background(), input, nil)
	require.NoError(t, err)
	rows, err := k.Query(`ambiguity_flag("/current_intent", Input, Hypothesis)`)
	require.NoError(t, err)
	require.Len(t, rows, 1, "intent ID belongs in the declared string slot")
	require.Equal(t, input, types.ExtractString(rows[0].Args[1]))
}
