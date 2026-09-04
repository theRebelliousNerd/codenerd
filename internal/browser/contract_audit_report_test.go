package browser

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildAuditReport_ShouldCountEveryFindingKindIncludingZero(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-zero",
		Discovery: ContractAuditDiscovery{
			Needles:  []string{},
			Findings: []AuditFinding{},
			Matches:  []RepoMatch{},
			Notes:    []string{},
		},
		Correlations: ContainerCorrelationResult{},
	}
	report := BuildAuditReport(in)
	wantKinds := []AuditFindingKind{
		AuditObservation,
		AuditInference,
		AuditSkipped,
		AuditApprovalRequired,
		AuditExecutionFailure,
		AuditContractMismatch,
	}
	for _, k := range wantKinds {
		v, ok := report.Counts[string(k)]
		if !ok {
			t.Fatalf("missing count for %q, counts=%v", k, report.Counts)
		}
		if v != 0 {
			t.Fatalf("expected 0 for %q, got %d", k, v)
		}
	}
	for _, extra := range []string{"needles", "matches", "correlations"} {
		if _, ok := report.Counts[extra]; !ok {
			t.Fatalf("missing extra count %q", extra)
		}
	}
	if report.Counts["needles"] != 0 || report.Counts["matches"] != 0 || report.Counts["correlations"] != 0 {
		t.Fatalf("expected zero extra counts, got %v", report.Counts)
	}
	in2 := AuditReportInput{
		SessionID: "sess-one",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{
				{Kind: AuditObservation, Subject: "s1", Detail: "d1"},
				{Kind: AuditInference, Subject: "s2", Detail: "d2"},
				{Kind: AuditSkipped, Subject: "s3", Detail: "d3"},
			},
			Needles: []string{"n1", "n2"},
			Matches: []RepoMatch{{Path: "a/b.go", Line: 1, Needle: "n1"}},
		},
		Correlations: ContainerCorrelationResult{
			Correlations: []ContainerCorrelation{{Container: "c1", LogMessage: "msg", DeltaMs: 5}},
		},
	}
	report2 := BuildAuditReport(in2)
	for _, k := range wantKinds {
		if _, ok := report2.Counts[string(k)]; !ok {
			t.Fatalf("missing count for %q in non-zero case", k)
		}
	}
	if report2.Counts[string(AuditObservation)] != 1 {
		t.Fatalf("expected 1 observation, got %d", report2.Counts[string(AuditObservation)])
	}
	if report2.Counts["needles"] != 2 {
		t.Fatalf("expected needles 2, got %d", report2.Counts["needles"])
	}
	if report2.Counts["matches"] != 1 {
		t.Fatalf("expected matches 1, got %d", report2.Counts["matches"])
	}
	if report2.Counts["correlations"] != 1 {
		t.Fatalf("expected correlations 1, got %d", report2.Counts["correlations"])
	}
}

func TestBuildAuditReport_ShouldEmitHandleOnlyForNonEmptySections(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-handle",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{
				{Kind: AuditObservation, Subject: "subj1", Detail: "detail1"},
			},
		},
		Correlations: ContainerCorrelationResult{},
	}
	report := BuildAuditReport(in)
	wantHandle := "audit:sess-handle:observations"
	found := false
	for _, h := range report.Handles {
		if h == wantHandle {
			found = true
		}
		if strings.Contains(h, "inferences") {
			t.Fatalf("unexpected handle for empty inferences: %v", report.Handles)
		}
		if strings.Contains(h, "skipped") {
			t.Fatalf("unexpected handle for empty skipped: %v", report.Handles)
		}
		if strings.Contains(h, "approval_required") {
			t.Fatalf("unexpected handle for empty approval_required: %v", report.Handles)
		}
		if strings.Contains(h, "contract_mismatches") {
			t.Fatalf("unexpected handle for empty contract_mismatches: %v", report.Handles)
		}
		if strings.Contains(h, "execution_failures") {
			t.Fatalf("unexpected handle for empty execution_failures: %v", report.Handles)
		}
		if strings.Contains(h, "sources") {
			t.Fatalf("unexpected handle for empty sources: %v", report.Handles)
		}
		if strings.Contains(h, "backend_logs") {
			t.Fatalf("unexpected handle for empty backend_logs: %v", report.Handles)
		}
	}
	if !found {
		t.Fatalf("expected handle %q not found in %v", wantHandle, report.Handles)
	}
	if len(report.Handles) != 1 {
		t.Fatalf("expected 1 handle, got %d: %v", len(report.Handles), report.Handles)
	}
	for _, sec := range []string{"inferences", "skipped", "approval_required", "contract_mismatches", "execution_failures"} {
		if _, ok := report.Sections[sec]; !ok {
			t.Fatalf("expected section %q to be present even when empty", sec)
		}
		if len(report.Sections[sec]) != 0 {
			t.Fatalf("expected empty section %q, got %v", sec, report.Sections[sec])
		}
	}
}

