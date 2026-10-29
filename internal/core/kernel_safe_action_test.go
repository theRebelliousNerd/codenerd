package core

import (
	"fmt"
	"slices"
	"testing"
)

// errKernel is a minimal types.Kernel-compatible stub whose Query always fails,
// used to verify the projection fails closed.
type errKernel struct {
	stubKernel
	err error
}

func (s *errKernel) Query(predicate string) ([]Fact, error) {
	return nil, s.err
}

func TestCanonicalizeSafeActionName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"slash_kept", "/read_file", "/read_file"},
		{"bare_gets_slash", "read_file", "/read_file"},
		{"blank_denies", "", ""},
		{"spaces_trimmed", "  read_file  ", "/read_file"},
		{"slash_with_spaces", "  /read_file  ", "/read_file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanonicalizeSafeActionName(tt.input); got != tt.want {
				t.Errorf("CanonicalizeSafeActionName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProjectSafeActions(t *testing.T) {
	tests := []struct {
		name      string
		safe      []Fact
		wantBoth  []string // actions that must resolve under both spellings
		wantEmpty bool
	}{
		{
			name: "slash_fact_resolves_both_spellings",
			safe: []Fact{{Predicate: "safe_action", Args: []any{"/read_file"}}},
			wantBoth: []string{
				"/read_file", "read_file",
			},
		},
		{
			name: "bare_fact_resolves_both_spellings",
			safe: []Fact{{Predicate: "safe_action", Args: []any{"write_file"}}},
			wantBoth: []string{
				"/write_file", "write_file",
			},
		},
		{
			name: "zero_arg_and_blank_rows_skipped",
			safe: []Fact{
				{Predicate: "safe_action", Args: []any{}},
				{Predicate: "safe_action", Args: []any{""}},
				{Predicate: "safe_action", Args: []any{"   "}},
				{Predicate: "safe_action", Args: []any{"/run_tests"}},
			},
			wantBoth: []string{"/run_tests", "run_tests"},
		},
		{
			name:      "empty_results_yield_empty_set",
			safe:      nil,
			wantEmpty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &stubKernel{safe: tt.safe}
			set, err := ProjectSafeActions(k)
			if err != nil {
				t.Fatalf("ProjectSafeActions error = %v", err)
			}
			if set == nil {
				t.Fatalf("ProjectSafeActions returned nil map")
			}
			if tt.wantEmpty && len(set) != 0 {
				t.Fatalf("expected empty set, got %v", set)
			}
			for _, action := range tt.wantBoth {
				if !LookupSafeAction(set, action) {
					t.Errorf("LookupSafeAction(set, %q) = false, want true (set=%v)", action, set)
				}
			}
		})
	}
}

func TestProjectSafeActions_FailClosed(t *testing.T) {
	t.Run("nil_kernel_returns_error", func(t *testing.T) {
		if _, err := ProjectSafeActions(nil); err == nil {
			t.Error("expected error for nil kernel, got nil")
		}
		if _, err := ListSafeActions(nil); err == nil {
			t.Error("expected error for nil kernel, got nil")
		}
	})
	t.Run("query_error_propagates", func(t *testing.T) {
		k := &errKernel{err: fmt.Errorf("boom")}
		if _, err := ProjectSafeActions(k); err == nil {
			t.Error("expected query error to propagate, got nil")
		}
		if _, err := ListSafeActions(k); err == nil {
			t.Error("expected query error to propagate, got nil")
		}
	})
}

func TestLookupSafeAction(t *testing.T) {
	set := map[string]bool{"/read_file": true, "read_file": true}
	tests := []struct {
		name   string
		set    map[string]bool
		action string
		want   bool
	}{
		{"slash_hit", set, "/read_file", true},
		{"bare_hit", set, "read_file", true},
		{"miss_denies", set, "/exec_cmd", false},
		{"nil_set_denies", nil, "/read_file", false},
		{"empty_set_denies", map[string]bool{}, "/read_file", false},
		{"blank_action_denies", set, "", false},
		{"spaces_only_denies", set, "   ", false},
		{"canonical_only_set_matches_bare", map[string]bool{"/grep": true}, "grep", true},
		{"bare_only_set_matches_slash", map[string]bool{"grep": true}, "/grep", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LookupSafeAction(tt.set, tt.action); got != tt.want {
				t.Errorf("LookupSafeAction(set, %q) = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestListSafeActions_SortedCanonicalDeduped(t *testing.T) {
	k := &stubKernel{safe: []Fact{
		{Predicate: "safe_action", Args: []any{"write_file"}},
		{Predicate: "safe_action", Args: []any{"/read_file"}},
		{Predicate: "safe_action", Args: []any{"/read_file"}},
		{Predicate: "safe_action", Args: []any{}},
		{Predicate: "safe_action", Args: []any{""}},
	}}
	got, err := ListSafeActions(k)
	if err != nil {
		t.Fatalf("ListSafeActions error = %v", err)
	}
	want := []string{"/read_file", "/write_file"}
	if !slices.Equal(got, want) {
		t.Errorf("ListSafeActions = %v, want %v", got, want)
	}
}

func TestSafeActionProjection_LiveKernel(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel error = %v", err)
	}
	set, err := ProjectSafeActions(k)
	if err != nil {
		t.Fatalf("ProjectSafeActions error = %v", err)
	}
	// The constitution must classify the core file read as safe; anything
	// else means policy failed to load, not that the projection is wrong.
	for _, action := range []string{"/read_file", "read_file"} {
		if !LookupSafeAction(set, action) {
			t.Errorf("live kernel missing %q from safe_action projection", action)
		}
	}
	if got := k.IsSafeAction("/read_file"); !got {
		t.Error("RealKernel.IsSafeAction(/read_file) = false, want true")
	}
	if got := k.IsSafeAction("definitely_not_a_real_action_xyz"); got {
		t.Error("RealKernel.IsSafeAction(unknown) = true, want false (fail closed)")
	}
	names, err := ListSafeActions(k)
	if err != nil {
		t.Fatalf("ListSafeActions error = %v", err)
	}
	if !slices.Contains(names, "/read_file") {
		t.Errorf("ListSafeActions missing /read_file: %v", names)
	}
	if !slices.IsSorted(names) {
		t.Errorf("ListSafeActions not sorted: %v", names)
	}
}
