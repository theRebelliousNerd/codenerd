package campaign

import "testing"

func TestToRecurseStatusJSON_OpenByPassRetentionPin(t *testing.T) {
	s := RecurseStatus{OpenByPass: []int{3, 1}}
	out := s.ToRecurseStatusJSON()
	if len(out.OpenByPass) != 2 || out.OpenByPass[0] != 3 || out.OpenByPass[1] != 1 {
		t.Fatalf("ToRecurseStatusJSON OpenByPass = %v, want [3 1]", out.OpenByPass)
	}
	s.OpenByPass[0] = 99
	if out.OpenByPass[0] != 3 {
		t.Fatalf("ToRecurseStatusJSON aliased OpenByPass: got %v after source mutation, want [3 1]", out.OpenByPass)
	}
}

func TestToRecurseStatusJSON_TextParityPin(t *testing.T) {
	s := RecurseStatus{
		Running: true, StopRequested: true, Pass: 3, Node: "lib",
		Cycles: 4, Kept: 1, Reverted: 1, Refused: 1, Unverified: 1,
		OpenByPass: []int{3, 1},
		Base: RecurseBaseStatus{
			Branch: "main", TipAtStart: "abcdef1234567890", LastAbsorbed: "1234567890abcdef", Absorbs: 2,
			Conflicts:   []RecurseConflict{{BaseCommit: "deadbeef1234567890", Detail: "merge conflict"}},
			Generations: []RecurseGeneration{{Commit: "cafef00d1234567890", Binary: ".nerd/bin/nerd", Failure: ""}, {Commit: "badc0de1234567890", Failure: "build failed"}},
		},
		ThisPass: []recurseRecord{
			{Cycle: 1, Outcome: "/kept", Node: "lib", Commit: "abcdef1234567890", Signature: "sig", Finding: "f", Why: "", Detail: ""},
			{Cycle: 2, Outcome: "/unverified", Node: "lib", Commit: "", Why: "/changed_code_unexecuted", Detail: "needs /check_not_green and /tests_not_green"},
			{Cycle: 3, Outcome: "/reverted", Node: "api", Commit: "123", Why: "other", Detail: "other"},
		},
	}
	out := s.ToRecurseStatusJSON()
	if !out.Running || !out.StopRequested {
		t.Fatalf("state flags = %v/%v, want true/true", out.Running, out.StopRequested)
	}
	if out.State != "running, stop requested" {
		t.Fatalf("State = %q, want %q", out.State, "running, stop requested")
	}
	if out.Pass != 3 || out.Cycles != 4 || out.Kept != 1 || out.Reverted != 1 || out.Refused != 1 || out.Unverified != 1 {
		t.Fatalf("counts = pass %d cycles %d kept %d reverted %d refused %d unverified %d", out.Pass, out.Cycles, out.Kept, out.Reverted, out.Refused, out.Unverified)
	}
	if out.Node != "lib" {
		t.Fatalf("Node = %q, want lib", out.Node)
	}
	if out.Base.Branch != "main" || out.Base.Tip != "abcdef1234567890" || out.Base.TipShort != shortCommit("abcdef1234567890") {
		t.Fatalf("base tip = %+v", out.Base)
	}
	if out.Base.LastAbsorbed != "1234567890abcdef" || out.Base.LastAbsorbedShort != shortCommit("1234567890abcdef") || out.Base.Absorbs != 2 {
		t.Fatalf("base absorbed = %+v", out.Base)
	}
	if len(out.Base.Conflicts) != 1 || out.Base.Conflicts[0].ShortCommit != shortCommit("deadbeef1234567890") {
		t.Fatalf("conflicts = %+v", out.Base.Conflicts)
	}
	if len(out.Base.Generations) != 2 || out.Base.Generations[0].ShortCommit != shortCommit("cafef00d1234567890") {
		t.Fatalf("generations = %+v", out.Base.Generations)
	}
	if len(out.Attempts) != 3 {
		t.Fatalf("attempts len = %d, want 3", len(out.Attempts))
	}
	if out.Attempts[0].ShortCommit != shortCommit("abcdef1234567890") || out.Attempts[0].Cycle != 1 || out.Attempts[0].Outcome != "/kept" {
		t.Fatalf("attempt 0 = %+v", out.Attempts[0])
	}
	if out.Attempts[1].Commit != "" || out.Attempts[1].ShortCommit != "" {
		t.Fatalf("empty commit shortCommit = %q, want empty (nil-safe)", out.Attempts[1].ShortCommit)
	}
	got := out.Attempts[1].MissingEvidence
	for _, want := range []string{"/changed_code_unexecuted", "/check_not_green", "/tests_not_green"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing evidence = %v, want %q", got, want)
		}
	}
	if len(out.Attempts[0].MissingEvidence) != 0 {
		t.Fatalf("empty why/detail evidence = %v, want empty", out.Attempts[0].MissingEvidence)
	}
	if len(out.Attempts[2].MissingEvidence) != 0 {
		t.Fatalf("unrelated why/detail evidence = %v, want empty", out.Attempts[2].MissingEvidence)
	}
	if _, err := s.MarshalRecurseStatusJSON(); err != nil {
		t.Fatalf("MarshalRecurseStatusJSON: %v", err)
	}
}
