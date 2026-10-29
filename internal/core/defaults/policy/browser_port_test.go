package policy

import (
	"os"
	"reflect"
	"testing"

	"codenerd/internal/mangle"
)

// Fixtures reproduce BrowserNERD's Apache-2.0 diagnosis and asset cases on
// codeNERD's production policy, including its Decl-directed fact conversion.
func browserFact(predicate string, args ...any) mangle.Fact {
	return mangle.Fact{Predicate: predicate, Args: args}
}

func browserParams() []mangle.Fact {
	values := map[string]int64{
		"slow_api_ms": 1000, "causal_bucket_ms": 2000,
		"unhydrated_age_ms": 3000, "attended_navigation_window_ms": 0,
		"priority_primary": 100, "priority_button": 80, "priority_input": 78,
		"priority_select": 72, "priority_checkbox": 68, "priority_radio": 66,
		"priority_link": 60, "priority_tab": 64, "priority_combobox_click": 62,
		"priority_combobox_type": 76, "priority_menuitem": 60,
		"priority_option": 58, "priority_clickable": 56,
	}
	facts := make([]mangle.Fact, 0, len(values))
	for key, value := range values {
		facts = append(facts, browserFact("config_param", "/browser_"+key, value))
	}
	return facts
}

func browserEngine(t *testing.T, facts ...mangle.Fact) *mangle.Engine {
	t.Helper()
	return browserEngineWithParams(t, browserParams(), facts...)
}

func browserEngineWithParams(t *testing.T, params []mangle.Fact, facts ...mangle.Fact) *mangle.Engine {
	t.Helper()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	for _, path := range []string{"config_params.mg", "../schemas_browser.mg", "browser.mg"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := engine.LoadSchemaString(string(content)); err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
	}
	if err := engine.AddFacts(append(params, facts...)); err != nil {
		t.Fatal(err)
	}
	return engine
}

func browserRows(t *testing.T, engine *mangle.Engine, predicate string) []mangle.Fact {
	t.Helper()
	rows, err := engine.GetFacts(predicate)
	if err != nil {
		t.Fatalf("GetFacts(%s): %v", predicate, err)
	}
	return rows
}

func browserContains(t *testing.T, engine *mangle.Engine, predicate string, want []any) {
	t.Helper()
	rows := browserRows(t, engine, predicate)
	for _, row := range rows {
		if reflect.DeepEqual(row.Args, want) {
			return
		}
	}
	t.Fatalf("%s lacks %v; got %v", predicate, want, rows)
}

func browserHTTP(s, r, url, resource string, status, start, done int64) []mangle.Fact {
	return []mangle.Fact{
		browserFact("net_request", s, r, "GET", url, "other", start),
		browserFact("net_response", s, r, status, int64(1), int64(1)),
		browserFact("net_http_error", s, r, url, status, resource, done),
	}
}

func TestBrowserResourceTypeFailureRules(t *testing.T) {
	for _, resource := range []struct {
		name  string
		asset bool
	}{
		{"Image", true}, {"Stylesheet", true}, {"Font", true}, {"Media", true},
		{"Manifest", true}, {"TextTrack", true}, {"Other", true},
		{"Script", false}, {"XHR", false}, {"Fetch", false}, {"Document", false}, {"Unknown", false},
	} {
		t.Run(resource.name, func(t *testing.T) {
			for _, status := range []int64{399, 400, 404, 500, 503} {
				engine := browserEngine(t, browserHTTP("s", "r", "https://example.test/favicon.ico", resource.name, status, 10, 20)...)
				want := 0
				if status >= 500 || (status >= 400 && !resource.asset) {
					want = 1
				}
				for _, predicate := range []string{"failed_request", "failed_request_at", "failed_request_done"} {
					if rows := browserRows(t, engine, predicate); len(rows) != want {
						t.Fatalf("%s status=%d: %v, want %d rows", predicate, status, rows, want)
					}
				}
			}
		})
	}
}

