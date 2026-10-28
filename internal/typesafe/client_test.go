package typesafe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const answersJSON = `{"model":"jev-1.14.0","usage":{"input_tokens":120,"output_tokens":12},"answers":{
 "billing":{"type":"noul","noul":0.98},
 "tone":{"type":"choice","choice":"calm","confidence":0.7,"probabilities":{"angry":0.15,"calm":0.8,"excited":0.05}},
 "urgency":{"type":"score","score":1.7,"confidence":0.55,"legend":{"0":"Can wait","1":"This week","2":"Today"},"probabilities":{"0":0.1,"1":0.1,"2":0.8}},
 "spot":{"type":"box","box":{"top":0.2,"bottom":0.6,"left":0.1,"right":0.5},"confidence":null}}}`

func testQuestions() map[string]Question {
	return map[string]Question{
		"billing": Noul{Instructions: "Is this about billing?", Criteria: &NoulCriteria{True: "a charge", False: "anything else"}},
		"tone":    Choice{Instructions: "Tone?", Criteria: map[string]any{"angry": "hostile", "calm": nil, "excited": "eager"}},
		"urgency": Score{Instructions: "How urgent?", Criteria: []any{"Can wait", "This week", "Today"}},
	}
}

func newTestClient(t *testing.T, h http.HandlerFunc, mutate func(*Options)) (*Client, *[]time.Duration) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	o := Options{Model: "jev-latest", BaseURL: srv.URL + "/", APIKey: "sekret-key", Retry: &RetryPolicy{Max: 3, Base: time.Second, Cap: 4 * time.Second, MaxWait: time.Minute}}
	if mutate != nil {
		mutate(&o)
	}
	c, err := NewClient(o)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	slept := &[]time.Duration{}
	c.sleep = func(ctx context.Context, d time.Duration) error {
		mu.Lock()
		*slept = append(*slept, d)
		mu.Unlock()
		return ctx.Err()
	}
	return c, slept
}

func reply(status int, body string, hdr ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i+1 < len(hdr); i += 2 {
			w.Header().Set(hdr[i], hdr[i+1])
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}
}

func TestAsk_RequestShapeAndAuth(t *testing.T) {
	var gotReq *http.Request
	var gotBody map[string]any
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		json.NewDecoder(r.Body).Decode(&gotBody)
		io.WriteString(w, answersJSON)
	}, func(o *Options) { o.LongContext = true })

	state := map[string]any{"message": "I was charged twice", "subject": "Duplicate charge"}
	if _, err := c.Ask(context.Background(), state, testQuestions()); err != nil {
		t.Fatal(err)
	}
	if gotReq.Method != "POST" || gotReq.URL.Path != "/v1/systemone" {
		t.Errorf("request = %s %s", gotReq.Method, gotReq.URL.Path)
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer sekret-key" {
		t.Errorf("Authorization = %q", got)
	}
	if gotReq.Header.Get("Content-Type") != "application/json" || gotReq.Header.Get("TypeSafe-Danger-Zone-Long-Context") != "true" {
		t.Errorf("headers = %v", gotReq.Header)
	}
	if gotBody["model"] != "jev-latest" {
		t.Errorf("model = %v", gotBody["model"])
	}
	if s := gotBody["state"].(map[string]any); s["subject"] != "Duplicate charge" {
		t.Errorf("state = %v", s)
	}
	qs := gotBody["questions"].(map[string]any)
	noul := qs["billing"].(map[string]any)
	if noul["type"] != "noul" || noul["criteria"].(map[string]any)["true"] != "a charge" {
		t.Errorf("noul = %v", noul)
	}
	choice := qs["tone"].(map[string]any)
	if v, ok := choice["criteria"].(map[string]any)["calm"]; !ok || v != nil {
		t.Errorf("a described-by-name option must be sent as null: %v", choice)
	}
	if lv := qs["urgency"].(map[string]any)["criteria"].([]any); len(lv) != 3 || lv[2] != "Today" {
		t.Errorf("score levels = %v", lv)
	}
}

