package deliberation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/sessionlog"
	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	toolscore "codenerd/internal/tools/core"
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

func TestPersistence_MissionValuesThreadsDoNotCollideWithSessionPending(t *testing.T) {
	ws := t.TempDir()
	s := &Service{Workspace: ws, Session: "sess-1"}
	m1 := &Thread{
		ID: "t-mission-1", Session: "sess-1", OwnerKind: "mission", OwnerID: "mission-1",
		OwnerDisposition: "retained_for_owner", DesignOnly: true, Status: "awaiting_values", Asked: true,
		Question: "First mission question?", Options: []string{"a: first (Recommended)", "b: second"},
	}
	m2 := &Thread{
		ID: "t-mission-2", Session: "sess-1", OwnerKind: "mission", OwnerID: "mission-2",
		OwnerDisposition: "retained_for_owner", DesignOnly: true, Status: "awaiting_values", Asked: true,
		Question: "Second mission question?", Options: []string{"a: first (Recommended)", "b: second"},
	}
	for _, th := range []*Thread{m1, m2} {
		if err := s.Save(th); err != nil {
			t.Fatal(err)
		}
	}
	// Two stopped mission threads in the same session must not cause Pending()
	// to fail with "multiple pending deliberations in session" or hijack chat.
	if pending, err := s.Pending(); err != nil || pending != nil {
		t.Fatalf("session Pending() with two mission threads = %+v, %v; want nil, nil", pending, err)
	}
	if got1, err := s.PendingForOwner("mission", "mission-1"); err != nil || got1 == nil || got1.ID != "t-mission-1" {
		t.Fatalf("PendingForOwner mission-1 = %+v, %v", got1, err)
	}
	if got2, err := s.PendingForOwner("mission", "mission-2"); err != nil || got2 == nil || got2.ID != "t-mission-2" {
		t.Fatalf("PendingForOwner mission-2 = %+v, %v", got2, err)
	}
	if all, err := s.PendingMissionThreads(); err != nil || len(all) != 2 {
		t.Fatalf("PendingMissionThreads = %d, %v; want 2", len(all), err)
	}
	// Adding one conversational chat question alongside the two mission threads
	// restores that chat question cleanly.
	chatWait := &Thread{
		ID: "t-chat-wait", Session: "sess-1", Request: "q", Status: "awaiting_values", Asked: true,
	}
	if err := s.Save(chatWait); err != nil {
		t.Fatal(err)
	}
	if pending, err := s.Pending(); err != nil || pending == nil || pending.ID != "t-chat-wait" {
		t.Fatalf("session Pending() with chat + mission threads = %+v, %v; want t-chat-wait", pending, err)
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

// The roles' tool offer is sent to a provider that rejects a null schema
// field (a live 400: "null is not of type array"). Every offered definition,
// rendered from the kernel's deliberation_read_tool envelope, must be free of
// nulls.
func TestReadToolDefinitions_CarryNoNullSchemaField(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	// The built-in registries production hydrates the modular tools from.
	reg := tools.NewRegistry()
	if err := toolscore.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	defs, err := ReadTools{Kernel: k, Registry: reg, ContextBytes: 1 << 16}.Definitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(defs) < 10 {
		t.Fatalf("offered %d definitions, want the core and CodeDOM reads", len(defs))
	}
	for _, d := range defs {
		b, err := json.Marshal(d.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "null") {
			t.Errorf("%s offers a null schema field: %s", d.Name, b)
		}
	}
}

func TestStabilize_IsMissionOwnedDistinguishesOwners(t *testing.T) {
	var nilThread *Thread
	if nilThread.IsMissionOwned() {
		t.Error("nil thread is not mission owned")
	}
	if (&Thread{}).IsMissionOwned() {
		t.Error("empty thread is not mission owned")
	}
	if !(&Thread{OwnerKind: "mission"}).IsMissionOwned() {
		t.Error("OwnerKind mission must be mission owned")
	}
	if !(&Thread{OwnerID: "mission-1"}).IsMissionOwned() {
		t.Error("non-empty OwnerID must be mission owned")
	}
	if !(&Thread{OwnerDisposition: "retained_for_owner"}).IsMissionOwned() {
		t.Error("retained_for_owner disposition must be mission owned")
	}
	if (&Thread{OwnerKind: "chat"}).IsMissionOwned() {
		t.Error("chat owner without ID is not mission owned")
	}
}

func TestStabilize_NumberConvertsKernelValues(t *testing.T) {
	if number(int64(3)) != 3 {
		t.Error("int64 not converted")
	}
	if number(int(4)) != 4 {
		t.Error("int not converted")
	}
	if number(float64(2.9)) != 2 {
		t.Error("float64 not truncated")
	}
	if number("x") != -1 {
		t.Error("string must be -1")
	}
	if number(nil) != -1 {
		t.Error("nil must be -1")
	}
}

func TestStabilize_CandidateFindsByID(t *testing.T) {
	r := validRound()
	c := candidate(&r, "a")
	if c == nil || c.ID != "a" || c.Brief != "read it one way" {
		t.Fatalf("candidate a not found: %+v", c)
	}
	if candidate(&r, "missing") != nil {
		t.Error("unknown id must return nil")
	}
}

func TestStabilize_OptionFormatsAndChoiceRejectsBlank(t *testing.T) {
	if got := option(Candidate{ID: "x", Brief: "do y"}); got != "x: do y" {
		t.Errorf("option = %q, want %q", got, "x: do y")
	}
	r := validRound()
	if choice(r, "") != nil {
		t.Error("blank answer must choose nothing")
	}
	if choice(r, "   ") != nil {
		t.Error("whitespace answer must choose nothing")
	}
}

func TestStabilize_DecodeTrimsWhitespaceAndEmptyEnvelope(t *testing.T) {
	var c Challenge
	if err := Decode("  {\"recommendation\":\"a\"}  ", &c); err != nil || c.Recommendation != "a" {
		t.Errorf("whitespace-trimmed decode failed: %+v, %v", c, err)
	}
	var d Challenge
	if err := Decode("{\"surface_response\":\"\", \"recommendation\":\"a\"}", &d); err != nil || d.Recommendation != "a" {
		t.Errorf("empty-surface envelope must decode outer object: %+v, %v", d, err)
	}
	var e Challenge
	if err := Decode("```\n{\"recommendation\":\"b\"}\n```", &e); err != nil || e.Recommendation != "b" {
		t.Errorf("fence without language tag must decode: %+v, %v", e, err)
	}
}

func TestStabilize_BoundObservationKeepsFitsAndCutsVisibly(t *testing.T) {
	if got := boundObservation("abc", 10); got != "abc" {
		t.Errorf("fitting text must be unchanged, got %q", got)
	}
	room := len(cutNote) + 32
	got := boundObservation(strings.Repeat("x", 300), room)
	if len(got) > room {
		t.Errorf("cut output %d bytes exceeds room %d", len(got), room)
	}
	if !strings.Contains(got, "cut at the deliberation context bound") {
		t.Errorf("cut must be visible: %q", got)
	}
	if got := boundObservation(strings.Repeat("x", 100), 5); got != cutNote[1:] {
		t.Errorf("tiny room must return the bare marker, got %q", got)
	}
}

func TestStabilize_AddNoteTrimsLargestToFitAndErrorsWithoutRoom(t *testing.T) {
	obs := []ProbeResult{{Key: "a", Tool: "read_file", Output: strings.Repeat("x", 1000)}}
	note := ProbeResult{Key: "n", Error: "e"}
	if err := addNote(&obs, note, &obs, 200); err != nil {
		t.Fatalf("addNote should trim to fit: %v", err)
	}
	if len(obs) != 2 {
		t.Fatalf("addNote must append the note, got %d observations", len(obs))
	}
	if !strings.Contains(obs[0].Output, "cut at the deliberation context bound") {
		t.Errorf("largest output should have been cut visibly: %.100s", obs[0].Output)
	}
	b, err := json.Marshal(&obs)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 200 {
		t.Errorf("trimmed observations still exceed bound: %d > 200", len(b))
	}

	empty := []ProbeResult{}
	if err := addNote(&empty, ProbeResult{Key: "n", Error: "e"}, &empty, 5); err == nil {
		t.Error("a bound with no room for a note must fail")
	}
}

func TestStabilize_OutcomeNoteCoversBranches(t *testing.T) {
	if got := (Outcome{}).Note(); got != "" {
		t.Errorf("nil thread note = %q, want empty", got)
	}
	if got := (Outcome{Thread: &Thread{Status: "running"}}).Note(); got != "Deliberation: running." {
		t.Errorf("non-ready note = %q", got)
	}
	ready := Outcome{Thread: &Thread{Status: "ready"}, Action: "proceed", Candidate: &Candidate{ID: "a", Brief: "keep it", SuccessTest: "tests pass"}}
	note := ready.Note()
	for _, want := range []string{"Decided: keep it", "Success test: tests pass"} {
		if !strings.Contains(note, want) {
			t.Errorf("ready note lacks %q: %s", want, note)
		}
	}
	if strings.Contains(note, "Assumed") {
		t.Errorf("proceed without assumptions must not mention them: %s", note)
	}
	withProof := Outcome{
		Thread:    &Thread{Status: "ready", Rounds: []Round{{Number: 1, Proof: json.RawMessage(`{"Goal":"g","Proofs":[{"ID":"p1"}]}`)}}},
		Action:    "proceed",
		Candidate: &Candidate{ID: "a", Brief: "keep it", SuccessTest: "tests pass"},
	}
	if got := withProof.Note(); !strings.Contains(got, "Policy proof: p1") {
		t.Errorf("ready note with proof must name it: %s", got)
	}
}

func TestStabilize_ProgressFingerprintIgnoresImportantAndOrdersProbes(t *testing.T) {
	base := validRound()
	onlyImportantA := validRound()
	onlyImportantA.Challenge.Assessments[1].Items[0].Severity = "important"
	onlyImportantA.Challenge.Assessments[1].Items[0].Detail = "first wording"
	onlyImportantB := validRound()
	onlyImportantB.Challenge.Assessments[1].Items[0].Severity = "important"
	onlyImportantB.Challenge.Assessments[1].Items[0].Detail = "second wording"
	if progressFingerprint(onlyImportantA) != progressFingerprint(onlyImportantB) {
		t.Error("important-only items must not affect progress")
	}
	roundA := validRound()
	roundA.Probes = []ProbeResult{{Key: "k1", Tool: "read_file", Output: "one"}, {Key: "k2", Tool: "read_file", Output: "two"}}
	roundB := validRound()
	roundB.Probes = []ProbeResult{{Key: "k2", Tool: "read_file", Output: "two"}, {Key: "k1", Tool: "read_file", Output: "one"}}
	if progressFingerprint(roundA) != progressFingerprint(roundB) {
		t.Error("probe order must not affect progress")
	}
	changed := validRound()
	changed.Challenge.Assessments[1].Items[0].Key = "other-key"
	if progressFingerprint(base) == progressFingerprint(changed) {
		t.Error("a new critical item key must count as progress")
	}
}

func TestStabilize_ValidateRoundRejectsEmptyAndBlank(t *testing.T) {
	if err := validateRound(Round{}, 3); err == nil {
		t.Error("empty round accepted")
	}
	r := validRound()
	r.Proposal.Candidates[0].ID = ""
	if err := validateRound(r, 3); err == nil {
		t.Error("blank candidate id accepted")
	}
	r = validRound()
	r.Proposal.Candidates[0].Brief = "   "
	if err := validateRound(r, 3); err == nil {
		t.Error("blank brief accepted")
	}
	r = validRound()
	r.Challenge.Assessments[1].Items[0].Key = ""
	if err := validateRound(r, 3); err == nil {
		t.Error("empty item key accepted")
	}
}

func TestStabilize_PendingWithNoDirectoryReturnsNil(t *testing.T) {
	s := &Service{Workspace: t.TempDir(), Session: "sess-absent"}
	if got, err := s.Pending(); err != nil || got != nil {
		t.Errorf("Pending with no directory = %+v, %v; want nil, nil", got, err)
	}
	if got, err := s.PendingForOwner("mission", "m1"); err != nil || got != nil {
		t.Errorf("PendingForOwner with no directory = %+v, %v; want nil, nil", got, err)
	}
	if all, err := s.PendingMissionThreads(); err != nil || len(all) != 0 {
		t.Errorf("PendingMissionThreads with no directory = %d, %v; want 0, nil", len(all), err)
	}
}
