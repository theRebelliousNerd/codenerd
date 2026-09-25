package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile/swebench"

	"github.com/spf13/cobra"
)

var (
	swebenchDataset    string
	swebenchInstanceID string
	swebenchPatchFile  string
	swebenchModelName  string
)

// swebenchCmd groups SWE-bench benchmark operations.
var swebenchCmd = &cobra.Command{
	Use:   "swebench",
	Short: "SWE-bench benchmark operations",
	Long: `SWE-bench benchmark operations.

This command bridges the SWE-bench dataset loader (internal/tactile/swebench)
and the kernel-routed handlers in internal/core/virtual_store_python.go, which
run each instance in a persistent container.`,
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

The handler builds the instance's environment in a container (requires
Docker) and fails, rather than claiming success, when it cannot. After routing
the kernel is queried for swebench_instance and swebench_expected_fail_to_pass
to prove the join landed expectation facts.`,
	RunE: runSwebenchSetup,
}

// swebenchEvaluateCmd sets an instance up, evaluates a patch against it and
// prints the kernel's verdict.
var swebenchEvaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Evaluate a patch against a SWE-bench instance and print the kernel's verdict",
	Long: `Set up a SWE-bench instance in a container, apply a model patch, run its
FAIL_TO_PASS and PASS_TO_PASS tests, and print the verdict the kernel derives
(benchmarks.mg swebench_resolved / swebench_unmet_expectation) from the
recorded test results. Requires Docker.`,
	RunE: runSwebenchEvaluate,
}

func init() {
	swebenchCmd.AddCommand(swebenchSetupCmd)
	swebenchCmd.AddCommand(swebenchEvaluateCmd)
	swebenchEvaluateCmd.Flags().StringVar(&swebenchDataset, "dataset", "", "Path to SWE-bench instances file (JSON or JSONL) (required)")
	swebenchEvaluateCmd.Flags().StringVar(&swebenchInstanceID, "instance", "", "Instance ID to evaluate (defaults to first instance in file)")
	swebenchEvaluateCmd.Flags().StringVar(&swebenchPatchFile, "patch-file", "", "Unified diff to evaluate (required)")
	swebenchEvaluateCmd.Flags().StringVar(&swebenchModelName, "model", "codenerd", "Model name recorded with the evaluation")
	_ = swebenchEvaluateCmd.MarkFlagRequired("dataset")
	_ = swebenchEvaluateCmd.MarkFlagRequired("patch-file")
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
	ctx, cancel := operationContext(baseCtx)
	defer cancel()

	inst, err := selectSwebenchInstance(swebenchDataset, swebenchInstanceID)
	if err != nil {
		return err
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

// selectSwebenchInstance loads the dataset and picks the named instance, or
// the first one when id is empty.
func selectSwebenchInstance(datasetPath, id string) (*swebench.Instance, error) {
	if datasetPath == "" {
		return nil, fmt.Errorf("--dataset is required")
	}
	instances, err := swebench.LoadInstances(datasetPath)
	if err != nil {
		// A single pretty-printed instance object is neither a JSON array nor
		// JSONL; accept it as a one-instance dataset.
		single, singleErr := swebench.LoadInstance(datasetPath)
		if singleErr != nil || single.InstanceID == "" {
			return nil, fmt.Errorf("load instances: %w", err)
		}
		instances = []*swebench.Instance{single}
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("no instances found in %s", datasetPath)
	}
	if id == "" {
		return instances[0], nil
	}
	for _, cand := range instances {
		if cand.InstanceID == id {
			return cand, nil
		}
	}
	return nil, fmt.Errorf("instance %q not found in %s", id, datasetPath)
}

func runSwebenchEvaluate(cmd *cobra.Command, args []string) error {
	baseCtx := cmd.Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := operationContext(baseCtx)
	defer cancel()

	inst, err := selectSwebenchInstance(swebenchDataset, swebenchInstanceID)
	if err != nil {
		return err
	}
	patch, err := os.ReadFile(swebenchPatchFile)
	if err != nil {
		return fmt.Errorf("read patch: %w", err)
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

	route := func(action string, payload map[string]any) error {
		fact := core.Fact{
			Predicate: "next_action",
			Args:      []any{fmt.Sprintf("swebench-%s-%d", strings.TrimPrefix(action, "/"), time.Now().UnixNano()), action, inst.InstanceID, payload},
		}
		if _, err := routePermittedAction(ctx, cortex.VirtualStore, cortex.Kernel, fact); err != nil {
			return fmt.Errorf("route %s: %w", action, err)
		}
		return nil
	}
	if err := route("/swebench_setup", swebenchSetupPayload(inst)); err != nil {
		return err
	}
	defer func() {
		_ = route("/swebench_teardown", map[string]any{"instance_id": inst.InstanceID})
	}()
	if err := route("/swebench_evaluate", map[string]any{
		"instance_id": inst.InstanceID,
		"patch":       string(patch),
		"model_name":  swebenchModelName,
	}); err != nil {
		return err
	}

	resolved, unmet, err := core.SWEBenchVerdict(cortex.Kernel, inst.InstanceID)
	if err != nil {
		return err
	}
	fmt.Printf("SWE-bench %s: resolved=%t\n", inst.InstanceID, resolved)
	for _, test := range unmet {
		fmt.Printf("  unmet: %s\n", test)
	}
	return nil
}
