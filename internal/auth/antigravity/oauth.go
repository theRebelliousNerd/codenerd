package antigravity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

const (
	// Antigravity Client ID (from Cloud Code IDE)
	ClientID     = "1071006060591-tmhssin2h21lcre235vtolojh4g403ep.apps.googleusercontent.com"
	ClientSecret = "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"
	AuthURL      = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenURL     = "https://oauth2.googleapis.com/token"
	RedirectURL  = "http://localhost:51121/oauth-callback"
	CallbackPort = ":51121"
)

var Scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
	"https://www.googleapis.com/auth/cclog",
	"https://www.googleapis.com/auth/experimentsandconfigs",
}

// Token holds the OAuth token details.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email,omitempty"`
	ProjectID    string    `json:"project_id,omitempty"` // Google Cloud Project ID
}

// TokenManager handles OAuth flow and token management.
type TokenManager struct {
	tokenFile string
	mu        sync.Mutex
	token     *Token
}

// NewTokenManager creates a new token manager.
func NewTokenManager() (*TokenManager, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	tokenFile := filepath.Join(home, ".nerd", "antigravity_tokens.json")

	tm := &TokenManager{
		tokenFile: tokenFile,
	}
	// Try to load existing token
	_ = tm.LoadToken()
	return tm, nil
}

// LoadToken loads the token from disk.
func (tm *TokenManager) LoadToken() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, err := os.ReadFile(tm.tokenFile)
	if err != nil {
		return err
	}

	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return err
	}
	tm.token = &token
	return nil
}

// SaveToken saves the token to disk.
func (tm *TokenManager) SaveToken() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.token == nil {
		return nil
	}

	data, err := json.MarshalIndent(tm.token, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(tm.tokenFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(tm.tokenFile, data, 0600)
}

// GetToken returns a valid access token, refreshing if necessary.
func (tm *TokenManager) GetToken(ctx context.Context) (*Token, error) {
	tm.mu.Lock()
	// Check if we have a token
	if tm.token == nil {
		tm.mu.Unlock()
		return nil, fmt.Errorf("no token found, authentication required")
	}

	// Check expiry (with margin)
	if time.Now().Add(5 * time.Minute).Before(tm.token.Expiry) {
		token := tm.token
		tm.mu.Unlock()
		return token, nil
	}
	tm.mu.Unlock()

	// Token expired, refresh it
	logging.PerceptionDebug("[Antigravity] Token expired, refreshing...")
	if err := tm.RefreshToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return tm.token, nil
}

// RefreshToken refreshes the access token using the refresh token.
func (tm *TokenManager) RefreshToken(ctx context.Context) error {
	tm.mu.Lock()
	if tm.token == nil || tm.token.RefreshToken == "" {
		tm.mu.Unlock()
		return fmt.Errorf("no refresh token available")
	}
	refreshToken := tm.token.RefreshToken
	tm.mu.Unlock()

	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("client_secret", ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("refresh failed: %s", string(body))
	}

	var newToken Token
	if err := json.NewDecoder(resp.Body).Decode(&newToken); err != nil {
		return err
	}

	tm.mu.Lock()
	tm.token.AccessToken = newToken.AccessToken
	tm.token.ExpiresIn = newToken.ExpiresIn
	tm.token.Expiry = time.Now().Add(time.Duration(newToken.ExpiresIn) * time.Second)
	// Response might contain a new refresh token (if rotated)
	if newToken.RefreshToken != "" {
		tm.token.RefreshToken = newToken.RefreshToken
	}
	tm.mu.Unlock()

	return tm.SaveToken()
}

// AuthFlowResult holds the result of the auth flow.
type AuthFlowResult struct {
	Verifier string
	State    string
	AuthURL  string
}

// StartAuth generates the PKCE challenge and authorization URL.
func StartAuth() (*AuthFlowResult, error) {
	// Generate PKCE Verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate Challenge (S256)
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	// Generate State
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Build URL
	u, err := url.Parse(AuthURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("client_id", ClientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", RedirectURL)
	q.Set("scope", strings.Join(Scopes, " "))
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("access_type", "offline")
	q.Set("prompt", "consent")
	u.RawQuery = q.Encode()

	return &AuthFlowResult{
		Verifier: verifier,
		State:    state,
		AuthURL:  u.String(),
	}, nil
}

// ExchangeCode executes the code exchange for tokens.
func (tm *TokenManager) ExchangeCode(ctx context.Context, code, verifier string) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("client_secret", ClientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", RedirectURL)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("exchange failed: %s", string(body))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	// Calculate expiry
	token.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)

	// Fetch user info (email)
	if email, err := fetchUserEmail(token.AccessToken); err == nil {
		token.Email = email
	}

	tm.mu.Lock()
	tm.token = &token
	tm.mu.Unlock()

	if err := tm.SaveToken(); err != nil {
		return nil, err
	}

	return &token, nil
}

func fetchUserEmail(accessToken string) (string, error) {
	req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v1/userinfo?alt=json", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var userInfo struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return "", err
	}
	return userInfo.Email, nil
}

// RefreshToken refreshes an access token using a refresh token (standalone function)
func RefreshToken(ctx context.Context, refreshToken string) (*Token, error) {
	data := url.Values{}
	data.Set("client_id", ClientID)
	data.Set("client_secret", ClientSecret)
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	token.Expiry = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return &token, nil
}

// WaitForCallback starts a local HTTP server to listen for the OAuth callback.
// Returns the code and state, or an error.
func WaitForCallback(ctx context.Context, expectedState string) (string, error) {
	codeChan := make(chan string)
	errChan := make(chan error)

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth-callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		state := q.Get("state")
		code := q.Get("code")
		errStr := q.Get("error")

		if state != expectedState {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			errChan <- fmt.Errorf("invalid state received")
			return
		}

		if errStr != "" {
			http.Error(w, "Auth failed: "+errStr, http.StatusBadRequest)
			errChan <- fmt.Errorf("auth failed: %s", errStr)
			return
		}

		if code == "" {
			http.Error(w, "No code received", http.StatusBadRequest)
			errChan <- fmt.Errorf("no code received")
			return
		}

		// Success
		w.Write([]byte("Authentication successful! You can close this window and return to the terminal."))
		codeChan <- code
	})

	server := &http.Server{Addr: CallbackPort, Handler: mux}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Cleanup server on return
	defer server.Close()

	select {
	case code := <-codeChan:
		return code, nil
	case err := <-errChan:
		return "", err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
