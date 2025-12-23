package config

import "codenerd/internal/mcp"

// IntegrationsConfig configures external MCP service integrations.
// Uses a dynamic map to support arbitrary MCP servers without code changes.
type IntegrationsConfig struct {
	// Servers is a map of server ID to server configuration.
	// Server IDs are arbitrary strings (e.g., "code_graph", "browser", "my_custom_server").
	Servers map[string]MCPServerIntegration `yaml:"servers" json:"servers,omitempty"`
}

// MCPServerIntegration configures a single MCP server integration.
type MCPServerIntegration struct {
	Enabled           bool   `yaml:"enabled" json:"enabled,omitempty"`
	Protocol          string `yaml:"protocol" json:"protocol,omitempty"`           // http, stdio, sse
	BaseURL           string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout           string `yaml:"timeout" json:"timeout,omitempty"`             // e.g., "30s", "2m"
	AutoConnect       bool   `yaml:"auto_connect" json:"auto_connect,omitempty"`
	AutoDiscoverTools bool   `yaml:"auto_discover_tools" json:"auto_discover_tools,omitempty"`
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

		configs[serverID] = mcp.MCPServerConfig{
			ID:                serverID,
			Enabled:           true,
			Protocol:          protocol,
			BaseURL:           server.BaseURL,
			Timeout:           timeout,
			AutoConnect:       server.AutoConnect,
			AutoDiscoverTools: server.AutoDiscoverTools,
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