func TestBuildAuditReport_WhenEvidenceTruncated_ShouldMarkReportTruncated(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-trunc",
		Discovery: ContractAuditDiscovery{
			Findings:  []AuditFinding{},
			Matches:   []RepoMatch{},
			Notes:     []string{"discovery truncated note"},
			Truncated: true,
		},
		Correlations: ContainerCorrelationResult{},
	}
	report := BuildAuditReport(in)
	if !report.Truncated {
		t.Fatal("expected Truncated true when discovery truncated")
	}
	foundNote := false
	for _, n := range report.Notes {
		if strings.Contains(n, "discovery truncated") {
			foundNote = true
		}
	}
	if !foundNote {
		t.Fatalf("expected discovery note carried through, got %v", report.Notes)
	}
	in2 := AuditReportInput{
		SessionID: "sess-trunc2",
		Discovery: ContractAuditDiscovery{Truncated: false},
		Correlations: ContainerCorrelationResult{
			Notes:     []string{"correlation truncated"},
			Truncated: true,
		},
	}
	report2 := BuildAuditReport(in2)
	if !report2.Truncated {
		t.Fatal("expected Truncated true when correlations truncated")
	}
	foundCorrNote := false
	for _, n := range report2.Notes {
		if strings.Contains(n, "correlation truncated") {
			foundCorrNote = true
		}
	}
	if !foundCorrNote {
		t.Fatalf("expected correlation note carried, got %v", report2.Notes)
	}
	in3 := AuditReportInput{
		SessionID: "sess-notrunc",
		Discovery: ContractAuditDiscovery{Findings: []AuditFinding{{Kind: AuditObservation, Subject: "s", Detail: "d"}}},
		Correlations: ContainerCorrelationResult{},
	}
	report3 := BuildAuditReport(in3)
	if report3.Truncated {
		t.Fatal("expected Truncated false when no evidence truncated and no line cap exceeded")
	}
}

func TestBuildAuditReport_ShouldEmitRelativeSourcePathsOnly(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-rel",
		Discovery: ContractAuditDiscovery{
			Matches: []RepoMatch{
				{Path: "/absolute/path/file.go", Line: 10, Needle: "test"},
				{Path: "relative/path/file.go", Line: 20, Needle: "test"},
				{Path: "/tmp/abs2.go", Line: 30, Needle: "test"},
			},
		},
	}
	report := BuildAuditReport(in)
	srcLines, ok := report.Sections["sources"]
	if !ok {
		t.Fatal("expected sources section")
	}
	if len(srcLines) != 3 {
		t.Fatalf("expected 3 source lines, got %d: %v", len(srcLines), srcLines)
	}
	for _, line := range srcLines {
		idx := strings.LastIndex(line, ":")
		if idx == -1 {
			t.Fatalf("source line missing colon: %q", line)
		}
		pathPart := line[:idx]
		if filepath.IsAbs(pathPart) {
			t.Fatalf("source path is absolute %q in line %q", pathPart, line)
		}
	}
	for _, line := range srcLines {
		if strings.HasPrefix(line, "/") {
			t.Fatalf("source line starts with slash, not relative: %q", line)
		}
	}
}

func TestBuildAuditReport_WhenTooManyLines_ShouldTruncateAndNote(t *testing.T) {
	var findings []AuditFinding
	for i := 0; i < 500; i++ {
		findings = append(findings, AuditFinding{
			Kind:    AuditObservation,
			Subject: fmt.Sprintf("subj-%d", i),
			Detail:  fmt.Sprintf("detail-%d", i),
		})
	}
	in := AuditReportInput{
		SessionID: "sess-many",
		Discovery: ContractAuditDiscovery{
			Findings: findings,
			Notes:    []string{"initial"},
		},
	}
	report := BuildAuditReport(in)
	if !report.Truncated {
		t.Fatal("expected Truncated true when too many lines")
	}
	total := 0
	for _, sec := range report.Sections {
		total += len(sec)
	}
	if total > maxAuditReportLines {
		t.Fatalf("total lines %d exceeds cap %d", total, maxAuditReportLines)
	}
	foundTruncNote := false
	for _, n := range report.Notes {
		if strings.Contains(strings.ToLower(n), "truncated") {
			foundTruncNote = true
		}
	}
	if !foundTruncNote {
		t.Fatalf("expected truncation note, got %v", report.Notes)
	}
	if len(report.Sections["observations"]) != maxAuditReportLines {
		t.Fatalf("expected observations truncated to %d, got %d", maxAuditReportLines, len(report.Sections["observations"]))
	}
}

