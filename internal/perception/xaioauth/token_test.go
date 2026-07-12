package xaioauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestTokenSource_Refresh(t *testing.T) {
	var refreshHits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"issuer":                        srvURLPlaceholder(r),
				"token_endpoint":                "http://" + r.Host + "/oauth2/token",
				"device_authorization_endpoint": "http://" + r.Host + "/oauth2/device/code",
			})
		case r.URL.Path == "/oauth2/token":
			refreshHits++
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" {
				t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "new-access",
				"refresh_token": "new-refresh",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Fix discovery issuer to the test server
	dir := t.TempDir()
	credPath := filepath.Join(dir, "creds.json")
	expired := &Credentials{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		ExpiresAt:    time.Now().Add(-time.Minute),
		ClientID:     DefaultClientID,
		Issuer:       srv.URL,
	}
	if err := SaveCredentials(credPath, expired); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Issuer:         srv.URL,
		ClientID:       DefaultClientID,
		CredentialPath: credPath,
		ImportGrokAuth: false,
		Timeout:        time.Minute,
		BaseURL:        DefaultBaseURL,
	}
	ts := NewTokenSource(cfg, srv.Client())
	if err := ts.Load(); err != nil {
		t.Fatal(err)
	}

	token, err := ts.AccessToken(context.Background())
	if err != nil {
		t.Fatalf("AccessToken: %v", err)
	}
	if token != "new-access" {
		t.Errorf("token = %q", token)
	}
	if refreshHits != 1 {
		t.Errorf("refreshHits = %d", refreshHits)
	}

	// Loaded from disk should have new tokens
	loaded, err := LoadCredentials(credPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != "new-access" {
		t.Errorf("persisted access = %q", loaded.AccessToken)
	}
}

// srvURLPlaceholder avoids capturing srv before assignment in the handler closure init.
func srvURLPlaceholder(r *http.Request) string {
	return "http://" + r.Host
}
