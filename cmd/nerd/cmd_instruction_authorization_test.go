package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/turnjudge"
	"codenerd/internal/typesafe"

	"go.uber.org/zap"
)

// `nerd run "fix X, then commit it"`: the one-shot command asks Jev what the
// instruction allowed and hands the kernel's conclusion to its delegation.
// The judgments it asserted must be gone before routing reads the kernel.

func fakeJevFor(t *testing.T, probs map[string]float64) (*typesafe.Client, *config.TypeSafeConfig) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Questions map[string]json.RawMessage `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		answers := map[string]any{}
		for name, q := range req.Questions {
			var wire struct {
				Type     string         `json:"type"`
				Criteria map[string]any `json:"criteria"`
			}
			_ = json.Unmarshal(q, &wire)
			if wire.Type == "choice" {
				dist := map[string]float64{}
				best := ""
				for o := range wire.Criteria {
					dist[o] = 1 / float64(len(wire.Criteria))
					if best == "" || o < best {
						best = o
					}
				}
				answers[name] = map[string]any{"type": "choice", "choice": best, "confidence": dist[best], "probabilities": dist}
				continue
			}
			p, ok := probs[name]
			if !ok {
				p = 0.5
			}
			answers[name] = map[string]any{"type": "noul", "noul": p}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-fake", "answers": answers, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k", Retry: &typesafe.RetryPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	return c, &config.TypeSafeConfig{Model: "jev-fake", BaseURL: srv.URL, APIKey: "k"}
}

func TestInstructionAuthorization_ThenCommitItIsCarriedAndTheJudgmentsAreTakenBack(t *testing.T) {
	logger = zap.NewNop()
	client, ts := fakeJevFor(t, map[string]float64{"asks_to_commit": 0.98, "rules_out_changes": 0.02, "hypothetical": 0.02})
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	got := authorizationFromJev(t.Context(), client, ts, k, "fix the typo in README.md, then commit it")
	if !got.Has(perception.AuthorizeCommit) {
		t.Fatalf("authorization = %v, want commit", got.Kinds())
	}
	for _, pred := range turnjudge.Predicates {
		if rows, _ := k.Query(pred); len(rows) != 0 {
			t.Fatalf("%s outlived the call and would steer routing: %v", pred, rows)
		}
	}
	if rows, _ := k.Query("needs_clarification"); len(rows) != 0 {
		t.Fatalf("the instruction's judgments left a routing conclusion behind: %v", rows)
	}
}

func TestInstructionAuthorization_NothingAuthorizedWithoutAYes(t *testing.T) {
	logger = zap.NewNop()
	for name, probs := range map[string]map[string]float64{
		"no commit word":       {"asks_to_commit": 0.02},
		"unsettled":            {"asks_to_commit": 0.5},
		"commit but readonly":  {"asks_to_commit": 0.98, "rules_out_changes": 0.95},
		"commit but a what-if": {"asks_to_commit": 0.98, "hypothetical": 0.95},
	} {
		t.Run(name, func(t *testing.T) {
			client, ts := fakeJevFor(t, probs)
			k, err := core.NewRealKernel()
			if err != nil {
				t.Fatal(err)
			}
			if got := authorizationFromJev(t.Context(), client, ts, k, "fix the typo in README.md"); !got.IsZero() {
				t.Fatalf("authorization = %v, want none", got.Kinds())
			}
		})
	}
	// A failing Jev is no evidence.
	client, ts := fakeJevFor(t, nil)
	ts.BaseURL = "http://127.0.0.1:1"
	failing, err := typesafe.NewClient(typesafe.Options{Model: "jev-fake", BaseURL: ts.BaseURL, APIKey: "k", Retry: &typesafe.RetryPolicy{}})
	if err != nil {
		t.Fatal(err)
	}
	_ = client
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if got := authorizationFromJev(t.Context(), failing, ts, k, "fix it, then commit it"); !got.IsZero() {
		t.Fatalf("a failed Jev call produced %v", got.Kinds())
	}
}
