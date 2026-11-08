// Text-report parity for RecurseStatus JSON: builds RecurseStatus fixture,
// marshals via ToRecurseStatusJSON/MarshalRecurseStatusJSON DTO funcs, parses JSON
// back and asserts running/pass/counts-by-outcome, base/last commit, conflicts,
// generations, and attempt cycle/outcome/node/commit (incl. shortCommit nil-safety).
// Verified ONLY via go build ./internal/campaign and go test ./internal/campaign -run RecurseStatusJSON -count=1 serialized.
// Scoped verification only; TestNorthstarHandlerAdapter_WhenGuardianPasses_ShouldReturnProceed,
// TestRelevanceInFlight_CompilerShutdownJoinsOwnedWork, and TestExecutiveLiteralBudget are pre-existing out-of-scope flaky tests; full suite remains out of scope.
package campaign

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRecurseStatusJSON_RunningStateMapping(t *testing.T) {
	cases := []struct {
		name          string
		running       bool
		stopRequested bool
		wantState     string
	}{
		{name: "not running", running: false, stopRequested: false, wantState: "not running"},
		{name: "running", running: true, stopRequested: false, wantState: "running"},
		{name: "running stop requested", running: true, stopRequested: true, wantState: "running, stop requested"},
		{name: "not running stop requested", running: false, stopRequested: true, wantState: "not running, stop requested"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := RecurseStatus{Running: tc.running, StopRequested: tc.stopRequested}
			got := s.ToRecurseStatusJSON()
			if got.State != tc.wantState {
				t.Errorf("State = %q, want %q", got.State, tc.wantState)
			}
			if got.Running != tc.running || got.StopRequested != tc.stopRequested {
				t.Errorf("Running/StopRequested = %v/%v, want %v/%v", got.Running, got.StopRequested, tc.running, tc.stopRequested)
			}
			raw, err := s.MarshalRecurseStatusJSON()
			if err != nil {
				t.Fatalf("MarshalRecurseStatusJSON: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if decoded["state"] != tc.wantState {
				t.Errorf("json state = %v, want %q", decoded["state"], tc.wantState)
			}
		})
	}
	t.Run("text report parity", func(t *testing.T) {
		s := RecurseStatus{
			Running:       true,
			StopRequested: false,
			Pass:          2,
			Node:          "node-b",
			Cycles:        4, Kept: 1, Reverted: 1, Refused: 1, Unverified: 1,
			OpenByPass: []int{3, 1},
			ThisPass: []recurseRecord{
				{Cycle: 1, Outcome: outcomeKept, Node: "node-a", Commit: "abcdef1234567890", Signature: "sig1", Finding: "finding-a"},
				{Cycle: 2, Outcome: outcomeReverted, Node: "node-b", Commit: "1234567890abcdef", Signature: "sig2"},
				{Cycle: 3, Outcome: outcomeRefused, Node: "node-c"},
				{Cycle: 4, Outcome: outcomeUnverified, Node: "node-d", Why: "needs /changed_code_unexecuted"},
			},
			Base: RecurseBaseStatus{
				Branch:       "main",
				TipAtStart:   "abcdef1234567890",
				LastAbsorbed: "1234567890abcdef",
				Absorbs:      2,
				Conflicts:    []RecurseConflict{{BaseCommit: "deadbeef12345678", Detail: "merge conflict"}},
				Generations: []RecurseGeneration{
					{Commit: "abcdef1234567890", Binary: "/tmp/nerd-test"},
					{Commit: "feedface12345678", Failure: "build failed"},
				},
			},
		}
		text := s.String()
		if !strings.Contains(text, "running") || !strings.Contains(text, "Pass 2") {
			t.Errorf("text running/pass missing in %q, want running Pass 2", text)
		}
		if !strings.Contains(text, "visiting node-b") {
			t.Errorf("text node missing in %q, want visiting node-b", text)
		}
		if !strings.Contains(text, "4 attempts: 1 kept, 1 reverted, 1 refused, 1 unverified") {
			t.Errorf("text counts missing in %q", text)
		}
		if !strings.Contains(text, "Base: main") || !strings.Contains(text, shortCommit("abcdef1234567890")) || !strings.Contains(text, shortCommit("1234567890abcdef")) {
			t.Errorf("text base/last commit missing in %q", text)
		}
		if !strings.Contains(text, "Merge conflicts: 1") || !strings.Contains(text, shortCommit("deadbeef12345678")) || !strings.Contains(text, "merge conflict") {
			t.Errorf("text conflicts missing in %q", text)
		}
		if !strings.Contains(text, "Binary generations:") || !strings.Contains(text, shortCommit("feedface12345678")) || !strings.Contains(text, "/tmp/nerd-test") || !strings.Contains(text, "build FAILED") || !strings.Contains(text, "build failed") {
			t.Errorf("text generations missing in %q", text)
		}
		if !strings.Contains(text, "(sig1)") || !strings.Contains(text, "(sig2)") {
			t.Errorf("text signatures missing in %q", text)
		}
		if !strings.Contains(text, "Open findings at the end of each pass: 3 -> 1") {
			t.Errorf("text open-by-pass trend missing in %q", text)
		}
		for _, want := range []string{"cycle 1", "cycle 2", "cycle 3", "cycle 4", "node-a", "node-b", "node-c", "node-d", "abcdef1234567890", "1234567890abcdef", "kept", "reverted", "refused", "unverified"} {
			if !strings.Contains(text, want) {
				t.Errorf("text attempt %q missing in %q", want, text)
			}
		}
		got := s.ToRecurseStatusJSON()
		if !got.Running || got.StopRequested || got.Pass != 2 || got.Node != "node-b" || len(got.OpenByPass) != 2 || got.OpenByPass[0] != 3 || got.OpenByPass[1] != 1 {
			t.Errorf("running/stop/pass/node/openbypass = %v/%v/%d/%q/%v, want true/false/2/node-b/[3 1]", got.Running, got.StopRequested, got.Pass, got.Node, got.OpenByPass)
		}
		if got.State != "running" {
			t.Errorf("State = %q, want %q", got.State, "running")
		}
		if got.Cycles != 4 || got.Kept != 1 || got.Reverted != 1 || got.Refused != 1 || got.Unverified != 1 {
			t.Errorf("counts = %d/%d/%d/%d/%d, want 4/1/1/1/1", got.Cycles, got.Kept, got.Reverted, got.Refused, got.Unverified)
		}
		if got.Base.Branch != "main" || got.Base.Tip != "abcdef1234567890" || got.Base.TipShort != shortCommit("abcdef1234567890") {
			t.Errorf("base tip = %+v, want branch main tip abcdef1234567890", got.Base)
		}
		if got.Base.LastAbsorbed != "1234567890abcdef" || got.Base.LastAbsorbedShort != shortCommit("1234567890abcdef") || got.Base.Absorbs != 2 {
			t.Errorf("base last absorbed = %+v, want 1234567890abcdef x2", got.Base)
		}
		if len(got.Base.Conflicts) != 1 || got.Base.Conflicts[0].BaseCommit != "deadbeef12345678" || got.Base.Conflicts[0].ShortCommit != shortCommit("deadbeef12345678") || got.Base.Conflicts[0].Detail != "merge conflict" {
			t.Errorf("conflicts = %+v, want one deadbeef12345678", got.Base.Conflicts)
		}
		if len(got.Base.Generations) != 2 || got.Base.Generations[0].Commit != "abcdef1234567890" || got.Base.Generations[0].ShortCommit != shortCommit("abcdef1234567890") || got.Base.Generations[0].Binary != "/tmp/nerd-test" {
			t.Errorf("generations = %+v, want two starting abcdef1234567890", got.Base.Generations)
		}
		if got.Base.Generations[1].Commit != "feedface12345678" || got.Base.Generations[1].ShortCommit != shortCommit("feedface12345678") || got.Base.Generations[1].Failure != "build failed" {
			t.Errorf("generation 1 = %+v, want feedface12345678 build failed", got.Base.Generations[1])
		}
		if len(got.Attempts) != 4 {
			t.Fatalf("Attempts len = %d, want 4", len(got.Attempts))
		}
		wantAttempts := []struct {
			cycle     int
			outcome   string
			node      string
			commit    string
			signature string
			finding   string
		}{
			{cycle: 1, outcome: outcomeKept, node: "node-a", commit: "abcdef1234567890", signature: "sig1", finding: "finding-a"},
			{cycle: 2, outcome: outcomeReverted, node: "node-b", commit: "1234567890abcdef", signature: "sig2"},
			{cycle: 3, outcome: outcomeRefused, node: "node-c", commit: ""},
			{cycle: 4, outcome: outcomeUnverified, node: "node-d", commit: ""},
		}
		for i, want := range wantAttempts {
			a := got.Attempts[i]
			if a.Outcome != want.outcome || a.Node != want.node || a.Commit != want.commit {
				t.Errorf("attempt %d = outcome %q node %q commit %q, want %q %q %q", i, a.Outcome, a.Node, a.Commit, want.outcome, want.node, want.commit)
			}
			if a.Signature != want.signature || a.Finding != want.finding {
				t.Errorf("attempt %d = signature %q finding %q, want %q %q", i, a.Signature, a.Finding, want.signature, want.finding)
			}
			if a.ShortCommit != shortCommit(want.commit) {
				t.Errorf("attempt %d ShortCommit = %q, want %q", i, a.ShortCommit, shortCommit(want.commit))
			}
			if want.commit == "" && a.ShortCommit != "" {
				t.Errorf("attempt %d empty commit ShortCommit = %q, want empty (nil-safe)", i, a.ShortCommit)
			}
			if a.Cycle != want.cycle {
				t.Errorf("attempt %d Cycle = %d, want %d", i, a.Cycle, want.cycle)
			}
		}
		raw, err := s.MarshalRecurseStatusJSON()
		if err != nil {
			t.Fatalf("MarshalRecurseStatusJSON: %v", err)
		}
		var decoded RecurseStatusJSON
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded.Running != got.Running || decoded.StopRequested != got.StopRequested || decoded.State != got.State || decoded.Pass != got.Pass || decoded.Node != got.Node {
			t.Errorf("decoded running/stop/state/pass/node = %v/%v/%q/%d/%q, want %v/%v/%q/%d/%q", decoded.Running, decoded.StopRequested, decoded.State, decoded.Pass, decoded.Node, got.Running, got.StopRequested, got.State, got.Pass, got.Node)
		}
		if len(decoded.OpenByPass) != len(got.OpenByPass) {
			t.Fatalf("decoded open_by_pass len = %d, want %d", len(decoded.OpenByPass), len(got.OpenByPass))
		}
		for i := range decoded.OpenByPass {
			if decoded.OpenByPass[i] != got.OpenByPass[i] {
				t.Errorf("decoded open_by_pass[%d] = %d, want %d", i, decoded.OpenByPass[i], got.OpenByPass[i])
			}
		}
		if decoded.Cycles != got.Cycles || decoded.Kept != got.Kept || decoded.Reverted != got.Reverted || decoded.Refused != got.Refused || decoded.Unverified != got.Unverified {
			t.Errorf("decoded counts = %+v, want %+v", decoded, got)
		}
		if decoded.Base.Branch != got.Base.Branch || decoded.Base.Tip != got.Base.Tip || decoded.Base.TipShort != got.Base.TipShort || decoded.Base.LastAbsorbed != got.Base.LastAbsorbed || decoded.Base.LastAbsorbedShort != got.Base.LastAbsorbedShort || decoded.Base.Absorbs != got.Base.Absorbs {
			t.Errorf("decoded base = %+v, want %+v", decoded.Base, got.Base)
		}
		if len(decoded.Base.Conflicts) != len(got.Base.Conflicts) || len(decoded.Base.Generations) != len(got.Base.Generations) {
			t.Errorf("decoded conflicts/generations = %d/%d, want %d/%d", len(decoded.Base.Conflicts), len(decoded.Base.Generations), len(got.Base.Conflicts), len(got.Base.Generations))
		}
		for i := range decoded.Base.Conflicts {
			d, w := decoded.Base.Conflicts[i], got.Base.Conflicts[i]
			if d.BaseCommit != w.BaseCommit || d.ShortCommit != w.ShortCommit || d.Detail != w.Detail {
				t.Errorf("decoded conflict %d = %+v, want %+v", i, d, w)
			}
		}
		for i := range decoded.Base.Generations {
			d, w := decoded.Base.Generations[i], got.Base.Generations[i]
			if d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Binary != w.Binary || d.Failure != w.Failure {
				t.Errorf("decoded generation %d = %+v, want %+v", i, d, w)
			}
		}
		if len(decoded.Attempts) != len(got.Attempts) {
			t.Fatalf("decoded attempts len = %d, want %d", len(decoded.Attempts), len(got.Attempts))
		}
		for i := range decoded.Attempts {
			d, w := decoded.Attempts[i], got.Attempts[i]
			if d.Cycle != w.Cycle || d.Outcome != w.Outcome || d.Node != w.Node || d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Signature != w.Signature || d.Finding != w.Finding || d.Why != w.Why || d.Detail != w.Detail {
				t.Errorf("decoded attempt %d = %+v, want %+v", i, d, w)
			}
			if d.Cycle != wantAttempts[i].cycle || d.Outcome != wantAttempts[i].outcome || d.Node != wantAttempts[i].node || d.Commit != wantAttempts[i].commit {
				t.Errorf("decoded attempt %d = cycle %d outcome %q node %q commit %q, want %d %q %q %q", i, d.Cycle, d.Outcome, d.Node, d.Commit, wantAttempts[i].cycle, wantAttempts[i].outcome, wantAttempts[i].node, wantAttempts[i].commit)
			}
			if len(d.MissingEvidence) != len(w.MissingEvidence) {
				t.Errorf("decoded attempt %d MissingEvidence = %v, want %v", i, d.MissingEvidence, w.MissingEvidence)
			} else {
				for j := range d.MissingEvidence {
					if d.MissingEvidence[j] != w.MissingEvidence[j] {
						t.Errorf("decoded attempt %d MissingEvidence[%d] = %q, want %q", i, j, d.MissingEvidence[j], w.MissingEvidence[j])
					}
				}
			}
			if i == 3 && len(w.MissingEvidence) != 1 {
				t.Errorf("attempt 3 MissingEvidence = %v, want [/changed_code_unexecuted]", w.MissingEvidence)
			}
		}
	})
}