func TestAsk_NoLongContextHeaderByDefault(t *testing.T) {
	var hdr http.Header
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header
		io.WriteString(w, answersJSON)
	}, nil)
	if _, err := c.Ask(context.Background(), "hi", testQuestions()); err != nil {
		t.Fatal(err)
	}
	if _, ok := hdr["Typesafe-Danger-Zone-Long-Context"]; ok {
		t.Error("long-context header sent while off")
	}
}

func TestResponse_EachAnswerTypeModelAndUsage(t *testing.T) {
	c, _ := newTestClient(t, reply(200, answersJSON), nil)
	resp, err := c.Ask(context.Background(), "x", testQuestions())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Model != "jev-1.14.0" || resp.Usage.InputTokens != 120 || resp.Usage.OutputTokens != 12 {
		t.Errorf("model/usage = %q %+v", resp.Model, resp.Usage)
	}
	if p, err := resp.Noul("billing"); err != nil || p != 0.98 {
		t.Errorf("Noul = %v, %v", p, err)
	}
	ch, err := resp.Choice("tone")
	if err != nil || ch.Choice != "calm" || ch.Confidence != 0.7 || ch.Probabilities["calm"] != 0.8 {
		t.Errorf("Choice = %+v, %v", ch, err)
	}
	sc, err := resp.Score("urgency")
	if err != nil || sc.Score != 1.7 || sc.Confidence != 0.55 || sc.Legend["2"] != "Today" || sc.Probabilities["2"] != 0.8 {
		t.Errorf("Score = %+v, %v", sc, err)
	}
}

func TestResponse_MissingOrMistypedAnswerIsAnError(t *testing.T) {
	c, _ := newTestClient(t, reply(200, answersJSON), nil)
	resp, _ := c.Ask(context.Background(), "x", testQuestions())
	if p, err := resp.Noul("absent"); err == nil || p != 0 || !strings.Contains(err.Error(), `"absent"`) {
		t.Errorf("missing noul: %v, %v", p, err)
	}
	if _, err := resp.Choice("absent"); err == nil {
		t.Error("missing choice accepted")
	}
	if _, err := resp.Score("absent"); err == nil {
		t.Error("missing score accepted")
	}
	if _, err := resp.Noul("tone"); err == nil || !strings.Contains(err.Error(), "choice") {
		t.Errorf("a choice read as a noul: %v", err)
	}
	if _, err := resp.Score("billing"); err == nil {
		t.Error("a noul read as a score")
	}
	if _, err := resp.Choice("spot"); err == nil {
		t.Error("a box read as a choice")
	}
}

