package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/features"
)

// The leaf command is exercised through RunE directly: featuresCmd is
// registered on rootCmd, so Execute() would run the ROOT and fall through to
// the interactive chat.

func runFeaturesCmd(t *testing.T) string {
	t.Helper()
	var out bytes.Buffer
	featuresCmd.SetOut(&out)
	t.Cleanup(func() { featuresCmd.SetOut(nil) })
	if err := featuresCmd.RunE(featuresCmd, nil); err != nil {
		t.Fatalf("nerd features: %v", err)
	}
	return out.String()
}

func TestFeaturesCmd_ShouldReportEveryFlagWithItsSource(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	featuresJSON, featuresSchema = false, false

	out := runFeaturesCmd(t)
	for _, f := range features.Resolved() {
		if !strings.Contains(out, f.Name) {
			t.Errorf("output omits flag %q", f.Name)
		}
	}
	if !strings.Contains(out, "SOURCE") {
		t.Error("output has no SOURCE column")
	}
}

func TestFeaturesCmd_WhenSchemaRequested_ShouldPrintTheConfigSnippet(t *testing.T) {
	featuresSchema = true
	featuresJSON = false
	t.Cleanup(func() { featuresSchema = false })

	out := runFeaturesCmd(t)
	if out != features.ConfigSchemaJSON() {
		t.Fatal("--schema must print exactly features.ConfigSchemaJSON(), or the docs and the CLI can drift")
	}
	if !strings.Contains(out, "\"features\": {") {
		t.Error("schema output does not show the features block")
	}
}

func TestFeaturesCmd_WhenJSONRequested_ShouldEmitParseableOutput(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	featuresJSON = true
	featuresSchema = false
	t.Cleanup(func() { featuresJSON = false })

	var payload struct {
		Flags []features.Flag `json:"flags"`
	}
	if err := json.Unmarshal([]byte(runFeaturesCmd(t)), &payload); err != nil {
		t.Fatalf("--json output is not valid JSON: %v", err)
	}
	if len(payload.Flags) != len(features.Resolved()) {
		t.Fatalf("JSON reported %d flags, want %d", len(payload.Flags), len(features.Resolved()))
	}
}

// TestFeaturesCmd_WhenALegacyEnvVarIsSet_ShouldWarn keeps the NERD_* → CODENERD_*
// migration visible on the surface an operator reaches for when a flag is not
// behaving the way they expect.
func TestFeaturesCmd_WhenALegacyEnvVarIsSet_ShouldWarn(t *testing.T) {
	features.SetActive(nil)
	t.Cleanup(func() { features.SetActive(nil) })
	featuresJSON, featuresSchema = false, false
	t.Setenv("NERD_SKIP_ONBOARDING", "1")

	out := runFeaturesCmd(t)
	if !strings.Contains(out, "warning:") {
		t.Errorf("no deprecation warning in output:\n%s", out)
	}
	if !strings.Contains(out, "CODENERD_SKIP_ONBOARDING") {
		t.Error("the warning does not name the canonical replacement")
	}
	if !strings.Contains(out, "legacy-env") {
		t.Error("the source column does not attribute the value to the legacy variable")
	}
}
