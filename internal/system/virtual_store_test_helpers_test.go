package system_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

func routePermittedAction(
	t *testing.T,
	ctx context.Context,
	virtualStore *core.VirtualStore,
	kernel *core.RealKernel,
	action core.Fact,
) (string, error) {
	t.Helper()
	if len(action.Args) < 3 {
		t.Fatalf("next_action requires action ID, type, and target: %v", action.Args)
	}

	actionType := fmt.Sprintf("%v", action.Args[1])
	if !strings.HasPrefix(actionType, "/") {
		actionType = "/" + actionType
	}
	payload := map[string]any{}
	if len(action.Args) > 3 && action.Args[3] != nil {
		var ok bool
		payload, ok = action.Args[3].(map[string]any)
		if !ok {
			t.Fatalf("next_action payload must be map[string]any, got %T", action.Args[3])
		}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal permission payload: %v", err)
	}

	pending := core.Fact{
		Predicate: "pending_action",
		Args: []any{
			fmt.Sprintf("%v", action.Args[0]),
			core.MangleAtom(actionType),
			fmt.Sprintf("%v", action.Args[2]),
			string(payloadJSON),
			time.Now().Unix(),
		},
	}
	if err := kernel.Assert(pending); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}
	defer func() {
		if err := kernel.RetractExactFact(pending); err != nil {
			t.Errorf("retract pending_action: %v", err)
		}
	}()

	return virtualStore.RouteAction(ctx, action)
}
