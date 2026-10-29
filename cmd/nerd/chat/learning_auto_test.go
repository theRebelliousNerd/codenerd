package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/learnjudge"
	"codenerd/internal/typesafe"
)

// verbJev answers the learning loop's verb choice with pick at confidence and
// its fit question with fits; every other noul gets 0.5, and the correction
// question correction.
func verbJev(t *testing.T, m Model, pick string, confidence, fits, correction float64) Model {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		answers := map[string]any{}
		for name := range req.Questions {
			switch name {
			case learnjudge.ChosenVerb:
				answers[name] = map[string]any{"type": "choice", "choice": pick, "confidence": confidence, "probabilities": map[string]float64{pick: confidence}}
			case learnjudge.VerbFits:
				answers[name] = map[string]any{"type": "noul", "noul": fits}
			case learnjudge.CorrectsLastTurn:
				answers[name] = map[string]any{"type": "noul", "noul": correction}
			default:
				answers[name] = map[string]any{"type": "noul", "noul": 0.5}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-fake", "answers": answers, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k", Retry: &typesafe.RetryPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	m.jev = c
	m.Config.TypeSafe = &config.TypeSafeConfig{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k"}
	return m
}

// The truth table: a mapping is learned only when Jev is confident both that
// a verb fits the request and which verb it is.
func TestChooseLearnedMapping_TruthTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		confidence, fits float64
		request          string
		wantFact, reason string
	}{
		{name: "confident on both learns", confidence: 0.99, fits: 0.93, request: "fix oversized prompt atoms",
			wantFact: `learned_exemplar("fix oversized prompt atoms", /fix, "", "", 0.99).`},
		{name: "no verb fits learns nothing", confidence: 0.99, fits: 0.25, request: "yes", reason: "no verb fits"},
		{name: "an unsure pick learns nothing", confidence: 0.52, fits: 0.95, request: "add a test for Max", reason: "unsure"},
		{name: "no earlier request learns nothing", confidence: 0.99, fits: 0.99, request: "  ", reason: "no earlier request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := verbJev(t, correctionModel(t, "no, I said Max"), "fix", tc.confidence, tc.fits, 0.9)
			got := m.chooseLearnedMapping(m.shutdownCtx, tc.request)
			if got.fact != tc.wantFact {
				t.Fatalf("fact = %q, want %q (reason %q)", got.fact, tc.wantFact, got.reason)
			}
			if tc.reason != "" && !strings.Contains(got.reason, tc.reason) {
				t.Fatalf("reason = %q, want it to say %q", got.reason, tc.reason)
			}
		})
	}
	m := correctionModel(t, "no, I said Max")
	m.jev = nil
	if got := m.chooseLearnedMapping(m.shutdownCtx, "fix it"); got.fact != "" || !strings.Contains(got.reason, "Jev is not configured") {
		t.Fatalf("no Jev: %+v", got)
	}
}

// A correction no longer replaces the turn: the corrected request runs and
// answers as usual, with no "Should I learn it?" and no "retry the request".
func TestCorrection_TheTurnStillRunsAndNothingAsksTheUser(t *testing.T) {
	t.Parallel()
	// The pick fits nothing, so the background learning stores nothing: the
	// shared taxonomy has no workspace in tests and would write .nerd/ here.
	m := verbJev(t, correctionModel(t, "no, I said Max, not Min"), "fix", 0.99, 0.10, 0.95)
	msg := awaitProcessStream(t, m.processInput("no, I said Max, not Min")())
	if reply, ok := msg.(responseMsg); !ok || string(reply) != "Understood." {
		t.Fatalf("the corrected turn became %T %+v, want its own reply", msg, msg)
	}
	if source, ok := m.learningCorrection(); !ok || source != "jev" {
		t.Fatalf("learning_correction = %q (%v), want jev", source, ok)
	}
	// The package directory holds tracked .nerd fixtures; the file learning
	// writes is the taxonomy it persists to.
	if _, err := os.Stat(".nerd/mangle/learned_taxonomy.mg"); err == nil {
		t.Fatal("learning wrote .nerd/mangle/learned_taxonomy.mg into the package directory")
	}
}
