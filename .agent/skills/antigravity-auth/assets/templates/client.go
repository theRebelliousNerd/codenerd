package antigravity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"time"
)

const (
	ProdEndpoint    = "https://cloudcode-pa.googleapis.com"
	SandboxEndpoint = "https://daily-cloudcode-pa.sandbox.googleapis.com"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	projectID  string
	userAgent  string
}

func NewClient(tokenSource oauth2.TokenSource, projectID string) *Client {
	return &Client{
		httpClient: oauth2.NewClient(context.Background(), tokenSource),
		baseURL:    ProdEndpoint,
		projectID:  projectID,
		userAgent:  "antigravity/1.11.5 windows/amd64", // Mimic official client
	}
}

type GenerateRequest struct {
	Project   string      `json:"project"`
	Model     string      `json:"model"`
	Request   RequestData `json:"request"`
	UserAgent string      `json:"userAgent"`
	RequestID string      `json:"requestId"`
}

type RequestData struct {
	Contents          []Content          `json:"contents"`
	GenerationConfig  *GenerationConfig  `json:"generationConfig,omitempty"`
	SystemInstruction *SystemInstruction `json:"systemInstruction,omitempty"`
	Tools             []Tool             `json:"tools,omitempty"`
}

type Content struct {
	Role  string `json:"role"` // "user" or "model"
	Parts []Part `json:"parts"`
}

type Part struct {
	Text string `json:"text,omitempty"`
}

type GenerationConfig struct {
	MaxOutputTokens int             `json:"maxOutputTokens,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	ThinkingConfig  *ThinkingConfig `json:"thinkingConfig,omitempty"`
}

type ThinkingConfig struct {
	ThinkingBudget  int  `json:"thinkingBudget"`
	IncludeThoughts bool `json:"includeThoughts"`
}

type SystemInstruction struct {
	Parts []Part `json:"parts"`
}

type Tool struct {
	// ... define tool structure matching Gemini spec
}

func (c *Client) GenerateContent(ctx context.Context, model string, req RequestData) (*Response, error) {
	fullReq := GenerateRequest{
		Project:   c.projectID,
		Model:     model,
		Request:   req,
		UserAgent: "antigravity",
		RequestID: fmt.Sprintf("req_%d", time.Now().UnixNano()),
	}

	body, err := json.Marshal(fullReq)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v1internal:generateContent", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", c.userAgent)
	httpReq.Header.Set("X-Goog-Api-Client", "google-cloud-sdk vscode_cloudshelleditor/0.1")
	httpReq.Header.Set("Client-Metadata", `{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}`)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp Response
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	return &apiResp, nil
}

type Response struct {
	// ... define response structure based on spec
}
