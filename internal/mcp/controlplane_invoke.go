package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// --- Call ------------------------------------------------------------------

// RiskGate decides whether a classified call may proceed.
//
// It exists so the kernel is the executive here as everywhere else. The Go side
// derives a risk class — mechanical work, from annotations and schema — and
// policy decides what that class is allowed to do. Which tools need
// confirmation is therefore a Mangle rule (mcp_tool_gated), not a constant in
// this file, and changing that judgement is a policy edit rather than a code
// change.
type RiskGate func(ctx context.Context, tool *MCPTool, confirmed bool) error

// SetRiskGate installs the policy hook.
func (cp *ControlPlane) SetRiskGate(gate RiskGate) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.gate = gate
}

func (cp *ControlPlane) riskGate() RiskGate {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return cp.gate
}

// CallOptions is one invocation.
type CallOptions struct {
	Tool   string
	Server string
	Args   map[string]any
	View   View
	// MaxItems narrows the result below the view's own ceiling. It can only
	// narrow: see DigestBudget.withCeiling.
	MaxItems int
	// ConfirmRisk is the caller's explicit acknowledgement that a destructive
	// or arbitrary-execution tool is meant to run.
	ConfirmRisk bool
}

// CallResult is a shaped invocation result.
type CallResult struct {
	Success   bool      `json:"success"`
	ToolID    string    `json:"tool_id"`
	Facet     Facet     `json:"facet,omitzero"`
	Risk      RiskClass `json:"risk,omitzero"`
	View      View      `json:"view"`
	Summary   string    `json:"summary"`
	Shape     string    `json:"shape,omitzero"`
	Data      any       `json:"data,omitzero"`
	Elided    []Elision `json:"elided,omitzero"`
	Handle    string    `json:"handle,omitzero"`
	Bytes     int       `json:"bytes,omitzero"`
	FullBytes int       `json:"full_bytes,omitzero"`
	Truncated bool      `json:"truncated,omitzero"`
	LatencyMs int64     `json:"latency_ms,omitzero"`
	Error     string    `json:"error,omitzero"`
	// Schema is populated only when the call was refused for bad arguments.
	// Returning it turns the failure into the disclosure the caller needed.
	Schema   string `json:"schema,omitzero"`
	NextStep string `json:"next_step,omitzero"`
}