func TestBrowserTransportAndRetainedFailures(t *testing.T) {
	engine := browserEngine(t,
		browserFact("net_request", "s", "refused", "GET", "https://example.test/api", "fetch", int64(100)),
		browserFact("net_loading_failed", "s", "refused", "net::ERR_CONNECTION_REFUSED", "false", int64(130)),
		browserFact("net_request", "s", "aborted", "GET", "https://example.test/search", "fetch", int64(200)),
		browserFact("net_loading_failed", "s", "aborted", "net::ERR_ABORTED", "true", int64(210)),
		browserFact("net_failure", "s", "aborted", "net::ERR_ABORTED", "", int64(210)),
		browserFact("net_request", "s", "stream", "GET", "https://example.test/stream", "fetch", int64(300)),
		browserFact("net_response", "s", "stream", int64(200), int64(1), int64(1)),
		browserFact("net_loading_failed", "s", "stream", "net::ERR_HTTP2_PROTOCOL_ERROR", "false", int64(900)),
		browserFact("net_loading_failed", "s", "retained", "net::ERR_CONNECTION_RESET", "false", int64(1000)),
		browserFact("console_event", "s", "error", "boom", int64(1001)),
		browserFact("net_http_error", "other", "retained-http", "https://example.test/api", int64(500), "Fetch", int64(500)),
	)
	if rows := browserRows(t, engine, "network_failure"); len(rows) != 3 {
		t.Fatalf("network_failure = %v", rows)
	}
	browserContains(t, engine, "network_failure", []any{"s", "retained", "", "net::ERR_CONNECTION_RESET", int64(1000)})
	browserContains(t, engine, "caused_by", []any{"s", "boom", "retained"})
	browserContains(t, engine, "failed_request_at", []any{"other", "retained-http", "https://example.test/api", int64(500), int64(500)})
}

func TestBrowserBucketedCausality(t *testing.T) {
	cases := []struct {
		name    string
		fail    int64
		errorTs int64
		want    int
	}{
		{"same bucket", 4100, 4300, 1}, {"previous bucket", 3999, 4001, 1},
		{"exact boundary excluded", 2000, 4000, 0}, {"older excluded", 1000, 5000, 0},
		{"future excluded", 4301, 4300, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := browserHTTP("s", "r", "https://example.test/api", "Fetch", 500, 1, tc.fail)
			facts = append(facts, browserFact("console_event", "s", "error", "boom", tc.errorTs))
			engine := browserEngine(t, facts...)
			if rows := browserRows(t, engine, "caused_by"); len(rows) != tc.want {
				t.Fatalf("caused_by = %v, want %d rows", rows, tc.want)
			}
		})
	}
	t.Run("nearest response rather than request start", func(t *testing.T) {
		facts := browserHTTP("s", "older", "https://example.test/old", "Fetch", 500, 3900, 3950)
		facts = append(facts, browserHTTP("s", "nearest", "https://example.test/new", "Fetch", 503, 10, 4100)...)
		facts = append(facts, browserHTTP("s", "asset", "https://example.test/logo", "Image", 404, 4190, 4200)...)
		facts = append(facts, browserHTTP("other", "cross-tab", "https://example.test/api", "Fetch", 500, 4200, 4250)...)
		facts = append(facts, browserFact("console_event", "s", "error", "boom", int64(4300)))
		engine := browserEngine(t, facts...)
		rows := browserRows(t, engine, "caused_by")
		if len(rows) != 1 {
			t.Fatalf("caused_by = %v", rows)
		}
		browserContains(t, engine, "caused_by", []any{"s", "boom", "nearest"})
		browserContains(t, engine, "root_cause_at", []any{"s", "boom", "network", "https://example.test/new", int64(4300)})
		for _, row := range browserRows(t, engine, "root_cause_at") {
			if row.Args[1] == "boom" && row.Args[2] == "console" {
				t.Fatalf("bound negation did not suppress fallback: %v", row)
			}
		}
	})
}

func TestBrowserStorageExpiry(t *testing.T) {
	engine := browserEngine(t,
		browserFact("browser_observed_at", "s", int64(5000)),
		browserFact("browser_observed_at", "s", int64(1000)),
		browserFact("browser_observed_at", "other", int64(1000)),
		browserFact("storage_entry", "s", "localStorage", "expired", "jwt", int64(4999)),
		browserFact("storage_entry", "s", "cookie", "at-boundary", "opaque", int64(5000)),
		browserFact("storage_entry", "s", "sessionStorage", "fresh", "jwt", int64(5001)),
		browserFact("storage_entry", "s", "localStorage", "unknown", "jwt", int64(0)),
		browserFact("storage_entry", "s", "localStorage", "json", "json", int64(0)),
		browserFact("storage_entry", "other", "localStorage", "expired", "jwt", int64(4999)),
		browserFact("net_http_error", "s", "auth", "https://example.test/api", int64(401), "XHR", int64(4000)),
	)
	if rows := browserRows(t, engine, "auth_expired"); len(rows) != 2 {
		t.Fatalf("auth_expired = %v", rows)
	}
	browserContains(t, engine, "auth_failed_request", []any{"s", "auth", "localStorage", "expired"})
	browserContains(t, engine, "root_cause_at", []any{"s", "https://example.test/api", "authentication", "expired", int64(4000)})
}

