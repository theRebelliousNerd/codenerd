package northstar

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// =============================================================================
// NS-VISION-01: import validation, no partial authority change
// =============================================================================

func validVision() *Vision {
	return &Vision{
		Mission:      "Ship trustworthy changes",
		Personas:     []Persona{{Name: "Operator", Needs: []string{"auditable decisions"}}},
		Capabilities: []Capability{{ID: "cap_a", Description: "Audit", Timeline: "now", Priority: "critical", Serves: []string{"Operator"}}},
		Risks:        []Risk{{ID: "risk_a", Description: "Drift", Likelihood: "high", Impact: "high", Mitigation: "reconcile"}},
		Requirements: []Requirement{{ID: "req_a", Type: "functional", Description: "One authority", Priority: "must_have",
			Supports: []string{"cap_a"}, Addresses: []string{"risk_a"}}},
	}
}

func issuePaths(t *testing.T, err error) []string {
	t.Helper()
	var verr *VisionValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("want *VisionValidationError, got %v", err)
	}
	if !errors.Is(err, ErrVisionImportRejected) {
		t.Fatal("validation errors must match ErrVisionImportRejected")
	}
	out := make([]string, 0, len(verr.Issues))
	for _, issue := range verr.Issues {
		out = append(out, issue.Path)
	}
	return out
}

func TestValidateVision_AcceptsAConnectedVision(t *testing.T) {
	if err := ValidateVision(validVision()); err != nil {
		t.Fatalf("valid vision rejected: %v", err)
	}
}

func TestValidateVision_ReportsEveryDanglingOrDuplicateReference(t *testing.T) {
	v := validVision()
	v.Capabilities[0].Serves = append(v.Capabilities[0].Serves, "Ghost", " ")
	v.Requirements[0].Supports = append(v.Requirements[0].Supports, "cap_missing")
	v.Requirements[0].Addresses = append(v.Requirements[0].Addresses, "risk_missing")
	v.Capabilities = append(v.Capabilities, Capability{ID: "cap_a", Description: "dup"})
	v.Personas = append(v.Personas, Persona{Name: "Operator"})
	v.Requirements = append(v.Requirements, Requirement{ID: ""})

	got := strings.Join(issuePaths(t, ValidateVision(v)), "\n")
	for _, want := range []string{
		"Capabilities[cap_a].serves[1]", "Capabilities[cap_a].serves[2]",
		"Requirements[req_a].supports[1]", "Requirements[req_a].addresses[1]",
		"Capabilities[1]", "Personas[1]", "Requirements[1]",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing diagnostic for %s; got:\n%s", want, got)
		}
	}
}

func TestValidateVision_RejectsMalformedMitigationRecords(t *testing.T) {
	sha := strings.Repeat("a", 40)
	cases := map[string]Risk{
		"Risks[r].mitigation_status":   {ID: "r", Mitigation: "x", MitigationStatus: "done"},
		"Risks[r].mitigation_evidence": {ID: "r", Mitigation: "x", MitigationStatus: MitigationVerified},
		"Risks[r].mitigation":          {ID: "r", MitigationStatus: MitigationImplemented},
		"Risks[r].mitigation_evidence[0].revision": {ID: "r", Mitigation: "x", MitigationStatus: MitigationVerified,
			MitigationEvidence: []MitigationEvidence{{Kind: "test", Ref: "t", Outcome: EvidencePassed, Revision: "abc123"}}},
		"Risks[r].mitigation_evidence[0].outcome": {ID: "r", Mitigation: "x", MitigationStatus: MitigationVerified,
			MitigationEvidence: []MitigationEvidence{{Kind: "test", Ref: "t", Outcome: "green", Revision: sha}}},
		"Risks[r].waiver":         {ID: "r", MitigationStatus: MitigationWaived},
		"Risks[r].waiver.expires": {ID: "r", MitigationStatus: MitigationWaived, Waiver: &RiskWaiver{Authority: "a", Scope: "s", Rationale: "r"}},
	}
	for want, risk := range cases {
		t.Run(want, func(t *testing.T) {
			v := &Vision{Mission: "m", Risks: []Risk{risk}}
			paths := issuePaths(t, ValidateVision(v))
			if !strings.Contains(strings.Join(paths, "\n"), want) {
				t.Fatalf("want diagnostic %s, got %v", want, paths)
			}
		})
	}
}

