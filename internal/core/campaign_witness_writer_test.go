package core

import (
	"slices"
	"testing"

	"codenerd/internal/types"
)

// A /test_run certifies a snapshot of the whole workspace; a writer running
// beside it changed the snapshot and a live campaign retried its check 8
// times beside a file_modify task still writing. The kernel orders them.
func TestCampaignPolicy_WorkspaceWitnessNeverOverlapsAWriter(t *testing.T) {
	t.Parallel()
	type task struct{ id, status, kind string }
	cases := []struct {
		name  string
		tasks []task
		deps  [][2]string
		want  []string
	}{
		{"both pending: the writer goes first",
			[]task{{"t_check", "/pending", "/test_run"}, {"t_write", "/pending", "/file_modify"}}, nil,
			[]string{"t_write"}},
		{"writer running: the check waits",
			[]task{{"t_check", "/pending", "/test_run"}, {"t_write", "/in_progress", "/file_modify"}}, nil,
			nil},
		{"check running: a writer waits",
			[]task{{"t_check", "/in_progress", "/test_run"}, {"t_write", "/pending", "/test_write"}}, nil,
			nil},
		{"a writer that waits on the check does not hold it back",
			[]task{{"t_check", "/pending", "/test_run"}, {"t_fix", "/pending", "/file_modify"}},
			[][2]string{{"t_fix", "t_check"}},
			[]string{"t_check"}},
		{"nor one that waits on it through another task",
			[]task{{"t_check", "/pending", "/test_run"}, {"t_read", "/pending", "/research"}, {"t_fix", "/pending", "/refactor"}},
			[][2]string{{"t_fix", "t_read"}, {"t_read", "t_check"}},
			[]string{"t_check"}},
		{"a reader runs beside the check",
			[]task{{"t_check", "/pending", "/test_run"}, {"t_read", "/pending", "/research"}}, nil,
			[]string{"t_check", "t_read"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("the shipped corpus must load: %v", err)
			}
			facts := []types.Fact{
				{Predicate: "campaign", Args: []any{"c1", types.MangleAtom("/feature"), "campaign", "src", types.MangleAtom("/active")}},
				{Predicate: "campaign_phase", Args: []any{"p1", "c1", "build", 1, types.MangleAtom("/in_progress"), "ctx"}},
			}
			for _, tk := range tc.tasks {
				facts = append(facts, types.Fact{Predicate: "campaign_task", Args: []any{tk.id, "p1", tk.id, types.MangleAtom(tk.status), types.MangleAtom(tk.kind)}})
			}
			for _, d := range tc.deps {
				facts = append(facts, types.Fact{Predicate: "task_dependency", Args: []any{d[0], d[1]}})
			}
			for _, f := range facts {
				if aerr := k.Assert(f); aerr != nil {
					t.Fatalf("assert %s%v: %v", f.Predicate, f.Args, aerr)
				}
			}
			eligible, err := k.Query("eligible_task")
			if err != nil {
				t.Fatalf("query eligible_task: %v", err)
			}
			var got []string
			for _, f := range eligible {
				got = append(got, types.ExtractString(f.Args[0]))
			}
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("eligible_task = %v, want %v", got, tc.want)
			}
			blocked, err := k.Query("campaign_blocked")
			if err != nil {
				t.Fatalf("query campaign_blocked: %v", err)
			}
			if len(blocked) != 0 {
				t.Fatalf("campaign_blocked = %v: waiting on a running task is not a block", blocked)
			}
		})
	}
}
