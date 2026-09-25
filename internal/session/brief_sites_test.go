package session

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
)

func TestBriefSites(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "main.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(ws), "elsewhere", "x.go")
	cases := []struct {
		name  string
		brief string
		want  []briefSite
	}{
		{"a path with a line", "fix the nil check in internal/x/y.go:42", []briefSite{{"internal/x/y.go", 42}}},
		{"one file at two lines is two sites", "internal/x/y.go:42 and internal/x/y.go:90 both drop the error",
			[]briefSite{{"internal/x/y.go", 42}, {"internal/x/y.go", 90}}},
		{"a repeated site counts once", "see internal/x/y.go, then edit internal/x/y.go", []briefSite{{"internal/x/y.go", 0}}},
		{"a bare file that exists", "main.go panics at startup", []briefSite{{"main.go", 0}}},
		{"a bare name that is not a file", "a.txt should say hello", nil},
		{"qualified identifiers are not files", "replace fmt.Errorf with errors.New and e.kernel with k", nil},
		{"a URL is not a workspace file", "see https://example.com/docs/page.html", nil},
		{"a sentence-final period is not part of the path", "change cmd/nerd/main.go.", []briefSite{{"cmd/nerd/main.go", 0}}},
		{"an absolute path inside the workspace", "edit " + filepath.Join(ws, "internal", "a.go") + ":7", []briefSite{{"internal/a.go", 7}}},
		{"an absolute path outside the workspace", "edit " + outside, nil},
		{"backslashes are directories too", `edit internal\session\executor.go`, []briefSite{{"internal/session/executor.go", 0}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := briefSites(ws, tc.brief); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("briefSites(%q) = %+v, want %+v", tc.brief, got, tc.want)
			}
		})
	}
}

// countPlanningCalls runs one write-oriented turn's planning decision with a
// write tool in the catalog and reports how many planning calls it made.
func countPlanningCalls(t *testing.T, e *Executor, client *stepScriptProvider, writeTool, brief string) int {
	t.Helper()
	calls := 0
	client.MockLLMClient.CompleteWithSystemFunc = func(context.Context, string, string) (string, error) {
		calls++
		return client.plan, nil
	}
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}}
	e.planTurnSteps(context.Background(), client, brief, &config.EffectiveAgentRuntimeConfig{AllowedTools: []string{writeTool}}, result)
	e.cleanupTurnFacts()
	return calls
}

// The sweep's proof for F9: a brief that names one site runs as one pass and
// never pays for a planning call. Measured 2026-09-21: 13 planning calls in
// one campaign run, every one on such a brief, every one answered "one step".
func TestPlanTurnSteps_AOneSiteBriefMakesNoPlanningCall(t *testing.T) {
	_, writeTool := registerStepTools(t)
	client := newStepScriptProvider("STEP internal/x/y.go :: fix it", nil)
	e := newPlannedStepsExecutor(t, client)
	if n := countPlanningCalls(t, e, client, writeTool, "fix the nil check in internal/x/y.go:42"); n != 0 {
		t.Fatalf("a one-site brief made %d planning call(s); want none", n)
	}
	if n := countPlanningCalls(t, e, client, writeTool, "fix the nil checks in internal/x/y.go:42 and internal/x/z.go:7"); n != 1 {
		t.Fatalf("a two-site brief made %d planning call(s); want one", n)
	}
}

// How many sites a brief needs is the user's (session.step_plan_min_sites),
// read by the policy as config_param, not a number in Go.
func TestPlanTurnSteps_TheSiteThresholdIsTheUsersConfig(t *testing.T) {
	_, writeTool := registerStepTools(t)
	client := newStepScriptProvider("STEP a/1.go :: x", nil)
	e := newPlannedStepsExecutor(t, client)
	e.config.StepPlanMinSites = 3
	two := "change a/1.go and a/2.go"
	three := "change a/1.go, a/2.go and a/3.go"
	if n := countPlanningCalls(t, e, client, writeTool, two); n != 0 {
		t.Fatalf("with step_plan_min_sites=3 a two-site brief made %d planning call(s); want none", n)
	}
	if n := countPlanningCalls(t, e, client, writeTool, three); n != 1 {
		t.Fatalf("with step_plan_min_sites=3 a three-site brief made %d planning call(s); want one", n)
	}
	// The kernel follows the executor's config when it changes: the row is
	// replaced, not left at the first value.
	e.config.StepPlanMinSites = 2
	if n := countPlanningCalls(t, e, client, writeTool, two); n != 1 {
		t.Fatalf("after step_plan_min_sites=2 a two-site brief made %d planning call(s); want one", n)
	}
}

// A Windows 8.3 short name (RUNNER~1) is one path, not two fragments split at
// the tilde. CI's path-alias run puts every temp dir under one, and there an
// absolute path inside the workspace came back as a cut-off relative site and
// one outside the workspace as a site inside it.
func TestBriefSites_AShortNameIsOnePath(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "RUNNER~1", "ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	inside := "edit " + filepath.Join(ws, "internal", "a.go") + ":7"
	if got, want := briefSites(ws, inside), []briefSite{{"internal/a.go", 7}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("briefSites(%q) = %+v, want %+v", inside, got, want)
	}
	outside := "edit " + filepath.Join(filepath.Dir(ws), "elsewhere", "x.go")
	if got := briefSites(ws, outside); got != nil {
		t.Fatalf("briefSites(%q) = %+v, want none", outside, got)
	}
}
