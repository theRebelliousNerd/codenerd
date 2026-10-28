package config

import (
	"fmt"
	"strings"
	"time"

	"codenerd/internal/mcp"
)

// IntegrationsConfig configures external MCP service integrations.
// Uses a dynamic map to support arbitrary MCP servers without code changes.
type IntegrationsConfig struct {
	// Servers is a map of server ID to server configuration.
	// Server IDs are arbitrary strings (e.g., "code_graph", "browser", "my_custom_server").
	Servers map[string]MCPServerIntegration `yaml:"servers" json:"servers,omitempty"`

	// DefaultTimeout is the transport fallback when a server's own timeout
	// is missing, unparseable or non-positive (client.go Connect). The
	// per-server timeout key keeps its own absent-key defaults
	// (DefaultTimeout below); this is what an unusable value falls back
	// to, installed into the MCP package at load because that package
	// cannot import this one.
	DefaultTimeout string `yaml:"default_timeout" json:"default_timeout,omitempty"`
}

// MCPServerIntegration configures a single MCP server integration.
type MCPServerIntegration struct {
	Enabled           bool   `yaml:"enabled" json:"enabled,omitempty"`
	Protocol          string `yaml:"protocol" json:"protocol,omitempty"` // http, stdio, sse
	BaseURL           string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout           string `yaml:"timeout" json:"timeout,omitempty"` // e.g., "30s", "2m"
	AutoConnect       bool   `yaml:"auto_connect" json:"auto_connect,omitempty"`
	AutoDiscoverTools bool   `yaml:"auto_discover_tools" json:"auto_discover_tools,omitempty"`

	// Endpoint is the command line for a stdio server. Without it a stdio
	// server could be configured but never launched, because BaseURL only
	// describes HTTP and SSE transports.
	Endpoint string `yaml:"endpoint" json:"endpoint,omitempty"`

	// Headers are attached to every HTTP and SSE request. Values support
	// ${ENV_VAR} expansion so a token can live in the environment rather than
	// in config.json; an unset variable drops the header instead of sending
	// the literal placeholder as a credential.
	Headers map[string]string `yaml:"headers" json:"headers,omitempty"`

	// Reviews admit this server's tools. Discovery only describes a tool;
	// a remote call is dispatched only for a tool with a host review whose
	// schema hash still matches what the server advertises. `nerd mcp tools`
	// prints each tool's schema hash and whether it is reviewed.
	Reviews []MCPToolReview `yaml:"reviews" json:"reviews,omitempty"`
}

// MCPToolReview is one operator review of a remote tool's effect.
type MCPToolReview struct {
	// Tool is the tool's name on this server (not server/name).
	Tool string `yaml:"tool" json:"tool"`
	// Effect is what a call does: read, write, delete or execute. delete
	// and execute still need a person to approve each exact request.
	Effect string `yaml:"effect" json:"effect"`
	// SchemaHash pins the reviewed schema (`nerd mcp tools`). A tool whose
	// advertised schema changes stops matching until it is reviewed again.
	SchemaHash string `yaml:"schema_hash" json:"schema_hash"`
	// TargetArgument names the argument holding the affected resource, when
	// the effect has one (a path, an id); policy sees its value.
	TargetArgument string `yaml:"target_argument" json:"target_argument,omitempty"`
	// RiskOverride replaces the inferred risk class (safe, mutating,
	// destructive, arbitrary) when the operator knows better.
	RiskOverride string `yaml:"risk_override" json:"risk_override,omitempty"`
}

func (r MCPToolReview) toEffect(serverID string) mcp.ReviewedToolEffect {
	return mcp.ReviewedToolEffect{
		Operation:      mcp.RemoteTool,
		ToolID:         serverID + "/" + strings.TrimSpace(r.Tool),
		SchemaHash:     strings.TrimSpace(r.SchemaHash),
		Effect:         mcp.RemoteEffect(strings.TrimSpace(r.Effect)),
		TargetArgument: strings.TrimSpace(r.TargetArgument),
		RiskOverride:   mcp.RiskClass(strings.TrimSpace(r.RiskOverride)),
	}
}

// DefaultTimeout returns a sensible default timeout based on server ID.
func DefaultTimeout(serverID string) string {
	switch serverID {
	case "scraper":
		return "120s"
	case "browser":
		return "60s"
	default:
		return "30s"
	}
}

// Check reports the contradictions in an integrations section, addressed
// under prefix ("integrations"): every configured review, and the
// section-level timeout fallback. A per-server timeout that is missing or
// unusable falls back at connect time, and refusing the file for it would
// newly refuse files that load today.
func (c IntegrationsConfig) Check(prefix string) []Problem {
	return append(c.reviewProblems(prefix), c.timeoutProblems(prefix)...)
}