func TestBuildAuditReport_ShouldBePure(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-pure",
		Discovery: ContractAuditDiscovery{
			Needles: []string{"needle1", "needle2"},
			Findings: []AuditFinding{
				{Kind: AuditObservation, Subject: "alpha", Detail: "detail a"},
				{Kind: AuditInference, Subject: "beta", Detail: "detail b"},
				{Kind: AuditSkipped, Subject: "gamma", Detail: "detail c"},
				{Kind: AuditApprovalRequired, Subject: "delta", Detail: "detail d"},
			},
			Matches: []RepoMatch{
				{Path: "a/b.go", Line: 10, Needle: "needle1"},
				{Path: "c/d.go", Line: 20, Needle: "needle2"},
			},
			Notes: []string{"note1"},
		},
		Correlations: ContainerCorrelationResult{
			Correlations: []ContainerCorrelation{
				{Container: "app", LogMessage: "msg1", DeltaMs: 100},
				{Container: "app", LogMessage: "msg2", DeltaMs: -50},
			},
			Notes: []string{"corr note"},
		},
		KernelFacts: 5,
	}
	report1 := BuildAuditReport(in)
	report2 := BuildAuditReport(in)
	if !reflect.DeepEqual(report1, report2) {
		t.Fatalf("BuildAuditReport not pure: first=%+v second=%+v", report1, report2)
	}
	if len(in.Discovery.Findings) != 4 {
		t.Fatalf("input mutated: findings len changed")
	}
	if len(in.Discovery.Matches) != 2 {
		t.Fatalf("input matches mutated")
	}
}

func TestResumeAuditEvidence_ShouldReturnOnlyRequestedSections(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-resume-one",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{
				{Kind: AuditObservation, Subject: "o1", Detail: "d1"},
				{Kind: AuditInference, Subject: "i1", Detail: "d2"},
			},
			Matches: []RepoMatch{{Path: "src/a.go", Line: 1, Needle: "o1"}},
		},
		Correlations: ContainerCorrelationResult{
			Correlations: []ContainerCorrelation{{Container: "c", LogMessage: "m", DeltaMs: 10}},
		},
	}
	report := BuildAuditReport(in)
	if len(report.Handles) < 4 {
		t.Fatalf("expected at least 4 handles, got %v", report.Handles)
	}
	handles := []string{"audit:sess-resume-one:observations"}
	result, _ := ResumeAuditEvidence(report, handles)
	if _, ok := result["observations"]; !ok {
		t.Fatalf("expected observations in result, got %v", result)
	}
	if _, ok := result["inferences"]; ok {
		t.Fatal("unexpected inferences in result when not requested")
	}
	if _, ok := result["sources"]; ok {
		t.Fatal("unexpected sources in result when not requested")
	}
	if _, ok := result["backend_logs"]; ok {
		t.Fatal("unexpected backend_logs in result when not requested")
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 section, got %d: %v", len(result), result)
	}
}

func TestResumeAuditEvidence_WhenEmptyHandles_ShouldReturnNothing(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-empty",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{{Kind: AuditObservation, Subject: "s", Detail: "d"}},
			Matches:  []RepoMatch{{Path: "a/b.go", Line: 1}},
		},
	}
	report := BuildAuditReport(in)
	result, notes := ResumeAuditEvidence(report, []string{})
	if len(result) != 0 {
		t.Fatalf("expected empty result for empty handles, got %v", result)
	}
	if len(notes) == 0 {
		t.Fatal("expected one note for empty handles, got none")
	}
	if len(result) == len(report.Sections) && len(result) != 0 {
		t.Fatal("empty handles fallback to whole report, which must not happen")
	}
}

func TestResumeAuditEvidence_WhenHandleFromAnotherSession_ShouldRefuse(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-A",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{{Kind: AuditObservation, Subject: "s", Detail: "d"}},
		},
	}
	report := BuildAuditReport(in)
	handles := []string{"audit:sess-B:observations"}
	result, notes := ResumeAuditEvidence(report, handles)
	if len(result) != 0 {
		t.Fatalf("expected empty result for cross-session handle, got %v", result)
	}
	if len(notes) == 0 {
		t.Fatal("expected note for session mismatch")
	}
	foundMismatch := false
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n), "session") || strings.Contains(strings.ToLower(n), "mismatch") {
			foundMismatch = true
		}
	}
	if !foundMismatch {
		t.Fatalf("expected session mismatch note, got %v", notes)
	}
	handles2 := []string{"audit:sess-A:observations"}
	result2, _ := ResumeAuditEvidence(report, handles2)
	if len(result2) == 0 {
		t.Fatal("expected result for correct session handle, got empty")
	}
}

