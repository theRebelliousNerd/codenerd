package config

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
	Enabled bool   `yaml:"enabled" json:"enabled,omitempty"`
	BaseURL string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout string `yaml:"timeout" json:"timeout,omitempty"`
}

// BrowserIntegration configures BrowserNERD.
type BrowserIntegration struct {
	Enabled bool   `yaml:"enabled" json:"enabled,omitempty"`
	BaseURL string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout string `yaml:"timeout" json:"timeout,omitempty"`
}

// ScraperIntegration configures the scraper service.
type ScraperIntegration struct {
	Enabled bool   `yaml:"enabled" json:"enabled,omitempty"`
	BaseURL string `yaml:"base_url" json:"base_url,omitempty"`
	Timeout string `yaml:"timeout" json:"timeout,omitempty"`
}
