package main

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// filePendingAction files the constitutional permission request for a
// CLI-routed next_action fact. constitution.mg (default-deny) derives
// permitted/3 only from safe_action + a matching pending_action/5 whose
// Action, Target and Payload columns exactly equal what
// VirtualStore.CheckKernelPermitted recomputes from the parsed request.
// The payload here mirrors parseActionFact: args beyond the third merge
// maps / become argN keys, and json.Marshal of the (possibly empty) map is
// the canonical form ("{}" for bare 3-arg facts).
//
// The returned retract func removes the request after routing so stale
// pending_action facts don't accumulate (the executive shard and session
// executor both retract the same way).
func filePendingAction(kernel core.Kernel, fact core.Fact) (func(), error) {
	if kernel == nil {
		return nil, fmt.Errorf("cannot file pending_action: no kernel")
	}
	if len(fact.Args) < 3 {
		return nil, fmt.Errorf("cannot file pending_action: want >=3-arg next_action fact (ActionID, Type, Target), got %d args", len(fact.Args))
	}

	actionType := types.ExtractString(fact.Args[1])
	if !strings.HasPrefix(actionType, "/") {
		actionType = "/" + actionType
	}

	payload := map[string]any{}
	for i := 3; i < len(fact.Args); i++ {
		if m, ok := fact.Args[i].(map[string]any); ok {
			maps.Copy(payload, m)
			continue
		}
		payload[fmt.Sprintf("arg%d", i-3)] = fact.Args[i]
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("cannot file pending_action: marshal payload: %w", err)
	}

	pending := core.Fact{
		Predicate: "pending_action",
		Args: []any{
			types.ExtractString(fact.Args[0]),
			core.MangleAtom(actionType),
			types.ExtractString(fact.Args[2]),
			string(payloadJSON),
			time.Now().Unix(),
		},
	}
	if err := kernel.Assert(pending); err != nil {
		return nil, fmt.Errorf("assert pending_action: %w", err)
	}
	return func() {
		_ = kernel.RetractExactFactsBatch([]core.Fact{pending})
	}, nil
}

// routePermittedAction files the pending_action for fact, routes it through
// the VirtualStore, and retracts the permission request afterwards. Every
// standalone CLI path that calls RouteAction directly (no executive shard to
// file permissions on its behalf) must go through this — a bare RouteAction
// is default-denied by the constitutional gate.
func routePermittedAction(ctx context.Context, vs *core.VirtualStore, kernel core.Kernel, fact core.Fact) (string, error) {
	retract, err := filePendingAction(kernel, fact)
	if err != nil {
		return "", err
	}
	defer retract()
	return vs.RouteAction(ctx, fact)
}
