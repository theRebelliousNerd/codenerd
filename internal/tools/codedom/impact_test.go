package codedom

import (
	"context"
	"testing"
)

// =============================================================================
// MOCKS
// =============================================================================

type mockKernel struct{}

func (m *mockKernel) Query(predicate string) ([]FactData, error) {
	return nil, nil // Return empty facts for now
}

type mockImpactProvider struct {
	analyzer *mockAnalyzer
}

func (m *mockImpactProvider) GetKernel() KernelQuerier {
	return &mockKernel{}
}

func (m *mockImpactProvider) GetProjectRoot() string {
	return "/tmp/project"
}

func (m *mockImpactProvider) NewTestDependencyAnalyzer() TestDependencyAnalyzer {
	return m.analyzer
}

type mockAnalyzer struct {
	impactedTests []ImpactedTestInfo
	buildErr      error
}

func (m *mockAnalyzer) Build(ctx context.Context) error {
	return m.buildErr
}

func (m *mockAnalyzer) GetImpactedTests(editedRefs []string) []ImpactedTestInfo {
	return m.impactedTests
}

func (m *mockAnalyzer) GetImpactedTestPackages(editedRefs []string) []string {
	pkgs := make(map[string]bool)
	for _, _ = range m.impactedTests {
		pkgs["codenerd/pkg"] = true
	}
	var res []string
	for p := range pkgs {
		res = append(res, p)
	}
	return res
}

func (m *mockAnalyzer) GetCoverageGaps() []string {
	return []string{}
}

// =============================================================================
// TOOL DEFINITION TESTS
// =============================================================================

func TestRunImpactedTestsTool_Definition(t *testing.T) {
	t.Parallel()
	tool := RunImpactedTestsTool()
	if tool.Name != "run_impacted_tests" {
		t.Errorf("Name mismatch: got %q, want run_impacted_tests", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Description should not be empty")
	}
}

func TestGetImpactedTestsTool_Definition(t *testing.T) {
	t.Parallel()
	tool := GetImpactedTestsTool()
	if tool.Name != "get_impacted_tests" {
		t.Errorf("Name mismatch: got %q, want get_impacted_tests", tool.Name)
	}
}

// =============================================================================
// LOGIC TESTS (Table-Driven)
// =============================================================================

func TestFilterByPriority(t *testing.T) {
	t.Parallel()

	input := []ImpactedTestInfo{
		{TestRef: "A", Priority: "high"},
		{TestRef: "B", Priority: "medium"},
		{TestRef: "C", Priority: "low"},
		{TestRef: "D", Priority: "high"},
	}

	cases := []struct {
		name     string
		priority string
		want     int // count
	}{
		{"high_only", "high", 2},
		{"medium_only", "medium", 1},
		{"low_only", "low", 1},
		{"unknown", "critical", 0},
		{"empty", "", 0},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := filterByPriority(input, tc.priority)
			if len(got) != tc.want {
				t.Errorf("filterByPriority(%q) count = %d, want %d", tc.priority, len(got), tc.want)
			}
		})
	}
}

func TestExecuteGetImpactedTests(t *testing.T) {
	// Not safe to run in parallel due to global state registration

	cases := []struct {
		name        string
		args        map[string]any
		mockImpact  []ImpactedTestInfo
		mockErr     error
		wantErr     bool
		wantContain string
	}{
		{
			name:        "missing_edited_refs",
			args:        map[string]any{},
			wantContain: "[]", // Expect empty success
		},
		{
			name:        "success_no_impact",
			args:        map[string]any{"edited_refs": []string{"foo.go"}},
			mockImpact:  []ImpactedTestInfo{},
			wantContain: "[]",
		},
		{
			name: "success_with_impact",
			args: map[string]any{"edited_refs": []string{"foo.go"}},
			mockImpact: []ImpactedTestInfo{
				{TestRef: "TestFoo", Priority: "high"},
			},
			wantContain: "TestFoo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := &mockAnalyzer{
				impactedTests: tc.mockImpact,
				buildErr:      tc.mockErr,
			}
			provider := &mockImpactProvider{analyzer: analyzer}

			oldProvider := globalTestProvider
			RegisterTestImpactProvider(provider)
			defer RegisterTestImpactProvider(oldProvider)

			out, err := executeGetImpactedTests(context.Background(), tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.wantContain != "" && out == "" {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestExecuteRunImpactedTests(t *testing.T) {
	// Table-driven for run execution paths
	cases := []struct {
		name       string
		args       map[string]any
		mockImpact []ImpactedTestInfo
		wantErr    bool
		wantResult string
	}{
		{
			name:       "no_impacted_tests",
			args:       map[string]any{"edited_refs": []string{"foo.go"}},
			mockImpact: []ImpactedTestInfo{},
			wantResult: "No impacted tests found for the edited code.",
		},
		{
			name:       "missing_refs_error",
			args:       map[string]any{},
			wantResult: "No edited refs specified and no plan_edit facts found in kernel.",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			analyzer := &mockAnalyzer{impactedTests: tc.mockImpact}
			provider := &mockImpactProvider{analyzer: analyzer}

			oldProvider := globalTestProvider
			RegisterTestImpactProvider(provider)
			defer RegisterTestImpactProvider(oldProvider)

			res, err := executeRunImpactedTests(context.Background(), tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res != tc.wantResult {
				t.Errorf("got %q, want %q", res, tc.wantResult)
			}
		})
	}
}