func TestResponse_IncompleteAnswersAreErrors(t *testing.T) {
	body := `{"model":"m","usage":{"input_tokens":1,"output_tokens":1},"answers":{
	 "n":{"type":"noul"},"n2":{"type":"noul","noul":1.5},
	 "c":{"type":"choice","choice":"a"},"s":{"type":"score","score":1}}}`
	c, _ := newTestClient(t, reply(200, body), nil)
	resp, err := c.Ask(context.Background(), "x", map[string]Question{"n": Noul{Instructions: "q"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resp.Noul("n"); err == nil {
		t.Error("noul without a probability accepted")
	}
	if _, err := resp.Noul("n2"); err == nil {
		t.Error("probability above 1 accepted")
	}
	if _, err := resp.Choice("c"); err == nil {
		t.Error("choice without a distribution accepted")
	}
	if _, err := resp.Score("s"); err == nil {
		t.Error("score without a legend accepted")
	}
}

func TestAsk_422SurfacesTheOffendingField(t *testing.T) {
	body := `{"detail":[{"loc":["body","questions","urgency","score","criteria"],"msg":"List should have at most 10 items","type":"too_long"}]}`
	c, slept := newTestClient(t, reply(422, body), nil)
	resp, err := c.Ask(context.Background(), "x", testQuestions())
	var apiErr *APIError
	if resp != nil || !errors.As(err, &apiErr) || apiErr.Status != 422 {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	if !strings.Contains(err.Error(), "body.questions.urgency.score.criteria") || !strings.Contains(err.Error(), "too_long") {
		t.Errorf("error lacks the field: %v", err)
	}
	if len(*slept) != 0 {
		t.Error("a 422 was retried")
	}
}

func TestAsk_StringAndOtherStatusesAreErrors(t *testing.T) {
	for status, detail := range map[int]string{400: "max_tokens_exceeded", 401: "Invalid API key", 413: "too large", 415: "bad encoding"} {
		body, _ := json.Marshal(map[string]string{"detail": detail})
		c, slept := newTestClient(t, reply(status, string(body)), nil)
		_, err := c.Ask(context.Background(), "x", testQuestions())
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr.Status != status || !strings.Contains(err.Error(), detail) {
			t.Errorf("status %d: %v", status, err)
		}
		if len(*slept) != 0 {
			t.Errorf("status %d was retried", status)
		}
	}
}

func TestAsk_429ThenSuccessHonorsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			io.WriteString(w, `{"detail":"slow down"}`)
			return
		}
		io.WriteString(w, answersJSON)
	}, nil)
	resp, err := c.Ask(context.Background(), "x", testQuestions())
	if err != nil || resp == nil {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	if calls.Load() != 2 || len(*slept) != 1 || (*slept)[0] != 3*time.Second {
		t.Errorf("calls=%d slept=%v; want 2 calls and one 3s wait", calls.Load(), *slept)
	}
}

func TestAsk_529ExhaustsToAnErrorWithBoundedBackoff(t *testing.T) {
	var calls atomic.Int32
	c, slept := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(529)
		io.WriteString(w, `{"detail":"overloaded"}`)
	}, nil)
	resp, err := c.Ask(context.Background(), "x", testQuestions())
	var apiErr *APIError
	if resp != nil || !errors.As(err, &apiErr) || apiErr.Status != 529 {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if calls.Load() != 4 || len(*slept) != len(want) {
		t.Fatalf("calls=%d slept=%v", calls.Load(), *slept)
	}
	for i := range want {
		if (*slept)[i] != want[i] {
			t.Errorf("wait %d = %v, want %v", i, (*slept)[i], want[i])
		}
	}
}

func TestAsk_RetryAfterBeyondTheLimitIsNotSlept(t *testing.T) {
	c, slept := newTestClient(t, reply(429, `{"detail":"x"}`, "Retry-After", "3600"), nil)
	_, err := c.Ask(context.Background(), "x", testQuestions())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 429 || len(*slept) != 0 {
		t.Errorf("err=%v slept=%v", err, *slept)
	}
}

func TestAsk_ContextCancelStopsTheCall(t *testing.T) {
	release := make(chan struct{})
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body) // an unread body keeps the server from noticing a disconnect
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}, nil)
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	resp, err := c.Ask(ctx, "x", testQuestions())
	if resp != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
}

func TestAsk_ContextCancelDuringRetryWait(t *testing.T) {
	c, _ := newTestClient(t, reply(429, `{"detail":"x"}`), nil)
	ctx, cancel := context.WithCancel(context.Background())
	c.sleep = func(ctx context.Context, d time.Duration) error { cancel(); return ctx.Err() }
	_, err := c.Ask(ctx, "x", testQuestions())
	var apiErr *APIError
	if !errors.Is(err, context.Canceled) || !errors.As(err, &apiErr) {
		t.Errorf("err=%v; want canceled wrapping the 429", err)
	}
}

func TestAsk_TimeoutBoundsTheCall(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { io.Copy(io.Discard, r.Body); <-r.Context().Done() }, func(o *Options) { o.Timeout = 30 * time.Millisecond })
	if _, err := c.Ask(context.Background(), "x", testQuestions()); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err=%v", err)
	}
}

