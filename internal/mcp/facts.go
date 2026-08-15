package mcp

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"codenerd/internal/logging"
)

// maxFactStringLen bounds the length of any string argument pushed into the
// kernel. Tool descriptions come from remote servers and are unbounded; a
// multi-kilobyte fact argument bloats every fixpoint evaluation for the rest
// of the session.
const maxFactStringLen = 400

// FactEmitter mirrors MCP runtime state into the Mangle kernel as EDB facts.
//
// Without it the kernel had Decls for the whole MCP surface but no facts, so
// policy_mcp.mg could never fire and tool selection silently degraded to the
// Go affinity heuristic. Emission is subject-keyed (one key per server, tool,
// or usage record) and every key remembers exactly which fact strings it
// asserted, because the kernel adapter retracts by exact fact — a wildcard
// retraction is a no-op there, which would leak stale facts on re-analyze.
type FactEmitter struct {
	mu      sync.Mutex
	kernel  KernelInterface
	emitted map[string][]string
}

// NewFactEmitter returns an emitter bound to kernel. A nil kernel yields a nil
// emitter; every method is nil-safe so callers need no branch.
func NewFactEmitter(kernel KernelInterface) *FactEmitter {
	if kernel == nil {
		return nil
	}
	return &FactEmitter{
		kernel:  kernel,
		emitted: make(map[string][]string),
	}
}

func serverKey(serverID string) string       { return "server:" + serverID }
func serverStatusKey(serverID string) string { return "server_status:" + serverID }
func toolKey(toolID string) string           { return "tool:" + toolID }
func toolUsageKey(toolID string) string      { return "tool_usage:" + toolID }

// replace retracts whatever was previously emitted under key and asserts the
// new set. Retraction happens first so re-analyze cannot leave a tool carrying
// both its old and new categories.
func (e *FactEmitter) replace(key string, facts []string) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.replaceLocked(key, facts)
}

func (e *FactEmitter) replaceLocked(key string, facts []string) {
	// Re-discovery of an unchanged tool is the common case. Retracting and
	// re-asserting identical facts would dirty the kernel and force a fixpoint
	// re-evaluation for nothing.
	if slices.Equal(e.emitted[key], facts) {
		return
	}

	for _, old := range e.emitted[key] {
		if err := e.kernel.Retract(old); err != nil {
			logging.Get(logging.CategoryTools).Debug("MCP fact retract failed (%s): %v", old, err)
		}
	}
	delete(e.emitted, key)

	if len(facts) == 0 {
		return
	}
	asserted := make([]string, 0, len(facts))
	for _, f := range facts {
		if err := e.kernel.Assert(f); err != nil {
			logging.Get(logging.CategoryTools).Warn("MCP fact assert failed (%s): %v", f, err)
			continue
		}
		asserted = append(asserted, f)
	}
	if len(asserted) > 0 {
		e.emitted[key] = asserted
	}
}

// EmitServer publishes registration, name, and capability facts for a server.
func (e *FactEmitter) EmitServer(server *MCPServer) {
	if e == nil || server == nil || server.ID == "" {
		return
	}
	e.replace(serverKey(server.ID), serverFacts(server))
	e.EmitServerStatus(server.ID, server.Status)
}

// EmitServerStatus updates only the status fact, leaving the server's
// registration facts in place. Availability in policy keys off this predicate,
// so a disconnect must flip it rather than drop the catalog.
func (e *FactEmitter) EmitServerStatus(serverID string, status ServerStatus) {
	if e == nil || serverID == "" {
		return
	}
	if status == "" {
		status = ServerStatusUnknown
	}
	e.replace(serverStatusKey(serverID), []string{
		fmt.Sprintf("mcp_server_status(%s, %s)", mangleString(serverID), mangleAtom(string(status))),
	})
}

// EmitTool publishes the full metadata fact set for a discovered tool.
// Re-analysis of the same tool ID replaces the previous set wholesale.
func (e *FactEmitter) EmitTool(tool *MCPTool) {
	if e == nil || tool == nil || tool.ToolID == "" {
		return
	}
	e.replace(toolKey(tool.ToolID), toolFacts(tool))
	e.EmitToolUsage(tool)
}

// EmitToolUsage publishes the usage counters policy uses for the success-rate
// boost and the slow-tool penalty.
func (e *FactEmitter) EmitToolUsage(tool *MCPTool) {
	if e == nil || tool == nil || tool.ToolID == "" {
		return
	}
	e.replace(toolUsageKey(tool.ToolID), toolUsageFacts(tool))
}

// EmitReady publishes the boot readiness fact. Readiness is a kernel-visible
// condition, not just a Go channel: policy that wants to wait for the MCP
// catalog before recommending an integration call can join on it.
func (e *FactEmitter) EmitReady(serverCount, toolCount int) {
	if e == nil {
		return
	}
	e.replace("integration_ready", []string{
		fmt.Sprintf("mcp_integration_ready(%d, %d)", serverCount, toolCount),
	})
}

