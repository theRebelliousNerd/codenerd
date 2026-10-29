package xaioauth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadCredentials(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "xai_oauth.json")

	creds := &Credentials{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().UTC().Add(time.Hour).Truncate(time.Second),
		ClientID:     DefaultClientID,
		Issuer:       DefaultIssuer,
		Source:       "device_code",
	}
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	loaded, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if loaded.AccessToken != "access-abc" {
		t.Errorf("AccessToken = %q", loaded.AccessToken)
	}
	if loaded.RefreshToken != "refresh-xyz" {
		t.Errorf("RefreshToken = %q", loaded.RefreshToken)
	}
	if loaded.ClientID != DefaultClientID {
		t.Errorf("ClientID = %q", loaded.ClientID)
	}
}

func TestLoadCredentials_Missing(t *testing.T) {
	_, err := LoadCredentials(filepath.Join(t.TempDir(), "missing.json"))
	if err != ErrNoCredentials {
		t.Fatalf("err = %v, want ErrNoCredentials", err)
	}
}

func TestImportGrokCLIAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	// Minimal Grok CLI shape: access token in "key"
	raw := `{
  "https://auth.x.ai::` + DefaultClientID + `": {
    "key": "jwt-access-token",
    "auth_mode": "oidc",
    "refresh_token": "refresh-token-value",
    "expires_at": "2099-01-01T00:00:00Z",
    "oidc_issuer": "https://auth.x.ai",
    "oidc_client_id": "` + DefaultClientID + `"
  }
}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	creds, err := ImportGrokCLIAuth(path)
	if err != nil {
		t.Fatalf("ImportGrokCLIAuth: %v", err)
	}
	if creds.AccessToken != "jwt-access-token" {
		t.Errorf("AccessToken = %q", creds.AccessToken)
	}
	if creds.RefreshToken != "refresh-token-value" {
		t.Errorf("RefreshToken = %q", creds.RefreshToken)
	}
	if creds.Source != "grok_cli_import" {
		t.Errorf("Source = %q", creds.Source)
	}
	if creds.ClientID != DefaultClientID {
		t.Errorf("ClientID = %q", creds.ClientID)
	}
}