// A rejected update changes nothing: not the store, not the export surfaces,
// not the kernel.
func TestUpdateVision_WhenInvalid_ShouldChangeNoAuthority(t *testing.T) {
	store := newTestStore(t)
	nerdDir := filepath.Dir(store.Path())
	g := NewGuardian(store, DefaultGuardianConfig())
	kernel := &recordingKernel{}
	g.SetParentKernel(kernel)
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(validVision()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	jsonBefore, _ := os.ReadFile(filepath.Join(nerdDir, VisionJSONFileName))
	mgBefore, _ := os.ReadFile(filepath.Join(nerdDir, VisionMangleFileName))
	storedBefore, _ := store.LoadVision()
	assertsBefore := kernel.asserts

	bad := validVision()
	bad.Mission = "A different mission"
	bad.Requirements[0].Supports = []string{"cap_typo"}
	if err := g.UpdateVision(bad); !errors.Is(err, ErrVisionImportRejected) {
		t.Fatalf("want rejection, got %v", err)
	}

	jsonAfter, _ := os.ReadFile(filepath.Join(nerdDir, VisionJSONFileName))
	mgAfter, _ := os.ReadFile(filepath.Join(nerdDir, VisionMangleFileName))
	storedAfter, _ := store.LoadVision()
	if string(jsonBefore) != string(jsonAfter) || string(mgBefore) != string(mgAfter) {
		t.Fatal("export surfaces changed after a rejected update")
	}
	if !VisionsEquivalent(storedBefore, storedAfter) || g.GetVision().Mission != storedBefore.Mission {
		t.Fatal("store or guardian changed after a rejected update")
	}
	if kernel.asserts != assertsBefore {
		t.Fatal("kernel facts were re-projected after a rejected update")
	}
}

// The boot-time import must leave an invalid operator file untouched rather
// than "repair" it by exporting the store over it.
func TestSyncVisionAuthority_WhenJSONInvalid_ShouldRejectWithoutWriting(t *testing.T) {
	for name, body := range map[string]string{
		"dangling": `{"Mission":"m","Requirements":[{"id":"req_a","supports":["cap_typo"]}]}`,
		"corrupt":  `{"Mission": "trunc`,
	} {
		t.Run(name, func(t *testing.T) {
			store := newTestStore(t)
			nerdDir := filepath.Dir(store.Path())
			if err := store.SaveVision(validVision()); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(nerdDir, VisionJSONFileName)
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			future := time.Now().Add(time.Hour)
			if err := os.Chtimes(path, future, future); err != nil {
				t.Fatal(err)
			}

			res, err := SyncVisionAuthority(store, nerdDir)
			if !errors.Is(err, ErrVisionImportRejected) || res.Direction != SyncRejected {
				t.Fatalf("want rejection, got direction=%s err=%v", res.Direction, err)
			}
			if got, _ := os.ReadFile(path); string(got) != body {
				t.Fatal("the rejected file was overwritten")
			}
			if _, err := os.Stat(filepath.Join(nerdDir, VisionMangleFileName)); err == nil {
				t.Fatal("a .mg projection was written during a rejected sync")
			}
			stored, _ := store.LoadVision()
			if stored == nil || stored.Mission != "Ship trustworthy changes" {
				t.Fatal("the store's authoritative vision changed")
			}
		})
	}
}

type recordingKernel struct{ asserts int }

func (k *recordingKernel) Assert(types.Fact) error { k.asserts++; return nil }
func (k *recordingKernel) Retract(string) error    { return nil }

// The blueprint's own JSON candidate must import cleanly, and its documented
// Mangle projection must be exactly what ToFacts emits, plus the mitigation
// state facts this change introduced (every mitigation there is /proposed).
func TestBlueprintCandidate_ImportsAndProjectsAsDocumented(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "Docs", "architecture", "northstar", "proposed-vision-and-evidence-blueprint.md"))
	if err != nil {
		t.Skipf("blueprint not present: %v", err)
	}
	blocks := regexp.MustCompile("(?s)```(json|mangle)\n(.*?)```").FindAllStringSubmatch(string(data), -1)
	var jsonBlock, mangleBlock string
	for _, b := range blocks {
		switch b[1] {
		case "json":
			jsonBlock = b[2]
		case "mangle":
			mangleBlock = b[2]
		}
	}
	var doc WizardDocument
	if err := json.Unmarshal([]byte(jsonBlock), &doc); err != nil {
		t.Fatalf("candidate JSON: %v", err)
	}
	v := doc.ToVision()
	if err := ValidateVision(v); err != nil {
		t.Fatalf("candidate rejected: %v", err)
	}

	projected := map[string]bool{}
	for _, line := range FactStrings(v) {
		projected[strings.TrimSuffix(line, ".")] = true
	}
	for _, line := range strings.Split(mangleBlock, "\n") {
		line = strings.TrimSuffix(strings.TrimSpace(line), ".")
		if line == "" {
			continue
		}
		if !projected[line] {
			t.Errorf("documented fact not projected: %s", line)
		}
		delete(projected, line)
	}
	for line := range projected {
		if !strings.HasPrefix(line, "northstar_mitigation_state(") || !strings.HasSuffix(line, "/proposed)") {
			t.Errorf("projected fact not in the documented set: %s", line)
		}
	}
}