// Call invokes one MCP tool and returns a shaped result.
//
// Three things happen before the call reaches the wire, in this order, and the
// order matters: arguments are checked (so a bad call costs nothing and answers
// with the schema), then risk is gated (so a destructive call is deliberate),
// then the tool runs. Checking risk before arguments would refuse a
// well-intentioned malformed call with a scary message about blast radius; the
// other way round, the agent fixes its arguments and meets the risk gate once,
// on a call it is actually ready to make.
func (cp *ControlPlane) Call(ctx context.Context, opts CallOptions) *CallResult {
	view := opts.View
	if view == "" {
		view = ViewCompact
	}

	tool, err := resolveTool(cp.tools(ctx), opts.Tool, opts.Server)
	if err != nil {
		return &CallResult{
			Success: false, ToolID: opts.Tool, View: view, Error: err.Error(),
			Summary:  "tool not resolved",
			NextStep: "mcp_map to list servers, mcp_probe(query=...) to find the tool",
		}
	}

	if err := ValidateArgs(tool, opts.Args); err != nil {
		result := &CallResult{
			Success: false, ToolID: tool.ToolID, Facet: tool.Facet, Risk: tool.Risk,
			View: view, Error: err.Error(), Summary: "arguments rejected before dispatch",
			NextStep: "correct the arguments against the schema below and call again",
		}
		var argErr *ArgumentError
		if errors.As(err, &argErr) {
			result.Schema = argErr.Schema
		}
		return result
	}

	if err := cp.checkRisk(ctx, tool, opts.ConfirmRisk); err != nil {
		return &CallResult{
			Success: false, ToolID: tool.ToolID, Facet: tool.Facet, Risk: tool.Risk,
			View: view, Error: err.Error(), Summary: "refused by risk gate",
			NextStep: "pass confirm_risk=true if this effect is intended, or choose a safer tool",
		}
	}

	started := time.Now()
	raw, err := cp.manager.CallTool(ctx, tool.ToolID, opts.Args)
	latency := time.Since(started).Milliseconds()

	switch {
	case err != nil:
		return &CallResult{
			Success: false, ToolID: tool.ToolID, Facet: tool.Facet, Risk: tool.Risk,
			View: view, LatencyMs: latency, Error: err.Error(),
			Summary: "transport error",
		}
	case raw == nil:
		return &CallResult{
			Success: false, ToolID: tool.ToolID, Facet: tool.Facet, Risk: tool.Risk,
			View: view, LatencyMs: latency, Error: "server returned no result",
			Summary: "empty result",
		}
	case !raw.Success:
		return &CallResult{
			Success: false, ToolID: tool.ToolID, Facet: tool.Facet, Risk: tool.Risk,
			View: view, LatencyMs: latency, Error: raw.Error,
			Summary: "tool reported failure",
		}
	}

	budget := BudgetFor(view).withCeiling(opts.MaxItems)
	digest := DigestJSON(raw.Output, view, budget)

	result := &CallResult{
		Success: true, ToolID: tool.ToolID, Facet: tool.Facet, Risk: tool.Risk,
		View: view, Shape: digest.Shape, Data: digest.Data, Elided: digest.Elided,
		Bytes: digest.Bytes, FullBytes: digest.FullBytes, Truncated: digest.Truncated,
		LatencyMs: latency,
	}

	// A handle is minted only when something was actually withheld. Handing
	// back a redemption ticket for a result that is already complete invites a
	// pointless second call, and every such call is a turn the agent did not
	// spend on the task.
	if digest.Truncated {
		result.Handle = cp.handles.Mint(tool.ToolID, raw.Output)
		if result.Handle != "" {
			result.NextStep = fmt.Sprintf("mcp_expand(handle=%q, pointer=\"/...\") for the elided parts", result.Handle)
		}
		cp.emitHandleFact(result.Handle, tool.ToolID, digest.FullBytes)
	}
	result.Summary = callSummary(result)
	return result
}

func callSummary(r *CallResult) string {
	if !r.Truncated {
		return fmt.Sprintf("%s ok in %dms, %s", r.ToolID, r.LatencyMs, r.Shape)
	}
	saved := r.FullBytes - r.Bytes
	if saved < 0 {
		saved = 0
	}
	return fmt.Sprintf("%s ok in %dms, %s — shaped to %d of %d bytes (%d withheld)",
		r.ToolID, r.LatencyMs, r.Shape, r.Bytes, r.FullBytes, saved)
}

// checkRisk asks the executive, or falls back to a conservative constant.
//
// Exactly one authority decides, and the kernel is preferred: two gates whose
// verdicts can disagree is how a tool ends up blocked for a reason nobody can
// locate. The built-in rule below applies only when no kernel is wired, and it
// is deliberately the stricter of the two readings — a degraded control plane
// should refuse more than policy would, never less.
//
// Confirmation is not a security boundary; the agent can set the flag. It is a
// deliberateness boundary, and that is the failure it actually prevents: a tool
// with an innocuous name turns out to delete a branch, and nothing in the
// transcript shows that anyone decided it should.
func (cp *ControlPlane) checkRisk(ctx context.Context, tool *MCPTool, confirmed bool) error {
	if gate := cp.riskGate(); gate != nil {
		return gate(ctx, tool, confirmed)
	}
	switch tool.Risk {
	case RiskDestructive, RiskArbitrary:
		if !confirmed {
			return fmt.Errorf(
				"%s is classified %s (from %s) and needs confirm_risk=true: %s",
				tool.ToolID, tool.Risk, tool.RiskSource, riskExplanation(tool.Risk))
		}
	}
	return nil
}

