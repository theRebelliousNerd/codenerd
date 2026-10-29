package campaign

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

func toolCreateOrchestrator(t *testing.T, kernel core.Kernel) *Orchestrator {
	t.Helper()
	return &Orchestrator{kernel: kernel, campaign: &Campaign{ID: "/campaign_tools", Goal: "keep vet clean"}}
}

// A tool-create task for a capability a registered tool already provides
// resolves at once to that tool. It used to poll for a registration the
// kernel's tool_generation_blocked guarantees never comes, for 30 minutes.
func TestToolCreate_AProvidedCapabilityResolvesToItsTool(t *testing.T) {
	t.Parallel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	o := toolCreateOrchestrator(t, kernel)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	start := time.Now()
	res, err := o.executeToolCreateTask(ctx, &Task{ID: "/t_vet", Artifacts: []TaskArtifact{{Path: "go_vet"}}})
	if err != nil {
		t.Fatalf("executeToolCreateTask: %v (after %s)", err, time.Since(start))
	}
	out, _ := res.(map[string]any)
	if out["status"] != "complete" || out["tool_name"] != "run_lint" {
		t.Fatalf("result = %v, want complete with run_lint", res)
	}
}

// A capability that must never be generated fails at once, with the reason.
func TestToolCreate_ADangerousCapabilityFailsWithTheReason(t *testing.T) {
	t.Parallel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	o := toolCreateOrchestrator(t, kernel)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = o.executeToolCreateTask(ctx, &Task{ID: "/t_exec", Description: "exec_arbitrary"})
	if err == nil || !strings.Contains(err.Error(), "blocked by policy (dangerous)") {
		t.Fatalf("err = %v, want a refusal naming the dangerous capability", err)
	}
}

// A generation autopoiesis reports failed ends the wait with its reason, and
// a timeout is a failure, not a pending success.
func TestToolCreate_AFailedGenerationEndsTheWait(t *testing.T) {
	t.Parallel()
	kernel := &MockKernel{Facts: []core.Fact{
		{Predicate: "tool_generation_failed", Args: []any{"/frobnicate_widgets", "safety check rejected the generated source"}},
	}}
	o := toolCreateOrchestrator(t, kernel)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := o.executeToolCreateTask(ctx, &Task{ID: "/t_new", Description: "frobnicate_widgets"})
	if err == nil || !strings.Contains(err.Error(), "safety check rejected") {
		t.Fatalf("err = %v, want the generation's failure", err)
	}
}

func TestCapabilityAtom(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]types.MangleAtom{"go_vet": "/go_vet", "/go_vet": "/go_vet", " lint2 ": "/lint2"} {
		if got, ok := capabilityAtom(in); !ok || got != want {
			t.Errorf("capabilityAtom(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, prose := range []string{"", "a tool that runs vet", "Go-Vet", "x\"y"} {
		if _, ok := capabilityAtom(prose); ok {
			t.Errorf("capabilityAtom(%q) named a capability", prose)
		}
	}
}
