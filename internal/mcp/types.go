// Package mcp provides MCP (Model Context Protocol) client integration
// with JIT-style tool compilation for intelligent tool serving.
package mcp

import (
	"context"
	"encoding/json"
	"time"
)

// ServerStatus represents the connection status of an MCP server.
type ServerStatus string

const (
	ServerStatusUnknown      ServerStatus = "unknown"
	ServerStatusConnecting   ServerStatus = "connecting"
	ServerStatusConnected    ServerStatus = "connected"
	ServerStatusDisconnected ServerStatus = "disconnected"
	ServerStatusError        ServerStatus = "error"
)

// Protocol represents the MCP transport protocol.
type Protocol string

const (
	ProtocolHTTP  Protocol = "http"
	ProtocolStdio Protocol = "stdio"
	ProtocolSSE   Protocol = "sse"
)

// RenderMode determines how a tool is rendered for LLM context.
type RenderMode string

const (
	RenderModeFull      RenderMode = "full"      // Complete schema and description
	RenderModeCondensed RenderMode = "condensed" // Name + one-liner
	RenderModeMinimal   RenderMode = "minimal"   // Name only
	RenderModeExcluded  RenderMode = "excluded"  // Not sent to LLM
)

// MCPServerConfig represents configuration for an MCP server from config.json.
type MCPServerConfig struct {
	ID                string `json:"id"`
	Enabled           bool   `json:"enabled"`
	Protocol          string `json:"protocol"`
	BaseURL           string `json:"base_url"`
	Endpoint          string `json:"endpoint,omitempty"` // For stdio protocol
	Timeout           string `json:"timeout"`
	AutoConnect       bool   `json:"auto_connect"`
	AutoDiscoverTools bool   `json:"auto_discover_tools"`
}

// MCPServer represents a connected MCP server.
type MCPServer struct {
	ID            string       `json:"server_id"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Endpoint      string       `json:"endpoint"`
	Protocol      Protocol     `json:"protocol"`
	Status        ServerStatus `json:"status"`
	Capabilities  []string     `json:"capabilities"`
	DiscoveredAt  time.Time    `json:"discovered_at"`
	LastConnected time.Time    `json:"last_connected"`
	LastPing      time.Time    `json:"last_ping"`
	RetryCount    int          `json:"retry_count"`
	Config        string       `json:"config"` // JSON string for server-specific config
}

// MCPTool represents a tool discovered from an MCP server.
type MCPTool struct {
	// Identity
	ToolID   string `json:"tool_id"`
	ServerID string `json:"server_id"`
	Name     string `json:"name"`

	// Schema (from MCP server)
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"input_schema"`
	OutputSchema json.RawMessage `json:"output_schema,omitempty"`

	// LLM-extracted metadata
	Categories      []string       `json:"categories"`       // ["filesystem", "code_analysis"]
	Capabilities    []string       `json:"capabilities"`     // ["/read", "/write", "/search"]
	Domain          string         `json:"domain"`           // "/go", "/python", "/general"
	ShardAffinities map[string]int `json:"shard_affinities"` // {"coder": 90, "tester": 50}
	UseCases        []string       `json:"use_cases"`
	Condensed       string         `json:"condensed"` // One-line description (max 80 chars)

	// Embedding
	Embedding      []float32 `json:"embedding,omitempty"`
	EmbeddingModel string    `json:"embedding_model,omitempty"`

	// Usage statistics
	UsageCount   int64     `json:"usage_count"`
	SuccessCount int64     `json:"success_count"`
	AvgLatencyMs int       `json:"avg_latency_ms"`
	LastUsed     time.Time `json:"last_used,omitempty"`

	// Timestamps
	RegisteredAt time.Time `json:"registered_at"`
	AnalyzedAt   time.Time `json:"analyzed_at,omitempty"`

	// Runtime state (not persisted)
	ServerStatus ServerStatus `json:"-"` // Cached from server
}

// MCPToolSchema represents the raw tool schema from an MCP server.
type MCPToolSchema struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	OutputSchema json.RawMessage `json:"outputSchema,omitempty"`
}

// MCPCapabilities represents server capabilities from the MCP protocol.
type MCPCapabilities struct {
	Tools     bool `json:"tools"`
	Resources bool `json:"resources"`
	Prompts   bool `json:"prompts"`
	Logging   bool `json:"logging"`
}

// ToolAnalysis represents the LLM-extracted metadata for a tool.
type ToolAnalysis struct {
	ToolID          string         `json:"tool_id"`
	Categories      []string       `json:"categories"`
	Capabilities    []string       `json:"capabilities"`
	Domain          string         `json:"domain"`
	ShardAffinities map[string]int `json:"shard_affinities"`
	UseCases        []string       `json:"use_cases"`
	Condensed       string         `json:"condensed"`
	Embedding       []float32      `json:"embedding,omitempty"`
}

// ToolCompilationContext provides context for JIT tool compilation.
type ToolCompilationContext struct {
	ShardType       string   `json:"shard_type"`
	TaskDescription string   `json:"task_description"`
	IntentVerb      string   `json:"intent_verb"`
	TargetLanguage  string   `json:"target_language"`
	CurrentFiles    []string `json:"current_files"`
	TokenBudget     int      `json:"token_budget"`
}