func TestAsk_MalformedReplyIsAnError(t *testing.T) {
	for _, body := range []string{`not json`, `{"model":"m","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`, `{"model":"m","answers":{"a":{"type":"noul","noul":1}}}`} {
		c, _ := newTestClient(t, reply(200, body), nil)
		if resp, err := c.Ask(context.Background(), "x", testQuestions()); err == nil || resp != nil {
			t.Errorf("body %q accepted", body)
		}
	}
}

func TestAsk_InvalidRequestsNeverReachTheServer(t *testing.T) {
	var calls atomic.Int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }, nil)
	deep := any("leaf")
	for i := 0; i < 70; i++ {
		deep = []any{deep}
	}
	opts := make(map[string]any, 256)
	for i := 0; i < 256; i++ {
		opts[strings.Repeat("o", i+1)] = "d"
	}
	cases := map[string]struct {
		state any
		qs    map[string]Question
	}{
		"no questions":      {"x", nil},
		"nil state":         {nil, testQuestions()},
		"number state":      {42, testQuestions()},
		"NaN in state":      {map[string]any{"v": nanValue()}, testQuestions()},
		"too deep":          {deep, testQuestions()},
		"no instructions":   {"x", map[string]Question{"q": Noul{}}},
		"choice no options": {"x", map[string]Question{"q": Choice{Instructions: "?"}}},
		"256 options":       {"x", map[string]Question{"q": Choice{Instructions: "?", Criteria: opts}}},
		"11 levels":         {"x", map[string]Question{"q": Score{Instructions: "?", Criteria: make([]any, 11)}}},
		"empty name":        {"x", map[string]Question{"": Noul{Instructions: "?"}}},
	}
	for name, tc := range cases {
		if resp, err := c.Ask(context.Background(), tc.state, tc.qs); err == nil || resp != nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if calls.Load() != 0 {
		t.Errorf("%d invalid request(s) reached the server", calls.Load())
	}
}

func TestListModels(t *testing.T) {
	var gotReq *http.Request
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		io.WriteString(w, `{"models":[{"name":"jev-latest","description":"General","release_date":"2026-09-15"}]}`)
	}, nil)
	models, err := c.ListModels(context.Background())
	if err != nil || len(models) != 1 || models[0].Name != "jev-latest" || models[0].ReleaseDate != "2026-09-15" {
		t.Fatalf("models=%v err=%v", models, err)
	}
	if gotReq.Method != "GET" || gotReq.URL.Path != "/v1/models" || gotReq.Header.Get("Authorization") != "Bearer sekret-key" {
		t.Errorf("request = %s %s %v", gotReq.Method, gotReq.URL.Path, gotReq.Header)
	}
}

func TestNewClient_RequiresEveryField(t *testing.T) {
	good := Options{Model: "m", BaseURL: "http://x.test", APIKey: "k"}
	if _, err := NewClient(good); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Options){
		"model":   func(o *Options) { o.Model = "" },
		"url":     func(o *Options) { o.BaseURL = "" },
		"key":     func(o *Options) { o.APIKey = "" },
		"timeout": func(o *Options) { o.Timeout = -1 },
	} {
		o := good
		mutate(&o)
		if _, err := NewClient(o); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	h := func(v string) http.Header { return http.Header{"Retry-After": {v}} }
	for _, tc := range []struct {
		in   string
		want time.Duration
	}{
		{"", 0}, {"7", 7 * time.Second}, {"-1", 0}, {"junk", 0},
		{now.Add(5 * time.Second).Format(http.TimeFormat), 5 * time.Second},
		{now.Add(-time.Minute).Format(http.TimeFormat), 0},
	} {
		if got := parseRetryAfter(h(tc.in), now); got != tc.want {
			t.Errorf("%q = %v, want %v", tc.in, got, tc.want)
		}
	}
}