func TestBrowserThresholdsComeFromConfig(t *testing.T) {
	for _, tc := range []struct {
		name  string
		width int64
		want  int
	}{
		{"missing", -1, 0}, {"zero", 0, 0}, {"narrow", 50, 0}, {"wide", 500, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			params := browserParams()
			filtered := make([]mangle.Fact, 0, len(params))
			for _, param := range params {
				if param.Args[0] != "/browser_causal_bucket_ms" {
					filtered = append(filtered, param)
				}
			}
			if tc.width >= 0 {
				filtered = append(filtered, browserFact("config_param", "/browser_causal_bucket_ms", tc.width))
			}
			facts := browserHTTP("s", "r", "https://example.test/api", "Fetch", 500, 100, 1000)
			facts = append(facts, browserFact("console_event", "s", "error", "boom", int64(1100)))
			engine := browserEngineWithParams(t, filtered, facts...)
			if rows := browserRows(t, engine, "caused_by"); len(rows) != tc.want {
				t.Fatalf("width=%d: %v", tc.width, rows)
			}
			if rows := browserRows(t, engine, "config_param_missing"); (len(rows) > 0) != (tc.width < 0) {
				t.Fatalf("missing config: %v", rows)
			}
		})
	}
	params := browserParams()
	for i, param := range params {
		if param.Args[0] == "/browser_priority_button" {
			params[i].Args[1] = int64(99)
		}
	}
	engine := browserEngineWithParams(t, params,
		browserFact("interactive", "s", "ref", "button", "Control", "click"),
		browserFact("element_enabled", "s", "ref", "true"),
	)
	browserContains(t, engine, "action_candidate", []any{"s", "ref", "Control", "click", int64(99), "enabled_button"})
}

func TestBrowserActionPriorityAndHydration(t *testing.T) {
	types := []struct {
		kind, action string
		priority     int64
		reason       string
	}{
		{"button", "click", 80, "enabled_button"}, {"input", "type", 78, "enabled_input"},
		{"select", "select", 72, "enabled_select"}, {"checkbox", "toggle", 68, "toggle_control"},
		{"radio", "toggle", 66, "radio_control"}, {"link", "click", 60, "link_click"},
		{"tab", "click", 64, "tab"}, {"combobox", "click", 62, "open_combobox"},
		{"combobox", "type", 76, "enabled_combobox_input"}, {"menuitem", "click", 60, "menu_item"},
		{"option", "click", 58, "option"}, {"clickable", "click", 56, "clickable_element"},
	}
	for _, tc := range types {
		t.Run(tc.kind+"_"+tc.action, func(t *testing.T) {
			engine := browserEngine(t,
				browserFact("interactive", "s", "ref", tc.kind, "Control", tc.action),
				browserFact("element_enabled", "s", "ref", "true"),
			)
			browserContains(t, engine, "action_candidate", []any{"s", "ref", "Control", tc.action, tc.priority, tc.reason})
		})
	}
	t.Run("dead disabled honeypot and foreign", func(t *testing.T) {
		facts := []mangle.Fact{
			browserFact("current_url", "s", "https://example.test/form"),
			browserFact("page_framework", "s", "https://example.test/form", int64(1000)),
			browserFact("browser_observed_at", "s", int64(4000)),
			browserFact("element_hydration", "s", "dead", "dead"),
			browserFact("element_hydration", "s", "unhydrated", "unhydrated"),
			browserFact("element_hydration", "s", "foreign", "foreign"),
			browserFact("element_hydration", "s", "link", "dead"),
			browserFact("computed_style", "s:trap", "display", "none"),
			browserFact("computed_style", "other:foreign", "display", "none"),
			browserFact("browser_control_attribute", "s", "primary", "type", "submit"),
			browserFact("element_enabled", "s", "disabled", "false"),
			browserFact("dead_control", "other", "foreign", "Control"),
		}
		for _, ref := range []string{"dead", "unhydrated", "foreign", "disabled", "trap", "primary", "link"} {
			kind := "button"
			if ref == "link" {
				kind = "link"
			}
			facts = append(facts, browserFact("interactive", "s", ref, kind, "Control", "click"))
			facts = append(facts, browserFact("element_enabled", "s", ref, "true"))
		}
		engine := browserEngine(t, facts...)
		browserContains(t, engine, "foreign_control", []any{"s", "foreign", "Control"})
		browserContains(t, engine, "dead_control", []any{"s", "unhydrated", "Control"})
		browserContains(t, engine, "action_candidate", []any{"s", "primary", "Control", "click", int64(100), "primary_action"})
		rows := browserRows(t, engine, "action_candidate")
		if len(rows) != 4 {
			t.Fatalf("only foreign, link and two primary ranks expected: %v", rows)
		}
	})
	t.Run("hydration requires configured age and absent hydration witness", func(t *testing.T) {
		for _, tc := range []struct {
			age      int64
			hydrated bool
			want     int
		}{
			{2999, false, 0}, {3000, false, 1}, {5000, true, 0},
		} {
			facts := []mangle.Fact{
				browserFact("page_framework", "s", "https://example.test/form", int64(1000)),
				browserFact("browser_observed_at", "s", int64(1000)+tc.age),
			}
			if tc.hydrated {
				facts = append(facts, browserFact("page_hydrated", "s", "https://example.test/form"))
			}
			engine := browserEngine(t, facts...)
			if rows := browserRows(t, engine, "unhydrated_page"); len(rows) != tc.want {
				t.Fatalf("age=%d hydrated=%v: %v", tc.age, tc.hydrated, rows)
			}
		}
	})
}

