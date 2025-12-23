package config

import "codenerd/internal/mcp"

// IntegrationsConfig configures external service integrations.
type IntegrationsConfig struct {
	// code-graph-mcp-server
	CodeGraph CodeGraphIntegration `yaml:"code_graph" json:"code_graph,omitempty"`

	// BrowserNERD
	Browser BrowserIntegration `yaml:"browser" json:"browser,omitempty"`

	// scraper_service
	Scraper ScraperIntegration `yaml:"scraper" json:"scraper,omitempty"`
}

// CodeGraphIntegration configures the code graph MCP server.
type CodeGraphIntegration struct {
	Enabled           bool   `yaml:"enabled" json:"enabled,omitempty"`
	Protocol          string `yaml:"protocol" json:"protocol,omitempty"`           // http, stdio, sse
	BaseURL           string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout           string `yaml:"timeout" json:"timeout,omitempty"`
	AutoConnect       bool   `yaml:"auto_connect" json:"auto_connect,omitempty"`
	AutoDiscoverTools bool   `yaml:"auto_discover_tools" json:"auto_discover_tools,omitempty"`
}

// BrowserIntegration configures BrowserNERD.
type BrowserIntegration struct {
	Enabled           bool   `yaml:"enabled" json:"enabled,omitempty"`
	Protocol          string `yaml:"protocol" json:"protocol,omitempty"`
	BaseURL           string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout           string `yaml:"timeout" json:"timeout,omitempty"`
	AutoConnect       bool   `yaml:"auto_connect" json:"auto_connect,omitempty"`
	AutoDiscoverTools bool   `yaml:"auto_discover_tools" json:"auto_discover_tools,omitempty"`
}

// ScraperIntegration configures the scraper service.
type ScraperIntegration struct {
	Enabled           bool   `yaml:"enabled" json:"enabled,omitempty"`
	Protocol          string `yaml:"protocol" json:"protocol,omitempty"`
	BaseURL           string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout           string `yaml:"timeout" json:"timeout,omitempty"`
	AutoConnect       bool   `yaml:"auto_connect" json:"auto_connect,omitempty"`
	AutoDiscoverTools bool   `yaml:"auto_discover_tools" json:"auto_discover_tools,omitempty"`
}

// ToMCPServerConfigs converts integrations config to MCP server configs.
func (c *IntegrationsConfig) ToMCPServerConfigs() map[string]mcp.MCPServerConfig {
	configs := make(map[string]mcp.MCPServerConfig)

	if c.CodeGraph.Enabled {
		protocol := c.CodeGraph.Protocol
		if protocol == "" {
			protocol = "http"
		}
		timeout := c.CodeGraph.Timeout
		if timeout == "" {
			timeout = "30s"
		}
		configs["code_graph"] = mcp.MCPServerConfig{
			ID:                "code_graph",
			Enabled:           true,
			Protocol:          protocol,
			BaseURL:           c.CodeGraph.BaseURL,
			Timeout:           timeout,
			AutoConnect:       c.CodeGraph.AutoConnect,
			AutoDiscoverTools: c.CodeGraph.AutoDiscoverTools,
		}
	}

	if c.Browser.Enabled {
		protocol := c.Browser.Protocol
		if protocol == "" {
			protocol = "http"
		}
		timeout := c.Browser.Timeout
		if timeout == "" {
			timeout = "60s"
		}
		configs["browser"] = mcp.MCPServerConfig{
			ID:                "browser",
			Enabled:           true,
			Protocol:          protocol,
			BaseURL:           c.Browser.BaseURL,
			Timeout:           timeout,
			AutoConnect:       c.Browser.AutoConnect,
			AutoDiscoverTools: c.Browser.AutoDiscoverTools,
		}
	}

	if c.Scraper.Enabled {
		protocol := c.Scraper.Protocol
		if protocol == "" {
			protocol = "http"
		}
		timeout := c.Scraper.Timeout
		if timeout == "" {
			timeout = "120s"
		}
		configs["scraper"] = mcp.MCPServerConfig{
			ID:                "scraper",
			Enabled:           true,
			Protocol:          protocol,
			BaseURL:           c.Scraper.BaseURL,
			Timeout:           timeout,
			AutoConnect:       c.Scraper.AutoConnect,
			AutoDiscoverTools: c.Scraper.AutoDiscoverTools,
		}
	}

	return configs
}
