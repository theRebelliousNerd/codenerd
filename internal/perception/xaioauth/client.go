package xaioauth

import (
	"net/http"
	"sync"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// Client is a SuperGrok OAuth LLM backend. Independent of perception.XAIClient.
type Client struct {
	cfg        Config
	httpClient *http.Client
	tokens     *TokenSource

	mu          sync.Mutex
	lastRequest time.Time
}

// NewClient builds a Client from package Config.
func NewClient(cfg Config) *Client {
	cfg = cfg.ApplyDefaults()
	// ImportGrokAuth default true when not set via FromUserConfig
	httpClient := &http.Client{Timeout: cfg.Timeout}
	// Prefer pooled transport when available from perception package — use std client
	// to keep xaioauth free of init cycles; factory can inject later if needed.
	ts := NewTokenSource(cfg, &http.Client{Timeout: 30 * time.Second})
	_ = ts.Load() // best-effort; errors surface on first request
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		tokens:     ts,
	}
}

// NewClientFromUserConfig maps codeNERD config.XAIOAuthConfig into a Client.
func NewClientFromUserConfig(uc *config.XAIOAuthConfig) *Client {
	cfg := DefaultConfig()
	cfg.ImportGrokAuth = true
	if uc != nil {
		if uc.Model != "" {
			cfg.Model = uc.Model
		}
		if uc.FallbackModel != "" {
			cfg.FallbackModel = uc.FallbackModel
		}
		if uc.Timeout > 0 {
			cfg.Timeout = time.Duration(uc.Timeout) * time.Second
		}
		if uc.BaseURL != "" {
			cfg.BaseURL = uc.BaseURL
		}
		if uc.AuthURL != "" {
			cfg.Issuer = uc.AuthURL
		}
		if uc.CredentialPath != "" {
			cfg.CredentialPath = uc.CredentialPath
		}
		if uc.ImportGrokAuth != nil {
			cfg.ImportGrokAuth = *uc.ImportGrokAuth
		}
		if uc.GrokAuthPath != "" {
			cfg.GrokAuthPath = uc.GrokAuthPath
		}
		if uc.MaxConcurrentCalls > 0 {
			cfg.MaxConcurrentCalls = uc.MaxConcurrentCalls
		}
	}
	c := NewClient(cfg)
	logging.Perception("xai-oauth client ready: model=%s import_grok=%v", c.cfg.Model, c.cfg.ImportGrokAuth)
	return c
}

// Config returns a copy of the runtime config.
func (c *Client) Config() Config {
	return c.cfg
}

// TokenSource exposes the token source for auth/probe flows.
func (c *Client) TokenSource() *TokenSource {
	return c.tokens
}

// SetModel updates the primary model.
func (c *Client) SetModel(model string) {
	if model != "" {
		c.cfg.Model = model
	}
}

// GetModel returns the primary model id.
func (c *Client) GetModel() string {
	return c.cfg.Model
}

// rateLimitPace applies a small client-side gap between requests.
func (c *Client) rateLimitPace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
}