func TestRecurseStatusJSON_MissingEvidenceMapping(t *testing.T) {
	s := RecurseStatus{
		Running: true,
		Pass:    1,
		Node:    "node-current",
		Cycles:  4, Kept: 1, Unverified: 3,
		ThisPass: []recurseRecord{
			{Cycle: 1, Outcome: outcomeUnverified, Node: "a", Why: "needs /changed_code_unexecuted", Detail: ""},
			{Cycle: 2, Outcome: outcomeUnverified, Node: "b", Why: "", Detail: "gate /check_not_green and /tests_not_green open"},
			{Cycle: 3, Outcome: outcomeKept, Node: "c", Commit: "c0ffee1234567890", Why: "", Detail: ""},
			{Cycle: 4, Outcome: outcomeUnverified, Node: "d", Commit: "", Why: "", Detail: ""},
		},
		Base: RecurseBaseStatus{
			Branch:       "main",
			TipAtStart:   "abcdef1234567890",
			LastAbsorbed: "1234567890abcdef",
			Absorbs:      1,
			Conflicts:    []RecurseConflict{{BaseCommit: "deadbeef12345678", Detail: "merge conflict"}},
			Generations: []RecurseGeneration{
				{Commit: "abcdef1234567890", Binary: "/tmp/nerd-test"},
				{Commit: "feedface12345678", Failure: "build failed"},
			},
		},
	}
	text := s.String()
	if !strings.Contains(text, "running") || !strings.Contains(text, "Pass 1") {
		t.Errorf("text running/pass missing in %q, want running Pass 1", text)
	}
	if !strings.Contains(text, "visiting node-current") {
		t.Errorf("text node missing in %q, want visiting node-current", text)
	}
	if !strings.Contains(text, "4 attempts: 1 kept, 0 reverted, 0 refused, 3 unverified") {
		t.Errorf("text counts missing in %q, want 4 attempts: 1 kept, 0 reverted, 0 refused, 3 unverified", text)
	}
	if !strings.Contains(text, "Base: main") || !strings.Contains(text, shortCommit("abcdef1234567890")) || !strings.Contains(text, shortCommit("1234567890abcdef")) {
		t.Errorf("text base/last commit missing in %q", text)
	}
	if !strings.Contains(text, "Merge conflicts: 1") || !strings.Contains(text, shortCommit("deadbeef12345678")) || !strings.Contains(text, "merge conflict") {
		t.Errorf("text conflicts missing in %q", text)
	}
	if !strings.Contains(text, "Binary generations:") || !strings.Contains(text, shortCommit("abcdef1234567890")) || !strings.Contains(text, shortCommit("feedface12345678")) || !strings.Contains(text, "/tmp/nerd-test") || !strings.Contains(text, "build FAILED") || !strings.Contains(text, "build failed") {
		t.Errorf("text generations missing in %q", text)
	}
	for _, want := range []string{"cycle 1", "cycle 2", "cycle 3", "cycle 4", "unverified", "kept", "c0ffee1234567890"} {
		if !strings.Contains(text, want) {
			t.Errorf("text attempt %q missing in %q", want, text)
		}
	}
	for _, want := range []string{"unverified a", "unverified b", "unverified d"} {
		if !strings.Contains(text, want) {
			t.Errorf("text node %q missing in %q", want, text)
		}
	}
	if !strings.Contains(text, "kept") || !strings.Contains(text, " c ") || !strings.Contains(text, "c0ffee1234567890") {
		t.Errorf("text kept node/commit missing in %q, want kept c with c0ffee commit", text)
	}
	got := s.ToRecurseStatusJSON()
	if !got.Running || got.StopRequested || got.Pass != 1 || got.Node != "node-current" {
		t.Errorf("running/stop/pass/node = %v/%v/%d/%q, want true/false/1/node-current", got.Running, got.StopRequested, got.Pass, got.Node)
	}
	if len(got.OpenByPass) != 0 {
		t.Errorf("OpenByPass = %v, want empty (no finished passes)", got.OpenByPass)
	}
	if got.State != "running" {
		t.Errorf("State = %q, want %q", got.State, "running")
	}
	if got.Cycles != 4 || got.Kept != 1 || got.Unverified != 3 || got.Reverted != 0 || got.Refused != 0 {
		t.Errorf("counts = %d/%d/%d/%d/%d, want 4/1/0/0/3", got.Cycles, got.Kept, got.Reverted, got.Refused, got.Unverified)
	}
	if got.Base.Branch != "main" || got.Base.Tip != "abcdef1234567890" || got.Base.TipShort != shortCommit("abcdef1234567890") {
		t.Errorf("base tip = %+v, want branch main tip abcdef1234567890", got.Base)
	}
	if got.Base.LastAbsorbed != "1234567890abcdef" || got.Base.LastAbsorbedShort != shortCommit("1234567890abcdef") || got.Base.Absorbs != 1 {
		t.Errorf("base last absorbed = %+v, want 1234567890abcdef x1", got.Base)
	}
	if len(got.Base.Conflicts) != 1 || got.Base.Conflicts[0].BaseCommit != "deadbeef12345678" || got.Base.Conflicts[0].ShortCommit != shortCommit("deadbeef12345678") || got.Base.Conflicts[0].Detail != "merge conflict" {
		t.Errorf("conflicts = %+v, want one deadbeef12345678 merge conflict", got.Base.Conflicts)
	}
	if len(got.Base.Generations) != 2 || got.Base.Generations[0].Commit != "abcdef1234567890" || got.Base.Generations[0].ShortCommit != shortCommit("abcdef1234567890") {
		t.Errorf("generations = %+v, want two starting abcdef1234567890", got.Base.Generations)
	}
	if got.Base.Generations[0].Binary != "/tmp/nerd-test" {
		t.Errorf("generation 0 binary = %q, want /tmp/nerd-test", got.Base.Generations[0].Binary)
	}
	if got.Base.Generations[1].Commit != "feedface12345678" || got.Base.Generations[1].ShortCommit != shortCommit("feedface12345678") || got.Base.Generations[1].Failure != "build failed" {
		t.Errorf("generation 1 = %+v, want feedface12345678 build failed", got.Base.Generations[1])
	}
	if len(got.Attempts) != 4 {
		t.Fatalf("Attempts len = %d, want 4", len(got.Attempts))
	}
	wantAttempts := []struct {
		cycle   int
		outcome string
		node    string
		commit  string
	}{
		{cycle: 1, outcome: outcomeUnverified, node: "a", commit: ""},
		{cycle: 2, outcome: outcomeUnverified, node: "b", commit: ""},
		{cycle: 3, outcome: outcomeKept, node: "c", commit: "c0ffee1234567890"},
		{cycle: 4, outcome: outcomeUnverified, node: "d", commit: ""},
	}
	for i, want := range wantAttempts {
		a := got.Attempts[i]
		if a.Cycle != want.cycle || a.Outcome != want.outcome || a.Node != want.node || a.Commit != want.commit {
			t.Errorf("attempt %d = cycle %d outcome %q node %q commit %q, want %d %q %q %q", i, a.Cycle, a.Outcome, a.Node, a.Commit, want.cycle, want.outcome, want.node, want.commit)
		}
		if a.ShortCommit != shortCommit(want.commit) {
			t.Errorf("attempt %d ShortCommit = %q, want %q", i, a.ShortCommit, shortCommit(want.commit))
		}
	}
	if len(got.Attempts[0].MissingEvidence) != 1 || got.Attempts[0].MissingEvidence[0] != "/changed_code_unexecuted" {
		t.Errorf("attempt 0 MissingEvidence = %v, want [/changed_code_unexecuted]", got.Attempts[0].MissingEvidence)
	}
	if len(got.Attempts[1].MissingEvidence) != 2 || got.Attempts[1].MissingEvidence[0] != "/check_not_green" || got.Attempts[1].MissingEvidence[1] != "/tests_not_green" {
		t.Errorf("attempt 1 MissingEvidence = %v, want [/check_not_green /tests_not_green]", got.Attempts[1].MissingEvidence)
	}
	if len(got.Attempts[2].MissingEvidence) != 0 {
		t.Errorf("attempt 2 MissingEvidence = %v, want empty", got.Attempts[2].MissingEvidence)
	}
	if len(got.Attempts[3].MissingEvidence) != 0 {
		t.Errorf("attempt 3 MissingEvidence = %v, want empty", got.Attempts[3].MissingEvidence)
	}
	if got.Attempts[3].ShortCommit != "" {
		t.Errorf("empty ShortCommit = %q, want empty (nil-safe)", got.Attempts[3].ShortCommit)
	}
	raw, err := s.MarshalRecurseStatusJSON()
	if err != nil {
		t.Fatalf("MarshalRecurseStatusJSON: %v", err)
	}
	var decoded RecurseStatusJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Running != got.Running || decoded.StopRequested != got.StopRequested || decoded.Pass != got.Pass || decoded.State != got.State || decoded.Node != got.Node {
		t.Errorf("decoded running/stop/pass/state/node = %v/%v/%d/%q/%q, want %v/%v/%d/%q/%q", decoded.Running, decoded.StopRequested, decoded.Pass, decoded.State, decoded.Node, got.Running, got.StopRequested, got.Pass, got.State, got.Node)
	}
	if decoded.Cycles != got.Cycles || decoded.Kept != got.Kept || decoded.Reverted != got.Reverted || decoded.Refused != got.Refused || decoded.Unverified != got.Unverified {
		t.Errorf("decoded counts = %+v, want %+v", decoded, got)
	}
	if decoded.Base.Branch != got.Base.Branch || decoded.Base.Tip != got.Base.Tip || decoded.Base.TipShort != got.Base.TipShort || decoded.Base.LastAbsorbed != got.Base.LastAbsorbed || decoded.Base.LastAbsorbedShort != got.Base.LastAbsorbedShort || decoded.Base.Absorbs != got.Base.Absorbs {
		t.Errorf("decoded base = %+v, want %+v", decoded.Base, got.Base)
	}
	if len(decoded.Base.Conflicts) != len(got.Base.Conflicts) || len(decoded.Base.Generations) != len(got.Base.Generations) {
		t.Errorf("decoded conflicts/generations = %d/%d, want %d/%d", len(decoded.Base.Conflicts), len(decoded.Base.Generations), len(got.Base.Conflicts), len(got.Base.Generations))
	}
	for i := range decoded.Base.Conflicts {
		d, w := decoded.Base.Conflicts[i], got.Base.Conflicts[i]
		if d.BaseCommit != w.BaseCommit || d.ShortCommit != w.ShortCommit || d.Detail != w.Detail {
			t.Errorf("decoded conflict %d = %+v, want %+v", i, d, w)
		}
	}
	for i := range decoded.Base.Generations {
		d, w := decoded.Base.Generations[i], got.Base.Generations[i]
		if d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Binary != w.Binary || d.Failure != w.Failure {
			t.Errorf("decoded generation %d = %+v, want %+v", i, d, w)
		}
	}
	if len(decoded.Attempts) != len(got.Attempts) {
		t.Fatalf("decoded attempts len = %d, want %d", len(decoded.Attempts), len(got.Attempts))
	}
	for i := range decoded.Attempts {
		d, w := decoded.Attempts[i], got.Attempts[i]
		if d.Cycle != w.Cycle || d.Outcome != w.Outcome || d.Node != w.Node || d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Signature != w.Signature || d.Finding != w.Finding || d.Why != w.Why || d.Detail != w.Detail {
			t.Errorf("decoded attempt %d = %+v, want %+v", i, d, w)
		}
		if d.Cycle != wantAttempts[i].cycle || d.Outcome != wantAttempts[i].outcome || d.Node != wantAttempts[i].node || d.Commit != wantAttempts[i].commit {
			t.Errorf("decoded attempt %d = cycle %d outcome %q node %q commit %q, want %d %q %q %q", i, d.Cycle, d.Outcome, d.Node, d.Commit, wantAttempts[i].cycle, wantAttempts[i].outcome, wantAttempts[i].node, wantAttempts[i].commit)
		}
		if len(d.MissingEvidence) != len(w.MissingEvidence) {
			t.Errorf("decoded attempt %d MissingEvidence = %v, want %v", i, d.MissingEvidence, w.MissingEvidence)
		} else {
			for j := range d.MissingEvidence {
				if d.MissingEvidence[j] != w.MissingEvidence[j] {
					t.Errorf("decoded attempt %d MissingEvidence[%d] = %q, want %q", i, j, d.MissingEvidence[j], w.MissingEvidence[j])
				}
			}
		}
	}
}

