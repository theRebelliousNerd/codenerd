package session

import (
	"testing"

	"codenerd/internal/types"
)

// The severity word as a severity_rank atom: the three the parser knows,
// case-insensitively, and /unknown for anything else, which ranks lowest.
func TestCriticSeverityAtom(t *testing.T) {
	cases := []struct {
		sev  string
		want types.MangleAtom
	}{
		{"high", "/high"},
		{"HIGH", "/high"},
		{"Medium", "/medium"},
		{"  medium  ", "/medium"},
		{"low", "/low"},
		{"Low", "/low"},
		{"critical", "/unknown"},
		{"warn", "/unknown"},
		{"P0", "/unknown"},
		{"", "/unknown"},
	}
	for _, tc := range cases {
		if got := criticSeverityAtom(tc.sev); got != tc.want {
			t.Errorf("criticSeverityAtom(%q) = %s, want %s", tc.sev, got, tc.want)
		}
	}
}

// Triage is derived (turn_needs_uplift): high and medium findings are worth
// an uplift round, in the reviewer's order; low and unknown severities are
// not. Pinned through a real kernel.
func TestActionableCriticFindings_Derived(t *testing.T) {
	cases := []struct {
		name     string
		findings []CriticFinding
		want     []CriticFinding
	}{
		{
			name: "high and medium kept in order",
			findings: []CriticFinding{
				{File: "z.go", Line: 9, Severity: "medium", Claim: "m"},
				{File: "a.go", Line: 1, Severity: "high", Claim: "h"},
				{File: "b.go", Line: 2, Severity: "low", Claim: "l"},
			},
			want: []CriticFinding{
				{File: "z.go", Line: 9, Severity: "medium", Claim: "m"},
				{File: "a.go", Line: 1, Severity: "high", Claim: "h"},
			},
		},
		{
			name: "severities compare case-insensitively",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "HIGH", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "Medium", Claim: "m1"},
				{File: "c.go", Line: 3, Severity: "LOW", Claim: "l1"},
			},
			want: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "HIGH", Claim: "h1"},
				{File: "b.go", Line: 2, Severity: "Medium", Claim: "m1"},
			},
		},
		{
			name: "unknown severity ignored",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "critical", Claim: "c1"},
				{File: "b.go", Line: 2, Severity: "high", Claim: "h1"},
			},
			want: []CriticFinding{
				{File: "b.go", Line: 2, Severity: "high", Claim: "h1"},
			},
		},
		{
			name: "low only is nothing worth acting on",
			findings: []CriticFinding{
				{File: "a.go", Line: 1, Severity: "low", Claim: "l1"},
				{File: "b.go", Line: 2, Severity: "low", Claim: "l2"},
			},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &Executor{kernel: realKernel(t)}
			got := e.actionableCriticFindings(types.MangleAtom("/turn_triage"), tc.findings)
			if len(got) != len(tc.want) {
				t.Fatalf("actionable = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("actionable[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
			if tc.want == nil && got != nil {
				t.Errorf("actionable = %v, want nil", got)
			}
		})
	}
}

// With no kernel there is no triage: the round is skipped, not decided in
// Go. The caller still records the review.
func TestActionableCriticFindings_NilKernelSkipsTheRound(t *testing.T) {
	var e *Executor
	got := e.actionableCriticFindings(types.MangleAtom("/turn_triage_nil"),
		[]CriticFinding{{File: "a.go", Line: 1, Severity: "high", Claim: "h"}})
	if got != nil {
		t.Fatalf("nil kernel triage = %v, want nil", got)
	}
}
