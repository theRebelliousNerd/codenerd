package system

import (
	"context"
	"testing"
	"time"
	"fmt"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

func BenchmarkEvaluatePolicy_MultipleActions(b *testing.B) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		b.Fatalf("NewRealKernel() error = %v", err)
	}

	exec := NewExecutivePolicyShard()
	exec.SetParentKernel(kernel)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
        // Assert 10 facts to evaluate
        for j := 0; j < 10; j++ {
            _ = kernel.Assert(types.Fact{
                Predicate: "next_action",
                Args:      []any{fmt.Sprintf("/id%d", j), "/action", "/target", "{\"key\":\"value\"}", "rule", time.Now().Unix()},
            })
        }
		exec.evaluatePolicy(ctx)
	}
}
