package antigravity

import (
	"net/http"
	"strings"
)

const (
	EndpointDaily    = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	EndpointAutopush = "https://autopush-cloudcode-pa.sandbox.googleapis.com"
	EndpointProd     = "https://cloudcode-pa.googleapis.com"
	
	GeminiCLIEndpoint = "https://cloudcode-pa.googleapis.com" // Same as prod for now
)

var EndpointFallbacks = []string{
	EndpointDaily,
	EndpointAutopush,
	EndpointProd,
}

// PrepareRequest adds necessary headers for Antigravity or Gemini CLI
func PrepareRequest(req *http.Request, accessToken string, headerStyle string) {
	req.Header.Set("Authorization", "Bearer "+accessToken)
	
	if headerStyle == "gemini-cli" {
		req.Header.Set("User-Agent", "google-api-nodejs-client/9.15.1")
		req.Header.Set("X-Goog-Api-Client", "gl-node/22.17.0")
		req.Header.Set("Client-Metadata", "ideType=IDE_UNSPECIFIED,platform=PLATFORM_UNSPECIFIED,pluginType=GEMINI")
	} else {
		// Antigravity default
		req.Header.Set("User-Agent", "antigravity/1.11.5 windows/amd64")
		req.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
		req.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)
	}
}

// GetHeaderStyle determines which headers to use based on model
func GetHeaderStyle(model string) string {
	if strings.Contains(model, "antigravity") || strings.Contains(model, "claude") {
		return "antigravity"
	}
	return "gemini-cli"
}

// GetQuotaKey returns the quota key for tracking
func GetQuotaKey(model string, headerStyle string) string {
	if strings.Contains(model, "claude") {
		return "claude"
	}
	if headerStyle == "antigravity" {
		return "gemini-antigravity"
	}
	return "gemini-cli"
}
