package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/northstar"
)

func TestCampaignRecurseFlags_CoverEveryRecurseConfigField(t *testing.T) {
	covered := map[string]string{
		"Mission":       "mission",
		"MaxWaves":      "waves",
		"Subsystems":    "subsystem",
		"ContextBudget": "context-budget",
		"BaseBranch":    "base-branch",
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
	cfg, err := resolveRecurseConfig(3, []string{"session"}, 1000, "trunk")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if cfg.MaxWaves != 3 || cfg.ContextBudget != 1000 || cfg.BaseBranch != "trunk" {
		t.Fatalf("scalars wrong: %+v", cfg)
	}
	if len(cfg.Subsystems) != 1 || cfg.Subsystems[0] != "session" {
		t.Fatalf("subsystems wrong: %+v", cfg.Subsystems)
	}
}

func TestResolveRecurseConfig_RejectsBadInput(t *testing.T) {
	if _, err := resolveRecurseConfig(-1, nil, 0, ""); err == nil {
		t.Error("negative passes must fail")
	}
	if _, err := resolveRecurseConfig(1, nil, -5, ""); err == nil {
		t.Error("a negative context budget must fail")
	}
}

// Recurse runs until stopped unless bounded, and is controlled from outside
// a running loop with status and stop.
func TestCampaignRecurse_ForeverByDefaultWithStatusAndStop(t *testing.T) {
	if got := campaignRecurseCmd.Flags().Lookup("waves").DefValue; got != "0" {
		t.Fatalf("--waves defaults to %s; recurse runs until stopped unless bounded", got)
	}
	for _, name := range []string{"status", "stop"} {
		found := false
		for _, c := range campaignRecurseCmd.Commands() {
			found = found || c.Name() == name
		}
		if !found {
			t.Fatalf("`nerd campaign recurse %s` is not registered", name)
		}
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
	t.Parallel()
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

// seedRecurseJournal writes records as JSONL to the workspace's recurse journal.
// Each test owns its TempDir, so no shared state leaks between table rows.
func seedRecurseJournal(t *testing.T, ws string, records []map[string]any) {
	t.Helper()
	var lines strings.Builder
	for _, r := range records {
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal seed record: %v", err)
		}
		lines.Write(raw)
		lines.WriteByte('\n')
	}
	journalPath := campaign.RecurseJournalPath(ws)
	if err := os.MkdirAll(filepath.Dir(journalPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(journalPath, []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runFreshStatus runs a fresh status command bound to ws, hermetically: each
// call owns its command, flag set, and output buffer, so no package-global
// workspace, flag, or SetOut state leaks between tests and t.Parallel stays safe.
func runFreshStatus(t *testing.T, ws string, asJSON bool) string {
	t.Helper()
	c := newRecurseStatusCmd(ws)
	var buf strings.Builder
	c.SetOut(&buf)
	flagValue := "false"
	if asJSON {
		flagValue = "true"
	}
	if err := c.Flags().Set("json", flagValue); err != nil {
		t.Fatalf("set --json=%s: %v", flagValue, err)
	}
	if err := c.RunE(c, nil); err != nil {
		t.Fatalf("status RunE (json=%v): %v", asJSON, err)
	}
	return buf.String()
}

func decodeStatusJSON(t *testing.T, out string) campaign.RecurseStatusJSON {
	t.Helper()
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "{") || strings.Contains(trimmed, "\n") {
		t.Fatalf("status --json should be one JSON object, got %q", out)
	}
	var decoded campaign.RecurseStatusJSON
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		t.Fatalf("status --json not parseable: %v output %q", err, out)
	}
	return decoded
}

// TestCampaignRecurseStatusJSON_SeededJournalParity seeds a TempDir journal and
// checks status --json emits one parseable object with text-report parity.
func TestCampaignRecurseStatusJSON_SeededJournalParity(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	records := []map[string]any{
		{"time": "2026-01-02T15:04:05Z", "step": "base", "pass": 1, "base": "main", "commit": "abcdef1234567890"},
		{"time": "2026-01-02T15:05:05Z", "step": "absorb", "pass": 1, "commit": "1234567890abcdef"},
		{"time": "2026-01-02T15:06:05Z", "step": "absorb_conflict", "pass": 1, "commit": "deadbeef12345678", "detail": "merge conflict"},
		{"time": "2026-01-02T15:07:05Z", "step": "rebuild", "pass": 1, "commit": "abcdef1234567890", "detail": "/tmp/nerd-test-binary"},
		{"time": "2026-01-02T15:08:05Z", "step": "rebuild_failed", "pass": 1, "commit": "feedface12345678", "detail": "build failed"},
		{"time": "2026-01-02T15:09:05Z", "step": "pass_start", "pass": 1},
		{"time": "2026-01-02T15:10:05Z", "step": "visit", "pass": 1, "node": "node-b"},
		{"time": "2026-01-02T15:11:05Z", "step": "ratchet", "pass": 1, "cycle": 1, "node": "node-a", "outcome": "/kept", "commit": "abcdef1234567890", "signature": "sig1", "finding": "finding-a"},
		{"time": "2026-01-02T15:12:05Z", "step": "ratchet", "pass": 1, "cycle": 2, "node": "node-b", "outcome": "/unverified"},
	}
	seedRecurseJournal(t, ws, records)

	jsonOut := runFreshStatus(t, ws, true)
	decoded := decodeStatusJSON(t, jsonOut)
	if decoded.Running {
		t.Errorf("Running = true, want false (no lock in TempDir)")
	}
	if decoded.StopRequested {
		t.Errorf("StopRequested = true, want false (no stop request in TempDir)")
	}
	if decoded.State != "not running" {
		t.Errorf("State = %q, want %q", decoded.State, "not running")
	}
	if decoded.Pass != 1 {
		t.Errorf("Pass = %d, want 1", decoded.Pass)
	}
	if decoded.Node != "node-b" {
		t.Errorf("Node = %q, want %q", decoded.Node, "node-b")
	}
	if decoded.Cycles != 2 || decoded.Kept != 1 || decoded.Unverified != 1 || decoded.Reverted != 0 || decoded.Refused != 0 {
		t.Errorf("counts = %d/%d/%d/%d/%d, want 2/1/0/0/1 (cycles/kept/reverted/refused/unverified)", decoded.Cycles, decoded.Kept, decoded.Reverted, decoded.Refused, decoded.Unverified)
	}
	if decoded.Base.Branch != "main" || decoded.Base.Tip != "abcdef1234567890" || decoded.Base.TipShort != "abcdef123" {
		t.Errorf("base tip = %+v, want branch main tip abcdef1234567890/abcdef123", decoded.Base)
	}
	if decoded.Base.LastAbsorbed != "1234567890abcdef" || decoded.Base.LastAbsorbedShort != "123456789" || decoded.Base.Absorbs != 1 {
		t.Errorf("base last absorbed = %+v, want 1234567890abcdef/123456789 x1", decoded.Base)
	}
	if len(decoded.Base.Conflicts) != 1 || decoded.Base.Conflicts[0].BaseCommit != "deadbeef12345678" || decoded.Base.Conflicts[0].ShortCommit != "deadbeef1" || decoded.Base.Conflicts[0].Detail != "merge conflict" {
		t.Errorf("conflicts = %+v, want one deadbeef12345678/deadbeef1", decoded.Base.Conflicts)
	}
	if len(decoded.Base.Generations) != 2 {
		t.Fatalf("generations len = %d, want 2", len(decoded.Base.Generations))
	}
	if decoded.Base.Generations[0].Commit != "abcdef1234567890" || decoded.Base.Generations[0].ShortCommit != "abcdef123" || decoded.Base.Generations[0].Binary != "/tmp/nerd-test-binary" {
		t.Errorf("generation 0 = %+v, want abcdef1234567890/abcdef123 binary", decoded.Base.Generations[0])
	}
	if decoded.Base.Generations[1].Commit != "feedface12345678" || decoded.Base.Generations[1].ShortCommit != "feedface1" || decoded.Base.Generations[1].Failure != "build failed" {
		t.Errorf("generation 1 = %+v, want feedface12345678/feedface1 failure", decoded.Base.Generations[1])
	}
	if len(decoded.Attempts) != 2 {
		t.Fatalf("Attempts len = %d, want 2", len(decoded.Attempts))
	}
	if decoded.Attempts[0].Cycle != 1 || decoded.Attempts[0].Outcome != "/kept" || decoded.Attempts[0].Node != "node-a" || decoded.Attempts[0].Commit != "abcdef1234567890" {
		t.Errorf("attempt 0 = %+v, want cycle 1 /kept node-a abcdef1234567890", decoded.Attempts[0])
	}
	if decoded.Attempts[0].ShortCommit != "abcdef123" {
		t.Errorf("attempt 0 ShortCommit = %q, want %q", decoded.Attempts[0].ShortCommit, "abcdef123")
	}
	if decoded.Attempts[1].Cycle != 2 || decoded.Attempts[1].Outcome != "/unverified" || decoded.Attempts[1].Node != "node-b" || decoded.Attempts[1].Commit != "" {
		t.Errorf("attempt 1 = %+v, want cycle 2 /unverified node-b empty commit", decoded.Attempts[1])
	}
	if decoded.Attempts[1].ShortCommit != "" {
		t.Errorf("attempt 1 ShortCommit = %q, want empty (nil-safe)", decoded.Attempts[1].ShortCommit)
	}

	// Text-vs-JSON cross-check on the same workspace: counts, node, and base
	// lines rendered by String() must match the decoded JSON, so a divergent
	// text path cannot slip through.
	textOut := runFreshStatus(t, ws, false)
	if strings.HasPrefix(strings.TrimSpace(textOut), "{") {
		t.Errorf("status text output should not be JSON, got %q", textOut)
	}
	wantCounts := strings.Join([]string{
		strconv.Itoa(decoded.Cycles) + " attempts:",
		strconv.Itoa(decoded.Kept) + " kept,",
		strconv.Itoa(decoded.Reverted) + " reverted,",
		strconv.Itoa(decoded.Refused) + " refused,",
		strconv.Itoa(decoded.Unverified) + " unverified.",
	}, " ")
	if !strings.Contains(textOut, wantCounts) {
		t.Errorf("text counts %q should contain %q", textOut, wantCounts)
	}
	if decoded.Node != "" && !strings.Contains(textOut, "visiting "+decoded.Node) {
		t.Errorf("text %q should mention visiting %q from JSON", textOut, decoded.Node)
	}
	if !strings.Contains(textOut, "Recurse: "+decoded.State) {
		t.Errorf("text %q should contain state %q from JSON", textOut, decoded.State)
	}
	if decoded.Base.Branch != "" && !strings.Contains(textOut, "Base: "+decoded.Base.Branch) {
		t.Errorf("text %q should contain base branch %q from JSON", textOut, decoded.Base.Branch)
	}
	if len(decoded.Base.Conflicts) > 0 && !strings.Contains(textOut, "Merge conflicts: "+strconv.Itoa(len(decoded.Base.Conflicts))) {
		t.Errorf("text %q should report %d merge conflicts from JSON", textOut, len(decoded.Base.Conflicts))
	}
	if len(decoded.Base.Generations) > 0 && !strings.Contains(textOut, "Binary generations:") {
		t.Errorf("text %q should list binary generations from JSON", textOut)
	}
}

// TestCampaignRecurseStatusJSON_Edges covers the empty journal, sub-9-char
// hashes, and stop-file states with hermetic fresh commands.
func TestCampaignRecurseStatusJSON_Edges(t *testing.T) {
	t.Parallel()
	t.Run("empty journal", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		decoded := decodeStatusJSON(t, runFreshStatus(t, ws, true))
		if decoded.Running {
			t.Errorf("Running = true, want false on empty journal")
		}
		if decoded.StopRequested {
			t.Errorf("StopRequested = true, want false on empty journal")
		}
		if decoded.State != "not running" {
			t.Errorf("State = %q, want %q on empty journal", decoded.State, "not running")
		}
		if decoded.Pass != 0 {
			t.Errorf("Pass = %d, want 0 on empty journal", decoded.Pass)
		}
		if decoded.Cycles != 0 || decoded.Kept != 0 || decoded.Reverted != 0 || decoded.Refused != 0 || decoded.Unverified != 0 {
			t.Errorf("counts = %d/%d/%d/%d/%d, want all zero on empty journal", decoded.Cycles, decoded.Kept, decoded.Reverted, decoded.Refused, decoded.Unverified)
		}
		if len(decoded.Attempts) != 0 {
			t.Errorf("Attempts len = %d, want 0 on empty journal", len(decoded.Attempts))
		}
		textOut := runFreshStatus(t, ws, false)
		if !strings.Contains(textOut, "Recurse: not running. Pass 0") {
			t.Errorf("empty text %q should report not running pass 0", textOut)
		}
		if !strings.Contains(textOut, "0 attempts: 0 kept, 0 reverted, 0 refused, 0 unverified.") {
			t.Errorf("empty text %q should report zero counts", textOut)
		}
	})
	t.Run("sub-9-char hash", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		seedRecurseJournal(t, ws, []map[string]any{
			{"time": "2026-01-02T15:04:05Z", "step": "base", "pass": 1, "base": "main", "commit": "ab12"},
			{"time": "2026-01-02T15:09:05Z", "step": "pass_start", "pass": 1},
			{"time": "2026-01-02T15:10:05Z", "step": "visit", "pass": 1, "node": "node-a"},
			{"time": "2026-01-02T15:11:05Z", "step": "ratchet", "pass": 1, "cycle": 1, "node": "node-a", "outcome": "/kept", "commit": "ab12"},
			{"time": "2026-01-02T15:12:05Z", "step": "ratchet", "pass": 1, "cycle": 2, "node": "node-a", "outcome": "/kept", "commit": ""},
		})
		decoded := decodeStatusJSON(t, runFreshStatus(t, ws, true))
		if decoded.Base.Tip != "ab12" || decoded.Base.TipShort != "ab12" {
			t.Errorf("base tip = %+v, want ab12/ab12 (short hash kept whole)", decoded.Base)
		}
		if len(decoded.Attempts) != 2 {
			t.Fatalf("Attempts len = %d, want 2", len(decoded.Attempts))
		}
		if decoded.Attempts[0].Commit != "ab12" || decoded.Attempts[0].ShortCommit != "ab12" {
			t.Errorf("attempt 0 = %+v, want commit ab12 short ab12", decoded.Attempts[0])
		}
		if decoded.Attempts[1].Commit != "" || decoded.Attempts[1].ShortCommit != "" {
			t.Errorf("attempt 1 = %+v, want empty commit/short (nil-safe)", decoded.Attempts[1])
		}
		// Text-vs-JSON cross-check on the same workspace: the short hash must
		// render without panicking and counts must match the decoded JSON.
		textOut := runFreshStatus(t, ws, false)
		if !strings.Contains(textOut, "2 attempts: 2 kept, 0 reverted, 0 refused, 0 unverified.") {
			t.Errorf("short-hash text %q should report 2 kept from JSON", textOut)
		}
		if !strings.Contains(textOut, "visiting node-a") {
			t.Errorf("short-hash text %q should mention visiting node-a from JSON", textOut)
		}
		if !strings.Contains(textOut, "ab12") {
			t.Errorf("short-hash text %q should render the sub-9-char hash", textOut)
		}
	})
	t.Run("stop file", func(t *testing.T) {
		t.Parallel()
		ws := t.TempDir()
		seedRecurseJournal(t, ws, []map[string]any{
			{"time": "2026-01-02T15:09:05Z", "step": "pass_start", "pass": 3},
			{"time": "2026-01-02T15:10:05Z", "step": "visit", "pass": 3, "node": "node-s"},
		})
		if err := campaign.RequestRecurseStop(ws); err != nil {
			t.Fatalf("RequestRecurseStop: %v", err)
		}
		decoded := decodeStatusJSON(t, runFreshStatus(t, ws, true))
		if decoded.Running {
			t.Errorf("Running = true, want false after stop file with no lock")
		}
		if !decoded.StopRequested {
			t.Errorf("StopRequested = false, want true after stop file")
		}
		if !strings.Contains(decoded.State, "stop requested") {
			t.Errorf("State = %q, want it to contain %q", decoded.State, "stop requested")
		}
		if decoded.Pass != 3 {
			t.Errorf("Pass = %d, want 3 after stop file", decoded.Pass)
		}
		if decoded.Node != "node-s" {
			t.Errorf("Node = %q, want %q after stop file", decoded.Node, "node-s")
		}
		if decoded.Cycles != 0 || decoded.Kept != 0 || decoded.Reverted != 0 || decoded.Refused != 0 || decoded.Unverified != 0 {
			t.Errorf("counts = %d/%d/%d/%d/%d, want all zero with no ratchets", decoded.Cycles, decoded.Kept, decoded.Reverted, decoded.Refused, decoded.Unverified)
		}
		textOut := runFreshStatus(t, ws, false)
		if !strings.Contains(textOut, "stop requested") {
			t.Errorf("text %q should mention stop requested", textOut)
		}
		if !strings.Contains(textOut, "Recurse: "+decoded.State) {
			t.Errorf("text %q should contain state %q from JSON", textOut, decoded.State)
		}
		if !strings.Contains(textOut, "visiting node-s") {
			t.Errorf("text %q should mention visiting node-s from JSON", textOut)
		}
		if !strings.Contains(textOut, "0 attempts: 0 kept, 0 reverted, 0 refused, 0 unverified.") {
			t.Errorf("text %q should report zero counts from JSON", textOut)
		}
	})
}
