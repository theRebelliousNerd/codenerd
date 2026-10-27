package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ProviderAPIKey names one credential. Exactly one of Env/APIKey is required.
// Pools replace the provider's legacy scalar key when explicitly configured.
type ProviderAPIKey struct {
	Name        string `json:"name"`
	Env         string `json:"env,omitempty"`
	APIKey      string `json:"api_key,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

// ResolvedProviderAPIKey is runtime-only; never marshal it into diagnostics.
type ResolvedProviderAPIKey struct {
	Name        string
	Key         string
	WorkspaceID string
}

var credentialAliasPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
var credentialEnvPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func (c *UserConfig) CheckAPIKeyPools() error {
	for provider, entries := range c.APIKeyPools {
		switch provider {
		case "anthropic", "openai", "gemini", "xai", "zai", "openrouter", "dashscope", "meta", "moonshot":
		default:
			return fmt.Errorf("api_key_pools: unsupported API provider %q", provider)
		}
		if len(entries) == 0 || len(entries) > 64 {
			return fmt.Errorf("api_key_pools.%s must contain between 1 and 64 credentials", provider)
		}
		names := map[string]bool{}
		for _, entry := range entries {
			if !credentialAliasPattern.MatchString(entry.Name) || names[entry.Name] {
				return fmt.Errorf("api_key_pools.%s requires unique credential names of 1-64 letters, digits, underscores or hyphens", provider)
			}
			names[entry.Name] = true
			if (entry.Env == "") == (entry.APIKey == "") {
				return fmt.Errorf("api_key_pools.%s.%s requires exactly one of env or api_key", provider, entry.Name)
			}
			if entry.Env != "" && !credentialEnvPattern.MatchString(entry.Env) {
				return fmt.Errorf("api_key_pools.%s.%s has an invalid environment variable name", provider, entry.Name)
			}
			if strings.TrimSpace(entry.APIKey) == "" && entry.Env == "" {
				return fmt.Errorf("api_key_pools.%s.%s has an empty key", provider, entry.Name)
			}
			if entry.WorkspaceID != "" && provider != "anthropic" {
				return fmt.Errorf("api_key_pools.%s.%s: workspace_id is supported only for Anthropic", provider, entry.Name)
			}
			if strings.ContainsAny(entry.WorkspaceID, "\r\n") {
				return fmt.Errorf("api_key_pools.%s.%s has an invalid workspace_id", provider, entry.Name)
			}
		}
	}
	return nil
}

// ResolveAPIKeyPool fails on missing environment references and duplicates;
// explicit configuration never quietly falls back to a scalar or another key.
func (c *UserConfig) ResolveAPIKeyPool(provider string) ([]ResolvedProviderAPIKey, error) {
	if c == nil {
		return nil, nil
	}
	if err := c.CheckAPIKeyPools(); err != nil {
		return nil, err
	}
	entries := c.APIKeyPools[strings.ToLower(strings.TrimSpace(provider))]
	out := make([]ResolvedProviderAPIKey, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		key := entry.APIKey
		if entry.Env != "" {
			key = os.Getenv(entry.Env)
		}
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("api_key_pools.%s.%s: credential is unavailable; set its configured environment variable", provider, entry.Name)
		}
		if strings.TrimSpace(key) != key || strings.ContainsAny(key, "\r\n\x00") {
			return nil, fmt.Errorf("api_key_pools.%s.%s: credential has invalid whitespace or control characters", provider, entry.Name)
		}
		if seen[key] {
			return nil, fmt.Errorf("api_key_pools.%s contains duplicate credentials", provider)
		}
		seen[key] = true
		out = append(out, ResolvedProviderAPIKey{Name: entry.Name, Key: key, WorkspaceID: entry.WorkspaceID})
	}
	return out, nil
}
