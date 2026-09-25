package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/northstar"
)

func TestCampaignRecurseFlags_CoverEveryRecurseConfigField(t *testing.T) {
	covered := map[string]string{
		"MaxWaves":      "waves",
		"Subsystems":    "subsystem",
		"ContextBudget": "context-budget",
	}
	cfgType := reflect.TypeOf(campaign.RecurseConfig{})
	for i := range cfgType.NumField() {
		field := cfgType.Field(i)
		flagName, ok := covered[field.Name]
		if !ok {
			t.Errorf("RecurseConfig.%s has no `nerd campaign recurse` flag", field.Name)
			continue
		}
		if campaignRecurseCmd.Flags().Lookup(flagName) == nil {
			t.Errorf("RecurseConfig.%s claims flag --%s, which is not registered", field.Name, flagName)
		}
	}
}

func TestResolveRecurseConfig_MapsFlags(t *testing.T) {
	cfg, err := resolveRecurseConfig(3, []string{"session"}, 1000)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.MaxWaves != 3 || cfg.ContextBudget != 1000 {
		t.Fatalf("scalars wrong: %+v", cfg)
	}
	if len(cfg.Subsystems) != 1 || cfg.Subsystems[0] != "session" {
		t.Fatalf("subsystems wrong: %+v", cfg.Subsystems)
	}
}

func TestResolveRecurseConfig_RejectsBadInput(t *testing.T) {
	if _, err := resolveRecurseConfig(-1, nil, 0); err == nil {
		t.Error("negative passes must fail")
	}
	if _, err := resolveRecurseConfig(1, nil, -5); err == nil {
		t.Error("a negative context budget must fail")
	}
}

func TestCheckRecurseYolo(t *testing.T) {
	unbounded := campaign.RecurseConfig{MaxWaves: 0}
	if err := checkRecurseYolo(unbounded, false); err == nil {
		t.Fatal("unbounded without yolo must be refused")
	}
	if err := checkRecurseYolo(unbounded, true); err != nil {
		t.Fatalf("unbounded with yolo must pass: %v", err)
	}
	bounded := campaign.RecurseConfig{MaxWaves: 2}
	if err := checkRecurseYolo(bounded, false); err != nil {
		t.Fatalf("bounded without yolo must pass: %v", err)
	}
}

func TestRecurseAttemptConfig_FreshObserverPerLaterAttempt(t *testing.T) {
	sentinel := northstar.NewCampaignObserver(nil)
	base := campaign.OrchestratorConfig{Workspace: "w", NorthstarObserver: sentinel}
	calls := 0
	newObserver := func() *northstar.CampaignObserver {
		calls++
		return northstar.NewCampaignObserver(nil)
	}

	got0 := recurseAttemptConfig(base, 0, newObserver)
	if got0.NorthstarObserver != sentinel {
		t.Fatalf("attempt 0 must reuse base observer")
	}
	if calls != 0 {
		t.Fatalf("attempt 0 must not create an observer: calls=%d", calls)
	}

	got1 := recurseAttemptConfig(base, 1, newObserver)
	if got1.NorthstarObserver == nil {
		t.Fatal("attempt 1 observer must be non-nil")
	}
	if got1.NorthstarObserver == sentinel {
		t.Fatal("attempt 1 must not reuse the sentinel observer")
	}
	if got1.Workspace != "w" {
		t.Fatalf("attempt 1 must keep base fields: Workspace=%q", got1.Workspace)
	}

	got2 := recurseAttemptConfig(base, 2, newObserver)
	if got2.NorthstarObserver == nil {
		t.Fatal("attempt 2 observer must be non-nil")
	}
	if got2.NorthstarObserver == sentinel {
		t.Fatal("attempt 2 must not reuse the sentinel observer")
	}
	if got2.NorthstarObserver == got1.NorthstarObserver {
		t.Fatal("attempt 1 and attempt 2 observers must differ")
	}
	if calls != 2 {
		t.Fatalf("calls=%d, want 2", calls)
	}
	if base.NorthstarObserver != sentinel {
		t.Fatal("base must not be mutated")
	}
}

func TestRecurseAttemptConfig_ObserverRefsAreIndependent(t *testing.T) {
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	a := northstar.BuildCampaignObserver(ws, nil, nil)
	b := northstar.BuildCampaignObserver(ws, nil, nil)
	if a == nil || b == nil {
		t.Skip("BuildCampaignObserver returned nil in this environment")
	}
	if got := northstar.GuardianRefCount(nerdDir); got != 2 {
		t.Fatalf("refcount after two builds = %d, want 2", got)
	}
	a.Close()
	if got := northstar.GuardianRefCount(nerdDir); got != 1 {
		t.Fatalf("refcount after first Close = %d, want 1", got)
	}
	b.Close()
	if got := northstar.GuardianRefCount(nerdDir); got != 0 {
		t.Fatalf("refcount after second Close = %d, want 0", got)
	}
}

// --plan runs nothing and needs no model: on any workspace it prints the order
// derived from the workspace's own imports and the gates that will judge the
// run, including the ones that cannot run here.
func TestWriteRecursePlan_PrintsDerivedOrderAndGates(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"app/core/__init__.py": "",
		"app/web/views.py":     "from app.core import thing\n",
		"nerd.md": "---\nschema: nerd/v1\ngates:\n" +
			"  - id: style\n    kind: lint\n    run: ./scripts/style.sh {node}\n    scope: node\n" +
			"  - id: fuzz\n    kind: audit\n    run: definitely-not-installed-fuzzer --all\n---\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out strings.Builder
	if err := writeRecursePlan(&out, root, campaign.RecurseConfig{}); err != nil {
		t.Fatalf("writeRecursePlan: %v", err)
	}
	got := out.String()
	core, web := strings.Index(got, "app/core (python)"), strings.Index(got, "app/web (python)")
	if core < 0 || web < 0 || core > web {
		t.Fatalf("app/core must be listed before app/web, which imports it:\n%s", got)
	}
	if !strings.Contains(got, "nerd.md:style") {
		t.Fatalf("a workspace-relative gate script is runnable:\n%s", got)
	}
	unavailable := got[strings.Index(got, "cannot run here"):]
	if !strings.Contains(unavailable, "nerd.md:fuzz") || !strings.Contains(unavailable, "not on PATH") {
		t.Fatalf("a gate whose program is missing must be listed as unavailable, with why:\n%s", got)
	}
	if strings.Contains(got, "internal/mangle") {
		t.Fatalf("a Python workspace's plan must not name codeNERD's packages:\n%s", got)
	}
}
