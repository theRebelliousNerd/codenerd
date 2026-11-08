package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"codenerd/internal/campaign"
	"github.com/spf13/cobra"
)

func TestRecurseBoundText_Variants(t *testing.T) {
	if got := recurseBoundText(campaign.RecurseConfig{Mission: "m"}); !strings.Contains(got, "bounded mission") {
		t.Fatalf("mission bound text = %q, want bounded mission", got)
	}
	if got := recurseBoundText(campaign.RecurseConfig{MaxWaves: 0}); !strings.Contains(got, "until stopped") {
		t.Fatalf("waves 0 text = %q, want until stopped", got)
	}
	if got := recurseBoundText(campaign.RecurseConfig{MaxWaves: 1}); got != "1 pass" {
		t.Fatalf("waves 1 text = %q, want 1 pass", got)
	}
	if got := recurseBoundText(campaign.RecurseConfig{MaxWaves: 3}); got != "3 passes" {
		t.Fatalf("waves 3 text = %q, want 3 passes", got)
	}
}

func TestPrintRecurseResult_NilWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	printRecurseResult(&buf, nil)
	if buf.Len() != 0 {
		t.Fatalf("nil result should write nothing, got %q", buf.String())
	}
}

func TestPrintRecurseResult_NonNilWritesSummary(t *testing.T) {
	var buf bytes.Buffer
	r := &campaign.RecurseCycleResult{}
	printRecurseResult(&buf, r)
	// Summary of zero value should still write a newline + summary (at least newline).
	if buf.Len() == 0 {
		t.Fatalf("non-nil result should write summary, got empty")
	}
}

func TestRunRecurse_ProgressGoesToOutOrStdout(t *testing.T) {
	oldWS := workspace
	oldPlan := recursePlan
	oldDry := recurseDryRun
	oldMission := recurseMission
	oldWaves := recurseWaves
	t.Cleanup(func() {
		workspace = oldWS
		recursePlan = oldPlan
		recurseDryRun = oldDry
		recurseMission = oldMission
		recurseWaves = oldWaves
	})
	ws := t.TempDir()
	workspace = ws
	recursePlan = false
	recurseDryRun = false
	recurseMission = ""
	recurseWaves = 0

	cmd := &cobra.Command{}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	_, err := runRecurse(cmd, nil)
	_ = err
	got := out.String()
	if !strings.Contains(got, "Recurse: improving") {
		t.Skipf("runRecurse did not reach progress banner (likely boot needs LLM); out=%q err=%v errBuf=%q", got, err, errBuf.String())
	}
	if !strings.Contains(got, ws) && !strings.Contains(got, "until stopped") {
		t.Errorf("banner %q should mention workspace or bound", got)
	}
}

func TestRunRecurse_BannerPinnedToOutOrStdout(t *testing.T) {
	oldWS := workspace
	oldKey := apiKey
	oldWaves := recurseWaves
	oldPlan := recursePlan
	oldDry := recurseDryRun
	oldMission := recurseMission
	t.Cleanup(func() {
		workspace = oldWS
		apiKey = oldKey
		recurseWaves = oldWaves
		recursePlan = oldPlan
		recurseDryRun = oldDry
		recurseMission = oldMission
	})
	ws := t.TempDir()
	workspace = ws
	apiKey = "pin-test-key"
	recurseWaves = 1
	recursePlan = false
	recurseDryRun = false
	recurseMission = ""
	cmd := &cobra.Command{}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	oldStdout := os.Stdout
	r, w, perr := os.Pipe()
	if perr != nil {
		t.Fatalf("pipe: %v", perr)
	}
	os.Stdout = w
	_, _ = runRecurse(cmd, nil)
	_ = w.Close()
	os.Stdout = oldStdout
	captured, _ := io.ReadAll(r)
	_ = r.Close()
	osOut := string(captured)
	outStr := out.String()
	if strings.Contains(outStr, "Recurse: improving") {
		if strings.Contains(osOut, "Recurse: improving") {
			t.Errorf("banner leaked to os.Stdout %q while also in OutOrStdout %q; must go only to OutOrStdout", osOut, outStr)
		}
		return
	}
	if strings.Contains(osOut, "Recurse: improving") {
		t.Errorf("banner went to os.Stdout %q instead of OutOrStdout %q; must use progressOut/OutOrStdout", osOut, outStr)
		return
	}
	raw, rerr := os.ReadFile("cmd_campaign_recurse.go")
	if rerr != nil {
		t.Fatalf("banner not reached (out=%q errBuf=%q osOut=%q) and cannot read source: %v", outStr, errBuf.String(), osOut, rerr)
	}
	src := string(raw)
	if strings.Contains(src, "fmt.Printf(\"\\nRecurse: improving") {
		t.Errorf("runRecurse still uses fmt.Printf for progress banner; must use progressOut/OutOrStdout (revert detected)")
	}
	if !strings.Contains(src, "fmt.Fprintf(progressOut") {
		t.Errorf("runRecurse must write banner via fmt.Fprintf(progressOut,...); got source without it")
	}
}
