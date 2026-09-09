// Package mcpctl exposes connected MCP servers to the model as a fixed,
// progressively-disclosed control plane rather than as a pile of tool schemas.
//
// The size of this package's tool surface is its whole point. Five verbs are
// registered, and five is what gets registered whether one MCP server is
// connected or twenty. Everything that varies — which servers exist, which
// facets they fill, what a given tool's arguments are, how big its results run
// — is discovered at runtime and disclosed only to the turn that asks for it.
//
// The alternative, rendering every discovered tool's schema into the prompt, is
// what made MCP tooling too expensive to wire in at all: it charges for the
// whole catalog on every turn to serve the one call an agent makes on some of
// them.
package mcpctl

import (
	"encoding/json"
	"fmt"
	"sync"

	"codenerd/internal/mcp"
)

// controlPlane is the process-wide plane the tools operate against. It mirrors
// how the browser tools bind to a Cortex-owned session manager: the runtime
// owns the object, the tool package holds a reference.
var (
	plane   *mcp.ControlPlane
	planeMu sync.RWMutex
)

// SetControlPlane binds the tools to the runtime's MCP control plane. Passing
// nil unbinds, which makes every tool report that MCP is not configured rather
// than panicking on a nil dereference.
func SetControlPlane(cp *mcp.ControlPlane) {
	planeMu.Lock()
	defer planeMu.Unlock()
	plane = cp
}

func getControlPlane() (*mcp.ControlPlane, error) {
	planeMu.RLock()
	defer planeMu.RUnlock()
	if plane == nil {
		return nil, fmt.Errorf("no MCP servers are configured for this workspace")
	}
	return plane, nil
}

// marshalResult renders a control-plane envelope as JSON.
//
// JSON rather than markdown, matching the browser progressive tools: the model
// consumes these results structurally, and a rendered heading costs tokens that
// buy nothing a key does not.
func marshalResult(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal MCP control-plane result: %w", err)
	}
	return string(data), nil
}

// failure renders a refusal in the same envelope shape as a success, so a
// caller never has to branch on whether it got an object or an error string.
func failure(summary, nextStep string) (string, error) {
	return marshalResult(map[string]any{
		"success":   false,
		"summary":   summary,
		"next_step": nextStep,
	})
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return value
}

func boolArg(args map[string]any, key string, fallback bool) bool {
	value, ok := args[key].(bool)
	if !ok {
		return fallback
	}
	return value
}

func intArg(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

// mapArg coerces a nested arguments object.
//
// It round-trips through JSON rather than type-asserting because a provider may
// hand the payload over as any JSON-decodable shape, and rejecting a
// well-formed call over the Go type it arrived in would be an implementation
// detail leaking into the agent's experience.
func mapArg(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	if direct, ok := value.(map[string]any); ok {
		return direct, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("args must be a JSON object: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("args must be a JSON object: %w", err)
	}
	return out, nil
}

// resolveView validates the disclosure depth.
func resolveView(args map[string]any) (mcp.View, error) {
	return mcp.NormalizeView(stringArg(args, "view"))
}
