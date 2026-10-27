package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/config"
)

// effortRecorder is an Anthropic endpoint that records the output_config.effort
// of every inference request it is sent.
type effortRecorder struct {
	mu      sync.Mutex
	efforts []string
	server  *httptest.Server
}

func newEffortRecorder(t *testing.T) *effortRecorder {
	t.Helper()
	er := &effortRecorder{}
	er.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/count_tokens") {
			io.WriteString(w, `{"input_tokens":4}`)
			return
		}
		var body struct {
			OutputConfig *struct {
				Effort string `json:"effort"`
			} `json:"output_config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		effort := ""
		if body.OutputConfig != nil {
			effort = body.OutputConfig.Effort
		}
		er.mu.Lock()
		er.efforts = append(er.efforts, effort)
		er.mu.Unlock()
		io.WriteString(w, anthropicChatOK)
	}))
	t.Cleanup(er.server.Close)
	return er
}

func (er *effortRecorder) last(t *testing.T) string {
	t.Helper()
	er.mu.Lock()
	defer er.mu.Unlock()
	if len(er.efforts) == 0 {
		t.Fatal("no inference request reached the endpoint")
	}
	return er.efforts[len(er.efforts)-1]
}

// A shard profile's route is sent an effort: its own when the profile sets one
// (riding the request), else the effort of the slot whose call it took. It
// used to be built from provider and model alone and sent none.
func TestRoutedClient_SendsTheResolvedEffort(t *testing.T) {
	planner := &config.SecondaryLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", ReasoningEffort: "high"}
	worker := &config.SecondaryLLMConfig{Provider: "meta", Model: "muse-spark-1.3-contributor", ReasoningEffort: "xhigh"}
	tests := []struct {
		name    string
		serving *config.SecondaryLLMConfig
		profile config.ShardProfile
		want    string
	}{
		{name: "the planner's route keeps the planner's effort", serving: planner, want: "high"},
		{name: "the worker's route keeps the worker's effort", serving: worker, want: "xhigh"},
		{name: "a profile's own effort wins", serving: planner, profile: config.ShardProfile{ReasoningEffort: "low"}, want: "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			er := newEffortRecorder(t)
			userCfg := &config.UserConfig{AnthropicAPIKey: "ak", BaseURL: er.server.URL}
			client, err := NewRoutedClientFromUserConfig(userCfg, tt.serving, "anthropic", "claude-opus-5-5")
			if err != nil {
				t.Fatalf("NewRoutedClientFromUserConfig: %v", err)
			}
			ctx := config.ShardProfileContext(context.Background(), tt.profile)
			if _, err := client.CompleteWithSystem(ctx, "sys", "review this"); err != nil {
				t.Fatalf("CompleteWithSystem: %v", err)
			}
			if got := er.last(t); got != tt.want {
				t.Errorf("output_config.effort = %q, want %q", got, tt.want)
			}
		})
	}
}

// The Meta classifier is sent the classification effort on the wire: Meta's
// lowest (minimal) by default, with the main slot at xhigh.
func TestMetaClassification_SendsTheClassificationEffort(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		got, _ = body["reasoning_effort"].(string)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"/fix"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()

	pc, err := ProviderConfigFromUserConfig(&config.UserConfig{
		Provider: "meta", Model: metaContributorModel, ClassificationModel: metaContributorModel,
		MetaAPIKey: "mk", BaseURL: srv.URL, ReasoningEffort: "xhigh",
	})
	if err != nil {
		t.Fatalf("ProviderConfigFromUserConfig: %v", err)
	}
	class, err := NewClassificationClientFromConfig(pc)
	if err != nil || class == nil {
		t.Fatalf("NewClassificationClientFromConfig = (%v, %v)", class, err)
	}
	if _, err := class.CompleteWithSystem(context.Background(), "classify", "fix the bug"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if got != "minimal" {
		t.Errorf("classification reasoning_effort on the wire = %q, want minimal", got)
	}
}
