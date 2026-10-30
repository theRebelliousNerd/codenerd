package northstar

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

type scopeCaptureClient struct {
	systems, users []string
	response       string
}

func (c *scopeCaptureClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	c.systems = append(c.systems, system)
	c.users = append(c.users, user)
	if c.response != "" {
		return c.response, nil
	}
	return "SCORE: 0.95\nRESULT: passed\nEXPLANATION: assessed", nil
}

type scopeSnapshot struct {
	fakeQuerier
	err   error
	reads int
}

func (q *scopeSnapshot) QueryAll() (map[string][]types.Fact, error) {
	q.reads++
	if q.err != nil {
		return nil, q.err
	}
	return q.fakeQuerier.QueryAll()
}

func newScopedGuardian(t *testing.T) (*Guardian, *scopeCaptureClient, *scopeSnapshot, string) {
	t.Helper()
	workspace := t.TempDir()
	store, err := NewStore(filepath.Join(workspace, ".nerd"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	g := NewGuardian(store, DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(&Vision{Mission: "Project mission", Constraints: []string{"Project constraint"}}); err != nil {
		t.Fatal(err)
	}
	client := &scopeCaptureClient{}
	g.SetLLMClient(client)
	q := &scopeSnapshot{fakeQuerier: fakeQuerier{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"pkg/a", "Purpose A"}},
			{Predicate: "effective_module_purpose", Args: []any{"pkg/a/nested", "Nested purpose"}},
			{Predicate: "effective_module_purpose", Args: []any{"pkg/b", "Purpose B"}},
		},
		"module_requirement": {
			{Predicate: "module_requirement", Args: []any{"pkg/a", "REQ-1", "Requirement A", types.MangleAtom("/blocker")}},
			{Predicate: "module_requirement", Args: []any{"pkg/b", "REQ-1", "Requirement B", types.MangleAtom("/major")}},
		},
	}}}
	g.SetQuerier(q)
	return g, client, q, workspace
}

func scopedPaths(workspace string, paths ...string) AlignmentScope {
	scope := AlignmentScope{Workspace: workspace, Disposition: ScopePaths}
	for _, p := range paths {
		scope.Paths = append(scope.Paths, AlignmentPath{Path: p, Role: PathPlannedWrite})
	}
	return scope
}

