package main

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile/swebench"

	"github.com/spf13/cobra"
)

var (
	swebenchDataset    string
	swebenchInstanceID string
)

// swebenchCmd groups SWE-bench benchmark operations.
var swebenchCmd = &cobra.Command{
	Use:   "swebench",
	Short: "SWE-bench benchmark operations",
	Long: `SWE-bench benchmark operations.

This command bridges the SWE-bench dataset loader (internal/tactile/swebench)
and the kernel-routed handlers in internal/core/virtual_store_python.go.`,
	RunE: parentGroupRunE,
}

// swebenchSetupCmd loads one instance and routes it through the kernel.
var swebenchSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup a SWE-bench instance environment via the kernel",
	Long: `Load a SWE-bench instance from a dataset file and route it through the kernel.

The dataset is a SWE-bench instances file (JSON array or JSONL). When --instance
is absent the first instance in the file is used. The instance is dispatched
through routePermittedAction as next_action(..., "/swebench_setup", <instance_id>, <payload>).

After routing the kernel is queried for swebench_instance and
swebench_expected_fail_to_pass to prove the join landed expectation facts.`,
	RunE: runSwebenchSetup,
}

func init() {
	swebenchCmd.AddCommand(swebenchSetupCmd)
	swebenchSetupCmd.Flags().StringVar(&swebenchDataset, "dataset", "", "Path to SWE-bench instances file (JSON or JSONL) (required)")
	swebenchSetupCmd.Flags().StringVar(&swebenchInstanceID, "instance", "", "Instance ID to setup (defaults to first instance in file)")
	_ = swebenchSetupCmd.MarkFlagRequired("dataset")
}

// swebenchSetupPayload builds the VirtualStore payload for handleSWEBenchSetup.
//
// handleSWEBenchSetup pulls its slice fields with Go type assertions:
//
//	req.Payload["fail_to_pass"].([]any)
//	req.Payload["pass_to_pass"].([]any)
//
// Instance.FailToPass and Instance.PassToPass are []string. A []string assigned
// directly into the payload FAILS the .([]any) assertion and the handler silently
// records zero expectations. Convert element-by-element into []any so the
// assertion succeeds and expectations are actually landed.
func swebenchSetupPayload(inst *swebench.Instance) map[string]any {
	if inst == nil {
		return map[string]any{
			"instance_id":       "",
			"repo":              "",
			"base_commit":       "",
			"problem_statement": "",
			"fail_to_pass":      []any{},
			"pass_to_pass":      []any{},
		}
	}
	failAny := make([]any, len(inst.FailToPass))
	for i, v := range inst.FailToPass {
		failAny[i] = v
	}
	passAny := make([]any, len(inst.PassToPass))
	for i, v := range inst.PassToPass {
		passAny[i] = v
	}
	return map[string]any{
		"instance_id":       inst.InstanceID,
		"repo":              inst.Repo,
		"base_commit":       inst.BaseCommit,
		"problem_statement": inst.ProblemStatement,
		"fail_to_pass":      failAny,
		"pass_to_pass":      passAny,
	}
}

func runSwebenchSetup(cmd *cobra.Command, args []string) error {
	baseCtx := cmd.Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	datasetPath := swebenchDataset
	if datasetPath == "" {
		return fmt.Errorf("--dataset is required")
	}

	instances, err := swebench.LoadInstances(datasetPath)
	if err != nil {
		return fmt.Errorf("load instances: %w", err)
	}
	if len(instances) == 0 {
		return fmt.Errorf("no instances found in %s", datasetPath)
	}

	var inst *swebench.Instance
	if swebenchInstanceID != "" {
		for _, cand := range instances {
			if cand.InstanceID == swebenchInstanceID {
				inst = cand
				break
			}
		}
		if inst == nil {
			return fmt.Errorf("instance %q not found in %s", swebenchInstanceID, datasetPath)
		}
	} else {
		inst = instances[0]
	}

	key := resolveAPIKey(apiKey, workspace)
	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	if cortex.VirtualStore != nil {
		cortex.VirtualStore.DisableBootGuard()
	}

	payload := swebenchSetupPayload(inst)

	actionID := fmt.Sprintf("swebench-setup-%d", time.Now().UnixNano())
	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			actionID,
			"/swebench_setup",
			inst.InstanceID,
			payload,
		},
	}

	if _, err := routePermittedAction(ctx, cortex.VirtualStore, cortex.Kernel, fact); err != nil {
		return fmt.Errorf("route /swebench_setup: %w", err)
	}

	// Prove the join rather than printing that it worked.
	instanceFacts, err := cortex.Kernel.Query("swebench_instance")
	if err != nil {
		return fmt.Errorf("query swebench_instance: %w", err)
	}
	failFacts, err := cortex.Kernel.Query("swebench_expected_fail_to_pass")
	if err != nil {
		return fmt.Errorf("query swebench_expected_fail_to_pass: %w", err)
	}
	passFacts, err := cortex.Kernel.Query("swebench_expected_pass_to_pass")
	if err != nil {
		return fmt.Errorf("query swebench_expected_pass_to_pass: %w", err)
	}

	fmt.Printf("swebench_instance facts: %d\n", len(instanceFacts))
	for _, f := range instanceFacts {
		fmt.Printf("  %s\n", f.String())
	}
	fmt.Printf("swebench_expected_fail_to_pass facts: %d\n", len(failFacts))
	for _, f := range failFacts {
		fmt.Printf("  %s\n", f.String())
	}
	fmt.Printf("swebench_expected_pass_to_pass facts: %d\n", len(passFacts))
	for _, f := range passFacts {
		fmt.Printf("  %s\n", f.String())
	}
	totalExpectation := len(failFacts) + len(passFacts)
	fmt.Printf("Total expectation facts: %d (fail_to_pass=%d + pass_to_pass=%d)\n", totalExpectation, len(failFacts), len(passFacts))

	expectedTotal := len(inst.FailToPass) + len(inst.PassToPass)
	if totalExpectation == 0 && expectedTotal > 0 {
		return fmt.Errorf("setup landed zero expectation facts (expected %d: fail_to_pass=%d pass_to_pass=%d) -- payload []string vs []any type-assertion bug; check swebenchSetupPayload converts slices element-by-element to []any", expectedTotal, len(inst.FailToPass), len(inst.PassToPass))
	}
	if len(failFacts) == 0 && len(inst.FailToPass) > 0 {
		return fmt.Errorf("setup landed zero fail_to_pass facts (expected %d) -- payload []string vs []any type-assertion bug", len(inst.FailToPass))
	}

	fmt.Printf("SWE-bench setup verified for %s (%s@%s) expectations=%d\n", inst.InstanceID, inst.Repo, inst.BaseCommit, totalExpectation)
	return nil
}
