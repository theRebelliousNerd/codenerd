package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportAuditFacts_WhenAuditLogExists_ShouldEmitDeclsAndUniqueFacts(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	a := Audit()
	a.SessionStart("session-1")
	a.SafetyCheck("/edit main.go", true, "kernel policy permitted")
	a.SafetyCheck("/shell rm -rf /", false, "dangerous, not permitted")
	a.ShardSpawn("shard-1", "coder")
	a.ShardSpawn("shard-1", "coder") // duplicate fact: must collapse
	CloseAll()

	path, err := LatestAuditLogPath()
	if err != nil {
		t.Fatalf("LatestAuditLogPath: %v", err)
	}

	var out bytes.Buffer
	stats, err := ExportAuditFacts(path, &out, nil)
	if err != nil {
		t.Fatalf("ExportAuditFacts: %v", err)
	}
	got := out.String()

	if stats.Events != 5 {
		t.Errorf("parsed %d events, want 5", stats.Events)
	}
	if stats.Duplicates != 1 {
		t.Errorf("collapsed %d duplicates, want 1", stats.Duplicates)
	}
	for _, want := range []string{
		"Decl safety_check(Arg1, Arg2, Arg3, Arg4).",
		"Decl session_event(Arg1, Arg2, Arg3).",
		"Decl shard_lifecycle(Arg1, Arg2, Arg3, Arg4, Arg5).",
		"/safety_allow",
		"/safety_block",
		"# Offline forensic artifact. Do not load into the live kernel.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("export missing %q\n---\n%s", want, got)
		}
	}
	if n := strings.Count(got, `shard_lifecycle(`); n != 2 { // one Decl + one fact
		t.Errorf("expected the duplicate shard fact to collapse, saw %d occurrences", n)
	}
}

func TestExportAuditFacts_WhenFilteredByEventType_ShouldEmitOnlyThoseFamilies(t *testing.T) {
	ws := newWorkspace(t, `"debug_mode": true, "level": "debug"`)
	resetAllLoggingState(t)
	defer resetAllLoggingState(t)

	if err := Initialize(ws); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	Audit().SessionStart("session-1")
	Audit().SafetyCheck("/edit main.go", false, "denied")
	CloseAll()

	path, err := LatestAuditLogPath()
	if err != nil {
		t.Fatalf("LatestAuditLogPath: %v", err)
	}

	var out bytes.Buffer
	stats, err := ExportAuditFacts(path, &out, []AuditEventType{AuditSafetyBlock})
	if err != nil {
		t.Fatalf("ExportAuditFacts: %v", err)
	}
	if stats.Events != 1 || stats.Facts != 1 {
		t.Errorf("filter did not apply: events=%d facts=%d", stats.Events, stats.Facts)
	}
	if strings.Contains(out.String(), "session_event") {
		t.Error("filtered export leaked an unrelated event family")
	}
}

func TestExportAuditFacts_WhenLogMissing_ShouldReportNoAuditLog(t *testing.T) {
	var out bytes.Buffer
	_, err := ExportAuditFacts(filepath.Join(t.TempDir(), "nope_audit.log"), &out, nil)
	if err != ErrNoAuditLog {
		t.Errorf("err = %v, want ErrNoAuditLog", err)
	}
}

func TestExportAuditFacts_WhenFactContainsCommasInStrings_ShouldNotInflateArity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "run_audit.log")
	// A message with commas and an escaped quote inside a Mangle string: a naive
	// comma split declares the wrong arity for exactly the facts carrying the
	// most information.
	line := `{"event":"error_generic","mangle":"error_event(1, /error_generic, \"kernel\", \"failed: a, b, c\")."}`
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out bytes.Buffer
	stats, err := ExportAuditFacts(path, &out, nil)
	if err != nil {
		t.Fatalf("ExportAuditFacts: %v", err)
	}
	if got := stats.Predicates["error_event"]; got != 4 {
		t.Errorf("arity = %d, want 4 (commas inside the string are data)", got)
	}
}

func TestParseFactShape_WhenMalformed_ShouldReject(t *testing.T) {
	for _, bad := range []string{
		"",
		"no_parens.",
		"unterminated(\"quote).",
		"(missing_name).",
	} {
		if _, _, ok := parseFactShape(bad); ok {
			t.Errorf("accepted malformed fact %q", bad)
		}
	}
}
