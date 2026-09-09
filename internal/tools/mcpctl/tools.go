package mcpctl

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/mcp"
	"codenerd/internal/tools"
)

// The five verbs below are a ladder, and each tool's Description says so
// explicitly. That is deliberate: progressive disclosure only saves anything if
// the model actually starts at the cheap rung, and the only place to teach that
// is the description it reads before choosing.
//
// Every executor validates its own arguments BEFORE looking up the control
// plane. The order is load-bearing rather than stylistic: a misspelled view or
// an unknown facet is wrong whether or not a server happens to be configured,
// and checking the plane first meant a workspace with no MCP servers answered
// every malformed call with a cheerful "nothing is configured" and never
// mentioned the typo.

// MapTool returns the atlas verb.
func MapTool() *tools.Tool {
	return &tools.Tool{
		Name:        "mcp_map",
		Effect:      tools.EffectRead,
		Description: `Map every connected MCP server: connection status, tool counts, and which facets (read, search, analyze, write, execute, manage) each server fills. Returns counts and a few sample tool names, NOT tool schemas, so it stays cheap no matter how many tools are connected. Start here, then mcp_probe to see signatures for one facet, then mcp_call to invoke.`,
		Category:    tools.CategoryResearch,
		Priority:    60,
		Execute:     executeMap,
		Schema: tools.ToolSchema{
			Properties: map[string]tools.Property{
				"server": {Type: "string", Description: "Limit the map to one server id"},
				"view":   {Type: "string", Description: "Disclosure depth", Default: "compact", Enum: []any{"summary", "compact", "full"}},
			},
		},
	}
}

func executeMap(ctx context.Context, args map[string]any) (string, error) {
	view, err := resolveView(args)
	if err != nil {
		return "", err
	}
	plane, err := getControlPlane()
	if err != nil {
		return failure(err.Error(), "configure integrations.servers in .nerd/config.json")
	}
	return marshalResult(plane.Atlas(ctx, mcp.AtlasOptions{
		Server: strings.TrimSpace(stringArg(args, "server")),
		View:   view,
	}))
}

