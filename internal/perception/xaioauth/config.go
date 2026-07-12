package xaioauth

import (
	"os"
	"path/filepath"
	"time"
)

const (
	// EngineID is the codeNERD engine string for SuperGrok OAuth.
	EngineID = "xai-oauth"

	// DefaultModel is Grok 4.5 — the flagship model and current Grok Build default.
	// API id confirmed via https://api.x.ai/v1/models under SuperGrok OAuth.
	DefaultModel = "grok-4.5"

	// DefaultBaseURL is the xAI OpenAI-compatible API root.
	DefaultBaseURL = "https://api.x.ai/v1"

	// DefaultIssuer is the OIDC issuer for SuperGrok OAuth.
	DefaultIssuer = "https://auth.x.ai"

	// DefaultClientID is the public OIDC client used by the official Grok CLI.
	// token_endpoint_auth_methods_supported includes "none" for public clients.
	DefaultClientID = "b1a00492-073a-47ea-816f-4c329264a828"

	// DefaultScopes mirror Grok CLI subscription access.
	DefaultScopes = "openid profile email offline_access grok-cli:access api:access conversations:read conversations:write"

	// DefaultTimeout is the HTTP/client timeout for long coding prompts.
	DefaultTimeout = 5 * time.Minute

	// DefaultMaxConcurrentCalls keeps subscription usage conservative.
	DefaultMaxConcurrentCalls = 2

	// DefaultCredentialFile is the relative filename under the user home/.nerd dir.
	DefaultCredentialFile = "xai_oauth.json"

	// TokenRefreshSkew refreshes slightly before expiry.
	TokenRefreshSkew = 2 * time.Minute
)

// Config holds runtime settings for the SuperGrok OAuth client.
// Independent of perception.XAIConfig / API-key path.
type Config struct {
	Model              string
	FallbackModel      string
	Timeout            time.Duration
	BaseURL            string
	Issuer             string
	ClientID           string
	Scopes             string
	CredentialPath     string
	ImportGrokAuth     bool
	GrokAuthPath       string
	MaxConcurrentCalls int
}

// DefaultConfig returns sensible SuperGrok OAuth defaults.
func DefaultConfig() Config {
	return Config{
		Model:              DefaultModel,
		Timeout:            DefaultTimeout,
		BaseURL:            DefaultBaseURL,
		Issuer:             DefaultIssuer,
		ClientID:           DefaultClientID,
		Scopes:             DefaultScopes,
		CredentialPath:     DefaultCredentialPath(),
		ImportGrokAuth:     true,
		GrokAuthPath:       DefaultGrokAuthPath(),
		MaxConcurrentCalls: DefaultMaxConcurrentCalls,
	}
}

// ApplyDefaults fills empty fields from DefaultConfig.
func (c Config) ApplyDefaults() Config {
	d := DefaultConfig()
	if c.Model == "" {
		c.Model = d.Model
	}
	if c.Timeout <= 0 {
		c.Timeout = d.Timeout
	}
	if c.BaseURL == "" {
		c.BaseURL = d.BaseURL
	}
	if c.Issuer == "" {
		c.Issuer = d.Issuer
	}
	if c.ClientID == "" {
		c.ClientID = d.ClientID
	}
	if c.Scopes == "" {
		c.Scopes = d.Scopes
	}
	if c.CredentialPath == "" {
		c.CredentialPath = d.CredentialPath
	}
	// ImportGrokAuth: zero value is false; callers that want default true should
	// use DefaultConfig or set the pointer-style config from user_config.
	if c.GrokAuthPath == "" {
		c.GrokAuthPath = d.GrokAuthPath
	}
	if c.MaxConcurrentCalls <= 0 {
		c.MaxConcurrentCalls = d.MaxConcurrentCalls
	}
	return c
}

// DefaultCredentialPath returns ~/.nerd/xai_oauth.json.
func DefaultCredentialPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return DefaultCredentialFile
	}
	return filepath.Join(home, ".nerd", DefaultCredentialFile)
}

// DefaultGrokAuthPath returns ~/.grok/auth.json.
func DefaultGrokAuthPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".grok", "auth.json")
	}
	return filepath.Join(home, ".grok", "auth.json")
}

// DiscoveryDocument is the OIDC discovery payload we care about.
type DiscoveryDocument struct {
	Issuer                            string `json:"issuer"`
	AuthorizationEndpoint             string `json:"authorization_endpoint"`
	DeviceAuthorizationEndpoint       string `json:"device_authorization_endpoint"`
	TokenEndpoint                     string `json:"token_endpoint"`
	UserinfoEndpoint                  string `json:"userinfo_endpoint"`
	RevocationEndpoint                string `json:"revocation_endpoint"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
}

// Credentials is the persisted SuperGrok OAuth token set (codeNERD store format).
type Credentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
	Issuer       string    `json:"issuer,omitempty"`
	Source       string    `json:"source,omitempty"` // "device_code" | "grok_cli_import" | "refresh"
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	// Quarantined is set when refresh fails with a terminal error.
	Quarantined bool   `json:"quarantined,omitempty"`
	QuarantineReason string `json:"quarantine_reason,omitempty"`
}
