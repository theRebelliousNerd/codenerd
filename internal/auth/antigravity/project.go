package antigravity

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"codenerd/internal/logging"
)

const (
	LoadCodeAssistEndpoint = "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:loadCodeAssist"
	DefaultProjectID       = "rising-fact-p41fc" // Backup project ID
)

// ProjectResolver resolves the Google Cloud Project ID for the authenticated user.
type ProjectResolver struct {
	accessToken string
}

func NewProjectResolver(accessToken string) *ProjectResolver {
	return &ProjectResolver{accessToken: accessToken}
}

// ResolveProjectID queries the loadCodeAssist endpoint to find the project ID.
func (p *ProjectResolver) ResolveProjectID() (string, error) {
	reqBody := map[string]interface{}{
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}
	jsonData, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", LoadCodeAssistEndpoint, bytes.NewReader(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+p.accessToken)
	req.Header.Set("Content-Type", "application/json")
	// Mimic headers
	req.Header.Set("User-Agent", "antigravity/1.11.5 windows/amd64")
	req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	req.Header.Set("Client-Metadata", "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logging.PerceptionError("[Antigravity] Failed to reach loadCodeAssist: %v", err)
		return DefaultProjectID, nil // Fallback
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logging.PerceptionWarn("[Antigravity] loadCodeAssist failed (status %d): %s", resp.StatusCode, string(body))
		return DefaultProjectID, nil // Fallback
	}

	var result struct {
		CloudAICompanionProject interface{} `json:"cloudaicompanionProject"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return DefaultProjectID, nil
	}

	// Handle polymorphism in response (string or object)
	if idStr, ok := result.CloudAICompanionProject.(string); ok && idStr != "" {
		return idStr, nil
	}
	if idObj, ok := result.CloudAICompanionProject.(map[string]interface{}); ok {
		if id, ok := idObj["id"].(string); ok && id != "" {
			return id, nil
		}
	}

	return DefaultProjectID, nil
}