// KernelRiskGate derives the confirmation requirement from policy.
//
// mcp_tool_gated is the rule; this only enforces its verdict. A kernel query
// that fails is treated as "gated": the executive could not be consulted, and
// dispatching an unclassified remote call because the check itself broke is the
// one outcome that must never happen quietly.
func KernelRiskGate(kernel KernelInterface) RiskGate {
	return func(_ context.Context, tool *MCPTool, confirmed bool) error {
		if kernel == nil || tool == nil {
			return nil
		}
		results, err := kernel.Query(fmt.Sprintf("mcp_tool_gated(%s)", mangleString(tool.ToolID)))
		if err != nil {
			logging.Get(logging.CategoryTools).Warn(
				"MCP risk gate: kernel query failed for %s, treating as gated: %v", tool.ToolID, err)
			if !confirmed {
				return fmt.Errorf(
					"%s could not be checked against policy (%v); pass confirm_risk=true to proceed anyway",
					tool.ToolID, err)
			}
			return nil
		}
		if len(results) == 0 || confirmed {
			return nil
		}
		return fmt.Errorf(
			"policy gates %s (risk %s, from %s) and needs confirm_risk=true: %s",
			tool.ToolID, tool.Risk, tool.RiskSource, riskExplanation(tool.Risk))
	}
}

func riskExplanation(r RiskClass) string {
	switch r {
	case RiskDestructive:
		return "it removes or overwrites state and there is no undo through this interface"
	case RiskArbitrary:
		return "it runs caller-supplied code, so its blast radius is whatever the server can reach"
	case RiskMutating:
		return "it changes state that outlives the call"
	default:
		return "its effect could not be determined from anything the server declared"
	}
}

// --- Expand ----------------------------------------------------------------

// ExpandOptions redeems a handle.
type ExpandOptions struct {
	Handle   string
	Pointer  string
	View     View
	MaxItems int
}

// ExpandResult is a reopened slice of a retained payload.
type ExpandResult struct {
	Success   bool      `json:"success"`
	Handle    string    `json:"handle"`
	Pointer   string    `json:"pointer,omitzero"`
	View      View      `json:"view"`
	Summary   string    `json:"summary"`
	Shape     string    `json:"shape,omitzero"`
	Data      any       `json:"data,omitzero"`
	Elided    []Elision `json:"elided,omitzero"`
	Bytes     int       `json:"bytes,omitzero"`
	Truncated bool      `json:"truncated,omitzero"`
	Error     string    `json:"error,omitzero"`
	NextStep  string    `json:"next_step,omitzero"`
}

// Expand reopens part of a previously shaped result.
//
// This never re-invokes the server. That is the whole reason the handle store
// retains bytes: the call behind a handle may have had side effects, and
// "show me the rest of what you already fetched" must not be able to fire them
// a second time.
func (cp *ControlPlane) Expand(_ context.Context, opts ExpandOptions) *ExpandResult {
	view := opts.View
	if view == "" {
		view = ViewCompact
	}
	budget := BudgetFor(view).withCeiling(opts.MaxItems)

	digest, err := cp.handles.Expand(opts.Handle, opts.Pointer, view, budget)
	if err != nil {
		result := &ExpandResult{
			Success: false, Handle: opts.Handle, Pointer: opts.Pointer,
			View: view, Error: err.Error(), Summary: "handle not expanded",
		}
		if errors.Is(err, ErrHandleNotFound) {
			result.NextStep = "the payload has expired; re-run the original mcp_call"
		}
		return result
	}

	result := &ExpandResult{
		Success: true, Handle: opts.Handle, Pointer: opts.Pointer, View: view,
		Shape: digest.Shape, Data: digest.Data, Elided: digest.Elided,
		Bytes: digest.Bytes, Truncated: digest.Truncated,
	}
	result.Summary = fmt.Sprintf("expanded %s%s: %s", opts.Handle, pointerSuffix(opts.Pointer), digest.Shape)
	if digest.Truncated {
		result.NextStep = "narrow with a deeper pointer, or raise view to full"
	}
	return result
}

func pointerSuffix(pointer string) string {
	if strings.TrimSpace(pointer) == "" {
		return ""
	}
	return "#" + pointer
}

// --- Context ---------------------------------------------------------------

// ContextOptions selects just-in-time context from server resources and
// prompts.
type ContextOptions struct {
	Query    string
	Server   string
	View     View
	MaxItems int
	// Read fetches the contents of the top-ranked resources rather than only
	// naming them. It is off by default because naming is cheap and reading is
	// not, and the agent is the one who knows whether it needs the text.
	Read bool
}

