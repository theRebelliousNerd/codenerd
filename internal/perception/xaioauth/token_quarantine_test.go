package xaioauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoad_QuarantinedSameToken_FailsWithAuthRequired(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "xai_oauth.json")
	dead := &Credentials{
		AccessToken:      "dead-access",
		RefreshToken:     "dead-refresh",
		ExpiresAt:        time.Now().Add(-time.Hour),
		ClientID:         DefaultClientID,
		Quarantined:      true,
		QuarantineReason: "invalid_grant: Refresh token has been revoked",
		Source:           "test",
	}
	if err := SaveCredentials(credPath, dead); err != nil {
		t.Fatal(err)
	}

	ts := NewTokenSource(Config{
		CredentialPath: credPath,
		ImportGrokAuth: false,
		Timeout:        time.Minute,
	}, nil)
	err := ts.Load()
	if err == nil {
		t.Fatal("expected error for quarantined credentials")
	}
	if !IsAuthRequired(err) {
		t.Fatalf("want IsAuthRequired, got %v", err)
	}
}

func TestLoad_QuarantinedReimportsDifferentGrokToken(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "xai_oauth.json")
	grokPath := filepath.Join(dir, "auth.json")

	dead := &Credentials{
		AccessToken:      "dead-access",
		RefreshToken:     "dead-refresh",
		ExpiresAt:        time.Now().Add(-time.Hour),
		Quarantined:      true,
		QuarantineReason: "invalid_grant: Refresh token has been revoked",
	}
	if err := SaveCredentials(credPath, dead); err != nil {
		t.Fatal(err)
	}

	// Official Grok CLI auth.json shape (issuer::client_id → entry).
	grokBlob := map[string]any{
		"https://auth.x.ai::" + DefaultClientID: map[string]any{
			"key":            "fresh-access",
			"refresh_token":  "fresh-refresh",
			"expires_at":     time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			"oidc_issuer":    DefaultIssuer,
			"oidc_client_id": DefaultClientID,
			"auth_mode":      "oauth",
		},
	}
	data, err := json.Marshal(grokBlob)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(grokPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	ts := NewTokenSource(Config{
		CredentialPath: credPath,
		GrokAuthPath:   grokPath,
		ImportGrokAuth: true,
		Timeout:        time.Minute,
	}, nil)

	if err := ts.Load(); err != nil {
		t.Fatalf("Load after Grok re-import: %v", err)
	}

	creds := ts.Credentials()
	if creds == nil || creds.Quarantined {
		t.Fatalf("expected recovered non-quarantined creds, got %+v", creds)
	}
	if creds.RefreshToken != "fresh-refresh" {
		t.Fatalf("refresh token = %q want fresh-refresh", creds.RefreshToken)
	}
	if creds.AccessToken != "fresh-access" {
		t.Fatalf("access token = %q", creds.AccessToken)
	}
}

func TestPrepareForReauth_DeletesStore(t *testing.T) {
	dir := t.TempDir()
	credPath := filepath.Join(dir, "xai_oauth.json")
	if err := SaveCredentials(credPath, &Credentials{AccessToken: "a", RefreshToken: "r"}); err != nil {
		t.Fatal(err)
	}
	ts := NewTokenSource(Config{CredentialPath: credPath}, nil)
	if err := ts.Load(); err != nil {
		t.Fatal(err)
	}
	if err := ts.PrepareForReauth(); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCredentials(credPath); err == nil {
		t.Fatal("expected file gone")
	}
	if ts.Credentials() != nil {
		t.Fatal("expected in-memory creds cleared")
	}
}

func TestRefresh_InvalidGrant_QuarantinesAndClearError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                        "http://" + r.Host,
				"token_endpoint":                "http://" + r.Host + "/oauth2/token",
				"device_authorization_endpoint": "http://" + r.Host + "/oauth2/device/code",
			})
		case "/oauth2/token":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":             "invalid_grant",
				"error_description": "Refresh token has been revoked",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, "creds.json")
	expired := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "revoked-refresh",
		ExpiresAt:    time.Now().Add(-time.Minute),
		ClientID:     DefaultClientID,
		Issuer:       srv.URL,
	}
	if err := SaveCredentials(credPath, expired); err != nil {
		t.Fatal(err)
	}

	ts := NewTokenSource(Config{
		Issuer:         srv.URL,
		ClientID:       DefaultClientID,
		CredentialPath: credPath,
		ImportGrokAuth: false,
		Timeout:        time.Minute,
	}, srv.Client())
	if err := ts.Load(); err != nil {
		t.Fatal(err)
	}

	_, err := ts.AccessToken(context.Background())
	if err == nil {
		t.Fatal("expected auth error after revoked refresh")
	}
	if !IsAuthRequired(err) {
		t.Fatalf("want IsAuthRequired, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "invalid_grant") && !strings.Contains(msg, "revoked") {
		t.Fatalf("error should mention invalid_grant/revoked: %s", msg)
	}
	if !strings.Contains(msg, "nerd auth grok") {
		t.Fatalf("error should tell user to run nerd auth grok: %s", msg)
	}

	loaded, lerr := LoadCredentials(credPath)
	if lerr != nil {
		t.Fatal(lerr)
	}
	if !loaded.Quarantined {
		t.Fatal("expected credentials quarantined on disk")
	}
	if !IsTerminalRefreshFailure(loaded.QuarantineReason) {
		t.Fatalf("quarantine reason = %q", loaded.QuarantineReason)
	}
}