// EmitResources publishes a server's resource catalog. The whole catalog is
// one subject so a rediscovery that drops a resource drops its facts too.
func (e *FactEmitter) EmitResources(serverID string, resources []MCPResource) {
	if e == nil || serverID == "" {
		return
	}
	facts := make([]string, 0, len(resources)*2)
	for _, r := range resources {
		if strings.TrimSpace(r.URI) == "" {
			continue
		}
		facts = append(facts, fmt.Sprintf("mcp_resource_registered(%s, %s)",
			mangleString(serverID), mangleString(r.URI)))
		if mime := strings.TrimSpace(r.MimeType); mime != "" {
			facts = append(facts, fmt.Sprintf("mcp_resource_mime(%s, %s)",
				mangleString(r.URI), mangleString(mime)))
		}
		if name := strings.TrimSpace(r.Name); name != "" {
			facts = append(facts, fmt.Sprintf("mcp_resource_name(%s, %s)",
				mangleString(r.URI), mangleString(name)))
		}
	}
	e.replace("resources:"+serverID, facts)
}

// EmitPrompts publishes a server's prompt-template catalog.
func (e *FactEmitter) EmitPrompts(serverID string, prompts []MCPPrompt) {
	if e == nil || serverID == "" {
		return
	}
	facts := make([]string, 0, len(prompts)*2)
	for _, p := range prompts {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		facts = append(facts, fmt.Sprintf("mcp_prompt_registered(%s, %s)",
			mangleString(serverID), mangleString(name)))
		for _, arg := range p.Arguments {
			argName := strings.TrimSpace(arg.Name)
			if argName == "" {
				continue
			}
			required := "/false"
			if arg.Required {
				required = "/true"
			}
			facts = append(facts, fmt.Sprintf("mcp_prompt_argument(%s, %s, %s)",
				mangleString(name), mangleString(argName), required))
		}
	}
	e.replace("prompts:"+serverID, facts)
}

// RetractTool removes every fact for a tool. Used when a server stops
// advertising a tool it previously exposed.
func (e *FactEmitter) RetractTool(toolID string) {
	if e == nil || toolID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.replaceLocked(toolKey(toolID), nil)
	e.replaceLocked(toolUsageKey(toolID), nil)
}

// RetractServer removes every fact for a server and for all tools emitted
// under it. Disconnect does NOT call this — it only flips status — because a
// cached catalog stays useful for reasoning. This is for full deregistration.
func (e *FactEmitter) RetractServer(serverID string) {
	if e == nil || serverID == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	e.replaceLocked(serverKey(serverID), nil)
	e.replaceLocked(serverStatusKey(serverID), nil)

	prefixes := []string{
		toolKey(serverID + "/"),
		toolUsageKey(serverID + "/"),
		"resources:" + serverID,
		"prompts:" + serverID,
	}
	stale := make([]string, 0)
	for key := range e.emitted {
		for _, prefix := range prefixes {
			if strings.HasPrefix(key, prefix) {
				stale = append(stale, key)
				break
			}
		}
	}
	sort.Strings(stale)
	for _, key := range stale {
		e.replaceLocked(key, nil)
	}
}

// EmittedFactCount reports how many facts this emitter currently believes it
// has asserted. Exposed for tests and the `nerd mcp facts` command.
func (e *FactEmitter) EmittedFactCount() int {
	if e == nil {
		return 0
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, facts := range e.emitted {
		n += len(facts)
	}
	return n
}

// EmittedFacts returns a sorted snapshot of every fact this emitter asserted.
func (e *FactEmitter) EmittedFacts() []string {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, 0, len(e.emitted))
	for _, facts := range e.emitted {
		out = append(out, facts...)
	}
	sort.Strings(out)
	return out
}

// serverFacts builds the EDB for a server, excluding its status.
func serverFacts(server *MCPServer) []string {
	registeredAt := server.DiscoveredAt
	if registeredAt.IsZero() {
		registeredAt = time.Now()
	}

	protocol := string(server.Protocol)
	if protocol == "" {
		protocol = "unknown"
	}
	endpoint := server.Endpoint
	if endpoint == "" {
		endpoint = server.ID
	}

	facts := []string{
		fmt.Sprintf("mcp_server_registered(%s, %s, %s, %d)",
			mangleString(server.ID), mangleString(endpoint), mangleAtom(protocol), registeredAt.Unix()),
	}
	if name := strings.TrimSpace(server.Name); name != "" {
		facts = append(facts, fmt.Sprintf("mcp_server_name(%s, %s)",
			mangleString(server.ID), mangleString(name)))
	}
	for _, capability := range dedupe(server.Capabilities) {
		facts = append(facts, fmt.Sprintf("mcp_server_capabilities(%s, %s)",
			mangleString(server.ID), mangleAtom(capability)))
	}
	return facts
}