// ContextResource is one ranked resource.
type ContextResource struct {
	ServerID    string `json:"server_id"`
	URI         string `json:"uri"`
	Name        string `json:"name,omitzero"`
	Description string `json:"description,omitzero"`
	MimeType    string `json:"mime_type,omitzero"`
	Score       int    `json:"score,omitzero"`
	Excerpt     string `json:"excerpt,omitzero"`
	Handle      string `json:"handle,omitzero"`
	Bytes       int    `json:"bytes,omitzero"`
	Error       string `json:"error,omitzero"`
}

// ContextPrompt is one server-side prompt template.
type ContextPrompt struct {
	ServerID    string   `json:"server_id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitzero"`
	Required    []string `json:"required,omitzero"`
	Optional    []string `json:"optional,omitzero"`
	Score       int      `json:"score,omitzero"`
}

// ContextResult is the JIT context surface.
type ContextResult struct {
	Success   bool              `json:"success"`
	View      View              `json:"view"`
	Summary   string            `json:"summary"`
	Resources []ContextResource `json:"resources,omitzero"`
	Prompts   []ContextPrompt   `json:"prompts,omitzero"`
	Matched   int               `json:"matched"`
	Truncated bool              `json:"truncated,omitzero"`
	Notes     []string          `json:"notes,omitzero"`
	NextStep  string            `json:"next_step,omitzero"`
}

const (
	contextDefaultMaxItems = 6
	contextHardCap         = 25
	// contextReadTop bounds how many resources are actually fetched when
	// Read is set. Reading is a round trip per resource; ranking exists so that
	// the two or three that matter are the ones paid for.
	contextReadTop = 3
)

// Context retrieves the non-tool half of an MCP server: its resources and
// prompts, ranked against what the agent is doing.
//
// This is the half that usually goes unused. A server that publishes its own
// schema documentation, its query dialect, or a worked example as a resource
// has already written the context the agent would otherwise have to infer from
// tool descriptions — and inferring it costs both tokens and mistakes. Ranking
// and bounding is what makes reading it affordable enough to be routine.
func (cp *ControlPlane) Context(ctx context.Context, opts ContextOptions) *ContextResult {
	view := opts.View
	if view == "" {
		view = ViewCompact
	}
	maxItems := opts.MaxItems
	if maxItems <= 0 {
		maxItems = contextDefaultMaxItems
	}
	if maxItems > contextHardCap {
		maxItems = contextHardCap
	}

	servers := cp.contextServers(ctx, opts.Server)
	terms := queryTerms(opts.Query)

	result := &ContextResult{Success: true, View: view}
	var ranked []ContextResource

	for _, serverID := range servers {
		cat := cp.catalogFor(ctx, serverID)
		if cat.err != "" && len(cat.resources) == 0 && len(cat.prompts) == 0 {
			result.Notes = append(result.Notes,
				fmt.Sprintf("%s: no resources or prompts (%s)", serverID, cat.err))
			continue
		}
		for _, r := range cat.resources {
			ranked = append(ranked, ContextResource{
				ServerID: serverID, URI: r.URI, Name: r.Name,
				Description: r.Description, MimeType: r.MimeType,
				Score: scoreText(terms, r.URI, r.Name, r.Description),
			})
		}
		for _, p := range cat.prompts {
			entry := ContextPrompt{
				ServerID: serverID, Name: p.Name, Description: p.Description,
				Score: scoreText(terms, p.Name, p.Description),
			}
			for _, arg := range p.Arguments {
				if arg.Required {
					entry.Required = append(entry.Required, arg.Name)
				} else {
					entry.Optional = append(entry.Optional, arg.Name)
				}
			}
			result.Prompts = append(result.Prompts, entry)
		}
	}

	// With a query, anything that matched nothing is noise. Without one, the
	// caller is asking "what is there?" and dropping zero-scored entries would
	// answer "nothing".
	if len(terms) > 0 {
		ranked = filterScored(ranked)
		result.Prompts = filterScoredPrompts(result.Prompts)
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score
		}
		return ranked[i].URI < ranked[j].URI
	})
	sort.SliceStable(result.Prompts, func(i, j int) bool {
		if result.Prompts[i].Score != result.Prompts[j].Score {
			return result.Prompts[i].Score > result.Prompts[j].Score
		}
		return result.Prompts[i].Name < result.Prompts[j].Name
	})

	result.Matched = len(ranked)
	if len(ranked) > maxItems {
		result.Truncated = true
		ranked = ranked[:maxItems]
	}
	if len(result.Prompts) > maxItems {
		result.Truncated = true
		result.Prompts = result.Prompts[:maxItems]
	}

	if opts.Read {
		cp.readTopResources(ctx, ranked, view)
	}
	result.Resources = ranked

	result.Summary = fmt.Sprintf("%d resource(s) and %d prompt(s) across %d server(s)",
		result.Matched, len(result.Prompts), len(servers))
	if !opts.Read && len(ranked) > 0 {
		result.NextStep = "mcp_context(read=true) to fetch the top resources' text"
	}
	return result
}