// ProbeTool returns the zoom verb.
func ProbeTool() *tools.Tool {
	return &tools.Tool{
		Name:        "mcp_probe",
		Effect:      tools.EffectRead,
		Description: `Open one slice of the MCP tool catalog. Without 'tool', returns bounded one-line signatures — name(required: type[, optional]) — for tools matching server/facet/query, ranked by proven success rate with safer tools first. With 'tool', returns that one tool's full JSON schema and description; that is the only call that spends full-schema tokens, so use it on the tool you are about to invoke. Risk classes (safe, mutating, destructive, arbitrary) are derived from the server's own annotations where it declares them.`,
		Category:    tools.CategoryResearch,
		Priority:    62,
		Execute:     executeProbe,
		Schema: tools.ToolSchema{
			Properties: map[string]tools.Property{
				"server":    {Type: "string", Description: "Limit to one server id"},
				"facet":     {Type: "string", Description: "Limit to one verb facet", Enum: []any{"read", "search", "analyze", "write", "execute", "manage"}},
				"tool":      {Type: "string", Description: "Full server/tool id or a unique bare name; returns that tool's full schema"},
				"query":     {Type: "string", Description: "Free-text terms matched against name, purpose, description and use cases"},
				"max_risk":  {Type: "string", Description: "Hide tools riskier than this", Enum: []any{"safe", "mutating", "destructive", "arbitrary"}},
				"view":      {Type: "string", Description: "Disclosure depth", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items": {Type: "integer", Description: "Maximum signatures returned; hard-capped at 60", Default: 12},
			},
		},
	}
}

func executeProbe(ctx context.Context, args map[string]any) (string, error) {
	view, err := resolveView(args)
	if err != nil {
		return "", err
	}
	facet := mcp.Facet(strings.ToLower(strings.TrimSpace(stringArg(args, "facet"))))
	if facet != "" && !facet.Valid() {
		return "", fmt.Errorf("mcp_probe: unsupported facet %q", facet)
	}
	maxRisk := mcp.RiskClass(strings.ToLower(strings.TrimSpace(stringArg(args, "max_risk"))))
	if maxRisk != "" && !maxRisk.Valid() {
		return "", fmt.Errorf("mcp_probe: unsupported max_risk %q", maxRisk)
	}
	plane, err := getControlPlane()
	if err != nil {
		return failure(err.Error(), "configure integrations.servers in .nerd/config.json")
	}

	return marshalResult(plane.Probe(ctx, mcp.ProbeOptions{
		Server:   strings.TrimSpace(stringArg(args, "server")),
		Facet:    facet,
		Tool:     strings.TrimSpace(stringArg(args, "tool")),
		Query:    strings.TrimSpace(stringArg(args, "query")),
		MaxRisk:  maxRisk,
		View:     view,
		MaxItems: intArg(args, "max_items", 0),
	}))
}

// CallTool returns the invocation verb.
func CallTool() *tools.Tool {
	return &tools.Tool{
		Name:        "mcp_call",
		Effect:      tools.EffectExternal,
		Description: `Invoke one MCP tool. Arguments are checked against the tool's declared schema before dispatch; if they do not fit, nothing is sent and the schema comes back with the error so the next attempt is informed. Results are shaped to 'view' and reported with their structural shape, so a large payload costs a sketch rather than the payload. Anything withheld is retained under a handle — pass it to mcp_expand to read the rest WITHOUT re-running the call. Tools policy classifies as destructive or arbitrary-execution require confirm_risk=true.`,
		Category:    tools.CategoryResearch,
		Priority:    64,
		Execute:     executeCall,
		Schema: tools.ToolSchema{
			Required: []string{"tool"},
			Properties: map[string]tools.Property{
				"tool":         {Type: "string", Description: "Full server/tool id, or a bare name unique across servers"},
				"server":       {Type: "string", Description: "Disambiguate a bare tool name"},
				"args":         {Type: "object", Description: "Arguments for the MCP tool, per its schema"},
				"view":         {Type: "string", Description: "Result disclosure depth", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items":    {Type: "integer", Description: "Narrow list results below the view's own ceiling"},
				"confirm_risk": {Type: "boolean", Description: "Acknowledge a destructive or arbitrary-execution effect", Default: false},
			},
		},
	}
}

func executeCall(ctx context.Context, args map[string]any) (string, error) {
	view, err := resolveView(args)
	if err != nil {
		return "", err
	}
	tool := strings.TrimSpace(stringArg(args, "tool"))
	if tool == "" {
		return "", fmt.Errorf("mcp_call: tool is required")
	}
	callArgs, err := mapArg(args["args"])
	if err != nil {
		return "", fmt.Errorf("mcp_call: %w", err)
	}
	plane, err := getControlPlane()
	if err != nil {
		return failure(err.Error(), "configure integrations.servers in .nerd/config.json")
	}

	return marshalResult(plane.Call(ctx, mcp.CallOptions{
		Tool:        tool,
		Server:      strings.TrimSpace(stringArg(args, "server")),
		Args:        callArgs,
		View:        view,
		MaxItems:    intArg(args, "max_items", 0),
		ConfirmRisk: boolArg(args, "confirm_risk", false),
	}))
}

// ExpandTool returns the handle-redemption verb.
func ExpandTool() *tools.Tool {
	return &tools.Tool{
		Name:        "mcp_expand",
		Effect:      tools.EffectRead,
		Description: `Reopen part of a result mcp_call already fetched, using the handle it returned. This never re-invokes the server, so it is safe on a handle from a call that had side effects. Give a JSON pointer (for example /items/12 or /results) to walk into one slice; omit it for the whole payload at the requested depth. Handles expire, and an expired one means re-running the original call.`,
		Category:    tools.CategoryResearch,
		Priority:    63,
		Execute:     executeExpand,
		Schema: tools.ToolSchema{
			Required: []string{"handle"},
			Properties: map[string]tools.Property{
				"handle":    {Type: "string", Description: "Handle returned by a previous mcp_call or mcp_context"},
				"pointer":   {Type: "string", Description: "RFC 6901 JSON pointer into the retained payload, e.g. /items/3"},
				"view":      {Type: "string", Description: "Disclosure depth", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items": {Type: "integer", Description: "Narrow list results below the view's own ceiling"},
			},
		},
	}
}

func executeExpand(ctx context.Context, args map[string]any) (string, error) {
	view, err := resolveView(args)
	if err != nil {
		return "", err
	}
	handle := strings.TrimSpace(stringArg(args, "handle"))
	if handle == "" {
		return "", fmt.Errorf("mcp_expand: handle is required")
	}
	plane, err := getControlPlane()
	if err != nil {
		return failure(err.Error(), "configure integrations.servers in .nerd/config.json")
	}
	return marshalResult(plane.Expand(ctx, mcp.ExpandOptions{
		Handle:   handle,
		Pointer:  strings.TrimSpace(stringArg(args, "pointer")),
		View:     view,
		MaxItems: intArg(args, "max_items", 0),
	}))
}

// ContextTool returns the JIT context verb.
func ContextTool() *tools.Tool {
	return &tools.Tool{
		Name:        "mcp_context",
		Effect:      tools.EffectExternal,
		Description: `Retrieve the non-tool half of connected MCP servers: their published resources (documents, schemas, examples) and prompt templates, ranked against a query. Many servers publish the very context that would otherwise have to be inferred from tool descriptions — a query dialect, a field reference, a worked example. Naming is cheap; pass read=true to fetch the text of the top few, which is bounded and handle-backed like any other result.`,
		Category:    tools.CategoryResearch,
		Priority:    61,
		Execute:     executeContext,
		Schema: tools.ToolSchema{
			Properties: map[string]tools.Property{
				"query":     {Type: "string", Description: "Terms matched against resource uri, name and description"},
				"server":    {Type: "string", Description: "Limit to one server id"},
				"read":      {Type: "boolean", Description: "Fetch and excerpt the top-ranked resources", Default: false},
				"view":      {Type: "string", Description: "Disclosure depth; also bounds excerpt width", Default: "compact", Enum: []any{"summary", "compact", "full"}},
				"max_items": {Type: "integer", Description: "Maximum resources and prompts returned; hard-capped at 25", Default: 6},
			},
		},
	}
}

func executeContext(ctx context.Context, args map[string]any) (string, error) {
	view, err := resolveView(args)
	if err != nil {
		return "", err
	}
	plane, err := getControlPlane()
	if err != nil {
		return failure(err.Error(), "configure integrations.servers in .nerd/config.json")
	}
	return marshalResult(plane.Context(ctx, mcp.ContextOptions{
		Query:    strings.TrimSpace(stringArg(args, "query")),
		Server:   strings.TrimSpace(stringArg(args, "server")),
		View:     view,
		MaxItems: intArg(args, "max_items", 0),
		Read:     boolArg(args, "read", false),
	}))
}
