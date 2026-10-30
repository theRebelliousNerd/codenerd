package deliberation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/sessionlog"
)

func TestDecode_AcceptsFencedAndEnveloped(t *testing.T) {
	for name, raw := range map[string]string{
		"plain":    `{"recommendation":"a"}`,
		"fenced":   "```json\n{\"recommendation\":\"a\"}\n```",
		"envelope": `{"surface_response":"{\"recommendation\":\"a\"}"}`,
	} {
		var c Challenge
		if err := Decode(raw, &c); err != nil || c.Recommendation != "a" {
			t.Errorf("%s: got %+v, %v", name, c, err)
		}
	}
	var c Challenge
	if err := Decode("not json", &c); err == nil {
		t.Error("malformed output decoded")
	}
}

func validRound() Round {
	return Round{
		Proposal: Proposal{Candidates: []Candidate{
			{ID: "a", Brief: "read it one way", SuccessTest: "test a"},
			{ID: "b", Brief: "read it another way", SuccessTest: "test b"},
		}},
		Challenge: Challenge{Recommendation: "a", Assessments: []Assessment{
			{CandidateID: "a", Success: true},
			{CandidateID: "b", Items: []Item{{Key: "k", Severity: "critical", Kind: "values"}}},
		}},
	}
}

func TestValidateRound_RejectsMalformedRounds(t *testing.T) {
	if err := validateRound(validRound(), 3); err != nil {
		t.Fatalf("valid round rejected: %v", err)
	}
	cases := map[string]func(*Round){
		"over the candidate cap": func(r *Round) {
			r.Proposal.Candidates = append(r.Proposal.Candidates, Candidate{ID: "c", Brief: "x", SuccessTest: "y"}, Candidate{ID: "d", Brief: "x", SuccessTest: "y"})
		},
		"duplicate candidate id":   func(r *Round) { r.Proposal.Candidates[1].ID = "a" },
		"missing success test":     func(r *Round) { r.Proposal.Candidates[0].SuccessTest = " " },
		"unassessed candidate":     func(r *Round) { r.Challenge.Assessments = r.Challenge.Assessments[:1] },
		"unknown recommendation":   func(r *Round) { r.Challenge.Recommendation = "z" },
		"invalid severity":         func(r *Round) { r.Challenge.Assessments[1].Items[0].Severity = "blocker" },
		"invalid kind":             func(r *Round) { r.Challenge.Assessments[1].Items[0].Kind = "vibes" },
		"critical data, no probe":  func(r *Round) { r.Challenge.Assessments[1].Items[0].Kind = "data" },
		"duplicate open-item key":  func(r *Round) { a := &r.Challenge.Assessments[1]; a.Items = append(a.Items, a.Items[0]) },
		"assessment of a stranger": func(r *Round) { r.Challenge.Assessments[1].CandidateID = "z" },
	}
	for name, mutate := range cases {
		r := validRound()
		mutate(&r)
		if err := validateRound(r, 3); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestChoice_MatchesIDOrOfferedOption(t *testing.T) {
	r := validRound()
	for _, answer := range []string{"b", " b ", option(r.Proposal.Candidates[1]), option(r.Proposal.Candidates[1]) + " (Recommended)"} {
		if c := choice(r, answer); c == nil || c.ID != "b" {
			t.Errorf("answer %q chose %v", answer, c)
		}
	}
	if choice(r, "something else") != nil {
		t.Error("an unrelated answer chose a candidate")
	}
}

func TestProgressFingerprint_IgnoresRewording(t *testing.T) {
	a, b := validRound(), validRound()
	b.Proposal.Candidates[0].Brief = "the same reading, reworded"
	b.Challenge.Assessments[1].Items[0].Detail = "new rationale"
	if progressFingerprint(a) != progressFingerprint(b) {
		t.Error("rewording counted as progress")
	}
	b.Probes = []ProbeResult{{Key: "k", Tool: "read_file", Output: "observed"}}
	if progressFingerprint(a) == progressFingerprint(b) {
		t.Error("a new probe observation did not count as progress")
	}
}

func TestPersistence_RoundTripAndPending(t *testing.T) {
	ws := t.TempDir()
	s := &Service{Workspace: ws, Session: "sess-1"}
	ready := &Thread{ID: "t-ready", Session: "sess-1", Request: "r", Status: "ready", Rounds: []Round{validRound()}}
	waiting := &Thread{ID: "t-wait", Session: "sess-1", Request: "q", Status: "awaiting_values", Asked: true}
	for _, th := range []*Thread{ready, waiting} {
		if err := s.Save(th); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load("t-ready")
	if err != nil || got.Request != "r" || len(got.Rounds) != 1 {
		t.Fatalf("round trip: %+v, %v", got, err)
	}
	pending, err := s.Pending()
	if err != nil || pending == nil || pending.ID != "t-wait" {
		t.Fatalf("pending: %+v, %v", pending, err)
	}
	if err := s.Save(&Thread{ID: "t-other", Session: "sess-2"}); err == nil {
		t.Error("saved a thread from another session")
	}
	if err := s.Save(&Thread{ID: "../escape", Session: "sess-1"}); err == nil {
		t.Error("saved a thread with a path-traversal id")
	}
	dir, err := sessionlog.SessionDir(ws, "sess-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "deliberation", "t-forged.json"), []byte(`{"id":"t-ready","session":"sess-1"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("t-forged"); err == nil || !strings.Contains(err.Error(), "identity") {
		t.Errorf("a file whose id disagrees with its name loaded: %v", err)
	}
}

func TestOutcomeNote_StatesAssumptions(t *testing.T) {
	c := Candidate{ID: "a", Brief: "keep the API", SuccessTest: "tests pass", Assumptions: []string{"callers unchanged"}}
	out := Outcome{Thread: &Thread{Status: "ready"}, Action: "proceed_assuming", Candidate: &c}
	note := out.Note()
	for _, want := range []string{"Decided: keep the API", "Assumed: callers unchanged", "yolo", "Success test: tests pass"} {
		if !strings.Contains(note, want) {
			t.Errorf("note lacks %q: %s", want, note)
		}
	}
	if (Outcome{Thread: &Thread{Status: "stopped"}}).Note() != "Deliberation: stopped." {
		t.Error("a stopped thread's note is wrong")
	}
}