// CompiledToolSet represents the result of JIT tool compilation.
type CompiledToolSet struct {
	FullTools      []MCPTool     `json:"full_tools"`      // Complete schema, high relevance
	CondensedTools []ToolSummary `json:"condensed_tools"` // Name + one-liner
	MinimalTools   []string      `json:"minimal_tools"`   // Just names

	Stats ToolCompilationStats `json:"stats"`
}

// ToolSummary provides a condensed view of a tool.
type ToolSummary struct {
	Name      string `json:"name"`
	Condensed string `json:"condensed"`
	ServerID  string `json:"server_id"`
}

// ToolCompilationStats tracks JIT compilation performance.
type ToolCompilationStats struct {
	Duration        time.Duration `json:"duration"`
	TotalTools      int           `json:"total_tools"`
	SelectedTools   int           `json:"selected_tools"`
	SkeletonTools   int           `json:"skeleton_tools"`
	FleshTools      int           `json:"flesh_tools"`
	VectorQueryMs   int64         `json:"vector_query_ms"`
	MangleQueryMs   int64         `json:"mangle_query_ms"`
	TokensUsed      int           `json:"tokens_used"`
	TokenBudget     int           `json:"token_budget"`
	CacheHit        bool          `json:"cache_hit"`
}

// ToolSelectionConfig holds thresholds and weights for tool selection.
type ToolSelectionConfig struct {
	SkeletonThreshold  int     `json:"skeleton_threshold"`  // Score for mandatory tools (default: 90)
	FullThreshold      int     `json:"full_threshold"`      // Score for full render (default: 70)
	CondensedThreshold int     `json:"condensed_threshold"` // Score for condensed render (default: 40)
	MinimalThreshold   int     `json:"minimal_threshold"`   // Score for minimal render (default: 20)
	LogicWeight        float64 `json:"logic_weight"`        // Weight for Mangle score (default: 0.7)
	VectorWeight       float64 `json:"vector_weight"`       // Weight for vector score (default: 0.3)
	MaxFullTools       int     `json:"max_full_tools"`      // Max tools with full schema (default: 10)
	MaxCondensedTools  int     `json:"max_condensed_tools"` // Max tools with condensed info (default: 20)
	TokenBudget        int     `json:"token_budget"`        // Token budget for tool descriptions (default: 4000)
}

// DefaultToolSelectionConfig returns sensible defaults for tool selection.
func DefaultToolSelectionConfig() ToolSelectionConfig {
	return ToolSelectionConfig{
		SkeletonThreshold:  90,
		FullThreshold:      70,
		CondensedThreshold: 40,
		MinimalThreshold:   20,
		LogicWeight:        0.7,
		VectorWeight:       0.3,
		MaxFullTools:       10,
		MaxCondensedTools:  20,
		TokenBudget:        4000,
	}
}

// SelectedTool represents a tool selected by the JIT compiler.
type SelectedTool struct {
	ToolID      string     `json:"tool_id"`
	RenderMode  RenderMode `json:"render_mode"`
	LogicScore  int        `json:"logic_score"`
	VectorScore int        `json:"vector_score"`
	FinalScore  int        `json:"final_score"`
}

// MCPCallResult represents the result of calling an MCP tool.
type MCPCallResult struct {
	Success   bool            `json:"success"`
	Output    json.RawMessage `json:"output,omitempty"`
	Error     string          `json:"error,omitempty"`
	LatencyMs int64           `json:"latency_ms"`
}

// MCPTransport defines the interface for MCP protocol transports.
type MCPTransport interface {
	// Connect establishes connection to the MCP server.
	Connect(ctx context.Context) error

	// Disconnect closes the connection.
	Disconnect() error

	// ListTools retrieves available tools from the server.
	ListTools(ctx context.Context) ([]MCPToolSchema, error)

	// CallTool invokes a tool on the MCP server.
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPCallResult, error)

	// GetCapabilities returns server capabilities.
	GetCapabilities(ctx context.Context) (*MCPCapabilities, error)

	// Ping checks if the server is responsive.
	Ping(ctx context.Context) error

	// IsConnected returns current connection status.
	IsConnected() bool
}

// ToolAvailableEntry represents an MCP tool entry in available_tools.json.
type ToolAvailableEntry struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name,omitempty"`
	Category      string   `json:"category"`
	Description   string   `json:"description,omitempty"`
	Type          string   `json:"type"` // "mcp" for MCP tools, empty for static
	MCPServer     string   `json:"mcp_server,omitempty"`
	MCPTool       string   `json:"mcp_tool,omitempty"`
	ShardAffinity string   `json:"shard_affinity,omitempty"`
	AutoAnalyze   bool     `json:"auto_analyze,omitempty"`
	Conditions    []string `json:"conditions,omitempty"`

	// Static tool fields (non-MCP)
	Command    string `json:"command,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
	InputType  string `json:"input_type,omitempty"`
	OutputType string `json:"output_type,omitempty"`
}

// IsMCPTool returns true if this is an MCP tool entry.
func (t *ToolAvailableEntry) IsMCPTool() bool {
	return t.Type == "mcp"
}
