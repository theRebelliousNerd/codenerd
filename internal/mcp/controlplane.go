package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// The control plane is the answer to a specific economic problem, and it is
// worth stating the problem before the solution.
//
// Discovering a server's tools and describing them to a model are two different
// costs, and only the first one is paid once. A fleet of MCP servers advertises
// dozens to hundreds of tools; rendering their schemas into a prompt costs
// thousands of tokens on EVERY turn, forever, whether or not the agent touches
// any of them. That standing cost is the reason a fully-built selection stack
// can sit finished and unwired: nobody wants to sign a recurring bill for
// capability that is usually idle.
//
// A control plane changes the shape of the bill. The model is given a small,
// FIXED set of verbs whose size does not depend on how many servers are
// connected — an atlas it can afford on every turn, and a zoom it pays for only
// when it is actually reaching for something. Sixty tools stop costing sixty
// tools' worth of prompt and start costing one atlas plus whatever the agent
// deliberately opens.
//
// The facade is fixed. What varies per server is derived: which facets it
// fills, how risky each tool is, and what its results look like once shaped.

// ControlPlane turns connected MCP servers into a progressively-disclosed
// context and tool surface.
type ControlPlane struct {
	manager *MCPClientManager
	store   *MCPToolStore
	handles *HandleStore
	facts   *FactEmitter

	mu       sync.RWMutex
	catalogs map[string]*serverCatalog
	// catalogTTL bounds how stale a cached resource/prompt catalog may be.
	catalogTTL time.Duration
	// gate is the policy hook consulted before any call. See RiskGate.
	gate RiskGate

	now func() time.Time
}

// serverCatalog caches a server's non-tool primitives.
//
// Caching these is not an optimisation, it is a correctness-of-cost decision: a
// context lookup that re-listed every resource on every call would make the
// cheap operation the expensive one, and an agent that learns context lookups
// are slow stops doing them.
type serverCatalog struct {
	resources []MCPResource
	prompts   []MCPPrompt
	fetchedAt time.Time
	// err records why a catalog is empty, so a server that does not support
	// resources is reported as such rather than as one with none.
	err string
}

// NewControlPlane builds a control plane over an existing client manager.
func NewControlPlane(manager *MCPClientManager, store *MCPToolStore, facts *FactEmitter) *ControlPlane {
	return &ControlPlane{
		manager:    manager,
		store:      store,
		handles:    NewHandleStore(DefaultHandleStoreConfig()),
		facts:      facts,
		catalogs:   make(map[string]*serverCatalog),
		catalogTTL: 5 * time.Minute,
		now:        time.Now,
	}
}

// Handles exposes the retention store so a caller can report or bound it.
func (cp *ControlPlane) Handles() *HandleStore { return cp.handles }

// tools returns the classified catalog, preferring the persisted store.
//
// The store is authoritative rather than the manager's in-memory cache because
// it survives a restart with classifications intact. An atlas that goes blank
// until rediscovery finishes is an atlas the agent learns not to trust.
func (cp *ControlPlane) tools(ctx context.Context) []*MCPTool {
	if cp.store != nil {
		if tools, err := cp.store.GetAllTools(ctx); err == nil && len(tools) > 0 {
			return tools
		}
	}
	if cp.manager != nil {
		return cp.manager.GetAllTools()
	}
	return nil
}

func (cp *ControlPlane) toolsByServer(ctx context.Context) map[string][]*MCPTool {
	grouped := make(map[string][]*MCPTool)
	for _, t := range cp.tools(ctx) {
		if t == nil || t.ServerID == "" {
			continue
		}
		grouped[t.ServerID] = append(grouped[t.ServerID], t)
	}
	return grouped
}

// serverStatuses reports the live status of every server the plane knows about.
func (cp *ControlPlane) serverStatuses(ctx context.Context) map[string]ServerStatus {
	statuses := make(map[string]ServerStatus)
	if cp.store != nil {
		if servers, err := cp.store.GetAllServers(ctx); err == nil {
			for _, s := range servers {
				if s != nil {
					statuses[s.ID] = s.Status
				}
			}
		}
	}
	// The live manager overrides the store: the store records what was true at
	// the last write, and a server that dropped since then must not be
	// advertised as reachable.
	if cp.manager != nil {
		for _, id := range cp.manager.GetConnectedServers() {
			statuses[id] = ServerStatusConnected
		}
	}
	return statuses
}

// --- Atlas -----------------------------------------------------------------

