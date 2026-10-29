package turnjudge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/problemlog"
	"codenerd/internal/retrieval"
	"codenerd/internal/typesafe"
)

// recordingAsker answers every noul with p and keeps each request it saw.
type recordingAsker struct {
	mu   sync.Mutex
	p    float64
	fail bool
	seen []recordedAsk
}

type recordedAsk struct {
	state     any
	questions map[string]typesafe.Question
}

func (r *recordingAsker) Ask(_ context.Context, state any, qs map[string]typesafe.Question) (*typesafe.Response, error) {
	r.mu.Lock()
	r.seen = append(r.seen, recordedAsk{state, qs})
	r.mu.Unlock()
	if r.fail {
		return nil, errors.New("down")
	}
	answers := map[string]any{}
	for name := range qs {
		answers[name] = map[string]any{"type": "noul", "noul": r.p}
	}
	raw, _ := json.Marshal(map[string]any{"model": "m", "answers": answers, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	var resp typesafe.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func payload(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInBatches_KeepsItemOrderAndFailsWhole(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}
	out, err := inBatches(items, 2, func(b []int) ([]int, error) { return b, nil })
	if err != nil || len(out) != 3 || out[2][0] != 5 || out[0][1] != 2 {
		t.Fatalf("batches = %v %v, want [1 2] [3 4] [5] in order", out, err)
	}
	out, err = inBatches(items, 2, func(b []int) ([]int, error) {
		if b[0] == 3 {
			return nil, errors.New("one batch failed")
		}
		return b, nil
	})
	if err == nil || out != nil {
		t.Fatalf("a failed batch must fail the whole, got %v %v", out, err)
	}
}

// Triage goes in requests of at most the batch size, each problem with its
// whole message and without its repeat count, and the probabilities come back
// in the log's order.
func TestTriageProblems_BatchesAndLeavesOutTheCount(t *testing.T) {
	a := &recordingAsker{p: 0.2}
	var problems []problemlog.Problem
	for i := 0; i < 5; i++ {
		problems = append(problems, problemlog.Problem{Signature: fmt.Sprint(i), Level: "error", Category: "api", Count: 977 + i, Sample: fmt.Sprintf("line one\nline two %d", i)})
	}
	probs, err := TriageProblems(context.Background(), a, problems, 2)
	if err != nil || len(probs) != 5 {
		t.Fatalf("probs = %v %v", probs, err)
	}
	if len(a.seen) != 3 {
		t.Fatalf("%d requests, want 3 for 5 problems at 2 each", len(a.seen))
	}
	for _, s := range a.seen {
		if len(s.questions) > 2 {
			t.Errorf("a request carries %d problems, more than the batch size", len(s.questions))
		}
		p := payload(t, s.questions)
		if strings.Contains(p, "977") || strings.Contains(p, `"count"`) {
			t.Errorf("the repeat count reached the model: %s", p)
		}
		if !strings.Contains(p, "line two") {
			t.Errorf("a message was cut: %s", p)
		}
	}
	a.fail = true
	if probs, err := TriageProblems(context.Background(), a, problems, 2); err == nil || probs != nil {
		t.Fatalf("a failed triage is an error and no probabilities, got %v %v", probs, err)
	}
}

func TestRelevance_FilesCarryOnlyWhatAPersonWouldRead(t *testing.T) {
	a := &recordingAsker{p: 0.9}
	j := RelevanceJudge{Asker: a, BatchSize: 2}
	files := []retrieval.RelevanceCandidate{
		{Path: "a.go", Keywords: []string{"retry"}, Excerpt: "package a", ExcerptOf: 9},
		{Path: "b.go", Keywords: []string{"retry"}, Excerpt: "package", ExcerptOf: 4000},
		{Path: "c.go"},
	}
	got, err := j.Relevance(context.Background(), "why does the retry loop spin", files)
	if err != nil || len(got) != 3 || got["c.go"] != 0.9 {
		t.Fatalf("relevance = %v %v", got, err)
	}
	if len(a.seen) != 2 {
		t.Fatalf("%d requests, want 2 for 3 files at 2 each", len(a.seen))
	}
	all := payload(t, a.seen[0].questions) + payload(t, a.seen[1].questions)
	if strings.Contains(all, "retry\"]") || strings.Contains(all, "matched_keywords") {
		t.Errorf("the keyword that retrieval matched reached the model: it pulls toward yes: %s", all)
	}
	if !strings.Contains(all, "the first 7 of 4000 bytes") {
		t.Errorf("a cut head does not say it is cut: %s", all)
	}
	if !strings.Contains(payload(t, a.seen[0].state), "why does the retry loop spin") {
		t.Error("the request is not in the shared state")
	}
}

func TestCheckFindings_StatesTheCitedLineAndBatches(t *testing.T) {
	dir := t.TempDir()
	writeLines(t, dir, "x.go", 30)
	a := &recordingAsker{p: 0.8}
	var fs []Finding
	for i := 1; i <= 5; i++ {
		fs = append(fs, Finding{File: "x.go", Line: 10 + i, Message: fmt.Sprintf("problem %d", i)})
	}
	got, err := CheckFindings(context.Background(), a, dir, fs, FindingOptions{BatchSize: 2, ContextLines: 3})
	if err != nil || len(got) != 5 || got[4].Message != "problem 5" {
		t.Fatalf("verdicts = %+v %v, want five in the findings' order", got, err)
	}
	if len(a.seen) != 3 {
		t.Fatalf("%d requests, want 3", len(a.seen))
	}
	if p := payload(t, a.seen[0].questions); !strings.Contains(p, `"cited_line":1`) {
		t.Errorf("the cited line is not stated outright: %s", p)
	}
}

func writeLines(t *testing.T, dir, name string, n int) {
	t.Helper()
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
