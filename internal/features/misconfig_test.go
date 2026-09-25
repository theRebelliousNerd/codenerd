package features

import (
	"strings"
	"testing"
)

// A refused env value is reported, and still refused (GAP-FEAT-05,
// Docs/architecture/features/TODO.md). CODENERD_DARK_MODE=yes used to run as
// false with no message anywhere.
func TestMisconfigurations_WhenAnEnvValueDoesNotParse_ShouldReportItAndStillIgnoreIt(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })
	t.Setenv("CODENERD_DARK_MODE", "yes")
	t.Setenv("NERD_FLIGHTREC", "on")
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "eight")

	got := strings.Join(Misconfigurations(), "\n")
	for _, want := range []string{`CODENERD_DARK_MODE="yes"`, `NERD_FLIGHTREC="on"`, `CODENERD_FAST_SCAN_WORKERS="eight"`} {
		if !strings.Contains(got, want) {
			t.Errorf("Misconfigurations() does not report %s:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "1, 0, true or false") {
		t.Errorf("the report does not say what to write instead:\n%s", got)
	}

	// The no-flip guarantee holds: a refused value resolves as unset.
	if IsDarkModeEnabled() {
		t.Error("CODENERD_DARK_MODE=yes flipped dark mode on; an unparseable value must not override")
	}
	if IsFlightRecorderEnabled() {
		t.Error("NERD_FLIGHTREC=on turned the flight recorder on; an unparseable value must not override")
	}
	if FastScanWorkers() != 0 {
		t.Errorf("FastScanWorkers() = %d from a non-numeric value, want 0", FastScanWorkers())
	}
	for _, f := range Resolved() {
		if f.Name == "dark_mode" && f.Source != SourceDefault {
			t.Errorf("dark_mode source = %s with a refused env value, want default", f.Source)
		}
	}
}

func TestMisconfigurations_WhenEveryValueParses_ShouldBeEmpty(t *testing.T) {
	for _, f := range boolFlags {
		t.Setenv(f.envVar, "")
		if f.legacyEnvVar != "" {
			t.Setenv(f.legacyEnvVar, "")
		}
	}
	for _, f := range intFlags {
		t.Setenv(f.envVar, "")
		t.Setenv(f.legacyEnvVar, "")
	}
	t.Setenv("CODENERD_DARK_MODE", "TRUE")
	t.Setenv("CODENERD_PROVENANCE", "0")
	t.Setenv("CODENERD_FAST_AST_MAX_BYTES", "4096")
	if got := Misconfigurations(); len(got) != 0 {
		t.Errorf("Misconfigurations() = %v for values that all parse", got)
	}
}
