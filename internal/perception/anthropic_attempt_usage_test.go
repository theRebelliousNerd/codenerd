package perception

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"testing/iotest"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

type anthropicAttemptObserver struct {
	mu      sync.Mutex
	reports []usage.Report
	unknown bool
}

func (o *anthropicAttemptObserver) Observed(r usage.Report) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.reports = append(o.reports, r)
}
func (o *anthropicAttemptObserver) UsageUnknown() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.unknown = true
}

func assertAnthropicAttempts(t *testing.T, o *anthropicAttemptObserver, reports, input, output int, unknown bool, operation string) {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	var in, out int
	for _, r := range o.reports {
		in += r.Usage.InputTokens
		out += r.Usage.OutputTokens
		if r.Provider != "anthropic" || r.Operation != operation {
			t.Errorf("wrong attribution: %+v", r)
		}
	}
	if len(o.reports) != reports || in != input || out != output || o.unknown != unknown {
		t.Errorf("reports/input/output/unknown = %d/%d/%d/%v; want %d/%d/%d/%v", len(o.reports), in, out, o.unknown, reports, input, output, unknown)
	}
}

func TestAnthropicAttemptUsage_RetryAcrossSurfaces(t *testing.T) {
	fastRetries(t)
	for _, surface := range []string{"chat", "tools", "history", "schema"} {
		for _, kind := range []string{"clean", "counted_incomplete", "missing_incomplete"} {
			t.Run(surface+"/"+kind, func(t *testing.T) {
				// Schema and the other surfaces may return the same text: this
				// fixture asserts transport accounting, not JSON schema validation.
				// Encode text with JSON escaping rather than editing wire syntax.
				var body map[string]any
				if err := json.Unmarshal([]byte(anthropicChatOK), &body); err != nil {
					t.Fatal(err)
				}
				body["content"] = []any{map[string]any{"type": "text", "text": `{"answer":"yes"}`}}
				encoded, err := json.Marshal(body)
				if err != nil {
					t.Fatal(err)
				}
				done := string(encoded)
				replies := []string{done}
				wantReports, wantInput, wantOutput, unknown := 1, 3, 7, false
				if kind != "clean" {
					counts := `{}`
					unknown = true
					if kind == "counted_incomplete" {
						counts = `{"input_tokens":11,"output_tokens":13,"cache_creation_input_tokens":2,"cache_read_input_tokens":5}`
						wantReports, wantInput, wantOutput, unknown = 2, 21, 20, false
					}
					replies = append([]string{`{"type":"message","content":[],"stop_reason":"end_turn","usage":` + counts + `}`}, replies...)
				}
				ws := newWebToolServer(t, replies...)
				raw := anthropicTestClient(ws.server.URL)
				meter := broker.NewMeter(broker.MeterConfig{})
				cfg := meter.ConfigFor(broker.ProviderCreds{Provider: "anthropic", Model: raw.GetModel()})
				receipts := broker.NewRingSink(4)
				cfg.Sink = receipts
				client, err := broker.Wrap(raw, cfg)
				if err != nil {
					t.Fatal(err)
				}
				ctx, tracker := trackedContext(t)
				outer, inner := &anthropicAttemptObserver{}, &anthropicAttemptObserver{}
				ctx = usage.WithObserver(usage.WithObserver(ctx, outer), inner)
				defs := []types.ToolDefinition{{Name: "lookup", Description: "fixture", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}}}
				operation := usageOpChat
				var text string
				switch surface {
				case "chat":
					text, err = client.CompleteWithSystem(ctx, "sys", "q")
				case "schema":
					text, err = client.(interface {
						CompleteWithSchema(context.Context, string, string, string) (string, error)
					}).CompleteWithSchema(ctx, "sys", "q", anthropicClosedSchema)
				case "tools", "history":
					operation = usageOpToolGen
					var response *types.LLMToolResponse
					if surface == "tools" {
						response, err = client.CompleteWithTools(ctx, "sys", "q", defs)
					} else {
						response, err = client.(types.ToolResultsProvider).CompleteWithToolResults(ctx, "sys", userTurn("q"), defs)
					}
					if response != nil {
						text = response.Text
						if response.Usage.InputTokens != 3 || response.Usage.OutputTokens != 7 {
							t.Errorf("accepted response usage changed: %+v", response.Usage)
						}
					}
				}
				if err != nil || text != `{"answer":"yes"}` || ws.sent() != len(replies) {
					t.Fatalf("behavior: text=%q err=%v requests=%d", text, err, ws.sent())
				}
				for _, o := range []*anthropicAttemptObserver{outer, inner} {
					assertAnthropicAttempts(t, o, wantReports, wantInput, wantOutput, unknown, operation)
					if kind == "counted_incomplete" && (o.reports[0].Usage.CacheWriteTokens != 2 || o.reports[0].Usage.CachedContentTokens != 5) {
						t.Errorf("cache subcounts lost: %+v", o.reports[0])
					}
				}
				if got := tracker.Stats().TotalProject.Total; got != int64(wantInput+wantOutput) {
					t.Errorf("tracker tokens=%d, want %d", got, wantInput+wantOutput)
				}
				status := broker.UsageReported
				if unknown {
					status = broker.UsagePartial
				}
				got := receipts.Receipts()
				if len(got) != 1 || got[0].UsageStatus != status || got[0].Actual.InputTokens != int64(wantInput) || got[0].Actual.OutputTokens != int64(wantOutput) || got[0].UsageConflict != "" {
					t.Errorf("receipt accounting: %+v", got)
				}
			})
		}
	}
}

