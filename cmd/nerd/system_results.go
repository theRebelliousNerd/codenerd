package main

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"time"
)

func systemResultBaselines(kernel core.Kernel) (int, int) {
	baseRouting := 0
	baseExec := 0
	if kernel == nil {
		return 0, 0
	}
	if facts, err := kernel.Query("routing_result"); err == nil {
		baseRouting = len(facts)
	}
	if facts, err := kernel.Query("execution_result"); err == nil {
		baseExec = len(facts)
	}
	return baseRouting, baseExec
}

func waitForSystemResults(ctx context.Context, kernel core.Kernel, baseRouting, baseExec int, timeout time.Duration) ([]core.Fact, []core.Fact) {
	if kernel == nil {
		return nil, nil
	}

	// Immediate check (helps when system shards are fast).
	routing := diffFacts(kernel, "routing_result", baseRouting)
	exec := diffFacts(kernel, "execution_result", baseExec)
	if len(routing) > 0 || len(exec) > 0 {
		return routing, exec
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, nil
		case <-timeoutCh:
			return diffFacts(kernel, "routing_result", baseRouting), diffFacts(kernel, "execution_result", baseExec)
		case <-ticker.C:
			routing := diffFacts(kernel, "routing_result", baseRouting)
			exec := diffFacts(kernel, "execution_result", baseExec)
			if len(routing) > 0 || len(exec) > 0 {
				return routing, exec
			}
		}
	}
}

func diffFacts(kernel core.Kernel, predicate string, baseline int) []core.Fact {
	facts, err := kernel.Query(predicate)
	if err != nil || len(facts) <= baseline {
		return nil
	}
	return facts[baseline:]
}

func formatSystemResults(routing, exec []core.Fact) string {
	if len(routing) == 0 && len(exec) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("System actions:\n")
	for _, f := range routing {
		if len(f.Args) >= 3 {
			sb.WriteString(fmt.Sprintf("- %v: %v (%v)\n", f.Args[0], f.Args[1], f.Args[2]))
		}
	}
	for _, f := range exec {
		if len(f.Args) >= 4 {
			sb.WriteString(fmt.Sprintf("- %v %v -> success=%v; %v\n", f.Args[0], f.Args[1], f.Args[2], f.Args[3]))
		}
	}
	return strings.TrimSpace(sb.String())
}
