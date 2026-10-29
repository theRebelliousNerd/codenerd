package perception

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// A streamed Gemini answer's grounding is the stream's own: it rides on the
// terminal result and replaces the client's last-sources record, which the
// streaming path used to leave holding an earlier request's URLs (Astra audit
// streaming-grounding-source-ownership).
func TestGeminiStream_GroundingIsRequestOwned(t *testing.T) {
	grounded := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		gm := ""
		if grounded {
			gm = `,"groundingMetadata":{"groundingChunks":[{"web":{"uri":"https://go.dev/doc","title":"Go"}},{"web":{"uri":"https://go.dev/doc"}},{"web":{"uri":"https://pkg.go.dev"}}]}`
		}
		fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"Go 1.27\"}],\"role\":\"model\"}%s}]}\n\n", gm)
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[],\"role\":\"model\"},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":3,\"candidatesTokenCount\":2,\"totalTokenCount\":5}}\n\n")
	}))
	defer srv.Close()
	c := NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", BaseURL: srv.URL, Model: "gemini-x", Timeout: 5 * time.Second})
	c.lastGroundingSources = []string{"https://stale.example/old"}

	content, done := c.CompleteWithStreaming(context.Background(), "", "latest go?", false)
	_, res := types.CollectStream(context.Background(), content, done, nil)
	if res.Err != nil {
		t.Fatalf("stream: %v", res.Err)
	}
	if got := strings.Join(res.GroundingSources, " "); got != "https://go.dev/doc https://pkg.go.dev" {
		t.Fatalf("result sources = %q, want the stream's own, each once", got)
	}
	if got := strings.Join(c.GetLastGroundingSources(), " "); got != "https://go.dev/doc https://pkg.go.dev" {
		t.Fatalf("client record = %q, want this stream's sources, not an earlier request's", got)
	}

	grounded = false
	content, done = c.CompleteWithStreaming(context.Background(), "", "again", false)
	if _, res := types.CollectStream(context.Background(), content, done, nil); res.GroundingSources != nil || c.GetLastGroundingSources() != nil {
		t.Fatalf("an ungrounded stream kept sources: result=%v client=%v", res.GroundingSources, c.GetLastGroundingSources())
	}
}

// A whole completion published as a stream carries its sources the same way.
func TestStreamWhole_CarriesTheCompletionsSources(t *testing.T) {
	content, done := types.StreamWhole(context.Background(), func(context.Context) (string, []string, error) {
		return "answer", []string{"https://a.example"}, nil
	})
	if _, res := types.CollectStream(context.Background(), content, done, nil); len(res.GroundingSources) != 1 || res.GroundingSources[0] != "https://a.example" {
		t.Fatalf("sources = %v", res.GroundingSources)
	}
}
