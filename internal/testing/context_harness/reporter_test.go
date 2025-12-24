package context_harness

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReporterReportJSON(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, "json")

	scenario := &Scenario{
		Name: "scenario-json",
	}
	result := &TestResult{
		Scenario:      scenario,
		ActualMetrics: Metrics{CompressionRatio: 2.0},
		Passed:        true,
	}

	if err := reporter.Report(result); err != nil {
		t.Fatalf("Report JSON failed: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	rawScenario, ok := decoded["Scenario"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Scenario object in JSON output")
	}
	if rawScenario["Name"] != "scenario-json" {
		t.Fatalf("Scenario.Name = %v, want %q", rawScenario["Name"], "scenario-json")
	}
}

func TestReporterReportConsole(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewReporter(&buf, "console")

	scenario := &Scenario{
		Name: "scenario-console",
	}
	result := &TestResult{
		Scenario:      scenario,
		ActualMetrics: Metrics{CompressionRatio: 1.5},
		Passed:        true,
	}

	if err := reporter.Report(result); err != nil {
		t.Fatalf("Report console failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "CONTEXT TEST HARNESS REPORT") {
		t.Fatalf("expected console report header, got: %q", output)
	}
	if !strings.Contains(output, scenario.Name) {
		t.Fatalf("expected scenario name in console report")
	}
	if !strings.Contains(output, "STATUS: PASSED") {
		t.Fatalf("expected status in console report")
	}
}
