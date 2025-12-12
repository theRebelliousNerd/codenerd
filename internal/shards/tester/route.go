package tester

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/core"
)

// assertNextActionAndWait routes an unsafe action through the kernel OODA pipeline
// and waits for a routing_result with the same action_id.
func (t *TesterShard) assertNextActionAndWait(ctx context.Context, actionType, target string, payload map[string]interface{}) error {
	if t.kernel == nil {
		return fmt.Errorf("kernel not attached")
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	actionID := fmt.Sprintf("action-%d", time.Now().UnixNano())
	payload["action_id"] = actionID

	// Use the canonical executable envelope. next_action/1 is reserved for IDB decisions.
	pending := core.Fact{
		Predicate: "pending_action",
		Args:      []interface{}{actionID, actionType, target, payload, time.Now().Unix()},
	}

	if err := t.kernel.Assert(pending); err != nil {
		return err
	}

	_, err := t.waitForRoutingResult(ctx, actionID)
	return err
}

// waitForRoutingResult blocks until a routing_result for actionID appears.
// Returns the tool output on success.
func (t *TesterShard) waitForRoutingResult(ctx context.Context, actionID string) (string, error) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if t.kernel == nil {
				return "", fmt.Errorf("kernel not attached")
			}

			results, err := t.kernel.Query("routing_result")
			if err != nil {
				continue
			}

			for _, f := range results {
				if len(f.Args) < 2 {
					continue
				}
				id := fmt.Sprintf("%v", f.Args[0])
				if id != actionID {
					continue
				}
				status := fmt.Sprintf("%v", f.Args[1])
				switch status {
				case "success", "/success":
					if len(f.Args) > 2 {
						return fmt.Sprintf("%v", f.Args[2]), nil
					}
					return "", nil
				case "failure", "/failure":
					reason := ""
					if len(f.Args) > 2 {
						reason = fmt.Sprintf("%v", f.Args[2])
					}
					return "", fmt.Errorf("routing failed for %s: %s", actionID, reason)
				}
			}
		}
	}
}