// reviewProblems checks every server's configured reviews with the same
// normalization installation applies.
func (c IntegrationsConfig) reviewProblems(prefix string) []Problem {
	var problems []Problem
	for serverID, server := range c.Servers {
		for i, review := range server.Reviews {
			if strings.TrimSpace(review.Tool) == "" {
				problems = append(problems, Problem{Severity: SeverityError,
					Path:    fmt.Sprintf("%s.servers.%s.reviews[%d].tool", prefix, serverID, i),
					Message: "a review names no tool", Fix: "the tool's name on this server, as `nerd mcp tools` lists it"})
				continue
			}
			if _, err := mcp.NormalizeReview(review.toEffect(serverID)); err != nil {
				problems = append(problems, Problem{Severity: SeverityError,
					Path:    fmt.Sprintf("%s.servers.%s.reviews[%d]", prefix, serverID, i),
					Message: err.Error(),
					Fix:     "effect is read, write, delete or execute; schema_hash is the hash `nerd mcp tools` prints; risk_override, if set, is safe, mutating, destructive or arbitrary"})
			}
		}
	}
	return problems
}

func (c IntegrationsConfig) timeoutProblems(prefix string) []Problem {
	timeout := c.DefaultTimeout
	if timeout == "" {
		timeout = DefaultIntegrationsConfig().DefaultTimeout
	}
	var problems []Problem
	d, err := time.ParseDuration(timeout)
	switch {
	case err != nil:
		return append(problems, Problem{
			Severity: SeverityError,
			Path:     prefix + ".default_timeout",
			Message:  fmt.Sprintf("%q is not a duration: %v", timeout, err),
			Fix:      `a Go duration such as "30s" or "2m"`,
		})
	case d <= 0:
		return append(problems, Problem{
			Severity: SeverityError,
			Path:     prefix + ".default_timeout",
			Message:  fmt.Sprintf("%q is not positive", timeout),
			Fix:      "a positive duration, or remove the key for the default",
		})
	}
	return problems
}

// ResolveDefaultTimeout defaults, checks and parses the transport
// fallback. LoadUserConfig is the production caller; it installs
// the result into the MCP package.
func (c IntegrationsConfig) ResolveDefaultTimeout() (time.Duration, error) {
	v := c
	if v.DefaultTimeout == "" {
		v.DefaultTimeout = DefaultIntegrationsConfig().DefaultTimeout
	}
	if problems := v.timeoutProblems("integrations"); len(problems) > 0 {
		return 0, fmt.Errorf("%s", problems[0].String())
	}
	d, _ := time.ParseDuration(v.DefaultTimeout) // Check parsed it
	return d, nil
}

// ToMCPServerConfigs converts integrations config to MCP server configs.
func (c *IntegrationsConfig) ToMCPServerConfigs() map[string]mcp.MCPServerConfig {
	configs := make(map[string]mcp.MCPServerConfig)

	if c.Servers == nil {
		return configs
	}

	for serverID, server := range c.Servers {
		if !server.Enabled {
			continue
		}

		protocol := server.Protocol
		if protocol == "" {
			protocol = "http"
		}

		timeout := server.Timeout
		if timeout == "" {
			timeout = DefaultTimeout(serverID)
		}

		var reviews []mcp.ReviewedToolEffect
		for _, review := range server.Reviews {
			reviews = append(reviews, review.toEffect(serverID))
		}
		configs[serverID] = mcp.MCPServerConfig{
			ID:                serverID,
			Enabled:           true,
			Protocol:          protocol,
			BaseURL:           server.BaseURL,
			Timeout:           timeout,
			AutoConnect:       server.AutoConnect,
			AutoDiscoverTools: server.AutoDiscoverTools,
			// Endpoint and Headers were absent from this mapping, so a stdio
			// server could be configured but never launched, and per-server
			// auth headers were unreachable from config.json entirely.
			Endpoint: server.Endpoint,
			Headers:  server.Headers,
			Reviews:  reviews,
		}
	}

	return configs
}

// GetServer returns the configuration for a specific server, or nil if not found.
func (c *IntegrationsConfig) GetServer(serverID string) *MCPServerIntegration {
	if c.Servers == nil {
		return nil
	}
	if server, ok := c.Servers[serverID]; ok {
		return &server
	}
	return nil
}

// IsServerEnabled returns true if the specified server is configured and enabled.
func (c *IntegrationsConfig) IsServerEnabled(serverID string) bool {
	server := c.GetServer(serverID)
	return server != nil && server.Enabled
}

// DefaultIntegrationsConfig returns an IntegrationsConfig with sensible defaults.
func DefaultIntegrationsConfig() *IntegrationsConfig {
	return &IntegrationsConfig{
		DefaultTimeout: "30s",
		Servers: map[string]MCPServerIntegration{
			"my_mcp_server": {
				Enabled:           false,
				Protocol:          "http",
				BaseURL:           "http://localhost:8000",
				Timeout:           "30s",
				AutoConnect:       true,
				AutoDiscoverTools: true,
			},
		},
	}
}