// AtlasOptions selects what the map covers.
type AtlasOptions struct {
	Server string
	View   View
}

// ServerAtlas is one server's row in the map.
type ServerAtlas struct {
	ServerID  string        `json:"server_id"`
	Status    ServerStatus  `json:"status"`
	Tools     int           `json:"tools"`
	Facets    []FacetCensus `json:"facets,omitzero"`
	Resources int           `json:"resources,omitzero"`
	Prompts   int           `json:"prompts,omitzero"`
}

// AtlasResult is the whole reachable surface at one disclosure depth.
type AtlasResult struct {
	Success    bool          `json:"success"`
	View       View          `json:"view"`
	Summary    string        `json:"summary"`
	Servers    []ServerAtlas `json:"servers"`
	TotalTools int           `json:"total_tools"`
	NextStep   string        `json:"next_step"`
}

// Atlas renders the map of everything reachable.
//
// This is the one call the agent should be able to make on any turn without
// thinking about cost. Summary view is a handful of lines for an entire fleet;
// even compact stays proportional to servers x facets rather than to tools,
// which is what keeps a 200-tool server from costing more than a 20-tool one.
func (cp *ControlPlane) Atlas(ctx context.Context, opts AtlasOptions) *AtlasResult {
	view := opts.View
	if view == "" {
		view = ViewCompact
	}

	grouped := cp.toolsByServer(ctx)
	statuses := cp.serverStatuses(ctx)

	// Every server with a status gets a row even if it has no tools yet: an
	// agent needs to be able to tell "connected, nothing discovered" from "not
	// connected", and both from "no such server".
	ids := make([]string, 0, len(grouped))
	seen := make(map[string]bool, len(grouped))
	for id := range grouped {
		ids = append(ids, id)
		seen[id] = true
	}
	for id := range statuses {
		if !seen[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	result := &AtlasResult{Success: true, View: view}
	for _, id := range ids {
		if opts.Server != "" && id != opts.Server {
			continue
		}
		row := ServerAtlas{
			ServerID: id,
			Status:   statuses[id],
			Tools:    len(grouped[id]),
		}
		if row.Status == "" {
			row.Status = ServerStatusUnknown
		}
		if view != ViewSummary {
			row.Facets = CensusFor(grouped[id])
		}
		if cat := cp.cachedCatalog(id); cat != nil {
			row.Resources = len(cat.resources)
			row.Prompts = len(cat.prompts)
		}
		result.TotalTools += row.Tools
		result.Servers = append(result.Servers, row)
	}

	result.Summary = atlasSummary(result)
	result.NextStep = "mcp_probe(server=..., facet=...) to see tool signatures; mcp_call to invoke"
	return result
}

func atlasSummary(a *AtlasResult) string {
	if len(a.Servers) == 0 {
		return "no MCP servers configured or connected"
	}
	connected := 0
	for _, s := range a.Servers {
		if s.Status == ServerStatusConnected {
			connected++
		}
	}
	return fmt.Sprintf("%d/%d server(s) connected, %d tool(s) reachable",
		connected, len(a.Servers), a.TotalTools)
}

// --- Probe -----------------------------------------------------------------

// ProbeOptions selects which slice of the catalog to open.
type ProbeOptions struct {
	Server   string
	Facet    Facet
	Tool     string
	Query    string
	MaxRisk  RiskClass
	View     View
	MaxItems int
}

// ProbeTool is one tool at browsing depth.
type ProbeTool struct {
	ToolID      string    `json:"tool_id"`
	Signature   string    `json:"signature"`
	Purpose     string    `json:"purpose,omitzero"`
	Facet       Facet     `json:"facet"`
	Risk        RiskClass `json:"risk"`
	RiskSource  string    `json:"risk_source,omitzero"`
	Schema      string    `json:"schema,omitzero"`
	Description string    `json:"description,omitzero"`
	UsageCount  int64     `json:"usage_count,omitzero"`
	SuccessRate int       `json:"success_rate,omitzero"`
}

// ProbeResult is a bounded window onto the catalog.
type ProbeResult struct {
	Success   bool        `json:"success"`
	View      View        `json:"view"`
	Summary   string      `json:"summary"`
	Tools     []ProbeTool `json:"tools"`
	Matched   int         `json:"matched"`
	Truncated bool        `json:"truncated"`
	NextStep  string      `json:"next_step,omitzero"`
}

// probeDefaultMaxItems bounds a browse. Twelve signature lines is roughly 200
// tokens, which is the most a browsing step should ever cost.
const probeDefaultMaxItems = 12

// probeHardCap bounds what a caller may ask for, mirroring the browser tools'
// documented ceilings.
const probeHardCap = 60

// Probe opens one slice of the catalog at the requested depth.
//
// Naming a single tool switches this from browsing to disclosure: that tool
// alone comes back with its full schema. That is the whole progressive ladder —
// atlas names facets, probe names tools, probing one tool names its arguments —
// and each rung is paid for only by the agent that climbs it.
func (cp *ControlPlane) Probe(ctx context.Context, opts ProbeOptions) *ProbeResult {
	view := opts.View
	if view == "" {
		view = ViewCompact
	}
	maxItems := opts.MaxItems
	if maxItems <= 0 {
		maxItems = probeDefaultMaxItems
	}
	if maxItems > probeHardCap {
		maxItems = probeHardCap
	}

	all := cp.tools(ctx)

	// A named tool is an exact request, so it bypasses every filter and every
	// bound below. Asking for one tool and receiving a ranked list of neighbours
	// would be a worse answer than the question deserved.
	if name := strings.TrimSpace(opts.Tool); name != "" {
		tool, err := resolveTool(all, name, opts.Server)
		if err != nil {
			return &ProbeResult{
				Success: false, View: view,
				Summary:  err.Error(),
				NextStep: "mcp_map to see what is reachable, or pass the full server/tool id",
			}
		}
		return &ProbeResult{
			Success: true, View: ViewFull, Matched: 1,
			Summary:  fmt.Sprintf("%s [%s/%s]", tool.ToolID, tool.Facet, tool.Risk),
			Tools:    []ProbeTool{probeToolFrom(tool, ViewFull)},
			NextStep: "mcp_call(tool=\"" + tool.ToolID + "\", args={...})",
		}
	}

	matches := make([]*MCPTool, 0, len(all))
	for _, t := range all {
		if t == nil || !cp.probeMatches(t, opts) {
			continue
		}
		matches = append(matches, t)
	}

	rankProbeMatches(matches, opts.Query)

	result := &ProbeResult{Success: true, View: view, Matched: len(matches)}
	if len(matches) > maxItems {
		result.Truncated = true
		matches = matches[:maxItems]
	}
	for _, t := range matches {
		result.Tools = append(result.Tools, probeToolFrom(t, view))
	}

	result.Summary = fmt.Sprintf("%d tool(s) matched, showing %d", result.Matched, len(result.Tools))
	if result.Truncated {
		result.NextStep = "narrow with facet=, query=, or server=; or raise max_items"
	} else if len(result.Tools) > 0 {
		result.NextStep = "mcp_probe(tool=\"<tool_id>\") for the full schema, or mcp_call to invoke"
	}
	return result
}

func (cp *ControlPlane) probeMatches(t *MCPTool, opts ProbeOptions) bool {
	if opts.Server != "" && t.ServerID != opts.Server {
		return false
	}
	if opts.Facet != "" && t.Facet != opts.Facet {
		return false
	}
	if opts.MaxRisk != "" && opts.MaxRisk.Valid() && t.Risk.Rank() > opts.MaxRisk.Rank() {
		return false
	}
	if q := strings.ToLower(strings.TrimSpace(opts.Query)); q != "" {
		haystack := strings.ToLower(strings.Join([]string{
			t.Name, t.Condensed, t.Description, t.Domain,
			strings.Join(t.Categories, " "), strings.Join(t.UseCases, " "),
		}, " "))
		for _, term := range strings.Fields(q) {
			if strings.Contains(haystack, term) {
				return true
			}
		}
		return false
	}
	return true
}

// rankProbeMatches orders a browse.
//
// Proven tools first, because usage is the only signal here that reflects what
// actually worked in this workspace rather than what a description claimed.
// Everything else is a tie-break that exists so the same query returns the same
// order twice running.
func rankProbeMatches(tools []*MCPTool, query string) {
	q := strings.ToLower(strings.TrimSpace(query))
	score := func(t *MCPTool) int {
		s := 0
		if q != "" && strings.Contains(strings.ToLower(t.Name), q) {
			// A name hit is a much stronger signal than a description hit,
			// which is why the filter above accepts both and the rank splits
			// them.
			s += 50
		}
		if t.UsageCount >= 3 {
			rate := int((t.SuccessCount * 100) / t.UsageCount)
			s += rate / 5
		}
		// Safer tools sort first at equal evidence: a browse is exploratory,
		// and an exploratory list headed by destructive operations invites the
		// mistake it should be preventing.
		s += (3 - t.Risk.Rank()) * 2
		return s
	}
	sort.SliceStable(tools, func(i, j int) bool {
		si, sj := score(tools[i]), score(tools[j])
		if si != sj {
			return si > sj
		}
		return tools[i].ToolID < tools[j].ToolID
	})
}

func probeToolFrom(t *MCPTool, view View) ProbeTool {
	out := ProbeTool{
		ToolID:    t.ToolID,
		Signature: ToolSignature(t),
		Purpose:   t.Condensed,
		Facet:     t.Facet,
		Risk:      t.Risk,
	}
	if t.UsageCount > 0 {
		out.UsageCount = t.UsageCount
		out.SuccessRate = int((t.SuccessCount * 100) / t.UsageCount)
	}
	switch view {
	case ViewSummary:
		// Summary is a name-and-shape list: enough to pick, not enough to call.
		out.Purpose = ""
		out.UsageCount, out.SuccessRate = 0, 0
	case ViewFull:
		out.Description = t.Description
		out.Schema = FullSchema(t, 4000)
		out.RiskSource = string(t.RiskSource)
	}
	return out
}

// AmbiguousToolError reports a bare name that matches tools on several servers.
type AmbiguousToolError struct {
	Name    string
	ToolIDs []string
}

func (e *AmbiguousToolError) Error() string {
	return fmt.Sprintf("tool name %q is ambiguous across servers: %s (use the full server/tool id)",
		e.Name, strings.Join(e.ToolIDs, ", "))
}

// resolveTool finds a tool, distinguishing "not found" from "ambiguous".
func resolveTool(tools []*MCPTool, name, server string) (*MCPTool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("tool is required")
	}
	var byName []*MCPTool
	for _, t := range tools {
		if t == nil {
			continue
		}
		if server != "" && t.ServerID != server {
			continue
		}
		if t.ToolID == name {
			return t, nil
		}
		if t.Name == name {
			byName = append(byName, t)
		}
	}
	switch len(byName) {
	case 0:
		return nil, fmt.Errorf("no tool %q is registered", name)
	case 1:
		return byName[0], nil
	default:
		ids := make([]string, 0, len(byName))
		for _, t := range byName {
			ids = append(ids, t.ToolID)
		}
		sort.Strings(ids)
		return nil, &AmbiguousToolError{Name: name, ToolIDs: ids}
	}
}

// --- Catalog caching -------------------------------------------------------

func (cp *ControlPlane) cachedCatalog(serverID string) *serverCatalog {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	cat, ok := cp.catalogs[serverID]
	if !ok || cp.now().Sub(cat.fetchedAt) > cp.catalogTTL {
		return nil
	}
	return cat
}

// catalogFor returns a server's resources and prompts, refreshing on miss.
//
// A server that advertises neither capability is cached as empty rather than
// re-probed: the answer will not change until it reconnects, and paying two
// failing round-trips per context lookup is how a cheap operation becomes an
// expensive one.
func (cp *ControlPlane) catalogFor(ctx context.Context, serverID string) *serverCatalog {
	if cat := cp.cachedCatalog(serverID); cat != nil {
		return cat
	}
	cat := &serverCatalog{fetchedAt: cp.now()}
	if cp.manager != nil {
		if resources, err := cp.manager.DiscoverResources(ctx, serverID); err == nil {
			cat.resources = resources
		} else {
			cat.err = err.Error()
			logging.Get(logging.CategoryTools).Debug("MCP control plane: no resources from %s: %v", serverID, err)
		}
		if prompts, err := cp.manager.DiscoverPrompts(ctx, serverID); err == nil {
			cat.prompts = prompts
		} else if cat.err == "" {
			cat.err = err.Error()
		}
	}
	cp.mu.Lock()
	cp.catalogs[serverID] = cat
	cp.mu.Unlock()
	return cat
}

// InvalidateCatalog drops a cached catalog, so a reconnect or an explicit
// rediscovery is reflected without waiting out the TTL.
func (cp *ControlPlane) InvalidateCatalog(serverID string) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	if serverID == "" {
		cp.catalogs = make(map[string]*serverCatalog)
		return
	}
	delete(cp.catalogs, serverID)
}
