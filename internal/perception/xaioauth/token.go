package xaioauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenSource resolves a valid access token, refreshing when needed.
type TokenSource struct {
	mu         sync.Mutex
	cfg        Config
	httpClient *http.Client
	creds      *Credentials
	discovery  *DiscoveryDocument
}

// NewTokenSource creates a token source. Call Load on it before EnsureValid if needed.
func NewTokenSource(cfg Config, httpClient *http.Client) *TokenSource {
	cfg = cfg.ApplyDefaults()
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &TokenSource{cfg: cfg, httpClient: httpClient}
}

// Load populates credentials from the codeNERD store, optionally importing Grok CLI auth.
func (ts *TokenSource) Load() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	creds, err := LoadCredentials(ts.cfg.CredentialPath)
	if err == nil && creds != nil {
		if creds.Quarantined {
			return &AuthRequiredError{Detail: creds.QuarantineReason}
		}
		ts.creds = creds
		return nil
	}

	if ts.cfg.ImportGrokAuth {
		imported, iErr := ImportGrokCLIAuth(ts.cfg.GrokAuthPath)
		if iErr == nil && imported != nil {
			// Persist into codeNERD store so refresh can update us without rewriting Grok CLI file.
			_ = SaveCredentials(ts.cfg.CredentialPath, imported)
			ts.creds = imported
			return nil
		}
	}

	if err != nil {
		return err
	}
	return ErrNoCredentials
}

// SetCredentials replaces in-memory credentials (e.g. after device login) and saves.
func (ts *TokenSource) SetCredentials(creds *Credentials) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if creds == nil {
		return fmt.Errorf("nil credentials")
	}
	creds.Quarantined = false
	creds.QuarantineReason = ""
	if err := SaveCredentials(ts.cfg.CredentialPath, creds); err != nil {
		return err
	}
	ts.creds = creds
	return nil
}

// Credentials returns a copy of the current credentials (may be nil).
func (ts *TokenSource) Credentials() *Credentials {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.creds == nil {
		return nil
	}
	cp := *ts.creds
	return &cp
}

// AccessToken returns a valid access token, refreshing if near expiry.
func (ts *TokenSource) AccessToken(ctx context.Context) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.creds == nil {
		return "", ErrNoCredentials
	}
	if ts.creds.Quarantined {
		return "", &AuthRequiredError{Detail: ts.creds.QuarantineReason}
	}

	if ts.creds.AccessToken != "" && !ts.needsRefreshLocked() {
		return ts.creds.AccessToken, nil
	}

	if ts.creds.RefreshToken == "" {
		if ts.creds.AccessToken != "" {
			// No refresh token; use access token until 401
			return ts.creds.AccessToken, nil
		}
		return "", &AuthRequiredError{Detail: "no refresh token"}
	}

	if err := ts.refreshLocked(ctx); err != nil {
		return "", err
	}
	return ts.creds.AccessToken, nil
}

func (ts *TokenSource) needsRefreshLocked() bool {
	if ts.creds.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().Add(TokenRefreshSkew).After(ts.creds.ExpiresAt)
}

func (ts *TokenSource) refreshLocked(ctx context.Context) error {
	doc, err := ts.ensureDiscoveryLocked(ctx)
	if err != nil {
		return err
	}

	clientID := ts.creds.ClientID
	if clientID == "" {
		clientID = ts.cfg.ClientID
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", ts.creds.RefreshToken)
	form.Set("client_id", clientID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.httpClient.Do(req)
	if err != nil {
		return &RefreshFailedError{Terminal: false, Detail: err.Error()}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return &RefreshFailedError{Terminal: false, Detail: err.Error()}
	}

	var tr tokenResponse
	_ = json.Unmarshal(body, &tr)

	if resp.StatusCode == http.StatusOK && tr.AccessToken != "" {
		prevRefresh := ts.creds.RefreshToken
		updated := credentialsFromTokenResponse(&tr, clientID, "refresh")
		if updated.RefreshToken == "" {
			updated.RefreshToken = prevRefresh
		}
		updated.Issuer = ts.creds.Issuer
		if updated.Issuer == "" {
			updated.Issuer = ts.cfg.Issuer
		}
		if err := SaveCredentials(ts.cfg.CredentialPath, updated); err != nil {
			return err
		}
		ts.creds = updated
		return nil
	}

	terminal := resp.StatusCode >= 400 && resp.StatusCode < 500
	detail := tr.Error
	if tr.ErrorDesc != "" {
		detail = detail + ": " + tr.ErrorDesc
	}
	if detail == "" {
		detail = fmt.Sprintf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	if terminal {
		ts.creds.Quarantined = true
		ts.creds.QuarantineReason = detail
		_ = SaveCredentials(ts.cfg.CredentialPath, ts.creds)
		return &AuthRequiredError{Detail: detail}
	}
	return &RefreshFailedError{Terminal: false, Detail: detail}
}

func (ts *TokenSource) ensureDiscoveryLocked(ctx context.Context) (*DiscoveryDocument, error) {
	if ts.discovery != nil {
		return ts.discovery, nil
	}
	issuer := ts.cfg.Issuer
	if ts.creds != nil && ts.creds.Issuer != "" {
		issuer = ts.creds.Issuer
	}
	doc, err := DiscoverOIDC(ctx, ts.httpClient, issuer)
	if err != nil {
		return nil, err
	}
	ts.discovery = doc
	return doc, nil
}

// InvalidateAccess clears the access token so the next call forces refresh.
func (ts *TokenSource) InvalidateAccess() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.creds != nil {
		ts.creds.AccessToken = ""
		ts.creds.ExpiresAt = time.Time{}
	}
}
