package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTokenManager(t *testing.T) {
	// Create a temporary directory for the home dir
	tmpDir, err := os.MkdirTemp("", "nerd_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set HOME env var for the test (or USERPROFILE on Windows)
	originalHome := os.Getenv("USERPROFILE")
	os.Setenv("USERPROFILE", tmpDir)
	defer os.Setenv("USERPROFILE", originalHome)

	// Only test if we can get a TokenManager. Detailed path checking might rely on OS specifics handled by os.UserHomeDir
	tm, err := NewTokenManager()
	if err != nil {
		t.Fatalf("NewTokenManager failed: %v", err)
	}

	if tm == nil {
		t.Error("Returned TokenManager is nil")
	}
}

func TestTokenPersistence(t *testing.T) {
	// Setup temp file
	tmpDir, err := os.MkdirTemp("", "nerd_test_token")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tokenFile := filepath.Join(tmpDir, "tokens.json")
	tm := &TokenManager{
		tokenFile: tokenFile,
	}

	// 1. Test Save with no token
	err = tm.SaveToken() // Should be no-op or nil error
	if err != nil {
		t.Errorf("SaveToken with nil token returned error: %v", err)
	}

	// 2. Test Save with token
	testToken := &Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-123",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
		Expiry:       time.Now().Add(time.Hour),
		Email:        "test@example.com",
		ProjectID:    "test-project",
	}
	tm.token = testToken

	if err := tm.SaveToken(); err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	// Vertify file exists
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		t.Error("Token file was not created")
	}

	// 3. Test Load
	newTm := &TokenManager{
		tokenFile: tokenFile,
	}
	if err := newTm.LoadToken(); err != nil {
		t.Fatalf("LoadToken failed: %v", err)
	}

	if newTm.token.AccessToken != testToken.AccessToken {
		t.Errorf("Loaded AcccessToken mismatch. Got %s, want %s", newTm.token.AccessToken, testToken.AccessToken)
	}
	if newTm.token.Email != testToken.Email {
		t.Errorf("Loaded Email mismatch. Got %s, want %s", newTm.token.Email, testToken.Email)
	}
}

func TestGetToken_Valid(t *testing.T) {
	tm := &TokenManager{
		token: &Token{
			AccessToken: "valid-token",
			Expiry:      time.Now().Add(time.Hour),
		},
	}

	token, err := tm.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken returned error: %v", err)
	}
	if token.AccessToken != "valid-token" {
		t.Errorf("GetToken returned wrong token: %s", token.AccessToken)
	}
}

func TestGetToken_Expired_Refresh(t *testing.T) {
	// Mock Token Endpoint
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "new-access-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"refresh_token": "new-refreshed-token", // Optional rotation
		})
	}))
	defer ts.Close()

	// Save original transport
	origTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = origTransport }()

	// Intercept requests
	http.DefaultClient.Transport = RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == TokenURL {
			w := httptest.NewRecorder()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "new-access-token",
				"expires_in":    3600,
				"token_type":    "Bearer",
				"refresh_token": "new-refreshed-token",
			})
			return w.Result(), nil
		}
		return nil, fmt.Errorf("unexpected request to %s", req.URL.String())
	})

	// Setup Temp File for SaveToken inside RefreshToken
	tmpDir, _ := os.MkdirTemp("", "nerd_test_refresh")
	defer os.RemoveAll(tmpDir)

	tm := &TokenManager{
		tokenFile: filepath.Join(tmpDir, "tokens.json"),
		token: &Token{
			AccessToken:  "expired-token",
			RefreshToken: "old-refresh",
			Expiry:       time.Now().Add(-time.Hour), // Expired
		},
	}

	token, err := tm.GetToken(context.Background())
	if err != nil {
		t.Fatalf("GetToken failed during refresh: %v", err)
	}

	if token.AccessToken != "new-access-token" {
		t.Errorf("Expected new access token, got %s", token.AccessToken)
	}
	if token.RefreshToken != "new-refreshed-token" {
		t.Errorf("Expected new refresh token, got %s", token.RefreshToken)
	}
}

func TestStartAuth(t *testing.T) {
	res, err := StartAuth()
	if err != nil {
		t.Fatalf("StartAuth failed: %v", err)
	}

	if res.Verifier == "" {
		t.Error("Verifier is empty")
	}
	if res.State == "" {
		t.Error("State is empty")
	}
}

func TestExchangeCode(t *testing.T) {
	// Mock Transport
	origTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = origTransport }()

	http.DefaultClient.Transport = RoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() == TokenURL {
			w := httptest.NewRecorder()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "exchanged-access",
				"refresh_token": "exchanged-refresh",
				"expires_in":    3600,
				"id_token":      "fake-id-token",
			})
			return w.Result(), nil
		}
		if req.URL.String() == "https://www.googleapis.com/oauth2/v1/userinfo?alt=json" {
			w := httptest.NewRecorder()
			json.NewEncoder(w).Encode(map[string]string{
				"email": "user@example.com",
			})
			return w.Result(), nil
		}
		return nil, fmt.Errorf("unexpected url: %s", req.URL.String())
	})

	tmpDir, _ := os.MkdirTemp("", "exch")
	defer os.RemoveAll(tmpDir)

	tm := &TokenManager{tokenFile: filepath.Join(tmpDir, "t.json")}
	token, err := tm.ExchangeCode(context.Background(), "fake-code", "fake-verifier")
	if err != nil {
		t.Fatalf("ExchangeCode failed: %v", err)
	}

	if token.AccessToken != "exchanged-access" {
		t.Errorf("Expected exchanged-access, got %s", token.AccessToken)
	}
	if token.Email != "user@example.com" {
		t.Errorf("Expected user@example.com, got %s", token.Email)
	}
}

func TestWaitForCallback(t *testing.T) {
	// Start the wait in a goroutine
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resultChan := make(chan string)
	errChan := make(chan error)

	go func() {
		code, err := WaitForCallback(ctx, "test-state")
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- code
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Simulate callback
	resp, err := http.Get("http://localhost:51121/oauth-callback?state=test-state&code=test-code")
	if err != nil {
		t.Fatalf("Failed to make callback request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Callback returned status %d", resp.StatusCode)
	}

	select {
	case code := <-resultChan:
		if code != "test-code" {
			t.Errorf("Expected code test-code, got %s", code)
		}
	case err := <-errChan:
		t.Fatalf("WaitForCallback failed: %v", err)
	case <-ctx.Done():
		t.Fatal("WaitForCallback timed out")
	}
}

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) (*http.Response, error)

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