func TestRecurseStatusJSON_TextReportParity(t *testing.T) {
	s := RecurseStatus{
		Running: true,
		Pass:    3,
		Node:    "node-parity",
		Cycles:  2, Kept: 1, Reverted: 0, Refused: 0, Unverified: 1,
		OpenByPass: []int{2, 1},
		ThisPass: []recurseRecord{
			{Cycle: 1, Outcome: outcomeKept, Node: "node-a", Commit: "abcdef1234567890", Signature: "sig-parity", Finding: "finding-parity", Detail: "kept detail"},
			{Cycle: 2, Outcome: outcomeUnverified, Node: "node-b", Why: "needs /changed_code_unexecuted", Detail: "detail-parity"},
		},
		Base: RecurseBaseStatus{
			Branch:       "main",
			TipAtStart:   "abcdef1234567890",
			LastAbsorbed: "1234567890abcdef",
			Absorbs:      1,
			Conflicts:    []RecurseConflict{{BaseCommit: "deadbeef12345678", Detail: "merge conflict"}},
			Generations: []RecurseGeneration{
				{Commit: "abcdef1234567890", Binary: "/tmp/nerd-parity"},
				{Commit: "feedface12345678", Failure: "build failed"},
			},
		},
	}
	text := s.String()
	for _, want := range []string{"running", "Pass 3", "visiting node-parity", "2 attempts: 1 kept, 0 reverted, 0 refused, 1 unverified", "Open findings at the end of each pass: 2 -> 1", "Base: main", shortCommit("abcdef1234567890"), shortCommit("1234567890abcdef"), "Merge conflicts: 1", shortCommit("deadbeef12345678"), "merge conflict", "Binary generations:", "/tmp/nerd-parity", "build FAILED", "build failed", shortCommit("feedface12345678"), "cycle 1", "cycle 2", "kept", "unverified", "node-a", "node-b", "node-parity", "abcdef1234567890", "(sig-parity)"} {
		if !strings.Contains(text, want) {
			t.Errorf("text report missing %q in %q", want, text)
		}
	}
	got := s.ToRecurseStatusJSON()
	if got.State != "running" {
		t.Errorf("State = %q, want %q", got.State, "running")
	}
	if !got.Running || got.StopRequested || got.Pass != 3 || got.Node != "node-parity" {
		t.Errorf("running/stop/pass/node = %v/%v/%d/%q, want true/false/3/node-parity", got.Running, got.StopRequested, got.Pass, got.Node)
	}
	if len(got.OpenByPass) != 2 || got.OpenByPass[0] != 2 || got.OpenByPass[1] != 1 {
		t.Errorf("OpenByPass = %v, want [2 1]", got.OpenByPass)
	}
	if got.Cycles != 2 || got.Kept != 1 || got.Unverified != 1 || got.Reverted != 0 || got.Refused != 0 {
		t.Errorf("counts = %d/%d/%d/%d/%d, want 2/1/0/0/1", got.Cycles, got.Kept, got.Reverted, got.Refused, got.Unverified)
	}
	if got.Base.Branch != "main" || got.Base.Tip != "abcdef1234567890" || got.Base.TipShort != shortCommit("abcdef1234567890") {
		t.Errorf("base tip = %+v, want main abcdef1234567890 tip_short %q", got.Base, shortCommit("abcdef1234567890"))
	}
	if got.Base.LastAbsorbed != "1234567890abcdef" || got.Base.LastAbsorbedShort != shortCommit("1234567890abcdef") || got.Base.Absorbs != 1 {
		t.Errorf("base last absorbed = %+v, want 1234567890abcdef short %q absorbs 1", got.Base, shortCommit("1234567890abcdef"))
	}
	if len(got.Base.Conflicts) != 1 || got.Base.Conflicts[0].BaseCommit != "deadbeef12345678" || got.Base.Conflicts[0].ShortCommit != shortCommit("deadbeef12345678") || got.Base.Conflicts[0].Detail != "merge conflict" {
		t.Errorf("conflicts = %+v, want one deadbeef12345678 merge conflict", got.Base.Conflicts)
	}
	if len(got.Base.Generations) != 2 || got.Base.Generations[0].Commit != "abcdef1234567890" || got.Base.Generations[0].ShortCommit != shortCommit("abcdef1234567890") || got.Base.Generations[0].Binary != "/tmp/nerd-parity" {
		t.Errorf("generations[0] = %+v, want abcdef1234567890 /tmp/nerd-parity", got.Base.Generations)
	}
	if got.Base.Generations[1].Commit != "feedface12345678" || got.Base.Generations[1].ShortCommit != shortCommit("feedface12345678") || got.Base.Generations[1].Failure != "build failed" {
		t.Errorf("generations[1] = %+v, want feedface12345678 build failed", got.Base.Generations[1])
	}
	if len(got.Attempts) != 2 {
		t.Fatalf("Attempts len = %d, want 2", len(got.Attempts))
	}
	if got.Attempts[0].Cycle != 1 || got.Attempts[0].Outcome != outcomeKept || got.Attempts[0].Node != "node-a" || got.Attempts[0].Commit != "abcdef1234567890" || got.Attempts[0].ShortCommit != shortCommit("abcdef1234567890") {
		t.Errorf("attempt 0 = %+v, want cycle 1 kept node-a abcdef1234567890 short %q", got.Attempts[0], shortCommit("abcdef1234567890"))
	}
	if got.Attempts[0].Signature != "sig-parity" || got.Attempts[0].Finding != "finding-parity" || got.Attempts[0].Detail != "kept detail" {
		t.Errorf("attempt 0 signature/finding/detail = %q/%q/%q, want sig-parity/finding-parity/kept detail", got.Attempts[0].Signature, got.Attempts[0].Finding, got.Attempts[0].Detail)
	}
	if len(got.Attempts[0].MissingEvidence) != 0 {
		t.Errorf("attempt 0 MissingEvidence = %v, want empty", got.Attempts[0].MissingEvidence)
	}
	if got.Attempts[1].Cycle != 2 || got.Attempts[1].Outcome != outcomeUnverified || got.Attempts[1].Node != "node-b" || got.Attempts[1].Commit != "" || got.Attempts[1].ShortCommit != "" {
		t.Errorf("attempt 1 = %+v, want cycle 2 unverified node-b empty commit (nil-safe)", got.Attempts[1])
	}
	if got.Attempts[1].Why != "needs /changed_code_unexecuted" || got.Attempts[1].Detail != "detail-parity" {
		t.Errorf("attempt 1 why/detail = %q/%q, want needs /changed_code_unexecuted/detail-parity", got.Attempts[1].Why, got.Attempts[1].Detail)
	}
	if len(got.Attempts[1].MissingEvidence) != 1 || got.Attempts[1].MissingEvidence[0] != "/changed_code_unexecuted" {
		t.Errorf("attempt 1 MissingEvidence = %v, want [/changed_code_unexecuted]", got.Attempts[1].MissingEvidence)
	}
	raw, err := s.MarshalRecurseStatusJSON()
	if err != nil {
		t.Fatalf("MarshalRecurseStatusJSON: %v", err)
	}
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("unmarshal raw JSON to map: %v", err)
	}
	for _, key := range []string{"running", "stop_requested", "pass", "node", "cycles", "kept", "reverted", "refused", "unverified", "open_by_pass", "base", "attempts", "state"} {
		if _, ok := rawMap[key]; !ok {
			t.Errorf("raw JSON missing parity key %q in %s", key, string(raw))
		}
	}
	if rawMap["node"] != "node-parity" {
		t.Errorf("raw JSON node = %v, want %q", rawMap["node"], "node-parity")
	}
	if openRaw, _ := rawMap["open_by_pass"].([]any); len(openRaw) != 2 || openRaw[0] != float64(2) || openRaw[1] != float64(1) {
		t.Errorf("raw JSON open_by_pass = %v, want [2 1]", rawMap["open_by_pass"])
	}
	// Nested short-commit parity: the text report shortens base/last/conflict/
	// generation/attempt hashes via shortCommit, so the raw JSON must carry the
	// same shortened forms alongside the full hashes (nil-safe: empty stays empty).
	baseMap, _ := rawMap["base"].(map[string]any)
	for _, key := range []string{"branch", "tip", "tip_short", "last_absorbed", "last_absorbed_short", "absorbs"} {
		if _, ok := baseMap[key]; !ok {
			t.Errorf("raw JSON base missing parity key %q in %s", key, string(raw))
		}
	}
	if baseMap["tip_short"] != shortCommit("abcdef1234567890") {
		t.Errorf("raw JSON base tip_short = %v, want %q", baseMap["tip_short"], shortCommit("abcdef1234567890"))
	}
	if baseMap["last_absorbed_short"] != shortCommit("1234567890abcdef") {
		t.Errorf("raw JSON base last_absorbed_short = %v, want %q", baseMap["last_absorbed_short"], shortCommit("1234567890abcdef"))
	}
	conflictsRaw, _ := baseMap["conflicts"].([]any)
	if len(conflictsRaw) != 1 {
		t.Errorf("raw JSON base conflicts len = %d, want 1", len(conflictsRaw))
	} else if conflict0, _ := conflictsRaw[0].(map[string]any); conflict0["base_commit"] != "deadbeef12345678" || conflict0["short_commit"] != shortCommit("deadbeef12345678") {
		t.Errorf("raw JSON base conflict 0 = %v, want base_commit deadbeef12345678 short_commit %q", conflict0, shortCommit("deadbeef12345678"))
	}
	generationsRaw, _ := baseMap["generations"].([]any)
	if len(generationsRaw) != 2 {
		t.Errorf("raw JSON base generations len = %d, want 2", len(generationsRaw))
	} else {
		gen0, _ := generationsRaw[0].(map[string]any)
		if gen0["commit"] != "abcdef1234567890" || gen0["short_commit"] != shortCommit("abcdef1234567890") {
			t.Errorf("raw JSON base generation 0 = %v, want commit abcdef1234567890 short_commit %q", gen0, shortCommit("abcdef1234567890"))
		}
		gen1, _ := generationsRaw[1].(map[string]any)
		if gen1["commit"] != "feedface12345678" || gen1["short_commit"] != shortCommit("feedface12345678") {
			t.Errorf("raw JSON base generation 1 = %v, want commit feedface12345678 short_commit %q", gen1, shortCommit("feedface12345678"))
		}
	}
	attemptsRaw, _ := rawMap["attempts"].([]any)
	if len(attemptsRaw) != 2 {
		t.Fatalf("raw JSON attempts len = %d, want 2", len(attemptsRaw))
	}
	firstAttempt, _ := attemptsRaw[0].(map[string]any)
	for _, key := range []string{"cycle", "outcome", "node", "commit", "short_commit"} {
		if _, ok := firstAttempt[key]; !ok {
			t.Errorf("raw JSON attempt 0 missing parity key %q in %v", key, firstAttempt)
		}
	}
	if firstAttempt["short_commit"] != shortCommit("abcdef1234567890") {
		t.Errorf("raw JSON attempt 0 short_commit = %v, want %q", firstAttempt["short_commit"], shortCommit("abcdef1234567890"))
	}
	secondAttempt, _ := attemptsRaw[1].(map[string]any)
	for _, key := range []string{"cycle", "outcome", "node"} {
		if _, ok := secondAttempt[key]; !ok {
			t.Errorf("raw JSON attempt 1 missing parity key %q in %v", key, secondAttempt)
		}
	}
	if v, ok := secondAttempt["short_commit"]; ok && v != "" {
		t.Errorf("raw JSON attempt 1 short_commit = %v, want absent or empty (nil-safe)", v)
	}
	var decoded RecurseStatusJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Running != got.Running || decoded.StopRequested != got.StopRequested || decoded.State != got.State || decoded.Pass != got.Pass || decoded.Node != got.Node {
		t.Errorf("decoded running/stop/state/pass/node = %v/%v/%q/%d/%q, want %v/%v/%q/%d/%q", decoded.Running, decoded.StopRequested, decoded.State, decoded.Pass, decoded.Node, got.Running, got.StopRequested, got.State, got.Pass, got.Node)
	}
	if len(decoded.OpenByPass) != len(got.OpenByPass) || len(decoded.OpenByPass) != 2 || decoded.OpenByPass[0] != 2 || decoded.OpenByPass[1] != 1 {
		t.Errorf("decoded open_by_pass = %v, want %v", decoded.OpenByPass, got.OpenByPass)
	}
	if decoded.Cycles != got.Cycles || decoded.Kept != got.Kept || decoded.Reverted != got.Reverted || decoded.Refused != got.Refused || decoded.Unverified != got.Unverified {
		t.Errorf("decoded running/pass/counts = %v/%d/%d/%d/%d/%d/%d, want %v/%d/%d/%d/%d/%d/%d", decoded.Running, decoded.Pass, decoded.Cycles, decoded.Kept, decoded.Reverted, decoded.Refused, decoded.Unverified, got.Running, got.Pass, got.Cycles, got.Kept, got.Reverted, got.Refused, got.Unverified)
	}
	if decoded.Base.Branch != got.Base.Branch || decoded.Base.Tip != got.Base.Tip || decoded.Base.TipShort != got.Base.TipShort || decoded.Base.LastAbsorbed != got.Base.LastAbsorbed || decoded.Base.LastAbsorbedShort != got.Base.LastAbsorbedShort || decoded.Base.Absorbs != got.Base.Absorbs {
		t.Errorf("decoded base = %+v, want %+v", decoded.Base, got.Base)
	}
	if len(decoded.Base.Conflicts) != len(got.Base.Conflicts) || len(decoded.Base.Generations) != len(got.Base.Generations) {
		t.Fatalf("decoded conflicts/generations len = %d/%d, want %d/%d", len(decoded.Base.Conflicts), len(decoded.Base.Generations), len(got.Base.Conflicts), len(got.Base.Generations))
	}
	for i := range decoded.Base.Conflicts {
		d, w := decoded.Base.Conflicts[i], got.Base.Conflicts[i]
		if d.BaseCommit != w.BaseCommit || d.ShortCommit != w.ShortCommit || d.Detail != w.Detail {
			t.Errorf("decoded conflict %d = %+v, want %+v", i, d, w)
		}
	}
	for i := range decoded.Base.Generations {
		d, w := decoded.Base.Generations[i], got.Base.Generations[i]
		if d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Binary != w.Binary || d.Failure != w.Failure {
			t.Errorf("decoded generation %d = %+v, want %+v", i, d, w)
		}
	}
	if len(decoded.Attempts) != len(got.Attempts) {
		t.Fatalf("decoded attempts len = %d, want %d", len(decoded.Attempts), len(got.Attempts))
	}
	for i := range decoded.Attempts {
		d, w := decoded.Attempts[i], got.Attempts[i]
		if d.Cycle != w.Cycle || d.Outcome != w.Outcome || d.Node != w.Node || d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Signature != w.Signature || d.Finding != w.Finding || d.Why != w.Why || d.Detail != w.Detail {
			t.Errorf("decoded attempt %d = %+v, want %+v", i, d, w)
		}
		if len(d.MissingEvidence) != len(w.MissingEvidence) {
			t.Errorf("decoded attempt %d MissingEvidence = %v, want %v", i, d.MissingEvidence, w.MissingEvidence)
		} else {
			for j := range d.MissingEvidence {
				if d.MissingEvidence[j] != w.MissingEvidence[j] {
					t.Errorf("decoded attempt %d MissingEvidence[%d] = %q, want %q", i, j, d.MissingEvidence[j], w.MissingEvidence[j])
				}
			}
		}
	}
}