func TestBrowserDialogDefaults(t *testing.T) {
	for _, tc := range []struct {
		name     string
		policies []string
		want     string
	}{
		{"absent", nil, "dismiss"}, {"accept", []string{"accept"}, "accept"},
		{"dismiss", []string{"dismiss"}, "dismiss"}, {"invalid", []string{"automatic"}, "dismiss"},
		{"conflict", []string{"accept", "dismiss"}, "dismiss"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := []mangle.Fact{browserFact("js_dialog", "s", "confirm", "Continue?", "pending", "", int64(1000))}
			for _, policy := range tc.policies {
				facts = append(facts, browserFact("dialog_policy", "s", policy))
			}
			facts = append(facts, browserFact("dialog_policy", "other", "accept"))
			engine := browserEngine(t, facts...)
			rows := browserRows(t, engine, "dialog_answer")
			var sessionRows []mangle.Fact
			for _, row := range rows {
				if row.Args[0] == "s" {
					sessionRows = append(sessionRows, row)
				}
			}
			if len(sessionRows) != 1 || !reflect.DeepEqual(sessionRows[0].Args, []any{"s", tc.want}) {
				t.Fatalf("dialog_answer = %v", sessionRows)
			}
			browserContains(t, engine, "dialog_answer", []any{"other", "accept"})
		})
	}
}

func TestBrowserBatchEffects(t *testing.T) {
	facts := []mangle.Fact{
		browserFact("act_batch", "s", "batch", int64(1000)),
		browserFact("console_event", "s", "error", "old", int64(999)),
		browserFact("console_event", "s", "error", "boundary", int64(1000)),
		browserFact("console_event", "s", "error", "new", int64(1001)),
		browserFact("console_event", "s", "warn", "noise", int64(1001)),
		browserFact("console_event", "other", "error", "cross-tab", int64(1001)),
		browserFact("js_dialog", "s", "alert", "hi", "false", "policy", int64(1002)),
		browserFact("download", "s", "guid", "https://example.test/file", "file.txt", "started", int64(1003)),
		browserFact("toast_notification", "s", "saved", "success", "dom", int64(1004)),
		browserFact("toast_notification", "s", "old toast", "error", "dom", int64(999)),
	}
	facts = append(facts, browserHTTP("s", "api", "https://example.test/api", "Fetch", 400, 999, 1005)...)
	facts = append(facts, browserHTTP("s", "asset", "https://example.test/style", "Stylesheet", 404, 1000, 1005)...)
	facts = append(facts, browserHTTP("s", "server-asset", "https://example.test/image", "Image", 500, 1000, 1006)...)
	engine := browserEngine(t, facts...)
	rows := browserRows(t, engine, "act_effect")
	if len(rows) != 6 {
		t.Fatalf("act_effect = %v, want six post-batch effects", rows)
	}
	browserContains(t, engine, "act_http_effect", []any{"s", "batch", "api", "https://example.test/api", int64(400), int64(1005)})
	for _, row := range rows {
		if row.Args[0] != "s" || row.Args[5].(int64) <= 1000 {
			t.Fatalf("out of scope effect: %v", row)
		}
	}
}

func TestBrowserUnattendedNavigationAndRetention(t *testing.T) {
	engine := browserEngine(t,
		browserFact("attended", "s", int64(500)),
		browserFact("attended", "s", int64(1000)),
		browserFact("navigation_event", "s", "https://example.test/old", int64(600)),
		browserFact("navigation_event", "s", "https://example.test/at", int64(1000)),
		browserFact("navigation_event", "s", "https://example.test/redirect", int64(1001)),
		browserFact("navigation_event", "other", "https://example.test/unmarked", int64(600)),
	)
	if rows := browserRows(t, engine, "unattended_navigation"); len(rows) != 2 {
		t.Fatalf("unattended_navigation = %v", rows)
	}
	browserContains(t, engine, "unattended_navigation", []any{"s", "https://example.test/redirect", int64(1001)})
	for _, name := range []string{"net_http_error", "net_loading_failed", "net_failure_body", "console_event", "browser_log", "ws_event", "page_load_failed", "js_dialog", "download"} {
		browserContains(t, engine, "failure_evidence_predicate", []any{name})
	}
	for _, row := range browserRows(t, engine, "failure_evidence_predicate") {
		if row.Args[0] == "dom_updated" {
			t.Fatal("polling must not acquire failure retention")
		}
	}
}
