package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/perception"
	"codenerd/internal/typesafe"
)

// jev_correction_test.go: the learning loop replaces a turn only when the
// kernel derives learning_correction (jev_correction.go,
// policy/jev_learning.mg). Jev's judgment outranks the rebuke-phrase reading;
// a turn Jev did not judge falls back to it.

// correctionJev answers corrects_last_turn with p and every other noul 0.5,
// and records which questions it was asked. fail makes every call a 500.
type correctionJev struct {
	mu    sync.Mutex
	p     float64
	fail  bool
	asked map[string]int
}

func (f *correctionJev) attach(t *testing.T, m Model, sec config.TypeSafeConfig) Model {
	t.Helper()
	f.asked = map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		for name := range req.Questions {
			f.asked[name]++
		}
		if f.fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		answers := map[string]any{}
		for name := range req.Questions {
			p := 0.5
			if name == learnjudge.CorrectsLastTurn {
				p = f.p
			}
			answers[name] = map[string]any{"type": "noul", "noul": p}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-fake", "answers": answers, "usage": map[string]any{"input_tokens": 100, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k", Retry: &typesafe.RetryPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	m.jev = c
	sec.Model, sec.BaseURL, sec.APIKey = "jev-fake", srv.URL, "k"
	m.Config.TypeSafe = &sec
	return m
}

func (f *correctionJev) timesAsked() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.asked[learnjudge.CorrectsLastTurn]
}

// correctionModel is a chat with one finished exchange and the new message in
// history, as handleSubmit leaves it.
func correctionModel(t *testing.T, input string) Model {
	t.Helper()
	m, _ := journeyModel(t, config.WorkModeYolo, perception.Intent{
		Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Understood.",
	})
	now := time.Now()
	m.history = []Message{
		{Role: "user", Content: "make Max return 0 for an empty slice", Time: now},
		{Role: "assistant", Content: "Done: Min in stats.go now returns 0 for an empty slice.", Time: now},
		{Role: "user", Content: input, Time: now},
	}
	return m
}

func correctionSource(t *testing.T, m Model, input string) (string, bool) {
	t.Helper()
	msg := awaitProcessStream(t, m.processInput(input)())
	lc, ok := msg.(learningCorrectionMsg)
	return lc.source, ok
}

func TestCorrection_TruthTable(t *testing.T) {
	failing := func() *correctionJev { return &correctionJev{fail: true} }
	cases := []struct {
		name    string
		input   string
		jev     *correctionJev
		sec     config.TypeSafeConfig
		noJev   bool
		want    string // "" means the turn routes as usual
		askedOK bool
	}{
		{name: "jev reads a correction", input: "no, I said Max, not Min", jev: &correctionJev{p: 0.93}, want: "jev", askedOK: true},
		{name: "jev correction without any rebuke word", input: "that's the other function", jev: &correctionJev{p: 0.88}, want: "jev", askedOK: true},
		{name: "a bare stop is not a correction", input: "stop", jev: &correctionJev{p: 0.04}, askedOK: true},
		{name: "jev no outranks the rebuke phrase", input: "wrong", jev: &correctionJev{p: 0.12}, askedOK: true},
		{name: "unsettled acts on nothing", input: "you made a mistake", jev: &correctionJev{p: 0.5}, askedOK: true},
		{name: "jev failing falls back to the rebuke phrase", input: "that's wrong", jev: failing(), want: "rebuke_phrase", askedOK: true},
		{name: "jev failing and no rebuke phrase", input: "stop", jev: failing(), askedOK: true},
		{name: "switch off falls back to the rebuke phrase", input: "wrong", jev: &correctionJev{p: 0.01}, sec: config.TypeSafeConfig{CorrectionJudgments: boolPtr(false)}, want: "rebuke_phrase"},
		{name: "no jev configured falls back to the rebuke phrase", input: "bad bot", noJev: true, want: "rebuke_phrase"},
		{name: "no jev and a task that mentions a failure", input: "Mean in stats.go is wrong - go test says TestMean fails and it returns 6 instead of 4", noJev: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := correctionModel(t, tc.input)
			if !tc.noJev {
				m = tc.jev.attach(t, m, tc.sec)
			}
			source, ok := correctionSource(t, m, tc.input)
			if tc.want == "" && ok {
				t.Fatalf("turn became the learning loop (source %s), want it routed as usual", source)
			}
			if tc.want != "" && (!ok || source != tc.want) {
				t.Fatalf("learning_correction = %q (%v), want %q", source, ok, tc.want)
			}
			if !tc.noJev {
				if asked := tc.jev.timesAsked() > 0; asked != tc.askedOK {
					t.Fatalf("correction question asked = %v, want %v", asked, tc.askedOK)
				}
			}
		})
	}
}

// The kernel's decision runs the learning loop through Update.
func TestCorrection_MessageStartsTheLearningLoop(t *testing.T) {
	m := correctionModel(t, "no, I said Max")
	next, cmd := m.Update(learningCorrectionMsg{input: "no, I said Max", source: "jev"})
	if cmd == nil {
		t.Fatal("the learning loop issued no command")
	}
	if got := lastAssistant(next.(Model)); got == "" || got == "Done: Min in stats.go now returns 0 for an empty slice." {
		t.Fatalf("the learning loop did not tell the user it is learning: %q", got)
	}
}

// A first message has no reply to correct: nothing is asked.
func TestCorrection_FirstTurnAsksNothing(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo, perception.Intent{Category: "/query", Verb: "/converse", Target: "none", Confidence: 0.9, Response: "Hi."})
	m.history = []Message{{Role: "user", Content: "wrong", Time: time.Now()}}
	jev := &correctionJev{p: 0.99}
	m = jev.attach(t, m, config.TypeSafeConfig{})
	source, ok := correctionSource(t, m, "wrong")
	if ok {
		t.Fatalf("a first message became a correction (%s)", source)
	}
	if jev.timesAsked() != 0 {
		t.Fatal("asked about a reply that does not exist")
	}
}

func TestLastExchange(t *testing.T) {
	h := []Message{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "reply one"},
		{Role: "user", Content: "second"},
		{Role: "assistant", Content: "reply two"},
		{Role: "user", Content: "no, the other one"},
	}
	got := lastExchange(h, "no, the other one")
	if got.PreviousRequest != "second" || got.LastReply != "reply two" || got.Message != "no, the other one" {
		t.Fatalf("lastExchange = %+v", got)
	}
	if got := lastExchange(h[:1], "first"); got.LastReply != "" {
		t.Fatalf("no reply yet, got %+v", got)
	}
}

func boolPtr(b bool) *bool { return &b }
