package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/perception"
	"codenerd/internal/problemlog"
	"codenerd/internal/typesafe"
	"codenerd/internal/usage"
)

// jev_uses_test.go: every Jev use from its config switch to what the user
// sees, with Jev slow, failing and switched off (jev_turn.go,
// policy/jev_evidence.mg, Docs/architecture/jev-turn-judgments.md).

// contentJev answers each noul by the first key its question text contains
// (0.5 when none does), after delay. fail makes every call a 500.
type contentJev struct {
	probs map[string]float64
	delay time.Duration
	fail  bool
	calls atomic.Int32
}

func (f *contentJev) attach(t *testing.T, m Model, sec config.TypeSafeConfig) Model {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		if f.delay > 0 {
			select {
			case <-time.After(f.delay):
			case <-r.Context().Done():
				return
			}
		}
		if f.fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		answers := map[string]any{}
		for name, q := range req.Questions {
			if a, ok := fakeChoiceAnswer(t, q, nil); ok {
				answers[name] = a
				continue
			}
			p := 0.5
			if v, ok := f.probs[name]; ok {
				p = v
			}
			for key, v := range f.probs {
				if strings.Contains(string(q), key) {
					p = v
				}
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

// A Jev slower than turn_judgment_timeout holds routing no longer than the
// timeout, and its late answer is no evidence.
func TestJevTurn_SlowJevIsBoundedByTheTurnTimeout(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	jev := &contentJev{probs: map[string]float64{"changes_files": 0.05}, delay: 5 * time.Second}
	m = jev.attach(t, m, config.TypeSafeConfig{TurnJudgmentTimeout: "100ms"})

	began := time.Now()
	m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "fix the README typo", nil))
	if waited := time.Since(began); waited > 2*time.Second {
		t.Fatalf("routing waited %s for a slow Jev, want about the 100ms bound", waited)
	}
	if rows, _ := m.kernel.Query("jev_judgment"); len(rows) != 0 {
		t.Fatalf("a timed-out call left evidence: %v", rows)
	}
}

// Every answered Jev call is billed to the session's usage tracker under its
// use, wherever it runs.
func TestJevTurn_JudgmentsAreBilledToTheUsageTracker(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	tracker, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tracker.Close() })
	m.usageTracker = tracker
	m = (&contentJev{}).attach(t, m, config.TypeSafeConfig{})

	m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "fix the README typo", nil))
	stats := tracker.Stats()
	if got := stats.ByOperation["jev_turn_judgments"].Input; got != 100 {
		t.Fatalf("jev_turn_judgments billed %d input tokens, want 100 (%+v)", got, stats.ByOperation)
	}
	if got := stats.ByProvider[typesafe.Provider].Input; got != 100 {
		t.Fatalf("provider %s billed %d input tokens, want 100", typesafe.Provider, got)
	}
}

