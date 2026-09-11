package xaioauth

import (
	"errors"
	"net/http"
	"strings"
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
//
// maxOutputTokens is the top-level max_output_tokens from the user's config;
// zero keeps DefaultMaxOutputTokens. It is a parameter rather than a field on
// XAIOAuthConfig because the ceiling is a property of the user's setup, not
// of the OAuth engine, and every other provider reads the same top-level key.
func NewClientFromUserConfig(uc *config.XAIOAuthConfig, maxOutputTokens int) *Client {
	cfg := DefaultConfig()
	cfg.ImportGrokAuth = true
	if maxOutputTokens > 0 {
		cfg.MaxOutputTokens = maxOutputTokens
	}
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

// ErrModelNotSet is returned by every request when xai_oauth.model is empty.
// Token import, login and credential handling still work without a model;
// inference does not, and no model is invented in its place.
var ErrModelNotSet = errors.New("xai_oauth.model is not set in .nerd/config.json; SuperGrok OAuth needs a model id (see https://api.x.ai/v1/models)")

// requestModel returns the model a request may be sent with, or
// ErrModelNotSet.
func requestModel(model string) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", ErrModelNotSet
	}
	return model, nil
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
