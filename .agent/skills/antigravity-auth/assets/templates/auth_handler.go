package antigravity

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Config holds the OAuth2 config
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type AuthHandler struct {
	oauthConfig *oauth2.Config
	// tokenStore  TokenStore // Interface to storage
}

func NewAuthHandler(cfg Config) *AuthHandler {
	return &AuthHandler{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes: []string{
				"https://www.googleapis.com/auth/cloud-platform",
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
				"https://www.googleapis.com/auth/cclog",
				"https://www.googleapis.com/auth/experimentsandconfigs",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	state := generateState() // Implement secure state generation

	// Generate PKCE verifier and challenge
	verifier := generateRandomString(32)
	challenge := generateCodeChallenge(verifier)

	// Store state & verifier in cookie/session
	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	c.SetCookie("oauth_verifier", verifier, 300, "/", "", false, true)

	url := h.oauthConfig.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *AuthHandler) Callback(c *gin.Context) {
	stateCookie, err := c.Cookie("oauth_state")
	verifier, _ := c.Cookie("oauth_verifier")

	if err != nil || c.Query("state") != stateCookie {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid_state"})
		return
	}

	code := c.Query("code")
	token, err := h.oauthConfig.Exchange(context.Background(), code,
		oauth2.SetAuthURLParam("code_verifier", verifier),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token_exchange_failed", "details": err.Error()})
		return
	}

	// 1. Fetch User Info
	client := h.oauthConfig.Client(context.Background(), token)
	userInfo, err := fetchUserInfo(client)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user_info_failed"})
		return
	}

	// 2. Discover Project ID (Crucial step)
	projectID, err := fetchProjectID(client, token.AccessToken)
	if err != nil {
		// Log warning, but maybe proceed if user can manually set it later?
		// For now, fail or return warning.
		fmt.Printf("Warning: failed to discover project ID: %v\n", err)
	}

	// TODO: Store token.RefreshToken, token.AccessToken, and projectID in secure storage
	// keyed by userInfo.Email

	c.JSON(http.StatusOK, gin.H{
		"message":   "success",
		"email":     userInfo.Email,
		"projectID": projectID,
	})
}

// fetchProjectID calls /v1internal:loadCodeAssist to find the active project
func fetchProjectID(client *http.Client, accessToken string) (string, error) {
	// Add required headers manually if the client wrapper doesn't do it yet
	// Note: The passed 'client' here is the oauth2 client, so it handles auth.
	// But we need to add the special headers:
	// X-Goog-Api-Client, Client-Metadata, etc.

	reqBody := []byte(`{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`)
	req, err := http.NewRequest("POST", "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity/1.11.5 windows/amd64")
	req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	req.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)

	// Since 'client' already has the TokenSource, we can just use it.
	// However, usually we wrap the *http.Request logic in a helper.

	resp, err := client.Do(req)
	if err != nil {
		// Try sandbox fallback if prod fails?
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("loadCodeAssist failed: %d", resp.StatusCode)
	}

	var result struct {
		CloudAICompanionProject struct {
			ID string `json:"id"`
		} `json:"cloudaicompanionProject"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.CloudAICompanionProject.ID, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func generateCodeChallenge(verifier string) string {
	// Implement S256 (SHA256 + Base64Url)
	// sha := sha256.Sum256([]byte(verifier))
	// return base64.RawURLEncoding.EncodeToString(sha[:])
	return "TODO_IMPLEMENT_SHA256"
}

type UserInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func fetchUserInfo(client *http.Client) (*UserInfo, error) {
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var info UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}
