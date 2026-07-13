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
// Quarantined local tokens are NOT sticky when a fresher Grok CLI import is available:
// re-import wins over stuck/revoked quarantined credentials.
func (ts *TokenSource) Load() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	creds, err := LoadCredentials(ts.cfg.CredentialPath)
	if err == nil && creds != nil && !creds.Quarantined {
		ts.creds = creds
		return nil
	}

	// Local missing or quarantined: prefer a successful Grok CLI re-import.
	if ts.cfg.ImportGrokAuth {
		if imported := ts.importGrokIfBetterLocked(creds); imported != nil {
			ts.creds = imported
			return nil
		}
	}

	if err == nil && creds != nil && creds.Quarantined {
		ts.creds = creds
		detail := creds.QuarantineReason
		if detail == "" {
			detail = "credentials quarantined after terminal refresh failure"
		}
		return &AuthRequiredError{Detail: detail}
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

// ClearQuarantineAndReload drops quarantined local tokens when a re-import is possible,
// then reloads. Safe to call from auth CLI / recovery paths.
func (ts *TokenSource) ClearQuarantineAndReload() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.cfg.ImportGrokAuth {
		if imported := ts.importGrokIfBetterLocked(ts.creds); imported != nil {
			ts.creds = imported
			return nil
		}
	}

	// No better import: if still quarantined, surface a clear error.
	if ts.creds != nil && ts.creds.Quarantined {
		detail := ts.creds.QuarantineReason
		if detail == "" {
			detail = "credentials quarantined"
		}
		return &AuthRequiredError{Detail: detail}
	}
	return nil
}

// PrepareForReauth deletes the local credential store and clears in-memory tokens
// so the next Load can re-import from Grok CLI or a fresh device login can proceed
// without stuck quarantined tokens blocking recovery.
func (ts *TokenSource) PrepareForReauth() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	path := ts.cfg.CredentialPath
	if path == "" {
		path = DefaultCredentialPath()
	}
	if err := DeleteCredentials(path); err != nil {
		return err
	}
	ts.creds = nil
	ts.discovery = nil
	return nil
}

// AccessToken returns a valid access token, refreshing if near expiry.
func (ts *TokenSource) AccessToken(ctx context.Context) (string, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if ts.creds == nil {
		return "", ErrNoCredentials
	}
	if ts.creds.Quarantined {
		// Last chance: re-import from Grok CLI may have fresher tokens.
		if ts.cfg.ImportGrokAuth {
			if imported := ts.importGrokIfBetterLocked(ts.creds); imported != nil {
				ts.creds = imported
			}
		}
		if ts.creds.Quarantined {
			return "", &AuthRequiredError{Detail: ts.creds.QuarantineReason}
		}
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

	if err := ts.refreshLocked(ctx, true); err != nil {
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

// refreshLocked performs refresh_token grant. allowReimport controls one-shot
// Grok CLI re-import after terminal invalid_grant (avoids infinite loops).
func (ts *TokenSource) refreshLocked(ctx context.Context, allowReimport bool) error {
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
		prevRefresh := ""
		if ts.creds != nil {
			prevRefresh = ts.creds.RefreshToken
		}
		ts.creds.Quarantined = true
		ts.creds.QuarantineReason = detail
		_ = SaveCredentials(ts.cfg.CredentialPath, ts.creds)

		// Prefer successful re-import over stuck quarantined tokens.
		if allowReimport && ts.cfg.ImportGrokAuth {
			if imported := ts.importGrokIfBetterLocked(ts.creds); imported != nil {
				ts.creds = imported
				// If import only refreshed access token and no refresh needed, done.
				if ts.creds.AccessToken != "" && !ts.needsRefreshLocked() {
					return nil
				}
				// Different refresh token: retry refresh once without re-import.
				if ts.creds.RefreshToken != "" && ts.creds.RefreshToken != prevRefresh {
					return ts.refreshLocked(ctx, false)
				}
			}
		}

		return &AuthRequiredError{Detail: detail}
	}
	return &RefreshFailedError{Terminal: false, Detail: detail}
}

// importGrokIfBetterLocked tries Grok CLI auth import and persists it when it is
// preferable to local (missing, quarantined, or different refresh/access tokens).
// Caller must hold ts.mu. Returns imported creds or nil.
func (ts *TokenSource) importGrokIfBetterLocked(local *Credentials) *Credentials {
	imported, iErr := ImportGrokCLIAuth(ts.cfg.GrokAuthPath)
	if iErr != nil || imported == nil {
		return nil
	}
	if imported.AccessToken == "" && imported.RefreshToken == "" {
		return nil
	}

	// Prefer import when local is missing or quarantined.
	prefer := local == nil || local.Quarantined
	if !prefer && local != nil {
		// Also prefer when CLI has a different refresh token (user re-logged in).
		if imported.RefreshToken != "" && imported.RefreshToken != local.RefreshToken {
			prefer = true
		}
		// Or a different non-empty access token while local has none usable.
		if !prefer && local.AccessToken == "" && imported.AccessToken != "" {
			prefer = true
		}
	}
	if !prefer {
		return nil
	}

	// When local is quarantined for the same refresh token and import has the
	// same refresh token, only accept if import still has a non-expired access token
	// we can use without refresh — otherwise re-import would just fail the same way.
	if local != nil && local.Quarantined &&
		imported.RefreshToken != "" && imported.RefreshToken == local.RefreshToken {
		if imported.AccessToken == "" {
			return nil
		}
		if !imported.ExpiresAt.IsZero() && time.Now().Add(TokenRefreshSkew).After(imported.ExpiresAt) {
			// Access expired and refresh token is the same revoked one — useless.
			return nil
		}
	}

	imported.Quarantined = false
	imported.QuarantineReason = ""
	if imported.Source == "" {
		imported.Source = "grok_cli_import"
	}
	_ = SaveCredentials(ts.cfg.CredentialPath, imported)
	return imported
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

// HasUsableCredentials reports whether Load would succeed without network refresh.
// Used by client factory for API-key fallback decisions (no chat probe).
func (ts *TokenSource) HasUsableCredentials() bool {
	if err := ts.Load(); err != nil {
		return false
	}
	creds := ts.Credentials()
	return creds != nil && !creds.Quarantined && (creds.AccessToken != "" || creds.RefreshToken != "")
}
