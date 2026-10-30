package perception

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// groundedReplyServer answers every grounded search with body.
func groundedReplyServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const groundedAnswer = `"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"The widget is calibrated by"}]}],"usage":{"input_tokens":5,"output_tokens":9,"total_tokens":14}`

// Meta's explicit grounded search returned any reply with text as a
// successful search: an HTTP-200 run the provider marked incomplete at its
// output limit became an answer whose prefix read as the whole. The run's
// terminal status is now admitted first: only a completed run (or one that
// reports no status) is a result, an output-limit stop is the typed
// truncation the broker restates, and every other non-completion is an error
// that carries no vendor text.
func TestGroundedWebSearch_AdmitsOnlyACompletedRun(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		ok         bool
		truncated  bool
	}{
		{"completed", `{"status":"completed",` + groundedAnswer + `}`, true, false},
		{"no status", `{` + groundedAnswer + `}`, true, false},
		{"output limit", `{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"error":null,` + groundedAnswer + `}`, false, true},
		{"content filter", `{"status":"incomplete","incomplete_details":{"reason":"content_filter"},` + groundedAnswer + `}`, false, false},
		{"in progress", `{"status":"in_progress",` + groundedAnswer + `}`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestCompatClient(t, ProviderMeta, groundedReplyServer(t, tc.body).URL)
			res, err := c.GroundedWebSearch(context.Background(), "how is a widget calibrated")
			if tc.ok {
				if err != nil || res == nil || !strings.Contains(res.Text, "calibrated") {
					t.Fatalf("a completed run was not a result: %+v, %v", res, err)
				}
				return
			}
			if err == nil || res != nil {
				t.Fatalf("a %s run became a successful search: %+v", tc.name, res)
			}
			if _, ok := types.AsOutputTruncated(err); ok != tc.truncated {
				t.Fatalf("truncation typed = %v, want %v: %v", ok, tc.truncated, err)
			}
		})
	}
}

// A refusal is the typed refused outcome, without the refusal's text.
func TestGroundedWebSearch_ARefusalIsTypedAndSanitized(t *testing.T) {
	body := `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"SECRET-PROVIDER-TEXT"}]}]}`
	c := newTestCompatClient(t, ProviderMeta, groundedReplyServer(t, body).URL)
	_, err := c.GroundedWebSearch(context.Background(), "q")
	var se *types.StreamError
	if !errors.As(err, &se) || se.Outcome != types.StreamRefused {
		t.Fatalf("a refusal is not the typed refused outcome: %v", err)
	}
	if strings.Contains(err.Error(), "SECRET-PROVIDER-TEXT") {
		t.Fatalf("the refusal's text reached the error: %v", err)
	}
}