func TestAnthropicAttemptUsage_ContinuationAndTerminalErrors(t *testing.T) {
	fastRetries(t)
	for _, kind := range []string{"complete", "missing_pause", "missing_final", "cancel", "window", "pause_limit", "malformed_next", "unreadable_next", "decoded_error", "incomplete_limit"} {
		t.Run(kind, func(t *testing.T) {
			pause := pausedFetchReply(1000)
			replies := []string{pause, continuationDone}
			wantReports, wantInput, wantOutput, unknown, wantSent := 2, 40, 12, false, 2
			window := 0
			switch kind {
			case "missing_pause":
				replies[0] = strings.Replace(pause, `"input_tokens":10,"output_tokens":5,`, "", 1)
				wantReports, wantInput, wantOutput, unknown = 1, 30, 7, true
			case "missing_final":
				replies[1] = strings.Replace(continuationDone, `"input_tokens":30,"output_tokens":7`, "", 1)
				wantReports, wantInput, wantOutput, unknown = 1, 10, 5, true
			case "cancel":
				wantReports, wantInput, wantOutput, wantSent = 1, 10, 5, 1
			case "window":
				window = 100
				wantReports, wantInput, wantOutput, wantSent = 1, 10, 5, 1
			case "pause_limit":
				replies = []string{pause}
				wantReports, wantInput, wantOutput, wantSent = anthropicMaxPauseResumes+1, 50, 25, anthropicMaxPauseResumes+1
			case "malformed_next":
				replies[1] = `{"type":`
				wantReports, wantInput, wantOutput, unknown = 1, 10, 5, true
			case "unreadable_next":
				wantReports, wantInput, wantOutput, unknown = 1, 10, 5, true
			case "decoded_error":
				replies[1] = `{"type":"error","error":{"type":"api_error","message":"fixture"},"usage":{"input_tokens":30,"output_tokens":7}}`
			case "incomplete_limit":
				replies = []string{`{"type":"message","content":[],"stop_reason":"end_turn","usage":{"input_tokens":2,"output_tokens":3}}`}
				wantReports, wantSent = llmMaxRetries()+1, llmMaxRetries()+1
				wantInput, wantOutput = 2*wantReports, 3*wantReports
			}
			ws := newWebToolServer(t, replies...)
			c := newContinuationClient(t, ws.server.URL, window, config.AnthropicProviderConfig{WebFetch: true})
			ctx, tracker := trackedContext(t)
			observer := &anthropicAttemptObserver{}
			ctx = usage.WithObserver(ctx, observer)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			if kind == "cancel" {
				c.httpClient.Transport = &cancelAfterFirstResponse{base: c.httpClient.Transport, cancel: cancel}
			}
			if kind == "unreadable_next" {
				base := c.httpClient.Transport
				if base == nil {
					base = http.DefaultTransport
				}
				attempts := 0
				c.httpClient.Transport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
					response, err := base.RoundTrip(r)
					attempts++
					if err == nil && attempts == 2 {
						response.Body = struct {
							io.Reader
							io.Closer
						}{iotest.ErrReader(io.ErrUnexpectedEOF), response.Body}
					}
					return response, err
				})
			}
			response, err := c.CompleteWithTools(ctx, "sys", "q", nil)
			if kind == "complete" || kind == "missing_pause" || kind == "missing_final" {
				if err != nil || response == nil || response.Text != "answer" || response.Usage.InputTokens != wantInput || response.Usage.OutputTokens != wantOutput {
					t.Fatalf("continuation changed: %+v / %v", response, err)
				}
				messages := ws.request(t, 1)["messages"].([]any)
				if len(messages) != 2 || messages[1].(map[string]any)["role"] != "assistant" {
					t.Errorf("paused replay lost: %+v", messages)
				}
			} else if err == nil || response != nil {
				t.Fatalf("expected terminal error: %+v / %v", response, err)
			}
			if kind == "cancel" && !errors.Is(err, context.Canceled) {
				t.Errorf("cancellation identity: %v", err)
			}
			if kind == "window" && !errors.Is(err, ErrInputWindowExceeded) {
				t.Errorf("admission identity: %v", err)
			}
			if ws.sent() != wantSent {
				t.Errorf("sent=%d want=%d", ws.sent(), wantSent)
			}
			assertAnthropicAttempts(t, observer, wantReports, wantInput, wantOutput, unknown, usageOpChat)
			if got := tracker.Stats().TotalProject.Total; got != int64(wantInput+wantOutput) {
				t.Errorf("tracker tokens=%d want=%d", got, wantInput+wantOutput)
			}
		})
	}
}