// Captures the actual messages passed to the injected completion client. The
// old observer sent prose to ModuleForPath and omitted every module below.
func TestCampaignObserver_ScopedProseReachesFinalRequestAndHistory(t *testing.T) {
	g, client, q, workspace := newScopedGuardian(t)
	o := NewCampaignObserver(g)
	check, err := o.StartCampaign(context.Background(), "campaign", "Repair cancellation behavior", scopedPaths(workspace,
		filepath.Join(workspace, "pkg/a/file.go"), `pkg\b\file.go`, "./pkg/a/file.go", "pkg/a/nested/x.go", "pkg/a2/x.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !check.Result.Assessed() || check.Coverage == nil || check.Coverage.Outcome != ScopeResolved || !check.Coverage.Complete {
		t.Fatalf("check = %+v", check)
	}
	if q.reads != 1 {
		t.Fatalf("snapshot reads = %d, want one per check", q.reads)
	}
	if len(client.users) != 1 {
		t.Fatalf("completions = %d", len(client.users))
	}
	for _, want := range []string{"Project mission", "Project constraint", "Purpose A", "Purpose B", "Nested purpose", "pkg/a/REQ-1/blocker", "pkg/b/REQ-1/major", "Repair cancellation behavior", "[planned_write] pkg/b/file.go"} {
		if !strings.Contains(client.users[0], want) {
			t.Errorf("final request lacks %q", want)
		}
	}
	if strings.Count(client.users[0], "## Module Northstar (pkg/a)") != 1 {
		t.Fatal("module not deduplicated")
	}
	if !strings.Contains(client.systems[0], "the module\nREFINES the project vision") {
		t.Fatal("module JIT atom missing")
	}
	if !reflect.DeepEqual(check.Coverage.Modules, []string{"pkg/a", "pkg/a/nested", "pkg/b"}) {
		t.Fatalf("modules = %v", check.Coverage.Modules)
	}
	if !reflect.DeepEqual(check.Coverage.UnmatchedPaths, []string{"pkg/a2/x.go"}) {
		t.Fatalf("unmatched = %v", check.Coverage.UnmatchedPaths)
	}
	history, err := g.store.GetAlignmentHistory(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || !reflect.DeepEqual(history[0].Coverage, check.Coverage) {
		t.Fatalf("coverage not retained: %+v", history)
	}
}

func TestGuardian_NestedFileSelectsOnlyMostSpecificModule(t *testing.T) {
	g, client, _, workspace := newScopedGuardian(t)
	check, err := g.CheckAlignment(context.Background(), TriggerManual, "Inspect nested module", "", scopedPaths(workspace, "pkg/a/nested/x.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(check.Coverage.Modules, []string{"pkg/a/nested"}) {
		t.Fatalf("modules = %v", check.Coverage.Modules)
	}
	if len(client.users) != 1 || strings.Contains(client.users[0], "Requirement A") {
		t.Fatal("parent requirement replaced most-specific selection")
	}
}

func TestGuardian_IncompleteScopePreservesProjectVerdictAndUnknownCoverage(t *testing.T) {
	for _, scenario := range []string{"snapshot failure", "absent querier", "query-only querier", "missing effective purpose", "missing predicate", "no module facts", "unmatched path", "malformed requirement", "conflicting requirement", "outside absolute", "outside relative", "glob", "ambiguous whitespace", "unknown role", "wrong workspace", "empty scoped paths", "unresolved", "project-only with paths"} {
		t.Run(scenario, func(t *testing.T) {
			for _, verdict := range []struct {
				result   AlignmentResult
				score    float64
				response string
			}{
				{AlignmentPassed, 0.95, "SCORE: 0.95\nRESULT: passed\nEXPLANATION: project verdict\nSUGGESTIONS: none"},
				{AlignmentBlocked, 0.1, "SCORE: 0.1\nRESULT: blocked\nEXPLANATION: project verdict\nSUGGESTIONS: rescope"},
			} {
				t.Run(string(verdict.result), func(t *testing.T) {
					g, client, q, workspace := newScopedGuardian(t)
					client.response = verdict.response
					scope := scopedPaths(workspace, "pkg/a/file.go")
					switch scenario {
					case "snapshot failure":
						q.err = errors.New("kernel unavailable")
					case "absent querier":
						g.SetQuerier(nil)
					case "query-only querier":
						g.SetQuerier(&queryOnlyScopeQuerier{})
					case "missing effective purpose":
						q.facts["effective_module_purpose"] = nil
					case "missing predicate":
						g.SetQuerier(&missingScopePredicate{})
					case "no module facts":
						q.facts = nil
					case "unmatched path":
						scope.Paths[0].Path = "pkg/other/file.go"
					case "malformed requirement":
						q.facts["module_requirement"][0].Args = []any{"pkg/a", "REQ-1"}
					case "conflicting requirement":
						q.facts["module_requirement"] = append(q.facts["module_requirement"], types.Fact{Predicate: "module_requirement", Args: []any{"pkg/a", "REQ-1", "Contradiction", "/blocker"}})
					case "outside absolute":
						scope.Paths[0].Path = filepath.Join(filepath.Dir(workspace), "outside", "pkg/a/file.go")
					case "outside relative":
						scope.Paths[0].Path = "../pkg/a/file.go"
					case "glob":
						scope.Paths[0].Path = "pkg/*/*.go"
					case "ambiguous whitespace":
						scope.Paths[0].Path = " pkg/a/file.go"
					case "unknown role":
						scope.Paths[0].Role = "unknown"
					case "wrong workspace":
						scope.Workspace = t.TempDir()
					case "empty scoped paths":
						scope.Paths = nil
					case "unresolved":
						scope.Disposition = ScopeUnresolved
					case "project-only with paths":
						scope.Disposition = ScopeProjectOnly
					}
					check, err := g.CheckAlignment(context.Background(), TriggerCampaignStart, "Repair cancellation", "", scope)
					if err != nil {
						t.Fatal(err)
					}
					if check.Result != verdict.result || check.Score != verdict.score || check.Explanation != "project verdict" {
						t.Fatalf("scope replaced project verdict: %+v", check)
					}
					if check.Coverage == nil || check.Coverage.Outcome != ScopeUnknown || check.Coverage.Complete || len(check.Coverage.Issues) == 0 {
						t.Fatalf("unknown scope concealed: %+v", check.Coverage)
					}
					if check.Coverage.Scope.Disposition != scope.Disposition {
						t.Fatalf("fallback relabeled requested scope: %+v", check.Coverage.Scope)
					}
					if len(client.users) != 1 || !strings.Contains(client.users[0], "Project mission") || !strings.Contains(client.users[0], "Project constraint") || !strings.Contains(client.users[0], "Outcome: unknown") {
						t.Fatalf("project check did not run: %v", client.users)
					}
					if strings.Contains(client.users[0], "## Module Northstar") || strings.Contains(client.systems[0], "the module\nREFINES the project vision") || len(check.Coverage.Modules) != 0 || check.Coverage.ModuleDigest != "" {
						t.Fatal("unresolved scope supplied module refinements")
					}
					for _, issue := range check.Coverage.Issues {
						if !strings.Contains(client.users[0], issue) {
							t.Errorf("request lost scope issue %q", issue)
						}
					}
					history, err := g.store.GetAlignmentHistory(1)
					if err != nil || len(history) != 1 || history[0].Result != verdict.result || !reflect.DeepEqual(history[0].Coverage, check.Coverage) {
						t.Fatalf("project verdict and scope outcome not retained: %v, %+v", err, history)
					}
				})
			}
		})
	}
}

type queryOnlyScopeQuerier struct{}

func (*queryOnlyScopeQuerier) Query(string) ([]types.Fact, error) { return nil, nil }

type missingScopePredicate struct{ queryOnlyScopeQuerier }

func (*missingScopePredicate) QueryAll() (map[string][]types.Fact, error) {
	return map[string][]types.Fact{}, nil
}

func TestGuardian_ManualAndBackgroundScopes(t *testing.T) {
	g, client, _, workspace := newScopedGuardian(t)
	if err := os.MkdirAll(filepath.Join(workspace, "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"pkg/a/file.go", "pkg", "Explain cancellation in pkg/a/file.go"} {
		check, err := g.CheckAlignment(context.Background(), TriggerManual, subject, "", ManualAlignmentScope(workspace, subject))
		if err != nil || !check.Result.Assessed() {
			t.Fatalf("manual: %+v, %v", check, err)
		}
		wantModule := subject == "pkg/a/file.go" || subject == "pkg"
		if (len(check.Coverage.Modules) > 0) != wantModule {
			t.Fatalf("manual scope = %+v", check.Coverage)
		}
	}
	h := NewBackgroundEventHandler(g, "session")
	assessment, err := h.HandleEvent(context.Background(), "file_modified", "editor", "pkg/b/file.go", nil, time.Now())
	if err != nil || assessment == nil || assessment.Level != "proceed" {
		t.Fatalf("background: %+v, %v", assessment, err)
	}
	if !strings.Contains(client.users[len(client.users)-1], "Purpose B") || !strings.Contains(client.users[len(client.users)-1], "[observed_effect] pkg/b/file.go") {
		t.Fatal("background target lost to display prose")
	}
	assessment, err = h.HandleEvent(context.Background(), "task_completed", "worker", "task-1", nil, time.Now())
	if err != nil || assessment == nil || assessment.Level != "proceed" || assessment.Metadata["scope_outcome"] != "unknown" || assessment.Metadata["scope_reason"] == "" {
		t.Fatalf("unscoped background: %+v, %v", assessment, err)
	}
}

func TestBackgroundEventHandler_UnknownScopeRetainsVerdictAndReason(t *testing.T) {
	for _, scenario := range []string{"no affected paths", "no snapshot", "no module facts"} {
		t.Run(scenario, func(t *testing.T) {
			for _, verdict := range []struct {
				result   AlignmentResult
				level    string
				score    int
				response string
			}{
				{AlignmentPassed, "proceed", 95, "SCORE: 0.95\nRESULT: passed\nEXPLANATION: on mission\nSUGGESTIONS: none"},
				{AlignmentBlocked, "block", 10, "SCORE: 0.1\nRESULT: blocked\nEXPLANATION: contradicts the mission\nSUGGESTIONS: rescope"},
			} {
				t.Run(string(verdict.result), func(t *testing.T) {
					g, client, q, _ := newScopedGuardian(t)
					client.response = verdict.response
					eventType := "file_modified"
					switch scenario {
					case "no affected paths":
						eventType = "task_completed"
					case "no snapshot":
						g.SetQuerier(nil)
					case "no module facts":
						q.facts = nil
					}
					h := NewBackgroundEventHandler(g, "session")
					assessment, err := h.HandleEvent(context.Background(), eventType, "worker", "pkg/a/file.go", nil, time.Now())
					if err != nil || assessment == nil {
						t.Fatalf("background check: %+v, %v", assessment, err)
					}
					if assessment.Level != verdict.level || assessment.Score != verdict.score || assessment.Metadata["result"] != string(verdict.result) {
						t.Fatalf("scope replaced background verdict: %+v", assessment)
					}
					if assessment.Metadata["scope_outcome"] != "unknown" || assessment.Metadata["scope_complete"] != "false" || assessment.Metadata["scope_reason"] == "" {
						t.Fatalf("scope outcome or reason hidden: %+v", assessment.Metadata)
					}
					history, err := g.store.GetAlignmentHistory(1)
					if err != nil || len(history) != 1 || history[0].Coverage == nil {
						t.Fatalf("background coverage not recorded: %+v, %v", history, err)
					}
					if history[0].Coverage.Outcome != ScopeUnknown || strings.Join(history[0].Coverage.Issues, "; ") != assessment.Metadata["scope_reason"] {
						t.Fatalf("assessment and history disagree: %+v, %+v", assessment, history[0])
					}
				})
			}
		})
	}
}

func TestCampaignObserver_FirstPhaseCoverageAndExpansion(t *testing.T) {
	for _, scenario := range []string{"covered", "expanded", "module changed", "vision changed", "start scope unknown"} {
		t.Run(scenario, func(t *testing.T) {
			g, client, q, workspace := newScopedGuardian(t)
			o := NewCampaignObserver(g)
			scope := scopedPaths(workspace, "pkg/a/file.go")
			if scenario == "start scope unknown" {
				g.SetQuerier(nil)
			}
			if _, err := o.StartCampaign(context.Background(), "c", "Repair cancellation", scope); err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "expanded":
				scope = scopedPaths(workspace, "pkg/b/file.go")
			case "module changed":
				q.facts["effective_module_purpose"][0].Args[1] = "Revised purpose"
			case "vision changed":
				if err := g.UpdateVision(&Vision{Mission: "Changed mission"}); err != nil {
					t.Fatal(err)
				}
			}
			check, err := o.OnPhaseStart(context.Background(), "phase", "Repair cancellation", scope)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "covered" {
				if check != nil || len(client.users) != 1 {
					t.Fatal("covered first phase rechecked")
				}
			} else if check == nil || len(client.users) != 2 {
				t.Fatal("new scope or context reused stale coverage")
			}
		})
	}
}

func TestCampaignObserver_PeriodicAndHighImpactCarryTaskScope(t *testing.T) {
	for _, periodic := range []bool{true, false} {
		t.Run(map[bool]string{true: "periodic", false: "high impact"}[periodic], func(t *testing.T) {
			g, client, _, workspace := newScopedGuardian(t)
			g.config.HighImpactPaths = []string{"pkg/a/"}
			o := NewCampaignObserver(g)
			if periodic {
				o.checkEveryNTasks = 1
			}
			scope := AlignmentScope{Workspace: workspace, Disposition: ScopePaths, Paths: []AlignmentPath{
				{Path: "pkg/a/file.go", Role: PathObservedEffect}, {Path: "pkg/b/file.go", Role: PathArtifact},
			}}
			check, err := o.OnTaskComplete(context.Background(), "task-1", "Repair cancellation", "done", scope)
			if err != nil || check == nil || !check.Result.Assessed() {
				t.Fatalf("task: %+v, %v", check, err)
			}
			for _, want := range []string{"Purpose A", "Purpose B", "[observed_effect] pkg/a/file.go", "[artifact] pkg/b/file.go"} {
				if !strings.Contains(client.users[0], want) {
					t.Errorf("task request lacks %q", want)
				}
			}
			observations, err := g.store.GetRecentObservations(20)
			if err != nil {
				t.Fatal(err)
			}
			for _, obs := range observations {
				if obs.Type == ObsFileChanged && obs.Subject == "pkg/b/file.go" {
					t.Fatal("artifact fabricated an observed change")
				}
			}
		})
	}
}

func TestGuardian_LargeModuleRequirementsAreNotTruncated(t *testing.T) {
	g, client, q, workspace := newScopedGuardian(t)
	statement := strings.Repeat("full requirement ", 10000) + "FINAL REQUIREMENT BYTES"
	q.facts["module_requirement"][0].Args[2] = statement
	check, err := g.CheckAlignment(context.Background(), TriggerManual, "Repair cancellation", "", scopedPaths(workspace, "pkg/a/file.go"))
	if err != nil || !check.Result.Assessed() || !strings.Contains(client.users[0], statement) {
		t.Fatalf("requirement lost: %+v, %v", check, err)
	}
}

func TestStore_HistoricalCheckDoesNotInventScope(t *testing.T) {
	store := newTestStore(t)
	if err := store.RecordAlignmentCheck(&AlignmentCheck{Subject: "historical", Result: AlignmentPassed, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	history, err := store.GetAlignmentHistory(1)
	if err != nil || len(history) != 1 || history[0].Coverage != nil {
		t.Fatalf("history: %+v, %v", history, err)
	}
}

func TestStore_OpeningEarlierSchemaPreservesUnknownHistoricalCoverage(t *testing.T) {
	nerdDir := filepath.Join(t.TempDir(), ".nerd")
	store, err := NewStore(nerdDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.RecordAlignmentCheck(&AlignmentCheck{Subject: "historical", Result: AlignmentPassed, Score: 0.9}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec("DROP TABLE alignment_coverage"); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = NewStore(nerdDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	history, err := store.GetAlignmentHistory(1)
	if err != nil || len(history) != 1 || history[0].Coverage != nil {
		t.Fatalf("earlier schema history: %+v, %v", history, err)
	}
	coverage := &AlignmentCoverage{Scope: AlignmentScope{Disposition: ScopeProjectOnly}, Outcome: ScopeResolved, Complete: true}
	if err := store.RecordAlignmentCheck(&AlignmentCheck{Subject: "new", Result: AlignmentPassed, Score: 0.9, Coverage: coverage}); err != nil {
		t.Fatal(err)
	}
	history, err = store.GetAlignmentHistory(2)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, check := range history {
		if check.Subject == "new" {
			found = reflect.DeepEqual(check.Coverage, coverage)
		}
	}
	if !found {
		t.Fatalf("new coverage after upgrade: %+v", history)
	}
}

type oversizedScopeClient struct {
	user  string
	limit int
}

func (c *oversizedScopeClient) CompleteWithSystem(_ context.Context, _, user string) (string, error) {
	c.user = user
	if len(user) > c.limit {
		return "", errors.New("request exceeds model window")
	}
	return "SCORE: 0.95\nRESULT: passed", nil
}

func TestGuardian_OversizedScopedRequestIsUnknownWithoutDroppingRequirements(t *testing.T) {
	g, _, q, workspace := newScopedGuardian(t)
	statement := strings.Repeat("required context ", 10000) + "FINAL REQUIREMENT BYTES"
	q.facts["module_requirement"][0].Args[2] = statement
	client := &oversizedScopeClient{limit: 4096}
	g.SetLLMClient(client)
	check, err := g.CheckAlignment(context.Background(), TriggerCampaignStart, "Repair cancellation", "", scopedPaths(workspace, "pkg/a/file.go"))
	if err != nil || check.Result != AlignmentUnknown || !strings.Contains(client.user, statement) {
		t.Fatalf("oversized request silently dropped evidence: %+v, %v", check, err)
	}
}