// Each use's switch turns that use off and only that use: Jev is never
// called for it, and the chat behaves as without Jev.
func TestJevUses_EachSwitchOffAsksNothing(t *testing.T) {
	off := false
	m, _ := journeyModel(t, config.WorkModeYolo)
	jev := &contentJev{probs: map[string]float64{"finding": 0.05, "problem": 0.05}}
	m = jev.attach(t, m, config.TypeSafeConfig{TurnJudgments: &off, RetrievalRelevance: &off, ReviewChecks: &off, DiagnosisTriage: &off})
	if err := os.WriteFile(filepath.Join(m.workspace, "calc.go"), []byte("package calc\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if ch := m.startTurnJudgments(m.shutdownCtx, "fix it", nil); ch != nil {
		t.Error("turn_judgments false still judges the turn")
	}
	if m.relevanceUse().Judge != nil {
		t.Error("retrieval_relevance false still judges retrieval")
	}
	if note := m.checkReviewFindings(m.shutdownCtx, "rev-1", "- [ERROR] calc.go:4: Add divides by zero"); note != "" {
		t.Errorf("review_checks false still checks findings: %q", note)
	}
	hot := []problemlog.Problem{{Signature: "s", Level: "error", Category: "api", Sample: "429 from provider"}}
	if defects, around := m.triageProblems(m.shutdownCtx, hot); len(defects) != 1 || around != nil {
		t.Errorf("diagnosis_triage false still triages: %v %v", defects, around)
	}
	if n := jev.calls.Load(); n != 0 {
		t.Fatalf("Jev was called %d times with every use off", n)
	}

	// On, each use calls Jev.
	m = jev.attach(t, m, config.TypeSafeConfig{})
	if m.relevanceUse().Judge == nil {
		t.Error("retrieval relevance is off with the section present")
	}
	if note := m.checkReviewFindings(m.shutdownCtx, "rev-2", "- [ERROR] calc.go:4: Add divides by zero"); note == "" {
		t.Error("review checks on, yet the refuted finding is not named")
	}
	if _, around := m.triageProblems(m.shutdownCtx, hot); len(around) != 1 {
		t.Errorf("diagnosis triage on, yet the provider failure is not listed apart: %v", around)
	}
	if jev.calls.Load() < 2 {
		t.Fatalf("Jev calls = %d with the uses on", jev.calls.Load())
	}
}

// Read-only work Jev reads as about codeNERD's own failures is the
// self-diagnosis chain, whatever verb perception chose; without the judgment
// (Jev failed) it is the plain background review perception asked for.
func TestJevJourney_AboutCodeNERDStartsTheSelfDiagnosisChain(t *testing.T) {
	intent := perception.Intent{Category: "/query", Verb: "/review", Target: "the logs", Confidence: 0.9}
	for _, tc := range []struct {
		name string
		jev  *contentJev
		want bool
	}{
		{name: "judged", jev: &contentJev{probs: map[string]float64{"about_codenerd": 0.93}}, want: true},
		{name: "failed", jev: &contentJev{fail: true}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := admissionModel(t, config.WorkModeExplicit, nil)
			m = tc.jev.attach(t, m, config.TypeSafeConfig{})
			assertRouteIntent(t, m, intent)
			m.recordTurnJudgments(m.shutdownCtx, m.startTurnJudgments(m.shutdownCtx, "look through your own logs and tell me why you failed", nil))
			route := m.decideRoute("look through your own logs and tell me why you failed", intent, "reviewer")
			if route.Kind != RouteDelegate || !route.Background {
				t.Fatalf("route = %+v, want a background delegation", route)
			}
			if route.SelfDiagnosis != tc.want {
				t.Fatalf("self-diagnosis = %v, want %v", route.SelfDiagnosis, tc.want)
			}
		})
	}
}

// The self-diagnosis task works codeNERD's own defects first and lists what
// the kernel derives as failures around it apart; a failed triage keeps the
// log's order with nothing listed apart.
func TestJevJourney_TriageListsFailuresAroundCodeNERDApart(t *testing.T) {
	log := "2026/10/28 06:19:54.100 [ERROR] [session] nil map in executor dispatch\n" +
		"2026/10/28 06:20:01.200 [ERROR] [api] provider connection refused\n"
	for _, tc := range []struct {
		name      string
		jev       *contentJev
		wantApart bool
	}{
		{name: "triaged", jev: &contentJev{probs: map[string]float64{"connection refused": 0.04, "nil map": 0.9}}, wantApart: true},
		{name: "failed", jev: &contentJev{fail: true}, wantApart: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			logs := filepath.Join(ws, ".nerd", "logs")
			if err := os.MkdirAll(logs, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(logs, "20261028_061954_problems.log"), []byte(log), 0o644); err != nil {
				t.Fatal(err)
			}
			m := admissionModel(t, config.WorkModePropose, nil)
			m.workspace = ws
			m = tc.jev.attach(t, m, config.TypeSafeConfig{})

			job, ok, reason := m.selfDiagnosisJob(m.shutdownCtx, "reviewer")
			if !ok {
				t.Fatalf("no diagnosis job: %s", reason)
			}
			apart := strings.Index(job.Task, "These look like failures around codeNERD")
			if (apart >= 0) != tc.wantApart {
				t.Fatalf("failures listed apart = %v, want %v:\n%s", apart >= 0, tc.wantApart, job.Task)
			}
			if tc.wantApart && (strings.Index(job.Task, "connection refused") < apart || strings.Index(job.Task, "nil map") > apart) {
				t.Fatalf("the provider failure is not the one listed apart:\n%s", job.Task)
			}
			if !strings.Contains(job.Task, "nil map") || !strings.Contains(job.Task, "connection refused") {
				t.Fatalf("a problem went missing:\n%s", job.Task)
			}
		})
	}
}