func TestResumeAuditEvidence_WhenUnknownSection_ShouldNote(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-unknown",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{{Kind: AuditObservation, Subject: "s", Detail: "d"}},
		},
	}
	report := BuildAuditReport(in)
	handles := []string{"audit:sess-unknown:unknown_section"}
	result, notes := ResumeAuditEvidence(report, handles)
	if len(result) != 0 {
		t.Fatalf("expected empty result for unknown section, got %v", result)
	}
	if len(notes) == 0 {
		t.Fatal("expected note for unknown section")
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, "unknown_section") || strings.Contains(strings.ToLower(n), "unknown") {
			if strings.Contains(n, "audit:sess-unknown:unknown_section") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected note naming handle, got %v", notes)
	}
}

func TestResumeAuditEvidence_WhenTooManyHandles_ShouldCapAndNote(t *testing.T) {
	in := AuditReportInput{
		SessionID: "sess-cap",
		Discovery: ContractAuditDiscovery{
			Findings: []AuditFinding{{Kind: AuditObservation, Subject: "s", Detail: "d"}},
		},
	}
	report := BuildAuditReport(in)
	handles := make([]string, 25)
	for i := 0; i < 25; i++ {
		handles[i] = "audit:sess-cap:observations"
	}
	result, notes := ResumeAuditEvidence(report, handles)
	if len(notes) == 0 {
		t.Fatal("expected cap note for too many handles")
	}
	foundCap := false
	for _, n := range notes {
		if strings.Contains(strings.ToLower(n), "capped") {
			foundCap = true
		}
	}
	if !foundCap {
		t.Fatalf("expected cap note mentioning capped, got %v", notes)
	}
	if _, ok := result["observations"]; !ok {
		t.Fatalf("expected observations in result even after cap, got %v", result)
	}
}
func TestAuditReportRelativePath_ShouldStripRootOnEitherPlatformConvention(t *testing.T) {
	// This test documents the platform trap that motivated looksRooted:
	// filepath.IsAbs is platform-specific - on Windows a leading "/" is not
	// absolute, so a naive IsAbs check would leak POSIX-absolute paths into
	// the audit report. The report is read on a machine we do not control,
	// so any rooted form (POSIX, Windows, drive, UNC) must be stripped on
	// every platform.
	rooted := []string{
		"/absolute/path/file.go",
		"\\windows\\style\\file.go",
		"C:/drive/file.go",
		"C:\\drive\\file.go",
		"//server/share/file.go",
	}
	for _, p := range rooted {
		got := auditReportRelativePath(p)
		if strings.HasPrefix(got, "/") || strings.HasPrefix(got, "\\") {
			t.Fatalf("rooted path %q should be stripped to relative, got %q", p, got)
		}
		if len(got) >= 2 && ((got[0] >= 'A' && got[0] <= 'Z') || (got[0] >= 'a' && got[0] <= 'z')) && got[1] == ':' {
			t.Fatalf("rooted path %q should not remain drive-rooted, got %q", p, got)
		}
		if got == "" {
			t.Fatalf("rooted path %q should not become empty, got empty", p)
		}
		if filepath.IsAbs(got) {
			t.Fatalf("rooted path %q should not be absolute after stripping, got %q", p, got)
		}
		// Also ensure no leading slash after buildAuditSections would leak.
		if strings.HasPrefix(got, "/") {
			t.Fatalf("rooted path %q still starts with slash: %q", p, got)
		}
	}

	// Genuinely relative input must be returned unchanged (modulo forward slashes).
	rel := "internal/browser/x.go"
	gotRel := auditReportRelativePath(rel)
	wantRel := filepath.ToSlash(rel)
	wantRel = strings.ReplaceAll(wantRel, "\\", "/")
	if gotRel != wantRel {
		t.Fatalf("relative path %q should be unchanged (modulo slashes), got %q want %q", rel, gotRel, wantRel)
	}

	// Degenerate inputs must not panic. We use a recover wrapper to surface
	// a panic as a test failure rather than crashing the suite.
	degenerate := []string{"", "/", "C:", "//", "\\\\"}
	for _, p := range degenerate {
		func(path string) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("auditReportRelativePath(%q) panicked: %v", path, r)
				}
			}()
			_ = auditReportRelativePath(path)
		}(p)
	}
}

