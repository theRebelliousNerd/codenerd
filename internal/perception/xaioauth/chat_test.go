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

func TestClient_CompleteWithSystem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": "  hello from grok  "}},
			},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, "creds.json")
	if err := SaveCredentials(credPath, &Credentials{
		AccessToken: "test-token",
		ExpiresAt:   time.Now().Add(time.Hour),
		ClientID:    DefaultClientID,
	}); err != nil {
		t.Fatal(err)
	}

	cfg := Config{
		Model:          "grok-4.5",
		BaseURL:        srv.URL + "/v1",
		CredentialPath: credPath,
		ImportGrokAuth: false,
		Timeout:        time.Minute,
		Issuer:         DefaultIssuer,
		ClientID:       DefaultClientID,
	}
	c := NewClient(cfg)

	out, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if out != "hello from grok" {
		t.Errorf("out = %q", out)
	}
}

func TestClient_TierForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"You do not have an active Grok subscription"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	credPath := filepath.Join(dir, "creds.json")
	_ = SaveCredentials(credPath, &Credentials{
		AccessToken: "tok",
		ExpiresAt:   time.Now().Add(time.Hour),
	})

	c := NewClient(Config{
		Model:          "grok-4.5",
		BaseURL:        srv.URL + "/v1",
		CredentialPath: credPath,
		ImportGrokAuth: false,
		Timeout:        time.Minute,
	})
	_, err := c.CompleteWithSystem(context.Background(), "s", "u")
	if !IsTierForbidden(err) {
		t.Fatalf("err = %v, want tier forbidden", err)
	}
}
