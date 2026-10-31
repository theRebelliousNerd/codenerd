package perception

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/iotest"

	"codenerd/internal/broker"
	"codenerd/internal/usage"
)

type acceptedReadFunc func([]byte) (int, error)

func (f acceptedReadFunc) Read(p []byte) (int, error) { return f(p) }

func TestAcceptedBodyReadFailurePreservesUsageUncertainty(t *testing.T) {
	for _, kind := range []string{"clean", "retry", "canceled"} {
		t.Run(kind, func(t *testing.T) {
			c := newTestCompatClient(t, ProviderDashScope, "http://fixture.invalid")
			ctx, tracker := trackedContext(t)
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			outer, inner := &retryUsageObserver{}, &retryUsageObserver{}
			ctx = usage.WithObserver(usage.WithObserver(ctx, outer), inner)
			requests, closes := 0, 0
			closed := false
			c.httpClient.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				body := io.Reader(strings.NewReader(`{"choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":20}}`))
				header := http.Header{"Retry-After": []string{"0"}}
				if requests == 1 && kind == "retry" {
					body = io.MultiReader(strings.NewReader(`{"choices":`), iotest.ErrReader(io.ErrUnexpectedEOF))
				}
				if kind == "canceled" {
					header.Set("Retry-After", "1")
					body = acceptedReadFunc(func([]byte) (int, error) { cancel(); return 0, context.Canceled })
				}
				return &http.Response{StatusCode: http.StatusOK, Header: header,
					Body: &trackingReadCloser{ReadCloser: io.NopCloser(body), closed: &closed, closes: &closes}}, nil
			})
			meter := broker.NewMeter(broker.MeterConfig{})
			cfg := meter.ConfigFor(broker.ProviderCreds{Provider: string(ProviderDashScope), Model: c.GetModel()})
			receipts := broker.NewRingSink(4)
			cfg.Sink = receipts
			client, err := broker.Wrap(c, cfg)
			if err != nil {
				t.Fatal(err)
			}
			answer, err := client.CompleteWithTools(ctx, "system", "request", nil)
			wantRequests, wantOutput, wantReports, wantTokens := 1, 20, 1, 30
			wantStatus := broker.UsageReported
			if kind == "retry" {
				wantRequests, wantStatus = 2, broker.UsagePartial
			}
			if kind == "canceled" {
				wantOutput, wantReports, wantTokens, wantStatus = 0, 0, 0, broker.UsageUnreported
				if !errors.Is(err, context.Canceled) || answer != nil {
					t.Fatalf("cancellation identity changed: answer=%+v err=%v", answer, err)
				}
			} else if err != nil || answer == nil || answer.Text != "answer" {
				t.Fatalf("completion behavior changed: answer=%+v err=%v", answer, err)
			}
			if requests != wantRequests || closes != wantRequests || !closed {
				t.Errorf("requests=%d closes=%d; want %d", requests, closes, wantRequests)
			}
			for name, observer := range map[string]*retryUsageObserver{"outer": outer, "inner": inner} {
				if observer.reports != wantReports || observer.output != wantOutput || (observer.unknown > 0) != (kind != "clean") {
					t.Errorf("%s reports=%d output=%d unknown=%d", name, observer.reports, observer.output, observer.unknown)
				}
			}
			if got := tracker.Stats().TotalProject.Total; got != int64(wantTokens) {
				t.Errorf("known tokens=%d, want %d without a synthetic report", got, wantTokens)
			}
			got := receipts.Receipts()
			if len(got) != 1 || got[0].UsageStatus != wantStatus || got[0].Actual.OutputTokens != int64(wantOutput) {
				t.Fatalf("wrong receipt certainty or counts: %+v", got)
			}
		})
	}
}
