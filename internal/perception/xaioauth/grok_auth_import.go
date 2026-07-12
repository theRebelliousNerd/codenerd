package xaioauth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// grokCLIAuthFile is the on-disk shape written by `grok login`.
// Keys are "issuer::client_id"; access token is field "key".
type grokCLIAuthFile map[string]grokCLIAuthEntry

type grokCLIAuthEntry struct {
	Key          string `json:"key"`
	AuthMode     string `json:"auth_mode"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	OIDCIssuer   string `json:"oidc_issuer"`
	OIDCClientID string `json:"oidc_client_id"`
	UserID       string `json:"user_id"`
	Email        string `json:"email"`
	TeamID       string `json:"team_id"`
}

// ImportGrokCLIAuth reads ~/.grok/auth.json (or path) and returns credentials
// suitable for Bearer API calls. Read-only: never writes back to Grok CLI store.
func ImportGrokCLIAuth(path string) (*Credentials, error) {
	if path == "" {
		return nil, ErrNoCredentials
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoCredentials
		}
		return nil, fmt.Errorf("read grok auth: %w", err)
	}

	var file grokCLIAuthFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse grok auth: %w", err)
	}
	if len(file) == 0 {
		return nil, ErrNoCredentials
	}

	// Prefer official Grok CLI client entry; else first usable entry.
	var chosen *grokCLIAuthEntry
	var chosenKey string
	preferSuffix := "::" + DefaultClientID
	for k, entry := range file {
		e := entry
		if e.Key == "" && e.RefreshToken == "" {
			continue
		}
		if strings.HasSuffix(k, preferSuffix) || e.OIDCClientID == DefaultClientID {
			chosen = &e
			chosenKey = k
			break
		}
		if chosen == nil {
			chosen = &e
			chosenKey = k
		}
	}
	if chosen == nil {
		return nil, ErrNoCredentials
	}

	creds := &Credentials{
		AccessToken:  strings.TrimSpace(chosen.Key),
		RefreshToken: strings.TrimSpace(chosen.RefreshToken),
		TokenType:    "Bearer",
		ClientID:     chosen.OIDCClientID,
		Issuer:       chosen.OIDCIssuer,
		Source:       "grok_cli_import",
		UpdatedAt:    time.Now().UTC(),
	}
	if creds.ClientID == "" {
		// Map key format: https://auth.x.ai::client-id
		if parts := strings.SplitN(chosenKey, "::", 2); len(parts) == 2 {
			creds.ClientID = parts[1]
			if creds.Issuer == "" {
				creds.Issuer = parts[0]
			}
		}
	}
	if creds.ClientID == "" {
		creds.ClientID = DefaultClientID
	}
	if creds.Issuer == "" {
		creds.Issuer = DefaultIssuer
	}

	if chosen.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339Nano, chosen.ExpiresAt); err == nil {
			creds.ExpiresAt = t.UTC()
		} else if t, err := time.Parse(time.RFC3339, chosen.ExpiresAt); err == nil {
			creds.ExpiresAt = t.UTC()
		}
	}

	if creds.AccessToken == "" && creds.RefreshToken == "" {
		return nil, ErrNoCredentials
	}
	return creds, nil
}
