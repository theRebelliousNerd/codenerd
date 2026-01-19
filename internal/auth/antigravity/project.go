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
	// Endpoints for loadCodeAssist (try prod first per shekohex plugin)
	LoadCodeAssistEndpointProd     = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
	LoadCodeAssistEndpointDaily    = "https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:loadCodeAssist"
	LoadCodeAssistEndpointAutopush = "https://autopush-cloudcode-pa.sandbox.googleapis.com/v1internal:loadCodeAssist"

	DefaultProjectID = "rising-fact-p41fc" // Backup project ID
)

// loadCodeAssistEndpoints is the order to try for project resolution (prod first)
var loadCodeAssistEndpoints = []string{
	LoadCodeAssistEndpointProd,
	LoadCodeAssistEndpointDaily,
	LoadCodeAssistEndpointAutopush,
}

// ProjectResolver resolves the Google Cloud Project ID for the authenticated user.
type ProjectResolver struct {
	accessToken string
}

func NewProjectResolver(accessToken string) *ProjectResolver {
	return &ProjectResolver{accessToken: accessToken}
}

// ResolveProjectID queries the loadCodeAssist endpoint to find the project ID.
// Tries prod → daily → autopush in order (per shekohex plugin behavior).
func (p *ProjectResolver) ResolveProjectID() (string, error) {
	reqBody := map[string]interface{}{
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}
	jsonData, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 10 * time.Second}

	for _, endpoint := range loadCodeAssistEndpoints {
		logging.PerceptionDebug("[Antigravity] Trying loadCodeAssist at %s", endpoint)

		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonData))
		if err != nil {
			continue
		}

		req.Header.Set("Authorization", "Bearer "+p.accessToken)
		req.Header.Set("Content-Type", "application/json")
		// Headers matching shekohex oauth.ts fetchProjectID (lines 133-138)
		req.Header.Set("User-Agent", "google-api-nodejs-client/9.15.1")
		req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
		req.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)

		resp, err := client.Do(req)
		if err != nil {
			logging.PerceptionDebug("[Antigravity] loadCodeAssist error at %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			logging.PerceptionDebug("[Antigravity] loadCodeAssist %d at %s: %s", resp.StatusCode, endpoint, string(body))
			continue
		}

		var result struct {
			CloudAICompanionProject interface{} `json:"cloudaicompanionProject"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			continue
		}

		// Handle polymorphism in response (string or object)
		if idStr, ok := result.CloudAICompanionProject.(string); ok && idStr != "" {
			logging.PerceptionDebug("[Antigravity] Resolved project ID: %s from %s", idStr, endpoint)
			if idStr == "resolute-airship-mq6tl" {
				logging.PerceptionWarn("[Antigravity] Project %s is known broken for this user, forcing fallback to %s", idStr, DefaultProjectID)
				return DefaultProjectID, nil
			}
			return idStr, nil
		}
		if idObj, ok := result.CloudAICompanionProject.(map[string]interface{}); ok {
			if id, ok := idObj["id"].(string); ok && id != "" {
				logging.PerceptionDebug("[Antigravity] Resolved project ID: %s from %s", id, endpoint)
				if id == "resolute-airship-mq6tl" {
					logging.PerceptionWarn("[Antigravity] Project %s is known broken for this user, forcing fallback to %s", id, DefaultProjectID)
					return DefaultProjectID, nil
				}
				return id, nil
			}
		}

		logging.PerceptionDebug("[Antigravity] loadCodeAssist at %s missing project id", endpoint)
	}

	// No project found - try onboarding to auto-provision
	logging.PerceptionDebug("[Antigravity] No project from loadCodeAssist, attempting onboarding...")
	if onboardedProject := p.onboardUser("FREE"); onboardedProject != "" {
		if onboardedProject == "resolute-airship-mq6tl" {
			logging.PerceptionWarn("[Antigravity] Onboarded project %s is known broken, forcing fallback to %s", onboardedProject, DefaultProjectID)
			return DefaultProjectID, nil
		}
		return onboardedProject, nil
	}

	logging.PerceptionWarn("[Antigravity] All loadCodeAssist endpoints failed, using fallback: %s", DefaultProjectID)
	return DefaultProjectID, nil
}

// onboardUser calls the onboardUser API to auto-provision a managed project.
// This handles accounts that were added before managed project provisioning was required.
func (p *ProjectResolver) onboardUser(tierId string) string {
	reqBody := map[string]interface{}{
		"tierId": tierId,
		"metadata": map[string]string{
			"ideType":    "IDE_UNSPECIFIED",
			"platform":   "PLATFORM_UNSPECIFIED",
			"pluginType": "GEMINI",
		},
	}
	jsonData, _ := json.Marshal(reqBody)

	client := &http.Client{Timeout: 10 * time.Second}

	// Endpoint fallback order: daily → autopush → prod (per shekohex plugin)
	endpoints := []string{
		"https://daily-cloudcode-pa.sandbox.googleapis.com/v1internal:onboardUser",
		"https://autopush-cloudcode-pa.sandbox.googleapis.com/v1internal:onboardUser",
		"https://cloudcode-pa.googleapis.com/v1internal:onboardUser",
	}

	for _, endpoint := range endpoints {
		logging.PerceptionDebug("[Antigravity] Trying onboardUser at %s", endpoint)

		req, err := http.NewRequest("POST", endpoint, bytes.NewReader(jsonData))
		if err != nil {
			continue
		}

		req.Header.Set("Authorization", "Bearer "+p.accessToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "antigravity/1.11.5 windows/amd64")
		req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
		req.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)

		resp, err := client.Do(req)
		if err != nil {
			logging.PerceptionDebug("[Antigravity] onboardUser error at %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			logging.PerceptionDebug("[Antigravity] onboardUser %d at %s: %s", resp.StatusCode, endpoint, string(body))
			continue
		}

		var result struct {
			Done     bool `json:"done"`
			Response struct {
				CloudAICompanionProject struct {
					ID string `json:"id"`
				} `json:"cloudaicompanionProject"`
			} `json:"response"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			continue
		}

		if result.Done && result.Response.CloudAICompanionProject.ID != "" {
			logging.PerceptionDebug("[Antigravity] Onboarded project: %s from %s", result.Response.CloudAICompanionProject.ID, endpoint)
			return result.Response.CloudAICompanionProject.ID
		}
	}

	return ""
}
