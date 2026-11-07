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

func TestEnvInt_CanonicalGarbageFallsThroughToLegacy(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })
	// A garbage canonical value is "no override", so the legacy var still decides.
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "eight")
	t.Setenv("NERD_FAST_SCAN_WORKERS", "6")
	if got := FastScanWorkers(); got != 6 {
		t.Fatalf("FastScanWorkers() = %d, want 6 (garbage canonical must fall through to legacy)", got)
	}
	t.Setenv("CODENERD_FAST_AST_MAX_BYTES", "huge")
	t.Setenv("NERD_FAST_AST_MAX_BYTES", "4096")
	if got := FastASTMaxBytes(); got != 4096 {
		t.Fatalf("FastASTMaxBytes() = %d, want 4096 (garbage canonical must fall through to legacy)", got)
	}
}

func TestEnvInt_WhitespaceTrimmedAndZeroIgnored(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })
	// envInt trims surrounding whitespace before parsing.
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "  8  ")
	t.Setenv("NERD_FAST_SCAN_WORKERS", "")
	if got := FastScanWorkers(); got != 8 {
		t.Fatalf("FastScanWorkers() = %d, want 8 (surrounding spaces must be trimmed)", got)
	}
	// "0" is not a positive integer, so it is ignored like an unset var.
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "0")
	t.Setenv("NERD_FAST_SCAN_WORKERS", "")
	if got := FastScanWorkers(); got != 0 {
		t.Fatalf("FastScanWorkers() = %d, want 0 (zero must resolve as unset)", got)
	}
	got := strings.Join(Misconfigurations(), "\n")
	if !strings.Contains(got, `CODENERD_FAST_SCAN_WORKERS="0"`) {
		t.Errorf("Misconfigurations() should report the ignored zero value:\n%s", got)
	}
}

func TestMisconfigurations_WhitespaceOnlyIsUnset(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })
	t.Setenv("CODENERD_DARK_MODE", "   ")
	t.Setenv("CODENERD_FAST_SCAN_WORKERS", "   ")
	if got := Misconfigurations(); len(got) != 0 {
		t.Errorf("Misconfigurations() = %v, want empty for whitespace-only values", got)
	}
}

func TestDeprecations_GarbageLegacyStillReported(t *testing.T) {
	SetActive(nil)
	t.Cleanup(func() { SetActive(nil) })
	// Deprecations keys off presence, not parseability: a garbage legacy value
	// is still doing nothing the operator must hear about, and Misconfigurations
	// reports the same var as ignored.
	t.Setenv("CODENERD_FLIGHT_RECORDER", "")
	t.Setenv("NERD_FLIGHTREC", "on")
	dep := strings.Join(Deprecations(), "\n")
	if !strings.Contains(dep, "NERD_FLIGHTREC") {
		t.Fatalf("Deprecations() omitted garbage legacy value:\n%s", dep)
	}
	mis := strings.Join(Misconfigurations(), "\n")
	if !strings.Contains(mis, `NERD_FLIGHTREC="on"`) {
		t.Errorf("Misconfigurations() omitted garbage legacy value:\n%s", mis)
	}
	// Whitespace-only legacy is unset and must stay silent on both surfaces.
	t.Setenv("NERD_FLIGHTREC", "   ")
	if got := Deprecations(); len(got) != 0 {
		t.Errorf("Deprecations() = %v, want empty for whitespace-only legacy", got)
	}
}
