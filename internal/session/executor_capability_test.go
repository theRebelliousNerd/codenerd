package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	modulartools "codenerd/internal/tools"
)

var capabilityTestToolCounter atomic.Uint64

func validCapabilityTestConfig(toolNames ...string) *config.EffectiveAgentRuntimeConfig {
	return &config.EffectiveAgentRuntimeConfig{
		IdentityPrompt: "Capability boundary test agent.",
		AllowedTools:   toolNames,
		Policies:       []string{"policy/validation.mg"},
	}
}

func TestExecutorToolCapabilityEnvelopeFailsClosed(t *testing.T) {
	executor := &Executor{}

	tests := []struct {
		name     string
		cfg      *config.EffectiveAgentRuntimeConfig
		toolName string
		want     bool
	}{
		{name: "nil-config", cfg: nil, toolName: "read_file", want: false},
		{name: "empty-capability-list", cfg: validCapabilityTestConfig(), toolName: "read_file", want: false},
		{
			name:     "listed-tool",
			cfg:      validCapabilityTestConfig("read_file"),
			toolName: "read_file",
			want:     true,
		},
		{
			name:     "unlisted-tool",
			cfg:      validCapabilityTestConfig("read_file"),
			toolName: "write_file",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := executor.isToolAllowed(tt.toolName, tt.cfg); got != tt.want {
				t.Errorf("isToolAllowed(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestExecutorExecuteToolCallRequiresEffectiveCapability(t *testing.T) {
	toolName := fmt.Sprintf("jit_capability_test_%d", capabilityTestToolCounter.Add(1))
	var executions atomic.Int64
	if err := modulartools.Global().Register(&modulartools.Tool{
		Name:        toolName,
		Description: "capability boundary regression tool",
		Execute: func(context.Context, map[string]any) (string, error) {
			executions.Add(1)
			return "executed", nil
		},
	}); err != nil {
		t.Fatalf("register modular test tool: %v", err)
	}

	executor := &Executor{config: DefaultExecutorConfig()}
	executor.config.EnableSafetyGate = false

	for _, cfg := range []*config.EffectiveAgentRuntimeConfig{nil, validCapabilityTestConfig()} {
		out, err := executor.executeToolCall(context.Background(), ToolCall{Name: toolName}, cfg)
		if err == nil {
			t.Fatalf("executeToolCall() output = %q, want missing-capability error", out)
		}
		if !strings.Contains(err.Error(), "not allowed by effective JIT config") {
			t.Fatalf("executeToolCall() error = %q, want effective-config context", err)
		}
	}
	if got := executions.Load(); got != 0 {
		t.Fatalf("denied capability executed tool %d times", got)
	}

	allowed := validCapabilityTestConfig(toolName)
	out, err := executor.executeToolCall(context.Background(), ToolCall{Name: toolName}, allowed)
	if err != nil {
		t.Fatalf("executeToolCall() allowed capability error = %v", err)
	}
	if out != "executed" {
		t.Fatalf("executeToolCall() output = %q, want executed", out)
	}
	if got := executions.Load(); got != 1 {
		t.Fatalf("allowed capability executed tool %d times, want 1", got)
	}
}

func TestExecutorOuroborosRegistryDoesNotGrantCapability(t *testing.T) {
	allowedName := fmt.Sprintf("jit_ouroboros_allowed_%d", capabilityTestToolCounter.Add(1))
	deniedName := fmt.Sprintf("jit_ouroboros_denied_%d", capabilityTestToolCounter.Add(1))
	registry := core.NewToolRegistry(t.TempDir())
	for _, toolName := range []string{allowedName, deniedName} {
		if err := registry.RegisterToolWithInfo(&core.Tool{
			Name:        toolName,
			Command:     "go",
			Description: "Ouroboros capability boundary regression tool",
		}); err != nil {
			t.Fatalf("register Ouroboros test tool %q: %v", toolName, err)
		}
	}

	executor := &Executor{config: DefaultExecutorConfig()}
	executor.config.EnableSafetyGate = false
	executor.SetOuroborosRegistry(registry)
	effective := validCapabilityTestConfig(allowedName)

	out, err := executor.executeToolCall(context.Background(), ToolCall{Name: deniedName}, effective)
	if err == nil {
		t.Fatalf("executeToolCall() output = %q, want unlisted Ouroboros capability error", out)
	}
	if !strings.Contains(err.Error(), "not allowed by effective JIT config") {
		t.Fatalf("executeToolCall() error = %q, want effective-config context", err)
	}
	deniedTool, _ := registry.GetTool(deniedName)
	if deniedTool.ExecuteCount != 0 {
		t.Fatalf("unlisted Ouroboros tool execution count = %d, want 0", deniedTool.ExecuteCount)
	}

	// The command intentionally receives a JSON argument that is not a valid Go
	// subcommand. Reaching the registry (rather than an allowlist denial) and
	// incrementing ExecuteCount proves the listed capability crossed this gate.
	_, err = executor.executeToolCall(context.Background(), ToolCall{Name: allowedName}, effective)
	if err != nil && strings.Contains(err.Error(), "not allowed by effective JIT config") {
		t.Fatalf("listed Ouroboros capability was denied: %v", err)
	}
	allowedTool, _ := registry.GetTool(allowedName)
	if allowedTool.ExecuteCount != 1 {
		t.Fatalf("listed Ouroboros tool execution count = %d, want 1", allowedTool.ExecuteCount)
	}
}

func TestExecutorPiggybackCatalogHonorsOuroborosCapabilities(t *testing.T) {
	allowedName := fmt.Sprintf("jit_catalog_allowed_%d", capabilityTestToolCounter.Add(1))
	deniedName := fmt.Sprintf("jit_catalog_denied_%d", capabilityTestToolCounter.Add(1))
	registry := core.NewToolRegistry(t.TempDir())
	for _, toolName := range []string{allowedName, deniedName} {
		if err := registry.RegisterToolWithInfo(&core.Tool{Name: toolName, Command: "go"}); err != nil {
			t.Fatalf("register Ouroboros test tool %q: %v", toolName, err)
		}
	}

	executor := &Executor{}
	executor.SetOuroborosRegistry(registry)
	if catalog := executor.buildToolCatalogForPiggyback(nil); catalog != "" {
		t.Fatalf("nil config catalog = %q, want empty", catalog)
	}
	if catalog := executor.buildToolCatalogForPiggyback(validCapabilityTestConfig()); catalog != "" {
		t.Fatalf("empty config catalog = %q, want empty", catalog)
	}

	catalog := executor.buildToolCatalogForPiggyback(validCapabilityTestConfig(allowedName))
	if !strings.Contains(catalog, allowedName) {
		t.Errorf("catalog omitted allowed Ouroboros tool %q", allowedName)
	}
	if strings.Contains(catalog, deniedName) {
		t.Errorf("catalog exposed unlisted Ouroboros tool %q", deniedName)
	}
}

func TestExecutorToolCapabilityReadsAreRaceSafe(t *testing.T) {
	executor := &Executor{}
	effective := validCapabilityTestConfig("read_file")

	const workers = 32
	var wg sync.WaitGroup
	violations := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !executor.isToolAllowed("read_file", effective) {
				violations <- "listed tool was denied"
			}
			if executor.isToolAllowed("write_file", effective) {
				violations <- "unlisted tool was allowed"
			}
		}()
	}
	wg.Wait()
	close(violations)
	for violation := range violations {
		t.Error(violation)
	}
}
