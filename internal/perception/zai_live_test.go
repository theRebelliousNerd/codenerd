package perception

import (
	"codenerd/internal/config"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func requireLiveZAIClient(t *testing.T) *ZAIClient {
	t.Helper()

	if os.Getenv("CODENERD_LIVE_LLM") != "1" {
		t.Skip("skipping live LLM test: set CODENERD_LIVE_LLM=1 to enable")
	}

	configPath := config.DefaultUserConfigPath()
	cfg, err := config.LoadUserConfig(configPath)
	if err != nil {
		t.Skipf("skipping live LLM test: load config %s: %v", configPath, err)
	}

	apiKey := cfg.ZAIAPIKey
	if apiKey == "" && cfg.Provider == "zai" && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" && cfg.Provider == "" && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" {
		t.Skipf("skipping live LLM test: zai_api_key not configured in %s", configPath)
	}

	client := NewZAIClient(apiKey)
	if cfg.Provider == "zai" && cfg.Model != "" {
		client.SetModel(cfg.Model)
	}
	return client
}

func liveLLMTimeout() time.Duration {
	timeout := config.GetLLMTimeouts().PerCallTimeout
	if timeout <= 0 {
		return 10 * time.Minute
	}
	return timeout
}

func buildLargePrompt(paragraphs int) string {
	base := "The system processes build artifacts, caches intermediate results, and validates integrity through staged checks. " +
		"Latency sources include disk I/O, dependency resolution, and network fetches for third-party packages. " +
		"Teams reported frequent context switching and degraded flow when build feedback exceeds one minute."

	var sb strings.Builder
	for i := 0; i < paragraphs; i++ {
		sb.WriteString(base)
		sb.WriteString(fmt.Sprintf(" Paragraph %d emphasizes offline reliability and Windows-first constraints.", i+1))
		sb.WriteString("\n")
	}
	return sb.String()
}

func TestZAICompleteWithSystem_LargePrompt(t *testing.T) {
	client := requireLiveZAIClient(t)

	sentinel := "SENTINEL_LARGE_PROMPT"
	userPrompt := fmt.Sprintf(`Summarize the following text in 5 bullets and include the token %s in your response.

%s`, sentinel, buildLargePrompt(120))

	ctx, cancel := context.WithTimeout(context.Background(), liveLLMTimeout())
	defer cancel()

	start := time.Now()
	response, err := client.CompleteWithSystem(ctx, "You are a concise assistant.", userPrompt)
	duration := time.Since(start)
	t.Logf("ZAI large prompt duration=%s response_len=%d prompt_len=%d", duration, len(response), len(userPrompt))

	if err != nil {
		t.Fatalf("ZAI large prompt failed: %v", err)
	}
	if strings.TrimSpace(response) == "" {
		t.Fatalf("expected non-empty response")
	}
	if !strings.Contains(response, sentinel) {
		t.Fatalf("expected response to include sentinel %q", sentinel)
	}
}

func TestZAICompleteWithSystem_ComplexPrompt(t *testing.T) {
	client := requireLiveZAIClient(t)

	sentinel := "SENTINEL_COMPLEX_PROMPT"
	userPrompt := fmt.Sprintf(`You are analyzing a product brief.

Tasks:
1. Provide a summary section.
2. List risks and constraints.
3. End with a short JSON object with keys "summary" and "constraints".
4. Include the token %s in the response.

Brief:
%s`, sentinel, buildLargePrompt(40))

	ctx, cancel := context.WithTimeout(context.Background(), liveLLMTimeout())
	defer cancel()

	start := time.Now()
	response, err := client.CompleteWithSystem(ctx, "Follow the requested structure.", userPrompt)
	duration := time.Since(start)
	t.Logf("ZAI complex prompt duration=%s response_len=%d prompt_len=%d", duration, len(response), len(userPrompt))

	if err != nil {
		t.Fatalf("ZAI complex prompt failed: %v", err)
	}
	if strings.TrimSpace(response) == "" {
		t.Fatalf("expected non-empty response")
	}
	if !strings.Contains(response, sentinel) {
		t.Fatalf("expected response to include sentinel %q", sentinel)
	}
	lower := strings.ToLower(response)
	if !strings.Contains(lower, "summary") || !strings.Contains(lower, "constraint") {
		t.Fatalf("expected response to mention summary and constraints")
	}
}
