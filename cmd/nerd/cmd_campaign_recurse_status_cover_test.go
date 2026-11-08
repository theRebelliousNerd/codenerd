package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/campaign"
)

func runStatusCmd(t *testing.T, jsonFlag bool) (string, error) {
	t.Helper()
	ws := t.TempDir()
	c := newRecurseStatusCmd(ws)
	buf := &bytes.Buffer{}
	c.SetOut(buf)
	flagValue := "false"
	if jsonFlag {
		flagValue = "true"
	}
	if err := c.Flags().Set("json", flagValue); err != nil {
		t.Fatalf("set --json=%s: %v", flagValue, err)
	}
	err := c.RunE(c, nil)
	return buf.String(), err
}

func TestCampaignRecurseStatus_JSONTrue_EmitsParseableJSON(t *testing.T) {
	out, err := runStatusCmd(t, true)
	if err != nil {
		t.Fatalf("status --json RunE error = %v", err)
	}
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "{") {
		t.Fatalf("status --json output should be JSON object, got %q", out)
	}
	var decoded campaign.RecurseStatusJSON
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		t.Fatalf("status --json output is not parseable: %v output %q", err, out)
	}
	if decoded.State == "" {
		t.Errorf("status --json state should mirror text running state, got empty")
	}
}

func TestCampaignRecurseStatus_JSONFalse_EmitsTextReport(t *testing.T) {
	out, err := runStatusCmd(t, false)
	if err != nil {
		t.Fatalf("status text RunE error = %v", err)
	}
	if !strings.Contains(out, "Recurse:") {
		t.Errorf("status text output should contain human report, got %q", out)
	}
	if strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("status text output should not be JSON, got %q", out)
	}
}
