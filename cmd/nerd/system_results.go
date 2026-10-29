package main

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
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
	const maxLines = 25
	const maxField = 160

	trunc := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
		if len(s) > maxField {
			return s[:maxField] + "..."
		}
		return s
	}

	lines := make([]string, 0, len(routing)+len(exec))

	// routing_result(ActionID, Result, Details, Timestamp).
	for _, f := range routing {
		if len(f.Args) < 2 {
			continue
		}
		actionID := types.ExtractString(f.Args[0])
		result := types.ExtractString(f.Args[1])
		details := ""
		if len(f.Args) >= 3 {
			details = trunc(types.ExtractString(f.Args[2]))
		}
		if details == "" || details == "()" {
			lines = append(lines, fmt.Sprintf("- %s: %s", actionID, result))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", actionID, result, details))
	}

	// execution_result(ActionID, Type, Target, Success, Output, Timestamp).
	for _, f := range exec {
		if len(f.Args) < 4 {
			continue
		}
		actionID := types.ExtractString(f.Args[0])
		actionType := types.ExtractString(f.Args[1])
		target := ""
		success := ""
		output := ""
		if len(f.Args) >= 3 {
			target = trunc(types.ExtractString(f.Args[2]))
		}
		if len(f.Args) >= 4 {
			success = types.ExtractString(f.Args[3])
		}
		if len(f.Args) >= 5 {
			output = trunc(types.ExtractString(f.Args[4]))
		}

		line := fmt.Sprintf("- %s: %s", actionID, actionType)
		if strings.TrimSpace(target) != "" {
			line += fmt.Sprintf(" target=%s", target)
		}
		if strings.TrimSpace(success) != "" {
			line += fmt.Sprintf(" success=%s", success)
		}
		if strings.TrimSpace(output) != "" {
			line += fmt.Sprintf(" output=%s", output)
		}
		lines = append(lines, line)
	}

	var sb strings.Builder
	total := len(lines)
	if total > maxLines {
		sb.WriteString(fmt.Sprintf("System actions (showing last %d of %d):\n", maxLines, total))
		lines = lines[total-maxLines:]
	} else {
		sb.WriteString("System actions:\n")
	}
	for _, line := range lines {
		sb.WriteString(line + "\n")
	}
	return strings.TrimSpace(sb.String())
}