// toolFacts builds the EDB for a tool, excluding usage counters.
func toolFacts(tool *MCPTool) []string {
	registeredAt := tool.RegisteredAt
	if registeredAt.IsZero() {
		registeredAt = time.Now()
	}

	facts := []string{
		fmt.Sprintf("mcp_tool_registered(%s, %s, %d)",
			mangleString(tool.ToolID), mangleString(tool.ServerID), registeredAt.Unix()),
	}
	if name := strings.TrimSpace(tool.Name); name != "" {
		facts = append(facts, fmt.Sprintf("mcp_tool_name(%s, %s)",
			mangleString(tool.ToolID), mangleString(name)))
	}
	if desc := strings.TrimSpace(tool.Description); desc != "" {
		facts = append(facts, fmt.Sprintf("mcp_tool_description(%s, %s)",
			mangleString(tool.ToolID), mangleString(desc)))
	}
	if condensed := strings.TrimSpace(tool.Condensed); condensed != "" {
		facts = append(facts, fmt.Sprintf("mcp_tool_condensed(%s, %s)",
			mangleString(tool.ToolID), mangleString(condensed)))
	}
	for _, capability := range dedupe(tool.Capabilities) {
		facts = append(facts, fmt.Sprintf("mcp_tool_capability(%s, %s)",
			mangleString(tool.ToolID), mangleAtom(capability)))
	}
	for _, category := range dedupe(tool.Categories) {
		facts = append(facts, fmt.Sprintf("mcp_tool_category(%s, %s)",
			mangleString(tool.ToolID), mangleAtom(category)))
	}
	if domain := strings.TrimSpace(tool.Domain); domain != "" {
		facts = append(facts, fmt.Sprintf("mcp_tool_domain(%s, %s)",
			mangleString(tool.ToolID), mangleAtom(domain)))
	}

	shards := make([]string, 0, len(tool.ShardAffinities))
	for shard := range tool.ShardAffinities {
		shards = append(shards, shard)
	}
	sort.Strings(shards)
	for _, shard := range shards {
		facts = append(facts, fmt.Sprintf("mcp_tool_shard_affinity(%s, %s, %d)",
			mangleString(tool.ToolID), mangleAtom(shard), tool.ShardAffinities[shard]))
	}

	if !tool.AnalyzedAt.IsZero() {
		facts = append(facts, fmt.Sprintf("mcp_tool_analyzed(%s)", mangleString(tool.ToolID)))
	}
	return facts
}

// toolUsageFacts builds the usage counter EDB. Success *rate* is deliberately
// not emitted: policy derives it from the raw counters so the formula lives in
// one place, in Mangle.
func toolUsageFacts(tool *MCPTool) []string {
	if tool.UsageCount <= 0 {
		return nil
	}
	facts := []string{
		fmt.Sprintf("mcp_tool_usage(%s, %d, %d)",
			mangleString(tool.ToolID), tool.UsageCount, tool.SuccessCount),
	}
	if !tool.LastUsed.IsZero() {
		facts = append(facts, fmt.Sprintf("mcp_tool_last_used(%s, %d)",
			mangleString(tool.ToolID), tool.LastUsed.Unix()))
	}
	if tool.AvgLatencyMs > 0 {
		facts = append(facts, fmt.Sprintf("mcp_tool_avg_latency(%s, %d)",
			mangleString(tool.ToolID), tool.AvgLatencyMs))
	}
	return facts
}

// mangleAtom converts an arbitrary label into a Mangle name constant.
// Analyzer output is already normalized ("/read", "filesystem"), but server
// capabilities and LLM-invented shard names are not.
func mangleAtom(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	trimmed = strings.TrimPrefix(trimmed, "/")

	var b strings.Builder
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "/unknown"
	}
	// A name constant must start with a letter; scores like "3d_render" would
	// otherwise produce an unparseable atom.
	if out[0] >= '0' && out[0] <= '9' {
		out = "n" + out
	}
	return "/" + out
}

// mangleString renders a Go string as a quoted Mangle string constant, with
// control characters and non-ASCII runes removed. The kernel's fact parser is
// stricter than Go's %q escaping, and remote tool descriptions routinely carry
// emoji and box-drawing characters.
func mangleString(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r > unicode.MaxASCII || r < 0x20 || r == 0x7f {
			b.WriteByte(' ')
			continue
		}
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	out := strings.TrimSpace(b.String())
	if len(out) > maxFactStringLen {
		out = strings.TrimSpace(out[:maxFactStringLen])
		// Never end on a dangling escape: "...\" would swallow the closing quote.
		for strings.HasSuffix(out, `\`) && !strings.HasSuffix(out, `\\`) {
			out = out[:len(out)-1]
		}
	}
	return `"` + out + `"`
}

func dedupe(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
