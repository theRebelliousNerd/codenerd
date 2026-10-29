package chat

import (
	"strings"
	"testing"

	"codenerd/internal/features"
)

func TestRenderFeaturesReport_ShouldListEveryResolvedFlag(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })

	report := renderFeaturesReport()

	for _, f := range features.Resolved() {
		if !strings.Contains(report, f.Name) {
			t.Errorf("/features omitted flag %q", f.Name)
		}
		if !strings.Contains(report, f.EnvVar) {
			t.Errorf("/features omitted env var %q", f.EnvVar)
		}
	}
	if !strings.Contains(report, "fast_scan_workers") {
		t.Error("/features omitted fast_scan_workers")
	}
}

// TestRenderFeaturesReport_ShouldMirrorSummary is the reason this command
// exists: the corpus asked for a chat surface mirroring Summary(), so the two
// must not be able to disagree.
func TestRenderFeaturesReport_ShouldMirrorSummary(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })

	if !strings.Contains(renderFeaturesReport(), features.Summary()) {
		t.Error("/features does not embed the Summary() line that Boot logs")
	}
}

func TestRenderFeaturesReport_WhenAnEnvOverrideIsActive_ShouldAttributeIt(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	t.Setenv("CODENERD_PROVENANCE", "1")

	report := renderFeaturesReport()
	if !strings.Contains(report, "| provenance | true | env |") {
		t.Errorf("/features did not attribute the env override:\n%s", report)
	}
}

// TestRenderFeaturesReport_WhenALegacyEnvVarIsSet_ShouldWarn keeps the
// migration visible where an operator actually is.
func TestRenderFeaturesReport_WhenALegacyEnvVarIsSet_ShouldWarn(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	t.Setenv("NERD_FLIGHTREC", "1")

	report := renderFeaturesReport()
	if !strings.Contains(report, "Deprecated environment variables") {
		t.Errorf("/features did not report the deprecated variable:\n%s", report)
	}
	if !strings.Contains(report, "CODENERD_FLIGHT_RECORDER") {
		t.Error("/features did not name the canonical replacement")
	}
	if !strings.Contains(report, "legacy-env") {
		t.Error("/features did not attribute the value to the legacy variable")
	}
}

// TestFeaturesCommand_ShouldBeRegisteredInHelp: an unregistered command works
// but is undiscoverable, which is how a slash command ends up unused.
func TestFeaturesCommand_ShouldBeRegisteredInHelp(t *testing.T) {
	for _, info := range CommandRegistry {
		if info.Name == "/features" {
			if !info.ShowInHelp {
				t.Error("/features is registered but hidden from /help")
			}
			return
		}
	}
	t.Fatal("/features is not in CommandRegistry, so /help will never list it")
}