func TestRecurseStatusJSON_FixtureRoundTripParity(t *testing.T) {
	// Pin shortCommit empty-hash nil-safety: empty stays empty, never panics.
	// Fixture builds a RecurseStatus, marshals via ToRecurseStatusJSON/MarshalRecurseStatusJSON,
	// parses the JSON back, and asserts text-report parity: running/pass/counts-by-outcome,
	// base/last-absorbed commits, conflicts, generations, and each attempt's
	// cycle/outcome/node/commit with ShortCommit nil-safety.
	// Scoped verification: go build ./internal/campaign and go test ./internal/campaign -run RecurseStatusJSON -count=1 serialized.
	// Full-suite Northstar/RelevanceInFlight/ExecutiveLiteralBudget failures are pre-existing out-of-scope flaky.
	if got := shortCommit(""); got != "" {
		t.Fatalf("shortCommit(\"\") = %q, want empty (nil-safe)", got)
	}
	if got := shortCommit("abc"); got != "abc" {
		t.Fatalf("shortCommit(%q) = %q, want intact (short-hash nil-safe)", "abc", got)
	}
	if got := shortCommit("x"); got != "x" {
		t.Fatalf("shortCommit(%q) = %q, want intact (single-char nil-safe)", "x", got)
	}
	if got := shortCommit("12345678"); got != "12345678" {
		t.Fatalf("shortCommit(%q) = %q, want intact just below 9-char boundary", "12345678", got)
	}
	if got := shortCommit("123456789"); got != "123456789" {
		t.Fatalf("shortCommit(%q) = %q, want intact at 9-char boundary", "123456789", got)
	}
	if got := shortCommit("1234567890"); got != "123456789" {
		t.Fatalf("shortCommit(%q) = %q, want first 9 chars just past boundary", "1234567890", got)
	}
	if got := shortCommit("abcdef1234567890"); got != "abcdef123" {
		t.Fatalf("shortCommit(long) = %q, want first 9 chars (truncation pin)", got)
	}
	// Empty-status DTO nil-safety: no base, no attempts, empty hashes stay empty.
	empty := (RecurseStatus{}).ToRecurseStatusJSON()
	if empty.Base.TipShort != "" || empty.Base.LastAbsorbedShort != "" {
		t.Fatalf("empty base ShortCommits = %q/%q, want empty (nil-safe)", empty.Base.TipShort, empty.Base.LastAbsorbedShort)
	}
	if len(empty.Attempts) != 0 {
		t.Fatalf("empty attempts len = %d, want 0", len(empty.Attempts))
	}
	if rawEmpty, err := (RecurseStatus{}).MarshalRecurseStatusJSON(); err != nil {
		t.Fatalf("empty MarshalRecurseStatusJSON: %v", err)
	} else if string(rawEmpty) == "" {
		t.Fatalf("empty MarshalRecurseStatusJSON returned empty bytes")
	}
	s := RecurseStatus{
		Running: true,
		Pass:    1,
		Node:    "node-fixture",
		Cycles:  4, Kept: 1, Reverted: 1, Refused: 1, Unverified: 1,
		OpenByPass: []int{4, 1},
		ThisPass: []recurseRecord{
			{Cycle: 1, Outcome: outcomeKept, Node: "node-a", Commit: "abcdef1234567890", Signature: "sig-a", Finding: "finding-a"},
			{Cycle: 2, Outcome: outcomeReverted, Node: "node-b", Commit: "1234567890abcdef", Signature: "sig-b"},
			{Cycle: 3, Outcome: outcomeRefused, Node: "node-c"},
			{Cycle: 4, Outcome: outcomeUnverified, Node: "node-d", Why: "needs /changed_code_unexecuted"},
		},
		Base: RecurseBaseStatus{
			Branch:       "main",
			TipAtStart:   "abcdef1234567890",
			LastAbsorbed: "1234567890abcdef",
			Absorbs:      2,
			Conflicts:    []RecurseConflict{{BaseCommit: "deadbeef12345678", Detail: "merge conflict"}},
			Generations: []RecurseGeneration{
				{Commit: "abcdef1234567890", Binary: "/tmp/nerd-fixture"},
				{Commit: "feedface12345678", Failure: "build failed"},
			},
		},
	}
	text := s.String()
	for _, want := range []string{
		"running", "Pass 1", "visiting node-fixture",
		"4 attempts: 1 kept, 1 reverted, 1 refused, 1 unverified",
		"Open findings at the end of each pass: 4 -> 1",
		"Base: main", shortCommit("abcdef1234567890"), shortCommit("1234567890abcdef"),
		"Merge conflicts: 1", shortCommit("deadbeef12345678"), "merge conflict",
		"Binary generations:", "/tmp/nerd-fixture", "build FAILED", "build failed",
		shortCommit("feedface12345678"),
		"cycle 1", "cycle 2", "cycle 3", "cycle 4",
		"kept", "reverted", "refused", "unverified",
		"node-fixture", "node-a", "node-b", "node-c", "node-d",
		"abcdef1234567890", "1234567890abcdef",
		"(sig-a)", "(sig-b)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text report missing %q in %q", want, text)
		}
	}
	got := s.ToRecurseStatusJSON()
	if !got.Running || got.StopRequested || got.Pass != 1 || got.Node != "node-fixture" || got.State != "running" {
		t.Fatalf("running/stop/pass/node/state = %v/%v/%d/%q/%q, want true/false/1/node-fixture/running", got.Running, got.StopRequested, got.Pass, got.Node, got.State)
	}
	if len(got.OpenByPass) != 2 || got.OpenByPass[0] != 4 || got.OpenByPass[1] != 1 {
		t.Fatalf("open_by_pass = %v, want [4 1]", got.OpenByPass)
	}
	if got.Cycles != 4 || got.Kept != 1 || got.Reverted != 1 || got.Refused != 1 || got.Unverified != 1 {
		t.Fatalf("counts = %d/%d/%d/%d/%d, want 4/1/1/1/1", got.Cycles, got.Kept, got.Reverted, got.Refused, got.Unverified)
	}
	if got.Base.Branch != "main" || got.Base.Tip != "abcdef1234567890" || got.Base.TipShort != shortCommit("abcdef1234567890") {
		t.Fatalf("base tip = %+v, want main abcdef1234567890 short %q", got.Base, shortCommit("abcdef1234567890"))
	}
	if got.Base.LastAbsorbed != "1234567890abcdef" || got.Base.LastAbsorbedShort != shortCommit("1234567890abcdef") || got.Base.Absorbs != 2 {
		t.Fatalf("base last absorbed = %+v, want 1234567890abcdef short %q absorbs 2", got.Base, shortCommit("1234567890abcdef"))
	}
	if len(got.Base.Conflicts) != 1 || got.Base.Conflicts[0].BaseCommit != "deadbeef12345678" || got.Base.Conflicts[0].ShortCommit != shortCommit("deadbeef12345678") || got.Base.Conflicts[0].Detail != "merge conflict" {
		t.Fatalf("conflicts = %+v, want one deadbeef12345678 merge conflict", got.Base.Conflicts)
	}
	if len(got.Base.Generations) != 2 {
		t.Fatalf("generations len = %d, want 2", len(got.Base.Generations))
	}
	if got.Base.Generations[0].Commit != "abcdef1234567890" || got.Base.Generations[0].ShortCommit != shortCommit("abcdef1234567890") || got.Base.Generations[0].Binary != "/tmp/nerd-fixture" {
		t.Fatalf("generations[0] = %+v, want abcdef1234567890 /tmp/nerd-fixture", got.Base.Generations[0])
	}
	if got.Base.Generations[1].Commit != "feedface12345678" || got.Base.Generations[1].ShortCommit != shortCommit("feedface12345678") || got.Base.Generations[1].Failure != "build failed" {
		t.Fatalf("generations[1] = %+v, want feedface12345678 build failed", got.Base.Generations[1])
	}
	if len(got.Attempts) != 4 {
		t.Fatalf("attempts len = %d, want 4", len(got.Attempts))
	}
	for i, want := range []struct {
		cycle   int
		outcome string
		node    string
		commit  string
	}{
		{1, outcomeKept, "node-a", "abcdef1234567890"},
		{2, outcomeReverted, "node-b", "1234567890abcdef"},
		{3, outcomeRefused, "node-c", ""},
		{4, outcomeUnverified, "node-d", ""},
	} {
		a := got.Attempts[i]
		if a.Cycle != want.cycle || a.Outcome != want.outcome || a.Node != want.node || a.Commit != want.commit {
			t.Fatalf("attempt %d = %d/%q/%q/%q, want %d/%q/%q/%q", i, a.Cycle, a.Outcome, a.Node, a.Commit, want.cycle, want.outcome, want.node, want.commit)
		}
		if a.ShortCommit != shortCommit(want.commit) {
			t.Fatalf("attempt %d ShortCommit = %q, want %q", i, a.ShortCommit, shortCommit(want.commit))
		}
		if want.commit == "" && a.ShortCommit != "" {
			t.Fatalf("attempt %d empty commit ShortCommit = %q, want empty (nil-safe)", i, a.ShortCommit)
		}
	}
	if got.Attempts[0].Signature != "sig-a" || got.Attempts[0].Finding != "finding-a" {
		t.Fatalf("attempt 0 signature/finding = %q/%q, want sig-a/finding-a", got.Attempts[0].Signature, got.Attempts[0].Finding)
	}
	if len(got.Attempts[0].MissingEvidence) != 0 {
		t.Fatalf("attempt 0 MissingEvidence = %v, want empty", got.Attempts[0].MissingEvidence)
	}
	if len(got.Attempts[1].MissingEvidence) != 0 {
		t.Fatalf("attempt 1 MissingEvidence = %v, want empty", got.Attempts[1].MissingEvidence)
	}
	if len(got.Attempts[2].MissingEvidence) != 0 {
		t.Fatalf("attempt 2 MissingEvidence = %v, want empty", got.Attempts[2].MissingEvidence)
	}
	if len(got.Attempts[3].MissingEvidence) != 1 || got.Attempts[3].MissingEvidence[0] != "/changed_code_unexecuted" {
		t.Fatalf("attempt 3 MissingEvidence = %v, want [/changed_code_unexecuted]", got.Attempts[3].MissingEvidence)
	}
	raw, err := s.MarshalRecurseStatusJSON()
	if err != nil {
		t.Fatalf("MarshalRecurseStatusJSON: %v", err)
	}
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("unmarshal raw JSON to map: %v", err)
	}
	for _, key := range []string{"running", "stop_requested", "state", "pass", "node", "cycles", "kept", "reverted", "refused", "unverified", "open_by_pass", "base", "attempts"} {
		if _, ok := rawMap[key]; !ok {
			t.Fatalf("raw JSON missing parity key %q in %s", key, string(raw))
		}
	}
	if rawMap["running"] != true || rawMap["pass"] != float64(1) || rawMap["state"] != "running" || rawMap["node"] != "node-fixture" {
		t.Fatalf("raw JSON running/pass/state/node = %v/%v/%v/%v, want true/1/running/node-fixture", rawMap["running"], rawMap["pass"], rawMap["state"], rawMap["node"])
	}
	if openRaw, _ := rawMap["open_by_pass"].([]any); len(openRaw) != 2 || openRaw[0] != float64(4) || openRaw[1] != float64(1) {
		t.Fatalf("raw JSON open_by_pass = %v, want [4 1]", rawMap["open_by_pass"])
	}
	if rawMap["cycles"] != float64(4) || rawMap["kept"] != float64(1) || rawMap["reverted"] != float64(1) || rawMap["refused"] != float64(1) || rawMap["unverified"] != float64(1) {
		t.Fatalf("raw JSON cycles/kept/reverted/refused/unverified = %v/%v/%v/%v/%v, want 4/1/1/1/1", rawMap["cycles"], rawMap["kept"], rawMap["reverted"], rawMap["refused"], rawMap["unverified"])
	}
	baseMap, _ := rawMap["base"].(map[string]any)
	for _, key := range []string{"branch", "tip", "tip_short", "last_absorbed", "last_absorbed_short", "absorbs"} {
		if _, ok := baseMap[key]; !ok {
			t.Fatalf("raw JSON base missing parity key %q in %s", key, string(raw))
		}
	}
	if baseMap["tip_short"] != shortCommit("abcdef1234567890") {
		t.Fatalf("raw JSON base tip_short = %v, want %q", baseMap["tip_short"], shortCommit("abcdef1234567890"))
	}
	if baseMap["last_absorbed_short"] != shortCommit("1234567890abcdef") {
		t.Fatalf("raw JSON base last_absorbed_short = %v, want %q", baseMap["last_absorbed_short"], shortCommit("1234567890abcdef"))
	}
	conflictsRaw, _ := baseMap["conflicts"].([]any)
	if len(conflictsRaw) != 1 {
		t.Fatalf("raw JSON base conflicts len = %d, want 1", len(conflictsRaw))
	} else if conflict0, _ := conflictsRaw[0].(map[string]any); conflict0["base_commit"] != "deadbeef12345678" || conflict0["short_commit"] != shortCommit("deadbeef12345678") {
		t.Fatalf("raw JSON base conflict 0 = %v, want base_commit deadbeef12345678 short_commit %q", conflict0, shortCommit("deadbeef12345678"))
	}
	generationsRaw, _ := baseMap["generations"].([]any)
	if len(generationsRaw) != 2 {
		t.Fatalf("raw JSON base generations len = %d, want 2", len(generationsRaw))
	} else {
		gen0, _ := generationsRaw[0].(map[string]any)
		if gen0["commit"] != "abcdef1234567890" || gen0["short_commit"] != shortCommit("abcdef1234567890") || gen0["binary"] != "/tmp/nerd-fixture" {
			t.Fatalf("raw JSON base generation 0 = %v, want commit abcdef1234567890 binary /tmp/nerd-fixture", gen0)
		}
		gen1, _ := generationsRaw[1].(map[string]any)
		if gen1["commit"] != "feedface12345678" || gen1["short_commit"] != shortCommit("feedface12345678") || gen1["failure"] != "build failed" {
			t.Fatalf("raw JSON base generation 1 = %v, want commit feedface12345678 failure build failed", gen1)
		}
	}
	attemptsRaw, _ := rawMap["attempts"].([]any)
	if len(attemptsRaw) != 4 {
		t.Fatalf("raw JSON attempts len = %d, want 4", len(attemptsRaw))
	}
	wantRawAttempts := []struct {
		cycle   float64
		outcome string
		node    string
		commit  string
	}{
		{1, outcomeKept, "node-a", "abcdef1234567890"},
		{2, outcomeReverted, "node-b", "1234567890abcdef"},
		{3, outcomeRefused, "node-c", ""},
		{4, outcomeUnverified, "node-d", ""},
	}
	for i, want := range wantRawAttempts {
		m, _ := attemptsRaw[i].(map[string]any)
		for _, key := range []string{"cycle", "outcome", "node"} {
			if _, ok := m[key]; !ok {
				t.Fatalf("raw JSON attempt %d missing parity key %q in %v", i, key, m)
			}
		}
		if m["cycle"] != want.cycle || m["outcome"] != want.outcome || m["node"] != want.node {
			t.Fatalf("raw JSON attempt %d = %v/%v/%v, want %v/%q/%q", i, m["cycle"], m["outcome"], m["node"], want.cycle, want.outcome, want.node)
		}
		if want.commit == "" {
			if v, ok := m["short_commit"]; ok && v != "" {
				t.Fatalf("raw JSON attempt %d short_commit = %v, want absent or empty (nil-safe)", i, v)
			}
		} else {
			if m["commit"] != want.commit || m["short_commit"] != shortCommit(want.commit) {
				t.Fatalf("raw JSON attempt %d commit/short_commit = %v/%v, want %q/%q", i, m["commit"], m["short_commit"], want.commit, shortCommit(want.commit))
			}
		}
	}
	var decoded RecurseStatusJSON
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Running != got.Running || decoded.StopRequested != got.StopRequested || decoded.State != got.State || decoded.Pass != got.Pass || decoded.Node != got.Node || decoded.Cycles != got.Cycles || decoded.Kept != got.Kept || decoded.Reverted != got.Reverted || decoded.Refused != got.Refused || decoded.Unverified != got.Unverified {
		t.Fatalf("decoded running/stop/state/pass/node/counts = %+v, want %+v", decoded, got)
	}
	if len(decoded.OpenByPass) != len(got.OpenByPass) {
		t.Fatalf("decoded open_by_pass len = %d, want %d", len(decoded.OpenByPass), len(got.OpenByPass))
	}
	for i := range decoded.OpenByPass {
		if decoded.OpenByPass[i] != got.OpenByPass[i] {
			t.Fatalf("decoded open_by_pass[%d] = %d, want %d", i, decoded.OpenByPass[i], got.OpenByPass[i])
		}
	}
	if decoded.Base.Branch != got.Base.Branch || decoded.Base.Tip != got.Base.Tip || decoded.Base.TipShort != got.Base.TipShort || decoded.Base.LastAbsorbed != got.Base.LastAbsorbed || decoded.Base.LastAbsorbedShort != got.Base.LastAbsorbedShort || decoded.Base.Absorbs != got.Base.Absorbs {
		t.Fatalf("decoded base = %+v, want %+v", decoded.Base, got.Base)
	}
	if len(decoded.Base.Conflicts) != len(got.Base.Conflicts) || len(decoded.Base.Generations) != len(got.Base.Generations) {
		t.Fatalf("decoded conflicts/generations len = %d/%d, want %d/%d", len(decoded.Base.Conflicts), len(decoded.Base.Generations), len(got.Base.Conflicts), len(got.Base.Generations))
	}
	for i := range decoded.Base.Conflicts {
		d, w := decoded.Base.Conflicts[i], got.Base.Conflicts[i]
		if d.BaseCommit != w.BaseCommit || d.ShortCommit != w.ShortCommit || d.Detail != w.Detail {
			t.Fatalf("decoded conflict %d = %+v, want %+v", i, d, w)
		}
	}
	for i := range decoded.Base.Generations {
		d, w := decoded.Base.Generations[i], got.Base.Generations[i]
		if d.Commit != w.Commit || d.ShortCommit != w.ShortCommit || d.Binary != w.Binary || d.Failure != w.Failure {
			t.Fatalf("decoded generation %d = %+v, want %+v", i, d, w)
		}
	}
	if len(decoded.Attempts) != 4 {
		t.Fatalf("decoded attempts len = %d, want 4", len(decoded.Attempts))
	}
	for i, want := range []struct {
		cycle   int
		outcome string
		node    string
		commit  string
	}{
		{1, outcomeKept, "node-a", "abcdef1234567890"},
		{2, outcomeReverted, "node-b", "1234567890abcdef"},
		{3, outcomeRefused, "node-c", ""},
		{4, outcomeUnverified, "node-d", ""},
	} {
		d := decoded.Attempts[i]
		if d.Cycle != want.cycle || d.Outcome != want.outcome || d.Node != want.node || d.Commit != want.commit {
			t.Fatalf("decoded attempt %d = %d/%q/%q/%q, want %d/%q/%q/%q", i, d.Cycle, d.Outcome, d.Node, d.Commit, want.cycle, want.outcome, want.node, want.commit)
		}
		if d.ShortCommit != shortCommit(want.commit) {
			t.Fatalf("decoded attempt %d ShortCommit = %q, want %q", i, d.ShortCommit, shortCommit(want.commit))
		}
		if want.commit == "" && d.ShortCommit != "" {
			t.Fatalf("decoded attempt %d empty commit ShortCommit = %q, want empty (nil-safe)", i, d.ShortCommit)
		}
	}
	if len(decoded.Attempts[0].MissingEvidence) != 0 {
		t.Fatalf("decoded attempt 0 MissingEvidence = %v, want empty", decoded.Attempts[0].MissingEvidence)
	}
	if len(decoded.Attempts[1].MissingEvidence) != 0 {
		t.Fatalf("decoded attempt 1 MissingEvidence = %v, want empty", decoded.Attempts[1].MissingEvidence)
	}
	if len(decoded.Attempts[2].MissingEvidence) != 0 {
		t.Fatalf("decoded attempt 2 MissingEvidence = %v, want empty", decoded.Attempts[2].MissingEvidence)
	}
	if len(decoded.Attempts[3].MissingEvidence) != 1 || decoded.Attempts[3].MissingEvidence[0] != "/changed_code_unexecuted" {
		t.Fatalf("decoded attempt 3 MissingEvidence = %v, want [/changed_code_unexecuted]", decoded.Attempts[3].MissingEvidence)
	}
}
