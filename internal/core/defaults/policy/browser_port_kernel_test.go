package policy

import (
	"fmt"
	"reflect"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/mangle"
)

func TestAssetHttpErrorDiscrimination(t *testing.T) {
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("production kernel boot: %v", err)
	}
	assert := func(facts []mangle.Fact) {
		t.Helper()
		batch := make([]core.Fact, len(facts))
		for i, fact := range facts {
			batch[i] = core.Fact{Predicate: fact.Predicate, Args: fact.Args}
		}
		if err := kernel.AssertBatch(batch); err != nil {
			t.Fatal(err)
		}
	}
	assert(browserParams())
	cases := []struct {
		resource string
		status   int64
		failed   bool
	}{
		{"Image", 404, false}, {"Stylesheet", 404, false}, {"Font", 403, false},
		{"Media", 404, false}, {"Manifest", 404, false}, {"TextTrack", 404, false},
		{"Other", 404, false}, {"Script", 404, true}, {"XHR", 401, true},
		{"Fetch", 429, true}, {"Document", 404, true}, {"Unknown", 400, true},
		{"Image", 500, true}, {"Stylesheet", 503, true}, {"Font", 599, true},
		{"Fetch", 399, false},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprintf("%s_%d", tc.resource, tc.status), func(t *testing.T) {
			session := fmt.Sprintf("asset-session-%d", i)
			// A misleading URL must not override the CDP resource type.
			url := "https://example.test/favicon.ico"
			assert(browserHTTP(session, "r", url, tc.resource, tc.status, 100, 120))
			for _, predicate := range []string{"failed_request", "failed_request_at", "failed_request_done"} {
				rows, err := kernel.Query(predicate)
				if err != nil {
					t.Fatal(err)
				}
				var matches []core.Fact
				for _, row := range rows {
					if row.Args[0] == session {
						matches = append(matches, row)
					}
				}
				want := 0
				if tc.failed {
					want = 1
				}
				if len(matches) != want {
					t.Fatalf("%s = %v, want %d rows", predicate, matches, want)
				}
				if want == 1 && predicate == "failed_request_at" && matches[0].Args[4] != int64(100) {
					t.Fatalf("request timestamp changed: %v", matches)
				}
			}
		})
	}
}

func TestReasonNetworkLevelFailures(t *testing.T) {
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("production kernel boot: %v", err)
	}
	facts := []mangle.Fact{
		browserFact("net_request", "s", "refused", "GET", "https://example.test/api/data", "fetch", int64(100)),
		browserFact("net_loading_failed", "s", "refused", "net::ERR_CONNECTION_REFUSED", "false", int64(130)),
		browserFact("net_request", "s", "aborted", "GET", "https://example.test/api/search", "fetch", int64(200)),
		browserFact("net_loading_failed", "s", "aborted", "net::ERR_ABORTED", "true", int64(210)),
		browserFact("net_failure", "s", "aborted", "net::ERR_ABORTED", "", int64(210)),
		browserFact("net_request", "s", "stream", "GET", "https://example.test/api/stream", "fetch", int64(300)),
		browserFact("net_response", "s", "stream", int64(200), int64(1), int64(1)),
		browserFact("net_loading_failed", "s", "stream", "net::ERR_HTTP2_PROTOCOL_ERROR", "false", int64(900)),
		browserFact("console_event", "s", "error", "stream stopped", int64(950)),
		browserFact("console_event", "other", "error", "another tab", int64(950)),
	}
	facts = append(facts, browserHTTP("s", "img", "https://example.test/static/logo.png", "Image", 404, 220, 225)...)
	facts = append(browserParams(), facts...)
	batch := make([]core.Fact, len(facts))
	for i, fact := range facts {
		batch[i] = core.Fact{Predicate: fact.Predicate, Args: fact.Args}
	}
	if err := kernel.AssertBatch(batch); err != nil {
		t.Fatal(err)
	}
	rows, err := kernel.Query("network_failure")
	if err != nil || len(rows) != 2 {
		t.Fatalf("network_failure = %v, %v; want refusal and post-200 body failure", rows, err)
	}
	for _, row := range rows {
		if row.Args[1] != "refused" && row.Args[1] != "stream" {
			t.Fatalf("noise became a transport failure: %v", row)
		}
	}
	causes, err := kernel.Query("caused_by")
	if err != nil || len(causes) != 1 || !reflect.DeepEqual(causes[0].Args, []any{"s", "stream stopped", "stream"}) {
		t.Fatalf("caused_by = %v, %v", causes, err)
	}
	failed, err := kernel.Query("failed_request")
	if err != nil || len(failed) != 0 {
		t.Fatalf("HTTP noise = %v, %v", failed, err)
	}
}
