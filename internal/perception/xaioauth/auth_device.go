package xaioauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeviceCodeResponse is the OAuth device authorization response.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceLoginResult is returned after a successful device-code exchange.
type DeviceLoginResult struct {
	Credentials *Credentials
	UserCode    string
	VerifyURL   string
}

// DiscoverOIDC fetches the OIDC discovery document from issuer/.well-known/openid-configuration.
func DiscoverOIDC(ctx context.Context, httpClient *http.Client, issuer string) (*DiscoveryDocument, error) {
	if issuer == "" {
		issuer = DefaultIssuer
	}
	endpoint := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("oidc discovery read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	var doc DiscoveryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("oidc discovery parse: %w", err)
	}
	if doc.TokenEndpoint == "" || doc.DeviceAuthorizationEndpoint == "" {
		return nil, fmt.Errorf("oidc discovery missing token or device endpoints")
	}
	return &doc, nil
}

// RequestDeviceCode starts the device authorization flow.
func RequestDeviceCode(ctx context.Context, httpClient *http.Client, deviceEndpoint, clientID, scopes string) (*DeviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", clientID)
	if scopes != "" {
		form.Set("scope", scopes)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("device code request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("device code read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("device code status %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	var out DeviceCodeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("device code parse: %w", err)
	}
	if out.DeviceCode == "" || out.UserCode == "" {
		return nil, fmt.Errorf("device code response missing device_code or user_code")
	}
	if out.Interval <= 0 {
		out.Interval = 5
	}
	return &out, nil
}

// tokenResponse is the OAuth token endpoint payload.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// PollDeviceToken polls the token endpoint until the user approves or the code expires.
func PollDeviceToken(ctx context.Context, httpClient *http.Client, tokenEndpoint, clientID, deviceCode string, interval time.Duration, expiresIn time.Duration) (*Credentials, error) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(expiresIn)
	if expiresIn <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, &AuthRequiredError{Detail: "device authorization timed out"}
		}

		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", deviceCode)
		form.Set("client_id", clientID)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("device token poll: %w", err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("device token read: %w", err)
		}

		var tr tokenResponse
		_ = json.Unmarshal(body, &tr)

		if resp.StatusCode == http.StatusOK && tr.AccessToken != "" {
			return credentialsFromTokenResponse(&tr, clientID, "device_code"), nil
		}

		switch tr.Error {
		case "authorization_pending":
			// keep polling
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token", "access_denied":
			return nil, &AuthRequiredError{Detail: tr.Error + ": " + tr.ErrorDesc}
		default:
			if tr.Error != "" {
				return nil, fmt.Errorf("device token error %q: %s", tr.Error, tr.ErrorDesc)
			}
			// Unexpected non-OK without structured error
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("device token status %d: %s", resp.StatusCode, truncate(string(body), 300))
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// LoginDeviceCode runs discovery → device code → poll → credentials.
// onCode is called once the user_code is available (print URL instructions).
func LoginDeviceCode(ctx context.Context, httpClient *http.Client, cfg Config, onCode func(DeviceCodeResponse)) (*Credentials, error) {
	cfg = cfg.ApplyDefaults()
	doc, err := DiscoverOIDC(ctx, httpClient, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	dc, err := RequestDeviceCode(ctx, httpClient, doc.DeviceAuthorizationEndpoint, cfg.ClientID, cfg.Scopes)
	if err != nil {
		return nil, err
	}
	if onCode != nil {
		onCode(*dc)
	}
	interval := time.Duration(dc.Interval) * time.Second
	expires := time.Duration(dc.ExpiresIn) * time.Second
	creds, err := PollDeviceToken(ctx, httpClient, doc.TokenEndpoint, cfg.ClientID, dc.DeviceCode, interval, expires)
	if err != nil {
		return nil, err
	}
	creds.Issuer = cfg.Issuer
	if creds.Issuer == "" && doc.Issuer != "" {
		creds.Issuer = doc.Issuer
	}
	return creds, nil
}

func credentialsFromTokenResponse(tr *tokenResponse, clientID, source string) *Credentials {
	expiresAt := time.Now().UTC().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if tr.ExpiresIn <= 0 {
		// JWT may embed exp; fall back to 1h
		expiresAt = time.Now().UTC().Add(time.Hour)
	}
	return &Credentials{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		TokenType:    tr.TokenType,
		ExpiresAt:    expiresAt,
		Scope:        tr.Scope,
		ClientID:     clientID,
		Source:       source,
		UpdatedAt:    time.Now().UTC(),
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
