package config

// OpenRouterProviderConfig holds OpenRouter request options.
type OpenRouterProviderConfig struct {
	// SiteURL and AppTitle are OpenRouter's app attribution: sent as the
	// HTTP-Referer and X-Title headers when set, so the app is credited in
	// OpenRouter's rankings. Unset, neither header is sent.
	SiteURL  string `json:"site_url,omitempty"`
	AppTitle string `json:"app_title,omitempty"`
}

// GetOpenRouterConfig returns the OpenRouter section, or nil when absent.
func (c *UserConfig) GetOpenRouterConfig() *OpenRouterProviderConfig {
	if c == nil {
		return nil
	}
	return c.OpenRouter
}