// contextServers picks which servers to consult.
func (cp *ControlPlane) contextServers(ctx context.Context, only string) []string {
	if only != "" {
		return []string{only}
	}
	statuses := cp.serverStatuses(ctx)
	ids := make([]string, 0, len(statuses))
	for id, status := range statuses {
		// Only connected servers: listing resources on a dead server costs a
		// timeout per call and returns nothing.
		if status == ServerStatusConnected {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// readTopResources fetches and excerpts the highest-ranked resources in place.
func (cp *ControlPlane) readTopResources(ctx context.Context, resources []ContextResource, view View) {
	budget := BudgetFor(view)
	limit := contextReadTop
	if len(resources) < limit {
		limit = len(resources)
	}
	for i := 0; i < limit; i++ {
		contents, err := cp.manager.ReadResource(ctx, resources[i].ServerID, resources[i].URI)
		if err != nil {
			resources[i].Error = err.Error()
			logging.Get(logging.CategoryTools).Debug(
				"MCP control plane: read %s failed: %v", resources[i].URI, err)
			continue
		}
		var sb strings.Builder
		for _, c := range contents {
			if c.Text != "" {
				sb.WriteString(c.Text)
				continue
			}
			if c.Blob != "" {
				// Binary content is named, never inlined: a base64 blob in a
				// prompt is pure cost with no readable content.
				sb.WriteString(fmt.Sprintf("[binary %s, %d base64 bytes]", c.MimeType, len(c.Blob)))
			}
		}
		full := sb.String()
		resources[i].Bytes = len(full)

		excerpt := full
		if budget.MaxStringBytes > 0 && len(excerpt) > budget.MaxStringBytes {
			cut := budget.MaxStringBytes
			for cut > 0 && !utf8Boundary(excerpt, cut) {
				cut--
			}
			excerpt = excerpt[:cut] + "…"
			if raw, err := json.Marshal(full); err == nil {
				resources[i].Handle = cp.handles.Mint("resource:"+resources[i].URI, raw)
			}
		}
		resources[i].Excerpt = excerpt
	}
}

// queryTerms shreds a query into lowercase terms.
//
// Terms shorter than three characters are dropped: they match everything and
// therefore rank nothing, which is worse than not matching at all because it
// looks like a result.
func queryTerms(query string) []string {
	fields := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	seen := make(map[string]bool, len(fields))
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		if len(f) < 3 || seen[f] {
			continue
		}
		seen[f] = true
		terms = append(terms, f)
	}
	return terms
}

func scoreText(terms []string, parts ...string) int {
	if len(terms) == 0 {
		return 0
	}
	haystack := strings.ToLower(strings.Join(parts, " "))
	score := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			score += 10
		}
	}
	return score
}

func filterScored(in []ContextResource) []ContextResource {
	out := in[:0]
	for _, r := range in {
		if r.Score > 0 {
			out = append(out, r)
		}
	}
	return out
}

func filterScoredPrompts(in []ContextPrompt) []ContextPrompt {
	out := in[:0]
	for _, p := range in {
		if p.Score > 0 {
			out = append(out, p)
		}
	}
	return out
}

// emitHandleFact publishes an outstanding handle to the kernel.
//
// Handles are planning state, not bookkeeping: "there is evidence I fetched and
// have not looked at" is a fact the executive can act on, and without it a
// truncated result is invisible to everything except the turn that produced it.
func (cp *ControlPlane) emitHandleFact(handle, toolID string, bytes int) {
	if handle == "" || cp.facts == nil {
		return
	}
	cp.facts.EmitHandle(handle, toolID, bytes)
}