func TestRefresh_InvalidGrant_ReimportsFreshGrokToken(t *testing.T) {
	var refreshHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                        "http://" + r.Host,
				"token_endpoint":                "http://" + r.Host + "/oauth2/token",
				"device_authorization_endpoint": "http://" + r.Host + "/oauth2/device/code",
			})
		case "/oauth2/token":
			refreshHits++
			_ = r.ParseForm()
			rt := r.Form.Get("refresh_token")
			if rt == "revoked-refresh" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":             "invalid_grant",
					"error_description": "Refresh token has been revoked",
				})
				return
			}
			if rt == "fresh-refresh" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token":  "new-access-after-reimport",
					"refresh_token": "fresh-refresh-rotated",
					"token_type":    "Bearer",
					"expires_in":    3600,
				})
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unknown"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, "creds.json")
	grokPath := filepath.Join(dir, "auth.json")

	expired := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "revoked-refresh",
		ExpiresAt:    time.Now().Add(-time.Minute),
		ClientID:     DefaultClientID,
		Issuer:       srv.URL,
	}
	if err := SaveCredentials(credPath, expired); err != nil {
		t.Fatal(err)
	}

	// Fresh Grok CLI tokens available for re-import after invalid_grant.
	// Access token is also expired so refresh of the new token is required.
	grokBlob := map[string]any{
		"https://auth.x.ai::" + DefaultClientID: map[string]any{
			"key":            "cli-access-expired",
			"refresh_token":  "fresh-refresh",
			"expires_at":     time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
			"oidc_issuer":    srv.URL,
			"oidc_client_id": DefaultClientID,
			"auth_mode":      "oauth",
		},
	}
	data, err := json.Marshal(grokBlob)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(grokPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	ts := NewTokenSource(Config{
		Issuer:         srv.URL,
		ClientID:       DefaultClientID,
		CredentialPath: credPath,
		GrokAuthPath:   grokPath,
		ImportGrokAuth: true,
		Timeout:        time.Minute,
	}, srv.Client())
	if err := ts.Load(); err != nil {
		t.Fatal(err)
	}

	token, err := ts.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken after reimport path: %v", err)
	}
	if token != "new-access-after-reimport" {
		t.Fatalf("token = %q", token)
	}
	if refreshHits < 2 {
		t.Fatalf("expected refresh then reimport refresh, hits=%d", refreshHits)
	}

	loaded, err := LoadCredentials(credPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Quarantined {
		t.Fatal("should not remain quarantined after successful reimport")
	}
	if loaded.AccessToken != "new-access-after-reimport" {
		t.Fatalf("persisted access = %q", loaded.AccessToken)
	}
}

func TestDeriveAuthStatus(t *testing.T) {
	ready := &ProbeResult{Classification: ProbeReady, Status: AuthStatusOK}
	if got := DeriveAuthStatus(ready, false, true); got != AuthStatusOK {
		t.Fatalf("got %s", got)
	}
	login := &ProbeResult{Classification: ProbeLoginRequired, Status: AuthStatusNeedsReauth}
	if got := DeriveAuthStatus(login, true, true); got != AuthStatusAPIFallback {
		t.Fatalf("want api_fallback, got %s", got)
	}
	if got := DeriveAuthStatus(login, false, true); got != AuthStatusNeedsReauth {
		t.Fatalf("want needs_reauth, got %s", got)
	}
	if got := DeriveAuthStatus(login, true, false); got != AuthStatusNeedsReauth {
		t.Fatalf("fallback disabled => needs_reauth, got %s", got)
	}
}

func TestAuthRequiredError_IncludesRecoveryHelp(t *testing.T) {
	err := &AuthRequiredError{Detail: "invalid_grant: Refresh token has been revoked"}
	msg := err.Error()
	if !strings.Contains(msg, "nerd auth grok") {
		t.Fatalf("missing nerd auth grok: %s", msg)
	}
	if !strings.Contains(msg, "xai_oauth.json") {
		t.Fatalf("missing quarantine clear path: %s", msg)
	}
}