// =============================================================================
// NS-VISION-02: unavailable, error and malformed assessments are unknown
// =============================================================================

func TestParseAlignmentResponse_UnknownAndStricterVerdictContract(t *testing.T) {
	g := NewGuardian(nil, DefaultGuardianConfig())
	cases := []struct {
		name      string
		response  string
		want      AlignmentResult
		wantScore float64
	}{
		{"empty", "", AlignmentUnknown, 0},
		{"prose only", "Looks aligned to me.", AlignmentUnknown, 0},
		{"out of range score only", "SCORE: 85", AlignmentUnknown, 0},
		{"bare passed claim", "RESULT: passed\nEXPLANATION: fine", AlignmentUnknown, 0},
		{"score only classifies", "SCORE: 0.6", AlignmentWarning, 0.6},
		{"bare blocked is honoured", "RESULT: blocked", AlignmentBlocked, 0},
		{"bare failed gets its band floor", "RESULT: failed", AlignmentFailed, 0.3},
		{"passed label cannot launder a failing score", "SCORE: 0.2\nRESULT: passed", AlignmentBlocked, 0.2},
		{"blocked label beats a high score", "SCORE: 0.95\nRESULT: blocked", AlignmentBlocked, 0.95},
		{"agreeing verdict", "SCORE: 0.9\nRESULT: passed", AlignmentPassed, 0.9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := &AlignmentCheck{}
			g.parseAlignmentResponse(tc.response, check)
			if check.Result != tc.want || check.Score != tc.wantScore {
				t.Fatalf("got %s/%.2f, want %s/%.2f", check.Result, check.Score, tc.want, tc.wantScore)
			}
			if check.Result == AlignmentUnknown && check.Result.Assessed() {
				t.Fatal("unknown must not count as assessed")
			}
		})
	}
}

func TestCheckAlignment_LLMError_IsUnknownAndLeavesAverageAlone(t *testing.T) {
	store := newTestStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(validVision()); err != nil {
		t.Fatal(err)
	}
	g.SetLLMClient(&mockLLMClient{err: errors.New("rate limited")})
	before, _ := store.GetState()

	check, err := g.CheckAlignment(context.Background(), TriggerManual, "subject", "")
	if err != nil {
		t.Fatal(err)
	}
	if check.Result != AlignmentUnknown || !strings.Contains(check.Explanation, "rate limited") {
		t.Fatalf("LLM error must be unknown with its cause, got %s: %s", check.Result, check.Explanation)
	}
	after, _ := store.GetState()
	if after.OverallAlignment != before.OverallAlignment {
		t.Fatalf("an unassessed check moved the running alignment %.3f -> %.3f", before.OverallAlignment, after.OverallAlignment)
	}

	g.SetLLMClient(&mockLLMClient{response: "SCORE: 0.5\nRESULT: warning"})
	if _, err := g.CheckAlignment(context.Background(), TriggerManual, "subject", ""); err != nil {
		t.Fatal(err)
	}
	m, err := store.GetMetrics()
	if err != nil {
		t.Fatal(err)
	}
	if m.TotalChecks != 2 || m.UnknownRate != 0.5 || m.MeanScore != 0.5 {
		t.Fatalf("metrics must average assessed checks only: total=%d unknown=%.2f mean=%.2f", m.TotalChecks, m.UnknownRate, m.MeanScore)
	}
}

func TestBackgroundEventHandler_UnknownIsClarifyNotProceed(t *testing.T) {
	store := newTestStore(t)
	g := NewGuardian(store, DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(validVision()); err != nil {
		t.Fatal(err)
	}
	h := NewBackgroundEventHandler(g, "s")
	a, err := h.HandleEvent(context.Background(), "task_completed", "coder", "x.go", nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if a.Level != "clarify" || a.Metadata["result"] != string(AlignmentUnknown) {
		t.Fatalf("unassessed event must surface as clarify/unknown, got %s/%s", a.Level, a.Metadata["result"])
	}
}

// The alignment prompt carries each mitigation's state with its prose.
func TestAlignmentPrompt_ShowsMitigationState(t *testing.T) {
	g := NewGuardian(nil, DefaultGuardianConfig())
	_, prompt := alignmentPrompts(t, g, validVision())
	if !strings.Contains(prompt, "Mitigation (proposed): reconcile") {
		t.Fatalf("prompt does not mark the mitigation as proposed:\n%s", prompt)
	}
}
