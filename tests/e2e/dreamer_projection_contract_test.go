//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
)

const projectionGuardedTarget = "/fixture/guarded"
const projectionSafeTarget = "/fixture/ordinary"
const projectionReason = "fixture target protected"

func projectionContractKernel(t *testing.T, action, target string) (*core.RealKernel, *core.Dreamer) {
	t.Helper()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	k.LoadPolicy(fmt.Sprintf("panic_state(Id, %q) :- projected_action(Id, %s, %s).", projectionReason, action, target))
	// Materialize the parent's policy before cloning. No speculative action is
	// asserted here; all rule firing must originate in the real Dreamer clone.
	if _, err := k.Query("panic_state"); err != nil {
		t.Fatalf("fixture policy did not evaluate: %v", err)
	}
	return k, core.NewDreamer(k)
}

func projectionParentFacts(t *testing.T, k *core.RealKernel) map[string][]core.Fact {
	t.Helper()
	facts := make(map[string][]core.Fact)
	for _, predicate := range []string{"projected_action", "projected_fact", "hypothetical", "panic_state"} {
		var err error
		facts[predicate], err = k.Query(predicate)
		if err != nil {
			t.Fatalf("query parent %s: %v", predicate, err)
		}
	}
	return facts
}

func assertProjectedRequest(t *testing.T, req core.ActionRequest, result core.DreamResult, unsafe bool) {
	t.Helper()
	if result.ActionID == "" || !reflect.DeepEqual(result.Request, req) || result.Unsafe != unsafe {
		t.Fatalf("wrong request/verdict: request=%+v result=%+v want_unsafe=%t", req, result, unsafe)
	}
	count := 0
	for _, fact := range result.ProjectedFacts {
		if fact.Predicate != "projected_action" {
			continue
		}
		count++
		if len(fact.Args) != 3 {
			t.Fatalf("projection arity=%d, want 3", len(fact.Args))
		}
		id, idOK := fact.Args[0].(string)
		action, actionOK := fact.Args[1].(core.MangleAtom)
		target, targetOK := fact.Args[2].(string)
		if !idOK || !actionOK || !targetOK || id != result.ActionID || action != core.MangleAtom("/"+string(req.Type)) || target != strings.TrimSpace(req.Target) {
			t.Fatalf("projection lost exact typed request: %#v, want id=%s action=/%s target=%q", fact.Args, result.ActionID, req.Type, strings.TrimSpace(req.Target))
		}
	}
	if count != 1 {
		t.Fatalf("expected one actual projected request, got %d", count)
	}
	if unsafe {
		if !strings.Contains(result.Reason, projectionReason) || !slices.Equal(result.PanicReasons, []string{projectionReason}) {
			t.Fatalf("unsafe verdict lacks exact rule provenance: %+v", result)
		}
	} else if result.Reason != "" || len(result.PanicReasons) != 0 {
		t.Fatalf("safe request inherited another request's panic: %+v", result)
	}
}

func TestDreamerProjectionContract(t *testing.T) {
	for _, mode := range []string{"typed_match", "other_target", "other_action", "target_atom", "action_string", "injection", "canceled", "recovery", "concurrent"} {
		t.Run(mode, func(t *testing.T) {
			action, target := "/edit_file", fmt.Sprintf("%q", projectionGuardedTarget)
			if mode == "target_atom" {
				target = projectionGuardedTarget // Deliberately wrong kind: /name, not /string.
			}
			if mode == "action_string" {
				action = `"edit_file"` // Deliberately wrong kind: /string, not /name.
			}
			k, dreamer := projectionContractKernel(t, action, target)
			before := projectionParentFacts(t, k)
			req := core.ActionRequest{Type: core.ActionEditFile, Target: projectionGuardedTarget}
			ctx := context.Background()
			switch mode {
			case "canceled":
				stopped, cancel := context.WithCancel(ctx)
				cancel()
				result := dreamer.SimulateAction(stopped, req)
				if !result.Unsafe || !strings.Contains(result.Reason, context.Canceled.Error()) || len(result.ProjectedFacts) != 0 || !reflect.DeepEqual(result.Request, req) {
					t.Fatalf("canceled owner did not fail closed before projection: %+v", result)
				}
			case "concurrent":
				const workers = 16
				start := make(chan struct{})
				type observation struct {
					request core.ActionRequest
					result  core.DreamResult
				}
				results := make(chan observation, workers)
				var wg sync.WaitGroup
				for i := 0; i < workers; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-start
						current := req
						if i%2 != 0 {
							current.Target = projectionSafeTarget
						}
						results <- observation{current, dreamer.SimulateAction(ctx, current)}
					}(i)
				}
				close(start)
				wg.Wait()
				close(results)
				seen := make(map[string]bool)
				unsafeCount := 0
				for current := range results {
					unsafe := current.request.Target == projectionGuardedTarget
					assertProjectedRequest(t, current.request, current.result, unsafe)
					if seen[current.result.ActionID] {
						t.Fatalf("two simulations shared an action ID: %s", current.result.ActionID)
					}
					seen[current.result.ActionID] = true
					if current.result.Unsafe {
						unsafeCount++
					}
				}
				if len(seen) != workers || unsafeCount != workers/2 {
					t.Fatalf("opposite concurrent decisions not observed: total=%d unsafe=%d", len(seen), unsafeCount)
				}
			default:
				if mode == "other_target" {
					req.Target = projectionSafeTarget
				}
				if mode == "other_action" {
					req.Type = core.ActionReadFile
				}
				if mode == "injection" {
					req.Target = `"/fixture/guarded"), panic_state(Id, "injected").`
				}
				unsafe := mode == "typed_match" || mode == "recovery"
				first := dreamer.SimulateAction(ctx, req)
				assertProjectedRequest(t, req, first, unsafe)
				if mode == "recovery" {
					req.Target = projectionSafeTarget
					second := dreamer.SimulateAction(ctx, req)
					assertProjectedRequest(t, req, second, false)
					if first.ActionID == second.ActionID {
						t.Fatal("recovery reused the blocked action identity")
					}
				}
			}
			if after := projectionParentFacts(t, k); !reflect.DeepEqual(before, after) {
				t.Fatalf("sandbox facts or verdicts escaped to the parent: before=%v after=%v", before, after)
			}
		})
	}
}
