package campaign

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// executeToolCreateTask asks autopoiesis for a tool through the kernel: it
// asserts missing_tool_for and goal_requires, the policy derives
// delegate_task(/tool_generator, Cap, /pending), and the autopoiesis
// orchestrator generates the tool and registers it.
//
// The kernel decides first whether the capability is generated at all
// (tool_generation_block_reason, policy/autopoiesis.mg). A capability a
// registered tool already provides resolves at once to that tool, and a
// capability that must never be generated fails at once with the reason. The
// task used to poll for a registration that a blocked capability can never
// produce, for 30 minutes, and then report the timeout as success.
func (o *Orchestrator) executeToolCreateTask(ctx context.Context, task *Task) (any, error) {
	logging.Campaign("Executing tool create task %s (Ouroboros)", task.ID)
	// For tool creation, the artifact path names the capability; the
	// description stands in when there is none.
	capability := strings.TrimSpace(task.Description)
	if len(task.Artifacts) > 0 && strings.TrimSpace(task.Artifacts[0].Path) != "" {
		capability = strings.TrimSpace(task.Artifacts[0].Path)
	}
	capAtom, named := capabilityAtom(capability)
	var capArg any = capability
	if named {
		capArg = capAtom
	}
	intentID := types.MangleAtom(mangleNameConst(fmt.Sprintf("campaign_%s_task_%s", o.campaign.ID, task.ID)))
	logging.CampaignDebug("Tool capability requested: %s (intent %s)", capability, intentID)

	if err := o.kernel.Assert(core.Fact{Predicate: "missing_tool_for", Args: []any{intentID, capArg}}); err != nil {
		return nil, fmt.Errorf("failed to assert missing_tool_for: %w", err)
	}
	if err := o.kernel.Assert(core.Fact{Predicate: "goal_requires", Args: []any{o.campaign.Goal, capArg}}); err != nil {
		return nil, fmt.Errorf("failed to assert goal_requires: %w", err)
	}

	if named {
		if result, decided, err := o.toolGenerationDecision(capability, capAtom); decided {
			return result, err
		}
	}

	o.emitEvent(EventToolGenerationRequested, "", task.ID, capability, map[string]any{
		"intent_id":  intentID,
		"capability": capability,
	})
	return o.awaitGeneratedTool(ctx, capability, capAtom, named)
}

// capabilityAtom is the name constant a capability carries in the kernel
// ("/go_vet"), when the text is one: lower-case letters, digits and
// underscores. Prose names no capability the policy can reason about.
func capabilityAtom(text string) (types.MangleAtom, bool) {
	name := strings.TrimPrefix(strings.TrimSpace(text), "/")
	if name == "" {
		return "", false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return "", false
		}
	}
	return types.MangleAtom("/" + name), true
}

// toolGenerationDecision reads why the kernel will not generate the
// capability, if it will not. decided is false when generation goes ahead.
func (o *Orchestrator) toolGenerationDecision(capability string, capAtom types.MangleAtom) (result any, decided bool, err error) {
	rows, qerr := o.kernel.Query("tool_generation_block_reason")
	if qerr != nil {
		return nil, true, fmt.Errorf("read tool_generation_block_reason for %s: %w", capability, qerr)
	}
	reason := ""
	for _, row := range rows {
		if len(row.Args) > 1 && types.ExtractString(row.Args[0]) == string(capAtom) {
			if r := types.ExtractString(row.Args[1]); reason == "" || r == "/dangerous" {
				reason = r
			}
		}
	}
	switch reason {
	case "":
		return nil, false, nil
	case "/provided":
		provider, perr := o.capabilityProvider(capAtom)
		if perr != nil {
			return nil, true, perr
		}
		logging.Campaign("Tool create task for %s resolved: registered tool %s provides it", capability, provider)
		return map[string]any{
			"status":     "complete",
			"capability": capability,
			"tool_name":  provider,
			"message":    fmt.Sprintf("registered tool %s already provides %s; nothing was generated", provider, capability),
		}, true, nil
	default:
		return nil, true, fmt.Errorf("tool generation for %s is blocked by policy (%s): it is never generated", capability, strings.TrimPrefix(reason, "/"))
	}
}

// capabilityProvider names the registered tool that provides capAtom: a
// built-in's tool_capabilities row or a generated tool's tool_capability.
func (o *Orchestrator) capabilityProvider(capAtom types.MangleAtom) (string, error) {
	for _, predicate := range []string{"tool_capabilities", "tool_capability"} {
		rows, err := o.kernel.Query(predicate)
		if err != nil {
			return "", fmt.Errorf("read %s for %s: %w", predicate, capAtom, err)
		}
		for _, row := range rows {
			if len(row.Args) > 1 && types.ExtractString(row.Args[1]) == string(capAtom) {
				return strings.TrimPrefix(types.ExtractString(row.Args[0]), "/"), nil
			}
		}
	}
	return "", fmt.Errorf("the kernel says a registered tool provides %s, and no tool_capabilities or tool_capability row names it", capAtom)
}

// awaitGeneratedTool waits for autopoiesis to register the tool, to fail it
// (tool_generation_failed), or for the context to end. A generation that does
// not finish inside the window fails the task: a tool that does not exist is
// not a completed tool-create task.
func (o *Orchestrator) awaitGeneratedTool(ctx context.Context, capability string, capAtom types.MangleAtom, named bool) (any, error) {
	timeout := time.After(30 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	// A query that errors every tick can never succeed; bail out rather than
	// burning the whole window on it.
	const maxConsecutiveQueryErrors = 3
	queryErrors := 0
	matches := func(arg any) bool {
		s := types.ExtractString(arg)
		return s == capability || (named && (s == string(capAtom) || s == strings.TrimPrefix(string(capAtom), "/")))
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("tool generation for %s did not finish within 30 minutes; no tool was registered", capability)
		case <-ticker.C:
			var failed string
			registered, provided := false, false
			var qerr error
			if rows, err := o.kernel.Query("tool_registered"); err != nil {
				qerr = err
			} else {
				for _, row := range rows {
					registered = registered || (len(row.Args) > 0 && matches(row.Args[0]))
				}
			}
			if rows, err := o.kernel.Query("tool_generation_failed"); err != nil {
				qerr = err
			} else {
				for _, row := range rows {
					if len(row.Args) > 1 && matches(row.Args[0]) {
						failed = types.ExtractString(row.Args[1])
					}
				}
			}
			if named {
				if rows, err := o.kernel.Query("has_capability"); err != nil {
					qerr = err
				} else {
					for _, row := range rows {
						provided = provided || (len(row.Args) == 1 && types.ExtractString(row.Args[0]) == string(capAtom))
					}
				}
			}
			switch {
			case registered || provided:
				return map[string]any{"status": "complete", "capability": capability, "tool_name": strings.TrimPrefix(capability, "/")}, nil
			case failed != "":
				return nil, fmt.Errorf("tool generation for %s failed: %s", capability, failed)
			case qerr != nil:
				queryErrors++
				logging.CampaignWarn("tool_create poll for %q failed (%d/%d): %v", capability, queryErrors, maxConsecutiveQueryErrors, qerr)
				if queryErrors >= maxConsecutiveQueryErrors {
					return nil, fmt.Errorf("tool registration poll for %q failed %d consecutive times: %w", capability, queryErrors, qerr)
				}
			default:
				queryErrors = 0
			}
		}
	}
}
